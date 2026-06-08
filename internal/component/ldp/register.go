// Design: plan/spec-mpls-2-ldp.md -- LDP component registration
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
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"

	ldpyang "codeberg.org/thomas-mangin/ze/internal/component/ldp/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
		var tree map[string]any
		if err := json.Unmarshal([]byte(sec.Data), &tree); err != nil {
			return cfg, fmt.Errorf("ldp: invalid config JSON: %w", err)
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
		if v, ok := tree["hello-interval"].(float64); ok && v > 0 {
			cfg.HelloInterval = time.Duration(v) * time.Second
		}
		if v, ok := tree["hello-hold-time"].(float64); ok && v > 0 {
			cfg.HelloHoldTime = time.Duration(v) * time.Second
		}
		if v, ok := tree["keepalive-time"].(float64); ok && v > 0 {
			cfg.KeepaliveTime = time.Duration(v) * time.Second
		}
		if ifaces, ok := tree["interfaces"].([]any); ok {
			for _, iface := range ifaces {
				if s, ok := iface.(string); ok {
					cfg.Interfaces = append(cfg.Interfaces, s)
				}
			}
		}
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
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				setMetricsRegistry(r)
			}
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setEventBus(e)
			}
		},
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

	lib := NewLIB()
	adjTable := NewAdjacencyTable()

	var activeCfg ldpConfig
	var sessionsMu sync.Mutex
	sessions := make(map[string]*Session)
	var fib *ldpFIB

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		_, err := parseLDPConfig(sections)
		return err
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseLDPConfig(sections)
		if err != nil {
			return err
		}
		activeCfg = cfg
		return nil
	})

	p.OnStarted(func(ctx context.Context) error {
		cfg := activeCfg
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

		go runDiscoveryLoop(ctx, log, cfg, lsrID, adjTable, func(adj *Adjacency) {
			startSessionForAdj(ctx, log, adj, lsrID, lib, sessions, &sessionsMu, fib)
		})

		go runAdjacencyExpiry(ctx, log, adjTable)

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
		for _, lb := range lib.LocalBindings() {
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

func emitLabelEvent(eb ze.EventBus, log *slog.Logger, evt *LabelBindEvent) {
	if eb == nil {
		return
	}
	if _, err := LabelBind.Emit(eb, evt); err != nil {
		log.Warn("ldp: label-bind emit failed", "error", err)
	}
}

func startSessionForAdj(ctx context.Context, log *slog.Logger, adj *Adjacency, lsrID [4]byte, lib *LIB, sessions map[string]*Session, sessionsMu *sync.Mutex, fib *ldpFIB) {
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
	dialer := net.Dialer{Timeout: 5 * time.Second}
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
	})

	peerAddr := adj.TransportAddr
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
		})
		for _, b := range removed {
			emitLabelEvent(eb, log, &LabelBindEvent{
				FEC:      b.FEC.String(),
				Label:    b.Label,
				PeerAddr: b.PeerAddr.String(),
				Action:   "withdraw",
			})
		}
	})
}

// runDiscoveryLoop runs LDP Basic Discovery. RFC 5036 Section 2.4.1 sends Hellos
// to 224.0.0.2 on each link LDP is enabled on, so discovery binds one multicast
// listener per configured interface (Hellos must egress the LDP link, not the
// system-default multicast interface). With no interface configured it falls back
// to the system-assigned interface, preserving the prior single-socket behavior.
func runDiscoveryLoop(ctx context.Context, log *slog.Logger, cfg ldpConfig, lsrID [4]byte, adjTable *AdjacencyTable, onNewAdj func(*Adjacency)) {
	if len(cfg.Interfaces) == 0 {
		discoverOnInterface(ctx, log, cfg, lsrID, "", adjTable, onNewAdj)
		return
	}
	var wg sync.WaitGroup
	for _, name := range cfg.Interfaces {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			discoverOnInterface(ctx, log, cfg, lsrID, name, adjTable, onNewAdj)
		}(name)
	}
	wg.Wait()
}

// ldpInterfaceRetry is how often discovery re-checks for a configured interface
// that is not yet present.
const ldpInterfaceRetry = 5 * time.Second

// waitForInterface resolves ifName, retrying every retry interval until it appears
// or ctx is canceled (returns nil on cancellation). This lets LDP recover when a
// configured interface comes up after the engine starts. The "not available"
// warning is logged once, then demoted to debug, so a permanently-misconfigured
// interface name does not spam the log every retry.
func waitForInterface(ctx context.Context, log *slog.Logger, ifName string, retry time.Duration) *net.Interface {
	warned := false
	for {
		if ifi, err := net.InterfaceByName(ifName); err == nil {
			return ifi
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
		case <-timer.C:
		}
	}
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

	udpConn, err := net.ListenMulticastUDP("udp4", ifi, multicastAddr)
	if err != nil {
		log.Error("ldp: failed to listen on multicast", "interface", ifName, "error", err)
		return
	}
	defer func() {
		if err := udpConn.Close(); err != nil {
			log.Debug("ldp: UDP close error", "error", err)
		}
	}()

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

	recvBuf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return
		case <-helloTicker.C:
			sendHello(udpConn, multicastAddr, lsrID, cfg, log)
		}

		if err := udpConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return
		}

		n, _, readErr := udpConn.ReadFromUDP(recvBuf)
		if readErr != nil {
			var ne net.Error
			if errors.As(readErr, &ne) && ne.Timeout() {
				continue
			}
			log.Warn("ldp: UDP read error", "error", readErr)
			continue
		}

		processDiscoveryPacket(recvBuf[:n], lsrID, adjTable, onNewAdj, log)
	}
}

func processDiscoveryPacket(data []byte, localLSRID [4]byte, adjTable *AdjacencyTable, onNewAdj func(*Adjacency), log *slog.Logger) {
	if len(data) < ldpHeaderLen+ldpMsgHdrLen {
		return
	}

	pdu, err := DecodePDUHeader(data)
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

	msgHdr, err := DecodeMessageHeader(data[bodyStart:])
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

	adj, isNew := adjTable.Update(pdu, hello)
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
	EncodePDUHeader(buf[:], PDUHeader{
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
		func(lm LabelMappingMessage, _ [4]byte) {
			// AC-4: resolve the data-plane next hop from the peer's advertised
			// interface addresses (Address message) when a label is imposed;
			// implicit-null forwards as plain IP and needs no next hop.
			var nextHop netip.Addr
			if lm.Label != ImplicitNull {
				nextHop = pickNextHop(sess.PeerAddr(), sess.PeerAddresses(), localPrefixes)
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
			emitLabelEvent(eb, log, &LabelBindEvent{
				FEC:      lm.FEC.Prefix.String(),
				Label:    lm.Label,
				PeerAddr: sess.PeerAddr().String(),
				Action:   "add",
			})
		},
		func(lw LabelWithdrawMessage, _ [4]byte) {
			// AC-5: drop the LIB entry and reconcile the kernel forwarding for the
			// FEC (re-point to a surviving peer, or withdraw the push) atomically.
			fib.withReconcileLock(func() {
				withdrawRemoteBinding(fib, lib, lw.FEC.Prefix, peerKey, log)
			})
			if m := ldpMetricsPtr.Load(); m != nil {
				m.bindingsActive.Set(float64(lib.Len()))
			}
			emitLabelEvent(eb, log, &LabelBindEvent{
				FEC:      lw.FEC.Prefix.String(),
				Label:    lw.Label,
				PeerAddr: sess.PeerAddr().String(),
				Action:   "withdraw",
			})
		},
		func() {
			// AC-3: session reached operational -- advertise our local FEC
			// bindings downstream-unsolicited (RFC 5036 Section 2.3).
			locals := lib.LocalBindings()
			for _, lb := range locals {
				if err := sess.SendLabelMapping(lb.FEC, lb.Label); err != nil {
					log.Warn("ldp: local label mapping send failed", "fec", lb.FEC, "error", err)
					return
				}
				emitLabelEvent(eb, log, &LabelBindEvent{
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

func runAdjacencyExpiry(ctx context.Context, log *slog.Logger, adjTable *AdjacencyTable) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expired := adjTable.ExpireSweep()
			for _, key := range expired {
				log.Info("ldp: adjacency expired", "peer", key)
			}
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

	locals := lib.LocalBindings()
	remotes := lib.AllBindings()
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
