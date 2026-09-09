package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/aylith-labs/pintle/internal/logger"
	tlsmgr "github.com/aylith-labs/pintle/internal/tls"
)

func StartHTTPS(ctx context.Context, port int, hostname string, tlsManager *tlsmgr.Manager, handler http.Handler) error {
	tlsConfig := &tls.Config{
		GetCertificate: tlsManager.GetCertificate,
		// Offer h2 via ALPN. net/http only serves HTTP/2 on Serve() when the
		// TLS config advertises it. WebSocket upgrades still arrive on separate
		// HTTP/1.1 connections (no extended CONNECT), so WS proxying is unaffected.
		NextProtos: []string{"h2", "http/1.1"},
	}

	addr := net.JoinHostPort(hostname, strconv.Itoa(port))
	server := &http.Server{
		Addr:      addr,
		Handler:   handler,
		TLSConfig: tlsConfig,
		ErrorLog:  log.New(io.Discard, "", 0), // suppress TLS handshake errors
	}

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("HTTPS listen: %w", err)
	}

	logger.Infof("HTTPS server on %s", addr)

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Errorf("HTTPS server error: %v", err)
		}
	}()

	return nil
}
