// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE engine component registration
package engine

import (
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/eap"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	loggerPtr      atomic.Pointer[slog.Logger]
	eventBusPtr    atomic.Pointer[ze.EventBus]
	activeTablePtr atomic.Pointer[SATable]

	peersMu        sync.RWMutex
	activePeersMap map[string]*PeerSession

	reEstablishFn atomic.Pointer[func()]
)

func ActiveTable() *SATable                    { return activeTablePtr.Load() }
func SetActiveTable(t *SATable)                { activeTablePtr.Store(t) }
func SetActivePeers(m map[string]*PeerSession) { setActivePeers(m) }

func ActivePeers() map[string]*PeerSession {
	peersMu.RLock()
	defer peersMu.RUnlock()
	if activePeersMap == nil {
		return nil
	}
	out := make(map[string]*PeerSession, len(activePeersMap))
	maps.Copy(out, activePeersMap)
	return out
}

// PeerInfoMap returns a snapshot of peer info for all active sessions.
func PeerInfoMap() map[string]PeerInfo {
	peersMu.RLock()
	defer peersMu.RUnlock()
	if activePeersMap == nil {
		return nil
	}
	out := make(map[string]PeerInfo, len(activePeersMap))
	for name, ps := range activePeersMap {
		out[name] = ps.Info()
	}
	return out
}

func setActivePeers(m map[string]*PeerSession) {
	peersMu.Lock()
	activePeersMap = m
	peersMu.Unlock()
}

func TerminateAllSAs() int {
	peersMu.Lock()
	if activePeersMap == nil {
		peersMu.Unlock()
		return 0
	}
	snapshot := make(map[string]*PeerSession, len(activePeersMap))
	maps.Copy(snapshot, activePeersMap)
	peersMu.Unlock()

	table := ActiveTable()
	bus := getEventBus()
	log := getLogger()
	count := 0
	for name, ps := range snapshot {
		// Delete from the active map BEFORE stopping (like TerminatePeerSA), so the
		// shared dispatch goroutine cannot accept a fresh IKE_SA_INIT for this peer that
		// would escape the cleanup below and leak.
		peersMu.Lock()
		delete(activePeersMap, name)
		peersMu.Unlock()
		// StopGraceful: the owner loop sends an authenticated INFORMATIONAL Delete on
		// its way out (RFC 7296 Section 1.4) so the peer tears down at once instead of
		// waiting for the DPD timeout -- the operator-visible half of the fix.
		ps.StopGraceful()
		// getSA (mutex-guarded): a responder's ps.sa is written by the dispatch
		// goroutine, not joined by Stop() (Finding 3).
		if sa := ps.getSA(); sa != nil && table != nil {
			table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
			emitSADown(bus, sa, log)
		}
		ps.cleanupPendingSA(table, dataplane.Get(), bus, log)
		count++
	}

	if fn := reEstablishFn.Load(); fn != nil {
		(*fn)()
	}
	return count
}

func TerminatePeerSA(name string) bool {
	peersMu.Lock()
	if activePeersMap == nil {
		peersMu.Unlock()
		return false
	}
	ps, ok := activePeersMap[name]
	if !ok {
		peersMu.Unlock()
		return false
	}
	delete(activePeersMap, name)
	peersMu.Unlock()

	ps.StopGraceful()
	table := ActiveTable()
	bus := getEventBus()
	log := getLogger()
	// getSA (mutex-guarded): a responder's ps.sa is written by the dispatch goroutine,
	// not joined by Stop() (Finding 3).
	if sa := ps.getSA(); sa != nil && table != nil {
		table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
		emitSADown(bus, sa, log)
	}
	ps.cleanupPendingSA(table, dataplane.Get(), bus, log)

	if fn := reEstablishFn.Load(); fn != nil {
		(*fn)()
	}
	return true
}

func setLogger(l *slog.Logger)   { loggerPtr.Store(l) }
func getLogger() *slog.Logger    { return loggerPtr.Load() }
func setEventBus(eb ze.EventBus) { eventBusPtr.Store(&eb) }

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
	RegisterHealthCheck()
	registerIPsecRedistSources()

	reg := registry.Registration{
		Name:        "ike",
		Description: "IKEv2 engine for native IPsec VPN",
		ConfigRoots: []string{"vpn", "pki"},
		RunEngine:   runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ike: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := getLogger()
	log.Debug("ike engine starting")

	if err := dataplane.Load(ikeDataplaneName()); err != nil {
		log.Warn("ike: dataplane load failed, SA installation disabled", "error", err)
	}

	p := sdk.NewWithConn("ike", conn)
	defer closeSDK(p)

	table := NewSATable()
	activeTablePtr.Store(table)
	var tr *transport.UDPTransport
	var trNATT *transport.UDPTransport
	var activeCfg *ipsec.IPsecConfig
	var ipPool *eap.Pool
	activePeers := make(map[string]*PeerSession)
	setActivePeers(activePeers)

	var ipsecMetrics *IPsecMetrics
	if reg := registry.GetMetricsRegistry(); reg != nil {
		ipsecMetrics = RegisterMetrics(reg)
	}

	type reEstablishCtx struct {
		cfg *ipsec.IPsecConfig
		tr  *transport.UDPTransport
	}
	var reCtx atomic.Pointer[reEstablishCtx]

	reEstablish := func() {
		rc := reCtx.Load()
		if rc == nil || rc.cfg == nil {
			return
		}
		eb := getEventBus()
		reconcilePeers(rc.cfg, nil, activePeers, table, rc.tr, eb, log)
	}
	reEstablishFn.Store(&reEstablish)

	metricsStop := make(chan struct{})
	if ipsecMetrics != nil {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-metricsStop:
					return
				case <-ticker.C:
					ipsecMetrics.Update()
				}
			}
		}()
	}

	// Reject a structurally valid but self-inconsistent config before it is
	// applied: a peer naming an undefined ike-group or esp-group, a certificate
	// reference the PKI store cannot resolve, an EAP-TLS peer with no trust
	// anchor, or a malformed remote-access pool.
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		if err := ValidateIPsecSections(sections); err != nil {
			return fmt.Errorf("ike config: %w", err)
		}
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseIPsecSections(sections)
		if err != nil {
			return fmt.Errorf("ike config: %w", err)
		}

		if cfg.Interface != "" {
			if ifIP := resolveInterfaceAddr(cfg.Interface); ifIP != "" {
				for name := range cfg.Peers {
					peer := cfg.Peers[name]
					if peer.LocalAddress == "" {
						peer.LocalAddress = ifIP
						cfg.Peers[name] = peer
						log.Debug("ike: resolved local-address from interface", "peer", name, "interface", cfg.Interface, "address", ifIP)
					}
				}
			} else {
				log.Warn("ike: no IPv4 address on interface, peers without local-address will fail", "interface", cfg.Interface)
			}
		}

		if tr == nil && len(cfg.Peers) > 0 {
			ifaceHost := ""
			if cfg.Interface != "" {
				ifaceHost = resolveInterfaceAddr(cfg.Interface)
			}
			peerLocal := ""
			for name := range cfg.Peers {
				if la := cfg.Peers[name].LocalAddress; la != "" {
					peerLocal = la
					break
				}
			}
			listenAddr := ikeAddr(ikeListenHost(ifaceHost, peerLocal))
			var tErr error
			tr, tErr = transport.NewUDPTransport(listenAddr, log)
			if tErr != nil {
				log.Warn("ike: failed to start UDP transport", "error", tErr)
			} else {
				go tr.Run()
				go dispatchInbound(tr, table, log)
			}
		}

		// RFC 3948: start NAT-T listener on port 4500 for UDP-encapsulated IKE and ESP.
		if trNATT == nil && len(cfg.Peers) > 0 {
			nattAddr := "0.0.0.0:4500"
			if cfg.Interface != "" {
				if ip := resolveInterfaceAddr(cfg.Interface); ip != "" {
					nattAddr = ip + ":4500"
				}
			}
			var nErr error
			trNATT, nErr = transport.NewUDPTransport(nattAddr, log)
			if nErr != nil {
				log.Warn("ike: failed to start NAT-T transport", "error", nErr)
			} else {
				go trNATT.Run()
				go dispatchNATTInbound(trNATT, table, log)
			}
		}

		// Create virtual IP pool from remote-access config.
		if cfg.RemoteAccess != nil && ipPool == nil {
			ra := cfg.RemoteAccess
			var poolErr error
			ipPool, poolErr = eap.NewPool(ra.Pool.Range, ra.Pool.Range6, ra.Pool.DNS, ra.Pool.Domain)
			if poolErr != nil {
				log.Warn("ike: failed to create virtual IP pool", "error", poolErr)
			} else {
				log.Info("ike: virtual IP pool created", "range", ra.Pool.Range)
			}
		}

		eb := getEventBus()
		reconcilePeers(cfg, activeCfg, activePeers, table, tr, eb, log)
		activeCfg = cfg
		reCtx.Store(&reEstablishCtx{cfg: cfg, tr: tr})

		if ipsecMetrics != nil {
			ipsecMetrics.Update()
		}

		log.Info("ike engine configured", "peers", len(cfg.Peers))
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"vpn"},
	}); err != nil {
		log.Error("ike engine failed", "error", err)
		return 1
	}

	// Cleanup after Run returns (shutdown).
	reEstablishFn.Store(nil)
	close(metricsStop)
	peersMu.Lock()
	shutdownPeers := make(map[string]*PeerSession, len(activePeers))
	maps.Copy(shutdownPeers, activePeers)
	peersMu.Unlock()
	shutdownBus := getEventBus()
	for name, ps := range shutdownPeers {
		ps.Stop()
		ps.cleanupPendingSA(table, dataplane.Get(), shutdownBus, log)
		peersMu.Lock()
		delete(activePeers, name)
		peersMu.Unlock()
	}
	if tr != nil {
		if err := tr.Close(); err != nil {
			log.Warn("ike: transport close error", "error", err)
		}
	}
	if trNATT != nil {
		if err := trNATT.Close(); err != nil {
			log.Warn("ike: NAT-T transport close error", "error", err)
		}
	}
	_ = ipPool
	if err := dataplane.CloseBackend(); err != nil {
		log.Warn("ike: dataplane close error", "error", err)
	}

	return 0
}

func closeSDK(p *sdk.Plugin) {
	if err := p.Close(); err != nil {
		getLogger().Debug("ike: sdk close", "error", err)
	}
}

// inboundRateLimiter is a simple token-bucket rate limiter for inbound IKE packets.
type inboundRateLimiter struct {
	mu       sync.Mutex
	tokens   int
	max      int
	lastFill time.Time
	rate     int // tokens per second
}

func newInboundRateLimiter(perSecond, burst int) *inboundRateLimiter {
	return &inboundRateLimiter{
		tokens:   burst,
		max:      burst,
		rate:     perSecond,
		lastFill: time.Now(),
	}
}

func (l *inboundRateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.lastFill)
	if elapsed >= time.Second {
		l.tokens = l.max
		l.lastFill = now
	} else {
		fill := int(elapsed.Seconds() * float64(l.rate))
		l.tokens = min(l.tokens+fill, l.max)
		if fill > 0 {
			l.lastFill = now
		}
	}
	if l.tokens <= 0 {
		return false
	}
	l.tokens--
	return true
}

// dispatchNATTInbound reads packets from the NAT-T transport (port 4500) and
// dispatches IKE packets after stripping the non-ESP marker.
// RFC 3948 Section 2.2: 4 zero bytes prefix distinguishes IKE from ESP.
func dispatchNATTInbound(tr *transport.UDPTransport, table *SATable, log *slog.Logger) {
	limiter := newInboundRateLimiter(100, 200)

	for pkt := range tr.Recv() {
		if transport.IsNATKeepalive(pkt.Data) {
			continue
		}

		ikeData, isIKE := transport.StripNonESPMarker(pkt.Data)
		if !isIKE {
			continue
		}

		if len(ikeData) < 28 {
			continue
		}
		if ikeData[17]>>4 != 2 {
			continue
		}
		if !limiter.allow() {
			continue
		}

		var iSPI, rSPI [8]byte
		copy(iSPI[:], ikeData[0:8])
		copy(rSPI[:], ikeData[8:16])

		if iSPI == [8]byte{} {
			continue
		}

		nattPkt := transport.Packet{
			Data:       ikeData,
			RemoteAddr: pkt.RemoteAddr,
			LocalAddr:  pkt.LocalAddr,
		}

		sa := table.Lookup(iSPI, rSPI)
		if sa == nil {
			sa = table.LookupByInitiatorSPI(iSPI)
		}
		if sa == nil {
			if tryResponderSAInit(nattPkt, iSPI, rSPI, table, tr, log) {
				continue
			}
			log.Debug("ike: no SA for NAT-T packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
			continue
		}

		routeInbound(sa, nattPkt, table, tr, log)
	}
}

// inboundQueueDepth bounds the per-session owner-loop inbound queue. Control-plane
// exchanges are one-at-a-time per SA, so a small buffer absorbs the establish
// hand-off window (SA marked established before maintainSA starts consuming)
// without letting a stalled owner back up the shared dispatch goroutine.
const inboundQueueDepth = 16

// lookupPeerSession returns the running session for a peer name, or nil.
func lookupPeerSession(name string) *PeerSession {
	peersMu.RLock()
	defer peersMu.RUnlock()
	if activePeersMap == nil {
		return nil
	}
	return activePeersMap[name]
}

// routeInbound delivers a received packet to the correct handler. For the SA that
// maintainSA currently owns it hands the packet to that owner loop (single-owner
// model, spec-ipsec-13) via a non-blocking send; every other SA -- an initial or a
// PARALLEL half-open handshake (spec-fixit-ipsec-clear-reestablish) -- is handled
// inline on the dispatch goroutine. If the owner queue is full the packet is dropped
// and the peer will retransmit.
func routeInbound(sa *SA, pkt transport.Packet, table *SATable, tr *transport.UDPTransport, log *slog.Logger) {
	// Key the owner-loop hand-off on SA identity, not the peer name: `ps.ownedSA` is
	// an atomic.Pointer the session goroutine keeps pointed at the exact SA maintainSA
	// owns (updated on an IKE-SA rekey swap too), so reading it here on the shared
	// dispatch goroutine does not race owner-side sa.State writes, and a parallel
	// half-open SA of the same peer is NOT misdelivered to the established SA's owner
	// loop (which would decrypt it under the wrong keys). RFC 7296 Section 2.4.
	if ps := lookupPeerSession(sa.PeerName); ps != nil && ps.inbound != nil && ps.ownedSA.Load() == sa {
		select {
		case ps.inbound <- pkt:
		default:
			log.Warn("ike: owner inbound queue full, dropping packet", "peer", sa.PeerName)
		}
		return
	}
	handleInbound(sa, pkt, table, tr, log)
}

// matchResponderPeer finds a running `respond` peer whose configured remote
// address equals the packet source, or nil. Used to accept an unsolicited
// IKE_SA_INIT. Called on the dispatch goroutine; reads immutable session config
// under the peers lock.
func matchResponderPeer(remoteAddr *net.UDPAddr) *PeerSession {
	if remoteAddr == nil {
		return nil
	}
	src := remoteAddr.IP.String()
	peersMu.RLock()
	defer peersMu.RUnlock()
	if activePeersMap == nil {
		return nil
	}
	for _, ps := range activePeersMap {
		if ps.peerCfg.ConnectionType != ipsec.ConnectionRespond {
			continue
		}
		if ps.peerCfg.RemoteAddress != "" && ps.peerCfg.RemoteAddress == src {
			return ps
		}
	}
	return nil
}

// tryResponderSAInit accepts an unsolicited IKE_SA_INIT request (no SATable entry)
// from a configured `respond` peer: it creates the responder SA, inserts it, and
// hands the packet to the handshake handler. Returns true when the packet was
// consumed (accepted or deliberately dropped as an unconfigured/duplicate attempt).
// RFC 7296 Section 1.2, Section 2.6.
func tryResponderSAInit(pkt transport.Packet, iSPI, rSPI [8]byte, table *SATable, tr *transport.UDPTransport, log *slog.Logger) bool {
	// Header: [18]=exchange type, [19]=flags. Must be an IKE_SA_INIT request with a
	// zero responder SPI (a fresh initiation, not a retransmit of a known SA).
	if len(pkt.Data) < 20 {
		return false
	}
	if pkt.Data[18] != wire.ExchangeIKESAInit || pkt.Data[19]&wire.FlagResponse != 0 {
		return false
	}
	if rSPI != ([8]byte{}) {
		return false
	}
	ps := matchResponderPeer(pkt.RemoteAddr)
	if ps == nil {
		log.Debug("ike: unsolicited IKE_SA_INIT from unconfigured source", "src", pkt.RemoteAddr)
		return false
	}
	// One in-flight HALF-OPEN handshake per responder peer (AC-6). A genuine
	// retransmit finds the SA already in the SATable and never reaches this path.
	// RFC 7296 Section 2.4: the busy gate is NOT held across an established SA's
	// lifetime, so a fresh IKE_SA_INIT that arrives while a tunnel is up passes the
	// CAS and is accepted in PARALLEL; the established SA is never touched by this
	// unauthenticated message and is superseded only once the new SA authenticates
	// (finishResponderEstablish). This is the AC-3 / AC-7 accept-in-parallel path.
	if !ps.responderBusy.CompareAndSwap(false, true) {
		log.Debug("ike: responder busy, dropping concurrent IKE_SA_INIT", "peer", ps.peerName)
		return true
	}
	sa, err := newResponderSA(ps.peerName, ps.peerCfg, ps.ikeGroup, ps.espGroup, iSPI)
	if err != nil {
		log.Warn("ike: create responder SA failed", "peer", ps.peerName, "error", err)
		ps.responderBusy.Store(false)
		return true
	}
	if !table.Insert(sa) {
		log.Debug("ike: responder SA insert conflict", "peer", ps.peerName)
		ps.responderBusy.Store(false)
		return true
	}
	if ps.ownedSA.Load() != nil {
		// An established SA already owns the loop: the new handshake coexists in the
		// second slot and drives inline on the dispatch goroutine (routeInbound keys
		// on SA identity, so it is not delivered to the old SA's owner loop).
		ps.setPendingSA(sa)
		log.Info("ike: accepting parallel inbound IKE_SA_INIT alongside established SA", "peer", ps.peerName, "src", pkt.RemoteAddr)
	} else {
		ps.setSA(sa)
		log.Info("ike: accepting inbound IKE_SA_INIT", "peer", ps.peerName, "src", pkt.RemoteAddr)
	}
	routeInbound(sa, pkt, table, tr, log)
	return true
}

// dispatchInbound reads packets from the transport and dispatches to the
// correct SA by SPI pair.
func dispatchInbound(tr *transport.UDPTransport, table *SATable, log *slog.Logger) {
	limiter := newInboundRateLimiter(100, 200)

	for pkt := range tr.Recv() {
		if len(pkt.Data) < 28 {
			continue
		}
		// RFC 7296 Section 3.1: major version in upper nibble of byte 17.
		if pkt.Data[17]>>4 != 2 {
			continue
		}
		if !limiter.allow() {
			continue
		}

		var iSPI, rSPI [8]byte
		copy(iSPI[:], pkt.Data[0:8])
		copy(rSPI[:], pkt.Data[8:16])

		// RFC 7296 Section 2.6: initiator SPI MUST NOT be zero.
		if iSPI == [8]byte{} {
			continue
		}

		sa := table.Lookup(iSPI, rSPI)
		if sa == nil {
			sa = table.LookupByInitiatorSPI(iSPI)
		}
		if sa == nil {
			if tryResponderSAInit(pkt, iSPI, rSPI, table, tr, log) {
				continue
			}
			log.Debug("ike: no SA for packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
			continue
		}

		routeInbound(sa, pkt, table, tr, log)
	}
}

// resolveInterfaceAddr returns the first IPv4 address of the logical interface,
// resolved through the shared iface resolver so the IKE bind/listen address
// honors the os-name / mac-match selectors instead of assuming name == kernel
// device.
func resolveInterfaceAddr(name string) string {
	addrs, err := iface.Addresses(name)
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if a.Family == "ipv4" {
			return a.Address
		}
	}
	return ""
}
