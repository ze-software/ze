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
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Token          string
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
	return &Streamable{
		cfg:       cfg,
		registry:  newSessionRegistry(cfg.SessionTTL, cfg.MaxSessionLifetime, maxSessions),
		maxBody:   maxB,
		originSet: originSet,
	}, nil
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
// all normalise to the same key.
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
	if !s.originAllowed(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}

	if r.URL.Path == OAuthMetadataPath && r.Method == http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path != Endpoint {
		http.NotFound(w, r)
		return
	}

	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodPost:
		s.handlePOST(w, r)
	case http.MethodGet:
		s.handleGET(w, r)
	case http.MethodDelete:
		s.handleDELETE(w, r)
	default:
		w.Header().Set("Allow", "POST, GET, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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

// canonicalOrigin normalises a string to scheme://host[:port] (lowercase
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
	// `https://xn--mnchen-3ya.example.com` canonicalise to the same key.
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

// authorized runs the bearer check used when Token is set.
func (s *Streamable) authorized(r *http.Request) bool {
	if s.cfg.Token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	expected := "Bearer " + s.cfg.Token
	return subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) == 1
}

// handlePOST processes a client-initiated JSON-RPC message.
func (s *Streamable) handlePOST(w http.ResponseWriter, r *http.Request) {
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
		sess, err := s.doInitialize(&req)
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

	// Notifications (no id per JSON-RPC 2.0) are acknowledged with 202 per the
	// MCP 2025-06-18 Streamable HTTP spec; no body is returned. Session
	// lastSeenAt was already refreshed by registry.Get above.
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	resp := s.runMethod(sess, &req)
	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSONResponse(w, resp)
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

// doInitialize creates the session for an initialize request.
// Returns errUnsupportedProtocolVersion when the client's protocolVersion is
// neither empty nor in supportedProtocolVersions.
func (s *Streamable) doInitialize(req *request) (*session, error) {
	negotiated, err := parseInitializeProtocolVersion(req)
	if err != nil {
		return nil, err
	}
	sess, err := s.registry.Create(negotiated)
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
