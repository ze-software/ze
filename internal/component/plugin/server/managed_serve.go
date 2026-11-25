// Design: docs/architecture/fleet-config.md -- hub-side managed config server (dedicated listener)
// Related: managed.go -- ManagedConfigService (per-client config fetch + version hashing)
//
// ManagedServer is a dedicated TLS listener that serves managed fleet clients. It is
// independent of the plugin PluginAcceptor (which only routes engine-spawned plugin
// processes to ExpectPlugin waiters): a managed client connects inbound at any time
// and has no waiter, so it needs its own accept path. It reuses the ipc auth and rpc
// (MuxConn) transport primitives -- no new auth or wire protocol -- so "No New Server"
// (fleet-config.md) holds in spirit: only a listener bound to the managed server block.
// The client half it serves lives in internal/component/managed/client.go.

package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

var managedLogger = slogutil.LazyLogger("hub.managed-server")

const managedAuthTimeout = 10 * time.Second

// managedMaxConns bounds concurrent connection handlers so an unauthenticated
// connection flood cannot exhaust goroutines/memory (each handler must authenticate
// within managedAuthTimeout or is dropped). Mirrors the plugin acceptor's cap.
const managedMaxConns = 128

// ManagedPushTimeout bounds a single config-changed round-trip so a client that
// accepts the notification but never replies cannot stall the notify worker
// indefinitely. A healthy client replies immediately (it acks before fetching).
// A managed client bounds every round-trip that keeps its notification loop
// busy below it (the client's heldUploadTimeout and configAckTimeout), except
// its config-fetch: configFetchTimeout (30 seconds) carries a 16 MiB answer, so
// a config-changed sent while a long fetch runs can fail here; the worker then
// closes the connection, and the client fetches again when it reconnects.
const ManagedPushTimeout = 10 * time.Second

// managedNotifyWorkers is the size of the config-changed push pool. More than one so a
// single slow/stalled client (blocking up to ManagedPushTimeout) does not delay pushes
// to other clients.
const managedNotifyWorkers = 4

// ManagedListener is one hub server block that accepts managed clients: the
// address it listens on and the clients it admits there. A client is admitted
// only on the address of the block that declares it, so a secret configured
// for one listener opens no other.
type ManagedListener struct {
	Block        string                       // Configuration block, for operator errors.
	Certificate  string                       // Certificate reference on this block.
	Addr         string                       // Listen address of the server block.
	Clients      map[string]string            // Client name -> secret, admitted on Addr only.
	ClientLabels map[string]map[string]string // Label assignments by client name.
}

// ManagedServerConfig configures the dedicated managed-config TLS server.
type ManagedServerConfig struct {
	Listeners  []ManagedListener // One per server block that declares client entries.
	ReadConfig ConfigReader      // Reads a client's config by name (over the hub blob store).
	Metrics    metrics.Registry  // Optional; nil installs no-op counters.

	// Store is the hub's store, which keeps the copy each holding client
	// sends of its configuration (HeldRecord, under KeyConfigHeld). It is
	// borrowed: the caller MUST keep it open until Stop returned. nil keeps
	// no copy: every config-held is refused and no config-fetch answer names
	// a held version.
	Store storage.Storage

	// Certificate names the pki store certificate the listener serves
	// (plugin/hub/server/certificate). An empty name serves a leaf that
	// Authority issues, which a client validates against the daemon root it
	// holds in a pki ca block.
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

	// Acceptor is the hub server block the plugin acceptor binds
	// (Servers[0], hubAcceptorServer). A managed address that collides with it
	// is the collision the local hub-managed-listener check reports from the
	// config, so Start logs it under that code and does not record it as
	// unbound. The zero value names port 0, which collides with nothing.
	Acceptor plugin.HubServerConfig
}

// managedLeafCommonName names the managed listener in the subject of the leaf
// it serves when no pki certificate is configured. It is not a hostname and is
// never matched against one; the SANs are.
const managedLeafCommonName = "ze-managed-hub"

// managedLoopbackHost is where a co-located client reaches the managed
// listener, so every issued leaf carries it whatever else is bound.
const managedLoopbackHost = "127.0.0.1"

// ManagedServer authenticates managed fleet clients (per-client secret), answers
// config-fetch/config-ack/config-held/ping over MuxConn, and pushes config-changed to connected
// clients. It owns its listeners and one goroutine per connection.
type ManagedServer struct {
	svc *ManagedConfigService
	// listens is ManagedServerConfig.Listeners, each with its own copy of the
	// startup declarations. Reload takes its own prepared declarations.
	listens []ManagedListener
	// devices holds one entry across all declarations of a name. Its session
	// pointer is also the destination for config-changed. Guarded by mu.
	recordIO *managedRecordIO
	devices  map[string]*managedDevice
	now      func() time.Time

	// source is installed once the reload is accepted. Each new handshake reads it.
	source         *managedCertSource
	certConfig     ManagedServerConfig // Only the authority, clock and material resolver.
	initialCertErr error
	// acceptor is ManagedServerConfig.Acceptor.
	acceptor plugin.HubServerConfig

	// doctor is what this server records for the managed runtime checks.
	doctor managedDoctorState

	mu sync.Mutex
	// pending names the connected clients whose config changed and who have
	// not been told yet. It holds connected names only, so managedMaxConns
	// bounds it and a change is never dropped for want of room. Guarded by mu.
	pending map[string]struct{}
	// pushing names the clients a notifyWorker is pushing to right now. A
	// client in it is left in pending for later, so a client that does not
	// answer holds one worker and never the whole pool. Guarded by mu.
	pushing map[string]struct{}

	// notifyWake tells a notifyWorker that pending is not empty. One token is
	// enough: a worker that takes a name and leaves others puts the token back.
	notifyWake chan struct{}
	// pushTimeout is ManagedPushTimeout. A test shortens it.
	pushTimeout time.Duration
	sem         chan struct{} // Bounds concurrent connection handlers (managedMaxConns).

	// Metrics handles (no-op unless a registry was supplied). Set once at construction,
	// so they are read without a lock.
	mConnected    metrics.Gauge      // ze_managed_clients_connected
	mFetch        metrics.CounterVec // ze_managed_config_fetch_total{result}
	mPushed       metrics.Counter    // ze_managed_config_changed_pushed_total
	mRecordWrites metrics.CounterVec // ze_managed_device_records_written_total{result}
	mHeld         metrics.CounterVec // ze_managed_config_held_total{result}

	lifecycleMu sync.Mutex // Serializes applying socket changes with shutdown.
	listeners   []*managedBound
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// managedListenerKey identifies a configured bind request. Multiple port-zero
// requests for the same host each receive a distinct socket from the kernel.
type managedListenerKey struct {
	addr       string
	block      string
	occurrence int
}

// managedBound is one bound listener and the clients its server block admits.
type managedBound struct {
	key      managedListenerKey
	listener net.Listener
	clients  map[string]string
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
	source, certErr := managedCertificate(cfg, nil)
	if certErr != nil && (cfg.Certificate == "" && cfg.Authority == nil || cfg.Certificate != "" && cfg.TLSMaterialResolver == nil) {
		return nil, certErr
	}
	listens := cloneManagedListeners(cfg.Listeners)
	devices := make(map[string]*managedDevice)
	for _, listen := range listens {
		for name := range listen.Clients {
			devices[name] = &managedDevice{
				ManagedDeviceRecord: ManagedDeviceRecord{Name: name},
				declared:            true, labels: listen.ClientLabels[name],
			}
		}
	}
	reg := cfg.Metrics
	if reg == nil {
		reg = metrics.NopRegistry{}
	} else {
		managedPanics().guard.SetMetricsRegistry(reg)
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.RealClock{}
	}
	svc := newManagedConfigService(cfg.ReadConfig)
	svc.records = newHeldRecords(cfg.Store)
	return &ManagedServer{
		svc:            svc,
		listens:        listens,
		devices:        devices,
		recordIO:       newManagedRecordIO(cfg.Store),
		now:            cfg.Clock.Now,
		source:         source,
		initialCertErr: certErr,
		certConfig:     ManagedServerConfig{Authority: cfg.Authority, TLSMaterialResolver: cfg.TLSMaterialResolver, Clock: cfg.Clock},
		acceptor:       cfg.Acceptor,
		pending:        make(map[string]struct{}),
		pushing:        make(map[string]struct{}),
		notifyWake:     make(chan struct{}, 1),
		pushTimeout:    ManagedPushTimeout,
		sem:            make(chan struct{}, managedMaxConns),
		mConnected: reg.Gauge("ze_managed_clients_connected",
			"Number of managed fleet clients currently connected to the hub."),
		mFetch: reg.CounterVec("ze_managed_config_fetch_total",
			"Managed config-fetch requests served, by result (served|current|error).", []string{managedMetricResult}),
		mPushed: reg.Counter("ze_managed_config_changed_pushed_total",
			"config-changed notifications pushed to connected managed clients."),
		mRecordWrites: reg.CounterVec("ze_managed_device_records_written_total",
			"Managed device history writes, by result (ok|failed).", []string{managedMetricResult}),
		mHeld: reg.CounterVec("ze_managed_config_held_total",
			"Copies of a held configuration managed clients sent, by result (stored|refused).", []string{managedMetricResult}),
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
// lives keeps presenting a valid certificate. The root is presented after the
// leaf (withAuthority). No authority is an error too: a
// self-signed leaf is one no client can validate and no operator can rotate.
func managedCertificate(cfg ManagedServerConfig, current *managedCertSource) (*managedCertSource, error) {
	if cfg.Certificate != "" {
		if cfg.TLSMaterialResolver == nil {
			var tb textbuf.Buffer
			tb.Str("managed server: certificate ").Str(cfg.Certificate).
				Str(" is configured but this hub resolves no certificate names")
			return nil, errors.New(tb.String())
		}
		certPEM, keyPEM, err := cfg.TLSMaterialResolver(cfg.Certificate)
		if err != nil {
			return nil, fmt.Errorf("certificate %s is not defined in pki: %w", cfg.Certificate, err)
		}
		pair, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("certificate %s material does not parse: %w", cfg.Certificate, err)
		}
		return &managedCertSource{name: cfg.Certificate, get: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return &pair, nil }}, nil
	}

	if cfg.Authority == nil {
		return nil, errors.New("managed server: no certificate name and no certificate authority")
	}
	addrs := make([]string, 0, len(cfg.Listeners))
	for _, listen := range cfg.Listeners {
		addrs = append(addrs, listen.Addr)
	}
	hosts := managedLeafHosts(addrs)
	if current != nil && current.name == "" && !slices.ContainsFunc(hosts, func(host string) bool { return !slices.Contains(current.hosts, host) }) {
		return current, nil
	}
	leaf, err := plugin.NewServingLeaf(cfg.Authority, managedLeafCommonName, hosts, cfg.Clock)
	if err != nil {
		return nil, fmt.Errorf("managed server: %w", err)
	}
	root, _ := pem.Decode(cfg.Authority.CertificatePEM())
	if root == nil {
		return nil, errors.New("managed server: the certificate authority has no root certificate in PEM")
	}
	return &managedCertSource{hosts: hosts, get: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		issued, err := leaf.Certificate(hello)
		if err != nil {
			return nil, err
		}
		return withAuthority(issued, root.Bytes), nil
	}}, nil
}

// withAuthority returns issued with the authority's root appended to the
// chain it presents. A client that already holds the root ignores it. A
// client told to trust on first use (ze init --hub-trust-first-use) has
// nothing else to learn the root from. issued is not modified: ServingLeaf
// answers the same certificate to every handshake until it reissues.
func withAuthority(issued *tls.Certificate, root []byte) *tls.Certificate {
	presented := *issued
	presented.Certificate = make([][]byte, 0, len(issued.Certificate)+1)
	presented.Certificate = append(presented.Certificate, issued.Certificate...)
	presented.Certificate = append(presented.Certificate, root)
	return &presented
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
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]net.Addr, 0, len(s.listeners))
	for _, bound := range s.listeners {
		out = append(out, bound.listener.Addr())
	}
	return out
}

// Start loads device history and binds each available address. A server with no
// bound address still publishes its registry and can accept a repairing reload.
func (s *ManagedServer) Start(ctx context.Context) error {
	if err := s.loadDeviceRecords(); err != nil {
		return err
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.ApplyManagedListeners(&ManagedListenerPlan{listens: s.listens, source: s.source, bindErr: s.initialCertErr})
	s.listens = nil
	if s.recordIO != nil {
		s.wg.Go(s.recordWriter) // long-lived: drains device history until cancellation
	}
	for range managedNotifyWorkers {
		s.wg.Go(s.notifyWorker) // long-lived pool: drains config-changed pushes
	}
	s.wg.Go(s.closeOnDone) // long-lived: closes listeners on shutdown
	managedLogger().Info("managed config server listening",
		"listeners", len(s.Addrs()))
	if s.source != nil && s.source.name == "" {
		managedLogger().Info("managed config server: no certificate configured, so it serves a leaf issued by " +
			"this daemon's certificate authority. Export the root and name it in each client's pki ca block")
	}
	publishManagedDoctor(s)
	return nil
}

// bindFailed logs one address Start could not bind and records it for doctor.
// A collision with the plugin acceptor is the condition the local
// hub-managed-listener check reports from the config, under its own code, so
// it is logged under that code and not recorded as unbound: one fault, one
// row. Any other failure is recorded with the kernel's error text.
func (s *ManagedServer) bindFailed(addr string, err error) {
	if s.collidesWithAcceptor(addr) {
		managedLogger().Error("managed listener disabled: the plugin acceptor already binds this address",
			"code", managedCodeCollision, "addr", addr, "error", err)
		return
	}
	s.doctor.recordUnbound(addr, err)
	managedLogger().Error("managed listener disabled: address unavailable",
		"code", CodeDoctorManagedListenerUnbound, "addr", addr, "error", err)
}

// collidesWithAcceptor reports whether addr asks for the socket the plugin
// acceptor holds, judged by plugin.HubAddressesCollide, the predicate the
// local collision check uses.
func (s *ManagedServer) collidesWithAcceptor(addr string) bool {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return false // Not host:port, so not the acceptor's address; the bind error says why.
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return false // Not a port the acceptor could hold; the bind error says why.
	}
	return plugin.HubAddressesCollide(s.acceptor, plugin.HubServerConfig{Host: host, Port: uint16(port)})
}

func (s *ManagedServer) acceptLoop(bound *managedBound) {
	for {
		conn, err := bound.listener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return // The server stopped.
			}
			if errors.Is(err, net.ErrClosed) {
				return // Reload removed the block.
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
			s.handleConn(conn, bound)
		})
	}
}

// handleConn authenticates one managed client against clients, the entries of
// the server block whose listener accepted it, and serves its RPCs until it
// disconnects. A panic while it does so is contained here: the deferred
// cleanup below has already closed the connection and released the client's
// name when the boundary runs, and the server keeps serving every other
// client.
func (s *ManagedServer) handleConn(conn net.Conn, bound *managedBound) {
	defer managedPanics().client.Contain(
		func() taskguard.Context {
			return taskguard.Context{Unit: "managed client connection from " + conn.RemoteAddr().String()}
		},
		func(*taskguard.Panic) {
			conn.Close() //nolint:errcheck,gosec // closes a connection the panic hit before the MuxConn owned it
		},
	)

	authCtx, cancel := context.WithTimeout(s.ctx, managedAuthTimeout)
	defer cancel()

	// Per-client secret only: no shared-secret fallback (empty sharedSecret), so an
	// unknown name cannot connect: AuthenticateWithLookup refuses an empty
	// expected token. The secret is looked up among THIS listener's clients, so
	// a client another server block declares is unknown here. The name is taken
	// ONLY from the authenticated session, never from a later request payload
	// (config isolation: a client can only fetch its own config).
	var verifiedSecret string
	lookup := func(name string) (string, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		secret, ok := bound.clients[name]
		verifiedSecret = secret
		return secret, ok
	}
	name, err := pluginipc.AuthenticateWithLookup(authCtx, conn, "", lookup)
	if err != nil {
		s.authRefused(conn.RemoteAddr().String(), err, lookup)
		return // AuthenticateWithLookup already closed conn on failure.
	}

	// The accept loop binds TCP listeners. Record the host without its
	// ephemeral port so a later handshake from the same device can be joined.
	remote, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		s.authRefused(conn.RemoteAddr().String(), err, lookup)
		conn.Close() //nolint:errcheck,gosec // cannot identify the remote endpoint
		return
	}
	mc := rpc.NewMuxConn(rpc.NewConn(conn, conn))
	defer mc.Close() //nolint:errcheck // cleanup
	if regErr := s.admitOnListener(name, remote.Addr(), mc, bound, verifiedSecret); regErr != nil {
		if errors.Is(regErr, ErrDuplicateClient) {
			s.doctor.recordRejected(name, ManagedRejectNameConnected, regErr.Error())
			managedLogger().Warn("managed client rejected", "code", CodeDoctorManagedClientRejected,
				"name", name, "error", regErr)
		} else {
			s.authRefused(conn.RemoteAddr().String(), regErr, lookup)
		}
		return
	}
	defer s.disconnectDevice(name, mc)
	s.mConnected.Inc()
	defer s.mConnected.Dec()

	managedLogger().Info("managed client connected", "name", name)
	s.serve(name, mc)
	managedLogger().Info("managed client disconnected", "name", name)
}

// managedSession is what the hub remembers of one client connection between
// its requests. serve owns it and handles one request at a time, so it needs
// no lock, and it ends with the connection.
type managedSession struct {
	// fetched is the version the client named in its last config-fetch on
	// this connection, the configuration it runs; empty before its first
	// fetch.
	fetched string
}

// serve reads inbound RPCs from a connected client until disconnect or shutdown.
func (s *ManagedServer) serve(name string, mc *rpc.MuxConn) {
	var session managedSession
	for {
		select {
		case req, ok := <-mc.Requests():
			if !ok {
				return
			}
			s.handleRequest(name, mc, req, &session)
		case <-mc.Done():
			return
		case <-s.ctx.Done():
			return
		}
	}
}

// handleRequest answers one request of client name on mc. session is that
// connection's memory, which only serve's loop passes.
func (s *ManagedServer) handleRequest(name string, mc *rpc.MuxConn, req *rpc.Request, session *managedSession) {
	switch req.Method {
	case fleet.VerbConfigFetch:
		var fr fleet.ConfigFetchRequest
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &fr); err != nil {
				_ = mc.SendError(s.ctx, req.ID, "bad config-fetch payload")
				return
			}
		}
		session.fetched = fr.Version
		resp, err := s.svc.HandleConfigFetch(name, fr)
		s.recordFetch(name, fr.Version, resp, err)
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
		s.handleConfigAck(name, ack, session.fetched)
		_ = mc.SendOK(s.ctx, req.ID)

	case fleet.VerbConfigHeld:
		s.handleConfigHeld(name, mc, req)

	case fleet.VerbPing:
		_ = mc.SendOK(s.ctx, req.ID)

	default:
		// Unknown methods are answered with an error, for forward compatibility.
		var tb textbuf.Buffer
		tb.Str("unknown method: ").Str(req.Method)
		_ = mc.SendError(s.ctx, req.ID, tb.String())
	}
}

// handleConfigAck records what client name's config-ack says. ok:true clears
// its refusal and its held record: the client runs what the hub holds, so
// neither shows anything to resolve. A refusal is recorded as the client's
// last refusal, unless the client says it holds and the hub keeps a copy of
// what it holds: the hold is then shown from that copy, never as a refused
// configuration. A holding client whose copy the hub does not keep (too
// large, refused, or never sent) is recorded as a refusal, so the hold stays
// visible. fetched is the version the client named in its last config-fetch
// on this connection, the one it runs.
func (s *ManagedServer) handleConfigAck(name string, ack fleet.ConfigAck, fetched string) {
	if ack.OK {
		s.recordAck(name, ack, ManagedAckApplied)
		s.doctor.clearRejected(name)
		if err := s.svc.records.clear(name); err != nil {
			managedLogger().Warn("the hub could not remove the held copy of a managed client that applied its config",
				"code", diagnostic.CodeDoctorStoreIntegrity, "name", name, "error", err)
		}
		managedLogger().Info("managed client applied config", "name", name, "version", ack.Version)
		return
	}
	if s.holdShownFromCopy(name, ack, fetched) {
		s.recordAck(name, ack, ManagedAckHolding)
		s.doctor.clearRejected(name)
		managedLogger().Info("managed client holds a configuration of its own, and the hub keeps a copy of it",
			"name", name, "version", ack.Version)
		return
	}
	s.recordAck(name, ack, ManagedAckRefused)
	s.doctor.recordRejected(name, ManagedRejectConfigRejected, ack.Error)
	managedLogger().Warn("managed client rejected config", "code", CodeDoctorManagedClientRejected,
		"name", name, "error", ack.Error)
}

// holdShownFromCopy reports whether a refusing ack is a hold the hub shows
// from the copy it keeps: the client says it holds, and the record is a copy
// of fetched, the version the client named in its last config-fetch, the one
// it runs. A record of an older version is not the hold the client reports:
// its newer copy was refused or too large to send, so the hub keeps no copy
// of what it runs, and the ack is recorded as a refusal. A record that cannot
// be read is logged and answers false, so the hold is recorded as a refusal
// rather than shown nowhere.
func (s *ManagedServer) holdShownFromCopy(name string, ack fleet.ConfigAck, fetched string) bool {
	if !ack.Holding {
		return false
	}
	held, err := s.svc.records.version(name)
	if err != nil {
		managedLogger().Warn("the hub could not read the held copy of a managed client that holds",
			"code", diagnostic.CodeDoctorStoreIntegrity, "name", name, "error", err)
		return false
	}
	return held != "" && held == fetched
}

// NotifyConfigChanged marks the named client as owed a config-changed push. It
// is non-blocking and safe to call from the storage write path: the round-trip
// runs on a notifyWorker. The mark is kept until a worker takes it, so a burst
// of writes never loses one, and several writes for one client before a worker
// reaches it become one push. A client that is not connected is not marked: it
// fetches the latest config when it connects.
func (s *ManagedServer) NotifyConfigChanged(name string) {
	s.mu.Lock()
	device := s.devices[name]
	connected := device != nil && device.conn != nil
	if connected {
		s.pending[name] = struct{}{}
	}
	s.mu.Unlock()
	if connected {
		s.wakeNotifyWorker()
	}
}

// wakeNotifyWorker leaves the one wake token for a notifyWorker. A token
// already there means a worker is about to look at pending, which is enough.
func (s *ManagedServer) wakeNotifyWorker() {
	select {
	case s.notifyWake <- struct{}{}:
	default: // A wake is already waiting to be read.
	}
}

// takePending removes and returns one client owed a push that no worker is
// pushing to, and marks it as being pushed to. The caller MUST call pushDone
// with that name after the push. When others remain it puts the wake token
// back, so another worker starts on them while this one is busy with the
// client it took.
func (s *ManagedServer) takePending() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name := range s.pending {
		if _, busy := s.pushing[name]; busy {
			continue // pushDone wakes a worker for it once that push ends.
		}
		delete(s.pending, name)
		s.pushing[name] = struct{}{}
		if len(s.pending) > 0 {
			s.wakeNotifyWorker()
		}
		return name, true
	}
	return "", false
}

// pushDone ends the push takePending started for name. It MUST be called
// after takePending returned that name. A change written while the push was
// in flight left the client in pending, so a worker is woken for what remains.
func (s *ManagedServer) pushDone(name string) {
	s.mu.Lock()
	delete(s.pushing, name)
	remaining := len(s.pending)
	s.mu.Unlock()
	if remaining > 0 {
		s.wakeNotifyWorker()
	}
}

// notifyWorker delivers the pushes owed to connected clients, one at a time.
// A long-lived goroutine started by Start; Stop ends it through the context.
func (s *ManagedServer) notifyWorker() {
	for {
		select {
		case <-s.notifyWake:
			s.pushPending()
		case <-s.ctx.Done():
			return
		}
	}
}

// pushPending pushes to every client owed a push that no other worker is
// busy with, until none is left. The loop ends because each turn removes one
// name from pending, which holds connected clients only.
func (s *ManagedServer) pushPending() {
	for {
		name, owed := s.takePending()
		if !owed {
			return
		}
		s.pushConfigChanged(name)
		s.pushDone(name)
	}
}

// pushConfigChanged sends a config-changed notification to the named client if it is
// currently connected. A no-op for a client that is not connected (it will fetch the
// latest config on its next connect).
//
// A connected client that does not accept the notification is disconnected. It
// would otherwise stay connected on its previous config with nothing left to
// tell it: closing the connection makes it reconnect, and a client fetches its
// config each time it connects.
func (s *ManagedServer) pushConfigChanged(name string) {
	s.mu.Lock()
	device := s.devices[name]
	var mc *rpc.MuxConn
	if device != nil {
		mc = device.conn
	}
	s.mu.Unlock()
	if mc == nil {
		return // A disconnected client will fetch when it reconnects.
	}
	changed, err := s.svc.BuildConfigChanged(name)
	if err != nil {
		var cause textbuf.Buffer
		cause.Str("building the notification failed: ").Err(err)
		notifyDropped(name, cause.String())
		return
	}
	// Bound the round-trip: a client that accepts the notification but never replies
	// must not stall this worker indefinitely (mux.CallRPC returns ctx.Err() on
	// deadline). The worker pool means one stalled client does not delay others.
	pushCtx, cancel := context.WithTimeout(s.ctx, s.pushTimeout)
	defer cancel()
	_, err = mc.CallRPC(pushCtx, fleet.VerbConfigChanged, changed)
	if err == nil {
		s.mPushed.Inc()
		return
	}
	if s.ctx.Err() != nil {
		return // The server is stopping, which is not a fault of this client.
	}
	var cause textbuf.Buffer
	cause.Str("the push failed: ").Err(err)
	closeErr := mc.Close()
	if closeErr == nil {
		cause.Str("; the hub closed its connection, so it fetches the config when it reconnects")
	} else {
		cause.Str("; the hub could not close its connection: ").Err(closeErr)
	}
	notifyDropped(name, cause.String())
}

// closeOnDone closes all listeners when the server context is canceled, unblocking
// the accept loops. A single long-lived goroutine started by Start.
func (s *ManagedServer) closeOnDone() {
	<-s.ctx.Done()
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	for _, bound := range s.listeners {
		bound.listener.Close() //nolint:errcheck,gosec // shutdown cleanup
	}
}

// connectedClients returns the names of currently connected managed clients. Internal
// (used by tests to observe connection state); the live count is exposed to operators
// via the ze_managed_clients_connected gauge.
func (s *ManagedServer) connectedClients() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for name, device := range s.devices {
		if device.conn != nil {
			out = append(out, name)
		}
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
	s.stopDeviceRecords() // final disconnects may follow writer cancellation
	withdrawManagedDoctor(s)
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

// Serves reports whether name is a managed client this server accepts, which is
// a client entry in one of the hub's serving blocks. The set follows the last accepted configuration. Safe for concurrent use.
func (s *ManagedServer) Serves(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.devices[name]
	return device != nil && device.declared
}

// ClientConfigKey returns the blob key (relative; storage adds the file/active/
// namespace) for a managed client's config.
func ClientConfigKey(name string) string {
	var tb textbuf.Buffer
	tb.Str(clientConfigPrefix).Str(name).Str(clientConfigSuffix)
	return tb.String()
}

// ClientNameFromConfigKey extracts the managed client name from a written blob key,
// reported as a relative name or a resolved active configuration key. A value
// under another namespace is never a change to the client's active file.
func ClientNameFromConfigKey(key string) (string, bool) {
	base := strings.TrimPrefix(key, zefs.KeyFileActive.Prefix())
	if strings.ContainsRune(base, '/') {
		return "", false
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

// managedMetricResult is the label of the managed counters that count by outcome.
const managedMetricResult = "result"
