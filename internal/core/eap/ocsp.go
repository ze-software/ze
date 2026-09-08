// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS certificate status
// Detail: revocation.go -- the CRL walk this sits beside; peer_chain.go -- the peer's caller
// RFC: rfc/short/rfc9190.md -- Section 5.4; rfc/short/rfc5216.md -- Section 5.4

package eap

import (
	"crypto/x509"
	"errors"
	"fmt"
	"slices"
	"time"

	"golang.org/x/crypto/ocsp"
)

// errNoStapledStatus refuses a chain whose leaf CertificateEntry carried no
// CertificateStatus extension, on a peer that asked for one.
//
// RFC 9190 Section 5.4: "When an EAP-TLS peer uses Certificate Status Requests
// to check the revocation status of the EAP-TLS server's certificate chain, it
// MUST treat a CertificateEntry (but not the trust anchor) without a valid
// CertificateStatus extension as invalid and abort the handshake with an
// appropriate alert."
//
// The message names the two remedies, because the operator who meets it
// configured a working certificate chain and is told the handshake failed for a
// reason no certificate carries.
var errNoStapledStatus = errors.New(
	"eap-tls: the authenticator stapled no OCSP response, and RFC 9190 Section 5.4 requires a valid " +
		"CertificateStatus on every CertificateEntry except the trust anchor once the peer asks for one. " +
		"Configure an ocsp-response on the authenticator's pki certificate, " +
		"or set certificate-status-request false on this peer to check revocation by crl alone")

// CheckCertificateStatus judges one OCSP response about one certificate, and
// answers an error when the certificate MUST NOT be relied on.
//
// It is the one judge of an OCSP response in Ze, and it has two callers with
// two sources: the peer's stapled response, which arrives inside the TLS
// handshake (checkStapledChainStatus below), and the response the post-
// authentication check fetches from the responder once the tunnel is up
// (engine/postauth_revocation.go). The judgement is identical, so it is written
// once (ai/rules/principles.md).
//
// FOUR PROPERTIES ARE REQUIRED TOGETHER, and any one of them alone would let an
// unrelated or stale document answer for this certificate.
//
// The response is signed by the certificate's own issuer, or by a responder that
// issuer delegated to, and it names this certificate's serial number.
// ocsp.ParseResponseForCert checks both.
//
// A delegated responder carries the OCSP signing extended key usage. RFC 6960
// Section 4.2.2.2: the CA "MUST also include a value of id-kp-OCSPSigning in an
// extended key usage extension" of the certificate it delegates to. x/crypto's
// parser checks that the issuer signed the embedded certificate and does not
// check what it was signed FOR, so any leaf that issuer ever signed would
// otherwise be able to answer for its siblings.
//
// The response speaks for now. RFC 6960 Section 2.4: "The response indicates ...
// thisUpdate ... nextUpdate", and Section 3.2 makes the time at which the status
// was known part of what the client accepts. A thisUpdate in the future and a
// passed nextUpdate each say the response is not about the present. nextUpdate
// is optional in Section 4.2.2.1, and a response without one never goes stale,
// which is how crlSet.currentListFrom treats a CRL with no nextUpdate.
//
// And the status is good. Revoked and unknown are both refusals: "unknown" says
// the responder cannot answer for this certificate, which is not a statement
// that it is valid.
func CheckCertificateStatus(der []byte, cert, issuer *x509.Certificate, now time.Time) error {
	resp, err := ocsp.ParseResponseForCert(der, cert, issuer)
	if err != nil {
		return fmt.Errorf("eap-tls: the OCSP response for %q is not usable: %w (RFC 9190 Section 5.4)", cert.Subject, err)
	}

	if resp.Certificate != nil && !delegatedOCSPSigner(resp.Certificate) {
		return fmt.Errorf(
			"eap-tls: the OCSP response for %q was signed by %q, which %q did not delegate to: "+
				"it carries no OCSP signing extended key usage (RFC 6960 Section 4.2.2.2)",
			cert.Subject, resp.Certificate.Subject, issuer.Subject)
	}

	if resp.ThisUpdate.After(now) {
		return fmt.Errorf(
			"eap-tls: the OCSP response for %q is dated %s, which is in the future, so it says nothing about now "+
				"(RFC 6960 Section 2.4)",
			cert.Subject, resp.ThisUpdate.UTC().Format(time.RFC3339))
	}
	if !resp.NextUpdate.IsZero() && now.After(resp.NextUpdate) {
		return fmt.Errorf(
			"eap-tls: the OCSP response for %q expired at %s, so it says nothing about now "+
				"(RFC 6960 Section 4.2.2.1). Fetch a fresh response from the responder",
			cert.Subject, resp.NextUpdate.UTC().Format(time.RFC3339))
	}

	switch resp.Status {
	case ocsp.Good:
		return nil
	case ocsp.Revoked:
		return fmt.Errorf(
			"eap-tls: certificate %q (serial %s) was revoked at %s, as reported by the OCSP responder "+
				"(RFC 9190 Section 5.4)",
			cert.Subject, cert.SerialNumber, resp.RevokedAt.UTC().Format(time.RFC3339))
	default:
		return fmt.Errorf(
			"eap-tls: the OCSP responder answered \"unknown\" for %q (serial %s), so its revocation status is "+
				"not known and it MUST NOT be relied on (RFC 6960 Section 2.2)",
			cert.Subject, cert.SerialNumber)
	}
}

// delegatedOCSPSigner reports whether this certificate is one a CA delegated the
// answering of OCSP requests to.
//
// RFC 6960 Section 4.2.2.2 names the extended key usage that says so, and Go
// parses it into ExtKeyUsage, which x509 read from a certificate whose length is
// already checked.
func delegatedOCSPSigner(cert *x509.Certificate) bool {
	return slices.Contains(cert.ExtKeyUsage, x509.ExtKeyUsageOCSPSigning)
}

// checkStapledChainStatus is the gate a ze EAP-TLS peer runs when the operator
// turned Certificate Status Requests on, over the chains it verified.
//
// RFC 9190 Section 5.4: "When an EAP-TLS peer uses Certificate Status Requests
// to check the revocation status of the EAP-TLS server's certificate chain, it
// MUST treat a CertificateEntry (but not the trust anchor) without a valid
// CertificateStatus extension as invalid and abort the handshake with an
// appropriate alert."
//
// The abort is the error return: crypto/tls turns a non-nil VerifyConnection
// error into a fatal bad_certificate alert (Conn.verifyServerCertificate,
// crypto/tls/handshake_client.go), which is the appropriate alert for a
// certificate the peer refuses to accept.
//
// ONE STAPLE COVERS THE LEAF ALONE, and that is a property of crypto/tls rather
// than of the RFC. Section 4.4.2.1 of RFC 8446 carries the OCSP information in
// each CertificateEntry, so a conformant server can answer for every certificate
// on the chain. Go's unmarshalCertificate (crypto/tls/handshake_messages.go)
// skips the extensions of every entry after the first -- "This library only
// supports OCSP and SCT for leaf certificates" -- so ze can read a
// CertificateStatus for the leaf and for nothing else. An intermediate below the
// trust anchor is therefore a CertificateEntry whose status ze cannot see, and
// the sentence above says what to do with one: treat it as invalid. Failing
// closed is also the only safe reading, because "no status visible" and "no
// status sent" are the same bytes to this role (ai/rules/principles.md).
func checkStapledChainStatus(chains [][]*x509.Certificate, staple []byte, now time.Time) error {
	if len(chains) == 0 {
		// The chain is what the check is ABOUT, so an empty set is a failure to
		// check and never a clean result (ai/rules/principles.md).
		return errors.New("eap-tls: no verified certificate chain to check the certificate status of (RFC 9190 Section 5.4)")
	}

	for _, chain := range chains {
		if err := checkStapledChain(chain, staple, now); err != nil {
			return err
		}
	}
	return nil
}

// checkStapledChain applies the Section 5.4 status rule to one verified chain.
//
// A chain that x509.Certificate.Verify built runs leaf first and trust anchor
// last, so chain[i+1] issued chain[i] and the last element is the anchor
// Section 5.4 excepts. A chain of one element is that anchor on its own, and the
// loop correctly asks nothing about it, exactly as crlSet.checkChain does.
func checkStapledChain(chain []*x509.Certificate, staple []byte, now time.Time) error {
	for i := 0; i+1 < len(chain); i++ {
		cert := chain[i]
		issuer := chain[i+1]

		if i > 0 {
			return fmt.Errorf(
				"eap-tls: the authenticator's chain carries the intermediate %q, whose CertificateStatus this "+
					"peer cannot read, so RFC 9190 Section 5.4 makes it invalid. Issue the authenticator a "+
					"certificate its trust anchor signed directly, or set certificate-status-request false on "+
					"this peer to check revocation by crl alone",
				cert.Subject)
		}

		if len(staple) == 0 {
			return errNoStapledStatus
		}
		if err := CheckCertificateStatus(staple, cert, issuer, now); err != nil {
			return err
		}
	}
	return nil
}
