package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/aylith-labs/pintle/internal/logger"
)

func StartHTTPRedirect(ctx context.Context, port int) error {
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.Path
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			// Strip port from redirect URL (iptables handles 443 -> LISTEN_PORT)
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}),
	}

	go func() {
		<-ctx.Done()
		server.Close()
	}()

	logger.Infof("HTTP redirect on :%d", port)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("HTTP redirect server error: %v", err)
		}
	}()

	return nil
}

// StartLoopbackHTTPRedirect binds synchronously so address conflicts fail
// startup. The legacy redirect above retains its port-mapping behavior.
func StartLoopbackHTTPRedirect(ctx context.Context, port int, hostname string, httpsPort int) error {
	ip := net.ParseIP(hostname)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("HTTP redirect requires a literal loopback IP")
	}
	addr := net.JoinHostPort(hostname, strconv.Itoa(port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("HTTP redirect listen: %w", err)
	}
	server := &http.Server{Addr: addr, Handler: loopbackRedirectHandler(httpsPort)}
	go func() {
		<-ctx.Done()
		server.Close()
	}()
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Errorf("HTTP redirect server error: %v", err)
		}
	}()
	logger.Infof("HTTP redirect on %s", addr)
	return nil
}

func loopbackRedirectHandler(httpsPort int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if name, _, err := net.SplitHostPort(host); err == nil {
			host = name
		}
		host = strings.Trim(host, "[]")
		if httpsPort != 443 {
			host = net.JoinHostPort(host, strconv.Itoa(httpsPort))
		} else if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		target := "https://" + host + r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}
