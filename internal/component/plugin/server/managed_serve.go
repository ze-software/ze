// Design: docs/architecture/fleet-config.md -- hub-side managed config server (dedicated listener)
// Related: managed.go -- ManagedConfigService (per-client config fetch + version hashing)
//
// ManagedServer is a dedicated TLS listener that serves managed fleet clients. It is
// independent of the plugin PluginAcceptor (which only routes engine-spawned plugin
// processes to WaitForPlugin waiters): a managed client connects inbound at any time
// and has no waiter, so it needs its own accept path. It reuses the ipc auth and rpc
// (MuxConn) transport primitives -- no new auth or wire protocol -- so "No New Server"
// (fleet-config.md) holds in spirit: only a listener bound to the managed server block.
// The client half it serves lives in internal/component/managed/client.go.

package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var managedLogger = slogutil.LazyLogger("hub.managed-server")

const managedAuthTimeout = 10 * time.Second

// managedNotifyBuffer bounds queued config-changed pushes. Config writes are rare
// (admin edits), so a modest buffer absorbs bursts; on overflow a notify is dropped
// and the client picks up the change on its next fetch/reconnect.
const managedNotifyBuffer = 64

// managedMaxConns bounds concurrent connection handlers so an unauthenticated
// connection flood cannot exhaust goroutines/memory (each handler must authenticate
// within managedAuthTimeout or is dropped). Mirrors the plugin acceptor's cap.
const managedMaxConns = 128

// managedPushTimeout bounds a single config-changed round-trip so a client that
// accepts the notification but never replies cannot stall the notify worker
// indefinitely. A healthy client replies immediately (it acks before fetching).
const managedPushTimeout = 10 * time.Second

// managedNotifyWorkers is the size of the config-changed push pool. More than one so a
// single slow/stalled client (blocking up to managedPushTimeout) does not delay pushes
// to other clients.
const managedNotifyWorkers = 4

// ManagedServerConfig configures the dedicated managed-config TLS server.
type ManagedServerConfig struct {
	Addrs         []string          // Listen addresses (server blocks that declare client entries).
	ClientSecrets map[string]string // Per-client name -> secret (authoritative for managed clients).
	ReadConfig    ConfigReader      // Reads a client's config by name (over the hub blob store).
	Metrics       metrics.Registry  // Optional; nil installs no-op counters.

	// Certificate names the pki store certificate the listener serves
	// (plugin/hub/server/certificate). An empty name generates an ephemeral
	// self-signed certificate. A client can verify that one only by pinning its
	// fingerprint, and the fingerprint changes on every hub restart.
	Certificate string

	// TLSMaterialResolver resolves a certificate NAME into serving PEM material
	// (leaf plus any intermediates, and the private key). It exists because the
	// pki component imports this package, so this package cannot import pki:
	// the hub injects pki.ServerTLSMaterial here. Same shape as
	// dnsserver.Options.TLSMaterialResolver, which the DoT/DoH listeners use.
	//
	// nil means no store reference is supported. A Certificate name given
	// anyway is an error, never a silent fallback.
	TLSMaterialResolver func(name string) (certPEM, keyPEM []byte, err error)

	// Authority is the daemon's certificate authority, used to issue the
	// listener's leaf when Certificate names nothing. Injected for the same
	// reason as TLSMaterialResolver: pki imports this package.
	//
	// nil with no Certificate name is an error, never a self-signed fallback.
	// A leaf nothing issued is one a client can neither validate nor rotate.
	Authority plugin.Authority

	// Clock decides when an ISSUED leaf is reissued. nil installs
	// clock.RealClock{}, and a test passes a fake clock to move the leaf's
	// expiry rather than wait for it. A named certificate reads no clock: the
	// operator owns that material and Ze never replaces it.
	Clock clock.Clock
}

// managedLeafCommonName names the managed listener in the subject of the leaf
// it serves when no pki certificate is configured. It is not a hostname and is
// never matched against one; the SANs are.
const managedLeafCommonName = "ze-managed-hub"

// managedLoopbackHost is where a co-located client reaches the managed
// listener, so every issued leaf carries it whatever else is bound.
const managedLoopbackHost = "127.0.0.1"

// ManagedServer authenticates managed fleet clients (per-client secret), answers
// config-fetch/config-ack/ping over MuxConn, and pushes config-changed to connected
// clients. It owns its listeners and one goroutine per connection.
type ManagedServer struct {
	svc    *ManagedConfigService
	lookup func(name string) (string, bool)
	addrs  []string

	// getCertificate answers the served certificate for each handshake. It is
	// tls.Config.GetCertificate, so an issued leaf is reissued as it ages
	// rather than served past its expiry.
	getCertificate func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	// certName is the configured pki certificate name, empty when the leaf was
	// issued by the daemon root. Kept only so Start can name it in its log.
	certName string

	mu    sync.Mutex
	conns map[string]*rpc.MuxConn // Connected client name -> mux, for config-changed push.

	notifyCh chan string   // Client names whose config changed; drained by notifyWorker.
	sem      chan struct{} // Bounds concurrent connection handlers (managedMaxConns).

	// Metrics handles (no-op unless a registry was supplied). Set once at construction,
	// so they are read without a lock.
	mConnected metrics.Gauge      // ze_managed_clients_connected
	mFetch     metrics.CounterVec // ze_managed_config_fetch_total{result}
	mPushed    metrics.Counter    // ze_managed_config_changed_pushed_total

	listeners []net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewManagedServer builds a managed server. ReadConfig is required.
//
// The listener serves the named pki store certificate when cfg.Certificate is
// set. With no name it serves a leaf cfg.Authority issued from the daemon's
// root. A managed client verifies either one the same way, by validating the
// chain against a CA it holds: the public CA that issued the named certificate,
// or the daemon root the operator exported into the client's pki ca block. The
// root outlives a restart, so a reissued leaf needs no client change.
func NewManagedServer(cfg ManagedServerConfig) (*ManagedServer, error) {
	if cfg.ReadConfig == nil {
		return nil, errors.New("managed server: ReadConfig is required")
	}
	getCertificate, err := managedCertificate(cfg)
	if err != nil {
		return nil, err
	}
	secrets := cfg.ClientSecrets
	reg := cfg.Metrics
	if reg == nil {
		reg = metrics.NopRegistry{}
	}
	return &ManagedServer{
		svc: newManagedConfigService(cfg.ReadConfig),
		lookup: func(name string) (string, bool) {
			s, ok := secrets[name]
			return s, ok
		},
		addrs:          cfg.Addrs,
		getCertificate: getCertificate,
		certName:       cfg.Certificate,
		conns:          make(map[string]*rpc.MuxConn),
		notifyCh:       make(chan string, managedNotifyBuffer),
		sem:            make(chan struct{}, managedMaxConns),
		mConnected: reg.Gauge("ze_managed_clients_connected",
			"Number of managed fleet clients currently connected to the hub."),
		mFetch: reg.CounterVec("ze_managed_config_fetch_total",
			"Managed config-fetch requests served, by result (served|current|error).", []string{"result"}),
		mPushed: reg.Counter("ze_managed_config_changed_pushed_total",
			"config-changed notifications pushed to connected managed clients."),
	}, nil
}

// managedCertificate returns what answers the managed listener's certificate
// for each handshake, as tls.Config.GetCertificate.
//
// The CONFIGURED name is answered first and issuance is never reached when one
// is set. It FAILS CLOSED there: an unresolvable name returns an error and no
// certificate, and the caller disables the listener. Such a listener looks like
// a working deployment while the config names a real certificate, until a
// client refuses the handshake. pki.ServerTLSMaterial, the resolver injected
// here, fails closed for the same reason.
//
// A named certificate is answered UNCHANGED at every handshake. That material
// is the operator's, so Ze does not reissue it, and renewing it is the
// operator's job through the pki store.
//
// With no name the daemon's certificate authority issues the leaf, and
// ServingLeaf reissues it as it ages: a hub that runs longer than one leaf
// lives keeps presenting a valid certificate. No authority is an error too: a
// self-signed leaf is one no client can validate and no operator can rotate.
func managedCertificate(cfg ManagedServerConfig) (func(*tls.ClientHelloInfo) (*tls.Certificate, error), error) {
	if cfg.Certificate != "" {
		if cfg.TLSMaterialResolver == nil {
			var tb textbuf.Buffer
			tb.Str("managed server: certificate ").Str(cfg.Certificate).
				Str(" is configured but this hub resolves no certificate names")
			return nil, errors.New(tb.String())
		}
		certPEM, keyPEM, err := cfg.TLSMaterialResolver(cfg.Certificate)
		if err != nil {
			return nil, fmt.Errorf("managed server: certificate %q: %w", cfg.Certificate, err)
		}
		pair, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("managed server: certificate %q: %w", cfg.Certificate, err)
		}
		return func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &pair, nil }, nil
	}

	if cfg.Authority == nil {
		return nil, errors.New("managed server: no certificate name and no certificate authority")
	}
	leaf, err := plugin.NewServingLeaf(cfg.Authority, managedLeafCommonName, managedLeafHosts(cfg.Addrs), cfg.Clock)
	if err != nil {
		return nil, fmt.Errorf("managed server: %w", err)
	}
	return leaf.Certificate, nil
}

// managedLeafHosts returns the SANs the issued leaf needs: the loopback every
// co-located client reaches, plus the host of each listen address that names
// one. An unspecified address (0.0.0.0 or ::) names no host a peer can verify,
// so it contributes nothing and the loopback stands for it.
func managedLeafHosts(addrs []string) []string {
	hosts := make([]string, 0, len(addrs)+1)
	hosts = append(hosts, managedLoopbackHost)
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil || host == "" || host == managedLoopbackHost {
			continue
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
			continue
		}
		hosts = append(hosts, host)
	}
	return hosts
}

// Addrs returns the bound listener addresses (useful when a port-0 was requested,
// and for tests to discover the ephemeral port).
func (s *ManagedServer) Addrs() []net.Addr {
	out := make([]net.Addr, 0, len(s.listeners))
	for _, ln := range s.listeners {
		out = append(out, ln.Addr())
	}
	return out
}

// Start binds the listeners and begins serving in background goroutines. It returns
// once binding succeeds; serving continues until Stop or ctx cancellation.
func (s *ManagedServer) Start(ctx context.Context) error {
	if len(s.addrs) == 0 {
		return errors.New("managed server: no listen addresses")
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	// Bind each address independently so one unavailable address (e.g. a server block
	// whose port collides with the plugin acceptor's listener) disables only that block,
	// not the whole managed server. The collision is logged loudly so the operator can
	// move managed clients to their own server block.
	for _, addr := range s.addrs {
		lns, err := pluginipc.StartListeners([]string{addr}, s.getCertificate)
		if err != nil {
			managedLogger().Error("managed listener disabled: address unavailable "+
				"(does it collide with the plugin acceptor's server block?)", "addr", addr, "error", err)
			continue
		}
		s.listeners = append(s.listeners, lns...)
	}
	if len(s.listeners) == 0 {
		s.cancel() // Release the context: no listeners bound, nothing to serve.
		return errors.New("managed server: no listeners could bind")
	}
	for _, ln := range s.listeners {
		s.wg.Go(func() { s.acceptLoop(ln) })
	}
	for range managedNotifyWorkers {
		s.wg.Go(s.notifyWorker) // long-lived pool: drains config-changed pushes
	}
	s.wg.Go(s.closeOnDone) // long-lived: closes listeners on shutdown
	managedLogger().Info("managed config server listening",
		"listeners", len(s.listeners),
		"certificate", s.certName)
	if s.certName == "" {
		managedLogger().Info("managed config server: no certificate configured, so it serves a leaf issued by " +
			"this daemon's certificate authority. Export the root and name it in each client's pki ca block")
	}
	return nil
}

func (s *ManagedServer) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return // Server stopped.
			}
			managedLogger().Debug("managed accept error", "error", err)
			continue
		}
		// Acquire a handler slot (backpressure when saturated). Abandon the connection
		// if the server is shutting down rather than block forever.
		select {
		case s.sem <- struct{}{}:
		case <-s.ctx.Done():
			conn.Close() //nolint:errcheck,gosec // shutting down
			return
		}
		s.wg.Go(func() {
			defer func() { <-s.sem }()
			s.handleConn(conn)
		})
	}
}

func (s *ManagedServer) handleConn(conn net.Conn) {
	authCtx, cancel := context.WithTimeout(s.ctx, managedAuthTimeout)
	defer cancel()

	// Per-client secret only: no shared-secret fallback (empty sharedSecret), so an
	// unknown name cannot connect. The name is taken ONLY from the authenticated
	// session, never from a later request payload (config isolation: a client can
	// only fetch its own config).
	name, err := pluginipc.AuthenticateWithLookup(authCtx, conn, "", s.lookup)
	if err != nil {
		return // AuthenticateWithLookup already closed conn on failure.
	}

	// One connection per name. A duplicate is refused so a second session cannot
	// shadow the first (RegisterClient tracks the connected set).
	if regErr := s.svc.RegisterClient(name); regErr != nil {
		managedLogger().Warn("managed client rejected", "name", name, "error", regErr)
		conn.Close() //nolint:errcheck,gosec // duplicate name; refuse the second connection
		return
	}
	defer s.svc.UnregisterClient(name)
	s.mConnected.Inc()
	defer s.mConnected.Dec()

	mc := rpc.NewMuxConn(rpc.NewConn(conn, conn))
	defer mc.Close() //nolint:errcheck // cleanup

	s.mu.Lock()
	s.conns[name] = mc
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, name)
		s.mu.Unlock()
	}()

	managedLogger().Info("managed client connected", "name", name)
	s.serve(name, mc)
	managedLogger().Info("managed client disconnected", "name", name)
}

// serve reads inbound RPCs from a connected client until disconnect or shutdown.
func (s *ManagedServer) serve(name string, mc *rpc.MuxConn) {
	for {
		select {
		case req, ok := <-mc.Requests():
			if !ok {
				return
			}
			s.handleRequest(name, mc, req)
		case <-mc.Done():
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *ManagedServer) handleRequest(name string, mc *rpc.MuxConn, req *rpc.Request) {
	switch req.Method {
	case fleet.VerbConfigFetch:
		var fr fleet.ConfigFetchRequest
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &fr); err != nil {
				_ = mc.SendError(s.ctx, req.ID, "bad config-fetch payload")
				return
			}
		}
		resp, err := s.svc.HandleConfigFetch(name, fr)
		if err != nil {
			s.mFetch.With("error").Inc()
			var tb textbuf.Buffer
			tb.Str("config-fetch: ").Err(err)
			_ = mc.SendError(s.ctx, req.ID, tb.String())
			return
		}
		if resp.Status == "current" {
			s.mFetch.With("current").Inc()
		} else {
			s.mFetch.With("served").Inc()
		}
		if err := mc.SendResult(s.ctx, req.ID, resp); err != nil {
			managedLogger().Debug("send config-fetch result failed", "name", name, "error", err)
		}

	case fleet.VerbConfigAck:
		var ack fleet.ConfigAck
		_ = json.Unmarshal(req.Params, &ack) //nolint:errcheck // ack is informational; log best-effort
		if ack.OK {
			managedLogger().Info("managed client applied config", "name", name, "version", ack.Version)
		} else {
			managedLogger().Warn("managed client rejected config", "name", name, "error", ack.Error)
		}
		_ = mc.SendOK(s.ctx, req.ID)

	case fleet.VerbPing:
		_ = mc.SendOK(s.ctx, req.ID)

	default:
		// Unknown methods are answered with an error, for forward compatibility.
		var tb textbuf.Buffer
		tb.Str("unknown method: ").Str(req.Method)
		_ = mc.SendError(s.ctx, req.ID, tb.String())
	}
}

// NotifyConfigChanged enqueues a config-changed push for the named client. It is
// non-blocking and safe to call from the storage write path: the round-trip runs on
// notifyWorker. On a full queue the notify is dropped (the client picks up the change
// on its next fetch/reconnect).
func (s *ManagedServer) NotifyConfigChanged(name string) {
	select {
	case s.notifyCh <- name:
	default:
		managedLogger().Warn("config-changed notify queue full; dropping", "name", name)
	}
}

// notifyWorker drains queued config-changed pushes and delivers them one at a time.
// A single long-lived goroutine started by Start.
func (s *ManagedServer) notifyWorker() {
	for {
		select {
		case name := <-s.notifyCh:
			s.pushConfigChanged(name)
		case <-s.ctx.Done():
			return
		}
	}
}

// pushConfigChanged sends a config-changed notification to the named client if it is
// currently connected. A no-op for a client that is not connected (it will fetch the
// latest config on its next connect).
func (s *ManagedServer) pushConfigChanged(name string) {
	s.mu.Lock()
	mc, ok := s.conns[name]
	s.mu.Unlock()
	if !ok {
		return
	}
	changed, err := s.svc.BuildConfigChanged(name)
	if err != nil {
		managedLogger().Warn("build config-changed failed", "name", name, "error", err)
		return
	}
	// Bound the round-trip: a client that accepts the notification but never replies
	// must not stall this worker indefinitely (mux.CallRPC returns ctx.Err() on
	// deadline). The worker pool means one stalled client does not delay others.
	pushCtx, cancel := context.WithTimeout(s.ctx, managedPushTimeout)
	defer cancel()
	if _, err := mc.CallRPC(pushCtx, fleet.VerbConfigChanged, changed); err != nil {
		managedLogger().Debug("push config-changed failed", "name", name, "error", err)
		return
	}
	s.mPushed.Inc()
}

// closeOnDone closes all listeners when the server context is canceled, unblocking
// the accept loops. A single long-lived goroutine started by Start.
func (s *ManagedServer) closeOnDone() {
	<-s.ctx.Done()
	for _, ln := range s.listeners {
		ln.Close() //nolint:errcheck,gosec // shutdown cleanup
	}
}

// connectedClients returns the names of currently connected managed clients. Internal
// (used by tests to observe connection state); the live count is exposed to operators
// via the ze_managed_clients_connected gauge.
func (s *ManagedServer) connectedClients() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.conns))
	for n := range s.conns {
		out = append(out, n)
	}
	return out
}

// Stop cancels serving and waits for all goroutines to drain. Listeners are closed by
// closeOnDone when the context is canceled.
func (s *ManagedServer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// Per-client config key convention: a managed client's config is stored on the hub
// under blob key file/active/client-<name>.conf (storage.resolveKey adds the
// file/active/ prefix). The "client-" prefix avoids collision with the hub's own
// config file. Both the ConfigReader and the config-changed write-observer use this
// single pair of derivations, so the two halves never drift.
const (
	clientConfigPrefix = "client-"
	clientConfigSuffix = ".conf"
)

// ClientConfigKey returns the blob key (relative; storage adds the file/active/
// namespace) for a managed client's config.
func ClientConfigKey(name string) string {
	var tb textbuf.Buffer
	tb.Str(clientConfigPrefix).Str(name).Str(clientConfigSuffix)
	return tb.String()
}

// ClientNameFromConfigKey extracts the managed client name from a written blob key,
// reporting false when the key is not a per-client config key. It tolerates the
// storage namespace prefix (file/active/) by matching on the key's basename, so it
// works whether the observer reports the relative or the resolved key.
func ClientNameFromConfigKey(key string) (string, bool) {
	base := key
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	if !strings.HasPrefix(base, clientConfigPrefix) || !strings.HasSuffix(base, clientConfigSuffix) {
		return "", false
	}
	name := base[len(clientConfigPrefix) : len(base)-len(clientConfigSuffix)]
	if name == "" {
		return "", false
	}
	return name, true
}
