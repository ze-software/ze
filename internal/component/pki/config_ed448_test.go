package pki

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/ed448"
)

// readEd448Fixture answers testdata/ed448/name as text, the way a config leaf
// carries a PEM.
func readEd448Fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "ed448", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(raw)
}

// TestPKIAcceptsEd448KeyAndCertificate proves the PKI store takes an Ed448 entry,
// whose key crypto/x509 reads neither in PKCS #8 nor in the certificate. Goal: AC-9
// and assumption A-2. Method: the openssl-minted Ed448 key parses to an
// ed448.PrivateKey and matches its own leaf (a P-256 CA issued it, so crypto/x509
// parses the leaf and leaves PublicKey nil); another Ed448 key and an Ed25519 key
// are refused with errPKIKeyMismatch; the show helpers name the key Ed448, 456 bits.
func TestPKIAcceptsEd448KeyAndCertificate(t *testing.T) {
	block, _ := pem.Decode([]byte(readEd448Fixture(t, "leaf.pem")))
	if block == nil {
		t.Fatal("leaf.pem holds no PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("crypto/x509 refused the Ed448 leaf: %v", err)
	}
	if cert.PublicKey != nil {
		t.Fatalf("crypto/x509 parsed the Ed448 key as %T", cert.PublicKey)
	}

	key, err := parsePrivateKey(readEd448Fixture(t, "leaf-key.pem"))
	if err != nil {
		t.Fatalf("parsePrivateKey: %v", err)
	}
	if _, ok := key.(ed448.PrivateKey); !ok {
		t.Fatalf("parsePrivateKey answered %T, want ed448.PrivateKey", key)
	}
	if err := verifyKeyMatchesCert(key, cert); err != nil {
		t.Fatalf("verifyKeyMatchesCert refused the leaf's own key: %v", err)
	}

	other, err := parsePrivateKey(readEd448Fixture(t, "untrusted-leaf-key.pem"))
	if err != nil {
		t.Fatalf("parsePrivateKey of the other Ed448 key: %v", err)
	}
	if err := verifyKeyMatchesCert(other, cert); !errors.Is(err, errPKIKeyMismatch) {
		t.Fatalf("another Ed448 key: verifyKeyMatchesCert = %v, want %v", err, errPKIKeyMismatch)
	}
	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519 key: %v", err)
	}
	if err := verifyKeyMatchesCert(ed25519Key, cert); !errors.Is(err, errPKIKeyMismatch) {
		t.Fatalf("an Ed25519 key: verifyKeyMatchesCert = %v, want %v", err, errPKIKeyMismatch)
	}

	if got := keyAlgorithmName(cert); got != ed448KeyAlgorithm {
		t.Fatalf("keyAlgorithmName = %q, want %q", got, ed448KeyAlgorithm)
	}
	if got := keySize(cert); got != 8*ed448.PublicKeySize {
		t.Fatalf("keySize = %d, want %d", got, 8*ed448.PublicKeySize)
	}
}
