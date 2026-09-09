package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoopbackRedirect(t *testing.T) {
	for _, tc := range []struct {
		url  string
		port int
		want string
	}{
		{"http://app.lvh.me/api%2Fitem?q=a%2Fb", 443, "https://app.lvh.me/api%2Fitem?q=a%2Fb"},
		{"http://app.lvh.me:9080/deep?q=1", 9443, "https://app.lvh.me:9443/deep?q=1"},
		{"http://app.lvh.me:80/", 443, "https://app.lvh.me/"},
		{"http://[::1]:9080/deep", 9443, "https://[::1]:9443/deep"},
		{"http://[::1]:80/", 443, "https://[::1]/"},
	} {
		rec := httptest.NewRecorder()
		loopbackRedirectHandler(tc.port).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.url, nil))
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != tc.want {
			t.Errorf("redirect %s: %d %q, want %q", tc.url, rec.Code, rec.Header().Get("Location"), tc.want)
		}
	}
}

func TestLoopbackRedirectRejectsNonLoopback(t *testing.T) {
	for _, address := range []string{"", "0.0.0.0", "::", "localhost", "192.0.2.1"} {
		if err := StartLoopbackHTTPRedirect(context.Background(), 9080, address, 9443); err == nil {
			t.Errorf("non-loopback address %q accepted", address)
		}
	}
}
