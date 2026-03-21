package tls

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
)

type Manager struct {
	mu    sync.RWMutex
	certs map[string]tls.Certificate // domain -> cert (e.g., "lvh.me" -> *.lvh.me)
}

func NewManager() *Manager {
	return &Manager{
		certs: make(map[string]tls.Certificate),
	}
}

func (m *Manager) LoadCerts(certsDir, baseDomain string, passthrough []provider.PassthroughDomain) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load base domain cert
	certPath := filepath.Join(certsDir, baseDomain+".pem")
	keyPath := filepath.Join(certsDir, baseDomain+"-key.pem")

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		logger.Errorf("Failed to load cert for *.%s: %v", baseDomain, err)
	} else {
		m.certs[baseDomain] = cert
	}

	// Load passthrough domain certs
	for _, pt := range passthrough {
		ptCertPath := filepath.Join(certsDir, pt.Domain+".pem")
		ptKeyPath := filepath.Join(certsDir, pt.Domain+"-key.pem")

		if _, err := os.Stat(ptCertPath); os.IsNotExist(err) {
			logger.Warnf("Passthrough cert missing for *.%s (run: mkcert -cert-file certs/%s.pem -key-file certs/%s-key.pem \"*.%s\")",
				pt.Domain, pt.Domain, pt.Domain, pt.Domain)
			continue
		}

		ptCert, err := tls.LoadX509KeyPair(ptCertPath, ptKeyPath)
		if err != nil {
			logger.Errorf("Failed to load cert for *.%s: %v", pt.Domain, err)
			continue
		}
		m.certs[pt.Domain] = ptCert
	}
}

func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	serverName := hello.ServerName

	// Try to find a matching certificate by walking up domain parts
	// e.g., "app.lvh.me" -> try "lvh.me" (which holds *.lvh.me)
	parts := strings.Split(serverName, ".")
	for i := range parts {
		domain := strings.Join(parts[i:], ".")
		if cert, ok := m.certs[domain]; ok {
			return &cert, nil
		}
	}

	// Fallback: return first available cert
	for _, cert := range m.certs {
		c := cert
		return &c, nil
	}

	return nil, fmt.Errorf("no certificate found for %s", serverName)
}

// GetRawCerts returns raw cert/key bytes for TCP TLS termination.
type RawCert struct {
	Cert   []byte
	Key    []byte
	Domain string
}

func (m *Manager) GetRawCerts(certsDir, baseDomain string, passthrough []provider.PassthroughDomain) []RawCert {
	var certs []RawCert

	certPath := filepath.Join(certsDir, baseDomain+".pem")
	keyPath := filepath.Join(certsDir, baseDomain+"-key.pem")

	if certData, err := os.ReadFile(certPath); err == nil {
		if keyData, err := os.ReadFile(keyPath); err == nil {
			certs = append(certs, RawCert{Cert: certData, Key: keyData, Domain: baseDomain})
		}
	}

	for _, pt := range passthrough {
		ptCertPath := filepath.Join(certsDir, pt.Domain+".pem")
		ptKeyPath := filepath.Join(certsDir, pt.Domain+"-key.pem")

		if certData, err := os.ReadFile(ptCertPath); err == nil {
			if keyData, err := os.ReadFile(ptKeyPath); err == nil {
				certs = append(certs, RawCert{Cert: certData, Key: keyData, Domain: pt.Domain})
			}
		}
	}

	return certs
}
