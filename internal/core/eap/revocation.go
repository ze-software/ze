// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS certificate revocation
// Detail: eap_tls.go -- the authenticator's tls.Config; peer.go -- the peer's
// Related: ../../component/ike/engine/remote_cert_checks.go -- the IKE peer chain check that calls VerifyChainsNotRevoked
// RFC: rfc/short/rfc9190.md -- Section 5.4; rfc/short/rfc5216.md -- Section 5.4
// RFC: rfc/short/rfc5280.md -- Sections 5.2, 5.2.5, 5.3 and 6.1.3

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
			return crlSet{}, fmt.Errorf("revocation material holds a %q PEM block, want %q", block.Type, pemBlockCRL)
		}
		list, err := x509.ParseRevocationList(block.Bytes)
		if err != nil {
			return crlSet{}, fmt.Errorf("parse certificate revocation list: %w", err)
		}
		set.lists = append(set.lists, list)
	}

	if !set.configured() {
		return crlSet{}, fmt.Errorf("the configured revocation material holds no %q PEM block", pemBlockCRL)
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
			// checkChain speaks for no protocol, because IKE reaches it too
			// (VerifyChainsNotRevoked). The role and the obligation it
			// discharges are this caller's to name.
			return fmt.Errorf("eap-tls: %w (RFC 9190 Section 5.4)", err)
		}
	}
	return nil
}

// VerifyChainsNotRevoked checks every certificate on every chain, the trust
// anchor of each excepted, against the revocation lists in crlPEM, and answers
// an error naming the first certificate that is revoked or whose status the
// lists cannot answer.
//
// It is the entry point for a relying party outside EAP-TLS, the IKE engine's
// peer certificate check, and it implements RFC 5280 Section 6.1.3 (a)(3) for
// that caller: "At the current time, the certificate is not revoked." It runs
// the same per-certificate judgement checkChainRevocation runs for EAP-TLS, so
// a list one protocol refuses the other refuses too.
//
// It never answers "not revoked" for a question it could not ask. Empty
// material, material with no list in it, and an empty set of chains are each an
// error: the caller decides whether a missing source is acceptable BEFORE it
// calls, and a nil from here means every certificate was checked.
//
// chains are leaf first and trust anchor last, as x509.Certificate.Verify
// returns them. Safe for concurrent use: it reads its arguments only.
func VerifyChainsNotRevoked(crlPEM []byte, chains [][]*x509.Certificate, now time.Time) error {
	crls, err := parseCRLs(crlPEM)
	if err != nil {
		return err
	}
	if !crls.configured() {
		return errors.New(
			"certificate revocation: no revocation list was given, so the revocation status of the chain cannot be checked. " +
				"Add the issuing CA's CRL to the pki ca that holds that CA")
	}
	if len(chains) == 0 {
		// The chain is what the check is ABOUT, so an empty set is a failure to
		// check and never a clean result (ai/rules/principles.md).
		return errors.New("certificate revocation: no verified certificate chain to check the revocation status of")
	}
	for _, chain := range chains {
		// RFC 5280 Section 6.1.3
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
//
// Its messages name no protocol and cite no protocol RFC: EAP-TLS
// (checkChainRevocation) and IKE (VerifyChainsNotRevoked) both reach it, and
// each caller adds its own role and the obligation it discharges.
func (s crlSet) checkChain(chain []*x509.Certificate, now time.Time) error {
	for i := 0; i+1 < len(chain); i++ {
		cert := chain[i]
		issuer := chain[i+1]

		list, err := s.currentListFrom(issuer, now)
		if err != nil {
			return fmt.Errorf(
				"no current certificate revocation list signed by %q, so the revocation status of certificate %q (serial %s) is unknown: %w",
				issuer.Subject, cert.Subject, cert.SerialNumber, err)
		}

		entry, revoked := revocationEntry(list, cert)
		if revoked {
			return fmt.Errorf(
				"certificate %q (serial %s) was revoked by %q at %s, %s. Issue that party a new certificate",
				cert.Subject, cert.SerialNumber, issuer.Subject, entry.RevocationTime.UTC().Format(time.RFC3339), crlName(list))
		}
	}
	return nil
}

// errNoListNamesIssuer is why an issuer's status cannot be read when no
// configured list names it as its issuer at all.
var errNoListNamesIssuer = errors.New(
	"no configured list names that CA as its issuer. Add that CA's CRL to the pki ca that holds it, " +
		"adding the CA as a pki ca with its crl when no entry holds it yet, and re-add the list after its nextUpdate passes")

// currentListFrom answers the configured revocation list that issuer signed,
// that still speaks for now, and that Ze can read in full. When no list does,
// it answers why, so the refusal names what the operator must change.
//
// Four properties are required together, and any one of them alone would let an
// unrelated or partly read document answer for this issuer. The list names the
// issuer as its own issuer. The issuer's key signed it, so a list an attacker
// substituted is not read. Its nextUpdate has not passed: RFC 5280 Section
// 5.1.2.5 defines that field as "the date by which the next CRL will be
// issued", so a list past it says nothing about the present. nextUpdate is
// optional in that section, and a list without one never goes stale. And it
// carries no critical extension, at list level or on any entry, because Ze
// processes none (criticalExtension).
//
// When several of the issuer's lists fail, the reason given is the one from the
// list that came closest to answering: a list that failed only on a critical
// extension says more about the fix than one signed by another key.
func (s crlSet) currentListFrom(issuer *x509.Certificate, now time.Time) (*x509.RevocationList, error) {
	unusable := errNoListNamesIssuer
	closest := 0
	for _, list := range s.lists {
		if !bytes.Equal(list.RawIssuer, issuer.RawSubject) {
			continue
		}
		if err := list.CheckSignatureFrom(issuer); err != nil {
			if closest < 1 {
				closest = 1
				unusable = fmt.Errorf(
					"a list naming that CA as its issuer is not signed by its key (%w). Add the CRL that CA itself published",
					err)
			}
			continue
		}
		if !list.NextUpdate.IsZero() && now.After(list.NextUpdate) {
			if closest < 2 {
				closest = 2
				unusable = fmt.Errorf(
					"its %s passed its nextUpdate at %s. Publish a fresh CRL from that CA and add it to the pki ca that holds it",
					crlName(list), list.NextUpdate.UTC().Format(time.RFC3339))
			}
			continue
		}
		// RFC 5280 Section 5.2: "If a CRL contains a critical extension that
		// the application cannot process, then the application MUST NOT use
		// that CRL to determine the status of certificates."
		// RFC 5280 Section 5.3: "If a CRL contains a critical CRL entry
		// extension that the application cannot process, then the application
		// MUST NOT use that CRL to determine the status of any certificates."
		// RFC 5280 Section 5.2.5: "However, implementations that do not support
		// this extension MUST either treat the status of any certificate not
		// listed on this CRL as unknown or locate another CRL that does not
		// contain any unrecognized critical extensions."
		// Ze processes no CRL or entry extension that may be critical
		// (issuingDistributionPoint 2.5.29.28, deltaCRLIndicator 2.5.29.27 and
		// certificateIssuer 2.5.29.29 included), and crypto/x509 records them
		// without judging the flag. So such a list is skipped, the next list
		// from the issuer is located by this loop, and when none answers the
		// status is unknown, which the caller refuses.
		if where, critical := criticalExtension(list); critical {
			if closest < 3 {
				closest = 3
				unusable = fmt.Errorf(
					"its %s carries the critical %s, which Ze does not process, so RFC 5280 Section 5.2 forbids using that list. "+
						"Publish a complete CRL without a delta CRL indicator or issuing distribution point and add it to the pki ca that holds that CA",
					crlName(list), where)
			}
			continue
		}
		return list, nil
	}
	return nil, unusable
}

// criticalExtension answers the first extension of a list, at list level or on
// one of its entries, that is marked critical, described for an operator, and
// whether there is one.
//
// Every critical extension counts, whatever its OID, because Ze implements the
// processing of none: crypto/x509 reads the CRL number and the authority key
// identifier for its own use and records the rest unjudged
// (x509.ParseRevocationList), and nothing in Ze reads them further. A
// non-critical extension is ignored, which RFC 5280 Sections 5.2 and 5.3 allow.
func criticalExtension(list *x509.RevocationList) (string, bool) {
	for _, ext := range list.Extensions {
		if ext.Critical {
			return "CRL extension " + ext.Id.String(), true
		}
	}
	for _, entry := range list.RevokedCertificateEntries {
		for _, ext := range entry.Extensions {
			if !ext.Critical {
				continue
			}
			serial := "with no serial number"
			if entry.SerialNumber != nil {
				serial = "for serial " + entry.SerialNumber.String()
			}
			return "CRL entry extension " + ext.Id.String() + " on the entry " + serial, true
		}
	}
	return "", false
}

// crlName names a list for an operator: its issuer is named by the caller, so
// this adds the CRL number RFC 5280 Section 5.2.3 gives each list, which tells
// two lists from the same issuer apart. A list without the extension is named
// as such rather than as number zero.
func crlName(list *x509.RevocationList) string {
	if list.Number == nil {
		return "CRL with no CRL number"
	}
	return "CRL number " + list.Number.String()
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
