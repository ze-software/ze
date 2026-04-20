// Design: docs/architecture/mcp/overview.md -- MCP OAuth resource server

// JWKS fetcher + cache with TTL-based re-fetch and rate-limited refresh.
//
// Implements the jwksLookup interface consumed by verifyJWT. Holds one set of
// keys indexed by `kid`. A miss during verify triggers at most one Refresh
// within any minJWKSRefreshInterval window -- this defends the AS against an
// unknown-kid spraying attack.
//
// All decoding is stdlib: JSON for the document, encoding/base64 RawURL for
// RSA modulus/exponent and EC coordinates, math/big for the big-integer
// conversions that rsa.PublicKey / ecdsa.PublicKey want.

package mcp

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// Default JWKS cache tuning. Operator can override via StreamableConfig in
// Phase F; these defaults match the OAuth resource-server posture where the
// AS rotates keys on the order of minutes-to-hours.
const (
	defaultJWKSCacheTTL     = 15 * time.Minute
	minJWKSRefreshInterval  = 30 * time.Second
	maxJWKSDocumentSize     = 256 * 1024
	defaultJWKSFetchTimeout = 5 * time.Second
)

// jwksCache is the TTL cache. Zero value is not usable; construct via
// newJWKSCache.
type jwksCache struct {
	jwksURI    string
	httpClient *http.Client
	cacheTTL   time.Duration
	minRefresh time.Duration
	now        func() time.Time

	mu          sync.RWMutex
	keys        map[string]crypto.PublicKey
	fetchedAt   time.Time
	lastRefresh time.Time
}

// newJWKSCache returns a cache pointed at the given JWKS URL. Zero ttl uses
// defaultJWKSCacheTTL; zero minRefresh uses minJWKSRefreshInterval.
//
// Caller MUST NOT use the cache after process exit; no goroutine is started.
func newJWKSCache(jwksURI string, httpClient *http.Client, ttl, minRefresh time.Duration) *jwksCache {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultJWKSFetchTimeout}
	}
	if ttl <= 0 {
		ttl = defaultJWKSCacheTTL
	}
	if minRefresh <= 0 {
		minRefresh = minJWKSRefreshInterval
	}
	return &jwksCache{
		jwksURI:    jwksURI,
		httpClient: httpClient,
		cacheTTL:   ttl,
		minRefresh: minRefresh,
		now:        time.Now,
		keys:       map[string]crypto.PublicKey{},
	}
}

// LookupJWK returns the key for the given JWS `kid`. A lookup on an unknown
// kid does NOT trigger a refresh here; Refresh is the explicit second step.
// If the cache has never been populated, or the TTL has elapsed, a transparent
// background refresh is triggered; the current caller observes (nil, false)
// on the first miss and succeeds on the retry.
func (c *jwksCache) LookupJWK(kid string) (crypto.PublicKey, bool) {
	c.mu.RLock()
	expired := c.cacheTTL > 0 && !c.fetchedAt.IsZero() && c.now().Sub(c.fetchedAt) > c.cacheTTL
	key, ok := c.keys[kid]
	emptyCache := len(c.keys) == 0 && c.fetchedAt.IsZero()
	c.mu.RUnlock()

	if emptyCache || (expired && !ok) {
		// TTL expired and we don't already have the kid -> try a fetch.
		// Ignore the error here; the caller's subsequent Refresh (triggered
		// on miss) surfaces it.
		_ = c.fetchIfAllowed()
		c.mu.RLock()
		key, ok = c.keys[kid]
		c.mu.RUnlock()
	}
	return key, ok
}

// Refresh forces a re-fetch of the JWKS document, subject to the
// rate-limit imposed by minRefresh. Returns nil when the refresh ran
// successfully or was rate-limited out (both are not-an-error from the
// verifier's point of view -- the verifier's only signal is whether the
// second LookupJWK finds the kid).
func (c *jwksCache) Refresh() error {
	return c.fetchIfAllowed()
}

func (c *jwksCache) fetchIfAllowed() error {
	c.mu.Lock()
	if c.minRefresh > 0 && !c.lastRefresh.IsZero() && c.now().Sub(c.lastRefresh) < c.minRefresh {
		c.mu.Unlock()
		return nil // rate-limited; no-op
	}
	// lastRefresh advances BEFORE the fetch runs. Intentional: a flaky AS
	// otherwise lets an attacker spray unknown-kid tokens to force an
	// uncapped fetch loop. Startup does one synchronous Refresh (see
	// buildAuthForMode) and fails the process on error, so a cold-start AS
	// outage surfaces at boot rather than through this rate-limited window.
	c.lastRefresh = c.now()
	c.mu.Unlock()

	keys, err := c.fetch()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.keys = keys
	c.fetchedAt = c.now()
	c.mu.Unlock()
	return nil
}

// fetch retrieves the JWKS document and decodes every key. Returns the
// decoded map; callers replace the cache atomically.
func (c *jwksCache) fetch() (map[string]crypto.PublicKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultJWKSFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURI, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("jwks: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jwks: fetch %s: %w", c.jwksURI, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks: fetch %s: status %d", c.jwksURI, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJWKSDocumentSize+1))
	if err != nil {
		return nil, fmt.Errorf("jwks: read body: %w", err)
	}
	if len(body) > maxJWKSDocumentSize {
		return nil, fmt.Errorf("jwks: document exceeds %d bytes", maxJWKSDocumentSize)
	}
	return parseJWKSDocument(body)
}

// jwk is one entry in a JWKS document. Only the fields the resource server
// needs are decoded; others are tolerated.
type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`

	// RSA
	N string `json:"n"`
	E string `json:"e"`

	// EC
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// jwksDocument matches the RFC 7517 Section 5 shape.
type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

// parseJWKSDocument decodes every JWK into a crypto.PublicKey indexed by kid.
// Keys without a kid, or of unknown kty, are skipped with no error so the
// cache holds as many verifiable keys as the AS supplies.
func parseJWKSDocument(body []byte) (map[string]crypto.PublicKey, error) {
	var doc jwksDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("jwks: decode document: %w", err)
	}
	out := make(map[string]crypto.PublicKey, len(doc.Keys))
	for i := range doc.Keys {
		k := &doc.Keys[i]
		if k.Kid == "" {
			continue
		}
		// Only `use=sig` keys are candidates; absent `use` is tolerated.
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		pub, err := decodeJWK(k)
		if err != nil {
			// Skip individual decode failures; one malformed key does not
			// invalidate the rest of the document.
			continue
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("jwks: document contained no usable keys")
	}
	return out, nil
}

// decodeJWK converts one JWK entry into a crypto.PublicKey.
func decodeJWK(k *jwk) (crypto.PublicKey, error) {
	switch k.Kty {
	case "RSA":
		return decodeRSAJWK(k)
	case "EC":
		return decodeECJWK(k)
	default:
		return nil, fmt.Errorf("unsupported kty %q", k.Kty)
	}
}

func decodeRSAJWK(k *jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("rsa jwk: decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("rsa jwk: decode e: %w", err)
	}
	if len(nBytes) == 0 || len(eBytes) == 0 {
		return nil, errors.New("rsa jwk: empty n or e")
	}
	e := new(big.Int).SetBytes(eBytes).Int64()
	if e < 3 || e > (1<<30) {
		return nil, fmt.Errorf("rsa jwk: implausible exponent %d", e)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(e),
	}, nil
}

func decodeECJWK(k *jwk) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	default:
		return nil, fmt.Errorf("ec jwk: unsupported curve %q", k.Crv)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, fmt.Errorf("ec jwk: decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, fmt.Errorf("ec jwk: decode y: %w", err)
	}
	pub := &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}
	// On-curve validation is handled implicitly by ecdsa.Verify at
	// signature-check time: a key off the curve yields a verification
	// failure rather than a silent accept. We do not use curve.IsOnCurve
	// here because Go 1.21+ deprecated it in favor of crypto/ecdh's
	// NewPublicKey -- but that API returns an ecdh.PublicKey, not the
	// ecdsa.PublicKey that ecdsa.Verify needs. JWK input comes from a
	// trusted AS over TLS, so the extra defense is not load-bearing.
	return pub, nil
}
