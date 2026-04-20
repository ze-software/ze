package mcp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// test helpers
// -----------------------------------------------------------------------------

// jwksServer is a tiny AS-like JWKS endpoint the test drives.
type jwksServer struct {
	mu   *atomic.Pointer[map[string]any]
	hits *atomic.Int64
}

func newJWKSServer(t *testing.T) (*httptest.Server, *jwksServer) {
	t.Helper()
	mu := &atomic.Pointer[map[string]any]{}
	hits := &atomic.Int64{}
	empty := map[string]any{"keys": []any{}}
	mu.Store(&empty)
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		body, err := json.Marshal(*mu.Load())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, werr := w.Write(body); werr != nil {
			t.Logf("write body: %v", werr)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &jwksServer{mu: mu, hits: hits}
}

func rsaJWK(t *testing.T, priv *rsa.PrivateKey, kid string) map[string]any {
	t.Helper()
	return map[string]any{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"use": "sig",
		"n":   base64.RawURLEncoding.EncodeToString(priv.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(intToBigEndian(priv.E)),
	}
}

func ecJWK(t *testing.T, priv *ecdsa.PrivateKey, kid string) map[string]any {
	t.Helper()
	return map[string]any{
		"kty": "EC",
		"kid": kid,
		"alg": "ES256",
		"use": "sig",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(priv.X.Bytes()), //nolint:staticcheck // JWK export of public coordinates
		"y":   base64.RawURLEncoding.EncodeToString(priv.Y.Bytes()), //nolint:staticcheck // JWK export of public coordinates
	}
}

// intToBigEndian returns the minimal big-endian byte representation of a
// non-negative int, as JWK wants (e=65537 -> 3 bytes).
func intToBigEndian(n int) []byte {
	buf := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		buf[i] = byte(n & 0xff)
		n >>= 8
	}
	first := 0
	for first < 7 && buf[first] == 0 {
		first++
	}
	return buf[first:]
}

// -----------------------------------------------------------------------------
// parseJWKSDocument
// -----------------------------------------------------------------------------

func TestParseJWKSDocument_RSA(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	body, err := json.Marshal(map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "k1")}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	keys, err := parseJWKSDocument(body)
	if err != nil {
		t.Fatalf("parseJWKSDocument: %v", err)
	}
	got, ok := keys["k1"].(*rsa.PublicKey)
	if !ok {
		t.Fatalf("key k1 type = %T, want *rsa.PublicKey", keys["k1"])
	}
	if got.N.Cmp(priv.N) != 0 {
		t.Fatal("decoded N does not match source")
	}
}

func TestParseJWKSDocument_EC(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa: %v", err)
	}
	body, err := json.Marshal(map[string]any{"keys": []map[string]any{ecJWK(t, priv, "k2")}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	keys, err := parseJWKSDocument(body)
	if err != nil {
		t.Fatalf("parseJWKSDocument: %v", err)
	}
	if _, ok := keys["k2"].(*ecdsa.PublicKey); !ok {
		t.Fatalf("key k2 type = %T, want *ecdsa.PublicKey", keys["k2"])
	}
}

func TestParseJWKSDocument_SkipsMalformed(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	body, err := json.Marshal(map[string]any{"keys": []map[string]any{
		{"kty": "RSA", "kid": "bad", "n": "$$$", "e": "AQAB"}, // malformed n
		rsaJWK(t, priv, "good"),
	}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	keys, err := parseJWKSDocument(body)
	if err != nil {
		t.Fatalf("parseJWKSDocument: %v", err)
	}
	if _, ok := keys["bad"]; ok {
		t.Fatal("malformed key was accepted")
	}
	if _, ok := keys["good"]; !ok {
		t.Fatal("good key missing")
	}
}

func TestParseJWKSDocument_RejectsEmpty(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"keys": []any{}})
	if _, err := parseJWKSDocument(body); err == nil {
		t.Fatal("expected error on empty key set")
	}
}

func TestParseJWKSDocument_SkipsEncryptionKey(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	encKey := rsaJWK(t, priv, "enc-key")
	encKey["use"] = "enc"
	sigKey := rsaJWK(t, priv, "sig-key")
	body, err := json.Marshal(map[string]any{"keys": []map[string]any{encKey, sigKey}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	keys, err := parseJWKSDocument(body)
	if err != nil {
		t.Fatalf("parseJWKSDocument: %v", err)
	}
	if _, ok := keys["enc-key"]; ok {
		t.Fatal("use=enc key should be filtered out")
	}
	if _, ok := keys["sig-key"]; !ok {
		t.Fatal("use=sig key missing")
	}
}

// -----------------------------------------------------------------------------
// jwksCache live HTTP
// -----------------------------------------------------------------------------

func TestJWKSCache_FetchAndLookup(t *testing.T) {
	srv, js := newJWKSServer(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	doc := map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "k1")}}
	js.mu.Store(&doc)

	cache := newJWKSCache(srv.URL+"/jwks", nil, 0, 0)
	if _, ok := cache.LookupJWK("k1"); !ok {
		t.Fatal("first lookup missed")
	}
	if _, ok := cache.LookupJWK("k1"); !ok {
		t.Fatal("second lookup missed")
	}
	if js.hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", js.hits.Load())
	}
}

func TestJWKSCache_RefreshRateLimit(t *testing.T) {
	srv, js := newJWKSServer(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	doc := map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "k1")}}
	js.mu.Store(&doc)

	cache := newJWKSCache(srv.URL+"/jwks", nil, time.Hour, time.Hour)
	if err := cache.Refresh(); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if err := cache.Refresh(); err != nil {
		t.Fatalf("rate-limited refresh returned error: %v", err)
	}
	if js.hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1 (second refresh should be rate-limited)", js.hits.Load())
	}
}

func TestJWKSCache_RefreshWhenClockAdvances(t *testing.T) {
	srv, js := newJWKSServer(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	doc := map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "k1")}}
	js.mu.Store(&doc)

	cache := newJWKSCache(srv.URL+"/jwks", nil, time.Hour, 10*time.Second)
	fakeNow := time.Unix(1_700_000_000, 0)
	cache.now = func() time.Time { return fakeNow }

	if err := cache.Refresh(); err != nil {
		t.Fatalf("first: %v", err)
	}
	fakeNow = fakeNow.Add(15 * time.Second)
	if err := cache.Refresh(); err != nil {
		t.Fatalf("second: %v", err)
	}
	if js.hits.Load() != 2 {
		t.Fatalf("hits = %d, want 2", js.hits.Load())
	}
}

func TestJWKSCache_FetchHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	cache := newJWKSCache(srv.URL+"/jwks", nil, 0, 0)
	if err := cache.Refresh(); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestJWKSCache_OversizeBody(t *testing.T) {
	big := make([]byte, maxJWKSDocumentSize+100)
	for i := range big {
		big[i] = 'a'
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, werr := w.Write(big); werr != nil {
			t.Logf("write: %v", werr)
		}
	}))
	t.Cleanup(srv.Close)
	cache := newJWKSCache(srv.URL+"/jwks", nil, 0, 0)
	if err := cache.Refresh(); err == nil {
		t.Fatal("expected error on oversize body")
	}
}

func TestJWKSCache_RefreshHonorsClientTimeout(t *testing.T) {
	// Plain listener that accepts but never writes; avoids the httptest
	// close-deadlock caused by handlers that block forever.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		if cerr := ln.Close(); cerr != nil {
			t.Logf("close listener: %v", cerr)
		}
	})
	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			return
		}
		<-t.Context().Done()
		if cerr := conn.Close(); cerr != nil {
			t.Logf("close conn: %v", cerr)
		}
	}()

	client := &http.Client{Timeout: 250 * time.Millisecond}
	cache := newJWKSCache("http://"+ln.Addr().String()+"/jwks", client, 0, 0)
	if err := cache.Refresh(); err == nil {
		t.Fatal("expected timeout error with short-timeout client")
	}
}

func TestJWKSCache_VerifyJWTIntegration(t *testing.T) {
	srv, js := newJWKSServer(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	doc := map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "live-key")}}
	js.mu.Store(&doc)

	cache := newJWKSCache(srv.URL+"/jwks", nil, 0, 0)
	now := time.Unix(1_700_000_000, 0)
	token := signRS256(t, priv, "live-key", standardClaims(now, "https://as/", "https://mcp/", time.Hour))

	var _ jwksLookup = cache // compile-time interface satisfaction
	res, err := verifyJWT(token, jwtVerifyOptions{
		ExpectedIssuer:   "https://as/",
		ExpectedAudience: "https://mcp/",
		Keys:             cache,
		Clock:            func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("verifyJWT via cache: %v", err)
	}
	if res.Subject != "alice" {
		t.Fatalf("subject = %q, want alice", res.Subject)
	}
}

// -----------------------------------------------------------------------------
// decodeJWK negatives
// -----------------------------------------------------------------------------

func TestDecodeRSAJWK_ImplausibleExponent(t *testing.T) {
	k := &jwk{Kty: "RSA", N: "AAAA", E: "AQ"} // e=1
	if _, err := decodeRSAJWK(k); err == nil {
		t.Fatal("expected error for e=1")
	}
}

func TestDecodeJWK_UnknownKty(t *testing.T) {
	if _, err := decodeJWK(&jwk{Kty: "OKP", Kid: "x"}); err == nil {
		t.Fatal("expected error for OKP kty")
	}
}
