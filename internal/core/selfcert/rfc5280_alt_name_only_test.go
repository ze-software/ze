// Design: docs/architecture/pki/tls-listeners.md -- the self-signed web certificate
// Related: selfcert_test.go -- generation, the SAN set and the TLS config
//
// VALIDATES: the self-signed web certificate names its listener only in the subject
// alternative name, so its subject is the empty sequence and the extension is critical.
// PREVENTS: a certificate whose only identity sits in a non-critical extension, which a
// verifier that skips non-critical extensions reads as identifying nothing.

package selfcert

import (
	"bytes"
	"encoding/asn1"
	"math/big"
	"slices"
	"testing"
)

// oidSubjectAltName is id-ce-subjectAltName, RFC 5280 Section 4.2.1.6.
var oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}

// TestRFC5280AltNameOnlyCertificateMarksTheExtensionCritical checks the subject and the
// subjectAltName extension of the certificate GenerateWebCertWithNames writes.
//
// Method: one certificate for a listen address and one extra DNS name is generated and
// read back from DER. Its subject is asserted to be the empty sequence (30 00), so the
// listener is named only by the alternative name, and that extension is found and
// asserted present, non-empty and critical. The certificate's subject is empty because
// RFC5280-4.1.2.4-1 is a {gap} in rfc/short/rfc5280.md: the issuer of this self-signed
// certificate is its subject. Closing that gap by giving the certificate a subject
// makes this test's first assertion red, and these three rows then need re-judging.
//
// RFC requirement: RFC5280-4.1.2.6-3 positive -- the certificate's naming information is
// only in subjectAltName, its subject is the empty sequence, and subjectAltName is critical.
// RFC requirement: RFC5280-4.2.1.6-3 positive -- the only subject identities are
// alternative names, the subject DN is the empty sequence, and subjectAltName is present.
// RFC requirement: RFC5280-4.2-5 positive -- this self-signed certificate has an empty
// subject and carries the subject alternative name extension, holding the listener's
// names.
func TestRFC5280AltNameOnlyCertificateMarksTheExtensionCritical(t *testing.T) {
	certPEM, _, err := GenerateWebCertWithNames("192.0.2.7:443", []string{"router.example.net"}, 0)
	if err != nil {
		t.Fatalf("GenerateWebCertWithNames: %v", err)
	}
	cert := parseCertForTest(t, certPEM)

	if !bytes.Equal(cert.RawSubject, []byte{0x30, 0x00}) {
		t.Fatalf("the subject is % x, want the empty sequence 30 00", cert.RawSubject)
	}
	found := false
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidSubjectAltName) {
			continue
		}
		found = true
		if !ext.Critical {
			t.Error("the subject is empty and the subjectAltName extension is not critical")
		}
	}
	if !found {
		t.Fatal("the subject is empty and the certificate carries no subjectAltName extension")
	}
	if !slices.Contains(cert.DNSNames, "router.example.net") {
		t.Errorf("the subjectAltName holds DNS %v, want router.example.net among them", cert.DNSNames)
	}
	if len(cert.IPAddresses) == 0 {
		t.Error("the subjectAltName holds no address for the listener")
	}
}

// TestRFC5280WebCertificateSerialIsPositive checks the serial GenerateWebCertWithNames
// draws.
//
// Method: 32 certificates are generated and each serial is asserted greater than zero
// and below 2^128. The zero this guards against has probability 2^-128 per draw, so the
// test cannot observe it; the bounds it can observe are the range the draw is taken from.
func TestRFC5280WebCertificateSerialIsPositive(t *testing.T) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	for i := range 32 {
		certPEM, _, err := GenerateWebCertWithNames("", nil, 0)
		if err != nil {
			t.Fatalf("GenerateWebCertWithNames %d: %v", i, err)
		}
		serial := parseCertForTest(t, certPEM).SerialNumber
		if serial.Sign() <= 0 {
			t.Fatalf("serial %d is not positive: %s", i, serial)
		}
		if serial.Cmp(limit) >= 0 {
			t.Fatalf("serial %d needs %d bits, more than 128", i, serial.BitLen())
		}
	}
}
