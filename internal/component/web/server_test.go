package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateWebCert verifies that GenerateWebCert produces valid PEM-encoded
// ECDSA P-256 certificate and key material suitable for TLS.
// VALIDATES: AC-9 (self-signed cert generated).
// PREVENTS: invalid or unparseable certificate material.
func TestGenerateWebCert(t *testing.T) {
	certPEM, keyPEM, err := GenerateWebCert()
	require.NoError(t, err)
	require.NotEmpty(t, certPEM, "certPEM must not be empty")
	require.NotEmpty(t, keyPEM, "keyPEM must not be empty")

	// Parse the certificate PEM block.
	certBlock, rest := pem.Decode(certPEM)
	require.NotNil(t, certBlock, "certPEM must contain a valid PEM block")
	assert.Equal(t, "CERTIFICATE", certBlock.Type)
	assert.Empty(t, rest, "certPEM must contain exactly one PEM block")

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err, "certificate must be valid X.509")

	// Verify ECDSA P-256 key type.
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	require.True(t, ok, "certificate public key must be ECDSA")
	assert.Equal(t, elliptic.P256(), pub.Curve, "certificate must use P-256 curve")

	// Verify SANs include localhost and loopback addresses.
	assert.Contains(t, cert.DNSNames, "localhost", "certificate must have localhost SAN")

	foundIPv4Loopback := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "127.0.0.1" {
			foundIPv4Loopback = true
		}
	}
	assert.True(t, foundIPv4Loopback, "certificate must have 127.0.0.1 SAN")

	// Verify key usage.
	assert.Equal(t, x509.KeyUsageDigitalSignature, cert.KeyUsage)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)

	// Parse the private key PEM block.
	keyBlock, rest := pem.Decode(keyPEM)
	require.NotNil(t, keyBlock, "keyPEM must contain a valid PEM block")
	assert.Equal(t, "EC PRIVATE KEY", keyBlock.Type)
	assert.Empty(t, rest, "keyPEM must contain exactly one PEM block")

	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	require.NoError(t, err, "private key must be valid ECDSA")
	assert.Equal(t, elliptic.P256(), key.Curve, "private key must use P-256 curve")
}

// TestGenerateWebCertWithAddr verifies that GenerateWebCertWithAddr adds the
// listen address as an extra SAN when it is a non-loopback IP.
// VALIDATES: AC-9 (cert includes listen address SAN).
// PREVENTS: TLS errors when accessing ze via non-loopback address.
func TestGenerateWebCertWithAddr(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		expectIP  string
		expectAdd bool // whether the IP should be added beyond the defaults
	}{
		{
			name:      "non-loopback IP added as SAN",
			addr:      "192.168.1.100:8443",
			expectIP:  "192.168.1.100",
			expectAdd: true,
		},
		{
			name:      "loopback IP not duplicated",
			addr:      "127.0.0.1:8443",
			expectIP:  "127.0.0.1",
			expectAdd: false, // already in defaults
		},
		{
			name:      "empty addr uses defaults only",
			addr:      "",
			expectIP:  "",
			expectAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certPEM, _, err := GenerateWebCertWithAddr(tt.addr)
			require.NoError(t, err)

			block, _ := pem.Decode(certPEM)
			require.NotNil(t, block)

			cert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			if tt.expectAdd {
				found := false
				for _, ip := range cert.IPAddresses {
					if ip.String() == tt.expectIP {
						found = true
						break
					}
				}
				assert.True(t, found, "certificate must include %s as SAN", tt.expectIP)
			}

			// Default SANs must always be present.
			assert.Contains(t, cert.DNSNames, "localhost")
		})
	}
}

// TestNewTLSConfig verifies that NewTLSConfig produces a valid tls.Config from
// generated PEM material with the expected minimum TLS version.
// VALIDATES: TLS works with generated cert.
// PREVENTS: misconfigured TLS settings, missing certificates.
func TestNewTLSConfig(t *testing.T) {
	certPEM, keyPEM, err := GenerateWebCert()
	require.NoError(t, err)

	tlsCfg, err := NewTLSConfig(certPEM, keyPEM)
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)

	// Must have exactly one certificate loaded.
	require.Len(t, tlsCfg.Certificates, 1, "TLS config must have one certificate")

	// Must enforce TLS 1.2 minimum.
	assert.Equal(t, uint16(tls.VersionTLS12), tlsCfg.MinVersion,
		"TLS config must enforce minimum TLS 1.2")
}

// TestNewTLSConfigInvalidPEM verifies that NewTLSConfig rejects invalid PEM data.
// PREVENTS: silent acceptance of corrupt certificate material.
func TestNewTLSConfigInvalidPEM(t *testing.T) {
	tests := []struct {
		name    string
		certPEM []byte
		keyPEM  []byte
	}{
		{
			name:    "empty cert",
			certPEM: []byte{},
			keyPEM:  []byte("not empty"),
		},
		{
			name:    "empty key",
			certPEM: []byte("not empty"),
			keyPEM:  []byte{},
		},
		{
			name:    "garbage data",
			certPEM: []byte("not a cert"),
			keyPEM:  []byte("not a key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTLSConfig(tt.certPEM, tt.keyPEM)
			assert.Error(t, err, "NewTLSConfig must reject invalid PEM data")
		})
	}
}

// TestNewWebServerRequiresFields verifies that NewWebServer rejects configurations
// with missing required fields.
// PREVENTS: server creation with no listen address or TLS material.
func TestNewWebServerRequiresFields(t *testing.T) {
	certPEM, keyPEM, err := GenerateWebCert()
	require.NoError(t, err)

	tests := []struct {
		name    string
		cfg     WebConfig
		wantErr string
	}{
		{
			name:    "missing listen address",
			cfg:     WebConfig{CertPEM: certPEM, KeyPEM: keyPEM},
			wantErr: "listen address is required",
		},
		{
			name:    "missing cert and key",
			cfg:     WebConfig{ListenAddr: "127.0.0.1:0"},
			wantErr: "certificate and key PEM data are required",
		},
		{
			name:    "missing key only",
			cfg:     WebConfig{ListenAddr: "127.0.0.1:0", CertPEM: certPEM},
			wantErr: "certificate and key PEM data are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWebServer(tt.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
