// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS certificate revocation
// Related: rfc5280_crl_path_test.go -- which list the gate reads for a certificate
//
// VALIDATES: a revocation list carrying a critical extension, at list level or on an
// entry, answers nothing about any certificate, and the issuer's other lists still do.
// PREVENTS: a delta CRL, an issuing-distribution-point CRL or a list with an entry
// extension Ze cannot process being read as a complete list, which reports "not
// revoked" for a certificate the full list would revoke.

package eap

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// Extension identifiers the critical-extension cases put on a list.
var (
	// oidTestPrivateCRLExtension is an extension no implementation processes, from
	// the example private arc.
	oidTestPrivateCRLExtension = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 32473, 1}
	// oidTestIssuingDistributionPoint is id-ce-issuingDistributionPoint (RFC 5280
	// Section 5.2.5), which the RFC makes critical.
	oidTestIssuingDistributionPoint = asn1.ObjectIdentifier{2, 5, 29, 28}
	// oidTestCertificateIssuer is id-ce-certificateIssuer (RFC 5280 Section 5.3.3),
	// a critical CRL entry extension.
	oidTestCertificateIssuer = asn1.ObjectIdentifier{2, 5, 29, 29}
)

// crlWithExtensions issues a list signed by caCert/caKey, numbered number,
// carrying listExtensions at list level and revoking each serial in revoked, each
// entry carrying entryExtensions. It is current from an hour ago to an hour ahead.
func crlWithExtensions(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, number int64,
	listExtensions, entryExtensions []pkix.Extension, revoked ...int64,
) []byte {
	t.Helper()
	entries := make([]x509.RevocationListEntry, 0, len(revoked))
	for _, serial := range revoked {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:    big.NewInt(serial),
			RevocationTime:  time.Now().Add(-time.Minute),
			ExtraExtensions: entryExtensions,
		})
	}
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:                    big.NewInt(number),
		ThisUpdate:                time.Now().Add(-time.Hour),
		NextUpdate:                time.Now().Add(time.Hour),
		RevokedCertificateEntries: entries,
		ExtraExtensions:           listExtensions,
	}, caCert, caKey)
	if err != nil {
		t.Fatalf("create the revocation list: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: der})
}

// TestRFC5280Section52UnprocessedCriticalCRL checks that the revocation gate refuses
// to read a list carrying a critical CRL or CRL entry extension.
//
// Method: one CA issues a leaf with serial 77. Each case configures lists from that
// CA with an extension marked critical or not, and runs the gate both EAP-TLS and
// IKE reach (crlSet.checkChain) over the verified chain [leaf, CA]. A list with a
// critical extension must be skipped: alone, the status is unknown and the chain is
// refused naming the extension; beside a plain list from the same CA, the plain list
// answers. The same extension marked non-critical leaves the list in use.
func TestRFC5280Section52UnprocessedCriticalCRL(t *testing.T) {
	ca, caKey, _ := newCA(t, "crl-critical-ca", 1)
	leafPEM, _ := newLeaf(t, ca, caKey, "crl-critical-node", 77, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
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
	// unknown asserts the gate refused the chain as having no usable list, naming
	// the extension that made the list unusable, and did not report it revoked.
	unknown := func(err error, oid asn1.ObjectIdentifier, what string) {
		t.Helper()
		if err == nil {
			t.Errorf("%s answered for the leaf", what)
			return
		}
		if strings.Contains(err.Error(), "was revoked") {
			t.Errorf("%s was read and revoked the leaf: %v", what, err)
		}
		if !strings.Contains(err.Error(), "no current certificate revocation list") {
			t.Errorf("the refusal for %s does not say no usable list exists: %v", what, err)
		}
		if !strings.Contains(err.Error(), oid.String()) {
			t.Errorf("the refusal for %s does not name extension %s: %v", what, oid, err)
		}
	}
	extension := func(oid asn1.ObjectIdentifier, critical bool) []pkix.Extension {
		return []pkix.Extension{{Id: oid, Critical: critical, Value: []byte{0x30, 0x00}}}
	}

	// RFC requirement: RFC5280-5.2-1 positive -- a list from the chain's CA that revokes
	// serial 77 and carries a critical list-level extension of an unknown OID is not
	// read: the gate reports no current list, names the OID, and does not report the
	// leaf revoked.
	unknown(gate(crlWithExtensions(t, ca, caKey, 1, extension(oidTestPrivateCRLExtension, true), nil, 77)),
		oidTestPrivateCRLExtension, "a list with a critical unknown extension")
	// RFC requirement: RFC5280-5.2-1 negative -- the same extension marked non-critical
	// leaves the list in use: a list revoking serial 77 reports the leaf revoked, and
	// one revoking nothing passes the chain.
	if err := gate(crlWithExtensions(t, ca, caKey, 1, extension(oidTestPrivateCRLExtension, false), nil, 77)); err == nil ||
		!strings.Contains(err.Error(), "was revoked") {
		t.Errorf("a list with a non-critical unknown extension did not revoke the leaf it lists: %v", err)
	}
	if err := gate(crlWithExtensions(t, ca, caKey, 1, extension(oidTestPrivateCRLExtension, false), nil)); err != nil {
		t.Errorf("a list with a non-critical unknown extension refused a leaf it does not list: %v", err)
	}

	// RFC requirement: RFC5280-5.2.5-1 positive -- a list from the chain's CA carrying a
	// critical issuingDistributionPoint and not listing serial 77 leaves the leaf's
	// status unknown: the gate refuses the chain naming 2.5.29.28.
	unknown(gate(crlWithExtensions(t, ca, caKey, 2, extension(oidTestIssuingDistributionPoint, true), nil)),
		oidTestIssuingDistributionPoint, "an issuing-distribution-point list")
	// RFC requirement: RFC5280-5.2.5-1 negative -- beside that list, a second list from
	// the same CA without the extension answers: revoking serial 77 it reports the leaf
	// revoked, and revoking nothing it passes the chain.
	partial := crlWithExtensions(t, ca, caKey, 2, extension(oidTestIssuingDistributionPoint, true), nil)
	revokedBoth := append(append([]byte{}, partial...), crlWithExtensions(t, ca, caKey, 3, nil, nil, 77)...)
	if err := gate(revokedBoth); err == nil || !strings.Contains(err.Error(), "was revoked") {
		t.Errorf("a complete list beside an issuing-distribution-point list did not revoke the leaf: %v", err)
	}
	cleanBoth := append(append([]byte{}, partial...), crlWithExtensions(t, ca, caKey, 3, nil, nil)...)
	if err := gate(cleanBoth); err != nil {
		t.Errorf("a complete list beside an issuing-distribution-point list did not answer: %v", err)
	}

	// RFC requirement: RFC5280-5.3-1 positive -- a list from the chain's CA whose only
	// entry, serial 99, carries a critical certificateIssuer entry extension is not
	// read, so serial 77, which it does not list, is refused as unknown, naming 2.5.29.29.
	unknown(gate(crlWithExtensions(t, ca, caKey, 4, nil, extension(oidTestCertificateIssuer, true), 99)),
		oidTestCertificateIssuer, "a list with a critical entry extension")
	// RFC requirement: RFC5280-5.3-1 negative -- the same entry extension marked
	// non-critical leaves the list in use, and serial 77, not listed, passes.
	if err := gate(crlWithExtensions(t, ca, caKey, 4, nil, extension(oidTestCertificateIssuer, false), 99)); err != nil {
		t.Errorf("a list with a non-critical entry extension refused a leaf it does not list: %v", err)
	}
}

// TestVerifyChainsNotRevokedNeverPassesUnchecked checks that the entry point the IKE
// engine calls answers an error, never a pass, when it was given nothing to check
// with or nothing to check, and that it reports a revoked leaf naming the CRL number.
//
// Method: one CA issues a leaf with serial 77 and a list revoking it, numbered 9.
// The entry point runs with empty material, with no chain, and with both present.
func TestVerifyChainsNotRevokedNeverPassesUnchecked(t *testing.T) {
	ca, caKey, _ := newCA(t, "crl-entry-ca", 1)
	leafPEM, _ := newLeaf(t, ca, caKey, "crl-entry-node", 77, nil)
	block, _ := pem.Decode(leafPEM)
	if block == nil {
		t.Fatal("the leaf PEM did not decode")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse the leaf: %v", err)
	}
	chains := [][]*x509.Certificate{{leaf, ca}}
	revoking := crlWithExtensions(t, ca, caKey, 9, nil, nil, 77)

	if err := VerifyChainsNotRevoked(nil, chains, time.Now()); err == nil {
		t.Error("no revocation material answered as not revoked")
	}
	if err := VerifyChainsNotRevoked(revoking, nil, time.Now()); err == nil {
		t.Error("no chain answered as not revoked")
	}
	err = VerifyChainsNotRevoked(revoking, chains, time.Now())
	if err == nil {
		t.Fatal("a revoked leaf passed")
	}
	for _, want := range []string{"was revoked", "(serial 77)", "CRL number 9"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal lacks %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "eap-tls") {
		t.Errorf("the role-neutral refusal names EAP-TLS: %v", err)
	}
}
