// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- remote identity policy
// Related: auth.go -- AUTH verification, which calls both checks below
// RFC: rfc/short/rfc7296.md -- Identification payloads (Section 3.5)
package engine

import (
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// oidSubjectAltName is the subject alternative name extension, RFC 5280 Section 4.2.1.6.
var oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}

// idTypeName names an IKE identity type for an operator who reads a refusal. RFC 7296
// Section 3.5 gives the values. An unassigned value prints as its number.
func idTypeName(idType uint8) string {
	switch idType {
	case wire.IDTypeIPv4Addr:
		return "ID_IPV4_ADDR"
	case wire.IDTypeFQDN:
		return "ID_FQDN"
	case wire.IDTypeRFC822Addr:
		return "ID_RFC822_ADDR"
	case wire.IDTypeIPv6Addr:
		return "ID_IPV6_ADDR"
	case wire.IDTypeDERASN1DN:
		return "ID_DER_ASN1_DN"
	case wire.IDTypeDERASN1GN:
		return "ID_DER_ASN1_GN"
	case wire.IDTypeKeyID:
		return "ID_KEY_ID"
	}
	var b textbuf.Buffer
	return b.Str("ID type ").Uint8(idType).String()
}

// asciiLower folds one ASCII letter to lower case and leaves every other octet alone.
func asciiLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// asciiEqualFold compares two strings with the ASCII letters folded.
//
// RFC 7296 Section 3.5 makes ID_FQDN ASCII, and a domain name is case-insensitive, so
// the fold belongs here. It is deliberately narrower than strings.EqualFold, which folds
// by Unicode rules. Unicode folding maps the Kelvin sign onto the letter k, so two
// different octet strings would read as one identity.
func asciiEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if asciiLower(a[i]) != asciiLower(b[i]) {
			return false
		}
	}
	return true
}

// assertedAddr reads an address identity out of an ID payload. RFC 7296 Section 3.5
// gives ID_IPV4_ADDR four octets and ID_IPV6_ADDR sixteen, so any other length is
// malformed. The result is unmapped, which makes one IPv4 address compare equal however
// the peer encoded it.
func assertedAddr(p *wire.PayloadID) (netip.Addr, bool) {
	switch p.IDType {
	case wire.IDTypeIPv4Addr:
		if len(p.IDData) != 4 {
			return netip.Addr{}, false
		}
	case wire.IDTypeIPv6Addr:
		if len(p.IDData) != 16 {
			return netip.Addr{}, false
		}
	default:
		return netip.Addr{}, false
	}
	addr, ok := netip.AddrFromSlice(p.IDData)
	if !ok {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// opaqueText renders an opaque identity. Printable ASCII prints as itself, because that
// is how an operator writes it in remote-id. Anything else prints as hex.
func opaqueText(data []byte) string {
	for _, c := range data {
		if c < 0x20 || c > 0x7e {
			var b textbuf.Buffer
			return b.Str("0x").Hex(data).String()
		}
	}
	return string(data)
}

// assertedIdentity renders the identity a peer asserted, and reports whether ze can
// compare it with a configured remote-id.
//
// Ze compares five of the seven types RFC 7296 Section 3.5 assigns: ID_IPV4_ADDR,
// ID_IPV6_ADDR, ID_FQDN, ID_RFC822_ADDR, and ID_KEY_ID. Those are the four every
// implementation MUST accept, plus the address type an IPv6-capable implementation MUST
// accept. ID_DER_ASN1_DN and ID_DER_ASN1_GN are binary X.500 structures, and comparing
// one against configured text needs a canonical form that no rule in RFC 7296 states.
// Ze does not guess at one. The text is always filled so a refusal can print it.
func assertedIdentity(p *wire.PayloadID) (text string, comparable bool) {
	if p == nil {
		return "none", false
	}
	switch p.IDType {
	case wire.IDTypeIPv4Addr, wire.IDTypeIPv6Addr:
		addr, ok := assertedAddr(p)
		if !ok {
			var b textbuf.Buffer
			return b.Str(idTypeName(p.IDType)).Str(" of ").Int(int64(len(p.IDData))).Str(" octets").String(), false
		}
		return addr.String(), true
	case wire.IDTypeFQDN, wire.IDTypeRFC822Addr, wire.IDTypeKeyID:
		// Every text type renders through opaqueText. RFC 7296 Section 3.5 makes
		// ID_FQDN and ID_RFC822_ADDR ASCII. A peer that breaks that rule must not put
		// a control octet into a log line, or into an error a caller formats with %s.
		return opaqueText(p.IDData), true
	}
	var b textbuf.Buffer
	return b.Str(idTypeName(p.IDType)).Str(" ").Str(opaqueText(p.IDData)).String(), false
}

// identityClass groups the certificate fields that one configured remote-id value can be
// carried by. RFC 7296 Section 3.5 carries an address as four or sixteen raw octets.
// It carries every other comparable identity as text.
// The two classes therefore never name the same thing.
type identityClass uint8

const (
	classText identityClass = iota
	classAddress
)

// configuredClass reads the class the operator wrote. encodeIKEID makes the same reading
// for the local identity, so both ends of one configuration agree on it.
// certificateCarriesIdentity reads it to pick the one certificate field that CAN bind.
func configuredClass(want string) identityClass {
	if _, err := netip.ParseAddr(want); err == nil {
		return classAddress
	}
	return classText
}

// trailingDotOnly reports whether two names agree once a trailing dot is removed. It only
// shapes a refusal message. The comparison itself stays exact.
func trailingDotOnly(asserted, want string) bool {
	return asserted != want &&
		asciiEqualFold(strings.TrimSuffix(asserted, "."), strings.TrimSuffix(want, "."))
}

// assertedClass reads the class of the identity type a peer put on the wire. RFC 7296
// Section 3.5 carries ID_IPV4_ADDR and ID_IPV6_ADDR as raw octets, and every other
// comparable type as text. A type ze cannot compare belongs to neither class, and the
// second result says so.
func assertedClass(idType uint8) (identityClass, bool) {
	switch idType {
	case wire.IDTypeIPv4Addr, wire.IDTypeIPv6Addr:
		return classAddress, true
	case wire.IDTypeFQDN, wire.IDTypeRFC822Addr, wire.IDTypeKeyID:
		return classText, true
	}
	return classText, false
}

// classMismatchHint explains a refusal whose two sides print the same characters. An
// address-valued remote-id names an address, so ID_FQDN carrying the text of that address
// is refused while it renders identically. Without the hint an operator reads
// "asserted 10.0.0.1, expects 10.0.0.1" and sees no difference (ai/rules/error-messages.md).
func classMismatchHint(want string, p *wire.PayloadID) string {
	var b textbuf.Buffer
	b.Str(" The two texts agree and the types do not. The peer asserted ")
	b.Str(idTypeName(p.IDType)).Str(", and remote-id holds ")
	if configuredClass(want) == classAddress {
		b.Str("an address, which accepts ID_IPV4_ADDR and ID_IPV6_ADDR alone.")
	} else {
		b.Str("text, which accepts ID_FQDN, ID_RFC822_ADDR, and ID_KEY_ID alone.")
	}
	return b.String()
}

// remoteIDMatches reports whether the identity a peer asserted equals the configured
// remote-id. Both sides must belong to one class before any comparison runs, and the
// CONFIGURED value picks that class. certificateCarriesIdentity reads it the same way.
//
// Without the class gate remote-id "10.0.0.1" was satisfied by a peer asserting ID_FQDN
// whose text reads 10.0.0.1. For X.509 and EAP certificateCarriesIdentity then denied that
// peer, because it binds an address value against cert.IPAddresses alone. A
// pre-shared-secret peer has no certificate half, so nothing denied it. The policy half
// now refuses the class the operator did not write (ai/rules/fail-closed-guards.md).
func remoteIDMatches(want string, p *wire.PayloadID) bool {
	class, known := assertedClass(p.IDType)
	if !known || class != configuredClass(want) {
		return false
	}
	switch p.IDType {
	case wire.IDTypeIPv4Addr, wire.IDTypeIPv6Addr:
		asserted, ok := assertedAddr(p)
		if !ok {
			return false
		}
		configured, err := netip.ParseAddr(want)
		if err != nil {
			return false
		}
		return asserted == configured.Unmap()
	case wire.IDTypeFQDN, wire.IDTypeRFC822Addr:
		return asciiEqualFold(string(p.IDData), want)
	case wire.IDTypeKeyID:
		return string(p.IDData) == want
	}
	return false
}

// checkRemoteIdentity refuses a peer whose asserted identity is not the one remote-id
// names. It is the policy half of the remote-id contract, and it runs for every
// authentication mode.
//
// An empty remote-id runs no check. The operator stated no expectation, so there is
// nothing to compare. getRemoteCert states the consequence in the log for the
// certificate case, where it matters.
func checkRemoteIdentity(sa *SA) error {
	want := sa.PeerCfg.Auth.RemoteID
	if want == "" {
		return nil
	}
	if sa.RemoteIDPayload == nil {
		return fmt.Errorf(
			"ike auth: peer %q sent no identification payload, and remote-id expects %q. "+
				"Configure the peer to send its identity, or clear remote-id",
			sa.PeerName, want)
	}
	asserted, comparable := assertedIdentity(sa.RemoteIDPayload)
	if !comparable {
		return fmt.Errorf(
			"ike auth: peer %q asserted %s, which ze cannot compare with the configured remote-id %q. "+
				"Configure the peer to send ID_IPV4_ADDR, ID_IPV6_ADDR, ID_FQDN, ID_RFC822_ADDR, or ID_KEY_ID",
			sa.PeerName, asserted, want)
	}

	if !remoteIDMatches(want, sa.RemoteIDPayload) {
		hint := ""
		switch {
		case trailingDotOnly(asserted, want):
			hint = " The two differ only by a trailing dot, " +
				"and RFC 7296 Section 3.5 puts the exact label on the wire."
		case asserted == want:
			// Equal text that still failed the comparison leaves one cause. The two
			// identities belong to different classes, and only the type says so.
			hint = classMismatchHint(want, sa.RemoteIDPayload)
		}
		return fmt.Errorf(
			"ike auth: peer %q asserted identity %q, and remote-id expects %q.%s "+
				"Correct remote-id, or configure the peer to assert the expected identity",
			sa.PeerName, asserted, want, hint)
	}
	return nil
}

// hasSubjectAltName reports whether the certificate carries a subject alternative name
// extension. crypto/x509 records every parsed extension in Extensions, and its own
// hasSANExtension reads the same OID for the same question.
func hasSubjectAltName(cert *x509.Certificate) bool {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oidSubjectAltName) {
			return true
		}
	}
	return false
}

// certificateCarriesIdentity reports whether the certificate authority attested that
// this certificate speaks for the asserted identity.
//
// RFC 7296 Section 3.5 leaves the binding to the implementation. It says the identity
// "does not necessarily have to match anything in the CERT payload". It then permits an
// implementation to use both fields for an access control decision. Ze uses both.
//
// The chain alone proves only that the authority issued the certificate. The peer picks
// the identity its signature covers. One authority that issues to many clients therefore
// lets any client assert any identity, until this check runs.
//
// A subject alternative name binds. A subject common name binds ONLY when the certificate
// carries no alternative name extension at all. The fallback is needed: gen-pki.sh mints
// client.pem with no alternative name, and many deployments predate them.
//
// The two fields do NOT carry the same attestation, which is why the fallback is
// conditional. crypto/x509 checkChainConstraints applies a permitted or excluded subtree
// to DNSNames, URIs, EmailAddresses and IPAddresses only, and skips a certificate that
// carries no alternative name extension. The subject distinguished name is never
// constrained. An authority that permits only dNSName .branch.example.com therefore binds
// the alternative name and leaves the common name free. Reading the common name after a
// present alternative name authenticates the holder as the identity the authority
// constrained it away from (ai/rules/fail-closed-guards.md).
//
// The test is the EXTENSION, not the field. A certificate CAN carry an address
// alternative name and no name, and only the extension tells "the authority named nothing
// here" from "the authority named something else here".
//
// An empty asserted identity never binds. asciiEqualFold("", "") is true, so the guard is
// stated here rather than left to the want != "" gate in getRemoteCert.
//
// The CONFIGURED value picks the certificate field, never the asserted type. remote-id
// "172.28.0.3" was satisfiable as ID_IPV4_ADDR against cert.IPAddresses, and equally as
// ID_FQDN against cert.DNSNames. One configured value reached two certificate fields, and
// the peer chose. An authority issues an address alternative name under a tighter policy
// than a name, so the peer chose the weaker field (ai/rules/fail-closed-guards.md).
//
// ID_KEY_ID never binds. A certificate holds no field that corresponds to an opaque
// vendor identity, so the check denies rather than guesses (ai/rules/fail-closed-guards.md).
func certificateCarriesIdentity(cert *x509.Certificate, p *wire.PayloadID, want string) bool {
	if cert == nil || p == nil || len(p.IDData) == 0 {
		return false
	}
	cn := ""
	if !hasSubjectAltName(cert) {
		cn = cert.Subject.CommonName
	}

	// checkRemoteIdentity has already proven the asserted value equals want, so binding
	// on want is binding on the identity, with the class the operator wrote.
	if configuredClass(want) == classAddress {
		configured, err := netip.ParseAddr(want)
		if err != nil {
			return false
		}
		configured = configured.Unmap()
		for _, ip := range cert.IPAddresses {
			if named, ok := netip.AddrFromSlice(ip); ok && named.Unmap() == configured {
				return true
			}
		}
		named, err := netip.ParseAddr(cn)
		return err == nil && named.Unmap() == configured
	}

	switch p.IDType {
	case wire.IDTypeFQDN:
		asserted := string(p.IDData)
		for _, name := range cert.DNSNames {
			if asciiEqualFold(name, asserted) {
				return true
			}
		}
		return asciiEqualFold(cn, asserted)
	case wire.IDTypeRFC822Addr:
		asserted := string(p.IDData)
		for _, mail := range cert.EmailAddresses {
			if asciiEqualFold(mail, asserted) {
				return true
			}
		}
		return asciiEqualFold(cn, asserted)
	}
	return false
}
