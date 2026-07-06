// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- config reconciliation
package engine

import (
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/dataplane"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// PeerSession manages a single IKE peer's goroutine lifecycle.
type PeerSession struct {
	peerName string
	peerCfg  ipsec.SiteToSitePeer
	espGroup ipsec.ESPGroup
	sa       *SA

	mu         sync.Mutex
	childSA    *ChildSA
	rekeyCount uint64

	// inbound carries packets for an ESTABLISHED SA from the shared
	// dispatchInbound goroutine to this session's maintainSA owner loop, so
	// all post-establishment SA/childSA mutation happens on one goroutine
	// (rekey design, spec-ipsec-13). Sends are non-blocking: dispatchInbound
	// serves every peer, so it must never block on one slow owner.
	inbound chan transport.Packet

	// pendingRekey and supersededChild are owned exclusively by the maintainSA
	// loop (no lock): the CREATE_CHILD_SA exchange we initiated and await a
	// response for, and (as rekey responder) the old Child SA kept installed
	// until the peer's INFORMATIONAL Delete arrives (make-before-break).
	pendingRekey    *pendingRekey
	supersededChild *ChildSA

	// established gates owner-loop routing. Set true when maintainSA owns the SA
	// and false during (re)handshake. routeInbound reads it on the shared dispatch
	// goroutine, so it is atomic and set-once-per-cycle: this avoids reading the
	// lockless sa.State across goroutines (which raced with owner-side State writes).
	established atomic.Bool

	stopCh   chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

func (ps *PeerSession) setChildSA(c *ChildSA) {
	ps.mu.Lock()
	ps.childSA = c
	ps.mu.Unlock()
}

func (ps *PeerSession) getChildSA() *ChildSA {
	ps.mu.Lock()
	c := ps.childSA
	ps.mu.Unlock()
	return c
}

func (ps *PeerSession) incRekeyCount() {
	ps.mu.Lock()
	ps.rekeyCount++
	ps.mu.Unlock()
}

// PeerInfo is a snapshot of a peer session's state for display.
type PeerInfo struct {
	PeerName      string
	RemoteAddress string
	LocalAddress  string
	AuthMode      string
	ChildInSPI    uint32
	ChildOutSPI   uint32
	ChildIfID     uint32
	TSLocal       string
	TSRemote      string
	ESPEncryption string
	ESPIntegrity  string
	Lifetime      uint32
	RekeyCount    uint64
	HasChild      bool
}

// Info returns a snapshot of the peer session for display.
func (ps *PeerSession) Info() PeerInfo {
	ps.mu.Lock()
	child := ps.childSA
	rekeys := ps.rekeyCount
	ps.mu.Unlock()

	info := PeerInfo{
		PeerName:      ps.peerName,
		RemoteAddress: ps.peerCfg.RemoteAddress,
		LocalAddress:  ps.peerCfg.LocalAddress,
		AuthMode:      ps.peerCfg.Auth.Mode.String(),
		Lifetime:      ps.espGroup.Lifetime,
		RekeyCount:    rekeys,
	}
	if child != nil {
		info.HasChild = true
		info.ChildInSPI = child.InboundSPI
		info.ChildOutSPI = child.OutboundSPI
		info.ChildIfID = child.IfID
		if child.TSLocal != nil {
			info.TSLocal = child.TSLocal.String()
		}
		if child.TSRemote != nil {
			info.TSRemote = child.TSRemote.String()
		}
		if len(ps.espGroup.Proposals) > 0 {
			info.ESPEncryption = ps.espGroup.Proposals[0].Encryption.String()
			info.ESPIntegrity = ps.espGroup.Proposals[0].Hash.String()
		}
	}
	return info
}

// Stop signals the peer session to shut down and waits for cleanup.
// Safe to call multiple times.
func (ps *PeerSession) Stop() {
	ps.stopOnce.Do(func() { close(ps.stopCh) })
	<-ps.done
}

// reconcilePeers diffs the new config against running peers and starts/stops
// peer sessions as needed. Follows the PPPoE reconciliation pattern.
func reconcilePeers(
	newCfg, _ *ipsec.IPsecConfig,
	active map[string]*PeerSession,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) {
	desired := make(map[string]ipsec.SiteToSitePeer, len(newCfg.Peers))
	maps.Copy(desired, newCfg.Peers)

	dp := dataplane.Get()

	// Build removal list under read lock, then stop outside the lock.
	type toStop struct {
		name string
		ps   *PeerSession
	}
	var removing []toStop

	peersMu.RLock()
	for name, ps := range active {
		newPeer, ok := desired[name]
		if !ok || peerConfigChanged(ps, newPeer) {
			removing = append(removing, toStop{name, ps})
		}
	}
	peersMu.RUnlock()

	for _, r := range removing {
		log.Info("ike: stopping peer", "peer", r.name)
		r.ps.Stop()
		child := r.ps.getChildSA()
		if child != nil {
			removeChildSA(child, dp, log)
			emitChildDown(bus, r.name, child, log)
			emitRouteRemove(bus, child.TSRemote, log)
			r.ps.setChildSA(nil)
		}
		if r.ps.sa != nil {
			table.Remove(r.ps.sa.InitiatorSPI, r.ps.sa.ResponderSPI)
			emitSADown(bus, r.ps.sa, log)
		}
		peersMu.Lock()
		delete(active, r.name)
		peersMu.Unlock()
	}

	// Start new or restarted peers.
	for name := range desired {
		peersMu.RLock()
		_, running := active[name]
		peersMu.RUnlock()
		if running {
			continue
		}
		peer := desired[name]
		ikeGroup, ok := newCfg.IKEGroups[peer.IKEGroup]
		if !ok {
			log.Warn("ike: peer references unknown ike-group", "peer", name, "ike-group", peer.IKEGroup)
			continue
		}
		espGroup, ok := newCfg.ESPGroups[peer.ESPGroup]
		if !ok {
			log.Warn("ike: peer references unknown esp-group", "peer", name, "esp-group", peer.ESPGroup)
			continue
		}
		ps := startPeerSession(name, peer, ikeGroup, espGroup, table, tr, bus, log)
		peersMu.Lock()
		active[name] = ps
		peersMu.Unlock()
		log.Info("ike: started peer", "peer", name, "connection-type", peer.ConnectionType)
	}
}

func peerConfigChanged(ps *PeerSession, newPeer ipsec.SiteToSitePeer) bool {
	old := ps.peerCfg
	return old.RemoteAddress != newPeer.RemoteAddress ||
		old.LocalAddress != newPeer.LocalAddress ||
		old.IKEGroup != newPeer.IKEGroup ||
		old.ESPGroup != newPeer.ESPGroup ||
		old.ConnectionType != newPeer.ConnectionType ||
		old.Auth.Mode != newPeer.Auth.Mode ||
		old.Auth.PSK != newPeer.Auth.PSK ||
		old.Auth.Certificate != newPeer.Auth.Certificate
}

func startPeerSession(
	name string,
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	espGroup ipsec.ESPGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) *PeerSession {
	ps := &PeerSession{
		peerName: name,
		peerCfg:  peer,
		espGroup: espGroup,
		inbound:  make(chan transport.Packet, inboundQueueDepth),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
	go ps.run(peer, ikeGroup, table, tr, bus, log)
	return ps
}

// run is the main goroutine for a peer session with reconnection on failure.
func (ps *PeerSession) run(
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) {
	defer close(ps.done)

	for {
		err := ps.runOnce(peer, ikeGroup, table, tr, bus, log)
		if err == nil || ps.stopped() {
			return
		}

		delay := reconnectDelay(ps)
		log.Info("ike: peer session ended, reconnecting",
			"peer", ps.peerName, "error", err, "delay", delay)

		select {
		case <-ps.stopCh:
			return
		case <-afterFunc(delay):
		}
	}
}

func (ps *PeerSession) stopped() bool {
	select {
	case <-ps.stopCh:
		return true
	default:
		return false
	}
}

func emitSADown(bus ze.EventBus, sa *SA, log *slog.Logger) {
	if bus == nil || sa == nil {
		return
	}
	if _, err := SADown.Emit(bus, &SAEvent{
		PeerName:      sa.PeerName,
		InitiatorSPI:  SPIHex(sa.InitiatorSPI),
		ResponderSPI:  SPIHex(sa.ResponderSPI),
		RemoteAddress: sa.PeerCfg.RemoteAddress,
		AuthMethod:    sa.PeerCfg.Auth.Mode.String(),
	}); err != nil {
		log.Warn("ike: emit sa-down failed", "error", err)
	}
}
