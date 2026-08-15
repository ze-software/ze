// Design: docs/architecture/web-interface.md -- Looking glass HTTP server
// Detail: handler_api.go -- Birdwatcher REST API handlers
// Detail: handler_ui.go -- HTMX web UI handlers
// Detail: handler_graph.go -- AS path topology graph handler
// Detail: render.go -- templ component rendering
// Detail: embed.go -- Embedded assets

// Package lg provides the looking glass HTTP server for Ze.
//
// The looking glass exposes BGP session state and route information via
// both an HTMX web UI and a birdwatcher-compatible REST API. It runs as
// a separate HTTP server from the web UI, on its own port. It is read-only,
// and open unless LGConfig.Token is set: a looking glass is an intentionally
// public surface, so the bearer gate in auth.go is opt-in.
//
// TLS is on by default (LGConfig.TLS), because the server binds 0.0.0.0 and
// publishes route data; a deployment behind a TLS-terminating proxy turns it
// off. When TLS is enabled, the server uses the same self-signed certificate
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
	"errors"
	"fmt"
	"io"
	"io/fs"
	stdlog "log"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/errorfragment"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
)

var (
	errLgServerAtLeastOneListen            = errors.New("lg server: at least one listen address is required")
	errLgServerListenAddressMustNot        = errors.New("lg server: listen address must not be empty")
	errLgServerCommandDispatcherIsRequired = errors.New("lg server: command dispatcher is required")
	errLgServerTlsEnabledButCertificate    = errors.New("lg server: TLS enabled but certificate/key PEM data missing")
	errLgServerShutDown                    = errors.New("lg server: server has been shut down")
)

// lgLogger is the structured logger for the looking glass subsystem.
var lgLogger = slogutil.Logger("lg.server")

// maxSSEClients limits concurrent SSE connections to prevent resource exhaustion.
const maxSSEClients = 100

// CommandDispatcher sends a command to the engine and returns the typed
// response. It is an alias for the unified plugin.CommandDispatcher every
// surface shares; the looking glass renders the JSON string at its edge via
// CommandDispatcher.JSON with a zero-value caller identity (public, read-only).
type CommandDispatcher = plugin.CommandDispatcher

// ASNDecorator resolves an AS number string to an organization name.
// Returns empty string on failure (graceful degradation).
type ASNDecorator func(asn string) string

// LGConfig holds the configuration for creating an LG server.
type LGConfig struct {
	// ListenAddrs is the list of addresses to bind (e.g., []string{"0.0.0.0:8443"}).
	// At least one entry is required. Every entry becomes a separate listener
	// on the same *http.Server; Shutdown closes all of them.
	ListenAddrs []string

	// TLS enables HTTPS. When false, the server uses plain HTTP.
	TLS bool

	// CertPEM is optional PEM-encoded certificate data (required when TLS is true).
	CertPEM []byte

	// KeyPEM is optional PEM-encoded private key data (required when TLS is true).
	KeyPEM []byte

	// Dispatch is the command dispatcher for querying the BGP engine.
	// MUST NOT be nil.
	Dispatch CommandDispatcher

	// DecorateASN resolves AS numbers to organization names via Team Cymru DNS.
	// If nil, ASN names are not shown.
	DecorateASN ASNDecorator

	// Token is an optional bearer token gating every route. Empty (the default)
	// leaves the looking glass open, which is its normal posture: a public
	// read-only surface. When set, every request must carry
	// `Authorization: Bearer <Token>`; see auth.go.
	Token string

	// Logger is the structured logger for the LG server.
	// If nil, the package-level lg logger is used.
	Logger *slog.Logger
}

// LGServer is the looking glass HTTP server.
// Routes are registered internally during construction.
// Caller MUST call Shutdown to release resources when the server is no longer needed.
// ListenAndServe binds every address in LGConfig.ListenAddrs before any
// serve goroutine starts; if ANY bind fails the already-bound listeners are
// closed and ListenAndServe returns the error.
type LGServer struct {
	mux *http.ServeMux
	// configured holds the addresses passed in by the caller, in original order.
	configured []string
	// bound holds the actual listen addresses once ListenAndServe has bound
	// them. Populated under mu.
	bound       []string
	listeners   map[string]net.Listener // bound addr -> listener
	mu          sync.RWMutex            // protects bound, listeners, stopped
	ready       chan struct{}           // closed once every listener is bound
	readyOnce   sync.Once               // prevents double-close panic on ready channel
	stopped     bool                    // set by Shutdown; Reconfigure checks this
	logger      *slog.Logger
	server      *http.Server
	useTLS      bool
	tlsCfg      *tls.Config
	dispatch    CommandDispatcher
	decorateASN ASNDecorator
	sseClients  atomic.Int32 // concurrent SSE connection counter
}

// NewLGServer creates a new looking glass HTTP server from the given configuration.
// When TLS is enabled, CertPEM and KeyPEM must be provided.
// Requires at least one entry in cfg.ListenAddrs.
func NewLGServer(cfg LGConfig) (*LGServer, error) {
	if len(cfg.ListenAddrs) == 0 {
		return nil, errLgServerAtLeastOneListen
	}
	if slices.Contains(cfg.ListenAddrs, "") {
		return nil, errLgServerListenAddressMustNot
	}

	if cfg.Dispatch == nil {
		return nil, errLgServerCommandDispatcherIsRequired
	}

	log := cfg.Logger
	if log == nil {
		log = lgLogger
	}

	var tlsCfg *tls.Config
	if cfg.TLS {
		if len(cfg.CertPEM) == 0 || len(cfg.KeyPEM) == 0 {
			return nil, errLgServerTlsEnabledButCertificate
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
	configured := append([]string(nil), cfg.ListenAddrs...)

	s := &LGServer{
		mux:         mux,
		configured:  configured,
		listeners:   make(map[string]net.Listener),
		ready:       make(chan struct{}),
		logger:      log,
		useTLS:      cfg.TLS,
		tlsCfg:      tlsCfg,
		dispatch:    cfg.Dispatch,
		decorateASN: cfg.DecorateASN,
		server: &http.Server{
			// Addr is informational; multi-listener serving uses Serve(ln).
			Addr: configured[0],
			// bearerAuth sits between the headers and the mux, so it gates
			// every route the mux serves -- including any registered later.
			// With no token it returns the mux unchanged (public looking glass).
			//
			// errorfragment.Middleware wraps both, so the 17 http.Error sites in
			// this package answer an htmx request with markup it can swap into
			// the target rather than a bare status line. It is written ONCE
			// here: handler_golden_test.go captures through this same chain, so
			// the daemon and the capture cannot disagree about it.
			Handler: securityHeaders(errorfragment.Middleware(bearerAuth(cfg.Token, mux))),
			// Timeouts prevent slow clients from holding connections indefinitely.
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
			// Suppress TLS handshake errors from browsers rejecting self-signed certs.
			ErrorLog: stdlog.New(io.Discard, "", 0),
		},
	}

	if tlsCfg != nil {
		s.server.TLSConfig = tlsCfg
	}

	// Register route handlers.
	if err := s.registerRoutes(); err != nil {
		return nil, fmt.Errorf("lg server: %w", err)
	}

	return s, nil
}

// registerRoutes sets up the mux with all LG route handlers.
func (s *LGServer) registerRoutes() error {
	// Embedded asset serving.
	assetsDir, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return fmt.Errorf("embedded assets sub-fs: %w", err)
	}
	s.mux.Handle("GET /lg/assets/", http.StripPrefix("/lg/assets/", http.FileServer(http.FS(assetsDir))))

	// API handlers (birdwatcher-compatible REST).
	s.mux.HandleFunc("GET /api/looking-glass/status", s.handleAPIStatus)
	s.mux.HandleFunc("GET /api/looking-glass/protocols/bgp", s.handleAPIProtocols)
	s.mux.HandleFunc("GET /api/looking-glass/protocols/short", s.handleAPIProtocolsShort)
	s.mux.HandleFunc("GET /api/looking-glass/routes/protocol/{name}", s.handleAPIRoutesProtocol)
	s.mux.HandleFunc("GET /api/looking-glass/routes/peer/{peer}", s.handleAPIRoutesPeer)
	s.mux.HandleFunc("GET /api/looking-glass/routes/table/{family}", s.handleAPIRoutesTable)
	s.mux.HandleFunc("GET /api/looking-glass/routes/filtered/{name}", s.handleAPIRoutesFiltered)
	s.mux.HandleFunc("GET /api/looking-glass/routes/export/{name}", s.handleAPIRoutesExport)
	s.mux.HandleFunc("GET /api/looking-glass/routes/noexport/{name}", s.handleAPIRoutesNoExport)
	s.mux.HandleFunc("GET /api/looking-glass/routes/count/protocol/{name}", s.handleAPIRoutesCount)
	s.mux.HandleFunc("GET /api/looking-glass/routes/prefix", s.handleAPIRoutesPrefix)
	s.mux.HandleFunc("GET /api/looking-glass/routes/search", s.handleAPIRoutesSearch)

	// BMP-specific API endpoints (separate from BGP looking glass).
	s.mux.HandleFunc("GET /api/looking-glass/protocols/bmp", s.handleAPIBMPProtocols)
	s.mux.HandleFunc("GET /api/looking-glass/routes/bmp/{name}", s.handleAPIBMPRoutes)

	// UI handlers (HTMX web pages with tab layout).
	s.mux.HandleFunc("GET /lg/peers", s.handleUIPeers)
	s.mux.HandleFunc("GET /lg/search", s.handleUISearchForm)
	s.mux.HandleFunc("POST /lg/search", s.handleUISearch)
	// Legacy /lg/lookup redirects to unified search.
	s.mux.HandleFunc("GET /lg/lookup", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/lg/search", http.StatusFound)
	})
	s.mux.HandleFunc("GET /lg/peer/{address}", s.handleUIPeerRoutes)
	s.mux.HandleFunc("GET /lg/peer/{address}/download", s.handleUIPeerDownload)
	s.mux.HandleFunc("GET /lg/route/detail", s.handleUIRouteDetail)
	s.mux.HandleFunc("GET /lg/events", s.handleUIEvents)

	// Graph handler (AS path topology SVG).
	s.mux.HandleFunc("GET /lg/graph", s.handleGraph)

	// Help page.
	s.mux.HandleFunc("GET /lg/help", s.handleUIHelp)

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

	return nil
}

// resolveASN returns the organization name for an ASN, or empty string if
// no decorator is configured or the lookup fails.
func (s *LGServer) resolveASN(asn string) string {
	if s.decorateASN == nil || asn == "" {
		return ""
	}
	return s.decorateASN(asn)
}

// ListenAndServe binds every configured listen address and starts serving.
// It blocks until the server is shut down or encounters a fatal error.
//
// Bind is all-or-nothing: if ANY listener fails to bind, the already-bound
// listeners are closed and the bind error is returned without entering the
// serve loop. Partial binding is never accepted.
func (s *LGServer) ListenAndServe(ctx context.Context) error {
	// Ensure ready channel is closed on any exit path so WaitReady never blocks
	// indefinitely (e.g., when every bind fails).
	defer s.readyOnce.Do(func() { close(s.ready) })

	var lc net.ListenConfig

	lnSlice := make([]net.Listener, 0, len(s.configured))
	lnMap := make(map[string]net.Listener, len(s.configured))
	bound := make([]string, 0, len(s.configured))
	for _, addr := range s.configured {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			closeAllLGListeners(lnSlice, s.logger)
			return fmt.Errorf("lg server bind %s: %w", addr, err)
		}
		resolvedAddr := ln.Addr().String()
		lnSlice = append(lnSlice, ln)
		lnMap[resolvedAddr] = ln
		bound = append(bound, resolvedAddr)
	}

	s.mu.Lock()
	s.bound = bound
	s.listeners = lnMap
	s.mu.Unlock()

	s.readyOnce.Do(func() { close(s.ready) })

	for _, addr := range bound {
		if s.useTLS {
			s.logger.Info("lg server listening (TLS)", "address", addr)
		} else {
			s.logger.Info("lg server listening", "address", addr)
		}
	}

	return s.serveAll(lnSlice)
}

func (s *LGServer) serveAll(lns []net.Listener) error {
	errCh := make(chan error, len(lns))
	var wg sync.WaitGroup
	for _, ln := range lns {
		serveLn := ln
		if s.useTLS {
			serveLn = tls.NewListener(ln, s.tlsCfg)
		}
		wg.Add(1)
		go func(serveLn net.Listener) {
			defer wg.Done()
			if serveErr := s.server.Serve(serveLn); serveErr != nil &&
				!errors.Is(serveErr, http.ErrServerClosed) &&
				!isLGClosedConnError(serveErr) {
				errCh <- serveErr
			}
		}(serveLn)
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

func (s *LGServer) serveOne(ln net.Listener) {
	serveLn := ln
	if s.useTLS {
		serveLn = tls.NewListener(ln, s.tlsCfg)
	}
	go func() {
		if serveErr := s.server.Serve(serveLn); serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) &&
			!isLGClosedConnError(serveErr) {
			s.logger.Error("lg server serve error", "error", serveErr)
		}
	}()
}

// Reconfigure migrates listeners to a new set of addresses.
// Bind-before-close: new listeners start serving before old ones are removed.
func (s *LGServer) Reconfigure(ctx context.Context, newAddrs []string) error {
	if len(newAddrs) == 0 {
		return errLgServerAtLeastOneListen
	}
	if slices.Contains(newAddrs, "") {
		return errLgServerListenAddressMustNot
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return errLgServerShutDown
	}

	_, toAdd, toRemove := lgListenerDiff(s.bound, newAddrs)
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil
	}

	var lc net.ListenConfig
	newLns := make([]net.Listener, 0, len(toAdd))
	resolved := make(map[string]string, len(toAdd))
	for _, addr := range toAdd {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			closeAllLGListeners(newLns, s.logger)
			return fmt.Errorf("lg server reconfigure bind %s: %w", addr, err)
		}
		newLns = append(newLns, ln)
		resolved[addr] = ln.Addr().String()
	}

	for _, ln := range newLns {
		resolvedAddr := ln.Addr().String()
		s.listeners[resolvedAddr] = ln
		s.logger.Info("lg server listener added", "address", resolvedAddr)
		s.serveOne(ln)
	}

	for _, addr := range toRemove {
		if ln, ok := s.listeners[addr]; ok {
			s.logger.Info("lg server listener removed", "address", addr)
			if closeErr := ln.Close(); closeErr != nil {
				s.logger.Warn("lg server close listener", "address", addr, "error", closeErr)
			}
			delete(s.listeners, addr)
		}
	}

	bound := make([]string, 0, len(newAddrs))
	for _, a := range newAddrs {
		if r, ok := resolved[a]; ok {
			bound = append(bound, r)
		} else if _, ok := s.listeners[a]; ok {
			bound = append(bound, a)
		}
	}
	s.bound = bound
	s.configured = append([]string(nil), newAddrs...)
	return nil
}

func lgListenerDiff(oldAddrs, newAddrs []string) (keep, add, remove []string) {
	oldSet := make(map[string]struct{}, len(oldAddrs))
	for _, a := range oldAddrs {
		oldSet[a] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newAddrs))
	for _, a := range newAddrs {
		newSet[a] = struct{}{}
	}
	for _, a := range newAddrs {
		if _, exists := oldSet[a]; exists {
			keep = append(keep, a)
		} else {
			add = append(add, a)
		}
	}
	for _, a := range oldAddrs {
		if _, exists := newSet[a]; !exists {
			remove = append(remove, a)
		}
	}
	return keep, add, remove
}

func isLGClosedConnError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "use of closed network connection")
}

// closeAllLGListeners closes every listener in the slice, logging any errors.
func closeAllLGListeners(listeners []net.Listener, log *slog.Logger) {
	for _, ln := range listeners {
		if closeErr := ln.Close(); closeErr != nil {
			log.Warn("lg server: close partial listener", "error", closeErr)
		}
	}
}

// Addresses returns every bound listen address, in the order they were
// configured. After ListenAndServe binds, entries reflect the resolved
// ip:port. Before ListenAndServe binds, Addresses returns the configured
// addresses.
func (s *LGServer) Addresses() []string {
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
// only care about the primary endpoint; multi-listener callers should use
// Addresses() instead.
func (s *LGServer) Address() string {
	addrs := s.Addresses()
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0]
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
// After Shutdown, Reconfigure returns errLgServerShutDown.
func (s *LGServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.logger.Info("lg server shutting down")
	return s.server.Shutdown(ctx)
}

// query dispatches a command to the engine and returns the result string.
// On dispatch error, returns a JSON error envelope so callers can surface
// the failure reason instead of showing a generic "engine unavailable".
func (s *LGServer) query(cmd string) string {
	// The looking glass is public and read-only: dispatch with a zero-value
	// caller identity (no username/remote-addr; the injected dispatcher supplies
	// the fixed surface). Render the typed response to its JSON string here.
	result, err := s.dispatch.JSON(context.Background(), plugin.CallerIdentity{}, cmd)
	if err != nil {
		s.logger.Warn("dispatch error", "command", cmd, "error", err)
		b, _ := json.Marshal(map[string]any{"error": err.Error()})
		return string(b)
	}
	return result
}

// writeJSONError writes a JSON error response with the given HTTP status code.
// Uses json.Marshal for the message to ensure valid JSON escaping.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msgBytes, _ := json.Marshal(message)                                //nolint:errcheck // marshal of string cannot fail
	if _, err := fmt.Fprintf(w, `{"error":%s}`, msgBytes); err != nil { //nolint:errcheck // output
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
		w.Header().Set("X-Ze-Version", version.HTTPHeader())
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
