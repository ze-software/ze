// Design: docs/architecture/mcp/overview.md -- Streamable HTTP transport for MCP
// Related: tools.go -- MCP tool dispatch types and primitives
// Related: meta.go -- per-request _meta parsing
// Related: headers.go -- standard request header validation
// Related: discover.go -- server/discover result assembly
// Related: streamable_auth.go -- OAuth/bearer authentication and origin validation
// Related: streamable_tools.go -- tool dispatch and task management

// Streamable HTTP (MCP 2026-07-28 basic/transports) dispatcher.
//
// One HTTP endpoint answering POST only. There is no handshake, no session and
// no server-to-client stream: every request carries its own protocol version,
// client identity and client capabilities in `params._meta`, mirrors selected
// body fields into HTTP headers the server cross-checks, and authenticates on
// its own. GET and DELETE, which earlier revisions used for the SSE stream and
// session termination, answer 405.

package mcp

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/audit"
)

var errMcpOauthAsMetadataEmptyIssuer = errors.New("mcp oauth: AS metadata: empty issuer")

// ProtocolVersion is the MCP protocol version this server speaks.
const ProtocolVersion = "2026-07-28"

// Endpoint is the single MCP endpoint path.
const Endpoint = "/mcp"

// OAuthMetadataPath is the RFC 9728 Protected Resource Metadata discovery URL.
const OAuthMetadataPath = "/.well-known/oauth-protected-resource"

// supportedProtocolVersions enumerates every MCP version this server accepts,
// in the order server/discover and an UnsupportedProtocolVersionError advertise
// them.
//
// Exactly one entry: the cutover to 2026-07-28 dropped the handshake-based
// revisions outright, and no shim, alias or default survives for them
// (ai/rules/compatibility.md). A slice rather than a set so the advertised
// order is deterministic; membership goes through
// isSupportedProtocolVersion.
var supportedProtocolVersions = []string{ProtocolVersion}

// isSupportedProtocolVersion reports whether this server implements v.
func isSupportedProtocolVersion(v string) bool {
	return slices.Contains(supportedProtocolVersions, v)
}

// StreamableConfig bundles what NewStreamable needs. Zero-value fields get defaults.
type StreamableConfig struct {
	Dispatch CommandDispatcher
	Commands CommandLister
	// Provider, when set, replaces the command-registry tool surface: tools/list
	// returns Provider.Tools(), tools/call delegates to Provider.CallTool, and
	// server/discover reports Provider.ServerName(). Nil (the ze daemon) serves
	// the command-registry surface instead. Provider mode changes only which
	// tools are offered: it takes the same header validation, the same
	// per-request metadata and the same per-request authentication as every
	// other caller.
	Provider       ToolProvider
	Token          string // AuthMode=Bearer: single shared secret
	AllowedOrigins []string
	// MaxBodyBytes caps one request body. Zero uses maxRequestBody (1 MB).
	// This is the only per-request size bound the transport enforces.
	MaxBodyBytes int64

	// AuthMode selects the authentication strategy. AuthUnspecified is
	// treated as AuthNone so existing callers (Phase 1) that leave this
	// field zero get permissive behavior plus the legacy Token field.
	AuthMode AuthMode
	// BearerList is the per-identity token table (AuthMode=BearerList).
	BearerList []BearerListEntry
	// OAuth holds resource-server settings (AuthMode=OAuth). Phase F wires it.
	OAuth OAuthConfig
	// Tasks holds per-server task registry limits.
	Tasks TaskRegistryConfig
	// AuditRecorder records failed authentication attempts when set.
	AuditRecorder audit.Recorder
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
// The server holds no per-client state: identity and capabilities are rebuilt
// from each request and passed by value into dispatch, so there is nothing
// keyed by client, connection or session to bound or expire. The task registry
// is the one long-lived structure, and it is keyed by authenticated principal
// with its own concurrency, retention and TTL caps.
//
// Lifecycle: create with NewStreamable; mount on any net/http listener; MUST
// call Close before process exit so the task-GC goroutine stops.
type Streamable struct {
	cfg       StreamableConfig
	tasks     *taskRegistry
	maxBody   int64
	originSet map[string]struct{}
	auth      authenticator
	authMode  AuthMode
	// oauthIssuer is the AS-reported issuer, populated after a successful
	// buildAuthForMode run. Used by the RFC 9728 metadata handler so the
	// advertised authorization_servers[0] matches the value the token
	// verifier enforces. Empty for non-OAuth modes.
	oauthIssuer string

	cachedResources []map[string]any // immutable after construction; from embedded FS walk
}

// NewStreamable returns a configured Streamable HTTP MCP server. Returns an
// error if any entry in cfg.AllowedOrigins fails to parse — silently falling
// back to "loopback only" would contradict the operator's intent.
//
// Caller MUST call Close before process exit.
func NewStreamable(cfg StreamableConfig) (*Streamable, error) {
	maxB := cfg.MaxBodyBytes
	if maxB == 0 {
		maxB = maxRequestBody
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
		cfg:             cfg,
		tasks:           newTaskRegistry(cfg.Tasks),
		maxBody:         maxB,
		originSet:       originSet,
		auth:            authRes.auth,
		authMode:        mode,
		oauthIssuer:     authRes.canonicalIssuer,
		cachedResources: listResources(),
	}, nil
}

// Close releases server resources. Idempotent.
func (s *Streamable) Close() {
	s.tasks.Close()
}

// ServeHTTP implements http.Handler.
func (s *Streamable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// RFC 9728 protected-resource metadata is served BEFORE the Origin
	// allowlist: the document is public by design, carries no client
	// state, and browser-based OAuth clients discover it cross-origin
	// from whatever domain hosts the SPA. CORS wildcard + OPTIONS
	// preflight admit those clients without weakening the Origin check
	// that protects the JSON-RPC endpoint.
	if r.URL.Path == OAuthMetadataPath {
		s.handleResourceMetadata(w, r)
		return
	}

	// MCP 2026-07-28 basic/transports/streamable-http Section "Security &
	// Endpoint": "Servers MUST validate the Origin header on all incoming
	// connections to prevent DNS rebinding attacks. If the Origin header is
	// present and invalid, servers MUST respond with HTTP 403 Forbidden."
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

	switch r.Method {
	case http.MethodPost:
		s.handlePOST(w, r)
	case http.MethodOptions:
		s.handleEndpointPreflight(w, r)
	default:
		// MCP 2026-07-28 basic/transports/streamable-http Section "Earlier
		// Streamable HTTP Revisions": "A server that supports only this
		// revision and receives such traffic from an older client SHOULD
		// respond as follows: HTTP GET or DELETE to the MCP endpoint: respond
		// with 405 Method Not Allowed."
		//
		// The endpoint is POST-only in this revision ("The server MUST provide
		// a single HTTP endpoint path ... that supports POST"): the GET stream
		// and the DELETE session-termination call of the 2025-03-26 through
		// 2025-11-25 shape do not exist here. Same rationale as the 404 branch
		// for the CORS headers: a browser client must be able to read the Allow
		// header and the error description.
		setMainPathCORS(w, r)
		w.Header().Set("Allow", "POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Response headers MCP clients need to read cross-origin. The fetch API only
// exposes CORS-safelisted response headers unless listed here; without
// WWW-Authenticate a browser-based client cannot discover the OAuth metadata
// URL on a 401. No session-id header exists in this revision, so none is
// exposed, and Retry-After is not listed either: its only emitter was the 429
// session-limit response, which went with the session registry. Advertising a
// header nothing sends tells a client to look for something that never arrives.
const corsExposeHeaders = "WWW-Authenticate"

// Headers the server accepts on non-safelisted cross-origin requests: the
// standard MCP request headers plus authentication and content negotiation.
// Mcp-Param-* is absent because CORS allow-lists cannot express a prefix and
// this server annotates no tool parameter with `x-mcp-header`, so no client is
// ever told to send one.
const corsAllowHeaders = "Authorization, Content-Type, MCP-Protocol-Version, Mcp-Method, Mcp-Name, Accept"

// setMainPathCORS emits the CORS response headers for the /mcp endpoint's
// real-request responses. Preflight uses a separate header set (see
// handleEndpointPreflight). Called at the top of every main-path handler: the
// Origin check in ServeHTTP already admitted the request, so echoing the Origin
// back is safe. No-op when Origin is absent (non-browser client; CORS does not
// apply).
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
	// another. Use Add so it composes with any Vary set by the handler.
	h.Add("Vary", "Origin")
}

// handleEndpointPreflight responds to a CORS preflight for the main /mcp
// endpoint. The Origin check has already admitted the request; echo the
// Origin back (wildcard is not compatible with credentialed requests, and
// MCP clients send an Authorization header on POST).
//
// Preflight responses include Vary: Origin so caches keyed by origin do
// not serve the wrong Access-Control-Allow-Origin to a different caller.
func (s *Streamable) handleEndpointPreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Preflight requires an Origin header; a non-browser client sending
		// OPTIONS without Origin is almost certainly misconfigured.
		w.Header().Set("Allow", "POST, OPTIONS")
		http.Error(w, "preflight requires Origin header", http.StatusBadRequest)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
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

// authenticate dispatches to the configured authenticator and returns the
// authenticated Identity. Returns a non-nil *authError that the caller renders
// into a 401 response.
//
// Runs on EVERY request. With the handshake gone there is no session id to
// stand in for a credential, so each POST presents its own and each POST is
// checked: a revoked token stops working on the next request rather than at
// session expiry, and there is no long-lived identifier to steal.
//
// A zero Identity means "authenticated as anonymous under auth-mode none", not
// "unauthenticated": an unauthenticated request is the *authError early return
// above, never a value a handler can receive.
func (s *Streamable) authenticate(r *http.Request) (Identity, *authError) {
	if s.auth == nil {
		return Identity{}, nil
	}
	return s.auth.Authenticate(r)
}

// handlePOST processes a client-initiated JSON-RPC message.
//
// The order below is the contract, not a convenience: header validation is a
// transport-level guard that MUST run before dispatch, or the header/body
// confusion it exists to prevent is already possible by the time it runs.
//
//  1. read and size-cap the body
//  2. parse the JSON-RPC envelope        -> -32700
//  3. validate the standard headers      -> -32020, HTTP 400
//  4. parse the per-request `_meta`      -> -32602, HTTP 400
//  5. check the declared version         -> -32022, HTTP 400
//  6. authenticate                       -> 401
//  7. acknowledge a notification         -> 202
//  8. dispatch
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
		writeJSONResponse(w, s.fail(nil, rpcParseError, "parse error"))
		return
	}

	// Decoded once and shared: header validation reads params.name / params.uri
	// and the declared `_meta` version, and parseRequestMeta reads the rest of
	// the same object.
	params := decodeParamsObject(req.Params)

	// MCP 2026-07-28 basic/transports/streamable-http Section "Server
	// Validation": "Servers MUST reject requests with a 400 Bad Request HTTP
	// status and JSON-RPC error code -32020 (HeaderMismatch) if any validation
	// fails."
	if headerErr := validateStandardHeaders(r, &req, params); headerErr != nil {
		writeJSONResponseStatus(w, http.StatusBadRequest,
			s.fail(req.ID, rpcHeaderMismatch, headerErr.Error()))
		return
	}

	// MCP 2026-07-28 basic/index Section "_meta": "A request missing any
	// required field is malformed; the server MUST reject it with JSON-RPC
	// error code -32602 (Invalid params). On HTTP, the response status MUST be
	// 400 Bad Request."
	meta, metaErr := parseRequestMeta(params)
	if metaErr != nil {
		writeJSONResponseStatus(w, http.StatusBadRequest,
			s.fail(req.ID, rpcInvalidParams, metaErr.Error()))
		return
	}

	// MCP 2026-07-28 basic/versioning Section "Protocol Version Negotiation":
	// "If the server does not implement the requested version (whether the
	// version is unknown to the server, or is a known version the server has
	// chosen not to support), it MUST respond with an
	// UnsupportedProtocolVersionError listing the versions it does support."
	// The binding pins the status: "For HTTP, the response status code MUST be
	// 400 Bad Request."
	if !isSupportedProtocolVersion(meta.ProtocolVersion) {
		writeJSONResponseStatus(w, http.StatusBadRequest,
			s.failUnsupportedVersion(req.ID, meta.ProtocolVersion))
		return
	}

	// MCP 2026-07-28 basic/patterns/mrtr Section "Client Requirements": "2. ...
	// If the InputRequiredResult does not contain a requestState field, the
	// client MUST NOT include one in the retry." This server issues none, so an
	// arriving value is a client protocol violation and, per server requirement
	// 4, unverifiable attacker-controlled input that MUST be rejected. Refused
	// here, before dispatch, so no handler can ever see one.
	if stateErr := rejectUnsolicitedRequestState(params); stateErr != nil {
		mrtrLogger.Warn("rejected a request carrying an unsolicited requestState",
			slog.String("method", req.Method), slog.String("remote", r.RemoteAddr))
		writeJSONResponseStatus(w, http.StatusBadRequest,
			s.fail(req.ID, rpcInvalidParams, stateErr.Error()))
		return
	}

	identity, aerr := s.authenticate(r)
	if aerr != nil {
		recordMCPAuthFailure(s.cfg.AuditRecorder, r.Header.Get("Authorization"), r.RemoteAddr)
		writeAuthError(w, aerr)
		return
	}

	// MCP 2026-07-28 basic/transports/streamable-http Section "Sending
	// Messages": a JSON-RPC notification (no id) the server accepts "MUST
	// return HTTP status code 202 Accepted with no body". This revision defines
	// no client-to-server notification on this transport, so nothing is
	// dispatched; the acknowledgement is the whole handling.
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	scope := requestScope{
		Identity:        identity,
		Capabilities:    meta.Capabilities,
		ProtocolVersion: meta.ProtocolVersion,
		ClientInfo:      meta.ClientInfo,
	}
	resp := s.runMethod(r.Context(), scope, &req, r.RemoteAddr)
	writeJSONResponseStatus(w, httpStatusForDispatch(resp), resp)
}

// httpStatusForDispatch maps a dispatched method's response to the HTTP status
// the MCP 2026-07-28 binding pins to it.
//
// Only two dispatch-time error codes carry a mandated status; every other
// result, success or failure, rides a 200 as JSON-RPC intends. Derived here
// rather than at each return site so a handler cannot forget the status that
// belongs with the code it chose.
func httpStatusForDispatch(resp *response) int {
	if resp == nil || resp.Error == nil {
		return http.StatusOK
	}
	switch resp.Error.Code {
	case rpcMethodNotFound:
		// MCP 2026-07-28 basic/transports/streamable-http Section "Protocol
		// Version Header": "If the server does not implement the requested RPC
		// method, it MUST respond with 404 Not Found and a JSON-RPC error with
		// code -32601 (Method not found). The JSON-RPC error body distinguishes
		// this case from a 404 returned by a legacy HTTP+SSE server that does
		// not host the modern MCP endpoint."
		return http.StatusNotFound
	case rpcMissingRequiredClientCapability:
		// MCP 2026-07-28 basic/index Section "Error Codes", schema
		// MissingRequiredClientCapabilityError: "For HTTP, the response status
		// code MUST be 400 Bad Request."
		return http.StatusBadRequest
	}
	return http.StatusOK
}
