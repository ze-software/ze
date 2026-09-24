// Design: docs/architecture/mcp/overview.md -- OAuth/bearer authentication and origin validation
// Related: streamable.go -- HTTP transport, streamable_tools.go -- tool dispatch

package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// authBuildResult carries the request authenticator and the validated issuer
// used in the protected-resource metadata response.
type authBuildResult struct {
	// auth runs on every MCP POST.
	auth authenticator
	// issuer is identical to the configured issuer after successful discovery.
	issuer string
}

// buildAuthForMode returns the authentication strategy for the given mode.
// AuthOAuth discovers metadata and signing keys before the MCP listener starts.
func buildAuthForMode(mode AuthMode, cfg StreamableConfig) (authBuildResult, error) {
	switch mode {
	case AuthNone, AuthBearer, AuthBearerList:
		return authBuildResult{auth: buildAuthenticator(mode, cfg)}, nil
	case AuthOAuth:
		// Discovery below constructs the OAuth authenticator before serving.
	default:
		return authBuildResult{}, errors.New("mcp: unsupported authentication mode")
	}
	metadataURL := resourceMetadataURL(cfg.OAuth)
	if metadataURL == "" {
		return authBuildResult{}, errors.New("mcp oauth: resource identifier must be an HTTPS URL without userinfo or fragment")
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultASMetadataTimeout)
	defer cancel()
	md, err := fetchASMetadata(ctx, nil, cfg.OAuth.AuthorizationServer)
	if err != nil {
		return authBuildResult{}, fmt.Errorf("mcp oauth: AS metadata: %w", err)
	}
	cache := newJWKSCache(md.JWKSURI, nil, 0, 0)
	// Warm the cache up-front so the first verify does not double-round-trip.
	if err := cache.Refresh(); err != nil {
		return authBuildResult{}, fmt.Errorf("mcp oauth: prime JWKS: %w", err)
	}
	a, err := buildOAuthAuthenticator(OAuthConfig{
		AuthorizationServer: md.Issuer,
		Audience:            cfg.OAuth.Audience,
		RequiredScopes:      cfg.OAuth.RequiredScopes,
		MetadataResource:    cfg.OAuth.MetadataResource,
	}, cache, metadataURL)
	if err != nil {
		return authBuildResult{}, fmt.Errorf("mcp oauth: %w", err)
	}
	return authBuildResult{auth: a, issuer: md.Issuer}, nil
}

// resourceMetadataURL returns the absolute URL of THIS server's RFC 9728
// protected-resource metadata document. Built from the operator-configured
// audience / metadata-resource so the URL matches what the client sees as
// the resource identity.
//
// RFC 9728 Section 3: "Protected resources supporting metadata MUST make a
// JSON document containing metadata as specified in Section 2 available at
// a URL formed by inserting a well-known URI string into the protected
// resource's resource identifier between the host component and the path
// and/or query components, if any." So `https://mcp.example/mcp` yields
// `https://mcp.example/.well-known/oauth-protected-resource/mcp`, never
// `https://mcp.example/mcp/.well-known/oauth-protected-resource`.
//
// RFC 9728 Section 3.1: "If the resource identifier value contains a path
// or query component, any terminating slash (/) following the host
// component MUST be removed before inserting /.well-known/ and the
// well-known URI path suffix". Only the terminating slash is removed; the
// identifier's remaining path and query retain their exact spelling.
//
// Returns the empty string when neither is set or when the base is
// unparseable or carries a fragment or userinfo. These cannot identify an
// RFC 9728 resource.
// Validate() enforces `Audience` is present for auth-mode=oauth so this
// returns empty only on misconfigured standalone calls.
func resourceMetadataURL(cfg OAuthConfig) string {
	origin, resourcePath, ok := resourceOriginAndPath(cfg)
	if !ok {
		return ""
	}
	return origin + OAuthMetadataPath + resourcePath
}

// resourceMetadataPath returns the escaped request path and query at which this
// server publishes metadata. It is the request-target part of resourceMetadataURL.
// Non-OAuth modes have no identifier and answer 404 at the bare suffix.
func resourceMetadataPath(cfg OAuthConfig) string {
	_, resourcePath, ok := resourceOriginAndPath(cfg)
	if !ok {
		return OAuthMetadataPath
	}
	return OAuthMetadataPath + resourcePath
}

// resourceOriginAndPath splits the resource identifier into its origin and
// escaped path plus query. No URL or Unicode normalization is permitted:
// separate spellings can identify separate protected resources.
func resourceOriginAndPath(cfg OAuthConfig) (origin, resourcePath string, ok bool) {
	base := cfg.MetadataResource
	if base == "" {
		base = cfg.Audience
	}
	u, err := oauthHTTPSURL(base)
	if err != nil {
		return "", "", false
	}
	resourcePath = strings.TrimRight(u.EscapedPath(), "/")
	if u.RawQuery != "" || u.ForceQuery {
		resourcePath += "?" + u.RawQuery
	}
	// Preserve the authority's spelling; URL.String would lowercase a scheme.
	authorityEnd := strings.Index(base, "://") + len("://")
	end := strings.IndexAny(base[authorityEnd:], "/?")
	if end < 0 {
		return base, resourcePath, true
	}
	return base[:authorityEnd+end], resourcePath, true
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
