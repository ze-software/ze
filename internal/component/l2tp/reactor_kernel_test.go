// Design: docs/architecture/l2tp/subscriber-session-model.md -- L2TP consumer of the family release
//
// Goal: prove the L2TP reactor's consumer of a PPP family release keeps the
// L2TP session's own address, the one `show l2tp sessions` prints, equal to
// the families that are open, and publishes the L2TP release notice. Method:
// a real PPP driver session assigns IPv4 through handlePPPEvent; releases of
// each family then go through the same entry point, and the reactor's
// snapshot (LookupSession) and the bus are read.

package l2tp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestReactorFamilyReleaseClearsAssignedAddr: an IPv6 release leaves the
// snapshot's AssignedAddr; an IPv4 release empties it; the next IPv4
// assignment sets it again. Each release publishes (l2tp, session-ip-released)
// with its family. Fails before the release case: the reactor had none.
func TestReactorFamilyReleaseClearsAssignedAddr(t *testing.T) {
	const tunnelID uint16 = 64131
	const sessionID uint16 = 64132
	peer := netip.MustParseAddr("192.0.2.61")
	reactor := newReactorForSnapshot(t)
	bus := newTestBus()
	reactor.SetEventBus(bus)
	session := &L2TPSession{localSID: sessionID, state: L2TPSessionEstablished, pppInterface: "ppp-release"}
	insertEstablishedTunnel(t, reactor, tunnelID, 201, netip.MustParseAddrPort("192.0.2.1:1701"), session)
	wire := newCurrentCoreAdmission(t, reactor, &currentCoreAdmissionConfig{
		ids: currentCoreSessionIDs{tunnelID: tunnelID, sessionID: sessionID},
	})
	reactor.handlePPPEvent(wire.opened)
	wire.prepareNetwork(t, "family-release", ppp.IPResponseArgs{
		Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: peer,
	}, [8]byte{})
	assignment := wire.assign(t, ppp.AddressFamilyIPv4)
	reactor.handlePPPEvent(assignment)

	var released []l2tpevents.AddressFamily
	t.Cleanup(l2tpevents.SessionIPReleased.Subscribe(bus, func(p *l2tpevents.SessionIPReleasedPayload) {
		if p.Birth == session.Birth() {
			released = append(released, p.Family)
		}
	}))
	assigned := func() netip.Addr {
		snapshot, found := reactor.LookupSession(sessionID)
		require.True(t, found)
		return snapshot.AssignedAddr
	}
	require.Equal(t, peer, assigned(), "the IPv4 assignment sets assigned-addr")

	release := ppp.EventSessionIPReleased{
		Birth: assignment.Birth, SubscriberID: assignment.SubscriberID, Control: assignment.Control,
		NetworkLifetime: assignment.NetworkLifetime, TunnelID: tunnelID, SessionID: sessionID,
	}
	release.Family = ppp.AddressFamilyIPv6
	reactor.handlePPPEvent(release)
	require.Equal(t, peer, assigned(), "an IPv6 release keeps the IPv4 assigned-addr")

	release.Family = ppp.AddressFamilyIPv4
	reactor.handlePPPEvent(release)
	require.False(t, assigned().IsValid(), "an IPv4 release empties assigned-addr")
	require.Equal(t, []l2tpevents.AddressFamily{l2tpevents.AddressFamilyIPv6, l2tpevents.AddressFamilyIPv4}, released)

	// The reinstall notice is the one onNCPOpened publishes on a reopen; it
	// carries no acknowledgment channel, as the original's was already used.
	reactor.handlePPPEvent(ppp.EventSessionIPAssigned{
		Birth: assignment.Birth, SubscriberID: assignment.SubscriberID, Control: assignment.Control,
		NetworkLifetime: assignment.NetworkLifetime, TunnelID: tunnelID, SessionID: sessionID,
		Family: ppp.AddressFamilyIPv4, Local: assignment.Local, Peer: peer,
	})
	require.Equal(t, peer, assigned(), "the IPv4 reinstall sets assigned-addr again")
}
