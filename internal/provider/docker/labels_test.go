package docker

import (
	"testing"
)

func TestParseProxyLabels(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		labels := map[string]string{
			"local-proxy.host": "app.lvh.me",
			"local-proxy.port": "5173",
		}
		result := parseProxyLabels(labels)
		if result == nil {
			t.Fatal("expected result, got nil")
		}
		if len(result.Hosts) != 1 || result.Hosts[0] != "app.lvh.me" {
			t.Errorf("hosts = %v, want [app.lvh.me]", result.Hosts)
		}
		if result.Port != 5173 {
			t.Errorf("port = %d, want 5173", result.Port)
		}
		if result.Path != "/" {
			t.Errorf("path = %q, want /", result.Path)
		}
	})

	t.Run("with path and strip", func(t *testing.T) {
		labels := map[string]string{
			"local-proxy.host":  "app.lvh.me",
			"local-proxy.port":  "5173",
			"local-proxy.path":  "/api",
			"local-proxy.strip": "true",
		}
		result := parseProxyLabels(labels)
		if result == nil {
			t.Fatal("expected result")
		}
		if result.Path != "/api" {
			t.Errorf("path = %q, want /api", result.Path)
		}
		if !result.Strip {
			t.Error("strip = false, want true")
		}
	})

	t.Run("multi host", func(t *testing.T) {
		labels := map[string]string{
			"local-proxy.host": "app.lvh.me, api.lvh.me",
		}
		result := parseProxyLabels(labels)
		if result == nil {
			t.Fatal("expected result")
		}
		if len(result.Hosts) != 2 {
			t.Fatalf("hosts count = %d, want 2", len(result.Hosts))
		}
		if result.Hosts[0] != "app.lvh.me" || result.Hosts[1] != "api.lvh.me" {
			t.Errorf("hosts = %v", result.Hosts)
		}
	})

	t.Run("no host label", func(t *testing.T) {
		labels := map[string]string{
			"local-proxy.port": "5173",
		}
		result := parseProxyLabels(labels)
		if result != nil {
			t.Error("expected nil, got result")
		}
	})
}

func TestParseTraefikLabels(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		labels := map[string]string{
			"traefik.enable":                                     "true",
			"traefik.http.routers.app.rule":                      "Host(`app.lvh.me`)",
			"traefik.http.services.app.loadbalancer.server.port": "5173",
		}
		result := parseTraefikLabels(labels)
		if result == nil {
			t.Fatal("expected result")
		}
		if len(result.Hosts) != 1 || result.Hosts[0] != "app.lvh.me" {
			t.Errorf("hosts = %v", result.Hosts)
		}
		if result.Port != 5173 {
			t.Errorf("port = %d, want 5173", result.Port)
		}
		if result.Path != "/" {
			t.Errorf("path = %q, want /", result.Path)
		}
	})

	t.Run("with path and strip", func(t *testing.T) {
		labels := map[string]string{
			"traefik.enable":                                          "true",
			"traefik.http.routers.app.rule":                           "Host(`app.lvh.me`) && PathPrefix(`/api`)",
			"traefik.http.services.app.loadbalancer.server.port":      "5173",
			"traefik.http.middlewares.app-strip.stripprefix.prefixes": "/api",
		}
		result := parseTraefikLabels(labels)
		if result == nil {
			t.Fatal("expected result")
		}
		if result.Path != "/api" {
			t.Errorf("path = %q, want /api", result.Path)
		}
		if !result.Strip {
			t.Error("strip = false, want true")
		}
	})

	t.Run("not enabled", func(t *testing.T) {
		labels := map[string]string{
			"traefik.enable":                "false",
			"traefik.http.routers.app.rule": "Host(`app.lvh.me`)",
		}
		result := parseTraefikLabels(labels)
		if result != nil {
			t.Error("expected nil")
		}
	})
}

func TestParseCaddyLabels(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		labels := map[string]string{
			"caddy":               "app.lvh.me",
			"caddy.reverse_proxy": "{{upstreams 5173}}",
		}
		results := parseCaddyLabels(labels)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Hosts[0] != "app.lvh.me" {
			t.Errorf("host = %q", results[0].Hosts[0])
		}
		if results[0].Port != 5173 {
			t.Errorf("port = %d, want 5173", results[0].Port)
		}
	})

	t.Run("handle_path strips", func(t *testing.T) {
		labels := map[string]string{
			"caddy":               "app.lvh.me",
			"caddy.handle_path":   "/api/*",
			"caddy.reverse_proxy": "{{upstreams 8000}}",
		}
		results := parseCaddyLabels(labels)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Path != "/api" {
			t.Errorf("path = %q, want /api", results[0].Path)
		}
		if !results[0].Strip {
			t.Error("strip = false, want true")
		}
	})

	t.Run("no caddy label", func(t *testing.T) {
		labels := map[string]string{
			"some.other.label": "value",
		}
		results := parseCaddyLabels(labels)
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestParseTraefikTcpLabels(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		labels := map[string]string{
			"traefik.enable":                                      "true",
			"traefik.tcp.routers.redis.rule":                      "HostSNI(`redis.lvh.me`)",
			"traefik.tcp.routers.redis.entrypoints":               "redis",
			"traefik.tcp.services.redis.loadbalancer.server.port": "6379",
		}
		result := parseTraefikTcpLabels(labels)
		if result == nil {
			t.Fatal("expected result")
		}
		if result.Hostname != "redis.lvh.me" {
			t.Errorf("hostname = %q", result.Hostname)
		}
		if result.Entrypoint != "redis" {
			t.Errorf("entrypoint = %q", result.Entrypoint)
		}
		if result.ContainerPort != 6379 {
			t.Errorf("containerPort = %d", result.ContainerPort)
		}
	})

	t.Run("not enabled", func(t *testing.T) {
		labels := map[string]string{
			"traefik.tcp.routers.redis.rule": "HostSNI(`redis.lvh.me`)",
		}
		result := parseTraefikTcpLabels(labels)
		if result != nil {
			t.Error("expected nil")
		}
	})
}

func TestParseTraefikLabelsHostRegexp(t *testing.T) {
	t.Run("host regexp only", func(t *testing.T) {
		labels := map[string]string{
			"traefik.enable":                                     "true",
			"traefik.http.routers.app.rule":                      "HostRegexp(`^[^.]+[.]internal[.]example$`)",
			"traefik.http.services.app.loadbalancer.server.port": "80",
		}
		result := parseTraefikLabels(labels)
		if result == nil {
			t.Fatal("expected result")
		}
		if len(result.Hosts) != 0 {
			t.Errorf("hosts = %v, want empty", result.Hosts)
		}
		if len(result.HostPatterns) != 1 || result.HostPatterns[0] != "^[^.]+[.]internal[.]example$" {
			t.Errorf("hostPatterns = %v", result.HostPatterns)
		}
		if result.Port != 80 {
			t.Errorf("port = %d, want 80", result.Port)
		}
	})

	t.Run("host and host regexp combined", func(t *testing.T) {
		labels := map[string]string{
			"traefik.enable":                                   "true",
			"traefik.http.routers.x.rule":                      "Host(`a.lvh.me`) || HostRegexp(`^b[.]lvh[.]me$`)",
			"traefik.http.services.x.loadbalancer.server.port": "5000",
		}
		result := parseTraefikLabels(labels)
		if result == nil {
			t.Fatal("expected result")
		}
		if len(result.Hosts) != 1 || result.Hosts[0] != "a.lvh.me" {
			t.Errorf("hosts = %v, want [a.lvh.me]", result.Hosts)
		}
		if len(result.HostPatterns) != 1 || result.HostPatterns[0] != "^b[.]lvh[.]me$" {
			t.Errorf("hostPatterns = %v", result.HostPatterns)
		}
	})
}

func TestParseTraefikLabelsH2C(t *testing.T) {
	labels := map[string]string{
		"traefik.enable":                                        "true",
		"traefik.http.routers.grpc.rule":                        "Host(`grpc.lvh.me`)",
		"traefik.http.services.grpc.loadbalancer.server.port":   "9090",
		"traefik.http.services.grpc.loadbalancer.server.scheme": "h2c",
	}
	result := parseTraefikLabels(labels)
	if result == nil {
		t.Fatal("expected result")
	}
	if !result.H2C {
		t.Error("H2C = false, want true")
	}

	t.Run("default scheme is not h2c", func(t *testing.T) {
		plain := map[string]string{
			"traefik.enable":                                     "true",
			"traefik.http.routers.web.rule":                      "Host(`web.lvh.me`)",
			"traefik.http.services.web.loadbalancer.server.port": "8080",
		}
		result := parseTraefikLabels(plain)
		if result == nil {
			t.Fatal("expected result")
		}
		if result.H2C {
			t.Error("H2C = true, want false")
		}
	})
}
