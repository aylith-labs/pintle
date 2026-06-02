package server

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
)

// ParseSNI extracts the server name from a TLS ClientHello message.
func ParseSNI(buf []byte) string {
	// TLS record: type(1) + version(2) + length(2) + handshake
	if len(buf) < 5 || buf[0] != 0x16 { // 0x16 = Handshake
		return ""
	}

	recordLength := int(buf[3])<<8 | int(buf[4])
	if len(buf) < 5+recordLength {
		return ""
	}

	// Handshake: type(1) + length(3) + ...
	pos := 5
	if buf[pos] != 0x01 { // 0x01 = ClientHello
		return ""
	}
	pos += 4 // Skip handshake type + length

	// ClientHello: version(2) + random(32) + session_id_len(1) + session_id + ...
	pos += 2 + 32 // version + random

	if pos >= len(buf) {
		return ""
	}
	sessionIDLen := int(buf[pos])
	pos += 1 + sessionIDLen

	// Cipher suites: length(2) + data
	if pos+2 > len(buf) {
		return ""
	}
	cipherSuitesLen := int(buf[pos])<<8 | int(buf[pos+1])
	pos += 2 + cipherSuitesLen

	// Compression methods: length(1) + data
	if pos >= len(buf) {
		return ""
	}
	compressionLen := int(buf[pos])
	pos += 1 + compressionLen

	// Extensions: total_length(2) + extension*
	if pos+2 > len(buf) {
		return ""
	}
	extensionsLen := int(buf[pos])<<8 | int(buf[pos+1])
	pos += 2

	extensionsEnd := pos + extensionsLen

	for pos+4 <= extensionsEnd && pos+4 <= len(buf) {
		extType := int(buf[pos])<<8 | int(buf[pos+1])
		extLen := int(buf[pos+2])<<8 | int(buf[pos+3])
		pos += 4

		if extType == 0x0000 { // SNI extension
			if pos+5 > len(buf) {
				return ""
			}
			nameType := buf[pos+2]
			if nameType != 0x00 { // 0x00 = host_name
				return ""
			}
			nameLen := int(buf[pos+3])<<8 | int(buf[pos+4])
			if pos+5+nameLen > len(buf) {
				return ""
			}
			return string(buf[pos+5 : pos+5+nameLen])
		}

		pos += extLen
	}

	return ""
}

type SNIForwardTarget struct {
	Match   func(hostname string) bool
	Resolve func() *net.TCPAddr
	Label   string
}

type SNIRouter struct {
	Port           int
	LocalTarget    *net.TCPAddr
	ForwardTargets []SNIForwardTarget
	// HasLocalRoute reports whether an explicit local route exists for a hostname.
	// When set, explicit routes win over passthrough domains.
	HasLocalRoute func(hostname string) bool
}

func pipeToTarget(client net.Conn, initialData []byte, addr *net.TCPAddr) {
	upstream, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		client.Close()
		return
	}

	// Write buffered ClientHello
	if _, err := upstream.Write(initialData); err != nil {
		upstream.Close()
		client.Close()
		return
	}

	// Bidirectional pipe
	go func() {
		io.Copy(upstream, client)
		upstream.Close()
	}()
	go func() {
		io.Copy(client, upstream)
		client.Close()
	}()
}

func (s *SNIRouter) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", s.Port))
	if err != nil {
		return fmt.Errorf("SNI router listen: %w", err)
	}

	logger.Infof("SNI router listening on :%d", s.Port)
	for _, target := range s.ForwardTargets {
		logger.Infof("  %s (passthrough, dynamic IP)", target.Label)
	}
	logger.Infof("  *.%s -> %s (local TLS)", "lvh.me", s.LocalTarget.String())

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				logger.Errorf("SNI accept error: %v", err)
				continue
			}
		}
		go s.handleConnection(conn)
	}
}

func (s *SNIRouter) handleConnection(client net.Conn) {
	buf := make([]byte, 16384)
	n, err := client.Read(buf)
	if err != nil {
		client.Close()
		return
	}
	data := buf[:n]

	sni := ParseSNI(data)

	// Find forward target
	if sni != "" {
		// Explicit local routes take precedence over passthrough domains, so a
		// host with its own route is served locally even when its parent domain
		// is otherwise passed through to another proxy.
		if s.HasLocalRoute != nil && s.HasLocalRoute(sni) {
			pipeToTarget(client, data, s.LocalTarget)
			return
		}

		for _, ft := range s.ForwardTargets {
			if ft.Match(sni) {
				target := ft.Resolve()
				if target != nil {
					pipeToTarget(client, data, target)
					return
				}
				// Target unavailable, fall back to local
				break
			}
		}
	}

	// Forward to local HTTPS server
	pipeToTarget(client, data, s.LocalTarget)
}
