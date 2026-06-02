package router

import (
	"testing"

	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
)

func TestResolveBasic(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "app.lvh.me", Path: "/", Target: "http://localhost:3000", Source: "static"},
	})

	result := r.Resolve("app.lvh.me", "/hello")
	if result == nil {
		t.Fatal("expected match, got nil")
	}
	if result.Target != "http://localhost:3000" {
		t.Errorf("target = %q, want %q", result.Target, "http://localhost:3000")
	}
	if result.RewrittenPath != "/hello" {
		t.Errorf("rewrittenPath = %q, want %q", result.RewrittenPath, "/hello")
	}
}

func TestResolvePathSpecificity(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "app.lvh.me", Path: "/", Target: "http://localhost:3000", Source: "static"},
		{Hostname: "app.lvh.me", Path: "/api", Target: "http://localhost:8000", Source: "static"},
		{Hostname: "app.lvh.me", Path: "/api/v2", Target: "http://localhost:9000", Source: "static"},
	})

	tests := []struct {
		path           string
		expectedTarget string
	}{
		{"/", "http://localhost:3000"},
		{"/hello", "http://localhost:3000"},
		{"/api", "http://localhost:8000"},
		{"/api/users", "http://localhost:8000"},
		{"/api/v2", "http://localhost:9000"},
		{"/api/v2/items", "http://localhost:9000"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := r.Resolve("app.lvh.me", tt.path)
			if result == nil {
				t.Fatal("expected match, got nil")
			}
			if result.Target != tt.expectedTarget {
				t.Errorf("target = %q, want %q", result.Target, tt.expectedTarget)
			}
		})
	}
}

func TestResolveStripPath(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "app.lvh.me", Path: "/api", Target: "http://localhost:8000", StripPath: true, Source: "static"},
	})

	result := r.Resolve("app.lvh.me", "/api/users")
	if result == nil {
		t.Fatal("expected match, got nil")
	}
	if result.RewrittenPath != "/users" {
		t.Errorf("rewrittenPath = %q, want %q", result.RewrittenPath, "/users")
	}

	// Strip to root
	result = r.Resolve("app.lvh.me", "/api")
	if result == nil {
		t.Fatal("expected match, got nil")
	}
	if result.RewrittenPath != "/" {
		t.Errorf("rewrittenPath = %q, want %q", result.RewrittenPath, "/")
	}
}

func TestResolveNoMatch(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "app.lvh.me", Path: "/", Target: "http://localhost:3000", Source: "static"},
	})

	result := r.Resolve("other.lvh.me", "/hello")
	if result != nil {
		t.Error("expected nil, got match")
	}
}

func TestGetAllRoutes(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "a.lvh.me", Path: "/", Target: "http://localhost:3000", Source: "static"},
		{Hostname: "b.lvh.me", Path: "/", Target: "http://localhost:4000", Source: "docker"},
	})

	all := r.GetAllRoutes()
	if len(all) != 2 {
		t.Errorf("len = %d, want 2", len(all))
	}
}

func TestResolveHostRegexp(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "^[^.]+[.]internal[.]example$", IsRegexp: true, Path: "/", Target: "http://app-backend:80", Source: "traefik"},
	})

	tests := []struct {
		host  string
		match bool
	}{
		{"hotels.internal.example", true},
		{"us2.internal.example", true},
		{"dashboard.internal.example", true},
		{"a.b.internal.example", false}, // two labels — regex requires a single label
		{"hotels.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			result := r.Resolve(tt.host, "/calendar")
			if tt.match && result == nil {
				t.Fatalf("expected match for %q", tt.host)
			}
			if !tt.match && result != nil {
				t.Fatalf("expected no match for %q, got %q", tt.host, result.Target)
			}
			if tt.match && result.Target != "http://app-backend:80" {
				t.Errorf("target = %q", result.Target)
			}
		})
	}
}

func TestExactHostBeatsRegexp(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "^[^.]+[.]internal[.]example$", IsRegexp: true, Path: "/", Target: "http://app-backend:80", Source: "traefik"},
		{Hostname: "dashboard.internal.example", Path: "/", Target: "http://dashboard:5770", Source: "traefik"},
	})

	result := r.Resolve("dashboard.internal.example", "/")
	if result == nil {
		t.Fatal("expected match")
	}
	if result.Target != "http://dashboard:5770" {
		t.Errorf("target = %q, want exact-host route to win over regex", result.Target)
	}

	// A host only the regex matches still resolves to the regex target.
	result = r.Resolve("hotels.internal.example", "/")
	if result == nil || result.Target != "http://app-backend:80" {
		t.Errorf("regex fallback failed: %+v", result)
	}
}

func TestHasHost(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "dashboard.internal.example", Path: "/", Target: "http://dashboard:5770", Source: "traefik"},
		{Hostname: "^[^.]+[.]internal[.]example$", IsRegexp: true, Path: "/", Target: "http://app-backend:80", Source: "traefik"},
	})

	cases := map[string]bool{
		"dashboard.internal.example": true, // exact
		"hotels.internal.example":    true, // regex
		"a.b.internal.example":       false,
		"unknown.lvh.me":             false,
	}
	for host, want := range cases {
		if got := r.HasHost(host); got != want {
			t.Errorf("HasHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestUpdateInvalidRegexpSkipped(t *testing.T) {
	r := New()
	// An invalid pattern must not panic or poison the table; valid routes survive.
	r.Update([]provider.Route{
		{Hostname: "(unclosed", IsRegexp: true, Path: "/", Target: "http://bad:1", Source: "traefik"},
		{Hostname: "good.lvh.me", Path: "/", Target: "http://good:2", Source: "static"},
	})

	if result := r.Resolve("good.lvh.me", "/"); result == nil || result.Target != "http://good:2" {
		t.Errorf("valid route lost after invalid regex: %+v", result)
	}
}
