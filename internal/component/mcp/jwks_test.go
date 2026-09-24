package mcp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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
	srv := newTrustedTLSServer(t, mux)
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
	srv := newTrustedTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	cache := newJWKSCache(srv.URL+"/jwks", nil, 0, 0)
	if err := cache.Refresh(); err == nil {
		t.Fatal("expected error on 500")
	}
}

// TestJWKSCacheDoesNotUseExpiredKeys proves known keys are subject to expiry
// and a refresh rate limit cannot extend their authorization lifetime.
func TestJWKSCacheDoesNotUseExpiredKeys(t *testing.T) {
	srv, js := newJWKSServer(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	doc := map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "old")}}
	js.mu.Store(&doc)
	cache := newJWKSCache(srv.URL+"/jwks", nil, time.Minute, time.Hour)
	now := time.Unix(1_700_000_000, 0)
	cache.now = func() time.Time { return now }
	if _, ok := cache.LookupJWK("old"); !ok {
		t.Fatal("current key rejected")
	}
	replacement := map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "new")}}
	js.mu.Store(&replacement)
	now = now.Add(2 * time.Minute)
	if _, ok := cache.LookupJWK("old"); ok {
		t.Fatal("rate-limited refresh extended an expired key")
	}
	if js.hits.Load() != 1 {
		t.Fatal("refresh ignored its rate limit")
	}
	now = now.Add(time.Hour)
	if _, ok := cache.LookupJWK("old"); ok {
		t.Fatal("a known key survived its removal from the refreshed JWKS")
	}
	if _, ok := cache.LookupJWK("new"); !ok {
		t.Fatal("replacement key was not loaded")
	}
	if js.hits.Load() != 2 {
		t.Fatal("expired known-key lookup did not refresh once")
	}
}

// TestJWKSCacheRefreshFailureRejectsExpiredKeys makes the key endpoint
// unavailable after expiry and checks that cached keys cannot authorize tokens.
func TestJWKSCacheRefreshFailureRejectsExpiredKeys(t *testing.T) {
	srv, js := newJWKSServer(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	doc := map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "old")}}
	js.mu.Store(&doc)
	cache := newJWKSCache(srv.URL+"/jwks", nil, time.Minute, time.Second)
	now := time.Unix(1_700_000_000, 0)
	cache.now = func() time.Time { return now }
	if _, ok := cache.LookupJWK("old"); !ok {
		t.Fatal("current key rejected")
	}
	srv.Close()
	now = now.Add(2 * time.Minute)
	if _, ok := cache.LookupJWK("old"); ok {
		t.Fatal("failed refresh allowed an expired key")
	}
	if _, ok := cache.LookupJWK("old"); ok {
		t.Fatal("rate limit after a failed refresh allowed an expired key")
	}
}

func TestJWKSCache_OversizeBody(t *testing.T) {
	big := make([]byte, maxJWKSDocumentSize+100)
	for i := range big {
		big[i] = 'a'
	}
	srv := newTrustedTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	cache := newJWKSCache("https://"+ln.Addr().String()+"/jwks", client, 0, 0)
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

// VALIDATES: an EC JWK whose (x, y) is not a point on the named curve is
// refused when the document is PARSED.
//
// PREVENTS: the decode accepting any pair of integers. It used to build
// &ecdsa.PublicKey{Curve, X, Y} straight from the JWK, which validates nothing,
// and its own comment deferred the check to ecdsa.Verify. That is fail-open in
// the shape that matters: an off-curve key becomes a SIGNATURE MISMATCH rather
// than a rejected key, so an operator reading the logs sees "bad token" and
// looks at the client instead of at the JWKS the server fetched. Some curves
// also admit small-subgroup attacks against a point nobody checked.
// ecdsa.ParseUncompressedPublicKey refuses an off-curve point and the point at
// infinity, so the failure happens where the evidence still exists.
func TestParseJWKSDocumentRejectsOffCurveEC(t *testing.T) {
	t.Parallel()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa: %v", err)
	}
	jwk := ecJWK(t, priv, "bad")
	// Flip one coordinate. x and the real y no longer satisfy the curve
	// equation, and nothing about the encoding changes.
	bad := new(big.Int).Add(priv.X, big.NewInt(1)) //nolint:staticcheck // JWK export of public coordinates
	jwk["x"] = base64.RawURLEncoding.EncodeToString(bad.Bytes())

	body, err := json.Marshal(map[string]any{"keys": []map[string]any{jwk}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	keys, err := parseJWKSDocument(body)
	if err == nil && len(keys) > 0 {
		t.Fatalf("an off-curve EC key was accepted: %#v", keys)
	}
	if _, ok := keys["bad"]; ok {
		t.Error("an off-curve EC key reached the key set; it must be refused at parse")
	}
}

// A key expires at the TTL boundary, including when a failed refresh would
// otherwise leave the old signing material in the cache.
func TestJWKSCacheExpiryBoundary(t *testing.T) {
	srv, js := newJWKSServer(t)
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "old")}}
	js.mu.Store(&doc)
	now := time.Unix(1_700_000_000, 0)
	cache := newJWKSCache(srv.URL+"/jwks", nil, time.Minute, time.Second)
	cache.now = func() time.Time { return now }
	if _, ok := cache.LookupJWK("old"); !ok {
		t.Fatal("fresh signing key rejected")
	}
	srv.Close()
	now = now.Add(time.Minute)
	if _, ok := cache.LookupJWK("old"); ok {
		t.Fatal("expired signing key accepted at the TTL boundary")
	}
}

type jwksRoundTripper func(*http.Request) (*http.Response, error)

func (f jwksRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Concurrent requests must await the same successful refresh instead of
// rejecting the second token while the first request is fetching its key.
func TestJWKSCacheConcurrentLookupWaitsForRefresh(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"keys": []map[string]any{rsaJWK(t, priv, "key")}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	fetching := make(chan struct{})
	release := make(chan struct{})
	var fetches atomic.Int64
	client := &http.Client{Transport: jwksRoundTripper(func(r *http.Request) (*http.Response, error) {
		if fetches.Add(1) == 1 {
			close(fetching)
		}
		select {
		case <-release:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(body))),
			}, nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})}
	cache := newJWKSCache("https://as.example/jwks", client, time.Minute, time.Second)
	results := make(chan *rsa.PublicKey, 2)
	lookup := func() {
		key, ok := cache.LookupJWK("key")
		if !ok {
			results <- nil
			return
		}
		pub, _ := key.(*rsa.PublicKey)
		results <- pub
	}
	go lookup()
	select {
	case <-fetching:
	case <-ctx.Done():
		t.Fatal("first lookup did not start a key fetch")
	}
	secondStarted := make(chan struct{})
	go func() {
		close(secondStarted)
		lookup()
	}()
	select {
	case <-secondStarted:
	case <-ctx.Done():
		t.Fatal("second lookup did not start")
	}
	// Mutex contention is not durably blocked in testing/synctest. Keep the
	// fetch outstanding for a bounded real interval to observe early rejection.
	select {
	case <-results:
		t.Fatal("lookup returned before the in-flight key refresh completed")
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal("shared refresh timed out before release")
	}
	close(release)
	for range 2 {
		select {
		case key := <-results:
			if key == nil {
				t.Fatal("successful shared refresh rejected a known key")
			}
			if key.N.Cmp(priv.N) != 0 || key.E != priv.E {
				t.Fatal("shared refresh returned the wrong public key")
			}
		case <-ctx.Done():
			t.Fatal("lookup did not finish after the shared refresh")
		}
	}
	if fetches.Load() != 1 {
		t.Fatalf("fetches = %d, want one shared refresh", fetches.Load())
	}
}
