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
