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
	"strings"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/api"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

var logger = slogutil.Logger("api.rest")

// maxRequestBody limits the size of request bodies (1 MB).
const maxRequestBody = 1 << 20

// RESTConfig holds REST server configuration.
type RESTConfig struct {
	ListenAddr string // e.g. "0.0.0.0:8081"
	Token      string // Bearer token for auth (empty = no auth)
	CORSOrigin string // Allowed CORS origin (empty = no CORS headers)
}

// OpenAPIProvider returns the OpenAPI spec bytes.
// Called lazily on the first /openapi.json request so it captures all commands.
type OpenAPIProvider func() []byte

// RESTServer is the REST API HTTP server.
// Caller MUST call Shutdown when done.
type RESTServer struct {
	engine     *api.APIEngine
	sessions   *api.ConfigSessionManager
	openAPI    OpenAPIProvider
	token      string
	corsOrigin string

	srv     *http.Server
	ready   atomic.Bool
	address string
}

// NewRESTServer creates a REST API server.
// openAPI is called lazily to generate the OpenAPI spec.
func NewRESTServer(cfg RESTConfig, engine *api.APIEngine, sessions *api.ConfigSessionManager, openAPI OpenAPIProvider) (*RESTServer, error) {
	if engine == nil {
		return nil, errors.New("engine is required")
	}

	s := &RESTServer{
		engine:     engine,
		sessions:   sessions,
		openAPI:    openAPI,
		token:      cfg.Token,
		corsOrigin: cfg.CORSOrigin,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.srv = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return s, nil
}

// ListenAndServe starts the server. Blocks until the server stops.
func (s *RESTServer) ListenAndServe(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.srv.Addr, err)
	}
	s.address = ln.Addr().String()
	s.ready.Store(true)
	logger.Info("REST API server listening", "addr", s.address)

	if srvErr := s.srv.Serve(ln); srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
		return srvErr
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *RESTServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// Address returns the listen address (available after ListenAndServe starts).
func (s *RESTServer) Address() string { return s.address }

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
	mux.HandleFunc("GET /api/v1/peers", s.withAuth(s.handleConvenience("bgp summary")))
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

	// CORS preflight (no auth required).
	mux.HandleFunc("OPTIONS /api/", s.handlePreflight)
}

// withAuth wraps a handler with Bearer token authentication and CORS.
func (s *RESTServer) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.corsOrigin)
		}
		if s.token != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + s.token
			if subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		}
		next(w, r)
	}
}

// authContext extracts auth info from the request.
func (s *RESTServer) authContext(_ *http.Request) api.AuthContext {
	return api.AuthContext{Username: "api"}
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

	result, execErr := s.engine.Execute(s.authContext(r), command)
	if errors.Is(execErr, api.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, result.Error)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *RESTServer) handleConvenience(command string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, execErr := s.engine.Execute(s.authContext(r), command)
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
	result, execErr := s.engine.Execute(s.authContext(r), "peer "+name+" detail")
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
		result, execErr := s.engine.Execute(s.authContext(r), "peer "+name+" "+action)
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
	result, execErr := s.engine.Execute(s.authContext(r), "peer "+name+" refresh")
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
	result, execErr := s.engine.Execute(s.authContext(r), command)
	if errors.Is(execErr, api.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, result.Error)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *RESTServer) handleConfigRunning(w http.ResponseWriter, r *http.Request) {
	result, execErr := s.engine.Execute(s.authContext(r), "show config dump")
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
	id, err := s.sessions.Enter("api")
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
	if err := s.sessions.Set(id, req.Path, req.Value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	if len(parts) == 1 {
		if err := s.sessions.Discard(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "discarded"})
		return
	}
	configPath := strings.ReplaceAll(parts[1], "/", ".")
	if err := s.sessions.Delete(id, configPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	diff, err := s.sessions.Diff(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	if err := s.sessions.Commit(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "committed"})
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
	ch, cancel, err := s.engine.Stream(r.Context(), s.authContext(r), command)
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

// docsHTML is a minimal Swagger UI page that loads the OpenAPI spec.
const docsHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Ze API</title>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({url: "/api/v1/openapi.json", dom_id: "#swagger-ui"});
  </script>
</body>
</html>`
