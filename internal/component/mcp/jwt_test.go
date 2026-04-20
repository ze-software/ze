package mcp

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// test helpers
// -----------------------------------------------------------------------------

// stubJWKS is a minimal jwksLookup stub: holds one kid -> key mapping plus a
// refresh counter so tests can assert on refresh behavior.
type stubJWKS struct {
	keys         map[string]crypto.PublicKey
	refreshCalls int
	refreshErr   error
	// addOnRefresh allows a refresh to surface a new kid that was missing at
	// first lookup (simulates AS key rotation).
	addOnRefresh map[string]crypto.PublicKey
}

func (s *stubJWKS) LookupJWK(kid string) (crypto.PublicKey, bool) {
	k, ok := s.keys[kid]
	return k, ok
}

func (s *stubJWKS) Refresh() error {
	s.refreshCalls++
	if s.refreshErr != nil {
		return s.refreshErr
	}
	for k, v := range s.addOnRefresh {
		if s.keys == nil {
			s.keys = map[string]crypto.PublicKey{}
		}
		s.keys[k] = v
	}
	return nil
}

// signRS256 signs header.payload with the given RSA key and returns a compact
// JWT string. Used to produce valid test tokens without pulling in a JWT lib.
func signRS256(t *testing.T, priv *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign RS256: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// signES256 produces an ES256 JWT with the stdlib ecdsa primitive.
func signES256(t *testing.T, priv *ecdsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	headerJSON, _ := json.Marshal(map[string]string{"alg": "ES256", "kid": kid, "typ": "JWT"})
	payloadJSON, _ := json.Marshal(claims)
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatalf("sign ES256: %v", err)
	}
	// RFC 7518: fixed-length R || S padded to curve size (32 bytes for P-256).
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):], sBytes)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// standardClaims builds a valid claim set for a given expiry offset.
func standardClaims(now time.Time, issuer, aud string, expIn time.Duration) map[string]any {
	return map[string]any{
		"iss":   issuer,
		"sub":   "alice",
		"aud":   aud,
		"exp":   now.Add(expIn).Unix(),
		"nbf":   now.Add(-time.Minute).Unix(),
		"iat":   now.Unix(),
		"scope": "mcp.read mcp.write",
	}
}

func newFixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// -----------------------------------------------------------------------------
// Positive paths: RS256, ES256
// -----------------------------------------------------------------------------

func TestVerifyJWT_RS256(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa generate: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "kid-1", standardClaims(now, "https://as/", "https://mcp/", time.Hour))

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"kid-1": &priv.PublicKey}}
	res, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "https://as/",
		ExpectedAudience: "https://mcp/",
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if err != nil {
		t.Fatalf("verifyJWT: %v", err)
	}
	if res.Subject != "alice" {
		t.Fatalf("subject = %q, want alice", res.Subject)
	}
	if len(res.Scopes) != 2 {
		t.Fatalf("scopes = %v, want 2 entries", res.Scopes)
	}
}

func TestVerifyJWT_ES256(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa generate: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	token := signES256(t, priv, "kid-es", standardClaims(now, "https://as/", "https://mcp/", time.Hour))

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"kid-es": &priv.PublicKey}}
	res, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "https://as/",
		ExpectedAudience: "https://mcp/",
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if err != nil {
		t.Fatalf("verifyJWT: %v", err)
	}
	if res.Subject != "alice" {
		t.Fatalf("subject = %q, want alice", res.Subject)
	}
}

// -----------------------------------------------------------------------------
// Negative paths
// -----------------------------------------------------------------------------

func TestVerifyJWT_RejectAlgNone(t *testing.T) {
	// Token with "alg":"none" and no signature segment.
	headerJSON, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	payloadJSON, _ := json.Marshal(map[string]any{"sub": "x"})
	token := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON) + "."

	_, err := verifyJWT(token, jwtVerifyOptions{Keys: &stubJWKS{}, ExpectedAudience: "x"})
	if !errors.Is(err, errJWTAlgNone) {
		t.Fatalf("expected errJWTAlgNone, got %v", err)
	}
}

func TestVerifyJWT_RejectHMACAlg(t *testing.T) {
	// HS256 is symmetric; the resource server never accepts one.
	headerJSON, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payloadJSON, _ := json.Marshal(map[string]any{"sub": "x"})
	token := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON) + "." +
		base64.RawURLEncoding.EncodeToString([]byte("fake-hmac"))

	_, err := verifyJWT(token, jwtVerifyOptions{Keys: &stubJWKS{}, ExpectedAudience: "x"})
	if !errors.Is(err, errJWTAlgUnsupported) {
		t.Fatalf("expected errJWTAlgUnsupported, got %v", err)
	}
}

func TestVerifyJWT_RejectExpired(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	// exp is 2 minutes ago, leeway default (60s) cannot save it.
	token := signRS256(t, priv, "k", standardClaims(now, "iss", "aud", -2*time.Minute))

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "iss",
		ExpectedAudience: "aud",
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if !errors.Is(err, errJWTExpired) {
		t.Fatalf("expected errJWTExpired, got %v", err)
	}
}

func TestVerifyJWT_RejectNotYetValid(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	claims := map[string]any{
		"iss": "iss",
		"aud": "aud",
		"exp": now.Add(time.Hour).Unix(),
		// nbf is 2 minutes in the future; leeway can't bridge.
		"nbf": now.Add(2 * time.Minute).Unix(),
	}
	token := signRS256(t, priv, "k", claims)

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer: "iss", ExpectedAudience: "aud",
		Keys: keys, Clock: newFixedClock(now),
	})
	if !errors.Is(err, errJWTNotYetValid) {
		t.Fatalf("expected errJWTNotYetValid, got %v", err)
	}
}

func TestVerifyJWT_RejectIssuerMismatch(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "k", standardClaims(now, "https://other/", "aud", time.Hour))

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "https://as/",
		ExpectedAudience: "aud",
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if !errors.Is(err, errJWTIssuerMismatch) {
		t.Fatalf("expected errJWTIssuerMismatch, got %v", err)
	}
}

func TestVerifyJWT_RejectAudienceMismatch(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "k", standardClaims(now, "iss", "https://wrong/", time.Hour))

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "iss",
		ExpectedAudience: "https://mcp/",
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if !errors.Is(err, errJWTAudienceMismatch) {
		t.Fatalf("expected errJWTAudienceMismatch, got %v", err)
	}
}

func TestVerifyJWT_AudienceArrayForm(t *testing.T) {
	// RFC 7519 allows aud to be a string OR array. Array form must work.
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	claims := map[string]any{
		"iss": "iss",
		"aud": []string{"https://one/", "https://mcp/"},
		"exp": now.Add(time.Hour).Unix(),
		"sub": "alice",
	}
	token := signRS256(t, priv, "k", claims)

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	res, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "iss",
		ExpectedAudience: "https://mcp/",
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if err != nil {
		t.Fatalf("verifyJWT: %v", err)
	}
	if res.Subject != "alice" {
		t.Fatalf("subject = %q, want alice", res.Subject)
	}
}

func TestVerifyJWT_RejectBadSignature(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "k", standardClaims(now, "iss", "aud", time.Hour))
	// Tamper with the payload -- any byte flip invalidates the signature.
	parts := strings.Split(token, ".")
	tampered := parts[0] + "." + parts[1] + "X." + parts[2]

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	_, err := verifyJWT(tampered, jwtVerifyOptions{
		ExpectedIssuer: "iss", ExpectedAudience: "aud",
		Keys: keys, Clock: newFixedClock(now),
	})
	if err == nil {
		t.Fatal("expected error on tampered token")
	}
}

func TestVerifyJWT_RejectMissingExp(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	claims := map[string]any{
		"iss": "iss",
		"aud": "aud",
		"sub": "alice",
		// no exp
	}
	token := signRS256(t, priv, "k", claims)
	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer: "iss", ExpectedAudience: "aud",
		Keys: keys, Clock: newFixedClock(now),
	})
	if !errors.Is(err, errJWTMissingExp) {
		t.Fatalf("expected errJWTMissingExp, got %v", err)
	}
}

func TestVerifyJWT_RejectMissingSub(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	claims := map[string]any{
		"iss": "iss",
		"aud": "aud",
		"exp": now.Add(time.Hour).Unix(),
		// no sub
	}
	token := signRS256(t, priv, "k", claims)
	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer: "iss", ExpectedAudience: "aud",
		Keys: keys, Clock: newFixedClock(now),
	})
	if !errors.Is(err, errJWTMissingSub) {
		t.Fatalf("expected errJWTMissingSub, got %v", err)
	}
}

func TestVerifyJWT_RejectMalformed(t *testing.T) {
	cases := []string{
		"",
		"only-one",
		"two.parts",
		"a.b.c.d",
		"!.!.!",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			_, err := verifyJWT(tc, jwtVerifyOptions{Keys: &stubJWKS{}, ExpectedAudience: "aud"})
			if err == nil {
				t.Fatalf("expected error on malformed %q", tc)
			}
		})
	}
}

func TestVerifyJWT_UnknownKidTriggersOneRefresh(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "kid-rotated", standardClaims(now, "iss", "aud", time.Hour))

	// Initial JWKS has no key for kid-rotated; refresh surfaces it.
	keys := &stubJWKS{
		keys:         map[string]crypto.PublicKey{},
		addOnRefresh: map[string]crypto.PublicKey{"kid-rotated": &priv.PublicKey},
	}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer: "iss", ExpectedAudience: "aud",
		Keys: keys, Clock: newFixedClock(now),
	})
	if err != nil {
		t.Fatalf("verify after refresh: %v", err)
	}
	if keys.refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", keys.refreshCalls)
	}
}

func TestVerifyJWT_RejectUnknownKidAfterRefresh(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "ghost-kid", standardClaims(now, "iss", "aud", time.Hour))

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{}} // no addOnRefresh
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer: "iss", ExpectedAudience: "aud",
		Keys: keys, Clock: newFixedClock(now),
	})
	if !errors.Is(err, errJWTUnknownKid) {
		t.Fatalf("expected errJWTUnknownKid, got %v", err)
	}
}

func TestVerifyJWT_InsufficientScope(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	claims := standardClaims(now, "iss", "aud", time.Hour)
	claims["scope"] = "mcp.read"
	token := signRS256(t, priv, "k", claims)

	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "iss",
		ExpectedAudience: "aud",
		RequiredScopes:   []string{"mcp.read", "mcp.admin"},
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if !errors.Is(err, errJWTInsufficientScope) {
		t.Fatalf("expected errJWTInsufficientScope, got %v", err)
	}
}

func TestVerifyJWT_NoKeysConfigured(t *testing.T) {
	_, err := verifyJWT("a.b.c", jwtVerifyOptions{ExpectedAudience: "x"})
	if err == nil {
		t.Fatal("expected error when Keys is nil")
	}
}

func TestVerifyJWT_RefreshErrorPropagates(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "kid", standardClaims(now, "iss", "aud", time.Hour))
	keys := &stubJWKS{keys: map[string]crypto.PublicKey{}, refreshErr: errors.New("boom")}
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer: "iss", ExpectedAudience: "aud",
		Keys: keys, Clock: newFixedClock(now),
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected refresh error, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// audClaim
// -----------------------------------------------------------------------------

func TestAudClaim_UnmarshalStringAndArray(t *testing.T) {
	var a audClaim
	if err := a.UnmarshalJSON([]byte(`"one"`)); err != nil {
		t.Fatalf("string: %v", err)
	}
	if len(a) != 1 || a[0] != "one" {
		t.Fatalf("string decoded as %v", a)
	}
	var b audClaim
	if err := b.UnmarshalJSON([]byte(`["one","two"]`)); err != nil {
		t.Fatalf("array: %v", err)
	}
	if len(b) != 2 || b[1] != "two" {
		t.Fatalf("array decoded as %v", b)
	}
}

func TestAudClaim_Matches(t *testing.T) {
	a := audClaim{"https://one/", "https://two/"}
	if !a.Matches("https://one/") {
		t.Fatal("should match https://one/")
	}
	if a.Matches("https://three/") {
		t.Fatal("should not match unknown audience")
	}
	if a.Matches("") {
		t.Fatal("empty expected must never match")
	}
}

// -----------------------------------------------------------------------------
// ES256 signature length defense (tamper path)
// -----------------------------------------------------------------------------

func TestVerifyECDSA_WrongSignatureLength(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	// Build a 63-byte fake signature (missing one byte). Our check must
	// reject before invoking ecdsa.Verify.
	fake := make([]byte, 63)
	err := verifyECDSA(&priv.PublicKey, crypto.SHA256, []byte("x"), fake, 32)
	if err == nil {
		t.Fatal("expected error on wrong ecdsa sig length")
	}
}

// -----------------------------------------------------------------------------
// splitScope / containsScope
// -----------------------------------------------------------------------------

func TestSplitScope(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a b c", []string{"a", "b", "c"}},
		{"a  b   c", []string{"a", "b", "c"}}, // strings.Fields eats runs of whitespace
	}
	for _, tc := range cases {
		got := splitScope(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("splitScope(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("splitScope(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

// sanity: rsa.PublicKey round-trip via signRS256 produces a valid signature we
// can re-verify through our helper directly (not the JWT path).
func TestRSASignature_Roundtrip(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	msg := []byte("hello")
	digest := sha256.Sum256(msg)
	sig, _ := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err := verifyRSA(&priv.PublicKey, crypto.SHA256, msg, sig); err != nil {
		t.Fatalf("verifyRSA: %v", err)
	}
	// Tamper with the message; verification must fail.
	if err := verifyRSA(&priv.PublicKey, crypto.SHA256, []byte("goodbye"), sig); err == nil {
		t.Fatal("verifyRSA accepted wrong message")
	}
	_ = big.NewInt(0) // keep math/big import honest if refactored
}
