package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
)

// StatusClientClosedRequest records a request whose client went away before a response was written.
const StatusClientClosedRequest = 499

// retryAfterSeconds is the hint sent with every 503.
const retryAfterSeconds = 2

// failure is what the proxy answers for one class of upstream error.
type failure struct {
	status int
	code   string // the JSON "error" value
	title  string
	detail string
}

var (
	failureTimeout = failure{
		status: http.StatusGatewayTimeout, code: "upstream_timeout",
		title:  "Service is not responding",
		detail: "The service took too long to answer.",
	}
	failureUnavailable = failure{
		status: http.StatusServiceUnavailable, code: "upstream_unavailable",
		title:  "Service unavailable",
		detail: "The service is restarting or not running.",
	}
	failureNoHealthyBackend = failure{
		status: http.StatusServiceUnavailable, code: "no_healthy_backend",
		title:  "Service unavailable",
		detail: "No copy of this service is ready to answer right now.",
	}
	failureBadGateway = failure{
		status: http.StatusBadGateway, code: "upstream_error",
		title:  "Service error",
		detail: "The service could not be reached or sent an answer that could not be used.",
	}
)

// classify maps a transport error to the answer for it. A gone client is reported separately
// because nothing is written for it.
func classify(r *http.Request, err error) (f failure, clientGone bool) {
	if r.Context().Err() != nil {
		return failure{}, true
	}
	if isDialError(err) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return failureUnavailable, false
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return failureTimeout, false
	}
	return failureBadGateway, false
}

// writeFailure answers in the caller's format and never names the upstream address.
func writeFailure(w http.ResponseWriter, r *http.Request, hostname string, f failure) {
	header := w.Header()
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	retry := f.status == http.StatusServiceUnavailable
	if retry {
		header.Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	}
	accept := r.Header.Get("Accept")
	switch {
	case strings.Contains(accept, "application/json") || strings.HasPrefix(r.URL.Path, "/api/"):
		header.Set("Content-Type", "application/json")
		w.WriteHeader(f.status)
		body := map[string]any{"error": f.code}
		if retry {
			body["retryAfter"] = retryAfterSeconds
		}
		_ = json.NewEncoder(w).Encode(body)
	case strings.Contains(accept, "text/html"):
		header.Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(failurePage(r, hostname, f)))
	default:
		header.Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.title + ". " + f.detail + " Try again in a few seconds.\n"))
	}
}

// pageRefreshSeconds is how often the browser error page reloads itself.
const pageRefreshSeconds = 5

// failurePage is self-contained and follows the system light or dark theme. It reloads itself only
// for GET and HEAD, so a form submission is never resent.
func failurePage(r *http.Request, hostname string, f failure) string {
	refresh, again := "", "Reload the page to try again."
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		refresh = `<meta http-equiv="refresh" content="` + strconv.Itoa(pageRefreshSeconds) + `">`
		again = "This page will try again by itself in a few seconds."
	}
	var page strings.Builder
	page.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8">`)
	page.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	page.WriteString(refresh)
	page.WriteString(`<title>` + html.EscapeString(f.title) + `</title><style>`)
	page.WriteString(`:root{color-scheme:light dark;--bg:#faf9f7;--fg:#1f1d1a;--muted:#6b665e;--card:#fff;--line:#e6e2db}`)
	page.WriteString(`@media (prefers-color-scheme:dark){:root{--bg:#1a1917;--fg:#ece9e4;--muted:#a39d93;--card:#24221f;--line:#3a3733}}`)
	page.WriteString(`body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:var(--bg);color:var(--fg);font:16px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;padding:16px;box-sizing:border-box}`)
	page.WriteString(`main{max-width:28rem;background:var(--card);border:1px solid var(--line);border-radius:12px;padding:24px}`)
	page.WriteString(`h1{font-size:1.25rem;margin:0 0 8px}p{margin:0 0 8px}.muted{color:var(--muted);font-size:.875rem;margin:0}`)
	page.WriteString(`</style></head><body><main>`)
	page.WriteString(`<h1>` + html.EscapeString(f.title) + `</h1>`)
	if hostname != "" {
		page.WriteString(`<p><strong>` + html.EscapeString(hostname) + `</strong></p>`)
	}
	page.WriteString(`<p>` + html.EscapeString(f.detail) + `</p>`)
	page.WriteString(`<p class="muted">` + html.EscapeString(again) + `</p>`)
	page.WriteString(`</main></body></html>`)
	return page.String()
}
