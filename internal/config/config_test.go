package config

import (
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/aylith-labs/pintle/internal/aggregator"
	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/router"
)

func testLoad(t *testing.T, args ...string) (*Config, error) {
	t.Helper()
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return load(flags, args)
}

func TestStaticOnlyConfiguration(t *testing.T) {
	for _, key := range []string{"LISTEN_PORT", "HTTP_PORT", "VITE_DEV_URL", "HOST_GATEWAY_IP"} {
		t.Setenv(key, "")
	}
	cfg, err := testLoad(t)
	if err != nil || cfg.StaticOnly || cfg.LoopbackAddress != "" || cfg.ListenPort != 9443 || cfg.HTTPPort != 9080 {
		t.Fatalf("legacy defaults changed: %+v, %v", cfg, err)
	}
	t.Setenv("HOST_GATEWAY_IP", "192.0.2.1")
	cfg, err = testLoad(t, "--static-only", "--listen-port=443", "--http-port=80")
	if err != nil || !cfg.StaticOnly || cfg.LoopbackAddress != "127.0.0.1" || cfg.HostAddress != "127.0.0.1" {
		t.Fatalf("static-only defaults/gateway isolation: %+v, %v", cfg, err)
	}
	for _, address := range []string{"127.0.0.1", "127.0.0.2", "::1", "::ffff:127.0.0.1"} {
		if _, err := testLoad(t, "--static-only", "--loopback-address="+address); err != nil {
			t.Errorf("loopback %s rejected: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0", "::", "192.168.1.10", "localhost", "lvh.me", "127.1", "127.0.0.1:443", "[::1]", "::1%lo"} {
		if _, err := testLoad(t, "--static-only", "--loopback-address="+address); err == nil || !strings.Contains(err.Error(), "literal loopback IP") {
			t.Errorf("unsafe/ambiguous address %q: %v", address, err)
		}
	}
	for _, args := range [][]string{
		{"--loopback-address=127.0.0.1"},
		{"--static-only", "--port-redirect"},
		{"--static-only", "--http-port=9443"},
		{"--static-only", "--http-port=0"},
		{"--static-only", "--listen-port=65536"},
		{"--static-only", "--listen-port=-1"},
	} {
		if _, err := testLoad(t, args...); err == nil {
			t.Errorf("invalid flags accepted: %v", args)
		}
	}
	t.Setenv("VITE_DEV_URL", "http://127.0.0.1:5175")
	if _, err := testLoad(t, "--static-only"); err == nil || !strings.Contains(err.Error(), "VITE_DEV_URL") {
		t.Fatalf("implicit dashboard upstream accepted: %v", err)
	}
}

func TestStaticOnlyDynamicBoundary(t *testing.T) {
	cfg := &Config{StaticOnly: true}
	agg, updates := aggregator.New()
	rtr := router.New()
	apply := func(msg provider.Message) error {
		if err := cfg.ValidateMessage(msg); err != nil {
			return err
		}
		agg.Update(msg)
		merged := <-updates
		if len(merged.TcpRoutes) != 0 || len(merged.Passthrough) != 0 {
			t.Fatal("forbidden listener configuration reached aggregation")
		}
		rtr.Update(merged.Routes)
		return nil
	}
	valid := provider.Message{ProviderName: "file", Routes: []provider.Route{
		{Hostname: "app.lvh.me", Path: "/api", Target: "http://127.0.0.1:8000", Source: "static", StripPath: true},
	}}
	if err := apply(valid); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []provider.Message{
		{ProviderName: "file", TcpRoutes: []provider.TcpRoute{{ListenPort: 5432}}},
		{ProviderName: "file", Passthrough: []provider.PassthroughDomain{{Domain: "example.test"}}},
		{ProviderName: "docker", Routes: valid.Routes},
		{ProviderName: "file", Err: errors.New("invalid YAML")},
	} {
		if err := apply(invalid); err == nil {
			t.Fatal("invalid update accepted")
		}
		if got := rtr.Resolve("app.lvh.me", "/api/items"); got == nil || got.RewrittenPath != "/items" || got.Target != "http://127.0.0.1:8000" {
			t.Fatalf("last accepted route changed: %+v", got)
		}
	}
	valid.Routes[0].Target = "http://127.0.0.1:8001"
	if err := apply(valid); err != nil || rtr.Resolve("app.lvh.me", "/api/items").Target != valid.Routes[0].Target {
		t.Fatalf("valid reload did not recover: %v", err)
	}
	if rtr.Resolve("other.lvh.me", "/api/items") != nil || rtr.Resolve("app.lvh.me", "/api-other") != nil {
		t.Fatal("exact host/path boundary widened")
	}
	if err := apply(provider.Message{ProviderName: "file"}); err != nil || rtr.HasHost("app.lvh.me") {
		t.Fatalf("valid removal failed: %v", err)
	}
	if err := (&Config{}).ValidateMessage(provider.Message{ProviderName: "docker", TcpRoutes: []provider.TcpRoute{{ListenPort: 5432}}}); err != nil {
		t.Fatalf("legacy dynamic mode restricted: %v", err)
	}
}
