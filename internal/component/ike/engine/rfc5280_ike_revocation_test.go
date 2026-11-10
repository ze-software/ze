// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// Related: remote_cert_checks.go -- checkPeerChainRevocation, the check these cases drive
//
// VALIDATES: IKE_AUTH refuses a peer whose verified chain carries a certificate revoked
// by a list a pki ca on that chain holds, or whose status those lists cannot answer, and
// authenticates a peer when no pki ca on its chain holds a list.
// PREVENTS: a revoked IKE peer certificate authenticating because getRemoteCert ran
// Verify and nothing checked RFC 5280 Section 6.1.3 (a)(3).

package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/pki"
)

// revocationCA builds a CA certificate with the given serial that may sign
// certificates and revocation lists. A nil issuer makes it a self-signed root.
func revocationCA(t *testing.T, cn string, serial int64, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey,
) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		// CRLSign because x509.CreateRevocationList refuses an issuer whose key
		// usage is set and omits it.
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	parent, parentKey := tmpl, key
	if issuer != nil {
		parent, parentKey = issuer, issuerKey
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatalf("create the CA certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse the CA certificate: %v", err)
	}
	return cert, der, key
}

// revocationList issues a DER list signed by issuer, numbered number, whose
// nextUpdate is nextUpdate, revoking each serial given.
func revocationList(t *testing.T, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey, number int64,
	nextUpdate time.Time, revoked ...int64,
) []byte {
	t.Helper()
	entries := make([]x509.RevocationListEntry, 0, len(revoked))
	for _, serial := range revoked {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:   big.NewInt(serial),
			RevocationTime: time.Now().Add(-time.Minute),
		})
	}
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:                    big.NewInt(number),
		ThisUpdate:                nextUpdate.Add(-2 * time.Hour),
		NextUpdate:                nextUpdate,
		RevokedCertificateEntries: entries,
	}, issuer, issuerKey)
	if err != nil {
		t.Fatalf("create the revocation list: %v", err)
	}
	return der
}

// TestRFC5280Section613RevokedIKEPeerRefused checks the revocation status IKE_AUTH
// reads for a peer certificate chain.
//
// Method: a root CA, loaded as the peer's ca-certificate, issues a leaf with serial 40
// directly, and an intermediate with serial 20 that issues a leaf with serial 41. Each
// case loads the pki store with the root, and for the two-level cases the intermediate,
// each holding the lists the case names, then runs verifyRemoteAuth over a valid AUTH
// payload. A refusal therefore comes from the revocation check alone.
func TestRFC5280Section613RevokedIKEPeerRefused(t *testing.T) {
	root, rootDER, rootKey := revocationCA(t, "revocation-root", 1, nil, nil)
	inter, interDER, interKey := revocationCA(t, "revocation-inter", 20, root, rootKey)
	leafDER, leafKey := purposeLeaf(t, "revocation-peer", 40, x509.KeyUsageDigitalSignature, nil, nil, root, rootKey)
	deepDER, deepKey := purposeLeaf(t, "revocation-deep", 41, x509.KeyUsageDigitalSignature, nil, nil, inter, interKey)
	current := time.Now().Add(time.Hour)
	rootEntry := func(lists ...[]byte) *pki.CACertEntry {
		return &pki.CACertEntry{Name: "revocation-root", Certificate: root, Raw: rootDER, RawCRLs: lists}
	}
	interEntry := func(lists ...[]byte) *pki.CACertEntry {
		return &pki.CACertEntry{Name: "revocation-inter", Certificate: inter, Raw: interDER, RawCRLs: lists}
	}

	t.Run("leaf-revoked", func(t *testing.T) {
		loadPKIForTest(t, rootEntry(revocationList(t, root, rootKey, 7, current, 40)))
		err := authenticatePeerCertificate(t, "revocation-root", "revocation-peer", leafDER, leafKey)
		// RFC requirement: RFC5280-6.1.3-1 positive -- a leaf whose serial the root's
		// current list revokes is refused, and the refusal names the leaf's serial, the
		// list's number and the pki ca the list came from.
		if err == nil {
			t.Fatal("a revoked peer certificate authenticated")
		}
		for _, want := range []string{"was revoked", "(serial 40)", "CRL number 7", "pki ca revocation-root"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal lacks %q: %v", want, err)
			}
		}
	})

	t.Run("leaf-not-listed", func(t *testing.T) {
		loadPKIForTest(t, rootEntry(revocationList(t, root, rootKey, 7, current, 99)))
		// RFC requirement: RFC5280-6.1.3-1 negative -- the same root list revoking
		// another serial leaves the leaf unrevoked, and the peer authenticates.
		if err := authenticatePeerCertificate(t, "revocation-root", "revocation-peer", leafDER, leafKey); err != nil {
			t.Errorf("a peer certificate the list does not revoke was refused: %v", err)
		}
	})

	t.Run("intermediate-revoked", func(t *testing.T) {
		loadPKIForTest(t,
			rootEntry(revocationList(t, root, rootKey, 8, current, 20)),
			interEntry(revocationList(t, inter, interKey, 3, current)))
		err := authenticatePeerCertificate(t, "revocation-root", "revocation-deep", deepDER, deepKey, interDER)
		if err == nil {
			t.Fatal("a peer chain through a revoked intermediate authenticated")
		}
		if !strings.Contains(err.Error(), "was revoked") || !strings.Contains(err.Error(), "(serial 20)") {
			t.Errorf("the refusal does not name the revoked intermediate: %v", err)
		}
	})

	t.Run("intermediate-without-list", func(t *testing.T) {
		// The root holds a list, so the check is strict, and nothing answers for the
		// leaf the intermediate issued.
		loadPKIForTest(t, rootEntry(revocationList(t, root, rootKey, 8, current)))
		err := authenticatePeerCertificate(t, "revocation-root", "revocation-deep", deepDER, deepKey, interDER)
		if err == nil {
			t.Fatal("a peer whose issuer has no list authenticated under a configured source")
		}
		if !strings.Contains(err.Error(), "no current certificate revocation list") {
			t.Errorf("the refusal does not say the status is unknown: %v", err)
		}
	})

	t.Run("two-level-clean", func(t *testing.T) {
		loadPKIForTest(t,
			rootEntry(revocationList(t, root, rootKey, 8, current)),
			interEntry(revocationList(t, inter, interKey, 3, current)))
		if err := authenticatePeerCertificate(t, "revocation-root", "revocation-deep", deepDER, deepKey, interDER); err != nil {
			t.Errorf("a two-level chain no list revokes was refused: %v", err)
		}
	})

	t.Run("no-list-configured", func(t *testing.T) {
		loadPKIForTest(t, rootEntry())
		if err := authenticatePeerCertificate(t, "revocation-root", "revocation-peer", leafDER, leafKey); err != nil {
			t.Errorf("a peer whose chain meets no list was refused: %v", err)
		}
	})

	t.Run("stale-list", func(t *testing.T) {
		loadPKIForTest(t, rootEntry(revocationList(t, root, rootKey, 7, time.Now().Add(-time.Minute))))
		err := authenticatePeerCertificate(t, "revocation-root", "revocation-peer", leafDER, leafKey)
		if err == nil {
			t.Fatal("a list past its nextUpdate answered for the peer")
		}
		if !strings.Contains(err.Error(), "nextUpdate") {
			t.Errorf("the refusal does not name the passed nextUpdate: %v", err)
		}
	})
}
