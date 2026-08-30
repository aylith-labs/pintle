package file

import (
	"os"
	"path/filepath"
	"testing"
)

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
