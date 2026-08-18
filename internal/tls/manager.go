package tls

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/aylith-labs/pintle/internal/logger"
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

// discoverDomains returns every domain with a "<domain>.pem" + "<domain>-key.pem" pair in
// certsDir. The certs directory is the source of truth: a cert is served because it is present,
// not because some other config section happens to name its domain.
func discoverDomains(certsDir string) []string {
	entries, err := os.ReadDir(certsDir)
	if err != nil {
		logger.Errorf("Failed to read certs dir %s: %v", certsDir, err)
		return nil
	}

	var domains []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, "-key.pem") {
			continue
		}
		domain := strings.TrimSuffix(name, ".pem")
		if _, err := os.Stat(filepath.Join(certsDir, domain+"-key.pem")); err != nil {
			logger.Warnf("Cert %s has no matching %s-key.pem, skipping", name, domain)
			continue
		}
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	return domains
}

func (m *Manager) LoadCerts(certsDir, baseDomain string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, domain := range discoverDomains(certsDir) {
		cert, err := tls.LoadX509KeyPair(
			filepath.Join(certsDir, domain+".pem"),
			filepath.Join(certsDir, domain+"-key.pem"),
		)
		if err != nil {
			logger.Errorf("Failed to load cert for *.%s: %v", domain, err)
			continue
		}
		if _, seen := m.certs[domain]; !seen {
			logger.Infof("Loaded cert for *.%s", domain)
		}
		m.certs[domain] = cert
	}

	if _, ok := m.certs[baseDomain]; !ok {
		logger.Errorf("No cert for the base domain *.%s (expected %s/%s.pem) — run: mkcert -cert-file %s/%s.pem -key-file %s/%s-key.pem \"*.%s\"",
			baseDomain, certsDir, baseDomain, certsDir, baseDomain, certsDir, baseDomain, baseDomain)
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

func (m *Manager) GetRawCerts(certsDir, baseDomain string) []RawCert {
	var certs []RawCert
	for _, domain := range discoverDomains(certsDir) {
		certData, err := os.ReadFile(filepath.Join(certsDir, domain+".pem"))
		if err != nil {
			continue
		}
		keyData, err := os.ReadFile(filepath.Join(certsDir, domain+"-key.pem"))
		if err != nil {
			continue
		}
		certs = append(certs, RawCert{Cert: certData, Key: keyData, Domain: domain})
	}
	return certs
}
