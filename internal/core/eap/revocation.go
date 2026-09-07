// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS certificate revocation
// Detail: eap_tls.go -- the authenticator's tls.Config; peer.go -- the peer's
// RFC: rfc/short/rfc9190.md -- Section 5.4; rfc/short/rfc5216.md -- Section 5.4

package eap

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

// pemBlockCRL is the PEM label of a certificate revocation list. RFC 7468
// Section 6 fixes the spelling, so it is what every X.509 tool writes and what
// an operator pastes into the config.
const pemBlockCRL = "X509 CRL"

// errNoRevocationSource refuses an EAP-TLS 1.3 session that has no way to answer
// the question RFC 9190 Section 5.4 makes mandatory.
//
// The message names the remedy, because the operator who meets it configured a
// working certificate chain and is told the handshake failed for a reason no
// certificate carries.
var errNoRevocationSource = errors.New(
	"eap-tls: no certificate revocation list is configured, so the peer chain's revocation status cannot be checked, " +
		"and RFC 9190 Section 5.4 requires it on TLS 1.3. " +
		"Add a crl to the pki ca this peer validates against, or configure the peer for TLS 1.2")

// crlSet holds the certificate revocation lists one EAP-TLS session checks a
// presented chain against.
//
// An EMPTY set means the operator configured no revocation source, and that is a
// DIFFERENT state from a set holding a list that names no serial number. The
// empty set answers nothing. A configured list answers "this certificate is not
// revoked", which is what a CA publishes while it has withdrawn nothing. The two
// lead to opposite decisions on TLS 1.3, so nothing collapses them
// (ai/rules/principles.md).
type crlSet struct {
	lists []*x509.RevocationList
}

// configured says whether this session was given anything to check against.
//
// It is the question RFC 9190 Section 5.4 turns on, so it has a name rather than
// leaving each caller to read a length and remember what zero means there.
func (s crlSet) configured() bool { return len(s.lists) > 0 }

// parseCRLs reads a concatenation of PEM certificate revocation lists, in the
// form `cat a.crl b.crl` produces.
//
// It answers an empty set for empty input, which is the "nothing configured"
// state crlSet documents. Input that holds bytes but no usable list is an ERROR
// rather than that state: an operator who pasted something meant to configure a
// source, and answering "nothing configured" would hide the paste that failed.
//
// The loop is bounded by the input: pem.Decode consumes the block it returns and
// answers nil once no BEGIN line is left.
func parseCRLs(pemBytes []byte) (crlSet, error) {
	var set crlSet
	if len(pemBytes) == 0 {
		return set, nil
	}

	rest := pemBytes
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != pemBlockCRL {
			return crlSet{}, fmt.Errorf("eap-tls: revocation material holds a %q PEM block, want %q", block.Type, pemBlockCRL)
		}
		list, err := x509.ParseRevocationList(block.Bytes)
		if err != nil {
			return crlSet{}, fmt.Errorf("eap-tls: parse certificate revocation list: %w", err)
		}
		set.lists = append(set.lists, list)
	}

	if !set.configured() {
		return crlSet{}, fmt.Errorf("eap-tls: the configured revocation material holds no %q PEM block", pemBlockCRL)
	}
	return set, nil
}

// checkChainRevocation is the revocation gate both EAP-TLS roles run over the
// chains their TLS stack verified. It answers an error when the session must not
// complete, and crypto/tls turns that error into a fatal bad_certificate alert.
//
// RFC 9190 Section 5.4: "When EAP-TLS is used with TLS 1.3, the revocation
// status of all the certificates in the certificate chains MUST be checked
// (except the trust anchor)."
//
// The obligation binds BOTH ends. RFC 9190 amends RFC 5216 Section 5.4, whose
// own sentence is addressed to both ("EAP-TLS peer and server implementations
// MUST support the use of Certificate Revocation Lists (CRLs)"), and Section 5.4
// names the peer's checks and the server's stapling in the same breath. So the
// authenticator runs this over the client chain and the peer runs it over the
// authenticator chain, from the same function.
//
// EVERY verified chain is checked, not only the first. crypto/tls hands over
// each path it built, ze does not control which one a later consumer relies on,
// and "the certificate chains" is the RFC's own plural. A revoked certificate on
// any presented path refuses the session.
func checkChainRevocation(crls crlSet, chains [][]*x509.Certificate, version uint16, now time.Time) error {
	if !crls.configured() {
		// RFC 9190 Section 5.4 makes the check mandatory on TLS 1.3, and a
		// session with no source cannot perform it. An unchecked chain is what
		// the MUST forbids, so the handshake is refused rather than completed
		// on a status nobody read.
		if version >= tls.VersionTLS13 {
			return errNoRevocationSource
		}
		// RFC 5216 Section 5.4 governs TLS 1.2, and it asks that "EAP-TLS peer
		// and server implementations MUST support the use of Certificate
		// Revocation Lists (CRLs)". Supporting them is what this file is. It
		// does not oblige a session the operator configured none for.
		return nil
	}

	if len(chains) == 0 {
		// The chain is what the check is ABOUT, so an empty set is a failure to
		// check and never a clean result (ai/rules/principles.md).
		return errors.New("eap-tls: no verified certificate chain to check the revocation status of (RFC 9190 Section 5.4)")
	}

	for _, chain := range chains {
		if err := crls.checkChain(chain, now); err != nil {
			return err
		}
	}
	return nil
}

// checkChain asks the configured lists about every certificate on one verified
// chain except its trust anchor.
//
// A chain that x509.Certificate.Verify built runs leaf first and trust anchor
// last, so chain[i+1] issued chain[i] and the last element is the anchor
// Section 5.4 excepts. A chain of one element is that anchor on its own, and the
// loop correctly asks nothing about it.
func (s crlSet) checkChain(chain []*x509.Certificate, now time.Time) error {
	for i := 0; i+1 < len(chain); i++ {
		cert := chain[i]
		issuer := chain[i+1]

		list := s.currentListFrom(issuer, now)
		if list == nil {
			return fmt.Errorf(
				"eap-tls: no current certificate revocation list signed by %q, so the revocation status of %q cannot be checked "+
					"(RFC 9190 Section 5.4). Add that CA's CRL, and re-add it after its nextUpdate passes",
				issuer.Subject, cert.Subject)
		}

		entry, revoked := revocationEntry(list, cert)
		if revoked {
			return fmt.Errorf(
				"eap-tls: certificate %q (serial %s) was revoked by %q at %s (RFC 9190 Section 5.4)",
				cert.Subject, cert.SerialNumber, issuer.Subject, entry.RevocationTime.UTC().Format(time.RFC3339))
		}
	}
	return nil
}

// currentListFrom answers the configured revocation list that issuer signed and
// that still speaks for now, or nil when none does.
//
// Three properties are required together, and any one of them alone would let an
// unrelated document answer for this issuer. The list names the issuer as its own
// issuer. The issuer's key signed it, so a list an attacker substituted is not
// read. And its nextUpdate has not passed: RFC 5280 Section 5.1.2.5 defines that
// field as "the date by which the next CRL will be issued", so a list past it
// says nothing about the present. nextUpdate is optional in that section, and a
// list without one never goes stale.
func (s crlSet) currentListFrom(issuer *x509.Certificate, now time.Time) *x509.RevocationList {
	for _, list := range s.lists {
		if !bytes.Equal(list.RawIssuer, issuer.RawSubject) {
			continue
		}
		if err := list.CheckSignatureFrom(issuer); err != nil {
			continue
		}
		if !list.NextUpdate.IsZero() && now.After(list.NextUpdate) {
			continue
		}
		return list
	}
	return nil
}

// revocationEntry answers the list's entry for a certificate, and whether the
// list carries one.
//
// The serial number is the identity RFC 5280 Section 5.1.2.6 revokes by, and it
// is unique for one issuer, which is why the caller has already matched the list
// to the certificate's issuer. An entry with no serial number cannot name a
// certificate and is skipped rather than dereferenced: the field is required by
// that section, so a list carrying one is malformed, and a malformed config file
// must not stop the daemon.
func revocationEntry(list *x509.RevocationList, cert *x509.Certificate) (x509.RevocationListEntry, bool) {
	for _, entry := range list.RevokedCertificateEntries {
		if entry.SerialNumber == nil {
			continue
		}
		if entry.SerialNumber.Cmp(cert.SerialNumber) == 0 {
			return entry, true
		}
	}
	return x509.RevocationListEntry{}, false
}
