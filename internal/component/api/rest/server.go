// Design: docs/architecture/api/architecture.md -- REST API transport
//
// Package rest provides an HTTP server that exposes the shared API engine
// as a RESTful JSON API. All logic lives in the engine; this package is a
// thin adapter handling HTTP routing, JSON marshaling, auth, and CORS.
package rest

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/api"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var logger = slogutil.Logger("api.rest")

// maxRequestBody limits the size of request bodies (1 MB).
const maxRequestBody = 1 << 20

// Authenticator validates an Authorization header value and returns the
// authenticated username. Returns ("", false) on invalid credentials.
// When nil, the server accepts all requests with no authentication.
type Authenticator func(authHeader string) (username string, ok bool)

// RESTConfig holds REST server configuration.
// ListenAddrs must contain at least one entry; every entry becomes a
// separate listener on the same *http.Server. Shutdown closes all of them.
type RESTConfig struct {
	ListenAddrs   []string      // e.g. []string{"0.0.0.0:8081", "127.0.0.1:18081"}
	Token         string        // Single bearer token (empty = no auth). Ignored when Authenticator is set.
	Authenticator Authenticator // Per-user auth callback. When set, Token is not checked.
	CORSOrigin    string        // Allowed CORS origin (empty = no CORS headers)
}

// OpenAPIProvider returns the OpenAPI spec bytes.
// Called lazily on the first /openapi.json request so it captures all commands.
type OpenAPIProvider func() []byte

// RESTServer is the REST API HTTP server.
// ListenAndServe binds every address in RESTConfig.ListenAddrs before any
// serve goroutine starts; if ANY bind fails the already-bound listeners are
// closed and ListenAndServe returns the error.
// Caller MUST call Shutdown when done.
type RESTServer struct {
	engine        *api.APIEngine
	sessions      *api.ConfigSessionManager
	openAPI       OpenAPIProvider
	token         string
	authenticator Authenticator
	corsOrigin    string

	srv *http.Server
	// configured holds the addresses passed in by the caller, in original order.
	configured []string
	// bound holds the actual listen addresses once ListenAndServe has bound
	// them. Populated under mu.
	bound []string
	mu    sync.RWMutex
	ready atomic.Bool
}

// NewRESTServer creates a REST API server.
// openAPI is called lazily to generate the OpenAPI spec.
// Requires at least one entry in cfg.ListenAddrs.
func NewRESTServer(cfg RESTConfig, engine *api.APIEngine, sessions *api.ConfigSessionManager, openAPI OpenAPIProvider) (*RESTServer, error) {
	if engine == nil {
		return nil, errors.New("engine is required")
	}
	if len(cfg.ListenAddrs) == 0 {
		return nil, errors.New("at least one listen address is required")
	}
	if slices.Contains(cfg.ListenAddrs, "") {
		return nil, errors.New("listen address must not be empty")
	}

	if cfg.Token == "" && cfg.Authenticator == nil {
		for _, addr := range cfg.ListenAddrs {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}
			ip := net.ParseIP(host)
			if ip != nil && !ip.IsLoopback() {
				return nil, fmt.Errorf("non-loopback listen address %q requires authentication (set token or users)", addr)
			}
			if ip == nil && host != "localhost" {
				return nil, fmt.Errorf("non-loopback listen address %q requires authentication (set token or users)", addr)
			}
		}
	}

	s := &RESTServer{
		engine:        engine,
		sessions:      sessions,
		openAPI:       openAPI,
		token:         cfg.Token,
		authenticator: cfg.Authenticator,
		corsOrigin:    cfg.CORSOrigin,
		configured:    append([]string(nil), cfg.ListenAddrs...),
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.srv = &http.Server{
		// Addr is informational; multi-listener serving uses Serve(ln).
		Addr:              cfg.ListenAddrs[0],
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return s, nil
}

// ListenAndServe binds every configured address and starts serving.
// Bind is all-or-nothing: any bind failure rolls back the already-bound
// listeners and returns the error without entering the serve loop.
func (s *RESTServer) ListenAndServe(ctx context.Context) error {
	var lc net.ListenConfig

	listeners := make([]net.Listener, 0, len(s.configured))
	bound := make([]string, 0, len(s.configured))
	for _, addr := range s.configured {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			for _, prev := range listeners {
				if closeErr := prev.Close(); closeErr != nil {
					logger.Warn("REST API: close partial listener", "error", closeErr)
				}
			}
			return fmt.Errorf("listen %s: %w", addr, err)
		}
		listeners = append(listeners, ln)
		bound = append(bound, ln.Addr().String())
	}

	s.mu.Lock()
	s.bound = bound
	s.mu.Unlock()
	s.ready.Store(true)

	for _, addr := range bound {
		logger.Info("REST API server listening", "addr", addr)
	}

	errCh := make(chan error, len(listeners))
	var wg sync.WaitGroup
	for _, ln := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			if serveErr := s.srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				errCh <- serveErr
			}
		}(ln)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// Shutdown gracefully shuts down the server, closing every listener.
func (s *RESTServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// Addresses returns every bound listen address in configured order.
// Before ListenAndServe binds, Addresses returns the configured addresses.
func (s *RESTServer) Addresses() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.bound) > 0 {
		out := make([]string, len(s.bound))
		copy(out, s.bound)
		return out
	}
	out := make([]string, len(s.configured))
	copy(out, s.configured)
	return out
}

// Address returns the first bound listen address. Retained for callers that
// only want the primary endpoint.
func (s *RESTServer) Address() string {
	addrs := s.Addresses()
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0]
}

// Ready returns true when the server is listening.
func (s *RESTServer) Ready() bool { return s.ready.Load() }

// registerRoutes wires all REST API routes.
func (s *RESTServer) registerRoutes(mux *http.ServeMux) {
	// Generic command execution.
	mux.HandleFunc("GET /api/v1/commands", s.withAuth(s.handleListCommands))
	mux.HandleFunc("GET /api/v1/commands/", s.withAuth(s.handleDescribeCommand))
	mux.HandleFunc("POST /api/v1/execute", s.withAuth(s.handleExecute))
	mux.HandleFunc("GET /api/v1/execute/stream", s.withAuth(s.handleStream))

	// Convenience routes (map to Execute).
	mux.HandleFunc("GET /api/v1/peers", s.withAuth(s.handleConvenience("summary")))
	mux.HandleFunc("GET /api/v1/peers/", s.withAuth(s.handlePeerByName))
	mux.HandleFunc("DELETE /api/v1/peers/", s.withAuth(s.handlePeerAction("teardown")))
	mux.HandleFunc("POST /api/v1/peers/", s.withAuth(s.handlePeerRefresh))
	mux.HandleFunc("GET /api/v1/rib/", s.withAuth(s.handleRIB))
	mux.HandleFunc("GET /api/v1/system/version", s.withAuth(s.handleConvenience("show version")))
	mux.HandleFunc("GET /api/v1/system/status", s.withAuth(s.handleConvenience("daemon status")))
	mux.HandleFunc("POST /api/v1/system/reload", s.withAuth(s.handleConvenience("daemon reload")))

	// Config session routes.
	mux.HandleFunc("GET /api/v1/config/running", s.withAuth(s.handleConfigRunning))
	mux.HandleFunc("POST /api/v1/config/sessions", s.withAuth(s.handleConfigEnter))
	mux.HandleFunc("PUT /api/v1/config/sessions/", s.withAuth(s.handleConfigSet))
	mux.HandleFunc("DELETE /api/v1/config/sessions/", s.withAuth(s.handleConfigDeleteOrDiscard))
	mux.HandleFunc("GET /api/v1/config/sessions/", s.withAuth(s.handleConfigDiff))
	mux.HandleFunc("POST /api/v1/config/sessions/", s.withAuth(s.handleConfigCommit))

	// Documentation (no auth required).
	mux.HandleFunc("GET /api/v1/openapi.json", s.handleOpenAPI)
	mux.HandleFunc("GET /api/v1/docs", s.handleDocs)
	mux.HandleFunc("GET /api/v1/docs/swagger-ui.css", s.handleSwaggerCSS)
	mux.HandleFunc("GET /api/v1/docs/swagger-ui-bundle.js", s.handleSwaggerJS)

	// CORS preflight (no auth required).
	mux.HandleFunc("OPTIONS /api/", s.handlePreflight)
}

// usernameKey is the request-context key for the authenticated username.
type usernameKeyType struct{}

var usernameKey = usernameKeyType{}

// withAuth wraps a handler with Bearer token authentication and CORS.
// On success, stores the authenticated username in the request context.
func (s *RESTServer) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
		}

		username := "api" // default for no-auth mode

		// Per-user authenticator takes precedence over single token.
		if s.authenticator != nil {
			auth := r.Header.Get("Authorization")
			user, ok := s.authenticator(auth)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			username = user
		} else if s.token != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + s.token
			if subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		}

		ctx := context.WithValue(r.Context(), usernameKey, username)
		next(w, r.WithContext(ctx))
	}
}

// callerIdentity extracts trusted caller metadata from the request.
func (s *RESTServer) callerIdentity(r *http.Request) api.CallerIdentity {
	if user, ok := r.Context().Value(usernameKey).(string); ok {
		return api.CallerIdentity{Username: user, RemoteAddr: r.RemoteAddr}
	}
	return api.CallerIdentity{Username: "api", RemoteAddr: r.RemoteAddr}
}

func (s *RESTServer) handleListCommands(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	writeJSON(w, http.StatusOK, s.engine.ListCommands(prefix))
}

func (s *RESTServer) handleDescribeCommand(w http.ResponseWriter, r *http.Request) {
	path := strings.ReplaceAll(strings.TrimPrefix(r.URL.Path, "/api/v1/commands/"), "/", " ")
	if path == "" {
		writeError(w, http.StatusBadRequest, "command path required")
		return
	}
	cmd, err := s.engine.DescribeCommand(path)
	if errors.Is(err, api.ErrNotFound) {
		writeError(w, http.StatusNotFound, "command not found: "+path)
		return
	}
	writeJSON(w, http.StatusOK, cmd)
}

func (s *RESTServer) handleExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string         `json:"command"`
		Params  map[string]any `json:"params"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	// Append typed params as "key value" pairs after the command.
	var cmd strings.Builder
	cmd.WriteString(req.Command)
	for key, val := range req.Params {
		if strings.ContainsAny(key, " \t\n\r") {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("parameter key %q must not contain whitespace", key))
			return
		}
		sval := fmt.Sprint(val)
		if sval == "" {
			continue
		}
		if strings.ContainsAny(sval, " \t\n\r") {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("parameter %q must not contain whitespace", key))
			return
		}
		cmd.WriteString(" ")
		cmd.WriteString(key)
		cmd.WriteString(" ")
		cmd.WriteString(sval)
	}
	command := cmd.String()

	result, execErr := s.engine.Execute(r.Context(), s.callerIdentity(r), command)
	if errors.Is(execErr, api.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, result.Error)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *RESTServer) handleConvenience(command string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, execErr := s.engine.Execute(r.Context(), s.callerIdentity(r), command)
		if errors.Is(execErr, api.ErrUnauthorized) {
			writeError(w, http.StatusForbidden, result.Error)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func (s *RESTServer) handlePeerByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/peers/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "peer name required")
		return
	}
	if err := validatePathSegment("peer", name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, execErr := s.engine.Execute(r.Context(), s.callerIdentity(r), "peer "+name+" detail")
	if errors.Is(execErr, api.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, result.Error)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *RESTServer) handlePeerAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/peers/")
		if name == "" {
			writeError(w, http.StatusBadRequest, "peer name required")
			return
		}
		if err := validatePathSegment("peer", name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, execErr := s.engine.Execute(r.Context(), s.callerIdentity(r), "peer "+name+" "+action)
		if errors.Is(execErr, api.ErrUnauthorized) {
			writeError(w, http.StatusForbidden, result.Error)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func (s *RESTServer) handlePeerRefresh(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/peers/")
	name, hasRefresh := strings.CutSuffix(path, "/refresh")
	if name == "" || !hasRefresh {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := validatePathSegment("peer", name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, execErr := s.engine.Execute(r.Context(), s.callerIdentity(r), "peer "+name+" refresh")
	if errors.Is(execErr, api.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, result.Error)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *RESTServer) handleRIB(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/rib/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "address family required")
		return
	}
	var command string
	if family, isBest := strings.CutSuffix(path, "/best"); isBest {
		if err := validatePathSegment("family", family); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		command = "rib best " + family
	} else {
		if err := validatePathSegment("family", path); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		command = "rib routes " + path
	}
	result, execErr := s.engine.Execute(r.Context(), s.callerIdentity(r), command)
	if errors.Is(execErr, api.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, result.Error)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *RESTServer) handleConfigRunning(w http.ResponseWriter, r *http.Request) {
	result, execErr := s.engine.Execute(r.Context(), s.callerIdentity(r), "show config dump")
	if errors.Is(execErr, api.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, result.Error)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *RESTServer) handleConfigEnter(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "config sessions not available")
		return
	}
	id, err := s.sessions.Enter(s.callerIdentity(r).Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"session-id": id})
}

func (s *RESTServer) handleConfigSet(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "config sessions not available")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/config/sessions/")
	if err := validateSessionID(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		Path  string `json:"path"`
		Value string `json:"value"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	username := s.callerIdentity(r).Username
	if err := s.sessions.Set(username, id, req.Path, req.Value); err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *RESTServer) handleConfigDeleteOrDiscard(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "config sessions not available")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/config/sessions/")
	parts := strings.SplitN(path, "/", 2) //nolint:mnd // id/path split
	id := parts[0]
	if err := validateSessionID(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	username := s.callerIdentity(r).Username
	if len(parts) == 1 {
		if err := s.sessions.Discard(username, id); err != nil {
			writeSessionError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "discarded"})
		return
	}
	configPath := strings.ReplaceAll(parts[1], "/", ".")
	if err := s.sessions.Delete(username, id, configPath); err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *RESTServer) handleConfigDiff(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "config sessions not available")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/config/sessions/")
	id, hasDiff := strings.CutSuffix(path, "/diff")
	if id == "" || !hasDiff {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := validateSessionID(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	diff, err := s.sessions.Diff(s.callerIdentity(r).Username, id)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"diff": diff})
}

func (s *RESTServer) handleConfigCommit(w http.ResponseWriter, r *http.Request) {
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "config sessions not available")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/config/sessions/")
	id, hasCommit := strings.CutSuffix(path, "/commit")
	if id == "" || !hasCommit {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err := validateSessionID(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.sessions.Commit(s.callerIdentity(r).Username, id); err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "committed"})
}

// writeSessionError writes an HTTP error for config session errors.
// ErrSessionForbidden becomes 403, other errors become 400.
func writeSessionError(w http.ResponseWriter, err error) {
	if errors.Is(err, api.ErrSessionForbidden) {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func (s *RESTServer) handleStream(w http.ResponseWriter, r *http.Request) {
	command := r.URL.Query().Get("command")
	if command == "" {
		writeError(w, http.StatusBadRequest, "command query parameter required")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	ch, cancel, err := s.engine.Stream(r.Context(), s.callerIdentity(r), command)
	if errors.Is(err, api.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, "unauthorized")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if s.corsOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
	}
	flusher.Flush()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			if _, fErr := fmt.Fprintf(w, "data: %s\n\n", event); fErr != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *RESTServer) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	if s.corsOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(s.openAPI()); err != nil {
		return
	}
}

func (s *RESTServer) handleDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := fmt.Fprint(w, docsHTML); err != nil {
		return
	}
}

func (s *RESTServer) handleSwaggerCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if _, err := w.Write(swaggerUICSS); err != nil {
		return
	}
}

func (s *RESTServer) handleSwaggerJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if _, err := w.Write(swaggerUIBundle); err != nil {
		return
	}
}

func (s *RESTServer) handlePreflight(w http.ResponseWriter, _ *http.Request) {
	if s.corsOrigin == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, writeErr := w.Write(data); writeErr != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := api.ExecResult{Status: api.StatusError, Error: msg}
	data, _ := json.Marshal(resp)
	if _, writeErr := w.Write(data); writeErr != nil {
		return
	}
}

// validateSessionID checks that a session ID looks like a hex string.
// Session IDs are 16-char hex strings from generateSessionID.
func validateSessionID(id string) error {
	if id == "" {
		return fmt.Errorf("session ID required")
	}
	if strings.ContainsAny(id, "/ \t\n\r") {
		return fmt.Errorf("invalid session ID: %q", id)
	}
	return nil
}

// validatePathSegment rejects values containing whitespace, newlines, or tabs.
// URL path segments that become dispatcher command tokens must not contain
// spaces -- the dispatcher tokenizes by whitespace, so embedded spaces would
// split a single value into multiple tokens and corrupt the command.
func validatePathSegment(field, value string) error {
	if strings.ContainsAny(value, " \t\n\r") {
		return fmt.Errorf("%s must not contain whitespace: %q", field, value)
	}
	return nil
}

func readJSON(r *http.Request, v any) error {
	body := http.MaxBytesReader(nil, r.Body, maxRequestBody)
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	return json.Unmarshal(data, v)
}

// docsHTML is a Swagger UI page that loads vendored assets and the OpenAPI spec.
// Assets served from /api/v1/docs/{swagger-ui.css,swagger-ui-bundle.js} to keep
// the daemon self-contained (no external CDN).
const docsHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Ze API</title>
  <link rel="stylesheet" type="text/css" href="/api/v1/docs/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="/api/v1/docs/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({url: "/api/v1/openapi.json", dom_id: "#swagger-ui"});
  </script>
</body>
</html>`
