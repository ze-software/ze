// RFC: rfc/short/rfc7427.md -- Section 3: Authentication Data is
// ASN.1 Length | AlgorithmIdentifier | Signature
//
// VALIDATES: the algorithm field Ze puts in a Digital Signature AUTH payload is a
// DER AlgorithmIdentifier (RFC 5280 Section 4.1.1.2), that is a SEQUENCE wrapping
// the signature OID, with NULL parameters for RSASSA-PKCS1-v1_5 (RFC 4055
// Section 5) and absent parameters for ECDSA (RFC 5758 Section 3.2).
// PREVENTS: the regression that shipped a BARE OID in that field. An
// AlgorithmIdentifier is a SEQUENCE by definition, so a bare OID is not one, and
// strongSwan refuses the payload with "digital signature authentication payload
// invalid". Ze's own verifier accepts both shapes (hashFromAlgID), so no
// Ze-to-Ze test could see the defect: it was only ever visible to another
// implementation.

package engine

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/rsa"
	"encoding/asn1"
	"testing"
)

// parseAlgorithmIdentifier asserts that der is a SEQUENCE and returns the OID it
// wraps plus whatever parameters follow it.
func parseAlgorithmIdentifier(t *testing.T, label string, der []byte) (asn1.ObjectIdentifier, []byte) {
	t.Helper()

	if len(der) == 0 {
		t.Fatalf("%s: empty algorithm identifier", label)
	}
	// 0x30 is the DER tag for SEQUENCE; 0x06 is OBJECT IDENTIFIER. The defect this
	// test exists to catch emitted the latter.
	if der[0] == 0x06 {
		t.Fatalf("%s: algorithm field is a bare OID (tag 0x06), not an AlgorithmIdentifier SEQUENCE: % x", label, der)
	}
	if der[0] != 0x30 {
		t.Fatalf("%s: algorithm field has DER tag 0x%02x, want 0x30 (SEQUENCE): % x", label, der[0], der)
	}

	var seq asn1.RawValue
	rest, err := asn1.Unmarshal(der, &seq)
	if err != nil {
		t.Fatalf("%s: algorithm identifier is not valid DER: %v", label, err)
	}
	if len(rest) != 0 {
		t.Fatalf("%s: %d trailing octets after the AlgorithmIdentifier", label, len(rest))
	}

	var oid asn1.ObjectIdentifier
	params, err := asn1.Unmarshal(seq.Bytes, &oid)
	if err != nil {
		t.Fatalf("%s: first element of the SEQUENCE is not an OID: %v", label, err)
	}
	return oid, params
}

// TestRFC7427AuthAlgorithmIdentifierIsWellFormed checks every algorithm
// selectSignatureAlgorithm can return, for both key types.
func TestRFC7427AuthAlgorithmIdentifierIsWellFormed(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	ecKey256, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		t.Fatalf("p256 key: %v", err)
	}
	ecKey384, err := ecdsa.GenerateKey(elliptic.P384(), crand.Reader)
	if err != nil {
		t.Fatalf("p384 key: %v", err)
	}

	cases := []struct {
		name string
		key  any
		// hashAlgos is the peer's SIGNATURE_HASH_ALGORITHMS list, which every
		// selection is drawn from. 2 is SHA-256, 3 SHA-384, 4 SHA-512 (RFC 7427
		// Section 4). Each case offers the one algorithm its key type and curve
		// signs with, because selectSignatureAlgorithm returns an error rather
		// than an identifier when the peer offered none.
		hashAlgos []uint16
		wantOID   asn1.ObjectIdentifier
		wantNull  bool
	}{
		{"rsa-sha256", rsaKey, []uint16{2}, asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 11}, true},
		{"rsa-sha384", rsaKey, []uint16{3}, asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 12}, true},
		{"rsa-sha512", rsaKey, []uint16{4}, asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 13}, true},
		{"ecdsa-p256", ecKey256, []uint16{2}, asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}, false},
		{"ecdsa-p384", ecKey384, []uint16{3}, asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 3}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			algID, _, err := selectSignatureAlgorithm(tc.key, tc.hashAlgos)
			if err != nil {
				t.Fatalf("selectSignatureAlgorithm: %v", err)
			}

			oid, params := parseAlgorithmIdentifier(t, tc.name, algID)
			if !oid.Equal(tc.wantOID) {
				t.Errorf("OID is %v, want %v", oid, tc.wantOID)
			}

			// RFC 4055 Section 5: RSASSA-PKCS1-v1_5 parameters MUST be present and
			// NULL. RFC 5758 Section 3.2: ECDSA parameters MUST be absent.
			if tc.wantNull && len(params) == 0 {
				t.Error("RSA AlgorithmIdentifier has no parameters, want an explicit DER NULL")
			}
			if tc.wantNull && len(params) != 0 && !bytes.Equal(params, asn1.NullBytes) {
				t.Errorf("RSA parameters are % x, want the DER NULL % x", params, asn1.NullBytes)
			}
			if !tc.wantNull && len(params) != 0 {
				t.Errorf("ECDSA AlgorithmIdentifier carries %d parameter octets, want none: % x", len(params), params)
			}

			// The length octet RFC 7427 Section 3 puts in front of the
			// AlgorithmIdentifier must be able to hold it.
			if len(algID) > 255 {
				t.Errorf("AlgorithmIdentifier is %d octets, which does not fit the single ASN.1 Length octet", len(algID))
			}
		})
	}
}
