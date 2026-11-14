package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
)

const (
	// A local upstream that has not completed a TCP handshake in 3s is down; a refused dial fails at once.
	dialTimeout = 3 * time.Second
	// Keep-alive probes detect a dead idle upstream connection before it is reused.
	dialKeepAlive = 30 * time.Second
	// TLS upstreams are local; a handshake slower than this is a hung peer.
	tlsHandshakeTimeout = 10 * time.Second
	// Send the body anyway if an upstream ignores "Expect: 100-continue" for this long.
	expectContinueTimeout = 1 * time.Second
	// Generous: a socket-activated upstream accepts at once and answers 6-26s after a restart, and
	// some endpoints legitimately take long to produce headers. It never applies after headers arrive,
	// so streams and upgraded connections are not cut.
	responseHeaderTimeout = 60 * time.Second
	// h2c: ping a connection that has been silent this long, so a dead gRPC upstream is not reused.
	h2cReadIdleTimeout = 30 * time.Second
	// h2c: close the connection when a ping is not answered in this long.
	h2cPingTimeout = 10 * time.Second
	// Total time spent retrying a dial that sent nothing, across all attempts.
	dialRetryBudget = 5 * time.Second
	// First wait between dial attempts; it doubles up to dialRetryMaxBackoff.
	dialRetryInitialBackoff = 50 * time.Millisecond
	dialRetryMaxBackoff     = 1 * time.Second
)

// transportSettings are the values tests shorten; production uses defaultTransportSettings.
type transportSettings struct {
	dialTimeout           time.Duration
	responseHeaderTimeout time.Duration
	retryBudget           time.Duration
}

var defaultTransportSettings = transportSettings{
	dialTimeout:           dialTimeout,
	responseHeaderTimeout: responseHeaderTimeout,
	retryBudget:           dialRetryBudget,
}

func newHTTP1Transport(settings transportSettings) *http.Transport {
	dialer := &net.Dialer{Timeout: settings.dialTimeout, KeepAlive: dialKeepAlive}
	return &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true, // a custom dialer disables HTTP/2 to https upstreams unless forced
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		ResponseHeaderTimeout: settings.responseHeaderTimeout,
		DisableCompression:    true, // pass compressed responses through
	}
}

// newH2CTransport speaks HTTP/2 cleartext for gRPC upstreams. AllowHTTP lets http:// URLs use
// HTTP/2, and the dialer returns a plain TCP conn so no TLS is attempted despite the DialTLSContext name.
func newH2CTransport(settings transportSettings) *http2.Transport {
	dialer := &net.Dialer{Timeout: settings.dialTimeout, KeepAlive: dialKeepAlive}
	return &http2.Transport{
		AllowHTTP:       true,
		ReadIdleTimeout: h2cReadIdleTimeout,
		PingTimeout:     h2cPingTimeout,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
	}
}

// dialRetryTransport retries a request whose connection could not be established at all, for any
// method: a failed dial wrote no byte of the request, so retrying it does not replay the request.
// It retries the same target and never selects another replica. Any error after a connection exists,
// or once the body has been read from, is returned as is.
type dialRetryTransport struct {
	next   http.RoundTripper
	budget time.Duration
}

func (t *dialRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempt := req
	var guard *retryBody
	if req.Body != nil && req.Body != http.NoBody {
		guard = &retryBody{body: req.Body}
		attempt = new(http.Request)
		*attempt = *req
		attempt.Body = guard
	}
	ctx := req.Context()
	deadline := time.Now().Add(t.budget)
	backoff := dialRetryInitialBackoff
	for {
		resp, err := t.next.RoundTrip(attempt)
		if err == nil || !isDialError(err) || (guard != nil && guard.started()) || ctx.Err() != nil {
			guard.release()
			return resp, err
		}
		wake := time.Now().Add(backoff)
		if wake.After(deadline) {
			guard.release()
			return nil, err
		}
		if ctxDeadline, ok := ctx.Deadline(); ok && wake.After(ctxDeadline) {
			guard.release()
			return nil, err
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			guard.release()
			return nil, err
		case <-timer.C:
		}
		backoff = min(backoff*2, dialRetryMaxBackoff)
	}
}

// isDialError reports whether err means no connection to the upstream was established.
// A dial timeout is included: the attempt connected to nothing.
func isDialError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// retryBody shields the request body from the Close a failed RoundTrip performs, so a retry can
// still send it. Once released, a pending or later Close reaches the real body. A body that has
// been read from is never retried, so it is never half-sent and then sent again.
type retryBody struct {
	body         io.ReadCloser
	read         atomic.Bool
	mu           sync.Mutex
	released     bool
	closePending bool
}

func (b *retryBody) Read(p []byte) (int, error) {
	b.read.Store(true)
	return b.body.Read(p)
}

func (b *retryBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.released {
		return b.body.Close()
	}
	b.closePending = true
	return nil
}

func (b *retryBody) started() bool { return b.read.Load() }

// release ends the retry window: a Close the transport already asked for happens now.
func (b *retryBody) release() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.released {
		return
	}
	b.released = true
	if b.closePending {
		_ = b.body.Close()
	}
}
