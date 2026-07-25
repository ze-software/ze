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

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
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
}

// ManagedServer authenticates managed fleet clients (per-client secret), answers
// config-fetch/config-ack/ping over MuxConn, and pushes config-changed to connected
// clients. It owns its listeners and one goroutine per connection.
type ManagedServer struct {
	svc    *ManagedConfigService
	lookup func(name string) (string, bool)
	addrs  []string
	cert   tls.Certificate

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

// NewManagedServer builds a managed server. ReadConfig is required. A self-signed
// TLS certificate is generated: the transport is encrypted, but a remote managed
// client cannot verify a self-signed cert against a CA, so today it must connect with
// tls-insecure. Verifiable server-cert distribution (CA cert or pinned fingerprint in
// the client config) is tracked in plan/spec-managed-server-hardening.md.
func NewManagedServer(cfg ManagedServerConfig) (*ManagedServer, error) {
	if cfg.ReadConfig == nil {
		return nil, errors.New("managed server: ReadConfig is required")
	}
	cert, err := pluginipc.GenerateSelfSignedCert()
	if err != nil {
		return nil, fmt.Errorf("managed server: generate cert: %w", err)
	}
	secrets := cfg.ClientSecrets
	reg := cfg.Metrics
	if reg == nil {
		reg = metrics.NopRegistry{}
	}
	return &ManagedServer{
		svc: NewManagedConfigService(cfg.ReadConfig),
		lookup: func(name string) (string, bool) {
			s, ok := secrets[name]
			return s, ok
		},
		addrs:    cfg.Addrs,
		cert:     cert,
		conns:    make(map[string]*rpc.MuxConn),
		notifyCh: make(chan string, managedNotifyBuffer),
		sem:      make(chan struct{}, managedMaxConns),
		mConnected: reg.Gauge("ze_managed_clients_connected",
			"Number of managed fleet clients currently connected to the hub."),
		mFetch: reg.CounterVec("ze_managed_config_fetch_total",
			"Managed config-fetch requests served, by result (served|current|error).", []string{"result"}),
		mPushed: reg.Counter("ze_managed_config_changed_pushed_total",
			"config-changed notifications pushed to connected managed clients."),
	}, nil
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
		lns, err := pluginipc.StartListeners([]string{addr}, s.cert)
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
	managedLogger().Info("managed config server listening", "listeners", len(s.listeners))
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
