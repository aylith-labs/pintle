package proxy

import (
	"net/http"
	"testing"
)

// TestRestoreWebSocketHeaderCase verifies that Go's canonicalised Sec-Websocket-* header
// keys are rewritten back to their RFC 6455 casing, which case-sensitive upstream WebSocket
// servers require to accept the handshake.
func TestRestoreWebSocketHeaderCase(t *testing.T) {
	header := http.Header{}
	header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==") // stored canonical: Sec-Websocket-Key
	header.Set("Sec-WebSocket-Version", "13")
	header.Set("Sec-WebSocket-Extensions", "permessage-deflate")
	header.Set("Content-Type", "text/plain")

	// Sanity: Go canonicalised the keys, losing the RFC casing.
	if _, ok := header["Sec-WebSocket-Key"]; ok {
		t.Fatalf("expected Go to canonicalise the header key before restore")
	}

	restoreWebSocketHeaderCase(header)

	for _, rfcKey := range []string{"Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Extensions"} {
		if _, ok := header[rfcKey]; !ok {
			t.Errorf("expected RFC-cased key %q to be present, header=%v", rfcKey, header)
		}
	}
	for _, canonicalKey := range []string{"Sec-Websocket-Key", "Sec-Websocket-Version", "Sec-Websocket-Extensions"} {
		if _, ok := header[canonicalKey]; ok {
			t.Errorf("expected canonical key %q to be removed, header=%v", canonicalKey, header)
		}
	}
	if got := header["Sec-WebSocket-Key"]; len(got) != 1 || got[0] != "dGhlIHNhbXBsZSBub25jZQ==" {
		t.Errorf("value not preserved for Sec-WebSocket-Key: %v", got)
	}
	if got := header.Get("Content-Type"); got != "text/plain" {
		t.Errorf("unrelated header was modified: %q", got)
	}
}
