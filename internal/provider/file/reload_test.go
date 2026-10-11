package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aylith-labs/pintle/internal/aggregator"
	"github.com/aylith-labs/pintle/internal/config"
	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/router"
)

func TestFileUpdatesKeepLastAcceptedConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.yaml")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fileProvider := New(path, "127.0.0.1")
	cfg := &config.Config{}
	agg, updates := aggregator.New()
	rtr := router.New()
	var last aggregator.Configuration
	apply := func(msg provider.Message) error {
		if err := cfg.ValidateMessage(msg); err != nil {
			return err
		}
		agg.Update(msg)
		last = <-updates
		rtr.Update(last.Routes)
		return nil
	}

	// A bad file must also be refused at startup in ordinary (non-static) mode.
	write("routes:\n  - host: app.lvh.me\n    target: '://bad'\n")
	if err := apply(fileProvider.loadFile()); err == nil {
		t.Fatal("invalid startup target replaced the empty route table")
	}

	write("routes:\n  - host: app.lvh.me\n    path: /api\n    target: http://127.0.0.1:8080\ntcp:\n  - host: cache.lvh.me\n    target: 127.0.0.1:6380\n    listen: 6380\npassthrough:\n  - domain: other.example\n    target: traefik\n")
	if err := apply(fileProvider.loadFile()); err != nil {
		t.Fatalf("valid initial file: %v", err)
	}
	assertAccepted := func(target string, tcpPort int, domain string) {
		t.Helper()
		resolved := rtr.Resolve("app.lvh.me", "/api/items")
		if resolved == nil || resolved.Target != target || resolved.RewrittenPath != "/api/items" {
			t.Fatalf("HTTP route changed: %+v", resolved)
		}
		if len(last.TcpRoutes) != 1 || last.TcpRoutes[0].TargetPort != tcpPort {
			t.Fatalf("TCP route changed: %+v", last.TcpRoutes)
		}
		if len(last.Passthrough) != 1 || last.Passthrough[0].Domain != domain {
			t.Fatalf("passthrough changed: %+v", last.Passthrough)
		}
	}
	assertAccepted("http://127.0.0.1:8080", 6380, "other.example")

	// A mixed valid/invalid update must not replace any part of the accepted set.
	write("routes:\n  - host: app.lvh.me\n    path: /api\n    target: http://127.0.0.1:8081\n  - host: broken.lvh.me\n    target: '://bad'\ntcp:\n  - host: cache.lvh.me\n    target: 127.0.0.1:6381\n    listen: 6381\npassthrough:\n  - domain: new.example\n    target: traefik\n")
	if err := apply(fileProvider.loadFile()); err == nil {
		t.Fatal("invalid reload replaced the last accepted configuration")
	}
	assertAccepted("http://127.0.0.1:8080", 6380, "other.example")
	if rtr.HasHost("broken.lvh.me") {
		t.Fatal("part of a rejected update reached the router")
	}

	write("routes:\n  - host: app.lvh.me\n    path: /api\n    target: http://127.0.0.1:8082\ntcp:\n  - host: cache.lvh.me\n    target: 127.0.0.1:6382\n    listen: 6382\npassthrough:\n  - domain: fixed.example\n    target: traefik\n")
	if err := apply(fileProvider.loadFile()); err != nil {
		t.Fatalf("corrected reload: %v", err)
	}
	assertAccepted("http://127.0.0.1:8082", 6382, "fixed.example")
}
