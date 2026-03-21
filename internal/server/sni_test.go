package server

import (
	"testing"
)

// buildClientHello creates a minimal TLS ClientHello with the given SNI hostname.
func buildClientHello(hostname string) []byte {
	hostBytes := []byte(hostname)
	hostLen := len(hostBytes)

	// SNI extension data:
	// server_name_list_length(2) + server_name_type(1) + host_name_length(2) + hostname
	sniExtData := make([]byte, 0, 5+hostLen)
	sniListLen := 3 + hostLen // type(1) + len(2) + hostname
	sniExtData = append(sniExtData, byte(sniListLen>>8), byte(sniListLen))
	sniExtData = append(sniExtData, 0x00) // host_name type
	sniExtData = append(sniExtData, byte(hostLen>>8), byte(hostLen))
	sniExtData = append(sniExtData, hostBytes...)

	// Extension: type(2) + length(2) + data
	ext := make([]byte, 0, 4+len(sniExtData))
	ext = append(ext, 0x00, 0x00) // SNI extension type
	ext = append(ext, byte(len(sniExtData)>>8), byte(len(sniExtData)))
	ext = append(ext, sniExtData...)

	// Extensions total length(2) + extensions
	extensions := make([]byte, 0, 2+len(ext))
	extensions = append(extensions, byte(len(ext)>>8), byte(len(ext)))
	extensions = append(extensions, ext...)

	// ClientHello body: version(2) + random(32) + session_id_len(1) + cipher_suites_len(2) + cipher(2) + compression_len(1) + compression(1) + extensions
	clientHelloBody := make([]byte, 0, 2+32+1+2+2+1+1+len(extensions))
	clientHelloBody = append(clientHelloBody, 0x03, 0x03) // TLS 1.2
	clientHelloBody = append(clientHelloBody, make([]byte, 32)...) // random
	clientHelloBody = append(clientHelloBody, 0x00) // session_id_len = 0
	clientHelloBody = append(clientHelloBody, 0x00, 0x02) // cipher_suites_len = 2
	clientHelloBody = append(clientHelloBody, 0x00, 0x2f) // TLS_RSA_WITH_AES_128_CBC_SHA
	clientHelloBody = append(clientHelloBody, 0x01) // compression_methods_len = 1
	clientHelloBody = append(clientHelloBody, 0x00) // null compression
	clientHelloBody = append(clientHelloBody, extensions...)

	// Handshake: type(1) + length(3) + ClientHello body
	handshake := make([]byte, 0, 4+len(clientHelloBody))
	handshake = append(handshake, 0x01) // ClientHello
	bodyLen := len(clientHelloBody)
	handshake = append(handshake, byte(bodyLen>>16), byte(bodyLen>>8), byte(bodyLen))
	handshake = append(handshake, clientHelloBody...)

	// TLS record: type(1) + version(2) + length(2) + handshake
	record := make([]byte, 0, 5+len(handshake))
	record = append(record, 0x16) // Handshake
	record = append(record, 0x03, 0x01) // TLS 1.0
	hsLen := len(handshake)
	record = append(record, byte(hsLen>>8), byte(hsLen))
	record = append(record, handshake...)

	return record
}

func TestParseSNI(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "valid hostname",
			input:    buildClientHello("app.lvh.me"),
			expected: "app.lvh.me",
		},
		{
			name:     "long hostname",
			input:    buildClientHello("very-long-subdomain.example.example-local.com"),
			expected: "very-long-subdomain.example.example-local.com",
		},
		{
			name:     "empty buffer",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "not TLS",
			input:    []byte{0x47, 0x45, 0x54, 0x20, 0x2f}, // "GET /"
			expected: "",
		},
		{
			name:     "truncated record",
			input:    []byte{0x16, 0x03, 0x01},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSNI(tt.input)
			if result != tt.expected {
				t.Errorf("ParseSNI() = %q, want %q", result, tt.expected)
			}
		})
	}
}
