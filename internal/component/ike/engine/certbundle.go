// Design: plan/learned/739-ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// RFC: rfc/short/rfc7296.md -- the X.509 bundle ASN.1 for Hash and URL (Section 3.6)
// Related: certurl.go -- the bounded fetcher that retrieves a bundle named by a URL
// Related: auth.go -- the CERT payload assembly that sends a bundle hash

package engine

import (
	"encoding/asn1"
	"errors"
	"fmt"
)

// The X.509 bundle carried by the "Hash and URL of X.509 bundle" encoding (13).
//
// RFC 7296 Section 3.6 gives the ASN.1 module verbatim:
//
//	DEFINITIONS EXPLICIT TAGS ::=
//	CertificateOrCRL ::= CHOICE { cert [0] Certificate, crl [1] CertificateList }
//	CertificateBundle ::= SEQUENCE OF CertificateOrCRL
//
// EXPLICIT TAGS is the load-bearing word. Each alternative wraps its value in a
// constructed context-specific tag. It does not replace the value's own tag. A
// certificate element is therefore A0 <len> <the whole Certificate SEQUENCE>, not a
// Certificate whose leading 0x30 has been overwritten. An IMPLICIT reading produces
// bytes strongSwan and every other conforming peer reject.
const (
	certBundleTagCert = 0 // cert [0] Certificate
	certBundleTagCRL  = 1 // crl  [1] CertificateList
)

// certBundleMaxElements bounds a bundle parsed from the network. The chain limit an
// operator configures bounds what ze then USES. This constant bounds what ze WALKS
// before that limit applies. As a result, a fetched bundle holding a million empty
// elements cannot spend the handshake's time budget.
//
// It counts every element of the CertificateOrCRL SEQUENCE, both alternatives. A CRL
// element costs the same unmarshal as a certificate element. But it adds no certificate,
// so a count of certificates does not see it.
const certBundleMaxElements = 64

var (
	errCertBundleTrailing = errors.New("ike cert-bundle: trailing bytes after the bundle")
	errCertBundleEmpty    = errors.New("ike cert-bundle: the bundle carries no certificate")
	errCertBundleTooMany  = errors.New("ike cert-bundle: the bundle carries too many elements")
)

// encodeCertBundle builds the DER CertificateBundle for a certificate chain, in the order
// given. RFC 7296 Section 3.6 requires the certificate holding the AUTH key first, and
// the caller passes it first.
func encodeCertBundle(chain [][]byte) ([]byte, error) {
	var elements []byte
	for _, der := range chain {
		element, err := asn1.Marshal(asn1.RawValue{
			Class:      asn1.ClassContextSpecific,
			Tag:        certBundleTagCert,
			IsCompound: true,
			Bytes:      der,
		})
		if err != nil {
			return nil, fmt.Errorf("ike cert-bundle: encode element: %w", err)
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
		return nil, fmt.Errorf("ike cert-bundle: encode bundle: %w", err)
	}
	return bundle, nil
}

// decodeCertBundle returns the DER of every certificate in a CertificateBundle, in bundle
// order. CRL elements are SKIPPED rather than refused. RFC 7296 Section 3.6 puts them in
// the same CHOICE, and a peer that includes one has not malformed the bundle.
//
// Nothing here parses a certificate. The elements are returned as DER. The caller runs
// them through x509 exactly as it runs an inline CERT payload. A bundle therefore reaches
// no parser that an ordinary certificate chain does not.
func decodeCertBundle(der []byte) ([][]byte, error) {
	var outer asn1.RawValue
	rest, err := asn1.Unmarshal(der, &outer)
	if err != nil {
		return nil, fmt.Errorf("ike cert-bundle: parse bundle: %w", err)
	}
	if len(rest) != 0 {
		return nil, errCertBundleTrailing
	}
	if outer.Class != asn1.ClassUniversal || outer.Tag != asn1.TagSequence || !outer.IsCompound {
		return nil, fmt.Errorf(
			"ike cert-bundle: the bundle is not a SEQUENCE (class %d, tag %d)",
			outer.Class, outer.Tag)
	}

	var certs [][]byte
	body := outer.Bytes
	// The cap counts ELEMENTS walked, not certificates kept. The CRL arm below keeps
	// nothing, so a count of certificates left a CRL-only bundle unbounded. A million
	// empty crl [1] elements passed the test on every iteration, and the loop
	// unmarshalled all of them. That is the time budget this bound exists to protect
	// (ai/rules/fail-closed-guards.md).
	for elements := 0; len(body) > 0; elements++ {
		if elements >= certBundleMaxElements {
			return nil, fmt.Errorf("%w: more than %d", errCertBundleTooMany, certBundleMaxElements)
		}
		var element asn1.RawValue
		body, err = asn1.Unmarshal(body, &element)
		if err != nil {
			return nil, fmt.Errorf("ike cert-bundle: parse element: %w", err)
		}
		if element.Class != asn1.ClassContextSpecific {
			return nil, fmt.Errorf(
				"ike cert-bundle: element is not a CertificateOrCRL choice (class %d)", element.Class)
		}
		switch element.Tag {
		case certBundleTagCert:
			certs = append(certs, element.Bytes)
		case certBundleTagCRL:
			// A revocation list is a legal alternative and carries no certificate.
		default:
			return nil, fmt.Errorf(
				"ike cert-bundle: element names choice %d, and CertificateOrCRL assigns 0 and 1",
				element.Tag)
		}
	}
	if len(certs) == 0 {
		return nil, errCertBundleEmpty
	}
	return certs, nil
}
