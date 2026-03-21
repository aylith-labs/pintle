package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
	"github.com/steven-pribilinskiy/local-proxy/internal/router"
	"github.com/steven-pribilinskiy/local-proxy/internal/stats"
)

type Handler struct {
	router    *router.Router
	stats     *stats.Collector
	transport http.RoundTripper
}

func NewHandler(r *router.Router, s *stats.Collector) *Handler {
	return &Handler{
		router: r,
		stats:  s,
		transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true, // pass compressed responses through
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hostname := strings.Split(r.Host, ":")[0]
	path := r.URL.Path

	result := h.router.Resolve(hostname, path)
	if result == nil {
		logger.Route(r.Method, hostname, path, "no route", 404)
		h.stats.Record(stats.RequestRecord{
			Timestamp:  time.Now().UnixMilli(),
			Method:     r.Method,
			Hostname:   hostname,
			Path:       path,
			Target:     "none",
			Status:     404,
			DurationMs: 0,
		})
		http.Error(w, "No route for "+hostname, http.StatusNotFound)
		return
	}

	targetURL, err := url.Parse(result.Target)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadGateway)
		return
	}

	start := time.Now()

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = result.RewrittenPath
			// Preserve query string
			if r.URL.RawQuery != "" {
				req.URL.RawQuery = r.URL.RawQuery
			}
			req.Header.Set("X-Forwarded-For", r.RemoteAddr)
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Forwarded-Host", hostname)
			req.Host = targetURL.Host
		},
		Transport: h.transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			durationMs := float64(time.Since(start).Microseconds()) / 1000.0
			logger.Error("Proxy error: "+hostname+path+" -> "+result.Target, err)
			h.stats.Record(stats.RequestRecord{
				Timestamp:  time.Now().UnixMilli(),
				Method:     r.Method,
				Hostname:   hostname,
				Path:       path,
				Target:     result.Target,
				Status:     502,
				DurationMs: durationMs,
			})
			http.Error(w, "Upstream unreachable: "+result.Target, http.StatusBadGateway)
		},
		ModifyResponse: func(resp *http.Response) error {
			durationMs := float64(time.Since(start).Microseconds()) / 1000.0
			logger.Route(r.Method, hostname, path, result.Target, resp.StatusCode)
			h.stats.Record(stats.RequestRecord{
				Timestamp:  time.Now().UnixMilli(),
				Method:     r.Method,
				Hostname:   hostname,
				Path:       path,
				Target:     result.Target,
				Status:     resp.StatusCode,
				DurationMs: durationMs,
			})
			// Remove hop-by-hop headers
			resp.Header.Del("Transfer-Encoding")
			return nil
		},
	}

	proxy.ServeHTTP(w, r)
}
