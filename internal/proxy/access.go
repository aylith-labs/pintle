package proxy

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"sync/atomic"
)

// accessWriter records the status and body size actually sent. It forwards Flush and Hijack and
// unwraps for http.ResponseController, so streaming and protocol upgrades pass through unchanged.
type accessWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	hijacked    bool
	bytes       int64
	writeErr    error
}

func newAccessWriter(w http.ResponseWriter) *accessWriter {
	return &accessWriter{ResponseWriter: w}
}

func (w *accessWriter) WriteHeader(code int) {
	// 1xx other than 101 are informational; the final status follows.
	if !w.wroteHeader && (code >= 200 || code == http.StatusSwitchingProtocols) {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *accessWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.status = http.StatusOK
		w.wroteHeader = true
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	if err != nil && w.writeErr == nil {
		w.writeErr = err
	}
	return n, err
}

func (w *accessWriter) Flush() {
	_ = w.FlushError()
}

func (w *accessWriter) FlushError() error {
	if !w.wroteHeader {
		w.status = http.StatusOK
		w.wroteHeader = true
	}
	err := http.NewResponseController(w.ResponseWriter).Flush()
	if err != nil && !errors.Is(err, http.ErrNotSupported) && w.writeErr == nil {
		w.writeErr = err
	}
	return err
}

func (w *accessWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		// ReverseProxy hijacks only to complete a 101, which it writes to the conn directly.
		w.hijacked = true
		w.status = http.StatusSwitchingProtocols
		w.wroteHeader = true
	}
	return conn, rw, err
}

func (w *accessWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// finalStatus is the status the client received; a handler that wrote nothing sent 200.
func (w *accessWriter) finalStatus() int {
	if !w.wroteHeader {
		return http.StatusOK
	}
	return w.status
}

// upstreamBody notes a read error on the upstream response body, which means the response the
// client got was cut short.
type upstreamBody struct {
	io.ReadCloser
	failed *atomic.Bool
}

func (b *upstreamBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && err != io.EOF {
		b.failed.Store(true)
	}
	return n, err
}
