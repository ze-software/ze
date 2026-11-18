// Design: docs/architecture/behavior/peer-lifecycle.md -- captured hot-binding state.
// Related: peer_derived_bindings.go -- bindProcesses owns ordered targeted UP.
package reactor

import (
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// boundPeerInfo names only the Session whose negotiation bindProcesses reads.
// The caller MUST pass its captured Session, including after replacement.
// Unlike a normal UP, a targeted attach MUST carry no InitialReplay receipt.
// Nil explicitly means no physical connection and grants no selected output.
func boundPeerInfo(peer *Peer, session *Session) plugin.PeerInfo {
	settings := peer.settingsSnapshot()
	info := plugin.PeerInfo{
		PeerOwner: peer.sourceOwner,
		Address:   settings.Address, AddressStr: peer.addrString,
		LocalAddress: settings.LocalAddress, LocalAddressStr: peer.localAddrString,
		Name: settings.Name, GroupName: settings.GroupName,
		LocalAS:  settings.LocalAS,
		RouterID: settings.RouterID,
		Connect:  settings.Connection.Connect, Accept: settings.Connection.Accept,
		State: plugin.PeerStateEstablished,
	}
	if session == nil {
		return info
	}
	info.ConnectionID = session.connectionID
	info.SessionGeneration = session.publishedGeneration.Load()
	info.DestinationSession, info.SelectedOwner = peer.selectedDestinationForSession(session)
	session.mu.RLock()
	if session.proposal != nil {
		info.LocalAS = session.proposal.settings.LocalAS
		info.RouterID = session.proposal.message.BGPIdentifier
	}
	if session.negotiated != nil {
		info.PeerAS = rpc.RemoteASOf(session.negotiated.PeerASN)
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
