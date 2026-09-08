// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- the anonymous NAI the EAP-TLS peer sends
// RFC: rfc/short/rfc9190.md -- Section 2.1.7, Identity; Section 2.1.8, Privacy
// Detail: rfc/full/rfc7542.txt -- Section 2.2, the NAI grammar; Section 2.4, username privacy
// Related: rfc/full/rfc5280.txt -- Section 4.2.1.6, the subjectAltName forms a name takes

package eap

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"strings"
	"unicode/utf8"
)

// anonymousUser is the username an anonymous NAI carries when the configured
// identity gives no realm to route on.
//
// RFC 9190 Section 2.1.8: "Following [RFC7542], it is RECOMMENDED to omit the
// username (i.e., the NAI is @realm), but other constructions such as a fixed
// username (e.g., anonymous@realm) or an encrypted username [...] are allowed."
// The omitted form needs a realm, and the grammar has no NAI that is neither a
// username nor a realm, so an identity with no realm takes the fixed username.
// It is the form RFC 7542 Section 2.4 names as current practice.
const anonymousUser = "anonymous"

// anonymousNAI returns the Network Access Identifier an EAP-TLS peer puts in the
// EAP-Response/Identity, derived from the NAI naiSource chose: the identity the
// operator configured, or the one the peer's own certificate carries.
//
// RFC 9190 Section 2.1.8: "A client supporting TLS 1.3 MUST NOT send its
// username (or any other permanent identifiers) in cleartext in the Identity
// Response (or any message used instead of the Identity Response)."
//
// The obligation binds a client that SUPPORTS TLS 1.3, not one that negotiated
// it, and that reading is the only one this code can act on: the Identity
// Response leaves before any ClientHello, so no negotiated version exists yet.
// Ze's EAP-TLS peer offers TLS 1.3, so every EAP-TLS identity is derived here,
// whatever version the handshake settles on.
//
// The realm is kept because it is what routes the exchange, and RFC 9190
// Section 2.1.3 asks for the same realm again on a resumption. Only the username
// is dropped, which is the construction Section 2.1.8 RECOMMENDS.
//
// The derived NAI is checked against the grammar before it is returned, which
// is what RFC 9190 Section 2.1.8 makes mandatory: "Note that the NAI MUST be a
// UTF-8 string as defined by the grammar in Section 2.2 of [RFC7542]." A
// source whose realm does not parse gives nothing to route on and nothing the
// grammar accepts, so it takes the fixed username. Every return is therefore a
// valid NAI, and none of them carries the username of the NAI it was given.
func anonymousNAI(identity string) string {
	if nai, ok := realmNAI(identity); ok {
		return nai
	}
	return anonymousUser
}

// realmNAI returns the anonymous "@realm" form of nai, and reports whether nai
// carried a realm the RFC 7542 Section 2.2 grammar accepts.
//
// The username is dropped HERE and reaches no return value: the only string
// this function can answer with is "@" and the text that followed the first "@"
// in its argument. That is what makes RFC 9190 Section 2.1.8's MUST NOT a
// property of the code rather than of the care its callers take, because
// anonymousNAI is the one writer of PeerSession.identity and this is the one
// value it keeps.
//
// A realm the grammar refuses is no realm: it gives nothing to route on, and
// RFC 9190 Section 2.1.8 requires the NAI to match that grammar. The caller
// takes the fixed username instead.
func realmNAI(nai string) (string, bool) {
	_, realm, found := strings.Cut(nai, "@")
	if !found {
		return "", false
	}
	anonymous := "@" + realm
	if !validNAI(anonymous) {
		return "", false
	}
	return anonymous, true
}

// naiSource returns the string anonymousNAI derives the peer's Identity
// Response from: the identity the operator configured, or, when that carries no
// realm to route on, the first NAI the peer's own certificate carries.
//
// RFC 9190 Section 2.1.7: "Many client certificates contain an identity such as
// an email address, which is already in NAI format.  When the client certificate
// contains an NAI as subject name or alternative subject name, an anonymous NAI
// SHOULD be derived from the NAI in the certificate; see Section 2.1.8."
//
// The configured identity comes first, because it is the realm the operator
// stated this deployment routes on, and RFC 9190 Section 2.1.3 wants that same
// realm: "It is RECOMMENDED to use Network Access Identifiers (NAIs) with the
// same realm during resumption and the original full handshake. [...] If this
// recommendation is not followed, resumption is likely impossible." The
// certificate answers only where the identity does not, which is the case that
// sentence in Section 2.1.7 was written for.
//
// The result is a SOURCE and not a wire value: it can still carry a username,
// exactly as the configured identity does. The caller MUST pass it through
// anonymousNAI, which is the one function that decides what the peer sends, and
// anonymousNAI MUST be given nothing else.
func naiSource(identity string, certPEM []byte) string {
	if _, ok := realmNAI(identity); ok {
		return identity
	}
	for _, name := range certificateNAIs(certPEM) {
		if _, ok := realmNAI(name); ok {
			return name
		}
	}
	return identity
}

// certificateNAIs returns the names the peer's own certificate carries that can
// hold an NAI, in the order RFC 9190 Section 2.1.7 makes them candidates.
//
// The subjectAltName comes before the subject, because it is where RFC 5280
// Section 4.2.1.6 puts a name of a defined form ("the subject alternative name
// extension allows identities to be bound to the subject of the certificate")
// while a common name is free text that only sometimes holds an NAI. The two
// disagree rarely, and where they do the structured assertion is the one the
// issuer made about the peer.
//
// Two subjectAltName forms are read. An rfc822Name is the form Section 2.1.7
// names itself, "an identity such as an email address, which is already in NAI
// format". A userPrincipalName otherName is the form an enterprise CA issues an
// 802.1X client certificate with, and it holds "user@domain", so it is an NAI as
// an alternative subject name exactly as the sentence describes.
//
// A dNSName is NOT read, and that is a decision the grammar makes rather than a
// gap. It carries no "@", so RFC 7542 Section 2.2 reads "example.com" as a
// utf8-username and not as a realm. Deriving "@example.com" from it would invent
// a realm the issuer never asserted, and returning it unchanged would put a
// permanent identifier on the wire, which is what Section 2.1.8 forbids.
//
// A certificate that does not parse yields no candidate. The material an
// operator configured is judged again by tls.X509KeyPair when the handshake
// starts (startTLSClient, peer.go), which is where a load failure is reported
// and where it can name the configuration. This constructor runs before any
// handshake exists, so it answers with the fixed username rather than failing a
// session that has not begun.
func certificateNAIs(certPEM []byte) []string {
	leaf := parseLeafCertificate(certPEM)
	if leaf == nil {
		return nil
	}

	names := make([]string, 0, len(leaf.EmailAddresses)+2)
	names = append(names, leaf.EmailAddresses...)
	names = append(names, userPrincipalNames(leaf)...)
	if leaf.Subject.CommonName != "" {
		names = append(names, leaf.Subject.CommonName)
	}
	return names
}

// parseLeafCertificate returns the first certificate in certPEM, which is the
// peer's own leaf: tls.X509KeyPair reads the same file and takes the first
// block as the leaf, with any intermediate after it.
//
// It answers nil for material that holds no certificate and for one that does
// not parse. The loop is bounded by the input, because pem.Decode consumes a
// whole block on each call and answers nil once none is left.
func parseLeafCertificate(certPEM []byte) *x509.Certificate {
	rest := certPEM
	for len(rest) > 0 {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		leaf, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil
		}
		return leaf
	}
	return nil
}

// oidSubjectAltName is the subjectAltName extension, RFC 5280 Section 4.2.1.6.
//
// crypto/x509 parses the forms it has fields for and keeps the whole extension
// in Certificate.Extensions, so an otherName is read from there.
var oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}

// oidUserPrincipalName is the otherName type-id Microsoft assigns to a
// userPrincipalName, 1.3.6.1.4.1.311.20.2.3. An enterprise CA writes the
// account's "user@domain" there, which is an NAI in the form RFC 9190
// Section 2.1.7 asks to derive from.
var oidUserPrincipalName = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 3}

// otherName is the subjectAltName form that carries a name X.509 has no field
// for. RFC 5280 Section 4.2.1.6:
//
//	OtherName ::= SEQUENCE {
//	     type-id    OBJECT IDENTIFIER,
//	     value      [0] EXPLICIT ANY DEFINED BY type-id }
type otherName struct {
	TypeID asn1.ObjectIdentifier
	Value  asn1.RawValue `asn1:"tag:0,explicit"`
}

// userPrincipalNames returns the userPrincipalName otherNames the leaf carries.
func userPrincipalNames(leaf *x509.Certificate) []string {
	var names []string
	for _, ext := range leaf.Extensions {
		if !ext.Id.Equal(oidSubjectAltName) {
			continue
		}
		names = append(names, parseUserPrincipalNames(ext.Value)...)
	}
	return names
}

// parseUserPrincipalNames walks one subjectAltName extension and returns the
// string value of every userPrincipalName otherName in it. RFC 5280
// Section 4.2.1.6:
//
//	SubjectAltName ::= GeneralNames
//	GeneralNames ::= SEQUENCE SIZE (1..MAX) OF GeneralName
//	GeneralName ::= CHOICE {
//	     otherName                  [0]  OtherName,
//	     rfc822Name                 [1]  IA5String,
//	     dNSName                    [2]  IA5String,
//	     ... }
//
// A certificate is attacker-influenced input in the general case, so every step
// answers nothing rather than failing: a name that does not parse is skipped and
// a sequence that does not parse yields what was read before it. The value is
// then judged by the RFC 7542 grammar before anything reaches the wire
// (realmNAI). The loop is bounded by the extension, because asn1.Unmarshal
// consumes at least the tag and the length of one value on each call.
func parseUserPrincipalNames(extension []byte) []string {
	var generalNames asn1.RawValue
	if _, err := asn1.Unmarshal(extension, &generalNames); err != nil {
		return nil
	}

	var names []string
	rest := generalNames.Bytes
	for len(rest) > 0 {
		var general asn1.RawValue
		remainder, err := asn1.Unmarshal(rest, &general)
		if err != nil {
			return names
		}
		rest = remainder

		if general.Class != asn1.ClassContextSpecific || general.Tag != 0 {
			continue
		}
		var other otherName
		if _, err := asn1.UnmarshalWithParams(general.FullBytes, &other, "tag:0"); err != nil {
			continue
		}
		if !other.TypeID.Equal(oidUserPrincipalName) {
			continue
		}
		text, ok := otherNameText(other.Value)
		if !ok {
			continue
		}
		names = append(names, text)
	}
	return names
}

// otherNameText returns the string an otherName value holds, and reports
// whether it held one.
//
// encoding/asn1 hands a RawValue field the tagged value AS IT STANDS, so the
// "explicit" tag on otherName.Value removes no wrapper: value carries the [0]
// wrapper, and its Bytes are the whole encoding of the ANY inside it. That inner
// value is read here.
//
// A userPrincipalName is a UTF8String. The other two string types are accepted
// because a CA that wrote one of them still wrote a name, and any other tag
// holds bytes that are not text.
func otherNameText(value asn1.RawValue) (string, bool) {
	var text asn1.RawValue
	if _, err := asn1.Unmarshal(value.Bytes, &text); err != nil {
		return "", false
	}
	if text.Class != asn1.ClassUniversal {
		return "", false
	}
	switch text.Tag {
	case asn1.TagUTF8String, asn1.TagPrintableString, asn1.TagIA5String:
		return string(text.Bytes), true
	}
	return "", false
}

// validNAI reports whether nai matches the grammar of RFC 7542 Section 2.2,
// which RFC 9190 Section 2.1.8 makes mandatory: "Note that the NAI MUST be a
// UTF-8 string as defined by the grammar in Section 2.2 of [RFC7542]."
//
//	nai            =   utf8-username
//	nai            =/  "@" utf8-realm
//	nai            =/  utf8-username "@" utf8-realm
//
// utf8-atext carries no "@", so a valid NAI holds at most one, and the first one
// separates the two halves. A second "@" lands inside the realm, where
// utf8-rtext refuses it.
func validNAI(nai string) bool {
	if !utf8.ValidString(nai) {
		return false
	}
	at := strings.IndexByte(nai, '@')
	if at < 0 {
		return validUsername(nai)
	}
	if at == 0 {
		return validRealm(nai[1:])
	}
	if !validUsername(nai[:at]) {
		return false
	}
	return validRealm(nai[at+1:])
}

// validUsername reports whether s is a utf8-username. RFC 7542 Section 2.2:
//
//	utf8-username  =  dot-string
//	dot-string     = string *("." string)
//	string         = 1*utf8-atext
//
// Each dot-separated part holds at least one character, so a leading dot, a
// trailing dot and an empty username are all refused.
//
// The caller has checked that s is valid UTF-8, which is what lets the range
// loop read a rune above 0x7F as one UTF8-xtra-char.
func validUsername(s string) bool {
	partLen := 0
	for _, r := range s {
		if r == '.' {
			if partLen == 0 {
				return false
			}
			partLen = 0
			continue
		}
		if !isAtext(r) {
			return false
		}
		partLen++
	}
	return partLen > 0
}

// validRealm reports whether s is a utf8-realm. RFC 7542 Section 2.2:
//
//	utf8-realm     =  1*( label "." ) label
//	label          =  utf8-rtext *(ldh-str)
//	ldh-str        =  *( utf8-rtext / "-" ) utf8-rtext
//	utf8-rtext     =  ALPHA / DIGIT / UTF8-xtra-char
//
// So a label starts and ends with a utf8-rtext and carries "-" only inside, and
// a realm holds two labels at least. "@example.com" is a realm this accepts and
// "@localhost" is not, because the grammar requires the dot.
//
// The caller has checked that s is valid UTF-8, which is what lets the range
// loop read a rune above 0x7F as one UTF8-xtra-char.
func validRealm(s string) bool {
	labels := 0
	labelLen := 0
	hyphenLast := false

	for _, r := range s {
		if r == '.' {
			if labelLen == 0 {
				return false
			}
			if hyphenLast {
				return false
			}
			labels++
			labelLen = 0
			continue
		}
		if r == '-' {
			if labelLen == 0 {
				return false
			}
			labelLen++
			hyphenLast = true
			continue
		}
		if !isRtext(r) {
			return false
		}
		labelLen++
		hyphenLast = false
	}

	if labelLen == 0 {
		return false
	}
	if hyphenLast {
		return false
	}
	labels++
	return labels >= 2
}

// isAtext reports whether r is a utf8-atext, the character class of the username
// half of an NAI (RFC 7542 Section 2.2).
func isAtext(r rune) bool {
	if isRtext(r) {
		return true
	}
	return strings.ContainsRune("!#$%&'*+-/=?^_`{|}~", r)
}

// isRtext reports whether r is a utf8-rtext: ALPHA, DIGIT, or UTF8-xtra-char
// (RFC 7542 Sections 2.1 and 2.2).
//
// UTF8-xtra-char is every multi-byte UTF-8 sequence RFC 3629 admits, so a rune
// above 0x7F answers it exactly when the string it came from is valid UTF-8.
// Each caller checks that before it decodes.
func isRtext(r rune) bool {
	if r > 0x7F {
		return true
	}
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	return r >= '0' && r <= '9'
}
