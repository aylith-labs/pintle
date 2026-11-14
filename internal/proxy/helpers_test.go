package proxy

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aylith-labs/pintle/internal/logger"
	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/router"
	"github.com/aylith-labs/pintle/internal/stats"
)

const testHost = "app.lvh.me"

// fastSettings keep failure tests quick; each test overrides the one value it exercises.
func fastSettings() transportSettings {
	return transportSettings{dialTimeout: time.Second, responseHeaderTimeout: 5 * time.Second, retryBudget: 150 * time.Millisecond}
}

func newTestHandler(t *testing.T, settings transportSettings, routes ...provider.Route) *Handler {
	t.Helper()
	r := router.New()
	r.Update(routes)
	return newHandler(r, stats.NewCollector(""), settings)
}

func routeTo(target string) provider.Route {
	return provider.Route{Hostname: testHost, Path: "/", Target: target, Source: "test"}
}

// frontServer runs the handler under a real http.Server, which is what supplies Hijack, Flush and
// the ErrAbortHandler recovery a production request gets.
func frontServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	return front
}

func sendThrough(t *testing.T, front *httptest.Server, method, path, body string, header http.Header) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, front.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = testHost
	for key, values := range header {
		req.Header[key] = values
	}
	resp, err := front.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// reserveAddr returns a loopback address that nothing is listening on.
func reserveAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	return addr
}

// logCapture collects logger output for one test.
type logCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *logCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// text is the captured output with colour codes removed.
func (c *logCapture) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return ansiCodes.ReplaceAllString(c.buf.String(), "")
}

func captureLog(t *testing.T) *logCapture {
	t.Helper()
	capture := &logCapture{}
	t.Cleanup(logger.SetOutput(capture))
	return capture
}

// waitFor polls until done reports true or the deadline passes.
func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
