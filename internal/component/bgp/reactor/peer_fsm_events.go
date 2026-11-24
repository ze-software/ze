// Design: docs/architecture/api/json-format.md -- the connected and fsm events
// Related: peer_run.go -- the FSM callback that calls emitFSMEvents
// Related: ../server/events.go -- deliverPeerLine delivers both events

package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// emitConnected reports TCP success from this exact Session before either
// OPEN rail runs. A pending candidate MUST name its own socket, rather than
// borrowing the incumbent's endpoint or a previously negotiated dynamic AS.
func (p *Peer) emitConnected(session *Session) {
	p.mu.RLock()
	reactor := p.reactor
	p.mu.RUnlock()
	if reactor == nil {
		return
	}
	if reactor.eventDispatcher == nil {
		return
	}
	settings := session.openSettingsSnapshot()
	info := plugin.PeerInfo{
		PeerOwner:    p.sourceOwner,
		ConnectionID: connectionIDOf(session),
		Address:      settings.Address, AddressStr: p.addrString,
		LocalAddress: settings.LocalAddress, LocalAddressStr: p.localAddrString,
		Name: settings.Name, GroupName: settings.GroupName,
		LocalAS: settings.LocalAS, RouterID: settings.RouterID,
		State:                p.State().PluginState(),
		RSClient:             settings.RSClient,
		RouteReflectorClient: settings.RouteReflectorClient,
	}
	if !settings.IsDynamic {
		info.PeerAS = rpc.RemoteASOf(settings.PeerAS)
		info.IsIBGP = settings.IsIBGP()
	}
	if transport := session.transport.Load(); transport != nil {
		if transport.local.IsValid() {
			info.LocalAddress = transport.local
			info.LocalAddressStr = transport.localString
		}
	}
	reactor.eventDispatcher.OnPeerConnected(&info)
}

// emitFSMEvents reports only an FSM transition. TCP success has its own
// Session callback because DelayOpen may keep the FSM in Connect or Active.
// It runs without the FSM lock or p.mu held, in transition order.
func (p *Peer) emitFSMEvents(from, to fsm.State) {
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
	dispatcher.OnPeerFSMChange(&peerInfo, from, to)
}

// fsmEventPeerInfo is the peer as an FSM event names it. The local address is
// the connected endpoint once a session holds a connection, which is the
// address the kernel chose when the configured one is "auto".
func fsmEventPeerInfo(peer *Peer) plugin.PeerInfo {
	s := peer.Settings()
	local, localString := connectedLocalEndpoint(peer, s.LocalAddress, peer.localAddrString)
	peerAS := peer.PeerAS()
	return plugin.PeerInfo{
		PeerOwner:       peer.sourceOwner,
		Address:         s.Address,
		LocalAddress:    local,
		AddressStr:      peer.addrString,
		LocalAddressStr: localString,
		Name:            s.Name,
		GroupName:       s.GroupName,
		LocalAS:         s.LocalAS,
		// PeerAS via the guarded accessor: a dynamic peer's establishment
		// writes it, and a transition can be drained on another goroutine.
		PeerAS:               rpc.RemoteASOf(peerAS),
		RouterID:             s.RouterID,
		State:                peer.State().PluginState(),
		IsIBGP:               s.isIBGPWith(peerAS),
		RSClient:             s.RSClient,
		RouteReflectorClient: s.RouteReflectorClient,
	}
}
