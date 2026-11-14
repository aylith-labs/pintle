package proxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aylith-labs/pintle/internal/logger"
	"github.com/aylith-labs/pintle/internal/router"
	"github.com/aylith-labs/pintle/internal/stats"
)

// Handler routes each request by host and path and proxies it upstream. One ReverseProxy serves
// every request; what differs per request travels in the request context as *requestState.
type Handler struct {
	router       *router.Router
	stats        *stats.Collector
	transport    http.RoundTripper
	h2cTransport http.RoundTripper
	proxy        *httputil.ReverseProxy
	streams      *bodyPhase
}

// requestState is the per-request data the shared ReverseProxy reads and fills in.
type requestState struct {
	hostname   string
	targetURL  *url.URL
	target     string // the upstream as routed; logged and recorded, never sent to the client
	outPath    string
	outRawPath string
	h2c        bool
	cancel     context.CancelFunc // ends the upstream exchange; a shutdown uses it to close streams

	clientGone     bool        // the client left before a response was written; nothing was sent
	upstreamFailed atomic.Bool // the upstream response body ended in an error
}

type stateKey struct{}

func stateFrom(ctx context.Context) *requestState {
	state, _ := ctx.Value(stateKey{}).(*requestState)
	return state
}

func NewHandler(r *router.Router, s *stats.Collector) *Handler {
	return newHandler(r, s, defaultTransportSettings)
}

func newHandler(r *router.Router, s *stats.Collector, settings transportSettings) *Handler {
	h := &Handler{
		router:       r,
		stats:        s,
		transport:    &dialRetryTransport{next: newHTTP1Transport(settings), budget: settings.retryBudget},
		h2cTransport: &dialRetryTransport{next: newH2CTransport(settings), budget: settings.retryBudget},
		streams:      newBodyPhase(),
	}
	h.proxy = &httputil.ReverseProxy{
		Rewrite:        h.rewrite,
		Transport:      roundTripperFunc(h.roundTrip),
		ErrorHandler:   h.handleError,
		ModifyResponse: h.modifyResponse,
		BufferPool:     newBufferPool(),
	}
	return h
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func (h *Handler) roundTrip(req *http.Request) (*http.Response, error) {
	if state := stateFrom(req.Context()); state != nil && state.h2c {
		return h.h2cTransport.RoundTrip(req)
	}
	return h.transport.RoundTrip(req)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	hostname := hostOnly(r.Host)
	path := r.URL.Path
	aw := newAccessWriter(w)
	ctx, cancel := context.WithCancel(r.Context())
	state := &requestState{hostname: hostname, cancel: cancel}
	label, unrouted := "", false

	defer func() {
		cancel()
		h.streams.leave(state)
		recovered := recover()
		h.complete(r, aw, state, label, unrouted, start, recovered != nil)
		if recovered != nil {
			panic(recovered)
		}
	}()

	result := h.router.Resolve(hostname, path)
	if result == nil {
		label, unrouted = "no route", true
		http.Error(aw, "No route for "+hostname, http.StatusNotFound)
		return
	}
	if result.Unavailable {
		label = "no healthy backend"
		writeFailure(aw, r, hostname, failureNoHealthyBackend)
		return
	}
	targetURL, err := url.Parse(result.Target)
	if err != nil {
		label = "invalid target"
		http.Error(aw, "Invalid target URL", http.StatusBadGateway)
		return
	}
	state.targetURL = targetURL
	state.target = result.Target
	state.h2c = result.Route.H2C
	state.outPath, state.outRawPath = rewritePath(r.URL, result.Route.Path, result.RewrittenPath, result.Route.StripPath)

	h.streams.begin(state, hostname+path)
	h.proxy.ServeHTTP(aw, r.WithContext(context.WithValue(ctx, stateKey{}, state)))
}

// complete logs and records the request once, after the response finished or failed.
func (h *Handler) complete(r *http.Request, aw *accessWriter, state *requestState, label string, unrouted bool, start time.Time, panicked bool) {
	elapsed := time.Since(start)
	status := aw.finalStatus()
	if state.clientGone {
		status = StatusClientClosedRequest
	}
	aborted := panicked || aw.writeErr != nil || state.upstreamFailed.Load()
	target := state.target
	logTarget := target
	if target == "" {
		target, logTarget = "none", label
	}
	logger.Access(r.Method, state.hostname, r.URL.Path, logTarget, status, elapsed, aw.bytes, aborted)
	h.stats.Record(stats.RequestRecord{
		Timestamp:  time.Now().UnixMilli(),
		Method:     r.Method,
		Hostname:   state.hostname,
		Path:       r.URL.Path,
		Target:     target,
		Status:     status,
		DurationMs: float64(elapsed.Microseconds()) / 1000.0,
		Bytes:      aw.bytes,
		Aborted:    aborted,
		Unrouted:   unrouted,
	})
}

func (h *Handler) rewrite(pr *httputil.ProxyRequest) {
	state := stateFrom(pr.In.Context())
	out := pr.Out
	out.URL.Scheme = state.targetURL.Scheme
	out.URL.Host = state.targetURL.Host
	out.URL.Path = state.outPath
	out.URL.RawPath = state.outRawPath
	// The query string goes upstream exactly as the client sent it.
	out.URL.RawQuery = pr.In.URL.RawQuery
	// ReverseProxy strips inbound forwarding headers before Rewrite: as the edge proxy, the client's
	// own X-Forwarded-For is discarded and replaced by the bare client IP.
	pr.SetXForwarded()
	out.Header.Set("X-Forwarded-Proto", "https")
	out.Header.Set("X-Forwarded-Host", state.hostname)
	// out.Host is the client's Host header, which upstreams that build URLs from it (Zitadel's
	// Console) or validate it (Vite's allowedHosts) rely on; ReverseProxy dials out.URL.Host.

	// Go canonicalises Sec-WebSocket-* to Sec-Websocket-*; case-sensitive upstream WebSocket servers
	// reject the handshake otherwise, and ReverseProxy writes these keys verbatim.
	restoreWebSocketHeaderCase(out.Header)
}

func (h *Handler) modifyResponse(resp *http.Response) error {
	// A 101 body is the upgraded connection, which ReverseProxy needs as an io.ReadWriteCloser.
	if resp.StatusCode == http.StatusSwitchingProtocols || resp.Body == nil || resp.Request == nil {
		return nil
	}
	if state := stateFrom(resp.Request.Context()); state != nil {
		resp.Body = &upstreamBody{ReadCloser: resp.Body, failed: &state.upstreamFailed}
		h.streams.enter(state)
	}
	return nil
}

func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	state := stateFrom(r.Context())
	f, clientGone := classify(r, err)
	if clientGone {
		state.clientGone = true
		return
	}
	logger.Error("Proxy error: "+state.hostname+r.URL.Path+" -> "+state.target, err)
	if aw, ok := w.(*accessWriter); ok && (aw.hijacked || aw.wroteHeader) {
		return
	}
	writeFailure(w, r, state.hostname, f)
}

// hostOnly drops the port from a Host header and the brackets from an IPv6 literal.
func hostOnly(hostport string) string {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	return strings.TrimSuffix(strings.TrimPrefix(hostport, "["), "]")
}

// rewritePath returns the upstream path and its escaped form. When the route strips a prefix, the
// same prefix is cut from the escaped path, so an encoded "/" (%2F) stays encoded upstream.
func rewritePath(in *url.URL, prefix, rewritten string, strip bool) (string, string) {
	if in.RawPath == "" {
		return rewritten, ""
	}
	if !strip {
		return rewritten, in.RawPath
	}
	raw := in.RawPath
	decoded, index := 0, 0
	for index < len(raw) && decoded < len(prefix) {
		if raw[index] == '%' && index+2 < len(raw) {
			index += 3
		} else {
			index++
		}
		decoded++
	}
	if unescaped, err := url.PathUnescape(raw[:index]); err != nil || unescaped != prefix {
		return rewritten, ""
	}
	rest := raw[index:]
	if rest == "" {
		rest = "/"
	}
	// url.URL ignores a RawPath that is not a valid encoding of Path, so a mismatch falls back safely.
	return rewritten, rest
}

// copyBufferSize matches ReverseProxy's own buffer size.
const copyBufferSize = 32 * 1024

// bufferPool reuses ReverseProxy's copy buffers across responses.
type bufferPool struct{ pool sync.Pool }

func newBufferPool() *bufferPool {
	return &bufferPool{pool: sync.Pool{New: func() any {
		buffer := make([]byte, copyBufferSize)
		return &buffer
	}}}
}

func (p *bufferPool) Get() []byte { return *p.pool.Get().(*[]byte) }

func (p *bufferPool) Put(buffer []byte) {
	if cap(buffer) < copyBufferSize {
		return
	}
	buffer = buffer[:copyBufferSize]
	p.pool.Put(&buffer)
}

// wsHeaderCanonicalToRFC maps Go's canonical header keys to the exact RFC 6455 casing.
// Go's http.Header canonicalises "Sec-WebSocket-Key" to "Sec-Websocket-Key"; some upstream
// WebSocket servers look these headers up case-sensitively and reject the handshake when the
// case differs. We restore the RFC casing on the way out so the upstream accepts the upgrade.
var wsHeaderCanonicalToRFC = map[string]string{
	"Sec-Websocket-Key":        "Sec-WebSocket-Key",
	"Sec-Websocket-Version":    "Sec-WebSocket-Version",
	"Sec-Websocket-Extensions": "Sec-WebSocket-Extensions",
	"Sec-Websocket-Protocol":   "Sec-WebSocket-Protocol",
	"Sec-Websocket-Accept":     "Sec-WebSocket-Accept",
}

// restoreWebSocketHeaderCase rewrites canonicalised Sec-Websocket-* header keys back to their
// RFC 6455 casing in place, preserving values.
func restoreWebSocketHeaderCase(h http.Header) {
	for canonical, rfc := range wsHeaderCanonicalToRFC {
		if values, ok := h[canonical]; ok {
			delete(h, canonical)
			h[rfc] = values
		}
	}
}
