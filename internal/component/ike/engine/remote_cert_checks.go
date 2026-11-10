// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- IKEv2 certificate payload handling
// Overview: cert_payload.go -- getRemoteCert, which runs these checks after Verify
// Related: ../../../core/eap/revocation.go -- VerifyChainsNotRevoked, the CRL judgement EAP-TLS shares
// RFC: rfc/short/rfc5280.md -- Sections 4.2.1.12 and 6.1.3

package engine

import (
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/eap"
)

// Object identifiers the IKE purpose check reads. crypto/x509 exposes neither
// extension's presence, only its decoded bits, so presence is read by OID.
var (
	// oidExtensionKeyUsage is id-ce-keyUsage (RFC 5280 Section 4.2.1.3).
	oidExtensionKeyUsage = asn1.ObjectIdentifier{2, 5, 29, 15}
	// oidExtensionExtKeyUsage is id-ce-extKeyUsage (RFC 5280 Section 4.2.1.12).
	oidExtensionExtKeyUsage = asn1.ObjectIdentifier{2, 5, 29, 37}
	// oidKeyPurposeAny is anyExtendedKeyUsage (RFC 5280 Section 4.2.1.12).
	oidKeyPurposeAny = asn1.ObjectIdentifier{2, 5, 29, 37, 0}
	// oidKeyPurposeIPsecIKE is id-kp-ipsecIKE (RFC 4945 Section 5.1.3.12).
	oidKeyPurposeIPsecIKE = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 17}
)

// checkIKEPurpose refuses a peer end-entity certificate whose key usage or
// extended key usage excludes signing an IKE AUTH payload.
//
// RFC 5280 Section 4.2.1.12: "If a certificate contains both a key usage
// extension and an extended key usage extension, then both extensions MUST be
// processed independently and the certificate MUST only be used for a purpose
// consistent with both extensions."
//
// The two extensions are judged here one after the other, each on its own
// terms, and the certificate is used for IKE only when neither excludes it.
// What "consistent with IKE" means for each is RFC 4945's rule for an IKE peer
// certificate, quoted at each test. Presence is read from the extension OID,
// never from a zero bit field: a key usage extension that sets no bit is
// present and refuses, while an absent one admits.
//
// x509.Certificate.Verify cannot run this check: Go has no ExtKeyUsage value
// for id-kp-ipsecIKE, and it never reads the leaf's key usage bits.
func checkIKEPurpose(cert *x509.Certificate) error {
	if hasCertExtension(cert, oidExtensionKeyUsage) {
		// RFC 4945 Section 5.1.3.2: "Since we are talking about using the
		// public key to validate a signature, if the KeyUsage extension is
		// present, then at least one of the digitalSignature or the
		// nonRepudiation bits in the KeyUsage extension MUST be set (both can
		// be set as well)."
		// x509.KeyUsageContentCommitment is the nonRepudiation bit, renamed by
		// X.509 (RFC 5280 Section 4.2.1.3).
		signing := x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment
		if cert.KeyUsage&signing == 0 {
			return errors.New(
				"its key usage extension sets neither digitalSignature nor nonRepudiation, so its key may not " +
					"sign an IKE AUTH payload (RFC 4945 Section 5.1.3.2). Reissue it with digitalSignature in " +
					"key usage, or without the key usage extension")
		}
	}

	ekuValue, present := certExtensionValue(cert, oidExtensionExtKeyUsage)
	if !present {
		// RFC 4945 Section 5.1.3.12: "If no EKU extension, continue."
		return nil
	}
	var purposes []asn1.ObjectIdentifier
	rest, err := asn1.Unmarshal(ekuValue, &purposes)
	if err != nil {
		return fmt.Errorf("its extended key usage extension does not decode (%w). Reissue the certificate", err)
	}
	if len(rest) != 0 {
		return errors.New("its extended key usage extension carries trailing data after the purpose list. Reissue the certificate")
	}
	// RFC 4945 Section 5.1.3.12: "Implementations that do support EKU MUST
	// support the following logic for certificate validation:"
	// "If EKU present AND contains either id-kp-ipsecIKE or
	// anyExtendedKeyUsage, continue."
	// "Otherwise, reject cert."
	for _, purpose := range purposes {
		if purpose.Equal(oidKeyPurposeIPsecIKE) {
			return nil
		}
		if purpose.Equal(oidKeyPurposeAny) {
			return nil
		}
	}
	return fmt.Errorf(
		"its extended key usage names %s and neither id-kp-ipsecIKE nor anyExtendedKeyUsage, so it is not issued "+
			"for IKE (RFC 4945 Section 5.1.3.12). Reissue it with id-kp-ipsecIKE (1.3.6.1.5.5.7.3.17) or "+
			"anyExtendedKeyUsage (2.5.29.37.0) among its extended key usages, or without the extended key usage extension",
		purposeList(purposes))
}

// purposeList names the key purposes an extended key usage extension carried,
// by OID, for the refusal. An empty extension is named as such rather than as
// an empty string.
func purposeList(purposes []asn1.ObjectIdentifier) string {
	if len(purposes) == 0 {
		return "no purpose"
	}
	names := make([]string, 0, len(purposes))
	for _, purpose := range purposes {
		names = append(names, purpose.String())
	}
	return strings.Join(names, ", ")
}

// hasCertExtension says whether the certificate carries the extension, critical
// or not.
func hasCertExtension(cert *x509.Certificate, oid asn1.ObjectIdentifier) bool {
	_, present := certExtensionValue(cert, oid)
	return present
}

// certExtensionValue answers the DER value of the certificate's extension with
// that OID, and whether it carries one. RFC 5280 Section 4.2 lets a certificate
// carry each extension once, and crypto/x509 refuses a duplicate at parse, so
// the first match is the only one.
func certExtensionValue(cert *x509.Certificate, oid asn1.ObjectIdentifier) ([]byte, bool) {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oid) {
			return ext.Value, true
		}
	}
	return nil, false
}

// checkPeerChainRevocation refuses a peer whose verified chains carry a revoked
// certificate, or one whose status the configured lists cannot answer.
//
// The revocation source is the crl leaf-list of every pki ca entry whose
// certificate sits on a verified chain, as the anchor or as an intermediate.
// IKE intermediates arrive in the peer's CERT payloads, so an operator who wants
// an intermediate's revocations read adds that intermediate as a pki ca with its
// list. Once any such list exists the check is strict over every chain, the
// same rule EAP-TLS applies: a certificate on it with no current list from its
// issuer is refused, never passed.
//
// With no list anywhere on the chain the peer authenticates, as strongSwan's
// default revocation policy and EAP-TLS on TLS 1.2 do, and a Warn says the
// status was not checked: silence would read as a check that passed.
func checkPeerChainRevocation(sa *SA, caName string, chains [][]*x509.Certificate) error {
	crlPEM, holders := chainRevocationSource(chains)
	if len(crlPEM) == 0 {
		getLogger().Warn(
			"ike: the revocation status of the peer certificate is not checked, because no pki ca on its verified chain holds a crl",
			"peer", sa.PeerName,
			"ca-certificate", caName,
			"fix", "add the issuing CA's revocation list to pki ca "+caName+" crl, "+
				"and add each intermediate CA as a pki ca with its own crl")
		return nil
	}

	// RFC 5280 Section 6.1.3: "The certificate MUST satisfy each of the
	// following:", condition (a)(3): "At the current time, the certificate is
	// not revoked."
	if err := eap.VerifyChainsNotRevoked(crlPEM, chains, time.Now()); err != nil {
		return fmt.Errorf("ike auth: peer %q certificate chain refused, judged against the crl of pki ca %s: %w",
			sa.PeerName, strings.Join(holders, ", "), err)
	}
	return nil
}

// chainRevocationSource gathers the revocation lists of every pki ca entry
// whose CA (subject and key, pki.CAEntriesFor) is on one of the chains, and the names of the entries that
// supplied any. Each entry is read once however many chains pass through it.
//
// The leaf, chain[0], is skipped: it is the peer, not a CA, and a pki ca entry
// holding it would be an operator error that grants no revocation authority.
func chainRevocationSource(chains [][]*x509.Certificate) ([]byte, []string) {
	var crlPEM []byte
	var holders []string
	seen := make(map[*pki.CACertEntry]bool)
	for _, chain := range chains {
		for i := 1; i < len(chain); i++ {
			for _, entry := range pki.CAEntriesFor(chain[i]) {
				if seen[entry] {
					continue
				}
				seen[entry] = true
				lists := entry.CRLPEM()
				if len(lists) == 0 {
					continue
				}
				crlPEM = append(crlPEM, lists...)
				holders = append(holders, entry.Name)
			}
		}
	}
	return crlPEM, holders
}
