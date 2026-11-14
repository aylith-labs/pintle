package server

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/aylith-labs/pintle/internal/logger"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownGrace     = 3 * time.Second // the listener stays closed this long, so every site refuses connections for it
)

// draining counts HTTP listeners still shutting down; WaitForShutdown blocks on it.
var draining sync.WaitGroup

// newHTTPServer sets the limits every listener shares. ReadTimeout and WriteTimeout
// stay unset on purpose: they would cut Server-Sent-Events streams, WebSockets and
// large bodies that the proxy carries.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          log.New(&serverLogWriter{emit: func(line string) { logger.Warnf("%s", line) }}, "", 0),
	}
}

// serverLogWriter forwards net/http's error log to the project logger, minus the
// per-connection "TLS handshake error" lines that every port scan and plain-HTTP
// client produces. Recovered handler panics, HTTP/2 errors and accept errors pass.
type serverLogWriter struct {
	emit func(line string)
}

func (w *serverLogWriter) Write(p []byte) (int, error) {
	line := bytes.TrimSpace(p)
	if len(line) > 0 && !bytes.Contains(line, []byte("TLS handshake error")) {
		w.emit(string(line))
	}
	return len(p), nil
}

// shutdownOnDone drains the server when ctx ends: in-flight requests get the grace
// period to finish, then whatever is left is closed. Shutdown does not wait for
// hijacked connections such as WebSockets; they end when the process exits.
func shutdownOnDone(ctx context.Context, srv *http.Server, grace time.Duration) {
	draining.Add(1)
	go func() {
		defer draining.Done()
		<-ctx.Done()
		drainCtx, cancel := context.WithTimeout(context.Background(), grace)
		defer cancel()
		if err := srv.Shutdown(drainCtx); err != nil {
			logger.Warnf("Shutdown of %s did not finish in %s, closing: %v", srv.Addr, grace, err)
			srv.Close()
		}
	}()
}

// WaitForShutdown blocks until every HTTP listener started by this package has
// drained. Call it after the context passed to the Start functions is cancelled.
func WaitForShutdown() {
	draining.Wait()
}
