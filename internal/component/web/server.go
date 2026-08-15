// Design: docs/architecture/web-interface.md -- Web server infrastructure
// Related: doctor.go -- the doctor check over this listener's TLS material
// Related: auth.go -- SecurityHeaders, the wrapper this server puts over its mux

// Package web provides the HTTPS web interface for ze.
//
// The server uses self-signed TLS certificates (ECDSA P-256) that are
// generated on first start and stored via a CertStore interface. Callers
// can also supply pre-existing PEM-encoded certificate and key material
// to skip generation entirely.
//
// Route handlers are registered externally via HandleFunc; the web package
// owns transport (TLS, listen, serve) but not application logic.
package web

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/slogutil"
)

var (
	errWebServerAtLeastOneListen     = errors.New("web server: at least one listen address is required")
	errWebServerListenAddressMustNot = errors.New("web server: listen address must not be empty")
	errWebServerCertificateAndKeyPem = errors.New("web server: certificate and key PEM data are required")
	errWebServerShutDown             = errors.New("web server: server has been shut down")
	errWebServerNoCertificate        = errors.New("web server: no TLS certificate installed")
)

// serverLogger is the structured logger for the web server subsystem.
// The auth logger is declared separately in auth.go as "web.auth".
var serverLogger = slogutil.Logger("web.server")

// WebConfig holds the configuration for creating a WebServer.
type WebConfig struct {
	// ListenAddrs is the list of addresses to bind (e.g., []string{"127.0.0.1:8443"}).
	// At least one entry is required. Every entry becomes a separate listener
	// on the same *http.Server; Shutdown closes all of them.
	ListenAddrs []string

	// CertPEM is optional PEM-encoded certificate data.
	// When set together with KeyPEM, certificate generation is skipped.
	CertPEM []byte

	// KeyPEM is optional PEM-encoded private key data.
	// When set together with CertPEM, certificate generation is skipped.
	KeyPEM []byte

	// Logger is the structured logger for the web server.
	// If nil, the package-level web logger is used.
	Logger *slog.Logger
}

// WebServer is the HTTPS web server.
// Routes are registered via HandleFunc before calling ListenAndServe.
// Callers MUST call Shutdown to release resources when the server is no longer needed.
// ListenAndServe binds every address in WebConfig.ListenAddrs before any
// serve goroutine starts; if ANY bind fails the already-bound listeners are
// closed and ListenAndServe returns the error.
type WebServer struct {
	mux       *http.ServeMux
	tlsConfig *tls.Config
	// configured holds the addresses passed in by the caller, in the original
	// order. Used at bind time.
	configured []string
	// bound holds the actual listen addresses once ListenAndServe has bound
	// them. For port 0 this differs from configured. Populated under mu.
	bound []string
	// listeners maps each bound address to its net.Listener. Populated by
	// ListenAndServe, updated by Reconfigure. Protected by mu.
	listeners map[string]net.Listener
	mu        sync.RWMutex  // protects bound, listeners, and stopped
	ready     chan struct{} // closed once every listener is bound
	stopped   bool          // set by Shutdown; Reconfigure checks this
	logger    *slog.Logger
	server    *http.Server
	// cert is the certificate every handshake serves. tls.Config.GetCertificate
	// reads it per handshake rather than per listener, so UpdateTLSCertificate
	// rotates the served material without touching a bound listener. Never nil
	// after NewWebServer returns.
	cert atomic.Pointer[tls.Certificate]
}

// NewWebServer creates a new WebServer from the given configuration.
// It requires TLS material (CertPEM and KeyPEM) to be present in cfg, and
// at least one entry in cfg.ListenAddrs.
// Use selfcert.LoadOrGenerateCert to obtain PEM data from a CertStore before
// calling NewWebServer.
func NewWebServer(cfg WebConfig) (*WebServer, error) {
	if len(cfg.ListenAddrs) == 0 {
		return nil, errWebServerAtLeastOneListen
	}
	if slices.Contains(cfg.ListenAddrs, "") {
		return nil, errWebServerListenAddressMustNot
	}

	log := cfg.Logger
	if log == nil {
		log = serverLogger
	}

	if len(cfg.CertPEM) == 0 || len(cfg.KeyPEM) == 0 {
		return nil, errWebServerCertificateAndKeyPem
	}

	tlsCfg, err := selfcert.NewTLSConfig(cfg.CertPEM, cfg.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("web server: %w", err)
	}

	mux := http.NewServeMux()
	configured := append([]string(nil), cfg.ListenAddrs...)

	srv := &WebServer{
		mux:        mux,
		tlsConfig:  tlsCfg,
		configured: configured,
		listeners:  make(map[string]net.Listener),
		ready:      make(chan struct{}),
		logger:     log,
		server: &http.Server{
			// Addr is informational; a multi-listener server binds via Serve(ln)
			// and does not use Server.Addr for ListenAndServe.
			Addr:      configured[0],
			Handler:   serverHandler(mux),
			TLSConfig: tlsCfg,
			// Timeouts prevent slow clients from holding connections indefinitely.
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
			// Suppress TLS handshake errors from browsers rejecting self-signed certs.
			ErrorLog: stdlog.New(io.Discard, "", 0),
		},
	}

	// Move the certificate behind a per-handshake lookup. crypto/tls consults
	// GetCertificate before Certificates, so an already-bound listener picks up
	// a rotated certificate on its NEXT handshake with no rebind and no dropped
	// connection (AC-9). Certificates is cleared so the two can never disagree
	// about what is being served.
	srv.cert.Store(&tlsCfg.Certificates[0])
	tlsCfg.Certificates = nil
	tlsCfg.GetCertificate = srv.getCertificate

	return srv, nil
}

// getCertificate is the tls.Config.GetCertificate callback. It returns the
// certificate currently installed, which UpdateTLSCertificate can replace at any
// time.
func (s *WebServer) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert := s.cert.Load()
	if cert == nil {
		// Unreachable: NewWebServer installs a certificate before returning and
		// UpdateTLSCertificate never installs nil. Refuse the handshake rather
		// than let crypto/tls fall through to an empty Certificates list, which
		// would serve nothing while looking like a working listener.
		return nil, errWebServerNoCertificate
	}
	return cert, nil
}

// UpdateTLSCertificate replaces the certificate served on every subsequent
// handshake. Bound listeners are untouched, so open connections (including SSE
// streams) survive the rotation.
//
// It is fail-closed on bad material: material that does not parse is refused and
// the previously installed certificate keeps serving. Installing unparseable
// material, or clearing the certificate, would break every future handshake on a
// listener that was working a moment earlier.
func (s *WebServer) UpdateTLSCertificate(certPEM, keyPEM []byte) error {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return errWebServerCertificateAndKeyPem
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("web server: rotate TLS certificate: %w", err)
	}
	s.cert.Store(&cert)
	s.logger.Info("web server TLS certificate rotated")
	return nil
}

// HandleFunc registers a handler function for the given pattern on the server's mux.
// Patterns follow net/http ServeMux conventions (e.g., "GET /api/status").
func (s *WebServer) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// Handle registers a handler for the given pattern on the server's mux.
func (s *WebServer) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// ListenAndServe binds every configured listen address and starts serving.
// It blocks until the server is shut down or encounters a fatal error.
// The context is used for the initial listener bind; use Shutdown for
// graceful termination of the running server.
//
// Bind is all-or-nothing: if ANY listener fails to bind, the already-bound
// listeners are closed and the bind error is returned without entering the
// serve loop. Partial binding is never accepted.
func (s *WebServer) ListenAndServe(ctx context.Context) error {
	var lc net.ListenConfig

	lnSlice := make([]net.Listener, 0, len(s.configured))
	lnMap := make(map[string]net.Listener, len(s.configured))
	bound := make([]string, 0, len(s.configured))
	for _, addr := range s.configured {
		ln, err := listenAddr(ctx, &lc, addr)
		if err != nil {
			closeAllListeners(lnSlice, s.logger)
			return fmt.Errorf("web server bind %s: %w", addr, err)
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

	close(s.ready)

	for _, addr := range bound {
		s.logger.Info("web server listening", "address", addr)
	}

	return s.serveAll(lnSlice)
}

// serveAll starts a Serve goroutine for each listener and blocks until all exit.
// Returns the first non-shutdown serve error, or nil.
func (s *WebServer) serveAll(lns []net.Listener) error {
	errCh := make(chan error, len(lns))
	var wg sync.WaitGroup
	for _, ln := range lns {
		tlsLn := tls.NewListener(ln, s.tlsConfig)
		wg.Add(1)
		go func(tlsLn net.Listener) {
			defer wg.Done()
			if serveErr := s.server.Serve(tlsLn); serveErr != nil &&
				!errors.Is(serveErr, http.ErrServerClosed) &&
				!isClosedConnError(serveErr) {
				errCh <- serveErr
			}
		}(tlsLn)
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

// listenAddr binds a single address, choosing tcp4 or tcp6 based on the address format.
func listenAddr(ctx context.Context, lc *net.ListenConfig, addr string) (net.Listener, error) {
	network := "tcp4"
	if strings.Contains(addr, "[") {
		network = "tcp6"
	}
	return lc.Listen(ctx, network, addr)
}

// webListenerDiff computes which addresses to keep, add, and remove when
// transitioning from oldAddrs to newAddrs. Private to the web package, matching
// the per-service local copies the other listener services carry
// (restListenerDiff, grpcListenerDiff, lgListenerDiff, mcpListenerDiff); the hub
// migrator has its own listenerDiff, so this needs no cross-package export.
func webListenerDiff(oldAddrs, newAddrs []string) (keep, add, remove []string) {
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

// Reconfigure migrates the server's listeners to a new set of addresses.
// New addresses are bound before old ones are closed, so there is no
// downtime when the old and new sets do not conflict. If any new address
// fails to bind, the error is returned and all original listeners remain
// active (no partial state).
func (s *WebServer) Reconfigure(ctx context.Context, newAddrs []string) error {
	if len(newAddrs) == 0 {
		return errWebServerAtLeastOneListen
	}
	if slices.Contains(newAddrs, "") {
		return errWebServerListenAddressMustNot
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return errWebServerShutDown
	}

	_, toAdd, toRemove := webListenerDiff(s.bound, newAddrs)

	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil
	}

	// Bind all new addresses first. On failure, close only newly-bound
	// listeners and return error (original listeners untouched).
	var lc net.ListenConfig
	newLns := make([]net.Listener, 0, len(toAdd))
	// Maps configured address to its resolved address (differs for port 0).
	resolved := make(map[string]string, len(toAdd))
	for _, addr := range toAdd {
		ln, err := listenAddr(ctx, &lc, addr)
		if err != nil {
			closeAllListeners(newLns, s.logger)
			return fmt.Errorf("web server reconfigure bind %s: %w", addr, err)
		}
		newLns = append(newLns, ln)
		resolved[addr] = ln.Addr().String()
	}

	// Start serving on new listeners.
	for _, ln := range newLns {
		resolvedAddr := ln.Addr().String()
		s.listeners[resolvedAddr] = ln
		s.logger.Info("web server listener added", "address", resolvedAddr)
		s.serveOne(ln)
	}

	// Close removed listeners. The Serve goroutine for each exits when its
	// listener is closed (net.ErrClosed).
	for _, addr := range toRemove {
		if ln, ok := s.listeners[addr]; ok {
			s.logger.Info("web server listener removed", "address", addr)
			if closeErr := ln.Close(); closeErr != nil {
				s.logger.Warn("web server close listener", "address", addr, "error", closeErr)
			}
			delete(s.listeners, addr)
		}
	}

	// Rebuild bound list in the order of newAddrs. For kept addresses, use
	// the existing resolved form; for added addresses, use the fresh resolve.
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

// serveOne starts a Serve goroutine for a single listener.
func (s *WebServer) serveOne(ln net.Listener) {
	tlsLn := tls.NewListener(ln, s.tlsConfig)
	go func() {
		if serveErr := s.server.Serve(tlsLn); serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) &&
			!isClosedConnError(serveErr) {
			s.logger.Error("web server serve error", "error", serveErr)
		}
	}()
}

// isClosedConnError returns true if err is the "use of closed network connection"
// error that net.Listener.Accept returns when the listener is deliberately closed.
func isClosedConnError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "use of closed network connection")
}

// closeAllListeners closes every listener in the slice, logging any errors.
// Used on the bind-failure path to release the partially-acquired set.
func closeAllListeners(listeners []net.Listener, log *slog.Logger) {
	for _, ln := range listeners {
		if closeErr := ln.Close(); closeErr != nil {
			log.Warn("web server: close partial listener", "error", closeErr)
		}
	}
}

// Addresses returns every bound listen address, in the order they were
// configured. After ListenAndServe binds, entries reflect the resolved
// ip:port (differing from the configured form when port was 0). Before
// ListenAndServe binds, Addresses returns the configured addresses.
func (s *WebServer) Addresses() []string {
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
func (s *WebServer) Address() string {
	addrs := s.Addresses()
	if len(addrs) == 0 {
		return ""
	}
	return addrs[0]
}

// WaitReady blocks until every listener has bound and the server is ready
// to accept connections, or until ctx is canceled.
func (s *WebServer) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown gracefully shuts down the server without interrupting active connections.
// It waits for active requests to complete or until the context deadline is reached.
// Shutdown closes every listener that ListenAndServe bound.
// After Shutdown, Reconfigure returns errWebServerShutDown.
func (s *WebServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	s.logger.Info("web server shutting down")
	return s.server.Shutdown(ctx)
}
