// Design: docs/architecture/behavior/peer-lifecycle.md -- one Session per TCP connection.
// Related: peer_run.go -- publication of the selected Session.
// Related: incoming_candidate.go -- independent incoming handshake ownership.
// RFC 4271 Section 8.2.1 -- see rfc/short/rfc4271.md.
package reactor

import (
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// newSession prepares immutable connection inputs before either TCP rail runs.
// Every Session of this Peer, a pending incoming candidate included, MUST be
// constructed here. Publication and route delivery remain runOnce's ownership.
func (p *Peer) newSession() *Session {
	return p.newSessionWithEpoch(selectedNegotiationEpoch{})
}

func (p *Peer) newIncomingSession() *Session {
	return p.newSessionWithEpoch(newPrivateNegotiationEpoch())
}

func (p *Peer) newSessionWithEpoch(epoch negotiationEpoch) *Session {
	p.mu.RLock()
	session := NewSession(p.settings)
	p.mu.RUnlock()
	session.negotiationEpoch = epoch
	session.bfdOptIn = p.bfdSettings
	session.typedNLRIHeld = &p.typedNLRIHeld
	session.SetClock(p.clock)
	session.SetDialer(p.dialer)
	session.SetSourceID(p.sourceID)
	session.SetConnectRetryCounter(&p.connectRetryCounter)
	session.setPluginCapabilitySnapshotGetter(p.pluginCapabilitySnapshot)
	session.setSettingsSnapshotGetter(p.settingsSnapshot)
	session.setPluginFamiliesGetter(p.getPluginFamilies)
	session.setOpenValidator(func(address string, local, remote *message.Open) error {
		return p.validateOpen(session, address, remote)
	})
	session.setBFDStateReader(p.bfdSessionState)
	session.adjOut = &p.adjOut
	session.asMigrationFallback = &p.asMigrationFallback
	session.onMessageReceived = session.sourceMessageCallback(p.messageCallback)
	if private, ok := epoch.(*privateNegotiationEpoch); ok {
		session.onMessageReceived = private.messageCallback(session.onMessageReceived)
	}
	session.aigpReactor = p.reactor
	session.aigpPeer = p
	session.egressFinalAccepts = func() bool {
		if facts := p.forwardFacts(); facts != nil {
			// RFC 8212 Section 3.
			return facts.exportPolicy.AcceptsAtEnd(facts.isEBGP)
		}
		p.mu.RLock()
		defer p.mu.RUnlock()
		if p.settings.PeerAS == 0 {
			return false
		}
		return p.settings.ExportPolicy.AcceptsAtEnd(p.settings.IsEBGP())
	}
	if p.reactor != nil {
		session.egressRouteFilter = p.egressGate(session)
		session.prefixMetrics = p.reactor.rmetrics
	}
	// All timer/writer callbacks and capture inputs are immutable once TCP
	// starts. Selection activates a file without replacing the reader's input.
	p.attachStatsCallbacks(session)
	p.prepareSessionCapture(session)
	session.onTCPConnected = func() { p.emitConnected(session) }
	if p.reactor != nil {
		settings := p.settingsSnapshot()
		wirePeer := plugin.PeerInfo{
			Address: settings.Address, AddressStr: p.addrString,
			LocalAddress: settings.LocalAddress, LocalAddressStr: p.localAddrString,
			LocalAS: settings.LocalAS, PeerAS: rpc.RemoteASOf(settings.PeerAS), RouterID: settings.RouterID,
		}
		session.onWireMessage = func(wire []byte, ctxID bgpctx.ContextID, sent bool, endpoints *sessionTransport) {
			p.reactor.dispatchObservedWire(&wirePeer, wire, ctxID, sent, endpoints)
		}
	}
	return session
}
