// Design: docs/architecture/mcp/overview.md -- OAuth/bearer authentication and origin validation
// Related: streamable.go -- HTTP transport, streamable_tools.go -- tool dispatch

package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"golang.org/x/net/idna"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// buildAuthForMode dispatches across modes. AuthOAuth triggers the one-off
// AS metadata fetch + JWKS cache construction so the resource server is
// ready to verify tokens as soon as the listener binds. The fetch is
// synchronous at startup so a misconfigured AS URL fails the process rather
// than lurking until the first token arrives.
//
// RFC 8414 Section 3.3: the issuer value in the AS metadata document MUST
// equal the authorization server URL used to fetch it. Enforced here so a
// misbehaving (or compromised) AS cannot assert an issuer string that
// differs from the one the operator trusts.
// authBuildResult bundles what NewStreamable needs to assemble a Streamable.
// Grouping into a struct keeps buildAuthForMode's signature stable as more
// fields accumulate (metadata refresh cadence, JWKS cache handle, etc.).
type authBuildResult struct {
	// auth is the strategy that runs on every initialize request.
	auth authenticator
	// canonicalIssuer is the AS-reported issuer string (empty for non-OAuth
	// modes). The RFC 9728 metadata handler publishes this so clients see
	// the same byte-exact form the token verifier enforces.
	canonicalIssuer string
}

// buildAuthForMode returns the authentication strategy for the given mode.
// AuthOAuth performs a synchronous AS-metadata fetch at startup so a
// misconfigured AS URL fails the daemon rather than lurking until the first
// token arrives.
func buildAuthForMode(mode AuthMode, cfg StreamableConfig) (authBuildResult, error) {
	if mode != AuthOAuth {
		return authBuildResult{auth: buildAuthenticator(mode, cfg)}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultASMetadataTimeout)
	defer cancel()
	md, err := fetchASMetadata(ctx, nil, cfg.OAuth.AuthorizationServer)
	if err != nil {
		return authBuildResult{}, fmt.Errorf("mcp oauth: AS metadata: %w", err)
	}
	if md.Issuer == "" {
		return authBuildResult{}, errMcpOauthAsMetadataEmptyIssuer
	}
	// RFC 8414 §3.3: issuer MUST match the authorization server URL
	// (URL-canonical compare: scheme + host + optional port elision +
	// trailing-slash strip).
	if !sameAuthServer(md.Issuer, cfg.OAuth.AuthorizationServer) {
		return authBuildResult{}, fmt.Errorf(
			"mcp oauth: AS metadata issuer %q does not match configured authorization-server %q",
			md.Issuer, cfg.OAuth.AuthorizationServer,
		)
	}
	// JWKS URI mirror-scheme rule: when the AS is reached over HTTPS, the
	// JWKS URI MUST also be HTTPS so a passive attacker cannot manipulate
	// the keyset over cleartext. A malicious (or misconfigured) AS could
	// otherwise point jwks_uri at plaintext HTTP and undermine signature
	// verification entirely.
	if err := validateJWKSURI(cfg.OAuth.AuthorizationServer, md.JWKSURI); err != nil {
		return authBuildResult{}, fmt.Errorf("mcp oauth: %w", err)
	}
	cache := newJWKSCache(md.JWKSURI, nil, 0, 0)
	// Warm the cache up-front so the first verify does not double-round-trip.
	if err := cache.Refresh(); err != nil {
		return authBuildResult{}, fmt.Errorf("mcp oauth: prime JWKS: %w", err)
	}
	metadataURL := resourceMetadataURL(cfg.OAuth)
	a, err := buildOAuthAuthenticator(OAuthConfig{
		AuthorizationServer: md.Issuer,
		Audience:            cfg.OAuth.Audience,
		RequiredScopes:      cfg.OAuth.RequiredScopes,
		MetadataResource:    cfg.OAuth.MetadataResource,
	}, cache, metadataURL)
	if err != nil {
		return authBuildResult{}, fmt.Errorf("mcp oauth: %w", err)
	}
	return authBuildResult{auth: a, canonicalIssuer: md.Issuer}, nil
}

// Scheme constants for URL validation. Kept unexported because only the
// oauth paths use them.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// validateJWKSURI enforces the mirror-scheme rule: when the AS base URL is
// HTTPS, the JWKS URI MUST also be HTTPS. Otherwise a malicious or
// misconfigured AS could point jwks_uri at cleartext HTTP and a passive
// attacker on the JWKS fetch path could substitute a keyset of their
// choosing, letting any attacker-minted token verify.
//
// Non-HTTPS AS configurations (loopback dev only) still require jwks_uri
// to be an HTTP/HTTPS URL (file://, data://, etc. are rejected).
func validateJWKSURI(asURL, jwksURL string) error {
	if jwksURL == "" {
		return errors.New("AS metadata: jwks_uri missing")
	}
	as, err := url.Parse(asURL)
	if err != nil {
		return fmt.Errorf("AS URL parse: %w", err)
	}
	asScheme := strings.ToLower(as.Scheme)
	// Fail-closed when the AS URL is not http(s): a typo like `htps://` or
	// any other scheme would otherwise skip the mirror-scheme guard below
	// and admit cleartext jwks_uri under a malformed HTTPS config.
	if asScheme != schemeHTTP && asScheme != schemeHTTPS {
		return fmt.Errorf("AS URL %q: unsupported scheme %q", asURL, as.Scheme)
	}
	jwks, err := url.Parse(jwksURL)
	if err != nil {
		return fmt.Errorf("jwks_uri parse: %w", err)
	}
	jwksScheme := strings.ToLower(jwks.Scheme)
	if jwksScheme != schemeHTTP && jwksScheme != schemeHTTPS {
		return fmt.Errorf("jwks_uri %q: unsupported scheme %q", jwksURL, jwks.Scheme)
	}
	if jwks.Host == "" {
		return fmt.Errorf("jwks_uri %q: missing host", jwksURL)
	}
	if asScheme == schemeHTTPS && jwksScheme != schemeHTTPS {
		return fmt.Errorf("jwks_uri %q must use HTTPS (AS is HTTPS)", jwksURL)
	}
	return nil
}

// sameAuthServer reports whether two authorization-server URLs refer to the
// same endpoint after RFC-style canonicalization (scheme + host + port +
// trailing-slash strip). Mirrors canonicalOrigin for consistency.
func sameAuthServer(a, b string) bool {
	ka, err := canonicalAuthServerURL(a)
	if err != nil {
		return false
	}
	kb, err := canonicalAuthServerURL(b)
	if err != nil {
		return false
	}
	return ka == kb
}

// normalizeURL returns a scheme://host[:port][path] canonical form.
// Lowercases scheme + host, elides default http/https ports, collapses
// repeated slashes in the path and strips trailing slashes, re-brackets
// IPv6 literals per RFC 3986 §3.2.2, and drops the trailing dot from
// fully-qualified DNS names. Query / fragment / userinfo are IGNORED
// (stripped). This is the shared helper for equality-compare normalization
// of both authorization-server and audience URLs, which per RFC 8414 §3.3
// and RFC 8707 §2 share canonicalization rules.
//
// `https://as.example/`, `https://as.example:443`, `https://AS.EXAMPLE/`,
// `https://as.example///`, `https://as.example//a///b/` all canonicalize to
// the expected form. `https://[::1]:443/` canonicalizes to `https://[::1]`.
func normaliseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", errors.New("URL must include scheme and host")
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == schemeHTTP && port == "80") || (scheme == schemeHTTPS && port == "443") {
		port = ""
	}
	host = strings.TrimRight(host, ".")
	// IDN normalisation: a spec-compliant AS may emit the issuer as
	// punycode (`xn--...`) while the operator types the Unicode form, or
	// vice-versa. Fold both to punycode-ASCII so sameAuthServer /
	// canonicalAudience match regardless of the input flavor. Skip for
	// IPv6 literals (host still contains colons at this point).
	if !strings.Contains(host, ":") && host != "" {
		if ascii, idnaErr := idna.Lookup.ToASCII(host); idnaErr == nil {
			host = ascii
		}
	}
	// Collapse repeated slashes, then trim trailing. path.Clean folds `//`
	// into `/` and also resolves `.`/`..` segments, which is the desired
	// semantic for issuer/audience identifier comparison.
	p := path.Clean(u.Path)
	if p == "." || p == "/" {
		p = ""
	} else {
		p = strings.TrimRight(p, "/")
	}
	// IPv6 literals in URL authority MUST be bracketed per RFC 3986 §3.2.2.
	// url.Hostname() strips the brackets; add them back whenever the host
	// carries a colon (only possible for an IPv6 literal).
	var tb textbuf.Buffer
	if strings.Contains(host, ":") {
		host = tb.Byte('[').Str(host).Byte(']').String()
	}
	if port == "" {
		return tb.Reset().Str(scheme).Str("://").Str(host).Str(p).String(), nil
	}
	return tb.Reset().Str(scheme).Str("://").Str(host).Byte(':').Str(port).Str(p).String(), nil
}

// canonicalAuthServerURL is the strict variant used for authorization-server
// identifier comparison and metadata document construction: query, fragment,
// and userinfo are REJECTED because RFC 8414 issuer identifiers forbid them;
// silently stripping would collapse distinct operator configurations into
// the same canonical form.
func canonicalAuthServerURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("URL must not carry query or fragment")
	}
	if u.User != nil {
		return "", errors.New("URL must not carry userinfo")
	}
	return normaliseURL(raw)
}

// canonicalAudience returns the RFC 8707 canonical form for resource
// audience comparison. Lenient vs canonicalAuthServerURL: operator-supplied
// audiences may carry weird extras that the AS might preserve or strip; we
// compare on the normalized authority+path and ignore what the URL parser
// can throw away. Returns the empty string on parse failure so tokens
// whose `aud` is unparseable never accidentally match a canonicalized
// configured value.
func canonicalAudience(raw string) string {
	out, err := normaliseURL(raw)
	if err != nil {
		return ""
	}
	return out
}

// resourceMetadataURL returns the absolute URL of THIS server's RFC 9728
// protected-resource metadata document. Built from the operator-configured
// audience / metadata-resource so the URL matches what the client sees as
// the resource identity.
//
// Returns the empty string when neither is set or when the base is
// unparseable / carries query / fragment / userinfo -- stray shell quoting
// in config should not produce a malformed URL in 401 challenge headers.
// Validate() enforces `Audience` is present for auth-mode=oauth so this
// returns empty only on misconfigured standalone calls.
func resourceMetadataURL(cfg OAuthConfig) string {
	base := cfg.MetadataResource
	if base == "" {
		base = cfg.Audience
	}
	if base == "" {
		return ""
	}
	// canonicalAuthServerURL rejects query/fragment/userinfo; we reuse its
	// strict canonicalization so a malformed Audience never turns into a
	// malformed resource_metadata URL the client then tries to fetch.
	canonical, err := canonicalAuthServerURL(base)
	if err != nil {
		return ""
	}
	return canonical + OAuthMetadataPath
}

// buildOriginSet parses allowed origins into their canonical scheme://host:port
// form. Each entry MUST be a valid absolute URL or the literal "null"
// (browser `file://` origin). Trailing slashes and default-port omission are
// handled so `https://foo.com`, `https://foo.com:443`, and `https://foo.com/`
// all normalize to the same key.
func buildOriginSet(origins []string) (map[string]struct{}, error) {
	set := make(map[string]struct{}, len(origins))
	for _, raw := range origins {
		key, err := canonicalOrigin(raw)
		if err != nil {
			return nil, fmt.Errorf("origin %q: %w", raw, err)
		}
		set[key] = struct{}{}
	}
	return set, nil
}

// canonicalOrigin normalizes a string to scheme://host[:port] (lowercase
// scheme and host, explicit default ports elided, no trailing slash, no path).
// "null" (browser file:// origin) is preserved as-is.
func canonicalOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty origin")
	}
	if strings.EqualFold(raw, "null") {
		return "null", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", errors.New("origin must include scheme and host")
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	// Normalise IDN / punycode so `https://münchen.example.com` and
	// `https://xn--mnchen-3ya.example.com` canonicalize to the same key.
	// Only apply to non-bracketed (non-IPv6) hosts.
	if !strings.Contains(host, ":") && host != "" {
		if ascii, idnaErr := idna.Lookup.ToASCII(host); idnaErr == nil {
			host = ascii
		}
	}
	port := u.Port()
	if port != "" {
		if n, atoiErr := strconv.Atoi(port); atoiErr != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("invalid port %q", port)
		}
	}
	// Elide default ports so `https://foo.com` and `https://foo.com:443` match.
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	// IPv6 literals MUST be bracketed per RFC 3986 Section 3.2.2; u.Hostname()
	// strips the brackets so we put them back when the host contains a colon.
	var tb textbuf.Buffer
	if strings.Contains(host, ":") {
		host = tb.Byte('[').Str(host).Byte(']').String()
	}
	if port == "" {
		return tb.Reset().Str(scheme).Str("://").Str(host).String(), nil
	}
	return tb.Reset().Str(scheme).Str("://").Str(host).Byte(':').Str(port).String(), nil
}

// isLoopbackOrigin returns true for origin values that resolve to loopback.
// Canonicalises via canonicalOrigin so IPv6 literals, trailing slashes, and
// default ports produce the same match as the allowlist-set path.
func isLoopbackOrigin(origin string) bool {
	key, err := canonicalOrigin(origin)
	if err != nil {
		return false
	}
	switch key {
	case "null",
		"http://localhost", "https://localhost",
		"http://127.0.0.1", "https://127.0.0.1",
		"http://[::1]", "https://[::1]":
		return true
	}
	// Accept any port on loopback host+scheme combinations.
	for _, prefix := range []string{
		"http://localhost:", "https://localhost:",
		"http://127.0.0.1:", "https://127.0.0.1:",
		"http://[::1]:", "https://[::1]:",
	} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// writeAuthError renders an authError to the HTTP response, attaching the
// RFC 6750 / RFC 9728 WWW-Authenticate header when the error carries a
// Bearer challenge. Cache-Control: no-store per RFC 6750 §5.3 so intermediary
// caches do not serve stale 401 responses.
func writeAuthError(w http.ResponseWriter, e *authError) {
	if e == nil {
		return
	}
	if challenge := e.WWWAuthenticate(); challenge != "" {
		w.Header().Set("WWW-Authenticate", challenge)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	status := e.Status
	if status == 0 {
		status = http.StatusUnauthorized
	}
	http.Error(w, e.Error(), status)
}
