package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
)

type TCPCert struct {
	Cert   []byte
	Key    []byte
	Domain string
}

type TCPRouter struct {
	mu        sync.RWMutex
	ports     map[int]net.Listener
	certs     []TCPCert
	getRoutes func() []provider.TcpRoute
}

func NewTCPRouter(certs []TCPCert, getRoutes func() []provider.TcpRoute) *TCPRouter {
	return &TCPRouter{
		ports:     make(map[int]net.Listener),
		certs:     certs,
		getRoutes: getRoutes,
	}
}

func (t *TCPRouter) UpdateCerts(certs []TCPCert) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.certs = certs
}

func (t *TCPRouter) StartPort(ctx context.Context, port int) error {
	t.mu.Lock()
	if _, exists := t.ports[port]; exists {
		t.mu.Unlock()
		return nil // already listening
	}
	t.mu.Unlock()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("TCP router listen on :%d: %w", port, err)
	}

	t.mu.Lock()
	t.ports[port] = listener
	t.mu.Unlock()

	logger.Infof("TCP router listening on :%d", port)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			go t.handleConnection(conn, port)
		}
	}()

	return nil
}

func (t *TCPRouter) findCert(hostname string) *TCPCert {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for i := range t.certs {
		if strings.HasSuffix(hostname, "."+t.certs[i].Domain) || hostname == t.certs[i].Domain {
			return &t.certs[i]
		}
	}
	return nil
}

func (t *TCPRouter) handleConnection(client net.Conn, port int) {
	buf := make([]byte, 16384)
	n, err := client.Read(buf)
	if err != nil {
		client.Close()
		return
	}
	data := buf[:n]

	sni := ParseSNI(data)
	if sni == "" {
		logger.Warnf("TCP[:%d] No SNI in ClientHello", port)
		client.Close()
		return
	}

	// Find route
	routes := t.getRoutes()
	var route *provider.TcpRoute
	for i := range routes {
		if routes[i].ListenPort == port && routes[i].Hostname == sni {
			route = &routes[i]
			break
		}
	}

	if route == nil {
		logger.Warnf("TCP[:%d] No route for %s", port, sni)
		client.Close()
		return
	}

	cert := t.findCert(sni)
	if cert == nil {
		logger.Warnf("TCP[:%d] No cert for %s", port, sni)
		client.Close()
		return
	}

	// Build TLS config
	tlsCert, err := tls.X509KeyPair(cert.Cert, cert.Key)
	if err != nil {
		logger.Errorf("TCP[:%d] Failed to load cert for %s: %v", port, sni, err)
		client.Close()
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	// Prepend buffered data by creating a combined connection
	prefixConn := &prefixedConn{Conn: client, prefix: data}

	tlsConn := tls.Server(prefixConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		logger.Warnf("TCP[:%d] TLS handshake failed for %s: %v", port, sni, err)
		client.Close()
		return
	}

	// Connect to upstream
	upstream, err := net.Dial("tcp", fmt.Sprintf("%s:%d", route.TargetHost, route.TargetPort))
	if err != nil {
		logger.Errorf("TCP[:%d] Failed to connect to %s:%d: %v", port, route.TargetHost, route.TargetPort, err)
		tlsConn.Close()
		return
	}

	logger.Infof("TCP[:%d] %s -> %s:%d", port, sni, route.TargetHost, route.TargetPort)

	go func() {
		io.Copy(upstream, tlsConn)
		upstream.Close()
	}()
	go func() {
		io.Copy(tlsConn, upstream)
		tlsConn.Close()
	}()
}

// prefixedConn wraps a net.Conn and prepends buffered data to the first Read.
type prefixedConn struct {
	net.Conn
	prefix []byte
	offset int
}

func (c *prefixedConn) Read(b []byte) (int, error) {
	if c.offset < len(c.prefix) {
		n := copy(b, c.prefix[c.offset:])
		c.offset += n
		return n, nil
	}
	return c.Conn.Read(b)
}
