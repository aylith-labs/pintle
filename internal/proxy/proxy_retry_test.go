package proxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestDialRetryReachesUpstreamThatStartsLate covers an upstream that is not listening on the first
// attempt (a restart): the request is retried and delivered, body included, exactly once.
func TestDialRetryReachesUpstreamThatStartsLate(t *testing.T) {
	cases := []struct {
		name   string
		method string
		body   string
	}{
		{"get without body", http.MethodGet, ""},
		{"post with body", http.MethodPost, "payload-that-must-arrive-once"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := reserveAddr(t)
			var deliveries atomic.Int32
			received := make(chan string, 4)
			upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				deliveries.Add(1)
				body, _ := io.ReadAll(r.Body)
				received <- string(body)
				_, _ = w.Write([]byte("ok"))
			}))
			t.Cleanup(upstream.Close)

			settings := fastSettings()
			settings.retryBudget = 5 * time.Second
			front := frontServer(t, newTestHandler(t, settings, routeTo("http://"+addr)))

			started := make(chan error, 1)
			go func() {
				time.Sleep(300 * time.Millisecond)
				listener, err := net.Listen("tcp", addr)
				if err == nil {
					upstream.Listener.Close()
					upstream.Listener = listener
					upstream.Start()
				}
				started <- err
			}()

			resp := sendThrough(t, front, tc.method, "/items", tc.body, nil)
			body := readBody(t, resp)
			if err := <-started; err != nil {
				t.Skipf("could not rebind the reserved port: %v", err)
			}
			if resp.StatusCode != http.StatusOK || body != "ok" {
				t.Fatalf("status %d body %q; want 200 after the upstream started", resp.StatusCode, body)
			}
			if got := deliveries.Load(); got != 1 {
				t.Fatalf("upstream received the request %d times; want exactly once", got)
			}
			if got := <-received; got != tc.body {
				t.Fatalf("upstream received body %q; want %q", got, tc.body)
			}
		})
	}
}

// TestNoRetryAfterRequestWritten: an upstream that accepts, reads the request and hangs up has
// received it, so the proxy must not send it again.
func TestNoRetryAfterRequestWritten(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { listener.Close() })
			var accepted atomic.Int32
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					accepted.Add(1)
					if req, err := http.ReadRequest(bufio.NewReader(conn)); err == nil {
						_, _ = io.Copy(io.Discard, req.Body)
					}
					conn.Close()
				}
			}()

			settings := fastSettings()
			settings.retryBudget = 2 * time.Second
			front := frontServer(t, newTestHandler(t, settings, routeTo("http://"+listener.Addr().String())))
			resp := sendThrough(t, front, method, "/items", "written-once", nil)
			readBody(t, resp)
			if resp.StatusCode != http.StatusBadGateway {
				t.Fatalf("status %d; want 502 for an upstream that hung up after reading", resp.StatusCode)
			}
			time.Sleep(100 * time.Millisecond)
			if got := accepted.Load(); got != 1 {
				t.Fatalf("upstream accepted %d connections; want 1 (no retry after the request was written)", got)
			}
		})
	}
}

// scriptedTransport plays one step per attempt.
type scriptedTransport struct {
	steps    []func(*http.Request) (*http.Response, error)
	attempts int
}

func (s *scriptedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	step := s.steps[min(s.attempts, len(s.steps)-1)]
	s.attempts++
	return step(req)
}

var errDialRefused = &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}

// failDialClosingBody behaves like http.Transport on a refused dial: it closes the body unread.
func failDialClosingBody(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		req.Body.Close()
	}
	return nil, errDialRefused
}

type trackedBody struct {
	io.Reader
	closed atomic.Int32
}

func (b *trackedBody) Close() error { b.closed.Add(1); return nil }

func TestDialRetryBodyGuard(t *testing.T) {
	t.Run("a body closed by a failed dial is still sent whole on retry", func(t *testing.T) {
		body := &trackedBody{Reader: strings.NewReader("whole-body")}
		var sent string
		next := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
			failDialClosingBody,
			failDialClosingBody,
			func(req *http.Request) (*http.Response, error) {
				data, _ := io.ReadAll(req.Body)
				sent = string(data)
				req.Body.Close()
				return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
			},
		}}
		retry := &dialRetryTransport{next: next, budget: 2 * time.Second}
		req := httptest.NewRequest(http.MethodPost, "http://example.internal/", nil)
		req.Body = body
		if _, err := retry.RoundTrip(req); err != nil {
			t.Fatal(err)
		}
		if next.attempts != 3 || sent != "whole-body" {
			t.Fatalf("attempts %d sent %q; want 3 attempts delivering the whole body", next.attempts, sent)
		}
		if got := body.closed.Load(); got != 1 {
			t.Fatalf("body closed %d times; want once, after the final attempt", got)
		}
	})

	t.Run("a body that was read from is never retried", func(t *testing.T) {
		next := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
			func(req *http.Request) (*http.Response, error) {
				_, _ = req.Body.Read(make([]byte, 3))
				return nil, errDialRefused
			},
			func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
			},
		}}
		retry := &dialRetryTransport{next: next, budget: 2 * time.Second}
		req := httptest.NewRequest(http.MethodPost, "http://example.internal/", strings.NewReader("partly-read"))
		if _, err := retry.RoundTrip(req); err == nil || next.attempts != 1 {
			t.Fatalf("attempts %d err %v; want one attempt and its error", next.attempts, err)
		}
	})

	t.Run("an error after the connection exists is not retried", func(t *testing.T) {
		next := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){
			func(*http.Request) (*http.Response, error) {
				return nil, &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}
			},
		}}
		retry := &dialRetryTransport{next: next, budget: 2 * time.Second}
		if _, err := retry.RoundTrip(httptest.NewRequest(http.MethodGet, "http://example.internal/", nil)); err == nil || next.attempts != 1 {
			t.Fatalf("attempts %d err %v; want one attempt", next.attempts, err)
		}
	})

	t.Run("retries stop at the budget", func(t *testing.T) {
		next := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){failDialClosingBody}}
		retry := &dialRetryTransport{next: next, budget: 300 * time.Millisecond}
		began := time.Now()
		if _, err := retry.RoundTrip(httptest.NewRequest(http.MethodGet, "http://example.internal/", nil)); !isDialError(err) {
			t.Fatalf("err %v; want the dial error", err)
		}
		if elapsed := time.Since(began); elapsed > 600*time.Millisecond || next.attempts < 2 {
			t.Fatalf("took %v over %d attempts; want several attempts within the 300ms budget", elapsed, next.attempts)
		}
	})

	t.Run("retries never outlast the request's own deadline", func(t *testing.T) {
		next := &scriptedTransport{steps: []func(*http.Request) (*http.Response, error){failDialClosingBody}}
		retry := &dialRetryTransport{next: next, budget: 5 * time.Second}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		began := time.Now()
		_, err := retry.RoundTrip(httptest.NewRequest(http.MethodGet, "http://example.internal/", nil).WithContext(ctx))
		if !isDialError(err) || time.Since(began) > 450*time.Millisecond {
			t.Fatalf("err %v after %v; want the dial error before the 200ms deadline plus one backoff", err, time.Since(began))
		}
	})
}
