// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- the anonymous NAI the EAP-TLS peer sends
// RFC: rfc/short/rfc9190.md -- Section 2.1.8, Privacy
// Detail: rfc/full/rfc7542.txt -- Section 2.2, the NAI grammar; Section 2.4, username privacy

package eap

import (
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
// EAP-Response/Identity, derived from the identity the operator configured.
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
// UTF-8 string as defined by the grammar in Section 2.2 of [RFC7542]." An
// identity whose realm does not parse gives nothing to route on and nothing the
// grammar accepts, so it takes the fixed username. Every return is therefore a
// valid NAI, and none of them carries a substring of what the operator wrote.
func anonymousNAI(identity string) string {
	_, realm, found := strings.Cut(identity, "@")
	if found {
		nai := "@" + realm
		if validNAI(nai) {
			return nai
		}
	}
	return anonymousUser
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
