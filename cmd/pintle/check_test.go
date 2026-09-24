package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckRoutesCLI(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "pintle")
	build := exec.Command("go", "build", "-o", bin, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, output)
	}

	for _, tc := range []struct {
		name, routes, want string
		valid, staticOnly  bool
	}{
		{"valid destinations", "routes:\n  - host: app.lvh.me\n    target: 5174\n  - host: api.lvh.me\n    target: http://127.0.0.1:3000\n", "2 HTTP route(s)", true, true},
		{"invalid destination", "routes:\n  - host: app.lvh.me\n    target: localhost:3000\n", "invalid HTTP target", false, true},
		{"invalid HTTP port", "routes:\n  - host: app.lvh.me\n    target: http://127.0.0.1:70000\n", "invalid HTTP target port", false, true},
		{"valid TCP and passthrough", "tcp:\n  - host: db.lvh.me\n    target: 5432\n    listen: 15432\npassthrough:\n  - domain: old.test\n    target: traefik\n", "1 TCP route(s), 1 passthrough domain(s)", true, false},
		{"invalid TCP destination", "tcp:\n  - host: db.lvh.me\n    target: localhost:bad\n    listen: 15432\n", "invalid TCP destination", false, false},
		{"unsupported passthrough destination", "passthrough:\n  - domain: old.test\n    target: caddy\n", "only traefik is supported", false, false},
		{"static-only excludes TCP", "tcp:\n  - host: db.lvh.me\n    target: 5432\n    listen: 15432\n", "--static-only forbids TCP routes", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "routes.yaml")
			if err := os.WriteFile(path, []byte(tc.routes), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			args := []string{"check", "--routes-file", path}
			if tc.staticOnly {
				args = append(args, "--static-only")
			}
			cmd := exec.CommandContext(ctx, bin, args...)
			cmd.Env = append(os.Environ(), "HOST_GATEWAY_IP=127.0.0.1")
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("check timed out: %v: %s", ctx.Err(), output)
			}
			if (err == nil) != tc.valid || !strings.Contains(string(output), tc.want) {
				t.Fatalf("check exit=%v output=%q, want valid=%v and %q", err, output, tc.valid, tc.want)
			}
		})
	}
}
