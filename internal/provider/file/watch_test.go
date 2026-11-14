package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aylith-labs/pintle/internal/provider"
)

func routesBody(target string) string {
	return "routes:\n  - host: app.lvh.me\n    target: " + target + "\n"
}

func waitMessage(t *testing.T, ch <-chan provider.Message, want string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case msg := <-ch:
			if len(msg.Routes) == 1 && msg.Routes[0].Target == want {
				return
			}
		case <-deadline:
			t.Fatalf("no reload carrying target %s", want)
		}
	}
}

func TestWatchSurvivesAtomicSaveAndIgnoresSiblings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.yaml")
	if err := os.WriteFile(path, []byte(routesBody("http://127.0.0.1:1111")), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan provider.Message, 64)
	done := make(chan struct{})
	go func() { defer close(done); New(path, "127.0.0.1").Run(ctx, ch) }()
	defer func() { cancel(); <-done }()

	waitMessage(t, ch, "http://127.0.0.1:1111") // initial load

	// Plain in-place write.
	if err := os.WriteFile(path, []byte(routesBody("http://127.0.0.1:2222")), 0o600); err != nil {
		t.Fatal(err)
	}
	waitMessage(t, ch, "http://127.0.0.1:2222")

	// Two atomic saves in a row: the second only works if the watch outlived the first.
	for _, target := range []string{"http://127.0.0.1:3333", "http://127.0.0.1:4444"} {
		tmp := filepath.Join(dir, ".routes.yaml.swp")
		if err := os.WriteFile(tmp, []byte(routesBody(target)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, path); err != nil {
			t.Fatal(err)
		}
		waitMessage(t, ch, target)
	}

	// An unrelated file in the directory must not trigger a reload.
	time.Sleep(300 * time.Millisecond)
	for len(ch) > 0 {
		<-ch
	}
	if err := os.WriteFile(filepath.Join(dir, "other.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-ch:
		t.Fatalf("unrelated file triggered a reload: %+v", msg)
	case <-time.After(700 * time.Millisecond):
	}
}
