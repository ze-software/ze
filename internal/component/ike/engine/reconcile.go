// Design: plan/spec-ipsec-7-ikev2-engine.md -- config reconciliation
package engine

import (
	"log/slog"
	"maps"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// PeerSession manages a single IKE peer's goroutine lifecycle.
type PeerSession struct {
	peerName string
	peerCfg  ipsec.SiteToSitePeer
	sa       *SA
	stopCh   chan struct{}
	done     chan struct{}
}

// Stop signals the peer session to shut down and waits for cleanup.
func (ps *PeerSession) Stop() {
	close(ps.stopCh)
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

	// Stop removed or changed peers.
	for name, ps := range active {
		newPeer, ok := desired[name]
		if !ok {
			log.Info("ike: stopping removed peer", "peer", name)
			ps.Stop()
			if ps.sa != nil {
				table.Remove(ps.sa.InitiatorSPI, ps.sa.ResponderSPI)
				emitSADown(bus, ps.sa, log)
			}
			delete(active, name)
			continue
		}
		if peerConfigChanged(ps, newPeer) {
			log.Info("ike: restarting changed peer", "peer", name)
			ps.Stop()
			if ps.sa != nil {
				table.Remove(ps.sa.InitiatorSPI, ps.sa.ResponderSPI)
				emitSADown(bus, ps.sa, log)
			}
			delete(active, name)
		}
	}

	// Start new or restarted peers.
	for name := range desired {
		if _, running := active[name]; running {
			continue
		}
		peer := desired[name]
		ikeGroup, ok := newCfg.IKEGroups[peer.IKEGroup]
		if !ok {
			log.Warn("ike: peer references unknown ike-group", "peer", name, "ike-group", peer.IKEGroup)
			continue
		}
		ps := startPeerSession(name, peer, ikeGroup, table, tr, bus, log)
		active[name] = ps
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
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) *PeerSession {
	ps := &PeerSession{
		peerName: name,
		peerCfg:  peer,
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
