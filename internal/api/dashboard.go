package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
)

// NewDashboardHandler returns a handler that either serves the embedded UI
// or proxies to a Vite dev server.
func NewDashboardHandler(viteDevURL string) http.Handler {
	if viteDevURL != "" {
		target, err := url.Parse(viteDevURL)
		if err != nil {
			logger.Errorf("Invalid VITE_DEV_URL: %v", err)
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Invalid VITE_DEV_URL", http.StatusInternalServerError)
			})
		}
		logger.Infof("Dashboard proxying to Vite dev server: %s", viteDevURL)
		return httputil.NewSingleHostReverseProxy(target)
	}

	// Production: serve embedded static files
	logger.Info("Dashboard serving embedded UI")
	return getEmbeddedHandler()
}
