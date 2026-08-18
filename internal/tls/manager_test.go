package tls

import (
	"os"
	"path/filepath"
	"testing"
)

// A cert is served because its file is present, never because another config section
// happens to name its domain. Regression: cert loading used to take the passthrough
// list, so dropping a passthrough entry silently stopped serving that domain's cert
// and the SNI callback fell back to the wrong one.
func TestDiscoverDomainsFindsEveryPair(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("lvh.me.pem")
	write("lvh.me-key.pem")
	write("example-local.com.pem")
	write("example-local.com-key.pem")
	write("orphan.pem") // no matching key — must be skipped
	write("notes.txt")  // not a cert at all

	got := discoverDomains(dir)
	want := []string{"example-local.com", "lvh.me"}

	if len(got) != len(want) {
		t.Fatalf("discoverDomains(%q) = %v, want %v", dir, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("domain %d = %q, want %q", i, got[i], want[i])
		}
	}
}
