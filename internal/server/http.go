package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
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
