package proxy

import (
	"context"
	"errors"
	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/router"
	"github.com/aylith-labs/pintle/internal/stats"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type replicaTransport func(*http.Request) (*http.Response, error)

func (f replicaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReplicaHandlerSelectsOncePreservesRequestAndDoesNotReplay(t *testing.T) {
	r := router.New()
	r.Update([]provider.Route{{Hostname: "app.test", Path: "/api", Target: "http://one:8000", StripPath: true, BackendPool: "static:0"}, {Hostname: "app.test", Path: "/api", Target: "http://two:8000", StripPath: true, BackendPool: "static:0"}})
	h := NewHandler(r, stats.NewCollector(""))
	var targets []string
	h.transport = replicaTransport(func(req *http.Request) (*http.Response, error) {
		targets = append(targets, req.URL.Host)
		body, _ := io.ReadAll(req.Body)
		if req.Host != "app.test" || req.URL.Path != "/items" || req.URL.RawQuery != "q=1" || string(body) != "private-input" || req.Header.Get("Authorization") != "Bearer controlled" {
			t.Fatal("request contract changed")
		}
		if len(targets) == 1 {
			return nil, errors.New("controlled backend failure")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("accepted"))}, nil
	})
	for i, code := range []int{502, 200} {
		req := httptest.NewRequest("POST", "http://app.test/api/items?q=1", strings.NewReader("private-input"))
		req.Header.Set("Authorization", "Bearer controlled")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != code || len(targets) != i+1 {
			t.Fatalf("unexpected response/replay %d/%v", rec.Code, targets)
		}
	}
	if targets[0] != "one:8000" || targets[1] != "two:8000" {
		t.Fatal(targets)
	}
}

func TestHealthHandlerUnavailableMakesNoUserAttempt(t *testing.T) {
	r := router.New()
	r.Update([]provider.Route{{Hostname: "app.test", Path: "/", Target: "http://one:8000", BackendPool: "static:0", Source: "static", Health: &provider.HealthCheck{Path: "/ready", IntervalSeconds: 5, TimeoutSeconds: 1}}})
	h := NewHandler(r, stats.NewCollector(""))
	attempts := 0
	h.transport = replicaTransport(func(*http.Request) (*http.Response, error) { attempts++; return nil, errors.New("controlled failure") })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "http://app.test/items", strings.NewReader("private-input")))
	if rec.Code != 503 || attempts != 0 {
		t.Fatalf("unknown backend forwarded: %d/%d", rec.Code, attempts)
	}
	observed := h.stats.GetHostStats()["app.test"]
	recent := h.stats.GetRecentRequests(1)
	if observed == nil || observed.TotalRequests != 1 || observed.ErrorCount != 1 || len(recent) != 1 || recent[0].Status != 503 || recent[0].Target != "none" {
		t.Fatal("unavailable refusal missing from request metrics")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.RunHealth(ctx, replicaTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 204, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
		}))
	}()
	deadline := time.Now().Add(time.Second)
	for r.Resolve("app.test", "/").Unavailable && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if r.Resolve("app.test", "/").Unavailable {
		t.Fatal("controlled health not admitted")
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "http://app.test/items", strings.NewReader("private-input")))
	if rec.Code != 502 || attempts != 1 {
		t.Fatalf("failed mutation replayed: %d/%d", rec.Code, attempts)
	}
	cancel()
	<-done
}
