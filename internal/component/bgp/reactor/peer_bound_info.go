// Design: docs/architecture/behavior/peer-lifecycle.md -- captured hot-binding state.
// Related: peer_derived_bindings.go -- bindProcesses owns ordered targeted UP.
package reactor

import (
	"github.com/ze-software/ze/internal/component/plugin"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// boundPeerInfo names only the Session whose negotiation bindProcesses reads.
// The caller MUST pass its captured Session, including after replacement.
// Unlike a normal UP, a targeted attach MUST carry no InitialReplay receipt.
// Nil explicitly means no physical connection and grants no selected output.
func boundPeerInfo(peer *Peer, session *Session) plugin.PeerInfo {
	settings := peer.settingsSnapshot()
	// Own only the names, not the graph's read-only slice. A known empty
	// capture differs from a synthetic PeerInfo that supplied no grant facts.
	senders := peer.reactor.DeliveryGraph().SendersForPeer(peer.addrString, bgpevents.SendUpdate)
	updateSendGrants := make([]string, len(senders))
	copy(updateSendGrants, senders)
	peerAS := peer.PeerAS()
	info := plugin.PeerInfo{
		PeerOwner: peer.sourceOwner,
		Address:   settings.Address, AddressStr: peer.addrString,
		LocalAddress: settings.LocalAddress, LocalAddressStr: peer.localAddrString,
		Name: settings.Name, GroupName: settings.GroupName,
		LocalAS:              settings.LocalAS,
		PeerAS:               rpc.RemoteASOf(peerAS),
		RouterID:             settings.RouterID,
		Connect:              settings.Connection.Connect,
		Accept:               settings.Connection.Accept,
		State:                plugin.PeerStateEstablished,
		IsIBGP:               settings.isIBGPWith(peerAS),
		RSClient:             settings.RSClient,
		RouteReflectorClient: settings.RouteReflectorClient,
		UpdateSendGrants:     updateSendGrants,
	}
	if session == nil {
		return info
	}
	info.ConnectionID = session.connectionID
	info.SessionGeneration = session.publishedGeneration.Load()
	info.DestinationSession, info.SelectedOwner = peer.selectedDestinationForSession(session)
	session.mu.RLock()
	activeSettings := settings
	if session.proposal != nil {
		if session.proposal.settings != nil {
			activeSettings = session.proposal.settings
			info.LocalAS = session.proposal.settings.LocalAS
		}
		info.RouterID = session.proposal.message.BGPIdentifier
	}
	if session.negotiated != nil {
		info.PeerAS = rpc.RemoteASOf(session.negotiated.PeerASN)
		info.IsIBGP = activeSettings.isIBGPWith(session.negotiated.PeerASN)
	}
	if session.peerOpen != nil {
		info.RemoteRouterID = session.peerOpen.BGPIdentifier
	}
	session.mu.RUnlock()
	if transport := session.transport.Load(); transport != nil {
		if transport.local.IsValid() {
			info.LocalAddress = transport.local
			info.LocalAddressStr = transport.localString
		}
		info.LocalPort = transport.localPort
		info.RemotePort = transport.remotePort
	}
	return info
}
