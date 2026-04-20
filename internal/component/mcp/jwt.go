// Design: docs/architecture/mcp/overview.md -- MCP OAuth resource server

// Stdlib JWT verifier for the MCP OAuth resource-server path.
//
// Supports RS256, RS384, RS512, ES256, ES384 -- the algorithms operator
// authorization servers realistically issue. HMAC (HS*) is rejected because
// the resource server never shares a symmetric key with the AS. "alg: none"
// is rejected at the parse step so an unsigned token cannot masquerade as a
// signed one.
//
// Everything below is pure stdlib: crypto/rsa, crypto/ecdsa, crypto/sha256
// (and sha384/sha512), encoding/base64, encoding/json, math/big.

package mcp

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// jwtHeader is the decoded JWS protected header ("alg", "kid", "typ"). MCP
// OAuth access tokens are JWS-signed JWTs; this matches RFC 7515 / 7519.
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// jwtClaims is the subset of RFC 7519 + RFC 8693 claims the resource server
// inspects. Unknown claims are tolerated (standard JWT forward-compat).
type jwtClaims struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Audience  audClaim `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	NotBefore int64    `json:"nbf"`
	IssuedAt  int64    `json:"iat"`
	Scope     string   `json:"scope"`
}

// audClaim decodes the `aud` claim which per RFC 7519 may be a single string
// or an array of strings. Both forms normalize into a []string.
type audClaim []string

// UnmarshalJSON implements the dual-shape decode.
func (a *audClaim) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*a = arr
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*a = []string{s}
	return nil
}

// Matches reports whether aud contains a canonically-equal match for
// expected. Empty expected matches nothing (audience binding must be
// explicit).
//
// RFC 8707 §2 mandates that audience comparison happen after URL
// canonicalization -- a spec-compliant AS may emit the audience with a
// different trailing-slash / port / case than the operator configured on
// the resource server, and exact-string compare would reject valid tokens.
// We normalize both sides via canonicalAudience (scheme+host+port+path,
// default ports elided, IPv6 bracketed, trailing slashes stripped).
//
// Non-URL audiences (rare; RFC 7519 permits any string) fall back to exact
// compare so a token asserting `aud: "my-service"` still matches a config
// audience of `"my-service"` verbatim.
func (a audClaim) Matches(expected string) bool {
	if expected == "" {
		return false
	}
	wantCanon := canonicalAudience(expected)
	for _, v := range a {
		if v == expected {
			return true
		}
		if wantCanon != "" {
			if gotCanon := canonicalAudience(v); gotCanon != "" && gotCanon == wantCanon {
				return true
			}
		}
	}
	return false
}

// jwtVerifyOptions carries everything the verifier needs from the caller.
// Keys is usually the cached JWKS; Clock and Leeway are overridable for tests.
type jwtVerifyOptions struct {
	ExpectedIssuer   string
	ExpectedAudience string
	RequiredScopes   []string
	Keys             jwksLookup // nil -> verify fails with errJWTNoKeys
	Clock            func() time.Time
	Leeway           time.Duration
}

// jwksLookup abstracts the JWKS cache so tests can inject a fake.
type jwksLookup interface {
	// LookupJWK returns the verification key for the given JWS kid. Returns
	// (nil, false) when the kid is unknown; the caller decides whether to
	// trigger a refresh and retry.
	LookupJWK(kid string) (crypto.PublicKey, bool)
	// Refresh forces a re-fetch of the JWKS document. Called at most once
	// per verification attempt when the initial lookup misses; an
	// implementation MAY rate-limit to defend against unknown-kid spraying.
	Refresh() error
}

// jwtVerifyResult carries what the caller needs after a successful verify.
type jwtVerifyResult struct {
	Subject string
	Scopes  []string
}

// Sentinel errors let callers (the OAuth strategy) render the right
// error_description on the 401 challenge.
var (
	errJWTMalformed         = errors.New("jwt: malformed token")
	errJWTAlgNone           = errors.New("jwt: alg=none rejected")
	errJWTAlgUnsupported    = errors.New("jwt: unsupported alg")
	errJWTBadSignature      = errors.New("jwt: signature does not verify")
	errJWTNoKeys            = errors.New("jwt: no verification keys available")
	errJWTUnknownKid        = errors.New("jwt: unknown kid after refresh")
	errJWTExpired           = errors.New("jwt: token expired")
	errJWTNotYetValid       = errors.New("jwt: token not yet valid")
	errJWTMissingExp        = errors.New("jwt: missing exp claim")
	errJWTMissingSub        = errors.New("jwt: missing sub claim")
	errJWTUnsafeSub         = errors.New("jwt: sub contains control character")
	errJWTIssuerMismatch    = errors.New("jwt: issuer mismatch")
	errJWTAudienceMismatch  = errors.New("jwt: audience mismatch")
	errJWTInsufficientScope = errors.New("jwt: insufficient scope")
)

// isSafeSubject reports whether the JWT sub claim is free of control
// characters AND valid UTF-8. Rejecting C0 / C1 + DEL defends downstream
// logs and structured output from line-injection / escape-sequence attacks;
// enforcing UTF-8 validity ensures that a malformed encoding (overlong, lone
// continuation byte, surrogate) cannot round-trip through UTF-8-strict
// serializers as U+FFFD and produce a different identity string than the
// byte-scan admitted here. Non-ASCII Unicode names are accepted.
func isSafeSubject(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if c < 0x20 || c == 0x7F {
			return false
		}
	}
	return true
}

// verifyJWT parses, verifies, and validates claims on a JWT. The opts carry
// issuer/audience/scope expectations plus the key source. Returns the
// authenticated subject + scopes on success.
func verifyJWT(token string, opts jwtVerifyOptions) (jwtVerifyResult, error) {
	header, claims, signingInput, signature, err := parseJWT(token)
	if err != nil {
		return jwtVerifyResult{}, err
	}

	// alg gate: explicit "none" rejection + supported-set check.
	if strings.EqualFold(header.Alg, "none") {
		return jwtVerifyResult{}, errJWTAlgNone
	}
	if !isSupportedAlg(header.Alg) {
		return jwtVerifyResult{}, fmt.Errorf("%w: %q", errJWTAlgUnsupported, header.Alg)
	}

	// Key lookup. One refresh attempt on miss so a rotating AS key does not
	// immediately invalidate every live token; subsequent misses are
	// unknown-kid outright.
	if opts.Keys == nil {
		return jwtVerifyResult{}, errJWTNoKeys
	}
	key, ok := opts.Keys.LookupJWK(header.Kid)
	if !ok {
		if refreshErr := opts.Keys.Refresh(); refreshErr != nil {
			return jwtVerifyResult{}, fmt.Errorf("jwt: kid %q lookup failed: %w", header.Kid, refreshErr)
		}
		key, ok = opts.Keys.LookupJWK(header.Kid)
		if !ok {
			return jwtVerifyResult{}, fmt.Errorf("%w: kid=%q", errJWTUnknownKid, header.Kid)
		}
	}

	if err := verifySignature(header.Alg, key, signingInput, signature); err != nil {
		return jwtVerifyResult{}, err
	}

	// Claim-time checks use the caller's clock (injectable for tests).
	now := time.Now
	if opts.Clock != nil {
		now = opts.Clock
	}
	currentTime := now()
	leeway := opts.Leeway
	if leeway <= 0 {
		leeway = 60 * time.Second
	}
	// RFC 7519: exp is OPTIONAL in general but MCP OAuth 2.1 access tokens
	// MUST have a bounded lifetime. Reject tokens without exp outright.
	if claims.ExpiresAt == 0 {
		return jwtVerifyResult{}, errJWTMissingExp
	}
	if currentTime.Add(-leeway).Unix() > claims.ExpiresAt {
		return jwtVerifyResult{}, errJWTExpired
	}
	if claims.NotBefore > 0 && currentTime.Add(leeway).Unix() < claims.NotBefore {
		return jwtVerifyResult{}, errJWTNotYetValid
	}
	if opts.ExpectedIssuer != "" && claims.Issuer != opts.ExpectedIssuer {
		return jwtVerifyResult{}, errJWTIssuerMismatch
	}
	if !claims.Audience.Matches(opts.ExpectedAudience) {
		return jwtVerifyResult{}, errJWTAudienceMismatch
	}
	// sub identifies the authenticated principal; Phase 4 task scoping
	// relies on it, so an empty sub is a rejection rather than an
	// anonymous-session fallback. Control characters (including CR/LF) are
	// refused to defang log injection when Phase 4 logs the subject as the
	// task-owner scope key.
	if claims.Subject == "" {
		return jwtVerifyResult{}, errJWTMissingSub
	}
	if !isSafeSubject(claims.Subject) {
		return jwtVerifyResult{}, errJWTUnsafeSub
	}

	scopes := splitScope(claims.Scope)
	for _, required := range opts.RequiredScopes {
		if !containsScope(scopes, required) {
			return jwtVerifyResult{}, fmt.Errorf("%w: missing %q", errJWTInsufficientScope, required)
		}
	}

	return jwtVerifyResult{Subject: claims.Subject, Scopes: scopes}, nil
}

// parseJWT splits a compact JWS and decodes the header + claims.
// Returns the signing input (header.payload as bytes) so signature verification
// can hash it directly without re-joining.
func parseJWT(token string) (jwtHeader, jwtClaims, []byte, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, jwtClaims{}, nil, nil, fmt.Errorf("%w: want 3 segments, got %d", errJWTMalformed, len(parts))
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, nil, nil, fmt.Errorf("%w: header base64: %w", errJWTMalformed, err)
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, nil, nil, fmt.Errorf("%w: payload base64: %w", errJWTMalformed, err)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return jwtHeader{}, jwtClaims{}, nil, nil, fmt.Errorf("%w: signature base64: %w", errJWTMalformed, err)
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return jwtHeader{}, jwtClaims{}, nil, nil, fmt.Errorf("%w: header json: %w", errJWTMalformed, err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return jwtHeader{}, jwtClaims{}, nil, nil, fmt.Errorf("%w: claims json: %w", errJWTMalformed, err)
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	return header, claims, signingInput, signature, nil
}

// isSupportedAlg reports whether alg is in the accept list. HMAC (HS256 etc)
// is deliberately rejected because the resource server holds no symmetric
// secret with the AS -- any HS token presented to us is either misconfigured
// or malicious.
func isSupportedAlg(alg string) bool {
	switch alg {
	case "RS256", "RS384", "RS512", "ES256", "ES384":
		return true
	}
	return false
}

// verifySignature dispatches to the right stdlib primitive based on alg.
func verifySignature(alg string, key crypto.PublicKey, signingInput, signature []byte) error {
	switch alg {
	case "RS256":
		return verifyRSA(key, crypto.SHA256, signingInput, signature)
	case "RS384":
		return verifyRSA(key, crypto.SHA384, signingInput, signature)
	case "RS512":
		return verifyRSA(key, crypto.SHA512, signingInput, signature)
	case "ES256":
		return verifyECDSA(key, crypto.SHA256, signingInput, signature, 32)
	case "ES384":
		return verifyECDSA(key, crypto.SHA384, signingInput, signature, 48)
	default:
		return fmt.Errorf("%w: %q", errJWTAlgUnsupported, alg)
	}
}

func verifyRSA(key crypto.PublicKey, hash crypto.Hash, signingInput, signature []byte) error {
	pub, ok := key.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("%w: key type %T is not *rsa.PublicKey", errJWTBadSignature, key)
	}
	digest := hashBytes(hash, signingInput)
	if err := rsa.VerifyPKCS1v15(pub, hash, digest, signature); err != nil {
		return errJWTBadSignature
	}
	return nil
}

func verifyECDSA(key crypto.PublicKey, hash crypto.Hash, signingInput, signature []byte, coordLen int) error {
	pub, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("%w: key type %T is not *ecdsa.PublicKey", errJWTBadSignature, key)
	}
	// RFC 7518 Section 3.4: JWS ECDSA signature is the fixed-length concat
	// of R || S, each padded to the curve size. stdlib ecdsa.Verify needs
	// big.Int operands.
	if len(signature) != 2*coordLen {
		return fmt.Errorf("%w: ecdsa signature length %d, want %d", errJWTBadSignature, len(signature), 2*coordLen)
	}
	r := new(big.Int).SetBytes(signature[:coordLen])
	s := new(big.Int).SetBytes(signature[coordLen:])
	digest := hashBytes(hash, signingInput)
	if !ecdsa.Verify(pub, digest, r, s) {
		return errJWTBadSignature
	}
	return nil
}

// hashBytes runs the crypto.Hash over the input and returns the digest.
func hashBytes(h crypto.Hash, input []byte) []byte {
	switch h {
	case crypto.SHA256:
		d := sha256.Sum256(input)
		return d[:]
	case crypto.SHA384:
		d := sha512.Sum384(input)
		return d[:]
	case crypto.SHA512:
		d := sha512.Sum512(input)
		return d[:]
	default:
		// Unreachable given verifySignature's dispatch table.
		panic("BUG: unsupported hash")
	}
}

// splitScope splits a space-separated scope claim into tokens. RFC 8693
// Section 4.2 mandates space separator; empty / missing scope is returned
// as nil.
func splitScope(scope string) []string {
	if scope == "" {
		return nil
	}
	return strings.Fields(scope)
}

// containsScope reports whether needle is an exact element of scopes.
func containsScope(scopes []string, needle string) bool {
	return slices.Contains(scopes, needle)
}
