package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestShutdownEndsStreamsButWaitsForSlowAnswers: once shutdown starts, a response that keeps
// streaming is closed after streamGrace, so the listener drain is not held for its whole grace
// period, while a request still waiting for its headers is answered normally.
func TestShutdownEndsStreamsButWaitsForSlowAnswers(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			time.Sleep(2500 * time.Millisecond)
			_, _ = w.Write([]byte("slow-ok"))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for {
			if _, err := w.Write([]byte("data: tick\n\n")); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
	}))
	t.Cleanup(upstream.Close)

	h := newTestHandler(t, fastSettings(), routeTo(upstream.URL))
	front := httptest.NewUnstartedServer(h)
	front.Start()
	t.Cleanup(front.Close)

	stream := sendThrough(t, front, http.MethodGet, "/events", "", nil)
	if _, err := stream.Body.Read(make([]byte, 16)); err != nil {
		t.Fatalf("stream did not start: %v", err)
	}
	slowDone := make(chan string, 1)
	go func() {
		resp := sendThrough(t, front, http.MethodGet, "/slow", "", nil)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		slowDone <- string(body)
	}()
	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go h.EndStreamsOnDone(ctx, stop)
	cancel()

	started := time.Now()
	drain, drainCancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer drainCancel()
	if err := front.Config.Shutdown(drain); err != nil {
		t.Fatalf("shutdown did not finish: %v", err)
	}
	took := time.Since(started)
	if body := <-slowDone; body != "slow-ok" {
		t.Fatalf("slow request got %q; want its answer", body)
	}
	if took > 4*time.Second {
		t.Fatalf("shutdown took %s; the stream held it open", took)
	}
	if _, err := io.ReadAll(stream.Body); err == nil {
		t.Fatal("stream ended cleanly; want it cut by the shutdown")
	}
}
