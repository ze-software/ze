// Design: docs/architecture/mcp/overview.md -- MCP OAuth resource server
//
// RFC 8414 Section 4, String Operations, as it binds the resource server
// when it compares the JWT `iss` claim (a JSON string) with the issuer the
// AS metadata reported: JSON escaping is removed first, no Unicode
// normalization is applied to either side, and the two strings are equal
// only when they are equal code point for code point.

package mcp

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"testing"
	"time"
)

// signRS256Payload signs a caller-written JSON payload so a test controls
// the exact JSON escaping of the `iss` string, which json.Marshal would
// choose on its own.
func signRS256Payload(t *testing.T, priv *rsa.PrivateKey, kid, payloadJSON string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"alg":"RS256","kid":"` + kid + `","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign RS256: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// issuerCompareFixture returns a signing key, its stub JWKS and a fixed
// clock so each test below differs only in the two issuer strings.
func issuerCompareFixture(t *testing.T) (*rsa.PrivateKey, *stubJWKS, time.Time) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa generate: %v", err)
	}
	keys := &stubJWKS{keys: map[string]crypto.PublicKey{"kid-1": &priv.PublicKey}}
	return priv, keys, time.Unix(1_700_000_000, 0)
}

// verifyWithIssuer verifies a token minted with tokenIssuer against an
// expected issuer and returns the verifier's error.
func verifyWithIssuer(t *testing.T, tokenIssuer, expectedIssuer string) error {
	t.Helper()
	priv, keys, now := issuerCompareFixture(t)
	token := signRS256(t, priv, "kid-1", standardClaims(now, tokenIssuer, "https://mcp/", time.Hour))
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   expectedIssuer,
		ExpectedAudience: "https://mcp/",
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	return err
}

// Precomposed and decomposed spellings of "é": equal under NFC or NFD
// normalization, different as code points.
const (
	issuerNFC = "https://as.example/\u00e9"
	issuerNFD = "https://as.example/e\u0301"
)

// VALIDATES: a JSON-escaped iss is unescaped before the comparison.
// RFC requirement: RFC8414-4-1 positive -- an iss claim written with JSON escapes
// ("https:\/\/as.example\/é") is unescaped to its code points before the comparison, so it
// matches the metadata issuer https://as.example/é and the token verifies.
func TestRFC8414IssuerCompareUnescapesJSON(t *testing.T) {
	priv, keys, now := issuerCompareFixture(t)
	payload := `{"iss":"https:\/\/as.example\/\u00e9","sub":"alice","aud":"https://mcp/",` +
		`"exp":` + strconv.FormatInt(now.Add(time.Hour).Unix(), 10) +
		`,"iat":` + strconv.FormatInt(now.Unix(), 10) + `}`
	token := signRS256Payload(t, priv, "kid-1", payload)
	res, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   issuerNFC,
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

// VALIDATES: the comparison runs on code points, never on the JSON text.
// RFC requirement: RFC8414-4-1 negative -- an expected issuer spelled as the raw JSON text
// (https:\/\/as.example\/é, backslashes included) does not match a token whose iss unescapes
// to https://as.example/é: the token is refused with errJWTIssuerMismatch.
func TestRFC8414IssuerCompareNotOnJSONText(t *testing.T) {
	priv, keys, now := issuerCompareFixture(t)
	payload := `{"iss":"https:\/\/as.example\/\u00e9","sub":"alice","aud":"https://mcp/",` +
		`"exp":` + strconv.FormatInt(now.Add(time.Hour).Unix(), 10) +
		`,"iat":` + strconv.FormatInt(now.Unix(), 10) + `}`
	token := signRS256Payload(t, priv, "kid-1", payload)
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   `https:\/\/as.example\/\u00e9`,
		ExpectedAudience: "https://mcp/",
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if !errors.Is(err, errJWTIssuerMismatch) {
		t.Fatalf("error = %v, want errJWTIssuerMismatch", err)
	}
}

// VALIDATES: identical decomposed strings match without any normalization.
// RFC requirement: RFC8414-4-2 positive -- an iss and an expected issuer both spelled in the
// decomposed form https://as.example/e+U+0301 are equal code point for code point and the token
// verifies, with no normalization needed on either side.
func TestRFC8414IssuerCompareNoNormalizationAccepts(t *testing.T) {
	if err := verifyWithIssuer(t, issuerNFD, issuerNFD); err != nil {
		t.Fatalf("verifyJWT: %v", err)
	}
}

// VALIDATES: NFC and NFD spellings of one issuer never match.
// RFC requirement: RFC8414-4-2 negative -- an iss in the precomposed form https://as.example/U+00E9
// against an expected issuer in the decomposed form https://as.example/e+U+0301 is refused with
// errJWTIssuerMismatch: normalizing either side would have made them equal.
func TestRFC8414IssuerCompareNoNormalizationRejects(t *testing.T) {
	err := verifyWithIssuer(t, issuerNFC, issuerNFD)
	if !errors.Is(err, errJWTIssuerMismatch) {
		t.Fatalf("error = %v, want errJWTIssuerMismatch", err)
	}
	err = verifyWithIssuer(t, issuerNFD, issuerNFC)
	if !errors.Is(err, errJWTIssuerMismatch) {
		t.Fatalf("reversed: error = %v, want errJWTIssuerMismatch", err)
	}
}

// VALIDATES: an iss equal code point for code point verifies.
// RFC requirement: RFC8414-4-3 positive -- an iss equal to the expected issuer code point for code
// point, https://as.example/é on both sides, verifies.
func TestRFC8414IssuerCompareCodePointsEqual(t *testing.T) {
	if err := verifyWithIssuer(t, issuerNFC, issuerNFC); err != nil {
		t.Fatalf("verifyJWT: %v", err)
	}
}

// VALIDATES: one differing code point refuses the token, with no case folding.
// RFC requirement: RFC8414-4-3 negative -- an iss that differs from the expected issuer in one
// code point (https://AS.example/é against https://as.example/é) is refused with
// errJWTIssuerMismatch: the comparison is code-point equality, not a case-insensitive one.
func TestRFC8414IssuerCompareCodePointsDiffer(t *testing.T) {
	err := verifyWithIssuer(t, "https://AS.example/\u00e9", issuerNFC)
	if !errors.Is(err, errJWTIssuerMismatch) {
		t.Fatalf("error = %v, want errJWTIssuerMismatch", err)
	}
}
