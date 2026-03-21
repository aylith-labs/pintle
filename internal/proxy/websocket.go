package proxy

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func IsWebSocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func ProxyWebSocket(w http.ResponseWriter, r *http.Request, targetURL string) {
	// Dial upstream
	dialer := websocket.Dialer{}
	upstreamHeaders := http.Header{}
	for k, v := range r.Header {
		switch strings.ToLower(k) {
		case "upgrade", "connection", "sec-websocket-key", "sec-websocket-version", "sec-websocket-extensions", "sec-websocket-protocol":
			continue
		default:
			upstreamHeaders[k] = v
		}
	}

	upstream, _, err := dialer.Dial(targetURL, upstreamHeaders)
	if err != nil {
		logger.Errorf("WebSocket dial failed: %s: %v", targetURL, err)
		http.Error(w, "WebSocket upstream unreachable", http.StatusBadGateway)
		return
	}

	// Upgrade client
	client, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Errorf("WebSocket upgrade failed: %v", err)
		upstream.Close()
		return
	}

	// Bidirectional pipe
	done := make(chan struct{})

	// upstream -> client
	go func() {
		defer close(done)
		for {
			msgType, msg, err := upstream.ReadMessage()
			if err != nil {
				return
			}
			if err := client.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	}()

	// client -> upstream
	go func() {
		for {
			msgType, msg, err := client.ReadMessage()
			if err != nil {
				upstream.Close()
				return
			}
			if err := upstream.WriteMessage(msgType, msg); err != nil {
				return
			}
		}
	}()

	<-done
	client.Close()
}
