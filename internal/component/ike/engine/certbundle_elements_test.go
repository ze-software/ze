// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// RFC: rfc/short/rfc7296.md -- the X.509 bundle ASN.1 for Hash and URL (Section 3.6)

package engine

import (
	"encoding/asn1"
	"errors"
	"testing"
)

// bundleOf builds a CertificateBundle from the given CHOICE tags, one element each.
func bundleOf(t *testing.T, tags []int) []byte {
	t.Helper()
	var elements []byte
	for _, tag := range tags {
		element, err := asn1.Marshal(asn1.RawValue{
			Class:      asn1.ClassContextSpecific,
			Tag:        tag,
			IsCompound: true,
			Bytes:      []byte{0x30, 0x00}, // an empty SEQUENCE stands in for the value
		})
		if err != nil {
			t.Fatalf("marshal element: %v", err)
		}
		elements = append(elements, element...)
	}
	bundle, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      elements,
	})
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	return bundle
}

// The DER identifier octets RFC 7296 Section 3.6's ASN.1 module produces, written as
// LITERALS.
//
// CertificateOrCRL ::= CHOICE { cert [0] Certificate, crl [1] CertificateList } under
// DEFINITIONS EXPLICIT TAGS. An EXPLICIT context-specific alternative is constructed, so the
// identifier octet is (class 2 << 6) | (constructed 1 << 5) | tag: 0xA0 for cert [0] and
// 0xA1 for crl [1]. The bundle itself is a SEQUENCE OF, identifier 0x30.
//
// These are written out rather than derived from certBundleTagCert and certBundleTagCRL on
// purpose. A derived expectation makes the encoder its own oracle. A moved constant then
// moves the expectation with it. Every assertion below stays green while ze emits bytes
// that strongSwan and every other conforming peer reject.
const (
	cbeCertIdentifier = 0xA0
	cbeCRLIdentifier  = 0xA1
	cbeSequence       = 0x30
)

// VALIDATES: the bundle ze ENCODES carries the identifier octets RFC 7296 Section 3.6's
// ASN.1 module requires, and the bundle ze DECODES reads those same octets. Both are pinned
// to the literal DER bytes rather than to the constants the codec uses.
// PREVENTS: the CHOICE tag numbers drifting silently. certbundle.go names them once and both
// halves of the codec read that one name, so a changed constant keeps ze self-consistent and
// makes it wrong against every peer. The literals below are the only thing in the package
// that disagrees when that happens.
func TestCertBundleUsesTheExplicitDERTagBytes(t *testing.T) {
	// Encode side. asn1.Unmarshal is stdlib, so the outer SEQUENCE is opened by something
	// other than the code under test, and its first content octet is read directly.
	bundle, err := encodeCertBundle([][]byte{{0x30, 0x00}})
	if err != nil {
		t.Fatalf("encodeCertBundle: %v", err)
	}
	if bundle[0] != cbeSequence {
		t.Errorf("the bundle starts with %#02x, want %#02x; CertificateBundle is a SEQUENCE OF",
			bundle[0], cbeSequence)
	}
	var outer asn1.RawValue
	if _, err := asn1.Unmarshal(bundle, &outer); err != nil {
		t.Fatalf("the encoded bundle is not valid DER: %v", err)
	}
	if len(outer.Bytes) == 0 {
		t.Fatal("the encoded bundle carries no element")
	}
	if outer.Bytes[0] != cbeCertIdentifier {
		t.Errorf("a certificate element starts with %#02x, want %#02x; cert [0] under EXPLICIT "+
			"TAGS is a constructed context-specific 0, and an IMPLICIT or renumbered reading "+
			"produces bytes a conforming peer rejects", outer.Bytes[0], cbeCertIdentifier)
	}

	// Decode side. The bundle below is written byte by byte, so nothing about it comes from
	// certbundle.go: SEQUENCE { [0] { SEQUENCE {} } }.
	literal := []byte{cbeSequence, 0x04, cbeCertIdentifier, 0x02, 0x30, 0x00}
	certs, err := decodeCertBundle(literal)
	if err != nil {
		t.Fatalf("a bundle carrying the literal %#02x identifier was refused: %v", cbeCertIdentifier, err)
	}
	if len(certs) != 1 {
		t.Fatalf("the literal bundle decoded to %d certificates, want 1", len(certs))
	}

	// The CRL alternative is the literal 0xA1, and it contributes no certificate.
	crl := []byte{cbeSequence, 0x04, cbeCRLIdentifier, 0x02, 0x30, 0x00}
	if _, err := decodeCertBundle(crl); !errors.Is(err, errCertBundleEmpty) {
		t.Errorf("a bundle of one literal %#02x element returned %v, want errCertBundleEmpty; "+
			"crl [1] is a legal alternative that carries no certificate", cbeCRLIdentifier, err)
	}

	// An identifier outside the CHOICE is refused rather than read as a certificate.
	unknown := []byte{cbeSequence, 0x04, 0xA2, 0x02, 0x30, 0x00}
	if _, err := decodeCertBundle(unknown); err == nil {
		t.Error("a bundle whose element identifier is 0xA2 was accepted, and CertificateOrCRL " +
			"assigns only 0 and 1")
	}
}

// VALIDATES: certBundleMaxElements bounds the elements WALKED, so a bundle made only of
// CRL elements is refused at the same count as one made of certificates.
// PREVENTS: the cap counting certificates kept. The CRL arm of the CHOICE keeps nothing.
// A bundle of a million empty crl [1] elements therefore passed the count test on every
// iteration, and the loop unmarshalled all of them. That is the handshake time budget
// this bound exists to protect.
func TestCertBundleCapCountsElementsNotCertificates(t *testing.T) {
	crlOnly := make([]int, certBundleMaxElements+1)
	for i := range crlOnly {
		crlOnly[i] = certBundleTagCRL
	}
	_, err := decodeCertBundle(bundleOf(t, crlOnly))
	if !errors.Is(err, errCertBundleTooMany) {
		t.Fatalf("a CRL-only bundle past the cap was not refused by the cap: %v", err)
	}

	// The same shape one element under the cap reaches the no-certificate refusal.
	// That proves the cap fired above, and not the emptiness check.
	crlUnder := make([]int, certBundleMaxElements-1)
	for i := range crlUnder {
		crlUnder[i] = certBundleTagCRL
	}
	if _, err := decodeCertBundle(bundleOf(t, crlUnder)); !errors.Is(err, errCertBundleEmpty) {
		t.Fatalf("a CRL-only bundle under the cap was refused by something other than "+
			"the empty check: %v", err)
	}
}

// VALIDATES: a mixed bundle is bounded by its total element count, not by how many of
// them happen to carry a certificate.
// PREVENTS: padding a bundle with CRL elements to walk past the cap while keeping the
// certificate count small.
func TestCertBundleCapCountsMixedElements(t *testing.T) {
	mixed := make([]int, 0, certBundleMaxElements+1)
	mixed = append(mixed, certBundleTagCert)
	for len(mixed) < certBundleMaxElements+1 {
		mixed = append(mixed, certBundleTagCRL)
	}
	if _, err := decodeCertBundle(bundleOf(t, mixed)); !errors.Is(err, errCertBundleTooMany) {
		t.Fatalf("a bundle padded with CRL elements walked past the cap: %v", err)
	}
}
