// Design: docs/architecture/mcp/overview.md -- MCP OAuth resource server

// RFC 8414 Authorization Server Metadata fetcher.
//
// The resource server uses the AS metadata document to discover `jwks_uri`
// (where the verification keys live) and `issuer` (the `iss` claim MUST
// match). Nothing else from the document is consumed; RFC 8414 defines
// many optional fields that are client concerns (authorization_endpoint,
// token_endpoint, scopes_supported).
//
// The well-known path is `/.well-known/oauth-authorization-server` per RFC
// 8414 Section 3, inserted between the host and the path of the issuer
// identifier (RFC 8414 Section 3.1), so `https://as/realm/x` is queried at
// `https://as/.well-known/oauth-authorization-server/realm/x`.
// OpenID Connect Discovery's appended path is not queried.
//
// Decoded via map[string]any to avoid struct tags with snake_case keys
// (ze's kebab-case rule exempts external specs but the linter hook does
// not; map decoding is the sanctioned pattern per the Phase 1 learned
// summary).

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/redact"
)

const (
	// RFC 8414 suffix appended to the authorization server's base URL.
	asMetadataWellKnownPath = "/.well-known/oauth-authorization-server"

	// AS metadata documents are JSON, normally <10 KB. 64 KB leaves
	// plenty of headroom for AS that ship verbose supported_* lists.
	maxASMetadataSize = 64 * 1024

	defaultASMetadataTimeout = 5 * time.Second
)

// asMetadata holds the subset of RFC 8414 fields the resource server uses.
type asMetadata struct {
	Issuer  string
	JWKSURI string
}

// fetchASMetadata retrieves the AS metadata from `<baseURL>/.well-known/...`
// and decodes it. Any HTTP failure, oversize body, or missing required field
// returns an error; the caller surfaces it to the operator at startup time
// so a misconfigured AS URL fails loudly rather than silently.
func fetchASMetadata(ctx context.Context, client *http.Client, baseURL string) (asMetadata, error) {
	client = oauthHTTPClient(client, defaultASMetadataTimeout)
	metadataURL, err := asMetadataURL(baseURL)
	if err != nil {
		return asMetadata{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, http.NoBody)
	if err != nil {
		return asMetadata{}, fmt.Errorf("as-metadata: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return asMetadata{}, fmt.Errorf("as-metadata: GET %s: %w", metadataURL, redact.URLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return asMetadata{}, fmt.Errorf("as-metadata: %s: status %d", metadataURL, resp.StatusCode)
	}
	// RFC 8414 Section 3.2: "A successful response MUST use the 200 OK HTTP
	// status code and return a JSON object using the \"application/json\"
	// content type".
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return asMetadata{}, fmt.Errorf("as-metadata: %s: expected application/json", metadataURL)
	}
	if mediaType != "application/json" {
		return asMetadata{}, fmt.Errorf("as-metadata: %s: expected application/json", metadataURL)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxASMetadataSize+1))
	if err != nil {
		return asMetadata{}, fmt.Errorf("as-metadata: read body: %w", err)
	}
	if len(body) > maxASMetadataSize {
		return asMetadata{}, fmt.Errorf("as-metadata: %s exceeds %d bytes", metadataURL, maxASMetadataSize)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return asMetadata{}, fmt.Errorf("as-metadata: decode: %w", err)
	}
	md := asMetadata{
		Issuer:  stringField(raw, "issuer"),
		JWKSURI: stringField(raw, "jwks_uri"),
	}
	if md.Issuer == "" {
		return asMetadata{}, fmt.Errorf("as-metadata: %s: missing issuer", metadataURL)
	}
	if md.JWKSURI == "" {
		return asMetadata{}, fmt.Errorf("as-metadata: %s: missing jwks_uri", metadataURL)
	}
	// RFC 8414 Section 3.3: "If these values are not identical, the data
	// contained in the response MUST NOT be used." Section 4 forbids Unicode
	// normalization; JSON decoding has already removed JSON escaping.
	if md.Issuer != baseURL {
		return asMetadata{}, fmt.Errorf("as-metadata: issuer does not match configured authorization-server")
	}
	// RFC 8414 Section 2: "This URL MUST use the \"https\" scheme."
	if _, err := oauthHTTPSURL(md.JWKSURI); err != nil {
		return asMetadata{}, fmt.Errorf("as-metadata: jwks_uri: %w", err)
	}
	return md, nil
}

// asMetadataURL builds the RFC 8414 metadata URL for an issuer identifier.
//
// RFC 8414 Section 3: the document lives "at a path formed by inserting a
// well-known URI string into the authorization server's issuer identifier
// between the host component and the path component, if any."
// RFC 8414 Section 3.1: "If the issuer identifier value contains a path
// component, any terminating "/" MUST be removed before inserting
// "/.well-known/" and the well-known URI suffix between the host component
// and the path component."
//
// So `https://as.example/` maps to
// `https://as.example/.well-known/oauth-authorization-server` and
// `https://as.example/realm/x/` maps to
// `https://as.example/.well-known/oauth-authorization-server/realm/x`.
// An issuer with no scheme or no host is refused rather than turned into a
// relative URL the HTTP client would reject later with a less useful error.
func asMetadataURL(issuer string) (string, error) {
	u, err := oauthHTTPSURL(issuer)
	if err != nil {
		return "", fmt.Errorf("as-metadata: issuer: %w", err)
	}
	// RFC 8414 Section 2 defines issuer as a URL with "no query or
	// fragment components".
	if u.RawQuery != "" || u.ForceQuery {
		return "", fmt.Errorf("as-metadata: issuer query is forbidden")
	}
	u.RawPath = asMetadataWellKnownPath + strings.TrimRight(u.EscapedPath(), "/")
	// Decode only after suffix insertion so an encoded trailing slash remains
	// part of its segment rather than being stripped as a path separator.
	u.Path, err = url.PathUnescape(u.RawPath)
	if err != nil {
		return "", fmt.Errorf("as-metadata: issuer path: %w", err)
	}
	return u.String(), nil
}

// oauthHTTPSURL validates URLs used to retrieve metadata and signing keys.
func oauthHTTPSURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("URL %q: %w", redact.URL(raw), redact.URLError(err))
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("URL %q must use HTTPS", redact.URL(raw))
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("URL %q: missing host", redact.URL(raw))
	}
	if u.User != nil {
		return nil, fmt.Errorf("URL userinfo is forbidden")
	}
	if strings.Contains(raw, "#") {
		return nil, fmt.Errorf("URL fragment is forbidden")
	}
	return u, nil
}

// oauthHTTPClient preserves the caller's transport and timeout while checking
// redirects before any request leaves TLS. The caller's client is never mutated.
func oauthHTTPClient(client *http.Client, timeout time.Duration) *http.Client {
	secured := http.Client{Timeout: timeout}
	if client != nil {
		secured = *client
	}
	checkRedirect := secured.CheckRedirect
	secured.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// RFC 8414 Section 6.1: "confidentiality protection MUST be applied
		// using TLS with a ciphersuite that provides confidentiality and
		// integrity protection." This applies to every redirect hop.
		if _, err := oauthHTTPSURL(req.URL.String()); err != nil {
			return err
		}
		if len(via) >= 10 {
			return fmt.Errorf("oauth: stopped after 10 redirects")
		}
		if checkRedirect != nil {
			return checkRedirect(req, via)
		}
		return nil
	}
	return &secured
}

// stringField returns the string value for key in m, or "" if the key is
// absent or not a string. Used to pull named fields out of the generic map
// decode that avoids snake_case struct tags.
func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
