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
// Each listener serves HTTPS or plain HTTP (ListenAddr.TLS), so one looking
// glass serves both at once: what it publishes is public, and the operator
// chooses per endpoint. When a listener serves HTTPS, the caller supplies the
// PEM material: a self-signed certificate, or a chain from the PKI store.
// UpdateTLSCertificate replaces it, and UpdateListenerTLS switches the protocol
// of a bound listener, both on a running server and without rebinding.
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
	"net/netip"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/errorfragment"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/version"
)

var (
	errLgServerAtLeastOneListen            = errors.New("lg server: at least one listen address is required")
	errLgServerListenAddressMustNot        = errors.New("lg server: listen address must not be empty")
	errLgServerCommandDispatcherIsRequired = errors.New("lg server: command dispatcher is required")
	errLgServerTlsEnabledButCertificate    = errors.New("lg server: TLS enabled but certificate/key PEM data missing")
	errLgServerShutDown                    = errors.New("lg server: server has been shut down")
	errLgServerNoCertificate               = errors.New("lg server: no TLS certificate installed")
	// errLgServerNoRecordedProtocol is Reconfigure's refusal of an address no
	// configuration gave a protocol: only a Ze defect in the caller's order
	// reaches it, because the hub names every address through
	// UpdateListenerTLS before it moves the listeners.
	errLgServerNoRecordedProtocol = errors.New("lg server: no protocol recorded for the listen address; UpdateListenerTLS must name it before Reconfigure binds it")
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

// LGConfig holds the configuration for creating an LG server.
type LGConfig struct {
	// ListenAddrs is the list of addresses to bind, each with the protocol it
	// serves. At least one entry is required. Every entry becomes a separate
	// listener on the same *http.Server; Shutdown closes all of them.
	ListenAddrs []ListenAddr

	// CertPEM is PEM-encoded certificate data, required when an entry of
	// ListenAddrs serves TLS. A chain is the leaf first, then one block per
	// intermediate.
	CertPEM []byte

	// KeyPEM is PEM-encoded private key data, required when an entry of
	// ListenAddrs serves TLS.
	KeyPEM []byte

	// Dispatch is the command dispatcher for querying the BGP engine. It also
	// answers the name of each AS number a page shows, through the resolve
	// component's `resolve cymru asn-name` command (as_names.go).
	// MUST NOT be nil.
	Dispatch CommandDispatcher

	// Token is an optional bearer token gating every route. Empty (the default)
	// leaves the looking glass open, which is its normal posture: a public
	// read-only surface. When set, every request must carry
	// `Authorization: Bearer <Token>`; see auth.go.
	Token string

	// NextHopRanges names ranges of next-hop addresses, keyed by the range
	// with its host bits cleared (zeconfig.ExtractLGNextHopRanges). The route
	// table and the next-hop graph show the name of the most specific range
	// beside a next hop inside it. Nil names nothing. UpdateNextHopRanges
	// replaces the list on a running server.
	NextHopRanges map[netip.Prefix]string

	// Hostname is the router name published by the RFC 8522 Looking Glass API
	// (/.well-known/looking-glass/v1/routers and data.router). Empty falls
	// back to os.Hostname(), then "ze".
	Hostname string

	// RFC8522RateLimitPerMin caps requests per client IP per minute on the
	// RFC 8522 API (/.well-known/looking-glass/). Zero selects
	// defaultRFC8522RateLimitPerMin.
	RFC8522RateLimitPerMin int

	// Logger is the structured logger for the LG server.
	// If nil, the package-level lg logger is used.
	Logger *slog.Logger
}

// ListenAddr is one address the looking glass binds, and the protocol it serves
// there.
type ListenAddr struct {
	// Addr is the ip:port to bind (e.g., "0.0.0.0:8443").
	Addr string
	// TLS is true for HTTPS and false for plain HTTP.
	TLS bool
}

// transportListener is one bound listener of the looking glass. Its Accept
// answers a TLS connection or the bare one, as serveTLS says at that moment, so
// UpdateListenerTLS switches the protocol of an address without closing its
// socket: a connection already accepted keeps the protocol it was accepted
// with, and the next one takes the new one. http.Server handshakes a *tls.Conn
// it is handed and sets Request.TLS from it, so a handler tells the two apart
// per connection (handler_rfc8522.go).
//
// Safe for concurrent use.
type transportListener struct {
	net.Listener
	// tlsCfg is the server's one TLS configuration. Its certificate is read
	// per handshake (LGServer.getCertificate).
	tlsCfg   *tls.Config
	serveTLS atomic.Bool
}

// Accept waits for the next connection and wraps it in TLS when this listener
// serves HTTPS. The accept error is returned as the listener gave it:
// http.Server reads its type to tell a closed listener from a transient fault.
func (l *transportListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if !l.serveTLS.Load() {
		return conn, nil
	}
	return tls.Server(conn, l.tlsCfg), nil
}

// LGServer is the looking glass HTTP server.
// Routes are registered internally during construction.
// Caller MUST call Shutdown to release resources when the server is no longer needed.
// ListenAndServe binds every address in LGConfig.ListenAddrs before any
// serve goroutine starts; if ANY bind fails the already-bound listeners are
// closed and ListenAndServe returns the error.
type LGServer struct {
	mux *http.ServeMux
	// configured holds the addresses passed in by the caller, in original
	// order, each with the protocol it serves.
	configured []ListenAddr
	// bound holds the actual listen addresses once ListenAndServe has bound
	// them. Populated under mu.
	bound     []string
	listeners map[string]*transportListener // bound addr -> listener
	// transport is the protocol a configuration gives an address: true for
	// TLS. Reconfigure reads it when it binds an address and refuses one it
	// holds no entry for, so a listener a reload adds, or a rollback puts
	// back, serves the protocol its entry asks for from its first connection.
	// NewLGServer, UpdateListenerTLS and Reconfigure write it.
	transport map[string]bool
	mu        sync.RWMutex  // protects configured, bound, listeners, transport, stopped
	ready     chan struct{} // closed once every listener is bound
	readyOnce sync.Once     // prevents double-close panic on ready channel
	stopped   bool          // set by Shutdown; Reconfigure checks this
	logger    *slog.Logger
	server    *http.Server
	// tlsCfg is the one TLS configuration every TLS-serving listener shares. It
	// is built whether or not a listener serves TLS at start, because a reload
	// can switch one to it.
	tlsCfg *tls.Config
	// cert is the certificate every handshake serves. tls.Config.GetCertificate
	// reads it per handshake rather than per listener, so UpdateTLSCertificate
	// rotates the served material without touching a bound listener. Nil while
	// no certificate was ever installed, which is the state of a server whose
	// listeners all serve plain HTTP. NewLGServer and UpdateListenerTLS refuse
	// a TLS listener on a server that holds none.
	cert       atomic.Pointer[tls.Certificate]
	dispatch   CommandDispatcher
	sseClients atomic.Int32 // concurrent SSE connection counter

	// nextHopRanges is the list of named next-hop ranges every page reads. A
	// reload replaces the pointer whole (UpdateNextHopRanges), so a reader
	// sees the old list or the new one. A nil pointer names nothing: it is
	// the state of a server no list was stored in.
	nextHopRanges atomic.Pointer[nextHopRanges]

	// hostname is the router hostname published by RFC 8522 endpoints. Written
	// once in NewLGServer and immutable thereafter.
	hostname string

	// rfc8522Limiter bounds per-client-IP request rate on the RFC 8522 API.
	rfc8522Limiter *rfc8522RateLimiter
}

// NewLGServer creates a new looking glass HTTP server from the given configuration.
// When an entry of cfg.ListenAddrs serves TLS, CertPEM and KeyPEM must be provided.
// Requires at least one entry in cfg.ListenAddrs.
func NewLGServer(cfg LGConfig) (*LGServer, error) {
	if len(cfg.ListenAddrs) == 0 {
		return nil, errLgServerAtLeastOneListen
	}
	configured := append([]ListenAddr(nil), cfg.ListenAddrs...)
	transport := make(map[string]bool, len(configured))
	servesTLS := false
	for _, listen := range configured {
		if listen.Addr == "" {
			return nil, errLgServerListenAddressMustNot
		}
		transport[listen.Addr] = listen.TLS
		if listen.TLS {
			servesTLS = true
		}
	}

	if cfg.Dispatch == nil {
		return nil, errLgServerCommandDispatcherIsRequired
	}

	log := cfg.Logger
	if log == nil {
		log = lgLogger
	}

	var cert *tls.Certificate
	if servesTLS {
		if len(cfg.CertPEM) == 0 {
			return nil, errLgServerTlsEnabledButCertificate
		}
		if len(cfg.KeyPEM) == 0 {
			return nil, errLgServerTlsEnabledButCertificate
		}

		pair, err := tls.X509KeyPair(cfg.CertPEM, cfg.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("lg server: parse TLS key pair: %w", err)
		}
		cert = &pair
	}

	ranges, rangesErr := newNextHopRanges(cfg.NextHopRanges)
	if rangesErr != nil {
		return nil, rangesErr
	}

	hostname := strings.TrimSpace(cfg.Hostname)
	if hostname == "" {
		if h, hostErr := os.Hostname(); hostErr == nil && strings.TrimSpace(h) != "" {
			hostname = strings.TrimSpace(h)
		} else {
			hostname = "ze"
		}
	}

	mux := http.NewServeMux()

	s := &LGServer{
		mux:            mux,
		configured:     configured,
		listeners:      make(map[string]*transportListener),
		transport:      transport,
		ready:          make(chan struct{}),
		logger:         log,
		dispatch:       cfg.Dispatch,
		hostname:       hostname,
		rfc8522Limiter: newRFC8522RateLimiter(cfg.RFC8522RateLimitPerMin),
		server: &http.Server{
			// Addr is informational; multi-listener serving uses Serve(ln).
			Addr: configured[0].Addr,
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

	s.nextHopRanges.Store(ranges)

	// The certificate sits behind a per-handshake lookup, and the configuration
	// holds no fixed Certificates list. crypto/tls serves that list ahead of
	// the callback when the client sends no SNI, so a populated one would let
	// the pointer and the wire disagree. A bound listener therefore serves
	// rotated material on its NEXT handshake, with no rebind and no dropped
	// connection. The configuration exists on an all-HTTP server too, because
	// UpdateListenerTLS can switch a listener to it.
	if cert != nil {
		s.cert.Store(cert)
	}
	s.tlsCfg = &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: s.getCertificate,
	}
	s.server.TLSConfig = s.tlsCfg

	// Register route handlers.
	if err := s.registerRoutes(); err != nil {
		return nil, fmt.Errorf("lg server: %w", err)
	}

	return s, nil
}

// ListenerTLS answers, for every bound address, whether it serves TLS now.
// The hub reads it to learn whether any listener will present a certificate
// once a reloaded configuration is applied (cmd/ze/hub/listener_migrate.go).
// The map is the caller's own. Safe for concurrent use.
func (s *LGServer) ListenerTLS() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]bool, len(s.listeners))
	for addr, ln := range s.listeners {
		out[addr] = ln.serveTLS.Load()
	}
	return out
}

// UpdateListenerTLS records the protocol each address in tlsByAddr serves, true
// for TLS, and switches every bound listener it names. A switched listener
// keeps its socket: its open connections finish on the protocol they started
// with, and its next connection takes the new one. An address that is not
// bound is recorded only, and Reconfigure reads the record when it binds it:
// a caller that moves the listeners MUST name every address of the new set
// here first, and the hub does, so a listener a reload adds serves its
// entry's protocol from its first connection. A bound address is named as
// Addresses reports it, an address to bind as Reconfigure will be given it.
//
// The record grows by at most one entry per address named here, and the next
// Reconfigure that moves a listener rebuilds it from the addresses it keeps
// and the ones it removes.
//
// It refuses, and changes nothing, when an address would serve TLS on a server
// that holds no certificate: every handshake there would fail on a listener
// that answered a moment earlier. Callers MUST install the certificate with
// UpdateTLSCertificate first.
//
// Safe for concurrent use.
func (s *LGServer) UpdateListenerTLS(tlsByAddr map[string]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return errLgServerShutDown
	}
	if s.cert.Load() == nil {
		for _, serveTLS := range tlsByAddr {
			if serveTLS {
				return errLgServerNoCertificate
			}
		}
	}

	for addr, serveTLS := range tlsByAddr {
		s.transport[addr] = serveTLS
		ln, bound := s.listeners[addr]
		if !bound {
			continue
		}
		if ln.serveTLS.Swap(serveTLS) != serveTLS {
			s.logger.Info("lg server listener protocol changed", "address", addr, "tls", serveTLS)
		}
	}
	return nil
}

// getCertificate is the tls.Config.GetCertificate callback. It returns the
// certificate installed now, which UpdateTLSCertificate can replace at any time.
// Safe for concurrent use.
func (s *LGServer) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert := s.cert.Load()
	if cert == nil {
		// No listener should reach this. A TLS listener is made by
		// NewLGServer or UpdateListenerTLS, which both refuse it on a server
		// with no certificate, or by Reconfigure from a record one of them
		// wrote, or from a listener that served TLS when it was removed. And
		// UpdateTLSCertificate never installs nil. A handshake is a peer's
		// action, so this refuses it rather than panic, and rather than let
		// crypto/tls fall through to an empty Certificates list.
		return nil, errLgServerNoCertificate
	}
	return cert, nil
}

// UpdateTLSCertificate replaces the certificate served on every later
// handshake. Bound listeners are untouched, so open connections, SSE streams
// among them, survive the rotation.
//
// It is fail-closed on bad material: material that does not parse is refused
// and the certificate installed before it keeps serving. Installing unparseable
// material, or clearing the certificate, would break every later handshake on a
// listener that worked a moment earlier.
//
// A server whose listeners all serve plain HTTP accepts the certificate too:
// that is how a reload switches one of them to TLS, certificate first, then
// UpdateListenerTLS.
//
// Safe for concurrent use.
func (s *LGServer) UpdateTLSCertificate(certPEM, keyPEM []byte) error {
	if len(certPEM) == 0 {
		return errLgServerTlsEnabledButCertificate
	}
	if len(keyPEM) == 0 {
		return errLgServerTlsEnabledButCertificate
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("lg server: rotate TLS certificate: %w", err)
	}

	s.cert.Store(&cert)
	s.logger.Info("lg server TLS certificate rotated")
	return nil
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

	// RFC 8522 Section 2: Looking Glass Command Set under /.well-known/looking-glass.
	s.mux.HandleFunc("/.well-known/looking-glass", s.handleRFC8522)
	s.mux.HandleFunc("/.well-known/looking-glass/", s.handleRFC8522)

	return nil
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

	s.mu.RLock()
	configured := append([]ListenAddr(nil), s.configured...)
	s.mu.RUnlock()

	lnSlice := make([]*transportListener, 0, len(configured))
	lnMap := make(map[string]*transportListener, len(configured))
	bound := make([]string, 0, len(configured))
	for _, listen := range configured {
		ln, err := lc.Listen(ctx, "tcp", listen.Addr)
		if err != nil {
			closeAllLGListeners(lnSlice, s.logger)
			return fmt.Errorf("lg server bind %s: %w", listen.Addr, err)
		}
		resolvedAddr := ln.Addr().String()
		served := s.newTransportListener(ln, listen.TLS)
		lnSlice = append(lnSlice, served)
		lnMap[resolvedAddr] = served
		bound = append(bound, resolvedAddr)
	}

	s.mu.Lock()
	s.bound = bound
	s.listeners = lnMap
	s.mu.Unlock()

	s.readyOnce.Do(func() { close(s.ready) })

	for i, addr := range bound {
		if configured[i].TLS {
			s.logger.Info("lg server listening (TLS)", "address", addr)
		} else {
			s.logger.Info("lg server listening", "address", addr)
		}
	}

	return s.serveAll(lnSlice)
}

// newTransportListener wraps a bound listener with the protocol it serves.
func (s *LGServer) newTransportListener(ln net.Listener, serveTLS bool) *transportListener {
	served := &transportListener{Listener: ln, tlsCfg: s.tlsCfg}
	served.serveTLS.Store(serveTLS)
	return served
}

func (s *LGServer) serveAll(lns []*transportListener) error {
	errCh := make(chan error, len(lns))
	var wg sync.WaitGroup
	for _, ln := range lns {
		wg.Add(1)
		go func(serveLn net.Listener) {
			defer wg.Done()
			if serveErr := s.server.Serve(serveLn); serveErr != nil &&
				!errors.Is(serveErr, http.ErrServerClosed) &&
				!isLGClosedConnError(serveErr) {
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

func (s *LGServer) serveOne(ln *transportListener) {
	go func() {
		if serveErr := s.server.Serve(ln); serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) &&
			!isLGClosedConnError(serveErr) {
			s.logger.Error("lg server serve error", "error", serveErr)
		}
	}()
}

// Reconfigure migrates listeners to a new set of addresses.
// Bind-before-close: new listeners start serving before old ones are removed.
//
// An address it binds serves, from its first connection, the protocol recorded
// for it: the one UpdateListenerTLS named, or the one it served when an earlier
// call removed it, which is what a rollback puts back. The hub names every
// address of the reloaded configuration through UpdateListenerTLS before this
// call. An address with no record is refused before anything is bound: the
// server does not choose a protocol the configuration never gave it. A
// listener that stays bound keeps its socket and its protocol here.
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

	for _, addr := range toAdd {
		if _, recorded := s.transport[addr]; !recorded {
			return fmt.Errorf("lg server reconfigure %s: %w", addr, errLgServerNoRecordedProtocol)
		}
	}

	var lc net.ListenConfig
	newLns := make([]*transportListener, 0, len(toAdd))
	resolved := make(map[string]string, len(toAdd))
	for _, addr := range toAdd {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			closeAllLGListeners(newLns, s.logger)
			return fmt.Errorf("lg server reconfigure bind %s: %w", addr, err)
		}
		newLns = append(newLns, s.newTransportListener(ln, s.transport[addr]))
		resolved[addr] = ln.Addr().String()
	}

	for _, ln := range newLns {
		resolvedAddr := ln.Addr().String()
		s.listeners[resolvedAddr] = ln
		s.logger.Info("lg server listener added", "address", resolvedAddr, "tls", ln.serveTLS.Load())
		s.serveOne(ln)
	}

	// The record of protocols is bounded: it keeps one entry per address of
	// newAddrs, as written and as bound, and one per address this call
	// removes, with the protocol it served when it was closed. The second set
	// is what a rollback reads: the hub undoes a failed reload by calling
	// Reconfigure with the addresses it had, and a listener put back that way
	// serves what it served before. The next call drops those entries.
	transport := make(map[string]bool, 2*len(newAddrs)+len(toRemove))
	for _, addr := range toRemove {
		if ln, ok := s.listeners[addr]; ok {
			s.logger.Info("lg server listener removed", "address", addr)
			transport[addr] = ln.serveTLS.Load()
			if closeErr := ln.Close(); closeErr != nil {
				s.logger.Warn("lg server close listener", "address", addr, "error", closeErr)
			}
			delete(s.listeners, addr)
		}
	}

	bound := make([]string, 0, len(newAddrs))
	configured := make([]ListenAddr, 0, len(newAddrs))
	for _, a := range newAddrs {
		boundAddr := a
		if r, ok := resolved[a]; ok {
			boundAddr = r
		}
		ln, ok := s.listeners[boundAddr]
		if !ok {
			continue
		}
		serveTLS := ln.serveTLS.Load()
		bound = append(bound, boundAddr)
		configured = append(configured, ListenAddr{Addr: a, TLS: serveTLS})
		transport[a] = serveTLS
		transport[boundAddr] = serveTLS
	}
	s.bound = bound
	s.configured = configured
	s.transport = transport
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
func closeAllLGListeners(listeners []*transportListener, log *slog.Logger) {
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
	out := make([]string, 0, len(s.configured))
	for _, listen := range s.configured {
		out = append(out, listen.Addr)
	}
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

// query dispatches a command to the engine and returns the JSON answer.
//
// A failed dispatch returns its error and no answer. It used to come back as
// the string {"error": ...}, which parsed, so every handler that branched only
// on a nil parse read the failure as data and answered 200 over it
// (plan/journal/zero-value-as-valid-answer.md, 2026-09-20). Callers MUST
// handle the error; ask does that for every page and endpoint.
func (s *LGServer) query(cmd string) (string, error) {
	return s.queryCtx(context.Background(), cmd)
}

// queryCtx dispatches a command with the caller's context and returns the JSON answer.
func (s *LGServer) queryCtx(ctx context.Context, cmd string) (string, error) {
	// The looking glass is public and read-only: dispatch with a zero-value
	// caller identity (no username/remote-addr; the injected dispatcher supplies
	// the fixed surface). Render the typed response to its JSON string here.
	result, err := s.dispatch.JSON(ctx, plugin.CallerIdentity{}, cmd)
	defer result.TransportComplete()
	if err != nil {
		s.logger.Warn("dispatch error", "command", cmd, "error", err)
		return "", err
	}
	return result.Output, nil
}

// The three classes a public refusal names. The looking glass answers anybody
// who can reach it, so a refusal names the command and one of these, and never
// the dispatcher's own error: that text can carry a socket path or another
// detail of the host. ask and query log the full reason for the operator.
const (
	// failureEngineError: the dispatch failed, or the engine answered with its
	// error envelope.
	failureEngineError = "engine error"
	// failureEngineUnavailable: the engine answered nothing at all.
	failureEngineUnavailable = "engine unavailable"
	// failureUnreadableReply: the engine answered, and the answer is not a
	// JSON object.
	failureUnreadableReply = "unreadable reply"
)

// engineFailure says why a command produced no answer, and which HTTP status
// that failure owes (docs/architecture/api/birdwatcher-compat.md Section 3.1).
type engineFailure struct {
	// status is http.StatusBadGateway when the engine answered with an error,
	// and http.StatusServiceUnavailable when it answered nothing parseable.
	status int
	// command is the command that produced no answer.
	command string
	// class is one of the failure constants above, the only part of the
	// reason a client is shown: engineRefusal words the why from it
	// (failureWhy) and names the log that holds the rest.
	class string
	// envelope is true when the engine answered done and the answer itself was
	// an {"error": ...} object, which is how bgp-rib refuses a command it
	// cannot parse. The peer-scoped route endpoints answer that one with 404.
	envelope bool
	// refusal is the envelope's own message, set only when envelope is true.
	// It is the engine's verdict on the request, never a transport error.
	refusal string
	// timedOut is true when the caller context deadline expired during dispatch.
	timedOut bool
}

// engineLogFix is the fix every engine failure names. ask and query log the
// full reason at WARN under this server's logger, lgLogger, and the looking
// glass shows none of it (the three classes above), so the operator reads it
// in the daemon log.
const engineLogFix = "ze show log recent level warn component lg.server"

// engineRefusal is the operator error a failed command answers, on every
// surface: the plain-text pages, the JSON API, the banners and the peer
// stream. Its first line is the command and the failure class, the text
// docs/architecture/api/birdwatcher-compat.md Section 3.1 names; the second
// says why there is no answer; the last is the command that shows the full
// reason (engineLogFix).
//
// failure MUST NOT be nil: a nil failure is an answer, and has no refusal.
func engineRefusal(failure *engineFailure) *opererr.Error {
	return opererr.New(failure.command+": "+failure.class, failureWhy(failure.class), engineLogFix)
}

// failureWhy says, for a failure class, why the command has no answer to
// show. ask is the only producer of an engineFailure, and it sets class to
// one of the three constants, so another value is a Ze defect.
func failureWhy(class string) string {
	switch class {
	case failureEngineError:
		return "the engine failed or refused this command, so there is no answer to show"
	case failureEngineUnavailable:
		return "the engine sent no answer to this command, so there is nothing to show"
	case failureUnreadableReply:
		return "the engine's answer to this command is not a JSON object, so the looking glass cannot read it"
	default:
		panic("BUG: engineFailure built with an unknown class: " + class)
	}
}

// message returns the text a view shows for the failure: engineRefusal's
// text. It returns the empty string when there is no failure, which is what a
// view's Error field means by no error. A writer that answers a failure it
// holds calls engineRefusal itself.
func (f *engineFailure) message() string {
	if f == nil {
		return ""
	}
	return engineRefusal(f).Error()
}

// ask dispatches one command and returns the answer it parsed, or the failure
// that stands in place of one. Exactly one of the two is non-nil. Every
// failure is logged here or in query, so a caller that drops one has not hidden
// it from the operator.
//
// Four replies are not answers: a dispatch that failed, an empty reply, a
// reply that is not a JSON object, and a reply that is the engine's error
// envelope.
func (s *LGServer) ask(command string) (map[string]any, *engineFailure) {
	return s.askCtx(context.Background(), command)
}

// askCtx dispatches one command with ctx and returns the parsed JSON answer or
// the failure that stands in place of one.
func (s *LGServer) askCtx(ctx context.Context, command string) (map[string]any, *engineFailure) {
	output, err := s.queryCtx(ctx, command)
	if err != nil {
		isTimeout := errors.Is(err, context.DeadlineExceeded)
		if !isTimeout {
			isTimeout = errors.Is(ctx.Err(), context.DeadlineExceeded)
		}
		if isTimeout {
			return nil, &engineFailure{
				status:   http.StatusGatewayTimeout,
				command:  command,
				class:    failureEngineError,
				timedOut: true,
			}
		}
		return nil, &engineFailure{status: http.StatusBadGateway, command: command, class: failureEngineError}
	}

	if output == "" {
		s.logger.Warn("engine answered nothing", "command", command)
		return nil, &engineFailure{status: http.StatusServiceUnavailable, command: command, class: failureEngineUnavailable}
	}

	ze := parseJSON(output)
	if ze == nil {
		s.logger.Warn("engine reply is not a JSON object", "command", command, "octets", len(output))
		return nil, &engineFailure{status: http.StatusServiceUnavailable, command: command, class: failureUnreadableReply}
	}

	if refusal, refused := ze["error"].(string); refused {
		s.logger.Warn("engine refused command", "command", command, "error", refusal)
		return nil, &engineFailure{
			status: http.StatusBadGateway, command: command, class: failureEngineError,
			envelope: true, refusal: refusal,
		}
	}

	return ze, nil
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
//
// The version banner is the one header an operator can turn off. `environment
// { hide-version true; }` keeps the build fingerprint off the wire here and on
// the web interface together, and it changes no other header.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if !version.HTTPHeaderHidden() {
			w.Header().Set("X-Ze-Version", version.HTTPHeader())
		}
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
