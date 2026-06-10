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

	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/eap"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
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
		ps.Stop()
		if ps.sa != nil && table != nil {
			table.Remove(ps.sa.InitiatorSPI, ps.sa.ResponderSPI)
			emitSADown(bus, ps.sa, log)
		}
		peersMu.Lock()
		delete(activePeersMap, name)
		peersMu.Unlock()
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

	ps.Stop()
	table := ActiveTable()
	if ps.sa != nil && table != nil {
		table.Remove(ps.sa.InitiatorSPI, ps.sa.ResponderSPI)
		bus := getEventBus()
		log := getLogger()
		emitSADown(bus, ps.sa, log)
	}

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
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setEventBus(e)
			}
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

	if err := dataplane.Load("xfrm"); err != nil {
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
		if mr, ok := reg.(metrics.Registry); ok {
			ipsecMetrics = RegisterMetrics(mr)
		}
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
			listenAddr := "0.0.0.0:500"
			if cfg.Interface != "" {
				if ip := resolveInterfaceAddr(cfg.Interface); ip != "" {
					listenAddr = ip + ":500"
				}
			}
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
	for name, ps := range shutdownPeers {
		ps.Stop()
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

		sa := table.Lookup(iSPI, rSPI)
		if sa == nil {
			sa = table.LookupByInitiatorSPI(iSPI)
		}
		if sa == nil {
			log.Debug("ike: no SA for NAT-T packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
			continue
		}

		nattPkt := transport.Packet{
			Data:       ikeData,
			RemoteAddr: pkt.RemoteAddr,
			LocalAddr:  pkt.LocalAddr,
		}
		handleInbound(sa, nattPkt, table, tr, log)
	}
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
			log.Debug("ike: no SA for packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
			continue
		}

		handleInbound(sa, pkt, table, tr, log)
	}
}

func resolveInterfaceAddr(name string) string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil || len(addrs) == 0 {
		return ""
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return ""
}
