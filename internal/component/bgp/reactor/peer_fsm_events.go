// Design: docs/architecture/api/json-format.md -- the connected and fsm events
// Related: peer_run.go -- the FSM callback that calls emitFSMEvents
// Related: ../server/events.go -- deliverPeerLine delivers both events

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/plugin"
)

// emitFSMEvents tells the subscribed processes about one FSM state change:
// the `fsm` event for every transition, and before it the `connected` event
// when the transition is the one a successful TCP connection drives.
//
// RFC 4271 Section 8.2.2 (Connect and Active states): "If the TCP connection
// succeeds (Event 16 or Event 17), the local system checks the DelayOpen
// attribute prior to processing. ... If the DelayOpen attribute is set to
// FALSE, the local system: ... sends an OPEN message to its peer, ... and
// changes its state to OpenSent." Ze runs no DelayOpen, so Event 17 into
// OpenSent is the one transition that says the connection is up and no OPEN
// has been read on it.
//
// It runs on the FSM callback (fsm.FSM.SetCallback), without the FSM lock and
// without p.mu held, once per transition and in transition order.
func (p *Peer) emitFSMEvents(from, to fsm.State, event fsm.Event) {
	p.mu.RLock()
	reactor := p.reactor
	p.mu.RUnlock()
	if reactor == nil {
		return
	}
	dispatcher := reactor.eventDispatcher
	if dispatcher == nil {
		return
	}
	peerInfo := fsmEventPeerInfo(p)
	if to == fsm.StateOpenSent && event == fsm.EventTCPConnectionConfirmed {
		dispatcher.OnPeerConnected(&peerInfo)
	}
	dispatcher.OnPeerFSMChange(&peerInfo, from, to)
}

// fsmEventPeerInfo is the peer as an FSM event names it. The local address is
// the connected endpoint once a session holds a connection, which is the
// address the kernel chose when the configured one is "auto".
func fsmEventPeerInfo(peer *Peer) plugin.PeerInfo {
	s := peer.Settings()
	local, localString := connectedLocalEndpoint(peer, s.LocalAddress, peer.localAddrString)
	return plugin.PeerInfo{
		Address:         s.Address,
		LocalAddress:    local,
		AddressStr:      peer.addrString,
		LocalAddressStr: localString,
		Name:            s.Name,
		GroupName:       s.GroupName,
		LocalAS:         s.LocalAS,
		// PeerAS via the guarded accessor: a dynamic peer's establishment
		// writes it, and a transition can be drained on another goroutine.
		PeerAS:   peer.PeerAS(),
		RouterID: s.RouterID,
		State:    peer.State().PluginState(),
	}
}
