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
// conversions that rsa.PublicKey wants. The EC coordinates are reassembled
// into a SEC 1 uncompressed point and parsed by crypto/ecdsa, which validates
// the point is on the curve.

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

	"github.com/ze-software/ze/internal/core/redact"
)

// The cache bounds both signing-key lifetime and requests to the authorization
// server. These limits apply to every OAuth listener.
const (
	defaultJWKSCacheTTL     = 15 * time.Minute
	minJWKSRefreshInterval  = 30 * time.Second
	maxJWKSDocumentSize     = 256 * 1024
	defaultJWKSFetchTimeout = 5 * time.Second
)

// jwksCache is the TTL cache. Safe for concurrent use. The zero value is not
// usable; construct via newJWKSCache.
type jwksCache struct {
	jwksURI    string
	httpClient *http.Client
	cacheTTL   time.Duration
	minRefresh time.Duration
	now        func() time.Time

	// refreshMu lets concurrent misses await the active fetch while fresh
	// known-key lookups continue under mu.
	refreshMu   sync.Mutex
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
	httpClient = oauthHTTPClient(httpClient, defaultJWKSFetchTimeout)
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

// LookupJWK returns the key for the given JWS kid. An expired cache is refreshed
// before any key can be used, including a kid already present in that cache.
// A failed or rate-limited refresh never authorizes a token with expired keys.
// A miss in a current cache leaves the rate-limited Refresh step to the verifier.
func (c *jwksCache) LookupJWK(kid string) (crypto.PublicKey, bool) {
	c.mu.RLock()
	expired := c.fetchedAt.IsZero()
	if !expired {
		expired = c.now().Sub(c.fetchedAt) >= c.cacheTTL
	}
	key, ok := c.keys[kid]
	c.mu.RUnlock()

	if expired {
		if err := c.fetchIfAllowed(); err != nil {
			return nil, false
		}
		c.mu.RLock()
		expired = c.fetchedAt.IsZero()
		if !expired {
			expired = c.now().Sub(c.fetchedAt) >= c.cacheTTL
		}
		key, ok = c.keys[kid]
		c.mu.RUnlock()
		if expired {
			return nil, false
		}
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
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	c.mu.Lock()
	if c.minRefresh > 0 && !c.lastRefresh.IsZero() && c.now().Sub(c.lastRefresh) < c.minRefresh {
		c.mu.Unlock()
		return nil // rate-limited; no-op
	}
	// lastRefresh advances BEFORE the fetch runs. Intentional: a flaky AS
	// otherwise lets an attacker spray unknown-kid tokens to force an
	// uncapped fetch loop. Startup does one synchronous Refresh (see
	// buildAuthForMode) and leaves the MCP listener disabled on error.
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
	// RFC 8414 Section 2: "This URL MUST use the \"https\" scheme."
	if _, err := oauthHTTPSURL(c.jwksURI); err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultJWKSFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURI, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("jwks: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jwks: fetch: %w", redact.URLError(err))
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
	// RFC 8414 Section 2: "When both signing and encryption keys are made
	// available, a \"use\" (public key use) parameter value is REQUIRED for
	// all keys in the referenced JWK Set to indicate each key's intended
	// usage." An encryption key makes an unlabelled key unsafe to use for
	// token verification, even if the latter's algorithm can sign.
	hasEncryption := false
	for i := range doc.Keys {
		if doc.Keys[i].Use == "enc" {
			hasEncryption = true
			break
		}
	}
	if hasEncryption {
		for i := range doc.Keys {
			if doc.Keys[i].Use == "" {
				return nil, errors.New("jwks: mixed-use key set contains a key without use")
			}
		}
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
	// SEC 1 uncompressed point: 0x04 || X || Y, each coordinate left-padded to
	// the curve's field size. RFC 7518 Section 6.2.1.2 asks for full-width
	// coordinates. A coordinate shorter than that is padded here rather than
	// refused, because a leading zero byte is ordinary and encoders that strip
	// it are common.
	size := (curve.Params().BitSize + 7) / 8
	if len(xBytes) > size || len(yBytes) > size {
		return nil, fmt.Errorf("ec jwk: coordinate longer than %d bytes for curve %q", size, k.Crv)
	}

	point := make([]byte, 1+2*size)
	point[0] = 4
	copy(point[1+size-len(xBytes):1+size], xBytes)
	copy(point[1+2*size-len(yBytes):], yBytes)

	// ParseUncompressedPublicKey refuses a point that is off the curve or is the
	// point at infinity, so a malformed key fails at decode. The raw-coordinate
	// form this replaced left that check to ecdsa.Verify, which reported an
	// invalid key as a signature mismatch.
	pub, err := ecdsa.ParseUncompressedPublicKey(curve, point)
	if err != nil {
		return nil, fmt.Errorf("ec jwk: invalid public key: %w", err)
	}

	return pub, nil
}
