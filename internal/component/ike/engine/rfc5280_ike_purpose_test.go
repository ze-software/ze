// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// Related: remote_cert_checks.go -- checkIKEPurpose, the check these cases drive
//
// VALIDATES: an IKE peer certificate is used only for a purpose consistent with both its
// key usage and its extended key usage, judged by RFC 4945's rule for IKE.
// PREVENTS: a certificate issued for TLS alone, or one whose key may not sign,
// authenticating an IKE SA because Verify ran with ExtKeyUsageAny and nothing read the
// leaf's key usage.

package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// purposeLeaf builds an end-entity certificate with the given serial, key usage and
// extended key usages, signed by issuer. A zero keyUsage leaves the key usage
// extension out, and empty usage lists leave the extended key usage extension out.
func purposeLeaf(t *testing.T, cn string, serial int64, keyUsage x509.KeyUsage, usages []x509.ExtKeyUsage,
	unknown []asn1.ObjectIdentifier, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey,
) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:       big.NewInt(serial),
		Subject:            pkix.Name{CommonName: cn},
		NotBefore:          time.Now().Add(-time.Hour),
		NotAfter:           time.Now().Add(time.Hour),
		KeyUsage:           keyUsage,
		ExtKeyUsage:        usages,
		UnknownExtKeyUsage: unknown,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatalf("create the leaf certificate: %v", err)
	}
	return der, key
}

// emptyKeyUsageLeaf builds an end-entity certificate whose key usage extension is
// present and sets no bit: a BIT STRING of no bits (03 01 00), marked critical as
// RFC 5280 Section 4.2.1.3 recommends. crypto/x509 parses it to a zero KeyUsage,
// the same value an absent extension leaves.
func emptyKeyUsageLeaf(t *testing.T, cn string, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey,
) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(6),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtraExtensions: []pkix.Extension{{
			Id:       oidExtensionKeyUsage,
			Critical: true,
			Value:    []byte{0x03, 0x01, 0x00},
		}},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatalf("create the leaf certificate: %v", err)
	}
	return der, key
}

// authenticatePeerCertificate runs IKE_AUTH verification of a peer that presents
// leafDER, signed AUTH with key, sends the intermediates given, and is configured
// with caName as its ca-certificate and identity as its remote-id. It answers
// what verifyRemoteAuth answers.
func authenticatePeerCertificate(t *testing.T, caName, identity string, leafDER []byte, key *ecdsa.PrivateKey,
	intermediates ...[]byte,
) error {
	t.Helper()
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthX509
	sa.PeerCfg.Auth.CACertificate = caName
	sa.PeerCfg.Auth.RemoteID = identity
	sa.RemoteIDPayload = &wire.PayloadID{
		IDPayloadType: wire.PayloadTypeIDr,
		IDType:        wire.IDTypeFQDN,
		IDData:        []byte(identity),
	}
	sa.RemoteCertRaw = leafDER
	sa.RemoteCertChainRaw = intermediates
	return verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key))
}

// loadPKIForTest loads the given ca entries into the pki store and clears it when
// the test ends.
func loadPKIForTest(t *testing.T, entries ...*pki.CACertEntry) {
	t.Helper()
	cas := make(map[string]*pki.CACertEntry, len(entries))
	for _, entry := range entries {
		cas[entry.Name] = entry
	}
	if err := pki.Load(&pki.PKIConfig{CACerts: cas}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})
}

// TestRFC5280Section42112IKEPurpose checks that IKE_AUTH refuses a peer certificate
// whose key usage or extended key usage excludes IKE, and accepts one both admit.
//
// Method: one CA, loaded as the peer's ca-certificate, issues leaves that differ only
// in their key usage and extended key usage extensions. Each leaf signs a valid AUTH
// payload, so a refusal comes from the purpose check and not from the chain or the
// signature.
func TestRFC5280Section42112IKEPurpose(t *testing.T) {
	caCert, caDER, caKey := rctCA(t, "purpose-ca")
	loadPKIForTest(t, &pki.CACertEntry{Name: "purpose-ca", Certificate: caCert, Raw: caDER})
	run := func(keyUsage x509.KeyUsage, usages []x509.ExtKeyUsage, unknown []asn1.ObjectIdentifier) error {
		t.Helper()
		der, key := purposeLeaf(t, "purpose-peer", 5, keyUsage, usages, unknown, caCert, caKey)
		return authenticatePeerCertificate(t, "purpose-ca", "purpose-peer", der, key)
	}

	// RFC requirement: RFC5280-4.2.1.12-1 positive -- a leaf whose extended key usage
	// names serverAuth alone, a purpose other than IKE, is refused for IKE
	// authentication, and the refusal names the extended key usage.
	// RFC requirement: RFC5280-4.2.1.12-2 positive -- a leaf whose key usage sets
	// digitalSignature and whose extended key usage names serverAuth alone is refused,
	// and the refusal names the extended key usage.
	err := run(x509.KeyUsageDigitalSignature, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, nil)
	if err == nil {
		t.Error("a serverAuth-only certificate authenticated an IKE peer")
	} else if !strings.Contains(err.Error(), "extended key usage") {
		t.Errorf("the refusal does not name the extended key usage: %v", err)
	}
	// RFC requirement: RFC5280-4.2.1.12-2 positive -- a leaf whose extended key usage
	// names id-kp-ipsecIKE and whose key usage sets keyEncipherment alone is refused,
	// and the refusal names the key usage and not the extended key usage.
	err = run(x509.KeyUsageKeyEncipherment, nil, []asn1.ObjectIdentifier{oidKeyPurposeIPsecIKE})
	if err == nil {
		t.Error("a certificate whose key usage forbids signing authenticated an IKE peer")
	} else {
		if !strings.Contains(err.Error(), "key usage") {
			t.Errorf("the refusal does not name the key usage: %v", err)
		}
		if strings.Contains(err.Error(), "extended key usage") {
			t.Errorf("the refusal blames the extended key usage, which admits IKE: %v", err)
		}
	}
	// RFC requirement: RFC5280-4.2.1.12-2 negative -- a leaf whose key usage sets
	// digitalSignature and whose extended key usage names id-kp-ipsecIKE authenticates.
	if err := run(x509.KeyUsageDigitalSignature, nil, []asn1.ObjectIdentifier{oidKeyPurposeIPsecIKE}); err != nil {
		t.Errorf("a certificate consistent with IKE in both extensions was refused: %v", err)
	}

	// The other shapes RFC 4945 admits: no extended key usage, anyExtendedKeyUsage
	// beside another purpose, and a key usage of nonRepudiation alone.
	if err := run(x509.KeyUsageDigitalSignature, nil, nil); err != nil {
		t.Errorf("a certificate with no extended key usage was refused: %v", err)
	}
	if err := run(x509.KeyUsageDigitalSignature, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageAny}, nil); err != nil {
		t.Errorf("a certificate carrying anyExtendedKeyUsage was refused: %v", err)
	}
	if err := run(x509.KeyUsageContentCommitment, nil, nil); err != nil {
		t.Errorf("a certificate whose key usage is nonRepudiation alone was refused: %v", err)
	}
	// A key usage extension that sets no bit is present, and it admits no signing
	// key: presence is read from the extension, never from a zero KeyUsage field,
	// which an absent extension leaves zero too.
	emptyDER, emptyKey := emptyKeyUsageLeaf(t, "purpose-peer", caCert, caKey)
	if err := authenticatePeerCertificate(t, "purpose-ca", "purpose-peer", emptyDER, emptyKey); err == nil {
		t.Error("a certificate whose key usage extension sets no bit authenticated an IKE peer")
	} else if !strings.Contains(err.Error(), "key usage") {
		t.Errorf("the refusal does not name the key usage: %v", err)
	}

	// RFC requirement: RFC5280-4.2.1.12-1 negative -- a leaf with no key usage whose
	// extended key usage names id-kp-ipsecIKE beside clientAuth authenticates, because
	// IKE is one of the purposes it indicates.
	if err := run(0, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, []asn1.ObjectIdentifier{oidKeyPurposeIPsecIKE}); err != nil {
		t.Errorf("a certificate naming id-kp-ipsecIKE beside clientAuth was refused: %v", err)
	}
}
