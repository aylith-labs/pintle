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
