// Design: docs/architecture/pki/pki-store.md -- explicit ephemeral authority
package pki

import (
	"bytes"
	"crypto/x509"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// An ephemeral authority signs renewable leaves from one retained root, while
// another construction (the next process lifetime) has a different trust anchor.
func TestEphemeralRootLifetime(t *testing.T) {
	previous := currentRoot.Load()
	t.Cleanup(func() { currentRoot.Store(previous) })
	first, err := NewEphemeralRoot()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		material, err := first.IssueLeaf("plugin", []string{"127.0.0.1"})
		if err != nil {
			t.Fatal(err)
		}
		leaf, err := x509.ParseCertificate(material.Certificate[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := leaf.CheckSignatureFrom(first.Certificate()); err != nil {
			t.Fatal(err)
		}
	}
	second, err := NewEphemeralRoot()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first.CertificatePEM(), second.CertificatePEM()) {
		t.Fatal("ephemeral authority survived a new lifetime")
	}
	if loadedRoot() != second {
		t.Fatal("active CA export does not name the selected authority")
	}
}

// A partial persistent pair and invalid stored material must never rotate trust.
func TestPersistentRootFailureNeverGeneratesReplacement(t *testing.T) {
	for _, key := range []string{zefs.KeyCACert.Pattern, zefs.KeyCAKey.Pattern} {
		t.Run(key, func(t *testing.T) {
			store, _ := newRootStore(t)
			original := []byte("broken persistent material")
			if err := store.WriteFile(key, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadOrGenerateRoot(store); err == nil {
				t.Fatal("partial CA accepted")
			}
			got, err := store.ReadFile(key)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("persistent evidence replaced: %q,%v", got, err)
			}
		})
	}
}
