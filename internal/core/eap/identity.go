// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS certificate identity extraction and authorization
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS peer server-identity verification
// RFC: rfc/short/rfc5216.md -- Section 5.2 (Peer-Id/Server-Id from subjectAltName), Section 5.3 (identity authorization and RFC 2818 Section 3.1)

package eap

import (
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// identityMaxBytes is the YANG length ceiling (length "1..255") shared by
	// allowed-identities entries and server-identity values.
	identityMaxBytes = 255

	// dnsLabelMaxBytes is the RFC 1035 Section 2.3.4 maximum length of one DNS label.
	dnsLabelMaxBytes = 63

	// maxSANIdentities bounds how many subjectAltName GeneralNames are decoded
	// from an untrusted peer certificate so a crafted extension cannot drive
	// unbounded allocation.
	maxSANIdentities = 1024

	diagNoCertificate = "(no certificate)"
	diagNoIdentity    = "(no identity)"
	urnPrefix         = "urn:"
)

// oidCommonName is the X.520 Common Name RDN attribute OID (2.5.4.3).
var oidCommonName = asn1.ObjectIdentifier{2, 5, 4, 3}

type allowedIdentityKind uint8

const (
	allowedKindUnspecified allowedIdentityKind = iota
	allowedKindExactDNS
	allowedKindWildcardDNS
	allowedKindExactNAI
	allowedKindNAIRealm
	allowedKindExactIP
	allowedKindExactURI
	allowedKindAny
)

// AllowedIdentity is a validated authenticator client-identity authorization rule.
// Constructed only by ParseAllowedIdentity.
type AllowedIdentity struct {
	kind allowedIdentityKind
	raw  string
	addr netip.Addr
}

type serverIdentityKind uint8

const (
	serverKindUnspecified serverIdentityKind = iota
	serverKindIP
	serverKindDNS
	serverKindNAI
)

// ServerIdentity is a validated peer expected server identity.
// Constructed only by ParseServerIdentity.
type ServerIdentity struct {
	kind serverIdentityKind
	raw  string
	addr netip.Addr
}

type certIdentityKind uint8

const (
	certIDUnspecified certIdentityKind = iota
	certIDDNS
	certIDNAI
	certIDIP
	certIDURI
	certIDCommonName
	certIDSubjectDN
)

// certIdentity is one extracted certificate identity carrying both its wire
// rendering and its X.509 name class so matching preserves strict class
// separation (for example, IP literals match iPAddress SANs alone).
type certIdentity struct {
	kind  certIdentityKind
	value string
	addr  netip.Addr
}

// String returns the canonical operator-configured rule text.
func (a AllowedIdentity) String() string { return a.raw }

// IsWildcardAny reports whether this rule is the explicit opt-in lone "*"
// wildcard that admits any valid CA-signed client certificate carrying a
// non-empty identity.
func (a AllowedIdentity) IsWildcardAny() bool { return a.kind == allowedKindAny }

// String returns the configured expected server identity text.
func (s ServerIdentity) String() string { return s.raw }

func (s ServerIdentity) valid() bool { return s.kind != serverKindUnspecified }

// ParseAllowedIdentity parses and validates one authenticator allowed-identities
// entry (exact DNS, single-label DNS wildcard, exact NAI, NAI realm, exact IP,
// exact URI, or explicit lone "*").
func ParseAllowedIdentity(raw string) (AllowedIdentity, error) {
	if raw == "" {
		return AllowedIdentity{}, errors.New("must not be empty")
	}
	if len(raw) > identityMaxBytes {
		return AllowedIdentity{}, fmt.Errorf("exceeds maximum length of %d bytes", identityMaxBytes)
	}
	if hasControlByte(raw) {
		return AllowedIdentity{}, errors.New("must not contain control characters")
	}
	if !utf8.ValidString(raw) {
		return AllowedIdentity{}, errors.New("must be valid UTF-8")
	}
	if hasWhitespace(raw) {
		return AllowedIdentity{}, errors.New("must not contain whitespace")
	}
	if raw == "*" {
		return AllowedIdentity{kind: allowedKindAny, raw: raw}, nil
	}

	if isExactURIShape(raw) {
		if err := validateExactURI(raw); err != nil {
			return AllowedIdentity{}, err
		}
		return AllowedIdentity{kind: allowedKindExactURI, raw: raw}, nil
	}

	if addr, err := netip.ParseAddr(raw); err == nil {
		if addr.Zone() != "" {
			return AllowedIdentity{}, errors.New("IP address must not include a zone identifier")
		}
		return AllowedIdentity{kind: allowedKindExactIP, raw: raw, addr: addr.Unmap()}, nil
	}
	if _, err := netip.ParsePrefix(raw); err == nil {
		return AllowedIdentity{}, errors.New("CIDR prefixes are not supported; list exact IP addresses instead")
	}

	if strings.ContainsRune(raw, '@') {
		return parseAllowedNAI(raw)
	}

	if strings.HasPrefix(raw, ".") {
		return AllowedIdentity{}, fmt.Errorf("leading dot subtree %q is not supported; use single-label wildcard \"*%s\" or exact DNS names", raw, raw)
	}

	if strings.ContainsRune(raw, '*') {
		return parseAllowedWildcardDNS(raw)
	}

	labels, ok := validDNSLabels(raw)
	if !ok {
		return AllowedIdentity{}, errors.New("invalid entry; expected a DNS name, \"*.domain.tld\" wildcard, NAI/email, \"@realm\", IP address, URI, or \"*\"")
	}
	if labels < 1 {
		return AllowedIdentity{}, errors.New("invalid entry; expected a DNS name, \"*.domain.tld\" wildcard, NAI/email, \"@realm\", IP address, URI, or \"*\"")
	}
	return AllowedIdentity{kind: allowedKindExactDNS, raw: raw}, nil
}

// ParseServerIdentity parses and validates one peer expected server-identity
// value (IP literal, DNS hostname/FQDN, or exact user@realm NAI).
func ParseServerIdentity(raw string) (ServerIdentity, error) {
	if raw == "" {
		return ServerIdentity{}, errors.New("must not be empty")
	}
	if len(raw) > identityMaxBytes {
		return ServerIdentity{}, fmt.Errorf("exceeds maximum length of %d bytes", identityMaxBytes)
	}
	if hasControlByte(raw) {
		return ServerIdentity{}, errors.New("must not contain control characters")
	}
	if !utf8.ValidString(raw) {
		return ServerIdentity{}, errors.New("must be valid UTF-8")
	}
	if hasWhitespace(raw) {
		return ServerIdentity{}, errors.New("must not contain whitespace")
	}
	if strings.ContainsRune(raw, '*') {
		return ServerIdentity{}, errors.New("server-identity must not contain wildcards")
	}
	if strings.HasPrefix(raw, ".") {
		return ServerIdentity{}, errors.New("server-identity must not start with a dot")
	}
	if strings.HasPrefix(raw, "@") {
		return ServerIdentity{}, errors.New("server-identity must not be a bare realm; specify an exact user@realm NAI, DNS name, or IP address")
	}

	if addr, err := netip.ParseAddr(raw); err == nil {
		if addr.Zone() != "" {
			return ServerIdentity{}, errors.New("server-identity IP address must not include a zone identifier")
		}
		return ServerIdentity{kind: serverKindIP, raw: raw, addr: addr.Unmap()}, nil
	}
	if _, err := netip.ParsePrefix(raw); err == nil {
		return ServerIdentity{}, errors.New("server-identity must be a single IP address, not a CIDR prefix")
	}

	if strings.ContainsRune(raw, '@') {
		user, _, _ := strings.Cut(raw, "@")
		if user == "" {
			return ServerIdentity{}, errors.New("invalid NAI; expected user@domain.tld with a non-empty username")
		}
		if !validNAI(raw) {
			return ServerIdentity{}, errors.New("invalid NAI; expected user@domain.tld with at least two realm labels")
		}
		return ServerIdentity{kind: serverKindNAI, raw: raw}, nil
	}

	labels, ok := validDNSLabels(raw)
	if !ok {
		return ServerIdentity{}, errors.New("invalid server-identity; expected an IP address, DNS hostname/FQDN, or user@realm NAI")
	}
	if labels < 1 {
		return ServerIdentity{}, errors.New("invalid server-identity; expected an IP address, DNS hostname/FQDN, or user@realm NAI")
	}
	return ServerIdentity{kind: serverKindDNS, raw: raw}, nil
}

// IsUnambiguousFQDN reports whether s is a multi-label DNS FQDN suitable as an
// implicit EAP-TLS server-identity fallback from remote-address.
func IsUnambiguousFQDN(s string) bool {
	if s == "" {
		return false
	}
	if len(s) > identityMaxBytes {
		return false
	}
	if hasControlByte(s) {
		return false
	}
	if hasWhitespace(s) {
		return false
	}
	if _, err := netip.ParseAddr(s); err == nil {
		return false
	}
	labels, ok := validDNSLabels(s)
	return ok && labels >= 2
}

// parseAllowedNAI validates an exact user@realm NAI or a leading-@ realm rule.
func parseAllowedNAI(raw string) (AllowedIdentity, error) {
	if strings.ContainsRune(raw, '*') {
		return AllowedIdentity{}, errors.New("wildcards are not permitted in NAI entries; use \"@domain.tld\" to match a realm")
	}
	if strings.HasPrefix(raw, "@") {
		realm := raw[1:]
		if !validRealm(realm) {
			return AllowedIdentity{}, errors.New("invalid NAI realm; expected \"@domain.tld\" with at least two valid realm labels")
		}
		return AllowedIdentity{kind: allowedKindNAIRealm, raw: raw}, nil
	}
	user, _, _ := strings.Cut(raw, "@")
	if user == "" {
		return AllowedIdentity{}, errors.New("invalid NAI; expected user@domain.tld with a non-empty username")
	}
	if !validNAI(raw) {
		return AllowedIdentity{}, errors.New("invalid NAI; expected user@domain.tld with a non-empty username and at least two realm labels")
	}
	return AllowedIdentity{kind: allowedKindExactNAI, raw: raw}, nil
}

// parseAllowedWildcardDNS validates a single leftmost full-label DNS wildcard
// of the form "*.a.b" with at least two valid DNS suffix labels.
func parseAllowedWildcardDNS(raw string) (AllowedIdentity, error) {
	suffix, ok := strings.CutPrefix(raw, "*.")
	if !ok {
		return AllowedIdentity{}, errors.New("wildcards are only supported as a lone \"*\" or a single leftmost label \"*.domain.tld\"")
	}
	if strings.ContainsRune(suffix, '*') {
		return AllowedIdentity{}, errors.New("multi-wildcard DNS patterns are not supported; use a single leftmost label \"*.domain.tld\"")
	}
	labels, valid := validDNSLabels(suffix)
	if !valid {
		return AllowedIdentity{}, errors.New("DNS wildcard must have valid DNS labels after \"*.\" (for example, \"*.branch.example.com\")")
	}
	if labels < 2 {
		return AllowedIdentity{}, errors.New("DNS wildcard must have at least two valid DNS labels after \"*.\" (for example, \"*.branch.example.com\")")
	}
	return AllowedIdentity{kind: allowedKindWildcardDNS, raw: raw}, nil
}

func isExactURIShape(raw string) bool {
	return strings.Contains(raw, "://") || strings.HasPrefix(raw, urnPrefix)
}

// validateExactURI validates an exact URI SAN entry containing "://" or
// beginning with "urn:".
func validateExactURI(raw string) error {
	if strings.ContainsRune(raw, '*') {
		return errors.New("URI entries must not contain wildcards")
	}
	for i := range len(raw) {
		c := raw[i]
		if c < 0x21 {
			return errors.New("URI entries must contain only printable non-whitespace ASCII")
		}
		if c > 0x7e {
			return errors.New("URI entries must contain only printable non-whitespace ASCII")
		}
	}
	if strings.HasPrefix(raw, urnPrefix) {
		if len(raw) <= len(urnPrefix) {
			return errors.New("URN entry must have a non-empty body after \"urn:\"")
		}
		return nil
	}
	scheme, body, found := strings.Cut(raw, "://")
	if !found {
		return errors.New("URI entry must have a valid non-empty scheme before \"://\"")
	}
	if !validURIScheme(scheme) {
		return errors.New("URI entry must have a valid non-empty scheme before \"://\"")
	}
	if body == "" {
		return errors.New("URI entry must have a non-empty body after \"://\"")
	}
	return nil
}

// validURIScheme checks RFC 3986 Section 3.1: ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ).
func validURIScheme(s string) bool {
	if s == "" {
		return false
	}
	if !isASCIIAlpha(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isURISchemeByte(s[i]) {
			return false
		}
	}
	return true
}

func isURISchemeByte(c byte) bool {
	return isASCIIAlpha(c) || isASCIIDigit(c) || c == '+' || c == '-' || c == '.'
}

// certificatePrimaryID returns the primary RFC 5216 Section 5.2 identity of cert
// for diagnostics and exported PeerID/ServerID accessors.
//
// RFC 5216 Section 5.2: "Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName."
func certificatePrimaryID(cert *x509.Certificate) string {
	if cert == nil {
		return diagNoCertificate
	}
	if ids := extractCertificateIdentities(cert); len(ids) > 0 {
		return ids[0].value
	}
	if dn := cert.Subject.String(); dn != "" && utf8.ValidString(dn) && !hasControlByte(dn) {
		return dn
	}
	return diagNoIdentity
}

// extractCertificateIdentities returns the structured authorization candidates
// carried by cert in certificate wire order.
//
// RFC 5216 Section 5.2: "Where the subjectAltName field is present in the peer or server certificate, the Peer-Id or Server-Id MUST be set to the contents of the subjectAltName."
// RFC 5216 Section 5.2: "EAP-TLS implementations supporting export of the Peer-Id and Server-Id SHOULD export Peer-Ids and Server-Ids in the same order in which they appear within the certificate."
// RFC 2818 Section 3.1: "If a subjectAltName extension of type dNSName is present, that MUST be used as the identity. Otherwise, the (most specific) Common Name field in the Subject field of the certificate MUST be used."
//
// When any subjectAltName extension (OID 2.5.29.17) is present, Subject CN and
// Subject DN are NEVER consulted, even if the extension holds zero supported
// GeneralNames: authorization then sees an empty candidate set and fails closed.
func extractCertificateIdentities(cert *x509.Certificate) []certIdentity {
	if cert == nil {
		return nil
	}

	sanPresent := false
	var sanIDs []certIdentity
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidSubjectAltName) {
			continue
		}
		sanPresent = true
		sanIDs = appendSANIdentities(sanIDs, ext.Value)
	}
	if sanPresent {
		return sanIDs
	}

	if cn, hasCN := mostSpecificCommonName(cert); hasCN {
		if cn == "" {
			return nil
		}
		return []certIdentity{{kind: certIDCommonName, value: cn}}
	}

	if dn := cert.Subject.String(); dn != "" && utf8.ValidString(dn) && !hasControlByte(dn) {
		return []certIdentity{{kind: certIDSubjectDN, value: dn}}
	}
	return nil
}

// appendSANIdentities walks one DER-encoded SubjectAltName GeneralNames sequence
// in certificate wire order and appends every supported GeneralName:
// rfc822Name [1], dNSName [2], uniformResourceIdentifier [6], iPAddress [7],
// and userPrincipalName otherName [0] (OID 1.3.6.1.4.1.311.20.2.3).
//
// The loop is bounded by both the remaining ASN.1 slice length (strict progress
// check on every step) and maxSANIdentities.
func appendSANIdentities(dst []certIdentity, extension []byte) []certIdentity {
	var generalNames asn1.RawValue
	if _, err := asn1.Unmarshal(extension, &generalNames); err != nil {
		return dst
	}
	if generalNames.Class != asn1.ClassUniversal {
		return dst
	}
	if generalNames.Tag != asn1.TagSequence {
		return dst
	}
	if !generalNames.IsCompound {
		return dst
	}

	rest := generalNames.Bytes
	for len(rest) > 0 && len(dst) < maxSANIdentities {
		prevLen := len(rest)
		var general asn1.RawValue
		remainder, err := asn1.Unmarshal(rest, &general)
		if err != nil {
			return dst
		}
		if len(remainder) >= prevLen {
			return dst
		}
		rest = remainder

		if general.Class != asn1.ClassContextSpecific {
			continue
		}
		if id, ok := decodeGeneralName(general); ok {
			dst = append(dst, id)
		}
	}
	return dst
}

// decodeGeneralName decodes one context-tagged GeneralName from a SAN sequence.
func decodeGeneralName(general asn1.RawValue) (certIdentity, bool) {
	switch general.Tag {
	case 0: // otherName
		var other otherName
		if _, err := asn1.UnmarshalWithParams(general.FullBytes, &other, "tag:0"); err != nil {
			return certIdentity{}, false
		}
		if !other.TypeID.Equal(oidUserPrincipalName) {
			return certIdentity{}, false
		}
		text, ok := otherNameText(other.Value)
		if !ok {
			return certIdentity{}, false
		}
		if !validCertText(text) {
			return certIdentity{}, false
		}
		return certIdentity{kind: certIDNAI, value: text}, true

	case 1: // rfc822Name
		text := string(general.Bytes)
		if !validCertText(text) {
			return certIdentity{}, false
		}
		return certIdentity{kind: certIDNAI, value: text}, true

	case 2: // dNSName
		text := string(general.Bytes)
		if !validCertText(text) {
			return certIdentity{}, false
		}
		return certIdentity{kind: certIDDNS, value: text}, true

	case 6: // uniformResourceIdentifier
		text := string(general.Bytes)
		if !validCertText(text) {
			return certIdentity{}, false
		}
		return certIdentity{kind: certIDURI, value: text}, true

	case 7: // iPAddress
		if len(general.Bytes) != 4 && len(general.Bytes) != 16 {
			return certIdentity{}, false
		}
		addr, ok := netip.AddrFromSlice(general.Bytes)
		if !ok {
			return certIdentity{}, false
		}
		unmapped := addr.Unmap()
		return certIdentity{kind: certIDIP, value: unmapped.String(), addr: unmapped}, true
	}
	return certIdentity{}, false
}

// mostSpecificCommonName returns the most specific (last) Common Name RDN in
// cert.Subject.Names per RFC 2818 Section 3.1. hasCN is true whenever a CN RDN
// exists; if that most specific CN carries invalid UTF-8 or a control byte,
// cn is returned empty with hasCN true so the caller fails closed rather than
// falling back to a less specific CN or Subject DN.
func mostSpecificCommonName(cert *x509.Certificate) (cn string, hasCN bool) {
	for _, atv := range slices.Backward(cert.Subject.Names) {
		if !atv.Type.Equal(oidCommonName) {
			continue
		}
		val, ok := atv.Value.(string)
		if !ok {
			continue
		}
		if val == "" {
			continue
		}
		if !validCertText(val) {
			return "", true
		}
		return val, true
	}
	if cert.Subject.CommonName != "" {
		if !validCertText(cert.Subject.CommonName) {
			return "", true
		}
		return cert.Subject.CommonName, true
	}
	return "", false
}

func identityValues(ids []certIdentity) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.value
	}
	return out
}

// authorizePeerCertificate checks whether leaf carries an identity authorized by
// at least one rule in allowed and returns the verified identities in wire order.
//
// RFC 5216 Section 5.3: "Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS."
// RFC 9190 Section 2.2: "If any of the configured names match any of the names in the SAN extension, then the name check passes."
func authorizePeerCertificate(leaf *x509.Certificate, allowed []AllowedIdentity) ([]string, error) {
	if leaf == nil {
		return nil, errors.New("eap-tls: client presented no certificate")
	}
	if len(allowed) == 0 {
		return nil, errors.New("eap-tls: no allowed-identities configured for client authorization (RFC 5216 Section 5.3)")
	}
	candidates := extractCertificateIdentities(leaf)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("eap-tls: client certificate %q carries no usable identity for EAP-TLS authorization (RFC 5216 Section 5.2)",
			certificatePrimaryID(leaf))
	}
	for _, rule := range allowed {
		for _, cand := range candidates {
			if allowedIdentityMatches(rule, cand) {
				return identityValues(candidates), nil
			}
		}
	}
	return nil, fmt.Errorf("eap-tls: client certificate identity %q is not authorized by allowed-identities (RFC 5216 Section 5.3)",
		certificatePrimaryID(leaf))
}

func allowedIdentityMatches(rule AllowedIdentity, cand certIdentity) bool {
	switch rule.kind {
	case allowedKindExactDNS:
		if cand.kind != certIDDNS && cand.kind != certIDCommonName {
			return false
		}
		return asciiEqualFold(cand.value, rule.raw)
	case allowedKindWildcardDNS:
		if cand.kind != certIDDNS && cand.kind != certIDCommonName {
			return false
		}
		return matchSingleLabelWildcard(cand.value, rule.raw)
	case allowedKindExactNAI:
		if cand.kind != certIDNAI && cand.kind != certIDCommonName {
			return false
		}
		return asciiEqualFold(cand.value, rule.raw)
	case allowedKindNAIRealm:
		if cand.kind != certIDNAI && cand.kind != certIDCommonName {
			return false
		}
		return matchNAIRealm(cand.value, rule.raw)
	case allowedKindExactIP:
		if cand.kind != certIDIP {
			return false
		}
		return cand.addr == rule.addr
	case allowedKindExactURI:
		if cand.kind != certIDURI {
			return false
		}
		return cand.value == rule.raw
	case allowedKindAny:
		return cand.kind != certIDUnspecified && cand.value != ""
	case allowedKindUnspecified:
		return false
	default:
		panic("BUG: unknown EAP-TLS allowed identity kind")
	}
}

// verifyServerCertificate verifies leaf against expected following RFC 2818
// Section 3.1 and returns the verified identities in wire order.
//
// RFC 5216 Section 5.3: "Once a TLS session is established, EAP-TLS peer and server implementations MUST validate that the identities represented in the certificate are appropriate and authorized for use with EAP-TLS."
// RFC 5216 Section 5.3: "When performing this comparison, implementations MUST follow the validation rules specified in Section 3.1 of [RFC2818]."
// RFC 2818 Section 3.1: "In some cases, the URI is specified as an IP address rather than a hostname. In this case, the iPAddress subjectAltName must be present in the certificate and must exactly match the IP in the URI."
// RFC 2818 Section 3.1: "Names may contain the wildcard character * which is considered to match any single domain name component or component fragment."
func verifyServerCertificate(leaf *x509.Certificate, expected ServerIdentity) ([]string, error) {
	if leaf == nil {
		return nil, errors.New("eap-tls: authenticator presented no certificate")
	}
	if !expected.valid() {
		return nil, errors.New("eap-tls: no expected server-identity configured (RFC 5216 Section 5.3)")
	}
	candidates := extractCertificateIdentities(leaf)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("eap-tls: server certificate %q carries no usable identity for expected server-identity %q (RFC 5216 Section 5.2)",
			certificatePrimaryID(leaf), expected.raw)
	}
	for _, cand := range candidates {
		if serverIdentityMatches(expected, cand) {
			return identityValues(candidates), nil
		}
	}
	return nil, fmt.Errorf("eap-tls: server certificate identity %q does not match expected server-identity %q (RFC 5216 Section 5.3 / RFC 2818 Section 3.1)",
		certificatePrimaryID(leaf), expected.raw)
}

func serverIdentityMatches(expected ServerIdentity, cand certIdentity) bool {
	switch expected.kind {
	case serverKindIP:
		if cand.kind != certIDIP {
			return false
		}
		return cand.addr == expected.addr
	case serverKindDNS:
		if cand.kind != certIDDNS && cand.kind != certIDCommonName {
			return false
		}
		if strings.ContainsRune(cand.value, '*') {
			return matchSingleLabelWildcard(expected.raw, cand.value)
		}
		return asciiEqualFold(cand.value, expected.raw)
	case serverKindNAI:
		if cand.kind != certIDNAI && cand.kind != certIDCommonName {
			return false
		}
		return asciiEqualFold(cand.value, expected.raw)
	case serverKindUnspecified:
		return false
	default:
		panic("BUG: unknown EAP-TLS server identity kind")
	}
}

// matchSingleLabelWildcard reports whether name matches a single leftmost
// full-label DNS wildcard pattern of the form "*.a.b" (at least two valid DNS
// suffix labels), matching exactly one non-empty valid DNS label before the dot.
func matchSingleLabelWildcard(name, pattern string) bool {
	suffix, ok := strings.CutPrefix(pattern, "*.")
	if !ok {
		return false
	}
	if strings.ContainsRune(suffix, '*') {
		return false
	}
	suffixLabels, valid := validDNSLabels(suffix)
	if !valid {
		return false
	}
	if suffixLabels < 2 {
		return false
	}
	dot := strings.IndexByte(name, '.')
	if dot <= 0 {
		return false
	}
	firstLabel := name[:dot]
	if !validSingleDNSLabel(firstLabel) {
		return false
	}
	return asciiEqualFold(name[dot+1:], suffix)
}

// matchNAIRealm reports whether candidate is a valid NAI with a non-empty
// username whose realm matches rule (which starts with "@") ASCII case-insensitively.
func matchNAIRealm(candidate, rule string) bool {
	wantRealm, ok := strings.CutPrefix(rule, "@")
	if !ok {
		return false
	}
	if wantRealm == "" {
		return false
	}
	user, realm, found := strings.Cut(candidate, "@")
	if !found {
		return false
	}
	if user == "" {
		return false
	}
	if !validNAI(candidate) {
		return false
	}
	return asciiEqualFold(realm, wantRealm)
}

// validDNSLabels validates a dot-separated sequence of ASCII DNS labels
// ([a-zA-Z0-9-], each 1..63 octets, no leading/trailing hyphen or dot) and
// returns the label count.
func validDNSLabels(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	labels := 0
	start := 0
	for i := range len(s) {
		if s[i] == '.' {
			if !validSingleDNSLabel(s[start:i]) {
				return 0, false
			}
			labels++
			start = i + 1
		}
	}
	if !validSingleDNSLabel(s[start:]) {
		return 0, false
	}
	labels++
	return labels, true
}

func validSingleDNSLabel(label string) bool {
	n := len(label)
	if n == 0 {
		return false
	}
	if n > dnsLabelMaxBytes {
		return false
	}
	if label[0] == '-' {
		return false
	}
	if label[n-1] == '-' {
		return false
	}
	for i := range n {
		if !isDNSLabelByte(label[i]) {
			return false
		}
	}
	return true
}

func isDNSLabelByte(c byte) bool {
	return isASCIIAlpha(c) || isASCIIDigit(c) || c == '-'
}

func validCertText(s string) bool {
	if s == "" {
		return false
	}
	if !utf8.ValidString(s) {
		return false
	}
	return !hasControlByte(s)
}

func hasControlByte(s string) bool {
	for i := range len(s) {
		c := s[i]
		if c < 0x20 {
			return true
		}
		if c == 0x7f {
			return true
		}
	}
	return false
}

func hasWhitespace(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func isASCIIAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func asciiLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// asciiEqualFold compares two strings with ASCII letters folded and every other
// octet compared verbatim, avoiding Unicode case-folding collisions such as
// U+212A (Kelvin sign) matching 'k'.
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
