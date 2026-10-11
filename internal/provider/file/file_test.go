package file

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestHealthConfigurationAdmission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.yaml")
	for _, h := range []string{
		"{path: /ready, interval_seconds: 5, timeout_seconds: 1}",
		"{path: /ready, interval_seconds: 300, timeout_seconds: 10}",
	} {
		os.WriteFile(path, []byte("routes:\n - host: app.test\n   targets: [8000, 8001]\n   health: "+h+"\n"), 0600)
		msg := New(path, "127.0.0.1").loadFile()
		if msg.Err != nil || len(msg.Routes) != 2 || msg.Routes[0].Health == nil {
			t.Fatalf("health config refused: %+v", msg)
		}
	}
	for _, h := range []string{
		"{path: 'http://foreign/ready', interval_seconds: 5, timeout_seconds: 1}",
		"{path: '//foreign/ready', interval_seconds: 5, timeout_seconds: 1}",
		"{path: '/ready?secret=1', interval_seconds: 5, timeout_seconds: 1}",
		"{path: '/ready#fragment', interval_seconds: 5, timeout_seconds: 1}",
		"{path: /ready, interval_seconds: 0, timeout_seconds: 1}",
		"{path: /ready, interval_seconds: 301, timeout_seconds: 1}",
		"{path: /ready, interval_seconds: 5, timeout_seconds: 6}",
		"{path: /ready, interval_seconds: 5.5, timeout_seconds: 1}",
	} {
		os.WriteFile(path, []byte("routes:\n - host: app.test\n   targets: [8000, 8001]\n   health: "+h+"\n"), 0600)
		if New(path, "127.0.0.1").loadFile().Err == nil {
			t.Fatalf("unsafe health accepted: %s", h)
		}
	}
	os.WriteFile(path, []byte("routes:\n - host: app.test\n   target: 8000\n   health: {path: /ready, interval_seconds: 5, timeout_seconds: 1}\n"), 0600)
	if New(path, "127.0.0.1").loadFile().Err == nil {
		t.Fatal("legacy health silently enabled")
	}
	for _, target := range []string{"http://foreign.example:8000", "http://192.0.2.1:8000", "http://localhost:8000", "http://host.docker.internal:8000", "http://host.wsl.internal:8000"} {
		os.WriteFile(path, []byte("routes:\n - host: app.test\n   targets: ['"+target+"']\n   health: {path: /ready, interval_seconds: 5, timeout_seconds: 1}\n"), 0600)
		if New(path, "127.0.0.1").loadFile().Err != nil {
			t.Fatal("explicit configured hostname/LAN health target refused")
		}
	}
	os.WriteFile(path, []byte("routes:\n - host: app.test\n   targets: ['http://[::1]:8000']\n   health: {path: /ready, interval_seconds: 5, timeout_seconds: 1}\n"), 0600)
	if New(path, "127.0.0.1").loadFile().Err != nil {
		t.Fatal("IPv6 loopback health refused")
	}
	document := "routes:\n"
	for i := 0; i < 129; i++ {
		document += fmt.Sprintf(" - host: app%d.test\n   targets: [8000]\n   health: {path: /ready, interval_seconds: 5, timeout_seconds: 1}\n", i)
	}
	os.WriteFile(path, []byte(document), 0600)
	if New(path, "127.0.0.1").loadFile().Err == nil {
		t.Fatal("unbounded health pairs accepted")
	}
}

func TestHostnames(t *testing.T) {
	cases := []struct {
		name  string
		route staticRouteConfig
		want  []string
	}{
		{
			name:  "host only",
			route: staticRouteConfig{Host: "stith.lvh.me"},
			want:  []string{"stith.lvh.me"},
		},
		{
			name:  "host and hosts union, declaration order",
			route: staticRouteConfig{Host: "stith.lvh.me", Hosts: []string{"stith.localhost", "stith.localtest.me"}},
			want:  []string{"stith.lvh.me", "stith.localhost", "stith.localtest.me"},
		},
		{
			name:  "hosts only, no host",
			route: staticRouteConfig{Hosts: []string{"a.lvh.me", "b.lvh.me"}},
			want:  []string{"a.lvh.me", "b.lvh.me"},
		},
		{
			name:  "duplicates collapse",
			route: staticRouteConfig{Host: "a.lvh.me", Hosts: []string{"a.lvh.me", "b.lvh.me"}},
			want:  []string{"a.lvh.me", "b.lvh.me"},
		},
		{
			// A stray "- " in the YAML must not register an empty hostname, which would
			// match every request the router could not otherwise place.
			name:  "blank entries dropped",
			route: staticRouteConfig{Host: "  ", Hosts: []string{"", "  ", "a.lvh.me"}},
			want:  []string{"a.lvh.me"},
		},
		{
			name:  "surrounding whitespace trimmed",
			route: staticRouteConfig{Hosts: []string{" a.lvh.me "}},
			want:  []string{"a.lvh.me"},
		},
		{
			name:  "nothing declared",
			route: staticRouteConfig{},
			want:  nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.route.hostnames()
			if len(got) != len(tc.want) {
				t.Fatalf("hostnames() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("hostnames()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// One service under several names is one route entry, and every name must reach the same target.
func TestLoadFileFansOutHosts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.yaml")
	body := `routes:
  - host: stith.lvh.me
    hosts:
      - stith.localhost
      - stith.localtest.me
    target: http://127.0.0.1:53307
  - host: solo.lvh.me
    target: 5174
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	msg := New(path, "127.0.0.1").loadFile()

	want := map[string]string{
		"stith.lvh.me":       "http://127.0.0.1:53307",
		"stith.localhost":    "http://127.0.0.1:53307",
		"stith.localtest.me": "http://127.0.0.1:53307",
	}
	got := make(map[string]string)
	for _, r := range msg.Routes {
		if _, ok := want[r.Hostname]; ok {
			got[r.Hostname] = r.Target
		}
	}
	if len(got) != len(want) {
		t.Fatalf("fanned-out routes = %v, want %v", got, want)
	}
	for host, target := range want {
		if got[host] != target {
			t.Errorf("%s -> %q, want %q", host, got[host], target)
		}
	}

	if len(msg.Routes) != 4 {
		t.Errorf("total routes = %d, want 4 (3 fanned out + 1 solo)", len(msg.Routes))
	}
}

func TestBlankReloadDocumentIsNotAnExplicitClear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.yaml")
	provider := New(path, "127.0.0.1")
	for _, content := range []string{"", " \n\t", "routes:\n", "routes: null\n", "{}\n", "# editor comment\n", "tcp:\n"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if provider.loadFile().Err == nil {
			t.Fatal("blank editor state accepted as clear")
		}
	}
	if err := os.WriteFile(path, []byte("routes: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got := provider.loadFile()
	if got.Err != nil || len(got.Routes) != 0 {
		t.Fatal("explicit empty routes must remain supported")
	}
}

func TestExplicitReplicaTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.yaml")
	for _, data := range []string{
		"routes:\n - host: app.test\n   targets: []\n",
		"routes:\n - host: app.test\n   targets: [8000.5, 8001]\n",
		"routes:\n - host: app.test\n   target: 8000\n   targets: [8001]\n",
		"routes:\n - host: app.test\n   targets: [8000, 8000]\n",
		"routes:\n - host: app.test\n   targets: [8000, 'http://user:secret@host:8001']\n",
		"routes:\n - host: app.test\n   targets: [8000, 'http://host:70000']\n",
	} {
		os.WriteFile(path, []byte(data), 0600)
		if msg := New(path, "127.0.0.1").loadFile(); msg.Err == nil {
			t.Fatalf("invalid pool accepted: %s", data)
		}
	}
	os.WriteFile(path, []byte("routes:\n - host: app.test\n   targets: [8000, 8001]\n   path: /api\n   strip: true\n"), 0600)
	msg := New(path, "127.0.0.1").loadFile()
	if msg.Err != nil || len(msg.Routes) != 2 {
		t.Fatalf("pool failed: %+v", msg)
	}
	for i, r := range msg.Routes {
		if r.BackendPool == "" || r.Path != "/api" || !r.StripPath || r.Target != []string{"http://127.0.0.1:8000", "http://127.0.0.1:8001"}[i] {
			t.Fatalf("bad member: %+v", r)
		}
	}
}

func TestHealthUsesConfiguredTargetResolution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.yaml")
	for _, tc := range []struct{ target, host, want string }{
		{"8000", "127.0.0.1", "http://127.0.0.1:8000"},
		{"8000", "host.docker.internal", "http://host.docker.internal:8000"},
		{"'http://192.168.1.50:3000'", "127.0.0.1", "http://192.168.1.50:3000"},
		{"'https://service.example:8443/api'", "127.0.0.1", "https://service.example:8443/api"},
		{"'http://host.docker.internal:3000'", "172.20.0.1", "http://172.20.0.1:3000"},
		{"'http://host.wsl.internal:3000'", "172.20.0.1", "http://host.wsl.internal:3000"},
	} {
		os.WriteFile(path, []byte("routes:\n - host: app.test\n   targets: ["+tc.target+"]\n   health: {path: /ready, interval_seconds: 5, timeout_seconds: 1}\n"), 0600)
		msg := New(path, tc.host).loadFile()
		if msg.Err != nil || len(msg.Routes) != 1 || msg.Routes[0].Target != tc.want || msg.Routes[0].Health == nil {
			t.Fatalf("configured target changed/refused: %+v expected %s", msg, tc.want)
		}
	}
}
