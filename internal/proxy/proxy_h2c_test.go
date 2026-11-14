package proxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/router"
	"github.com/aylith-labs/pintle/internal/stats"
)

// TestH2CRouteUsesHTTP2 verifies that a route flagged H2C reaches the upstream
// over HTTP/2 cleartext (what gRPC services like permission/guest/gps need),
// while a plain route stays on HTTP/1.1.
func TestH2CRouteUsesHTTP2(t *testing.T) {
	var protoMajor int
	h2cUpstream := httptest.NewServer(h2c.NewHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			protoMajor = r.ProtoMajor
			w.WriteHeader(http.StatusOK)
		}),
		&http2.Server{},
	))
	defer h2cUpstream.Close()

	r := router.New()
	r.Update([]provider.Route{
		{Hostname: "grpc.lvh.me", Path: "/", Target: h2cUpstream.URL, H2C: true, Source: "test"},
	})
	h := NewHandler(r, stats.NewCollector(""))

	req := httptest.NewRequest(http.MethodGet, "http://grpc.lvh.me/v1/whatever", nil)
	req.Host = "grpc.lvh.me"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if protoMajor != 2 {
		t.Errorf("upstream saw HTTP/%d; expected HTTP/2 for an h2c route", protoMajor)
	}
}

func TestPlainRouteUsesHTTP1(t *testing.T) {
	var protoMajor int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		protoMajor = r.ProtoMajor
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	r := router.New()
	r.Update([]provider.Route{
		{Hostname: "web.lvh.me", Path: "/", Target: upstream.URL, Source: "test"},
	})
	h := NewHandler(r, stats.NewCollector(""))

	req := httptest.NewRequest(http.MethodGet, "http://web.lvh.me/", nil)
	req.Host = "web.lvh.me"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if protoMajor != 1 {
		t.Errorf("upstream saw HTTP/%d; expected HTTP/1.1 for a plain route", protoMajor)
	}
}

// TestH2CDialRetryReachesUpstreamThatStartsLate: the gRPC path retries a refused dial the same way
// the HTTP/1 path does, so an h2c upstream that is restarting is reached once it listens.
func TestH2CDialRetryReachesUpstreamThatStartsLate(t *testing.T) {
	addr := reserveAddr(t)
	var protoMajor atomic.Int32
	upstream := httptest.NewUnstartedServer(h2c.NewHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			protoMajor.Store(int32(r.ProtoMajor))
			_, _ = w.Write([]byte("ok"))
		}),
		&http2.Server{},
	))
	t.Cleanup(upstream.Close)

	settings := fastSettings()
	settings.retryBudget = 5 * time.Second
	route := routeTo("http://" + addr)
	route.H2C = true
	front := frontServer(t, newTestHandler(t, settings, route))

	started := make(chan error, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			upstream.Listener.Close()
			upstream.Listener = listener
			upstream.Start()
		}
		started <- err
	}()

	resp := sendThrough(t, front, http.MethodGet, "/v1/whatever", "", nil)
	body := readBody(t, resp)
	if err := <-started; err != nil {
		t.Skipf("could not rebind the reserved port: %v", err)
	}
	if resp.StatusCode != http.StatusOK || body != "ok" {
		t.Fatalf("status %d body %q; want 200 after the h2c upstream started", resp.StatusCode, body)
	}
	if got := protoMajor.Load(); got != 2 {
		t.Fatalf("upstream saw HTTP/%d; want HTTP/2", got)
	}
}
