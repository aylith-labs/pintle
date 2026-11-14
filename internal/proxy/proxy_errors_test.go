package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aylith-labs/pintle/internal/provider"
)

// TestCancelledClientRecorded499: a client that gives up is not an upstream failure. Nothing is
// written, the status recorded is 499, it is not counted as an error, and no ERROR line is logged.
func TestCancelledClientRecorded499(t *testing.T) {
	capture := captureLog(t)
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(upstream.Close)
	t.Cleanup(func() { close(release) })

	h := newTestHandler(t, fastSettings(), routeTo(upstream.URL))
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "http://app.lvh.me/api/health", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	time.AfterFunc(100*time.Millisecond, cancel)
	h.ServeHTTP(rec, req)

	if rec.Body.Len() != 0 || len(rec.Header()) != 0 {
		t.Fatalf("wrote %q with headers %v to a client that left; want nothing", rec.Body.String(), rec.Header())
	}
	recent := h.stats.GetRecentRequests(1)
	if len(recent) != 1 || recent[0].Status != StatusClientClosedRequest {
		t.Fatalf("recorded %+v; want status 499", recent)
	}
	if got := h.stats.GetHostStats()[testHost].ErrorCount; got != 0 {
		t.Fatalf("error count %d; a client cancel is not an upstream error", got)
	}
	logged := capture.text()
	if strings.Contains(logged, "ERROR") || !strings.Contains(logged, "499") {
		t.Fatalf("log %q; want a 499 access line and no ERROR line", logged)
	}
}

// TestSlowHeadersYield504 uses a short header timeout to stand in for the 60s production value.
func TestSlowHeadersYield504(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(upstream.Close)
	t.Cleanup(func() { close(release) })

	settings := fastSettings()
	settings.responseHeaderTimeout = 100 * time.Millisecond
	front := frontServer(t, newTestHandler(t, settings, routeTo(upstream.URL)))
	resp := sendThrough(t, front, http.MethodGet, "/slow", "", http.Header{"Accept": {"application/json"}})
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusGatewayTimeout || !strings.Contains(body, `"upstream_timeout"`) {
		t.Fatalf("status %d body %q; want 504 upstream_timeout", resp.StatusCode, body)
	}
	if resp.Header.Get("Retry-After") != "" {
		t.Fatalf("Retry-After %q on a 504; it is sent only with 503", resp.Header.Get("Retry-After"))
	}
}

// TestHeaderTimeoutDoesNotCutStream: once headers arrived, a stream outlives the header timeout.
func TestHeaderTimeoutDoesNotCutStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		for chunk := range 4 {
			fmt.Fprintf(w, "chunk-%d;", chunk)
			http.NewResponseController(w).Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	t.Cleanup(upstream.Close)

	settings := fastSettings()
	settings.responseHeaderTimeout = 50 * time.Millisecond
	front := frontServer(t, newTestHandler(t, settings, routeTo(upstream.URL)))
	resp := sendThrough(t, front, http.MethodGet, "/stream", "", nil)
	if body := readBody(t, resp); body != "chunk-0;chunk-1;chunk-2;chunk-3;" {
		t.Fatalf("body %q; want every chunk of a stream that outlived the header timeout", body)
	}
}

// TestRefusedUpstreamAnswers503 checks the status, the Retry-After hint, the format chosen for the
// caller, and that the internal upstream address never reaches the client.
func TestRefusedUpstreamAnswers503(t *testing.T) {
	addr := reserveAddr(t)
	host, _, _ := net.SplitHostPort(addr)
	cases := []struct {
		name        string
		path        string
		accept      string
		contentType string
		check       func(t *testing.T, body string)
	}{
		{"json accept", "/items", "application/json", "application/json", func(t *testing.T, body string) {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(body), &parsed); err != nil || parsed["error"] != "upstream_unavailable" || parsed["retryAfter"] != float64(2) {
				t.Fatalf("body %q; want upstream_unavailable with retryAfter 2", body)
			}
		}},
		{"api path", "/api/things", "", "application/json", func(t *testing.T, body string) {
			if !strings.Contains(body, `"upstream_unavailable"`) {
				t.Fatalf("body %q; want JSON for an /api/ path", body)
			}
		}},
		{"browser", "/", "text/html,application/xhtml+xml,*/*;q=0.8", "text/html", func(t *testing.T, body string) {
			for _, want := range []string{`http-equiv="refresh"`, "prefers-color-scheme", "restarting or not running", testHost} {
				if !strings.Contains(body, want) {
					t.Fatalf("page lacks %q: %s", want, body)
				}
			}
		}},
		{"plain", "/file.txt", "*/*", "text/plain", func(t *testing.T, body string) {
			if !strings.HasPrefix(body, "Service unavailable.") {
				t.Fatalf("body %q; want plain text", body)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			front := frontServer(t, newTestHandler(t, fastSettings(), routeTo("http://"+addr)))
			header := http.Header{}
			if tc.accept != "" {
				header.Set("Accept", tc.accept)
			}
			resp := sendThrough(t, front, http.MethodGet, tc.path, "", header)
			body := readBody(t, resp)
			if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") != "2" {
				t.Fatalf("status %d Retry-After %q; want 503 with Retry-After 2", resp.StatusCode, resp.Header.Get("Retry-After"))
			}
			if !strings.HasPrefix(resp.Header.Get("Content-Type"), tc.contentType) {
				t.Fatalf("Content-Type %q; want %s", resp.Header.Get("Content-Type"), tc.contentType)
			}
			if strings.Contains(body, addr) || strings.Contains(body, host) {
				t.Fatalf("body names the upstream address: %q", body)
			}
			tc.check(t, body)
		})
	}
}

func TestBrowserPageDoesNotRefreshAPost(t *testing.T) {
	front := frontServer(t, newTestHandler(t, fastSettings(), routeTo("http://"+reserveAddr(t))))
	resp := sendThrough(t, front, http.MethodPost, "/form", "field=1", http.Header{"Accept": {"text/html"}})
	if body := readBody(t, resp); strings.Contains(body, `http-equiv="refresh"`) {
		t.Fatalf("a POST's error page reloads itself, which would resend the form: %s", body)
	}
}

func TestEmptyPoolAnswers503WithRetryAfter(t *testing.T) {
	h := newTestHandler(t, fastSettings(), provider.Route{
		Hostname: testHost, Path: "/", Target: "http://example.internal:8000", BackendPool: "static:0", Source: "static",
		Health: &provider.HealthCheck{Path: "/ready", IntervalSeconds: 5, TimeoutSeconds: 1},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://app.lvh.me/api/items", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "2" || !strings.Contains(rec.Body.String(), `"no_healthy_backend"`) {
		t.Fatalf("status %d Retry-After %q body %q; want 503, Retry-After 2, no_healthy_backend", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout awaiting response headers" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestClassify(t *testing.T) {
	live := httptest.NewRequest(http.MethodGet, "http://app.lvh.me/", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gone := live.WithContext(ctx)
	cases := []struct {
		name       string
		req        *http.Request
		err        error
		status     int
		clientGone bool
	}{
		{"client gone wins over any error", gone, errDialRefused, 0, true},
		{"refused dial", live, errDialRefused, 503, false},
		{"dial timeout", live, &net.OpError{Op: "dial", Err: timeoutError{}}, 503, false},
		{"reset", live, &net.OpError{Op: "read", Err: syscall.ECONNRESET}, 503, false},
		{"header timeout", live, timeoutError{}, 504, false},
		{"upstream deadline", live, fmt.Errorf("round trip: %w", context.DeadlineExceeded), 504, false},
		{"anything else", live, io.ErrUnexpectedEOF, 502, false},
		{"unknown", live, errors.New("controlled failure"), 502, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, clientGone := classify(tc.req, tc.err)
			if clientGone != tc.clientGone || f.status != tc.status {
				t.Fatalf("got status %d clientGone %v; want %d %v", f.status, clientGone, tc.status, tc.clientGone)
			}
		})
	}
}

func TestTransportTimeouts(t *testing.T) {
	http1 := newHTTP1Transport(defaultTransportSettings)
	if http1.DialContext == nil || http1.TLSHandshakeTimeout != 10*time.Second || http1.ExpectContinueTimeout != time.Second ||
		http1.ResponseHeaderTimeout != 60*time.Second || !http1.ForceAttemptHTTP2 || !http1.DisableCompression || http1.Proxy != nil {
		t.Fatalf("HTTP/1 transport settings changed: %+v", http1)
	}
	h2c := newH2CTransport(defaultTransportSettings)
	if h2c.ReadIdleTimeout != 30*time.Second || h2c.PingTimeout != 10*time.Second || !h2c.AllowHTTP {
		t.Fatalf("h2c transport settings changed: %+v", h2c)
	}
}
