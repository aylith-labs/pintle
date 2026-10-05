package main

import "testing"

func TestConfiguredPassthroughTarget(t *testing.T) {
	calls := 0
	discovered := func() (string, int) { calls++; return "127.0.0.2", 443 }
	direct := resolvePassthroughTarget("127.0.0.1:15443", discovered)
	if direct == nil || direct.IP.String() != "127.0.0.1" || direct.Port != 15443 || calls != 0 {
		t.Fatal("explicit target was replaced by discovery")
	}
	for _, name := range []string{"", "traefik"} {
		x := resolvePassthroughTarget(name, discovered)
		if x == nil || x.IP.String() != "127.0.0.2" || x.Port != 443 {
			t.Fatal("discovery default changed")
		}
	}
	for _, name := range []string{"caddy", "other-container", "127.0.0.1:0", "127.0.0.1:65536", "http://127.0.0.1:443", "127.0.0.1:not-a-port"} {
		if resolvePassthroughTarget(name, discovered) != nil {
			t.Fatalf("invalid explicit target accepted: %s", name)
		}
	}
	if calls != 2 {
		t.Fatal("unknown target silently used discovered proxy")
	}
}
