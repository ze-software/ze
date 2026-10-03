package mcp

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

// escapedAudienceResource is a resource identifier whose query holds an
// ampersand. encoding/json writes it as \u0026, so the signed token carries a
// JSON-escaped spelling of the identifier ze publishes.
const escapedAudienceResource = "https://ze.example/mcp?tenant=a&scope=b"

// signedAudienceToken signs a token whose aud claim is aud and checks the
// escaping the test depends on is really in the payload.
func signedAudienceToken(t *testing.T, aud, wantInPayload string) (string, *stubJWKS, time.Time) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa generate: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "k", standardClaims(now, "iss", aud, time.Hour))
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !bytes.Contains(payload, []byte(wantInPayload)) {
		t.Fatalf("payload %s does not carry %s", payload, wantInPayload)
	}
	return token, &stubJWKS{keys: map[string]crypto.PublicKey{"k": &priv.PublicKey}}, now
}

// TestRFC9728EscapedAudienceMatchesResource compares a JSON-escaped aud claim
// with the resource identifier ze expects.
//
// VALIDATES: RFC 9728 Section 6 step 1 -- "Remove any JSON-applied escaping to
// produce an array of Unicode code points" before the comparison.
// PREVENTS: comparing the raw JSON bytes of the claim, which refuses a token
// issued for this resource.
func TestRFC9728EscapedAudienceMatchesResource(t *testing.T) {
	// RFC requirement: RFC9728-6-3 positive -- an aud claim whose ampersand is JSON-escaped as \u0026 matches the unescaped resource identifier, and verifyJWT accepts the token.
	token, keys, now := signedAudienceToken(t, escapedAudienceResource, `\u0026`)
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "iss",
		ExpectedAudience: escapedAudienceResource,
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if err != nil {
		t.Fatalf("verifyJWT refused a token for this resource: %v", err)
	}
}

// TestRFC9728EscapeTextIsNotUnescapedTwice sends an aud claim whose code points
// are the six characters of an escape sequence.
//
// VALIDATES: RFC 9728 Section 6 step 1 removes the escaping JSON applied, once.
// The escape text inside the decoded string stays text.
// PREVENTS: a comparison that unescapes the decoded string again and matches a
// different identifier.
func TestRFC9728EscapeTextIsNotUnescapedTwice(t *testing.T) {
	// RFC requirement: RFC9728-6-3 negative -- an aud claim holding the literal text \u0026, JSON-escaped as \\u0026, does not match the identifier that holds an ampersand, and verifyJWT refuses the token with errJWTAudienceMismatch.
	literal := strings.Replace(escapedAudienceResource, "&", `\u0026`, 1)
	token, keys, now := signedAudienceToken(t, literal, `\\u0026`)
	_, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "iss",
		ExpectedAudience: escapedAudienceResource,
		Keys:             keys,
		Clock:            newFixedClock(now),
	})
	if !errors.Is(err, errJWTAudienceMismatch) {
		t.Fatalf("expected errJWTAudienceMismatch, got %v", err)
	}
}
