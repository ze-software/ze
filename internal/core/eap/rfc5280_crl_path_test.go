// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS certificate revocation
// Related: rfc9190_revocation_test.go -- the same gate driven through whole conversations
//
// VALIDATES: the revocation list EAP-TLS reads for a certificate is one its issuer's own
// key signed, reached through the issuer on the chain the certificate was verified on.
// PREVENTS: a list that names the same issuer but was signed under another trust anchor
// answering for a certificate, revoked or not.

package eap

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

// TestRFC5280CRLMustChainToTheCertificatesOwnAnchor checks which configured list the
// revocation gate reads for one certificate.
//
// Method: a CA and an impostor CA share one subject DN and hold different keys. The CA
// issues a leaf with serial 77. Each CA signs a list revoking serial 77, and the gate
// runs over the verified chain [leaf, CA] with each list alone and with an unrevoking
// list from the CA. A list is used only when the chain's CA key signed it.
func TestRFC5280CRLMustChainToTheCertificatesOwnAnchor(t *testing.T) {
	ca, caKey, _ := newCA(t, "crl-path-ca", 1)
	impostor, impostorKey, _ := newCA(t, "crl-path-ca", 2)
	leafPEM, _ := newLeaf(t, ca, caKey, "crl-path-node", 77, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	block, _ := pem.Decode(leafPEM)
	if block == nil {
		t.Fatal("the leaf PEM did not decode")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse the leaf: %v", err)
	}
	chain := []*x509.Certificate{leaf, ca}
	now := time.Now()
	gate := func(crlPEM []byte) error {
		t.Helper()
		set, err := parseCRLs(crlPEM)
		if err != nil {
			t.Fatalf("parseCRLs: %v", err)
		}
		return set.checkChain(chain, now)
	}

	// RFC requirement: RFC5280-5.1.1.3-1 positive -- the certificate and the list are
	// both signed with the one CA private key, the gate validates the list against that
	// CA and reads it: the leaf's serial is reported revoked.
	if err := gate(newCRL(t, ca, caKey, 77)); err == nil || !strings.Contains(err.Error(), "was revoked") {
		t.Errorf("a list the leaf's own CA signed was not read: %v", err)
	}
	// RFC requirement: RFC5280-5.1.1.3-1 negative -- an unrevoking list signed with the
	// same CA key is read too, and the chain passes.
	if err := gate(newCRL(t, ca, caKey)); err != nil {
		t.Errorf("an unrevoking list the leaf's own CA signed refused the chain: %v", err)
	}

	// RFC requirement: RFC5280-6.3.3-1 positive -- a list naming the same issuer DN but
	// signed by another trust anchor's key is not read, so it neither revokes the leaf
	// nor answers for it: the gate reports that no current list from the chain's CA exists.
	err = gate(newCRL(t, impostor, impostorKey, 77))
	if err == nil {
		t.Fatal("a list from another trust anchor answered for the leaf")
	}
	if strings.Contains(err.Error(), "was revoked") {
		t.Errorf("a list from another trust anchor revoked the leaf: %v", err)
	}
	if !strings.Contains(err.Error(), "no current certificate revocation list") {
		t.Errorf("the refusal does not say the chain's CA published no list: %v", err)
	}
	// RFC requirement: RFC5280-6.3.3-1 negative -- with both lists configured, the one
	// under the chain's own anchor answers, so the leaf is reported revoked by it.
	both := append(newCRL(t, impostor, impostorKey), newCRL(t, ca, caKey, 77)...)
	if err := gate(both); err == nil || !strings.Contains(err.Error(), "was revoked") {
		t.Errorf("the list under the chain's own anchor was not read beside an impostor's: %v", err)
	}
}
