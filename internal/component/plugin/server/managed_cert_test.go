// VALIDATES: the managed listener serves the pki certificate the config names, and
// fails closed when that name resolves to nothing (spec-managed-server-hardening AC-1).
// PREVENTS: returning to the ephemeral self-signed certificate that made a client's
// fingerprint pin useless (it changed on every restart) and left tls-insecure as the
// one way a deployment can connect.

package server

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/selfcert"
)

// hubCertMaterial returns a certificate/key PEM pair and the hex SHA-256
// fingerprint of the leaf, standing in for what pki.ServerTLSMaterial returns.
func hubCertMaterial(t *testing.T) (certPEM, keyPEM []byte, fingerprint string) {
	t.Helper()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1:0", nil, time.Hour)
	if err != nil {
		t.Fatalf("generate certificate: %v", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("certificate PEM did not decode")
	}
	sum := sha256.Sum256(block.Bytes)
	return certPEM, keyPEM, hex.EncodeToString(sum[:])
}

// TestManagedServerServesConfiguredCertificate: the listener presents the named
// certificate, and CertificateFingerprint reports the fingerprint an operator
// copies into a client's certificate-fingerprint leaf.
//
// MUTATION: ignore cfg.Certificate in managedCertificate and generate a
// self-signed certificate instead -- the served leaf no longer matches the
// resolved material and both assertions fail.
func TestManagedServerServesConfiguredCertificate(t *testing.T) {
	certPEM, keyPEM, fingerprint := hubCertMaterial(t)

	srv, err := NewManagedServer(ManagedServerConfig{
		Addrs:         []string{"127.0.0.1:0"},
		ClientSecrets: map[string]string{testClientName: testClientSecret},
		ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
		Certificate:   "fleet-hub",
		TLSMaterialResolver: func(name string) ([]byte, []byte, error) {
			if name != "fleet-hub" {
				return nil, nil, errors.New("unexpected certificate name " + name)
			}
			return certPEM, keyPEM, nil
		},
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(srv.Stop)

	if got := srv.CertificateFingerprint(); got != fingerprint {
		t.Errorf("CertificateFingerprint = %s, want %s", got, fingerprint)
	}

	// A client that pins the configured certificate completes the handshake.
	dialer := &tls.Dialer{Config: pinnedConfig(fingerprint)}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := dialer.DialContext(ctx, "tcp", srv.Addrs()[0].String())
	if err != nil {
		t.Fatalf("dial with the configured certificate pinned: %v", err)
	}
	conn.Close() //nolint:errcheck // test cleanup
}

// pinnedConfig verifies the server certificate against fingerprint. It repeats
// what ipc.TLSConfigWithFingerprint does rather than calling it, so this test
// still fails if that helper is ever weakened.
func pinnedConfig(fingerprint string) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // verified below by fingerprint
		MinVersion:         tls.VersionTLS13,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("no peer certificate")
			}
			sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
			if hex.EncodeToString(sum[:]) != fingerprint {
				return errors.New("fingerprint mismatch")
			}
			return nil
		},
	}
}

// TestManagedServerFailsClosedOnCertificate: a configured name that cannot be
// resolved refuses to build the server. It never falls back to a self-signed
// certificate. Such a listener looks healthy while the config names a real
// certificate, until a client refuses the handshake.
//
// MUTATION: fall back to GenerateSelfSignedCert on a resolver error and both
// cases fail.
func TestManagedServerFailsClosedOnCertificate(t *testing.T) {
	base := ManagedServerConfig{
		Addrs:         []string{"127.0.0.1:0"},
		ClientSecrets: map[string]string{testClientName: testClientSecret},
		ReadConfig:    func(string) ([]byte, error) { return []byte(testClientConfig), nil },
		Certificate:   "missing",
	}

	t.Run("resolver-error", func(t *testing.T) {
		cfg := base
		cfg.TLSMaterialResolver = func(string) ([]byte, []byte, error) {
			return nil, nil, errors.New("certificate missing not found")
		}
		srv, err := NewManagedServer(cfg)
		if err == nil {
			srv.Stop()
			t.Fatal("NewManagedServer accepted an unresolvable certificate name")
		}
	})

	t.Run("no-resolver", func(t *testing.T) {
		cfg := base
		srv, err := NewManagedServer(cfg)
		if err == nil {
			srv.Stop()
			t.Fatal("NewManagedServer accepted a certificate name with no resolver")
		}
	})
}
