// Design: docs/architecture/web-interface.md -- Looking glass HTTP server
// Detail: handler_api.go -- Birdwatcher REST API handlers
// Detail: handler_ui.go -- HTMX web UI handlers
// Detail: handler_graph.go -- AS path topology graph handler
// Detail: render.go -- Template rendering
// Detail: assets.go -- Embedded CSS and JS

// Package lg provides the looking glass HTTP server for Ze.
//
// The looking glass exposes BGP session state and route information via
// both an HTMX web UI and a birdwatcher-compatible REST API. It runs as
// a separate HTTP server from the web UI, on its own port, with no
// authentication (public, read-only).
//
// TLS is optional (looking glasses are often behind reverse proxies).
// When TLS is enabled, the server uses the same self-signed certificate
// infrastructure as the web UI.
//
// All BGP data is accessed via CommandDispatcher, preserving plugin
// isolation. The LG never imports RIB or peer plugin packages directly.
//
// Caller MUST call Shutdown when the server is no longer needed.
package lg

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// lgLogger is the structured logger for the looking glass subsystem.
var lgLogger = slogutil.Logger("lg.server")

// maxSSEClients limits concurrent SSE connections to prevent resource exhaustion.
const maxSSEClients = 100

// CommandDispatcher sends a command string to the engine and returns the
// JSON response. This is the same interface used by the web UI for admin
// commands.
type CommandDispatcher func(cmd string) (string, error)

// LGConfig holds the configuration for creating an LG server.
type LGConfig struct {
	// ListenAddr is the address to bind (e.g., "0.0.0.0:8444").
	ListenAddr string

	// TLS enables HTTPS. When false, the server uses plain HTTP.
	TLS bool

	// CertPEM is optional PEM-encoded certificate data (required when TLS is true).
	CertPEM []byte

	// KeyPEM is optional PEM-encoded private key data (required when TLS is true).
	KeyPEM []byte

	// Dispatch is the command dispatcher for querying the BGP engine.
	// MUST NOT be nil.
	Dispatch CommandDispatcher

	// Logger is the structured logger for the LG server.
	// If nil, the package-level lg logger is used.
	Logger *slog.Logger
}

// LGServer is the looking glass HTTP server.
// Routes are registered internally during construction.
// Caller MUST call Shutdown to release resources when the server is no longer needed.
type LGServer struct {
	mux        *http.ServeMux
	addr       string
	mu         sync.RWMutex  // protects addr after ListenAndServe updates it
	ready      chan struct{} // closed when the listener is bound
	readyOnce  sync.Once     // prevents double-close panic on ready channel
	logger     *slog.Logger
	server     *http.Server
	useTLS     bool
	tlsCfg     *tls.Config
	dispatch   CommandDispatcher
	sseClients atomic.Int32 // concurrent SSE connection counter
}

// NewLGServer creates a new looking glass HTTP server from the given configuration.
// When TLS is enabled, CertPEM and KeyPEM must be provided.
func NewLGServer(cfg LGConfig) (*LGServer, error) {
	if cfg.ListenAddr == "" {
		return nil, fmt.Errorf("lg server: listen address is required")
	}

	if cfg.Dispatch == nil {
		return nil, fmt.Errorf("lg server: command dispatcher is required")
	}

	log := cfg.Logger
	if log == nil {
		log = lgLogger
	}

	var tlsCfg *tls.Config
	if cfg.TLS {
		if len(cfg.CertPEM) == 0 || len(cfg.KeyPEM) == 0 {
			return nil, fmt.Errorf("lg server: TLS enabled but certificate/key PEM data missing")
		}

		cert, err := tls.X509KeyPair(cfg.CertPEM, cfg.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("lg server: parse TLS key pair: %w", err)
		}

		tlsCfg = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	mux := http.NewServeMux()

	s := &LGServer{
		mux:      mux,
		addr:     cfg.ListenAddr,
		ready:    make(chan struct{}),
		logger:   log,
		useTLS:   cfg.TLS,
		tlsCfg:   tlsCfg,
		dispatch: cfg.Dispatch,
		server: &http.Server{
			Addr:    cfg.ListenAddr,
			Handler: securityHeaders(mux),
			// Timeouts prevent slow clients from holding connections indefinitely.
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			IdleTimeout:       120 * time.Second,
			// Suppress TLS handshake errors from browsers rejecting self-signed certs.
			ErrorLog: stdlog.New(io.Discard, "", 0),
		},
	}

	if tlsCfg != nil {
		s.server.TLSConfig = tlsCfg
	}

	// Register route handlers.
	s.registerRoutes()

	return s, nil
}

// registerRoutes sets up the mux with all LG route handlers.
func (s *LGServer) registerRoutes() {
	// API handlers (birdwatcher-compatible REST).
	s.mux.HandleFunc("GET /api/looking-glass/status", s.handleAPIStatus)
	s.mux.HandleFunc("GET /api/looking-glass/protocols/bgp", s.handleAPIProtocols)
	s.mux.HandleFunc("GET /api/looking-glass/routes/protocol/{name}", s.handleAPIRoutesProtocol)
	s.mux.HandleFunc("GET /api/looking-glass/routes/table/{family}", s.handleAPIRoutesTable)
	s.mux.HandleFunc("GET /api/looking-glass/routes/filtered/{name}", s.handleAPIRoutesFiltered)
	s.mux.HandleFunc("GET /api/looking-glass/routes/search", s.handleAPIRoutesSearch)

	// UI handlers (HTMX web pages).
	s.mux.HandleFunc("GET /lg/peers", s.handleUIPeers)
	s.mux.HandleFunc("GET /lg/lookup", s.handleUILookupForm)
	s.mux.HandleFunc("POST /lg/lookup", s.handleUILookup)
	s.mux.HandleFunc("GET /lg/search/aspath", s.handleUIASPathSearchForm)
	s.mux.HandleFunc("POST /lg/search/aspath", s.handleUIASPathSearch)
	s.mux.HandleFunc("GET /lg/search/community", s.handleUICommunitySearchForm)
	s.mux.HandleFunc("POST /lg/search/community", s.handleUICommunitySearch)
	s.mux.HandleFunc("GET /lg/peer/{address}", s.handleUIPeerRoutes)
	s.mux.HandleFunc("GET /lg/route/detail", s.handleUIRouteDetail)
	s.mux.HandleFunc("GET /lg/events", s.handleUIEvents)

	// Graph handler (AS path topology SVG).
	s.mux.HandleFunc("GET /lg/graph", s.handleGraph)

	// Static assets.
	s.mux.HandleFunc("GET /lg/assets/", s.handleAssets)

	// Root redirect.
	s.mux.HandleFunc("GET /lg/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lg/" {
			http.Redirect(w, r, "/lg/peers", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// Root redirect for bare /lg.
	s.mux.HandleFunc("GET /lg", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/lg/peers", http.StatusFound)
	})

	// Site root redirect.
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/lg/peers", http.StatusFound)
	})

	// Catch-all for unknown API paths.
	s.mux.HandleFunc("/api/looking-glass/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSONError(w, http.StatusNotFound, "unknown API endpoint")
	})
}

// ListenAndServe starts the HTTP(S) server on the configured address.
// It blocks until the server is shut down or encounters a fatal error.
func (s *LGServer) ListenAndServe(ctx context.Context) error {
	// Ensure ready channel is closed on any exit path so WaitReady never blocks
	// indefinitely (e.g., when bind fails).
	defer s.readyOnce.Do(func() { close(s.ready) })

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("lg server bind: %w", err)
	}

	// Update address to reflect the actual bound address (e.g., when port is 0).
	s.mu.Lock()
	s.addr = ln.Addr().String()
	s.mu.Unlock()

	s.readyOnce.Do(func() { close(s.ready) })

	if s.useTLS {
		s.logger.Info("lg server listening (TLS)", "address", s.addr)
		tlsLn := tls.NewListener(ln, s.tlsCfg)
		return s.server.Serve(tlsLn)
	}

	s.logger.Info("lg server listening", "address", s.addr)
	return s.server.Serve(ln)
}

// Address returns the configured listen address.
// After ListenAndServe, this reflects the actual bound address.
func (s *LGServer) Address() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.addr
}

// WaitReady blocks until the server has bound its listener and is ready
// to accept connections, or until ctx is canceled.
func (s *LGServer) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown gracefully shuts down the server without interrupting active connections.
func (s *LGServer) Shutdown(ctx context.Context) error {
	s.logger.Info("lg server shutting down")
	return s.server.Shutdown(ctx)
}

// query dispatches a command to the engine and returns the result string.
// On error, returns an empty string (callers check for nil parseJSON result).
func (s *LGServer) query(cmd string) string {
	result, err := s.dispatch(cmd)
	if err != nil {
		s.logger.Debug("dispatch error", "command", cmd, "error", err)
		return ""
	}
	return result
}

// writeJSONError writes a JSON error response with the given HTTP status code.
// Uses json.Marshal for the message to ensure valid JSON escaping.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msgBytes, _ := json.Marshal(message) //nolint:errcheck // marshal of string cannot fail
	if _, err := fmt.Fprintf(w, `{"error":%s}`, msgBytes); err != nil {
		lgLogger.Debug("write error response failed", "error", err)
	}
}

// securityHeaders wraps a handler to set standard security headers on all responses.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// writeSVG writes an SVG response with the correct Content-Type.
func writeSVG(w http.ResponseWriter, svg string) {
	w.Header().Set("Content-Type", "image/svg+xml")
	if _, err := fmt.Fprint(w, svg); err != nil {
		lgLogger.Debug("write svg response failed", "error", err)
	}
}
