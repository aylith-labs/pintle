package proxy

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
)

// TestClassifyConnectionLost: a refused, reset or aborted connection is "unavailable" whether the
// error carries the POSIX code or the Winsock one a Windows build sees.
func TestClassifyConnectionLost(t *testing.T) {
	cases := []struct {
		name  string
		errno syscall.Errno
	}{
		{"posix reset", syscall.ECONNRESET},
		{"posix refused", syscall.ECONNREFUSED},
		{"posix aborted", syscall.ECONNABORTED},
		{"posix broken pipe", syscall.EPIPE},
		{"winsock reset", wsaECONNRESET},
		{"winsock refused", wsaECONNREFUSED},
		{"winsock aborted", wsaECONNABORTED},
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.internal/", nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := &net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("wsarecv", tc.errno)}
			if f, gone := classify(req, err); gone || f.status != http.StatusServiceUnavailable {
				t.Fatalf("status %d gone %v; want 503", f.status, gone)
			}
		})
	}
	t.Run("an unrelated error stays a 502", func(t *testing.T) {
		if f, _ := classify(req, errors.New("malformed response")); f.status != http.StatusBadGateway {
			t.Fatalf("status %d; want 502", f.status)
		}
	})
}
