package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
	"github.com/steven-pribilinskiy/local-proxy/internal/router"
	"github.com/steven-pribilinskiy/local-proxy/internal/stats"
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
