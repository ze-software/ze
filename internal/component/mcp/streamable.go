// Design: docs/architecture/mcp/overview.md -- Streamable HTTP transport for MCP
// Related: handler.go -- MCP tool dispatch types and primitives
// Related: session.go -- session registry and SSE outbound queue

// Streamable HTTP (MCP 2025-06-18 basic/transports) dispatcher.
//
// One HTTP endpoint answering POST (client -> server JSON-RPC) and GET (open
// server -> client SSE stream). Origin header validated, Mcp-Session-Id header
// assigned at initialize, required on subsequent calls.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

// ProtocolVersion is the negotiated MCP protocol version this server speaks.
const ProtocolVersion = "2025-06-18"

// LegacyProtocolVersion is assumed when a client omits the MCP-Protocol-Version
// header after initialize (per the 2025-06-18 transports spec).
const LegacyProtocolVersion = "2025-03-26"

// Endpoint is the single MCP endpoint path.
const Endpoint = "/mcp"

// OAuthMetadataPath is the RFC 9728 Protected Resource Metadata discovery URL.
const OAuthMetadataPath = "/.well-known/oauth-protected-resource"

// supportedProtocolVersions enumerates the MCP versions this server will
// negotiate with at initialize time.
var supportedProtocolVersions = map[string]struct{}{
	ProtocolVersion:       {},
	LegacyProtocolVersion: {},
	"2024-11-05":          {},
}

// errUnsupportedProtocolVersion is returned when a client sends an
// initialize with a protocolVersion this server does not understand.
var errUnsupportedProtocolVersion = errors.New("mcp: unsupported protocol version")

// StreamableConfig bundles what NewStreamable needs. Zero-value fields get defaults.
type StreamableConfig struct {
	Dispatch       CommandDispatcher
	Commands       CommandLister
	Token          string // AuthMode=Bearer: single shared secret
	AllowedOrigins []string
	SessionTTL     time.Duration
	MaxBodyBytes   int64
	// MaxSessions caps concurrent sessions. Zero uses defaultMaxSessions (1024);
	// negative disables the cap (not recommended in production).
	MaxSessions int
	// MaxSessionLifetime caps the absolute age of a session regardless of
	// activity — defends against clients that hold sessions open via GET SSE
	// streams indefinitely (their heartbeats otherwise keep lastSeenAt fresh).
	// Zero disables the lifetime cap. Recommend a value > SessionTTL.
	MaxSessionLifetime time.Duration

	// AuthMode selects the authentication strategy. AuthUnspecified is
	// treated as AuthNone so existing callers (Phase 1) that leave this
	// field zero get permissive behavior plus the legacy Token field.
	AuthMode AuthMode
	// BearerList is the per-identity token table (AuthMode=BearerList).
	BearerList []BearerListEntry
	// OAuth holds resource-server settings (AuthMode=OAuth). Phase F wires it.
	OAuth OAuthConfig
}

// BearerListEntry is one row of the AuthMode=BearerList identity table.
// Token is sensitive; NewStreamable copies it into the dispatcher and the
// caller is free to zero the slice afterwards.
type BearerListEntry struct {
	Name   string
	Token  string
	Scopes []string
}

// OAuthConfig is the Phase F resource-server configuration. Phase C carries
// the type so StreamableConfig is stable; Phase F populates the fields.
type OAuthConfig struct {
	AuthorizationServer string
	Audience            string
	RequiredScopes      []string
	// MetadataResource is the absolute URL (with scheme + host + path) the
	// RFC 9728 `/.well-known/oauth-protected-resource` handler returns as
	// the `resource` field. Set to `cfg.OAuth.Audience` when blank.
	MetadataResource string
}

// Streamable is the Streamable HTTP MCP server. Implements http.Handler.
//
// Lifecycle: create with NewStreamable; mount on any net/http listener; MUST
// call Close before process exit so the session-GC goroutine stops.
type Streamable struct {
	cfg            StreamableConfig
	registry       *sessionRegistry
	maxBody        int64
	originSet      map[string]struct{}
	heartbeatEvery time.Duration // override for tests; 0 → sessionHeartbeatWindow
	auth           authenticator
	authMode       AuthMode
	// oauthIssuer is the AS-reported issuer, populated after a successful
	// buildAuthForMode run. Used by the RFC 9728 metadata handler so the
	// advertised authorization_servers[0] matches the value the token
	// verifier enforces. Empty for non-OAuth modes.
	oauthIssuer string
}

// NewStreamable returns a configured Streamable HTTP MCP server. Returns an
// error if any entry in cfg.AllowedOrigins fails to parse — silently falling
// back to "loopback only" would contradict the operator's intent — or if
// MaxSessionLifetime is set but shorter than SessionTTL (which would make the
// idle TTL dead code).
//
// Caller MUST call Close before process exit.
func NewStreamable(cfg StreamableConfig) (*Streamable, error) {
	if cfg.MaxSessionLifetime > 0 && cfg.SessionTTL > 0 && cfg.MaxSessionLifetime < cfg.SessionTTL {
		return nil, fmt.Errorf("MaxSessionLifetime (%v) must be >= SessionTTL (%v) or zero",
			cfg.MaxSessionLifetime, cfg.SessionTTL)
	}
	maxB := cfg.MaxBodyBytes
	if maxB == 0 {
		maxB = maxRequestBody
	}
	// Session cap semantics (unified across config and registry):
	//   0  -> use defaultMaxSessions (1024)
	//   <0 -> unlimited (disabled cap; not recommended)
	maxSessions := cfg.MaxSessions
	if maxSessions == 0 {
		maxSessions = defaultMaxSessions
	}
	originSet, err := buildOriginSet(cfg.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	// Auth-mode inference for legacy callers: Token set with AuthMode zero
	// means "single shared bearer" (the Phase-1 behavior).
	mode := cfg.AuthMode
	if mode == AuthUnspecified {
		if cfg.Token != "" {
			mode = AuthBearer
		} else {
			mode = AuthNone
		}
	}
	authRes, err := buildAuthForMode(mode, cfg)
	if err != nil {
		return nil, err
	}
	return &Streamable{
		cfg:         cfg,
		registry:    newSessionRegistry(cfg.SessionTTL, cfg.MaxSessionLifetime, maxSessions),
		maxBody:     maxB,
		originSet:   originSet,
		auth:        authRes.auth,
		authMode:    mode,
		oauthIssuer: authRes.canonicalIssuer,
	}, nil
}

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
		return authBuildResult{}, fmt.Errorf("mcp oauth: AS metadata: empty issuer")
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
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port == "" {
		return scheme + "://" + host + p, nil
	}
	return scheme + "://" + host + ":" + port + p, nil
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

// heartbeatInterval returns the SSE heartbeat cadence, honoring a test
// override when set. Overrides below minHeartbeatInterval are clamped to that
// floor so a stray tiny value (e.g. 1 ns) cannot saturate the scheduler.
func (s *Streamable) heartbeatInterval() time.Duration {
	if s.heartbeatEvery > 0 {
		if s.heartbeatEvery < minHeartbeatInterval {
			return minHeartbeatInterval
		}
		return s.heartbeatEvery
	}
	return sessionHeartbeatWindow
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

// Close releases server resources. Idempotent.
func (s *Streamable) Close() {
	s.registry.Close()
}

// ServeHTTP implements http.Handler.
func (s *Streamable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// RFC 9728 protected-resource metadata is served BEFORE the Origin
	// allowlist: the document is public by design, carries no session
	// state, and browser-based OAuth clients discover it cross-origin
	// from whatever domain hosts the SPA. CORS wildcard + OPTIONS
	// preflight admit those clients without weakening the Origin check
	// that protects the JSON-RPC endpoint.
	if r.URL.Path == OAuthMetadataPath {
		s.handleResourceMetadata(w, r)
		return
	}

	if !s.originAllowed(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}

	if r.URL.Path != Endpoint {
		// 404 on a wrong sub-path also needs CORS headers; the origin
		// already passed the allowlist above, and a browser client that
		// probes an unexpected path otherwise sees "CORS error" instead
		// of the descriptive 404.
		setMainPathCORS(w, r)
		http.NotFound(w, r)
		return
	}

	// Auth runs on initialize only; subsequent requests are gated by a
	// valid Mcp-Session-Id, which was issued only after a successful
	// auth-dispatcher run at create time. The 128-bit session ID is the
	// per-request token. See spec-mcp-2-remote-oauth "Entry Point (target)"
	// and AC-11a.
	switch r.Method {
	case http.MethodPost:
		s.handlePOST(w, r)
	case http.MethodGet:
		s.handleGET(w, r)
	case http.MethodDelete:
		s.handleDELETE(w, r)
	case http.MethodOptions:
		s.handleEndpointPreflight(w, r)
	default:
		// Same rationale as the 404 branch: CORS headers on the 405 so a
		// browser client can read the Allow header and error description.
		setMainPathCORS(w, r)
		w.Header().Set("Allow", "POST, GET, DELETE, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Response headers MCP clients need to read cross-origin. The fetch API
// only exposes CORS-safelisted response headers unless listed here; without
// Mcp-Session-Id a browser-based client cannot extract the session token
// from the initialize response, and without WWW-Authenticate it cannot
// discover the OAuth metadata URL on a 401.
const corsExposeHeaders = "Mcp-Session-Id, WWW-Authenticate, Retry-After"

// Headers the server accepts on non-safelisted cross-origin requests.
const corsAllowHeaders = "Authorization, Content-Type, Mcp-Session-Id, MCP-Protocol-Version, Accept"

// setMainPathCORS emits the CORS response headers for the /mcp endpoint's
// real-request responses (POST / GET / DELETE). Preflight uses a separate
// header set (see handleEndpointPreflight). Called at the top of every
// main-path handler: the Origin check in ServeHTTP already admitted the
// request, so echoing the Origin back is safe. No-op when Origin is absent
// (non-browser client; CORS does not apply).
//
// Must run before the response body is written because `http.Error` and
// `ResponseWriter.Write` flush headers on first byte.
func setMainPathCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Expose-Headers", corsExposeHeaders)
	// Vary: Origin so shared caches do not serve one origin's response to
	// another. Use Add so it composes with any Vary set by the handler
	// (SSE handlers currently do not set Vary, but defensive for future
	// changes like Accept-Encoding).
	h.Add("Vary", "Origin")
}

// handleEndpointPreflight responds to a CORS preflight for the main /mcp
// endpoint. The Origin check has already admitted the request; echo the
// Origin back (wildcard is not compatible with credentialed requests, and
// MCP clients send Authorization + session-id headers on POST/DELETE).
// Methods, headers, and max-age enumerate what the real request may carry.
//
// Preflight responses include Vary: Origin so caches keyed by origin do
// not serve the wrong Access-Control-Allow-Origin to a different caller.
func (s *Streamable) handleEndpointPreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Preflight requires an Origin header; a non-browser client sending
		// OPTIONS without Origin is almost certainly misconfigured.
		w.Header().Set("Allow", "POST, GET, DELETE, OPTIONS")
		http.Error(w, "preflight requires Origin header", http.StatusBadRequest)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "600")
	w.Header().Set("Vary", "Origin")
	w.WriteHeader(http.StatusNoContent)
}

// handleResourceMetadata serves the RFC 9728 protected-resource metadata
// document. Public by design: no auth, CORS wildcard, preflight-friendly.
// Returns 404 when AuthMode != AuthOAuth so the URL only exists when
// meaningful. Allowed methods are GET + OPTIONS (preflight); other methods
// return 405.
func (s *Streamable) handleResourceMetadata(w http.ResponseWriter, r *http.Request) {
	// CORS wildcard + preflight. MCP clients running in a browser may be
	// loaded from a different origin than the MCP server; they need to
	// fetch the metadata document to discover the authorization server.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "600")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.authMode != AuthOAuth {
		http.NotFound(w, r)
		return
	}
	// Publish the AS-reported issuer so the string the client sees in
	// `authorization_servers[0]` matches the value the token verifier
	// enforces. buildAuthForMode ran sameAuthServer() so this is
	// byte-identical to what tokens carry in their `iss` claim.
	advertised := s.cfg.OAuth
	advertised.AuthorizationServer = s.oauthIssuer
	writeResourceMetadata(w, advertised)
}

// originAllowed reports whether the Origin header is permitted.
//
// Empty Origin header is treated as non-browser traffic and accepted. When
// AllowedOrigins is empty, only loopback-shaped origins pass. Otherwise the
// request's Origin is parsed and compared against the canonical allowlist.
func (s *Streamable) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if len(s.originSet) == 0 {
		return isLoopbackOrigin(origin)
	}
	key, err := canonicalOrigin(origin)
	if err != nil {
		return false
	}
	_, ok := s.originSet[key]
	return ok
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
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port == "" {
		return scheme + "://" + host, nil
	}
	return scheme + "://" + host + ":" + port, nil
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

// authenticate dispatches to the configured authenticator and attaches the
// resulting Identity to the request context when successful. Returns a
// non-nil *authError that the caller renders into a 401 response.
//
// Phase 2 policy: identity is bound at initialize. Subsequent requests with
// a valid Mcp-Session-Id are trusted by session-id validity alone (MCP
// 2025-06-18 auth is per-session, not per-request). The initialize handler
// calls this function; other handlers skip the bearer check because the
// session carries the identity.
func (s *Streamable) authenticate(r *http.Request) (Identity, *authError) {
	if s.auth == nil {
		return Identity{}, nil
	}
	return s.auth.Authenticate(r)
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

// handlePOST processes a client-initiated JSON-RPC message.
func (s *Streamable) handlePOST(w http.ResponseWriter, r *http.Request) {
	setMainPathCORS(w, r)
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONResponse(w, &response{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
		return
	}

	if req.Method == "initialize" {
		identity, aerr := s.authenticate(r)
		if aerr != nil {
			writeAuthError(w, aerr)
			return
		}
		sess, err := s.doInitialize(&req, identity)
		if err != nil {
			switch {
			case errors.Is(err, errSessionLimitReached):
				w.Header().Set("Retry-After", "30")
				http.Error(w, "session limit reached", http.StatusTooManyRequests)
			case errors.Is(err, errUnsupportedProtocolVersion):
				writeJSONResponse(w, s.fail(req.ID, -32602, err.Error()))
			default:
				writeJSONResponse(w, s.fail(req.ID, -32603, err.Error()))
			}
			return
		}
		w.Header().Set("Mcp-Session-Id", sess.ID())
		writeJSONResponse(w, s.buildInitializeResult(&req))
		return
	}

	if !s.validateProtocolVersionHeader(r) {
		http.Error(w, "unsupported MCP-Protocol-Version", http.StatusBadRequest)
		return
	}
	headerID := r.Header.Get("Mcp-Session-Id")
	if headerID == "" {
		writeJSONResponse(w, s.fail(req.ID, -32600, "Mcp-Session-Id header required"))
		return
	}
	sess, ok := s.registry.Get(headerID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// JSON-RPC response body (client's reply to a server-initiated request
	// such as elicitation/create). Has id and result|error, no method.
	// Routed to the correlation map; returns 202 Accepted with empty body.
	// Detected BEFORE the notification branch because a response also has
	// id != nil. See spec-mcp-3-elicitation AC-13 / AC-15b.
	if req.Method == "" && req.ID != nil {
		s.handleElicitResponse(w, sess, &req, body)
		return
	}

	// Notifications (no id per JSON-RPC 2.0) are acknowledged with 202 per the
	// MCP 2025-06-18 Streamable HTTP spec; no body is returned. Session
	// lastSeenAt was already refreshed by registry.Get above.
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Bind a per-POST reply sink BEFORE running the method. The sink
	// starts as jsonReplySink; a tool handler that elicits triggers an
	// in-place upgrade to sseReplySink (session.UpgradeCurrentSinkToSSE)
	// so the elicitation/create frame AND the terminal tool result ride
	// the same HTTP response as SSE events. Non-eliciting handlers leave
	// the sink untouched; handlePOST then writes the JSON response via
	// writeJSONResponse as before.
	jsonSink := newJSONReplySink(w)
	release, sinkErr := sess.SetActivePostSink(jsonSink)
	if sinkErr != nil {
		// Concurrent POST already owns a sink. MCP expects one client per
		// session, so this is either a misbehaving client or a race in a
		// multi-client testing setup. Fail fast rather than risk
		// interleaved frames on the same HTTP response.
		writeJSONResponse(w, s.fail(req.ID, -32603, "another request is in flight on this session"))
		return
	}
	defer release()

	resp := s.runMethod(sess, &req)
	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// If the handler upgraded the sink during dispatch (i.e. elicited),
	// the terminal response MUST ride the SSE stream too — otherwise the
	// client sees the elicit frame but no response. Route via the sink.
	if sess.CurrentPostSink().IsSSE() {
		data, err := json.Marshal(resp)
		if err != nil {
			// Headers already committed; best-effort.
			return
		}
		_ = sess.CurrentPostSink().WriteFrame(data)
		return
	}
	writeJSONResponse(w, resp)
}

// handleElicitResponse routes a client's JSON-RPC response body (the reply
// to a server-initiated elicitation/create) to the pending-correlation
// channel. Returns 202 Accepted regardless of whether the id matched --
// AC-15b silently drops unknown-id replies so the server does not leak
// which elicit ids are live. Malformed action values propagate via
// ErrElicitMalformed when the suspended Elicit resolves.
//
// Both JSON-RPC response shapes are handled:
//
//   - Success: {"id":...,"result":{"action":...,"content":...}}. Normal path.
//   - Error:   {"id":...,"error":{...}}. MCP does not document elicit-error
//     semantics; we deliver it as an explicit cancel (Action="cancel",
//     Content=nil) so the suspended handler unblocks via ErrElicitCanceled
//     rather than ErrElicitMalformed. An RPC-level failure on the reply leg
//     is the client saying "never mind", not "I broke your protocol."
//
// Content-type guards: when action=="accept" and result carries a content
// key that is not a JSON object, we reject with 400 -- an accept without a
// parseable content map is a protocol violation the client should see.
func (s *Streamable) handleElicitResponse(w http.ResponseWriter, sess *session, req *request, body []byte) {
	// Extract the JSON-RPC id as a string; we only generate string ids
	// (base64url of 128 random bits) so a non-string id cannot match
	// anything in the correlations map.
	var idStr string
	if req.ID != nil {
		_ = json.Unmarshal(*req.ID, &idStr)
	}
	if idStr == "" {
		http.Error(w, "id must be a non-empty string", http.StatusBadRequest)
		return
	}
	// Re-parse the body as a generic map to pull result / error without
	// struct tags (check-json-kebab.sh bans camelCase struct tags).
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		http.Error(w, "malformed JSON", http.StatusBadRequest)
		return
	}
	var action string
	var content map[string]any
	if result, ok := parsed["result"].(map[string]any); ok {
		action, _ = result["action"].(string)
		if contentRaw, hasContent := result["content"]; hasContent {
			if contentMap, isMap := contentRaw.(map[string]any); isMap {
				content = contentMap
			} else {
				// Accept with non-object content is a protocol violation.
				http.Error(w, "result.content must be a JSON object", http.StatusBadRequest)
				return
			}
		}
	} else if _, hasErr := parsed["error"]; hasErr {
		// Client returned an error response -- treat as explicit cancel.
		// This lets the suspended Elicit return ErrElicitCanceled (a
		// typed sentinel handlers already branch on) instead of
		// ErrElicitMalformed for what is really "client said no."
		action = elicitActionCancel
	}
	// Unknown action values (including "") are delivered verbatim; Elicit
	// translates them to ErrElicitMalformed.
	sess.ResolveElicit(idStr, elicitResponse{Action: action, Content: content})
	w.WriteHeader(http.StatusAccepted)
}

// handleGET opens a server-to-client SSE stream bound to an existing session.
//
// A periodic heartbeat (SSE comment line, sessionHeartbeatWindow cadence)
// keeps the TCP connection open through network intermediaries with short
// idle timeouts, and refreshes the session's last-seen timestamp so the TTL
// sweep does not reap a session whose only activity is the stream itself.
//
// Only one concurrent stream is allowed per session. A second GET with the
// same Mcp-Session-Id is rejected with 409 Conflict — Go channel semantics
// would otherwise route each server-sent frame to a non-deterministic
// receiver, breaking task-status routing and resumability.
func (s *Streamable) handleGET(w http.ResponseWriter, r *http.Request) {
	setMainPathCORS(w, r)
	if !acceptsEventStream(r) {
		http.Error(w, "GET requires Accept: text/event-stream", http.StatusNotAcceptable)
		return
	}
	id := r.Header.Get("Mcp-Session-Id")
	sess, ok := s.registry.Get(id)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if !sess.streamActive.CompareAndSwap(false, true) {
		// Strip the "mcp:" internal-package prefix from the public-facing
		// body; keep the sentinel for log-level correlation.
		http.Error(w, "session already has an active stream", http.StatusConflict)
		return
	}
	defer sess.streamActive.Store(false)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Defeat buffering by nginx / common CDN fronts; without this the
	// heartbeat keepalive is batched until a full buffer page fills and
	// clients see the stream as hung.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	heartbeat := time.NewTicker(s.heartbeatInterval())
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
			sess.Touch(s.registry.now())
		case frame, open := <-sess.Outbound():
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", frame); err != nil {
				return
			}
			flusher.Flush()
			sess.Touch(s.registry.now())
		}
	}
}

// handleDELETE terminates a session.
func (s *Streamable) handleDELETE(w http.ResponseWriter, r *http.Request) {
	setMainPathCORS(w, r)
	id := r.Header.Get("Mcp-Session-Id")
	if id == "" {
		http.Error(w, "Mcp-Session-Id header required", http.StatusBadRequest)
		return
	}
	if !s.registry.Delete(id) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// doInitialize creates the session for an initialize request, binding the
// authenticated identity to the session. The identity flows through the
// registry immutably so later handlers (tasks, elicitation) can scope by it
// without re-auth.
func (s *Streamable) doInitialize(req *request, identity Identity) (*session, error) {
	negotiated, err := parseInitializeProtocolVersion(req)
	if err != nil {
		return nil, err
	}
	sess, err := s.registry.Create(negotiated, identity)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// buildInitializeResult assembles the InitializeResult body.
// MCP uses camelCase JSON keys per the external spec; Ze's kebab-case rule
// exempts MCP. Keys are built via map literals to preserve the spec shape.
func (s *Streamable) buildInitializeResult(req *request) *response {
	return &response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "ze-mcp",
				"version": "2.0.0",
			},
		},
	}
}

// validateProtocolVersionHeader applies the 2025-06-18 rule: missing header ->
// accept (spec assumes 2025-03-26). Present with unknown value -> 400.
func (s *Streamable) validateProtocolVersionHeader(r *http.Request) bool {
	v := r.Header.Get("MCP-Protocol-Version")
	if v == "" {
		return true
	}
	return v == ProtocolVersion || v == LegacyProtocolVersion || v == "2024-11-05"
}

// runMethod runs a JSON-RPC method handler to completion synchronously.
func (s *Streamable) runMethod(sess *session, req *request) *response {
	_ = sess
	switch req.Method {
	case "notifications/initialized":
		return nil
	case "tools/list":
		return s.ok(req.ID, map[string]any{"tools": s.allTools()})
	case "tools/call":
		return s.callTool(req)
	default:
		return s.fail(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

// allTools returns the combined handcrafted + auto-generated tool list.
func (s *Streamable) allTools() []map[string]any {
	if s.cfg.Commands == nil {
		result := make([]map[string]any, len(handcraftedTools))
		copy(result, handcraftedTools)
		return result
	}
	groups := groupCommands(s.cfg.Commands())
	generated := generateTools(groups, handcraftedNames())
	result := make([]map[string]any, len(handcraftedTools), len(handcraftedTools)+len(generated))
	copy(result, handcraftedTools)
	result = append(result, generated...)
	return result
}

// callTool executes a tools/call request.
func (s *Streamable) callTool(req *request) *response {
	var params callParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.fail(req.ID, -32602, "invalid params: "+err.Error())
	}
	runner := &server{dispatch: s.cfg.Dispatch, commands: s.cfg.Commands}
	if handler, ok := toolHandlers[params.Name]; ok {
		return s.ok(req.ID, handler(runner, params.Arguments))
	}
	if s.cfg.Commands != nil {
		if prefix, validActions, ok := s.findGeneratedTool(params.Name); ok {
			return s.ok(req.ID, runner.dispatchGenerated(prefix, validActions, params.Arguments))
		}
	}
	return s.fail(req.ID, -32602, fmt.Sprintf("unknown tool: %s", params.Name))
}

// findGeneratedTool maps an auto-generated tool name back to its command prefix.
func (s *Streamable) findGeneratedTool(name string) (string, map[string]bool, bool) {
	skip := handcraftedNames()
	groups := groupCommands(s.cfg.Commands())
	for _, g := range groups {
		if skip[toolName(g.prefix)] {
			continue
		}
		if toolName(g.prefix) == name {
			valid := make(map[string]bool, len(g.actions))
			for _, a := range g.actions {
				valid[a.name] = true
			}
			return g.prefix, valid, true
		}
	}
	return "", nil, false
}

func (s *Streamable) ok(id *json.RawMessage, result any) *response {
	return &response{JSONRPC: "2.0", ID: id, Result: result}
}

func (s *Streamable) fail(id *json.RawMessage, code int, msg string) *response {
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// writeJSONResponse writes a single JSON-RPC response with Content-Type JSON.
func writeJSONResponse(w http.ResponseWriter, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, writeErr := w.Write(data); writeErr != nil {
		return
	}
}

// acceptsEventStream reports whether Accept permits text/event-stream.
func acceptsEventStream(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	for part := range strings.SplitSeq(accept, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mediaType == "text/event-stream" || mediaType == "*/*" {
			return true
		}
	}
	return false
}

// parseInitializeProtocolVersion extracts protocolVersion from an initialize
// request.
//
//   - Missing / empty / unparseable params -> server's preferred ProtocolVersion.
//   - Known value -> echo it back (the client asked, the server honors).
//   - Unknown value -> errUnsupportedProtocolVersion.
//
// MCP uses camelCase externally; parse via generic map to avoid struct tags.
func parseInitializeProtocolVersion(req *request) (string, error) {
	if len(req.Params) == 0 {
		return ProtocolVersion, nil
	}
	var p map[string]any
	if err := json.Unmarshal(req.Params, &p); err != nil {
		// Malformed params is not a version mismatch; let the client initialize
		// at the server's preferred version instead of 400-ing them out.
		return ProtocolVersion, nil //nolint:nilerr // intentional permissive fallback
	}
	raw, present := p["protocolVersion"]
	if !present {
		return ProtocolVersion, nil
	}
	v, ok := raw.(string)
	if !ok || v == "" {
		return ProtocolVersion, nil
	}
	if _, known := supportedProtocolVersions[v]; !known {
		return "", fmt.Errorf("%w: %q", errUnsupportedProtocolVersion, v)
	}
	return v, nil
}
