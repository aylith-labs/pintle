package proxy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"

	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/stats"
)

// TestSSEFlushedPerEvent: the second event is only produced after the client has read the first,
// so a writer that buffers instead of flushing deadlocks the read and fails the test.
func TestSSEFlushedPerEvent(t *testing.T) {
	next := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\n")
		http.NewResponseController(w).Flush()
		select {
		case <-next:
		case <-time.After(5 * time.Second):
		}
		_, _ = io.WriteString(w, "data: two\n\n")
	}))
	t.Cleanup(upstream.Close)

	front := frontServer(t, newTestHandler(t, fastSettings(), routeTo(upstream.URL)))
	// Unflushed, even the response headers stay buffered, so the clock starts before the request.
	type firstEvent struct {
		reader *bufio.Reader
		body   io.Closer
		line   string
	}
	first := make(chan firstEvent, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodGet, front.URL+"/events", nil)
		req.Host = testHost
		resp, err := front.Client().Do(req)
		if err != nil {
			first <- firstEvent{line: "request failed: " + err.Error()}
			return
		}
		reader := bufio.NewReader(resp.Body)
		line, _ := reader.ReadString('\n')
		first <- firstEvent{reader: reader, body: resp.Body, line: line}
	}()
	var event firstEvent
	select {
	case event = <-first:
		if event.line != "data: one\n" {
			t.Fatalf("first line %q", event.line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first event not delivered before the second was produced; the stream is buffered")
	}
	defer event.body.Close()
	reader := event.reader
	close(next)
	rest, _ := io.ReadAll(reader)
	if string(rest) != "\ndata: two\n\n" {
		t.Fatalf("rest of stream %q", rest)
	}
}

// TestWebSocketUpgradeEndToEnd performs a real upgrade through the wrapped writer and keeps the
// connection idle past the header timeout, which must not apply once upgraded.
func TestWebSocketUpgradeEndToEnd(t *testing.T) {
	capture := captureLog(t)
	upstream := httptest.NewServer(websocket.Handler(func(conn *websocket.Conn) {
		_, _ = io.Copy(conn, conn)
	}))
	t.Cleanup(upstream.Close)

	settings := fastSettings()
	settings.responseHeaderTimeout = 100 * time.Millisecond
	h := newTestHandler(t, settings, provider.Route{Hostname: "127.0.0.1", Path: "/", Target: upstream.URL, Source: "test"})
	front := frontServer(t, h)

	conn, err := websocket.Dial("ws://"+front.Listener.Addr().String()+"/socket", "", "http://127.0.0.1/")
	if err != nil {
		t.Fatalf("upgrade through the proxy failed: %v", err)
	}
	for _, message := range []string{"hello", "after-idle"} {
		if message == "after-idle" {
			time.Sleep(300 * time.Millisecond)
		}
		if err := websocket.Message.Send(conn, message); err != nil {
			t.Fatal(err)
		}
		var echoed string
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if err := websocket.Message.Receive(conn, &echoed); err != nil || echoed != message {
			t.Fatalf("echo %q err %v; want %q", echoed, err, message)
		}
	}
	conn.Close()

	waitFor(t, "the upgrade to be recorded", func() bool { return len(h.stats.GetRecentRequests(1)) == 1 })
	if recent := h.stats.GetRecentRequests(1)[0]; recent.Status != http.StatusSwitchingProtocols {
		t.Fatalf("recorded status %d; want 101", recent.Status)
	}
	if !strings.Contains(capture.text(), "101   GET 127.0.0.1/socket -> ") {
		t.Fatalf("no 101 access line: %q", capture.text())
	}
}

// TestAccessLogLine keeps the "<status> <METHOD> host/path -> target" shape other tooling greps
// for, with duration and size appended, and logs once per request.
func TestAccessLogLine(t *testing.T) {
	capture := captureLog(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello world")
	}))
	t.Cleanup(upstream.Close)

	h := newTestHandler(t, fastSettings(), routeTo(upstream.URL))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://app.lvh.me/hello", nil))

	line := regexp.MustCompile(`(?m)^\d\d:\d\d:\d\d 200   GET app\.lvh\.me/hello -> ` + regexp.QuoteMeta(upstream.URL) + ` \d+(\.\d+)?(µs|ms|s) 11B$`)
	if matches := line.FindAllString(capture.text(), -1); len(matches) != 1 {
		t.Fatalf("want exactly one access line with duration and 11B, got %d in %q", len(matches), capture.text())
	}
	recent := h.stats.GetRecentRequests(1)
	if len(recent) != 1 || recent[0].Bytes != 11 || recent[0].Aborted {
		t.Fatalf("recorded %+v; want one clean 11-byte record", recent)
	}
}

// TestMidStreamAbortLogged: an upstream that dies after headers leaves the client a short body;
// that must be logged and recorded as aborted, not as a clean 200.
func TestMidStreamAbortLogged(t *testing.T) {
	capture := captureLog(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "0123456789")
		http.NewResponseController(w).Flush()
		conn, _, err := http.NewResponseController(w).Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	t.Cleanup(upstream.Close)

	h := newTestHandler(t, fastSettings(), routeTo(upstream.URL))
	front := frontServer(t, h)
	// The client sees either a short body or a connection closed before the buffered headers left.
	req, _ := http.NewRequest(http.MethodGet, front.URL+"/large", nil)
	req.Host = testHost
	if resp, err := front.Client().Do(req); err == nil {
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	waitFor(t, "the aborted response to be recorded", func() bool { return len(h.stats.GetRecentRequests(1)) == 1 })
	recent := h.stats.GetRecentRequests(1)[0]
	if !recent.Aborted || recent.Bytes != 10 {
		t.Fatalf("recorded %+v; want aborted after 10 bytes", recent)
	}
	if got := h.stats.GetHostStats()[testHost].ErrorCount; got != 1 {
		t.Fatalf("error count %d; an aborted response is an error", got)
	}
	if !regexp.MustCompile(`200   GET app\.lvh\.me/large -> \S+ \S+ 10B aborted`).MatchString(capture.text()) {
		t.Fatalf("no aborted access line in %q", capture.text())
	}
}

func TestXForwardedHeadersReceivedUpstream(t *testing.T) {
	cases := []struct {
		name    string
		inbound []string
	}{
		{"no inbound header", nil},
		{"spoofed inbound chain is dropped", []string{"203.0.113.9", "192.0.2.1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen http.Header
			var seenHost string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen, seenHost = r.Header.Clone(), r.Host
			}))
			t.Cleanup(upstream.Close)

			h := newTestHandler(t, fastSettings(), routeTo(upstream.URL))
			req := httptest.NewRequest(http.MethodGet, "http://app.lvh.me:443/", nil)
			req.RemoteAddr = "198.51.100.7:4321"
			req.Header["X-Forwarded-For"] = tc.inbound
			h.ServeHTTP(httptest.NewRecorder(), req)

			if got := seen.Values("X-Forwarded-For"); len(got) != 1 || got[0] != "198.51.100.7" {
				t.Fatalf("upstream X-Forwarded-For %q; want exactly the client IP with no port", got)
			}
			if seen.Get("X-Forwarded-Proto") != "https" || seen.Get("X-Forwarded-Host") != testHost || seenHost != "app.lvh.me:443" {
				t.Fatalf("proto %q host %q Host %q", seen.Get("X-Forwarded-Proto"), seen.Get("X-Forwarded-Host"), seenHost)
			}
		})
	}
}

func TestPathReachesUpstreamEncoded(t *testing.T) {
	cases := []struct {
		name  string
		route provider.Route
		path  string
		want  string
	}{
		{"strip keeps an encoded slash", provider.Route{Path: "/api", StripPath: true}, "/api/files/a%2Fb?x=1", "/files/a%2Fb?x=1"},
		{"strip of an unencoded path", provider.Route{Path: "/api", StripPath: true}, "/api/plain", "/plain"},
		{"strip down to the root", provider.Route{Path: "/api", StripPath: true}, "/api", "/"},
		{"no strip keeps an encoded slash", provider.Route{Path: "/"}, "/raw/a%2Fb", "/raw/a%2Fb"},
		{"strip with an encoded prefix character", provider.Route{Path: "/a b", StripPath: true}, "/a%20b/c%2Fd", "/c%2Fd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seenURI string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenURI = r.RequestURI
			}))
			t.Cleanup(upstream.Close)
			route := tc.route
			route.Hostname, route.Target, route.Source = testHost, upstream.URL, "test"
			h := newTestHandler(t, fastSettings(), route)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://app.lvh.me"+tc.path, nil))
			if rec.Code != http.StatusOK || seenURI != tc.want {
				t.Fatalf("status %d upstream saw %q; want %q", rec.Code, seenURI, tc.want)
			}
		})
	}
}

func TestHostOnly(t *testing.T) {
	cases := map[string]string{
		"app.lvh.me":     "app.lvh.me",
		"app.lvh.me:443": "app.lvh.me",
		"[::1]:8443":     "::1",
		"[::1]":          "::1",
		"::1":            "::1",
		"":               "",
	}
	for in, want := range cases {
		if got := hostOnly(in); got != want {
			t.Errorf("hostOnly(%q) = %q; want %q", in, got, want)
		}
	}
}

// TestIPv6 routes an IPv6-literal Host header and reaches an upstream listening on an IPv6 literal.
func TestIPv6(t *testing.T) {
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback: %v", err)
	}
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "v6")
	}))
	upstream.Listener.Close()
	upstream.Listener = listener
	upstream.Start()
	t.Cleanup(upstream.Close)
	if parsed, _ := url.Parse(upstream.URL); !strings.HasPrefix(parsed.Host, "[::1]:") {
		t.Fatalf("upstream URL %q is not an IPv6 literal", upstream.URL)
	}

	h := newTestHandler(t, fastSettings(), provider.Route{Hostname: "::1", Path: "/", Target: upstream.URL, Source: "test"})
	req := httptest.NewRequest(http.MethodGet, "http://[::1]:8443/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "v6" {
		t.Fatalf("status %d body %q; want the IPv6 route served", rec.Code, rec.Body.String())
	}
}

func TestUnroutedRequestsShareOneStatsKey(t *testing.T) {
	h := newTestHandler(t, fastSettings())
	for _, host := range []string{"random-1.example.internal", "random-2.example.internal", "random-3.example.internal"} {
		req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	hosts := h.stats.GetHostStats()
	if len(hosts) != 1 || hosts[stats.UnroutedHost] == nil || hosts[stats.UnroutedHost].TotalRequests != 3 {
		t.Fatalf("host stats %v; want all three under %q", hosts, stats.UnroutedHost)
	}
	if edges := h.stats.GetEdgeStats(); len(edges) != 1 {
		t.Fatalf("edge stats %v; want one key", edges)
	}
	if recent := h.stats.GetRecentRequests(1); recent[0].Hostname != "random-3.example.internal" || recent[0].Status != 404 {
		t.Fatalf("recent %+v; the request list keeps the real host", recent)
	}
}
