package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
	"github.com/steven-pribilinskiy/local-proxy/internal/router"
	"github.com/steven-pribilinskiy/local-proxy/internal/stats"
)

// TestHostHeaderPreservation guards against regressing back to the
// "rewrite Host to upstream" behavior. Apps like Zitadel build URLs from
// r.Host (e.g. Console's environment.json `api` field) and break if the
// proxy substitutes the upstream's bind address. Traefik and Caddy both
// preserve Host by default; we must too.
func TestHostHeaderPreservation(t *testing.T) {
	// Upstream that records the Host header it received.
	var seenHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost = r.Host
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	// Build a Handler with a router that resolves auth.aylith.lvh.me to our upstream.
	r := router.New()
	r.Update([]provider.Route{
		{Hostname: "auth.aylith.lvh.me", Target: upstream.URL, Source: "test"},
	})
	h := NewHandler(r, stats.NewCollector(""))

	// Simulate an incoming request with a public hostname.
	req := httptest.NewRequest(http.MethodGet, "http://auth.aylith.lvh.me/anything", nil)
	req.Host = "auth.aylith.lvh.me"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxy, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	if seenHost != "auth.aylith.lvh.me" {
		if strings.HasPrefix(seenHost, upstreamURL.Host) {
			t.Errorf("Host was rewritten to upstream bind address %q; expected client Host preserved", seenHost)
		} else {
			t.Errorf("upstream saw Host=%q; expected %q", seenHost, "auth.aylith.lvh.me")
		}
	}
}
