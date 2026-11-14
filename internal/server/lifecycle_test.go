package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServersCarryHeaderAndIdleLimitsOnly(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if srv.ReadHeaderTimeout != 10*time.Second || srv.IdleTimeout != 120*time.Second {
		t.Fatalf("limits: header %v idle %v", srv.ReadHeaderTimeout, srv.IdleTimeout)
	}
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Fatal("ReadTimeout/WriteTimeout would cut streams and large bodies")
	}
	if srv.ErrorLog == nil {
		t.Fatal("ErrorLog unset")
	}
}

func TestServerLogFilter(t *testing.T) {
	var got []string
	w := &serverLogWriter{emit: func(line string) { got = append(got, line) }}
	for _, line := range []string{
		"http: TLS handshake error from 192.0.2.1:5555: EOF\n",
		"http: panic serving 192.0.2.1:5556: boom\n",
		"http2: server: error reading preface from client: timeout\n",
		"\n",
	} {
		if n, err := w.Write([]byte(line)); n != len(line) || err != nil {
			t.Fatalf("Write(%q) = %d, %v", line, n, err)
		}
	}
	if len(got) != 2 || got[0] != "http: panic serving 192.0.2.1:5556: boom" {
		t.Fatalf("forwarded lines: %q", got)
	}
}

func TestShutdownLetsInFlightRequestFinish(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	srv := newHTTPServer("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		io.WriteString(w, "finished")
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(listener)
	ctx, cancel := context.WithCancel(context.Background())
	shutdownOnDone(ctx, srv, 5*time.Second)

	type result struct {
		body string
		err  error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		got <- result{string(body), err}
	}()
	<-started
	cancel()

	waited := make(chan struct{})
	go func() { WaitForShutdown(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("shutdown finished while a request was in flight")
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if r := <-got; r.err != nil || r.body != "finished" {
		t.Fatalf("in-flight request: %q, %v", r.body, r.err)
	}
	select {
	case <-waited:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not complete after the request finished")
	}
}

func TestShutdownClosesWhatIsLeftAfterGrace(t *testing.T) {
	started := make(chan struct{})
	stuck := make(chan struct{})
	defer close(stuck)
	srv := newHTTPServer("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-stuck
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(listener)
	ctx, cancel := context.WithCancel(context.Background())
	shutdownOnDone(ctx, srv, 100*time.Millisecond)
	errc := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			resp.Body.Close()
		}
		errc <- err
	}()
	<-started
	cancel()
	done := make(chan struct{})
	go func() { WaitForShutdown(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("a stuck request held shutdown past the grace period")
	}
	if err := <-errc; err == nil {
		t.Fatal("stuck request completed instead of being closed")
	}
}
