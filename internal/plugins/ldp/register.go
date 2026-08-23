// Design: docs/architecture/ldp/mpls-ldp.md -- LDP component registration
// Related: wire.go -- LDP message codec
// Related: discovery.go -- adjacency table
// Related: session.go -- TCP session FSM
// Related: lib.go -- Label Information Base
// Related: events.go -- event bus handles
//
// Package ldp implements the Label Distribution Protocol (RFC 5036).
// LDP distributes MPLS labels between label-switching routers for
// FEC-to-label bindings using downstream unsolicited mode.
package ldp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/slogutil"
	ldpyang "github.com/ze-software/ze/internal/plugins/ldp/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	loggerPtr   atomic.Pointer[slog.Logger]
	eventBusPtr atomic.Pointer[ze.EventBus]
)

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

type ldpMetrics struct {
	sessionsActive metrics.Gauge
	sessionsTotal  metrics.Counter
	bindingsActive metrics.Gauge
	bindingsTotal  metrics.Counter
}

var ldpMetricsPtr atomic.Pointer[ldpMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	m := &ldpMetrics{
		sessionsActive: reg.Gauge("ze_ldp_sessions_active", "Current number of active LDP sessions."),
		sessionsTotal:  reg.Counter("ze_ldp_sessions_total", "Total LDP sessions established."),
		bindingsActive: reg.Gauge("ze_ldp_bindings_active", "Current number of label bindings in LIB."),
		bindingsTotal:  reg.Counter("ze_ldp_bindings_total", "Total label bindings received."),
	}
	ldpMetricsPtr.Store(m)
}

type ldpConfig struct {
	LSRID         netip.Addr
	HelloInterval time.Duration
	HelloHoldTime time.Duration
	KeepaliveTime time.Duration
	Interfaces    []string
	TransportAddr netip.Addr
}

// configNumber coerces a config-tree scalar to a float64. Tree.ToMap renders
// every YANG leaf as a JSON string (e.g. "5"), so a numeric leaf arrives as a
// string here, not a JSON number. We accept both for robustness.
func configNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func parseLDPConfig(sections []sdk.ConfigSection) (ldpConfig, error) {
	cfg := ldpConfig{
		HelloInterval: DefaultHelloInterval,
		HelloHoldTime: DefaultHelloHoldTime,
		KeepaliveTime: DefaultKeepaliveTime,
	}
	for _, sec := range sections {
		if sec.Root != "ldp" || sec.Data == "" {
			continue
		}
		// BuildPluginConfigSections wraps the subtree under its root key, so the
		// delivered JSON is {"ldp": {...}}. Unwrap before reading leaves.
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(sec.Data), &wrapper); err != nil {
			return cfg, fmt.Errorf("ldp: invalid config JSON: %w", err)
		}
		tree, _ := wrapper["ldp"].(map[string]any)
		if tree == nil {
			continue
		}
		if v, ok := tree["lsr-id"].(string); ok {
			addr, err := netip.ParseAddr(v)
			if err != nil {
				return cfg, fmt.Errorf("ldp: invalid lsr-id: %w", err)
			}
			cfg.LSRID = addr
		}
		if v, ok := tree["transport-address"].(string); ok {
			addr, err := netip.ParseAddr(v)
			if err != nil {
				return cfg, fmt.Errorf("ldp: invalid transport-address: %w", err)
			}
			cfg.TransportAddr = addr
		}
		if v, ok := configNumber(tree["hello-interval"]); ok && v > 0 {
			cfg.HelloInterval = time.Duration(v) * time.Second
		}
		if v, ok := configNumber(tree["hello-hold-time"]); ok && v > 0 {
			cfg.HelloHoldTime = time.Duration(v) * time.Second
		}
		if v, ok := configNumber(tree["keepalive-time"]); ok && v > 0 {
			cfg.KeepaliveTime = time.Duration(v) * time.Second
		}
		cfg.Interfaces = append(cfg.Interfaces, configvalue.LeafList(tree["interfaces"])...)
	}
	return cfg, nil
}

func registerLDP() {
	_ = events.RegisterNamespace(Namespace, EventSessionUp, EventSessionDown, EventLabelBind)

	reg := registry.Registration{
		Name:         "ldp",
		Description:  "Label Distribution Protocol (RFC 5036): MPLS label distribution",
		Features:     "yang",
		YANG:         ldpyang.ZeLDPConfYANG,
		ConfigRoots:  []string{"ldp"},
		Dependencies: []string{"fib-kernel", "sysctl"},
		RunEngine:    runLDPEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "ldp-port",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        720,
			Dependencies: []string{"fib-kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-ldp-port-unavailable"},
			Check:        checkLDPPort,
		}},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ldp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func init() { registerLDP() }

func runLDPEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ldp engine starting")

	p := sdk.NewWithConn("ldp", conn)
	defer func() { _ = p.Close() }()

	lib := newLIB()
	adjTable := newAdjacencyTable()

	var activeCfg ldpConfig
	var pendingCfg ldpConfig
	var havePending bool
	var sessionsMu sync.Mutex
	sessions := make(map[string]*Session)
	var fib *ldpFIB
	var discoveryMgr *discoveryManager
	var mgrMu sync.Mutex // guards activeCfg/pendingCfg/havePending and discoveryMgr

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseLDPConfig(sections)
		if err != nil {
			return err
		}
		// Stash the validated config so OnConfigApply (the reload-commit step) can
		// reconcile discovery to it. OnConfigure is the startup-only delivery.
		mgrMu.Lock()
		pendingCfg = cfg
		havePending = true
		mgrMu.Unlock()
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseLDPConfig(sections)
		if err != nil {
			return err
		}
		mgrMu.Lock()
		activeCfg = cfg
		mgrMu.Unlock()
		return nil
	})

	// OnConfigApply is the reload-pipeline commit step (OnConfigure does not fire
	// on reload). Adopt the verified pending config and reconcile discovery so an
	// added/removed LDP interface starts/stops without restarting the engine (AC-9).
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		mgrMu.Lock()
		if havePending {
			activeCfg = pendingCfg
			havePending = false
		}
		cfg := activeCfg
		mgr := discoveryMgr
		mgrMu.Unlock()
		if mgr != nil {
			mgr.reconcile(cfg)
		}
		return nil
	})

	p.OnStarted(func(ctx context.Context) error {
		mgrMu.Lock()
		cfg := activeCfg
		mgrMu.Unlock()
		if !cfg.LSRID.IsValid() {
			log.Warn("ldp: no lsr-id configured, engine idle")
			return nil
		}

		var lsrID [4]byte
		if cfg.LSRID.Is4() {
			lsrID = cfg.LSRID.As4()
		}

		fib = newLDPFIB(getEventBus(), log)

		// AC-3: originate label bindings for the FECs this LSR is egress for
		// (its LSR-ID and connected prefixes). Allocate a stable local label per
		// FEC and program its egress pop now; the bindings are advertised to each
		// session once it reaches the operational state.
		for _, fec := range localFECs(cfg.LSRID, connectedPrefixes(cfg.Interfaces, log)) {
			lb := lib.EnsureLocal(fec)
			fib.ProgramPop(fec, lb.Label)
		}

		startFn := func(ifctx context.Context, ifName string, c ldpConfig) {
			localTransport := cfg.TransportAddr
			discoverOnInterface(ifctx, log, c, lsrID, ifName, adjTable, func(adj *Adjacency) {
				startSessionForAdj(ctx, log, adj, lsrID, localTransport, lib, sessions, &sessionsMu, fib)
			})
		}
		mgr := newDiscoveryManager(ctx, log, startFn)
		mgrMu.Lock()
		discoveryMgr = mgr
		mgrMu.Unlock()
		mgr.reconcile(cfg)

		go runAdjacencyExpiry(ctx, log, adjTable, sessions, &sessionsMu)

		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		switch command {
		case "show ldp neighbor":
			return "done", showNeighbors(adjTable, sessions, &sessionsMu), nil
		case "show ldp binding":
			return "done", showBindings(lib), nil
		default:
			return "error", "", fmt.Errorf("unknown command: %s", command)
		}
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"ldp"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "show ldp neighbor"},
			{Name: "show ldp binding"},
		},
	})
	if err != nil {
		log.Error("ldp engine failed", "error", err)
		return 1
	}

	sessionsMu.Lock()
	for _, sess := range sessions {
		sess.Stop()
	}
	sessionsMu.Unlock()

	// Withdraw the egress pop entries this LSR programmed for its local FECs so
	// fib-kernel does not retain stale AF_MPLS state after the engine exits.
	if fib != nil {
		for _, lb := range lib.localBindings() {
			fib.RemovePop(lb.FEC, lb.Label)
		}
	}

	return 0
}

// connectedPrefixes collects the advertisable connected prefixes across the
// configured LDP interfaces, logging (but not failing on) interfaces it cannot read.
func connectedPrefixes(ifaces []string, log *slog.Logger) []netip.Prefix {
	var out []netip.Prefix
	for _, name := range ifaces {
		prefixes, err := interfacePrefixes(name)
		if err != nil {
			log.Warn("ldp: cannot read interface addresses", "interface", name, "error", err)
			continue
		}
		out = append(out, prefixes...)
	}
	return out
}

func emitSessionEvent(eb ze.EventBus, log *slog.Logger, handle *events.Event[*SessionEvent], evt *SessionEvent) {
	if eb == nil {
		return
	}
	if _, err := handle.Emit(eb, evt); err != nil {
		log.Warn("ldp: event emit failed", "event", handle.EventType(), "error", err)
	}
}

func emitLabelEvent(eb ze.EventBus, log *slog.Logger, evt *labelBindEvent) {
	if eb == nil {
		return
	}
	if _, err := LabelBind.Emit(eb, evt); err != nil {
		log.Warn("ldp: label-bind emit failed", "error", err)
	}
}

// ldpSessionDialer builds the TCP dialer for an outbound LDP session. When the
// local transport address is configured (valid), it is bound as the source so
// the session originates from the address advertised in this LSR's LDP Hellos
// (RFC 5036: the transport address determines the TCP session). When unset, the
// OS selects the source (unchanged behavior).
func ldpSessionDialer(localTransport netip.Addr) *network.RealDialer {
	dialer := &network.RealDialer{Timeout: 5 * time.Second}
	if localTransport.IsValid() {
		dialer.LocalAddr = &net.TCPAddr{IP: localTransport.AsSlice()}
	}
	return dialer
}

func startSessionForAdj(ctx context.Context, log *slog.Logger, adj *Adjacency, lsrID [4]byte, localTransport netip.Addr, lib *LIB, sessions map[string]*Session, sessionsMu *sync.Mutex, fib *ldpFIB) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	key := AdjacencyKey(adj.PeerLSRID, adj.PeerLabelSpace)
	if _, exists := sessions[key]; exists {
		return
	}

	tcpAddr := net.TCPAddr{
		IP:   adj.TransportAddr.AsSlice(),
		Port: ldpSessionPort,
	}
	dialer := ldpSessionDialer(localTransport)
	tcpConn, err := dialer.DialContext(ctx, "tcp", tcpAddr.String())
	if err != nil {
		log.Warn("ldp: TCP connect failed", "peer", adj.TransportAddr, "error", err)
		return
	}

	sess := NewSession(tcpConn, lsrID, 0, adj.PeerLSRID, adj.PeerLabelSpace, adj.TransportAddr, lib, log)
	sessions[key] = sess

	if m := ldpMetricsPtr.Load(); m != nil {
		m.sessionsTotal.Inc()
		m.sessionsActive.Set(float64(len(sessions)))
	}

	emitSessionEvent(getEventBus(), log, SessionUp, &SessionEvent{
		PeerAddress:   adj.TransportAddr.String(),
		LDPIdentifier: key,
		SessionState:  StateOperational.String(),
		Interface:     adj.Interface,
	})

	peerAddr := adj.TransportAddr
	ifName := adj.Interface
	go runSession(ctx, log, sess, lib, key, fib, func() {
		sessionsMu.Lock()
		delete(sessions, key)
		if m := ldpMetricsPtr.Load(); m != nil {
			m.sessionsActive.Set(float64(len(sessions)))
		}
		sessionsMu.Unlock()

		// Remove the peer's bindings and reconcile each affected FEC (re-point to a
		// surviving peer, or withdraw the push). reconcilePeerDown takes the reconcile
		// lock per FEC so a large peer's teardown does not stall other sessions.
		removed := reconcilePeerDown(fib, lib, key, log)
		eb := getEventBus()
		emitSessionEvent(eb, log, SessionDown, &SessionEvent{
			PeerAddress:   peerAddr.String(),
			LDPIdentifier: key,
			SessionState:  StateNonExistent.String(),
			Interface:     ifName,
		})
		for _, b := range removed {
			emitLabelEvent(eb, log, &labelBindEvent{
				FEC:      b.FEC.String(),
				Label:    b.Label,
				PeerAddr: b.PeerAddr.String(),
				Action:   "withdraw",
			})
		}
	})
}

// ldpInterfaceRetry is how often discovery re-checks for a configured interface
// that is not yet present.
const ldpInterfaceRetry = 5 * time.Second

// waitForInterface resolves the logical name ifName, retrying until it appears
// or ctx is canceled (returns nil on cancellation). It translates the logical
// name to its kernel device through the shared iface resolver (honoring the
// os-name / mac-match selectors), then fetches the real *net.Interface the
// stdlib multicast calls need. iface.Subscribe wakes it the moment the device
// appears, so LDP recovers promptly when a configured interface comes up after
// the engine starts; the retry timer is a bootstrap fallback in case an event
// is missed (the resolver drops events on a full channel). The "not available"
// warning is logged once, then demoted to debug, so a permanently-misconfigured
// interface name does not spam the log every retry.
func waitForInterface(ctx context.Context, log *slog.Logger, ifName string, retry time.Duration) *net.Interface {
	events, cancelSub := iface.Subscribe(ifName)
	defer cancelSub()
	warned := false
	for {
		if b, err := iface.Resolve(ifName); err == nil {
			if ifi, ierr := net.InterfaceByName(b.OsName); ierr == nil {
				return ifi
			}
		}
		if !warned {
			log.Warn("ldp: discovery interface not available, retrying", "interface", ifName, "retry", retry)
			warned = true
		} else {
			log.Debug("ldp: discovery interface still not available", "interface", ifName)
		}
		timer := time.NewTimer(retry)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-events:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// listenDiscovery opens the per-interface multicast UDP socket for Basic Discovery.
// It is a package var so tests can substitute a loopback socket and drive
// discoverOnInterface end-to-end without multicast joins or the privileged port 646.
var listenDiscovery = func(ifi *net.Interface, addr *net.UDPAddr) (*net.UDPConn, error) {
	return net.ListenMulticastUDP("udp4", ifi, addr)
}

// discoverOnInterface sends and receives multicast Hellos on a single interface
// (ifName ""), the system-assigned multicast interface).
func discoverOnInterface(ctx context.Context, log *slog.Logger, cfg ldpConfig, lsrID [4]byte, ifName string, adjTable *AdjacencyTable, onNewAdj func(*Adjacency)) {
	multicastAddr := &net.UDPAddr{
		IP:   net.IPv4(224, 0, 0, 2),
		Port: ldpHelloPort,
	}

	var ifi *net.Interface
	if ifName != "" {
		// An LDP interface may not exist (or be up) yet at startup, and can flap;
		// retry until it appears rather than abandoning discovery for its lifetime.
		ifi = waitForInterface(ctx, log, ifName, ldpInterfaceRetry)
		if ifi == nil {
			return // context canceled before the interface appeared
		}
	}

	udpConn, err := listenDiscovery(ifi, multicastAddr)
	if err != nil {
		log.Error("ldp: failed to listen on multicast", "interface", ifName, "error", err)
		return
	}
	// closeConn is idempotent: the ctx-cancel path closes the socket to unblock the
	// reader promptly, and this defer is a harmless backstop for every other path.
	var closeOnce sync.Once
	closeConn := func() {
		closeOnce.Do(func() {
			if err := udpConn.Close(); err != nil {
				log.Debug("ldp: UDP close error", "error", err)
			}
		})
	}
	defer closeConn()

	// Pin multicast egress to this link and use TTL 1 (RFC 5036 Section 2.4.1:
	// Basic Discovery Hellos are link-scoped), so Hellos leave on the LDP
	// interface rather than the system-default multicast interface.
	if ifi != nil {
		pc := ipv4.NewPacketConn(udpConn)
		if err := pc.SetMulticastInterface(ifi); err != nil {
			log.Warn("ldp: cannot set multicast egress interface", "interface", ifName, "error", err)
		}
		if err := pc.SetMulticastTTL(1); err != nil {
			log.Debug("ldp: cannot set multicast TTL", "interface", ifName, "error", err)
		}
	}

	helloTicker := time.NewTicker(cfg.HelloInterval)
	defer helloTicker.Stop()

	sendHello(udpConn, multicastAddr, lsrID, cfg, log)

	// Drain inbound Hellos in a dedicated reader goroutine, decoupled from the
	// send tick. Previously ReadFromUDP was gated behind the helloTicker select,
	// so the socket was drained only once per HelloInterval (5s) and one datagram
	// at a time: on a shared segment with N neighbors, N-1 Hellos per interval
	// were dropped, hold timers expired, and adjacencies flapped. The reader now
	// loops continuously on ReadFromUDP so every neighbor's Hello is consumed.
	// Models the ISIS readLoop (internal/plugins/isis/transport/backend_linux.go:196):
	// a receiver runs its own loop, uses its own buffer, and exits on socket
	// close / ctx cancel. sendHello stays on helloTicker below (net.UDPConn is
	// safe for one concurrent Read and Write), and AdjacencyTable is RWMutex-
	// guarded so processDiscoveryPacket -> Update races safely with the expiry sweep.
	readerDone := make(chan struct{})
	go readDiscoveryLoop(ctx, udpConn, lsrID, ifName, adjTable, onNewAdj, log, readerDone)

	for {
		select {
		case <-ctx.Done():
			// Close the socket to unblock a blocked ReadFromUDP immediately, then
			// wait for the reader to exit so neither the goroutine nor the socket
			// leaks across a config reload / shutdown.
			closeConn()
			<-readerDone
			return
		case <-helloTicker.C:
			sendHello(udpConn, multicastAddr, lsrID, cfg, log)
		}
	}
}

// readDiscoveryLoop continuously reads inbound Basic Discovery Hellos on udpConn
// and feeds each datagram to processDiscoveryPacket, decoupled from the Hello send
// cadence. It exits when udpConn is closed or ctx is canceled, closing done on the
// way out so discoverOnInterface can join it. A 1s read deadline is a backstop so
// a missed socket close still wakes the loop to re-check ctx (spec A-3).
func readDiscoveryLoop(ctx context.Context, udpConn *net.UDPConn, lsrID [4]byte, ifName string, adjTable *AdjacencyTable, onNewAdj func(*Adjacency), log *slog.Logger, done chan<- struct{}) {
	defer close(done)
	recvBuf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return
		}
		if err := udpConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return // socket closed on shutdown / reload
		}

		n, _, readErr := udpConn.ReadFromUDP(recvBuf)
		if readErr != nil {
			if errors.Is(readErr, net.ErrClosed) {
				return // socket closed on shutdown / reload
			}
			var ne net.Error
			if errors.As(readErr, &ne) && ne.Timeout() {
				continue // deadline backstop: re-check ctx and read again
			}
			log.Warn("ldp: UDP read error", "error", readErr)
			continue
		}

		processDiscoveryPacket(recvBuf[:n], lsrID, ifName, adjTable, onNewAdj, log)
	}
}

// processDiscoveryPacket decodes one received Hello and creates/refreshes the
// adjacency, tagging it with the local interface it arrived on (ifName) so the
// emitted SessionEvent carries the discovering interface for LDP-IGP sync
// consumers (RFC 5443 / RFC 6138). ifName is set inside AdjacencyTable.Update
// under the table lock, so a concurrent All()/Get() snapshot never sees a torn write.
func processDiscoveryPacket(data []byte, localLSRID [4]byte, ifName string, adjTable *AdjacencyTable, onNewAdj func(*Adjacency), log *slog.Logger) {
	if len(data) < ldpHeaderLen+ldpMsgHdrLen {
		return
	}

	pdu, err := decodePDUHeader(data)
	if err != nil {
		log.Debug("ldp: invalid PDU header", "error", err)
		return
	}

	if pdu.LSRID == localLSRID {
		return
	}

	bodyStart := ldpHeaderLen
	if bodyStart >= len(data) {
		return
	}

	msgHdr, err := decodeMessageHeader(data[bodyStart:])
	if err != nil {
		return
	}

	if msgHdr.Type != MsgTypeHello {
		return
	}

	msgBodyStart := bodyStart + ldpMsgHdrLen
	msgBodyEnd := bodyStart + ldpTLVHdrLen + int(msgHdr.Length)
	if msgBodyEnd > len(data) {
		return
	}

	hello, err := DecodeHello(msgHdr.MessageID, data[msgBodyStart:msgBodyEnd])
	if err != nil {
		log.Debug("ldp: invalid Hello", "error", err)
		return
	}

	adj, isNew := adjTable.Update(pdu, hello, ifName)
	if isNew && onNewAdj != nil {
		onNewAdj(adj)
	}
}

func sendHello(conn *net.UDPConn, dest *net.UDPAddr, lsrID [4]byte, cfg ldpConfig, log *slog.Logger) {
	var buf [128]byte
	bodyLen := EncodeHello(buf[ldpHeaderLen:], HelloMessage{
		MessageID:     1,
		HoldTime:      uint16(cfg.HelloHoldTime.Seconds()),
		TransportAddr: cfg.TransportAddr,
	})
	pduLen := uint16(bodyLen + 6)
	encodePDUHeader(buf[:], PDUHeader{
		Version:    ldpVersion,
		PDULength:  pduLen,
		LSRID:      lsrID,
		LabelSpace: 0,
	})
	if _, err := conn.WriteToUDP(buf[:ldpHeaderLen+bodyLen], dest); err != nil {
		log.Debug("ldp: hello send failed", "error", err)
	}
}

func runSession(ctx context.Context, log *slog.Logger, sess *Session, lib *LIB, peerKey string, fib *ldpFIB, onDone func()) {
	defer onDone()
	defer sess.Stop()

	if err := sess.SendInit(); err != nil {
		log.Warn("ldp: init send failed", "peer", sess.PeerAddr(), "error", err)
		return
	}

	if err := sess.SendKeepalive(); err != nil {
		log.Warn("ldp: keepalive send failed", "peer", sess.PeerAddr(), "error", err)
		return
	}

	kaCtx, kaCancel := context.WithCancel(ctx)
	defer kaCancel()
	go func() {
		// Re-read the keepalive each cycle: handleInit lowers it during the
		// Initialization exchange, so a fixed ticker sized from the pre-negotiation
		// default would send too slowly (RFC 5036 Section 2.5.3).
		for {
			period := sess.currentKeepalive() / 3
			if period <= 0 {
				period = time.Second
			}
			timer := time.NewTimer(period)
			select {
			case <-kaCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
				if err := sess.SendKeepalive(); err != nil {
					return
				}
			}
		}
	}()

	eb := getEventBus()

	// Connected prefixes change rarely; read them once per session (not per label
	// mapping) so resolving next hops for a peer's full FEC set does not issue a
	// netlink/InterfaceAddrs syscall per binding.
	localPrefixes := allConnectedPrefixes()

	err := sess.ReadLoop(
		func(lm labelMappingMessage, _ [4]byte) {
			// AC-4: resolve the data-plane next hop from the peer's advertised
			// interface addresses (Address message) when a label is imposed;
			// implicit-null forwards as plain IP and needs no next hop.
			var nextHop netip.Addr
			if lm.Label != ImplicitNull {
				nextHop = pickNextHop(sess.PeerAddr(), sess.peerAddresses(), localPrefixes)
			}
			// Record the binding and reconcile the FEC's forwarding atomically so a
			// concurrent update for the same FEC cannot interleave (reconcileFEC
			// selects the best binding, so this also picks the active peer).
			fib.withReconcileLock(func() {
				lib.AddRemote(lm.FEC.Prefix, lm.Label, peerKey, sess.PeerAddr(), nextHop)
				reconcileFEC(fib, lib, lm.FEC.Prefix, log)
			})
			if m := ldpMetricsPtr.Load(); m != nil {
				m.bindingsTotal.Inc()
				m.bindingsActive.Set(float64(lib.Len()))
			}
			emitLabelEvent(eb, log, &labelBindEvent{
				FEC:      lm.FEC.Prefix.String(),
				Label:    lm.Label,
				PeerAddr: sess.PeerAddr().String(),
				Action:   "add",
			})
		},
		func(lw labelWithdrawMessage, _ [4]byte) {
			// AC-5: drop the LIB entry and reconcile the kernel forwarding for the
			// FEC (re-point to a surviving peer, or withdraw the push) atomically.
			fib.withReconcileLock(func() {
				withdrawRemoteBinding(fib, lib, lw.FEC.Prefix, peerKey, log)
			})
			if m := ldpMetricsPtr.Load(); m != nil {
				m.bindingsActive.Set(float64(lib.Len()))
			}
			emitLabelEvent(eb, log, &labelBindEvent{
				FEC:      lw.FEC.Prefix.String(),
				Label:    lw.Label,
				PeerAddr: sess.PeerAddr().String(),
				Action:   "withdraw",
			})
		},
		func() {
			// AC-3: session reached operational -- advertise our local FEC
			// bindings downstream-unsolicited (RFC 5036 Section 2.3).
			locals := lib.localBindings()
			for _, lb := range locals {
				if err := sess.SendLabelMapping(lb.FEC, lb.Label); err != nil {
					log.Warn("ldp: local label mapping send failed", "fec", lb.FEC, "error", err)
					return
				}
				emitLabelEvent(eb, log, &labelBindEvent{
					FEC:      lb.FEC.String(),
					Label:    lb.Label,
					PeerAddr: sess.PeerAddr().String(),
					Action:   "advertise",
				})
			}
			if len(locals) > 0 {
				log.Debug("ldp: advertised local label mappings", "peer", sess.PeerAddr(), "count", len(locals))
			}
		},
	)
	if err != nil {
		log.Info("ldp: session ended", "peer", sess.PeerAddr(), "reason", err)
	}
}

func runAdjacencyExpiry(ctx context.Context, log *slog.Logger, adjTable *AdjacencyTable, sessions map[string]*Session, sessionsMu *sync.Mutex) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expireAdjacencies(log, adjTable, sessions, sessionsMu)
		}
	}
}

// expireAdjacencies sweeps timed-out adjacencies and tears the session for each.
// Stopping a session closes its TCP connection, so runSession exits and withdraws
// the peer's labels via reconcilePeerDown (F6). Without this the session would
// linger until its own keepalive timeout, leaving stale labels and FIB state after
// an interface is removed (discovery stops -> Hellos stop -> adjacency expires
// here). The adjacency-table key and the session-map key are both
// AdjacencyKey(LSR-ID, label-space).
func expireAdjacencies(log *slog.Logger, adjTable *AdjacencyTable, sessions map[string]*Session, sessionsMu *sync.Mutex) {
	for _, key := range adjTable.ExpireSweep() {
		log.Info("ldp: adjacency expired", "peer", key)
		sessionsMu.Lock()
		sess, ok := sessions[key]
		sessionsMu.Unlock()
		if ok {
			sess.Stop()
		}
	}
}

func showNeighbors(adjTable *AdjacencyTable, sessions map[string]*Session, mu *sync.Mutex) any {
	adjs := adjTable.All()
	mu.Lock()
	defer mu.Unlock()

	type neighborInfo struct {
		LSRID         string `json:"lsr-id"`
		TransportAddr string `json:"transport-address"`
		State         string `json:"state"`
		HoldTime      int    `json:"hold-time"`
	}

	neighbors := make([]neighborInfo, 0, len(adjs))
	for key, adj := range adjs {
		state := "discovered"
		if sess, ok := sessions[key]; ok {
			state = sess.State().String()
		}
		lsrID := adj.PeerLSRID
		neighbors = append(neighbors, neighborInfo{
			LSRID:         netip.AddrFrom4(lsrID).String(),
			TransportAddr: adj.TransportAddr.String(),
			State:         state,
			HoldTime:      int(adj.HoldTime.Seconds()),
		})
	}

	return neighbors
}

func showBindings(lib *LIB) any {
	// AC-8: report both directions -- the labels this LSR originates (local) and
	// the labels learned from peers (remote). Local bindings have no peer address.
	type bindingInfo struct {
		FEC       string `json:"fec"`
		Label     uint32 `json:"label"`
		Direction string `json:"direction"`
		PeerAddr  string `json:"peer-address,omitempty"`
	}

	locals := lib.localBindings()
	remotes := lib.allBindings()
	out := make([]bindingInfo, 0, len(locals)+len(remotes))
	for _, b := range locals {
		out = append(out, bindingInfo{
			FEC:       b.FEC.String(),
			Label:     b.Label,
			Direction: "local",
		})
	}
	for _, b := range remotes {
		out = append(out, bindingInfo{
			FEC:       b.FEC.String(),
			Label:     b.Label,
			Direction: "remote",
			PeerAddr:  b.PeerAddr.String(),
		})
	}

	return out
}
