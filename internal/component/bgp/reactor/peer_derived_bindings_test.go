package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// setSessionFacts gives the test session what the read path and the OPEN
// exchange would: whether the peer announced a route, and whether it
// advertised Route Refresh.
func setSessionFacts(peer *Peer, receivedRoute, routeRefresh bool) {
	peer.session.receivedRoute.Store(receivedRoute)
	peer.session.mu.Lock()
	peer.session.negotiated = &capability.Negotiated{RouteRefresh: routeRefresh}
	peer.session.mu.Unlock()
	peer.negotiated.Load().RouteRefresh = routeRefresh
}

// withDerivedRIBFeed returns a copy of settings carrying the derived bgp-rib
// feed the config builder adds for RFC 7311 AIGP selection, built by the real
// producer (EnsureProcessBinding).
func withDerivedRIBFeed(t *testing.T, settings *PeerSettings) *PeerSettings {
	t.Helper()
	next := *settings
	next.ProcessBindings = append([]ProcessBinding(nil), settings.ProcessBindings...)
	require.NoError(t, EnsureProcessBinding(&next, "bgp-rib", "update-received state", ""))
	return &next
}

// TestDerivedFeedSwapKeepsSessionAndCatchesUp proves the reload path for a
// derived feed-only binding: the session stays up, the binding reaches the
// running peer and its delivery edges, and the peer is asked to re-send what it
// already sent so the new consumer learns it.
//
// VALIDATES: for a session that announced routes, peerSettingsSwapPlan judges
// the change swappable, and
// applyHotSwappableSettings writes one ROUTE-REFRESH per negotiated family on
// the SAME session (RFC 2918 Section 3).
// PREVENTS: adding the first iBGP peer restarting every established session
// (reload-dynamic-peer-survives.ci), and a new RIB consumer that holds none of
// the routes the peer sent before the reload.
func TestDerivedFeedSwapKeepsSessionAndCatchesUp(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, family.IPv6Unicast)
	setSessionFacts(peer, true, true)
	session := peer.session

	next := withDerivedRIBFeed(t, peer.settings)
	copier, reason := peerSettingsSwapPlan(peer.settings, next, session)
	require.Empty(t, reason, "a derived feed-only binding must not restart the session")
	require.NotNil(t, copier)

	peer.applyHotSwappableSettings(next, copier)

	assert.Same(t, session, peer.session, "the running session must survive the swap")
	assert.Equal(t, PeerStateEstablished, peer.State())
	require.Len(t, peer.settings.ProcessBindings, 1)
	assert.Equal(t, "bgp-rib", peer.settings.ProcessBindings[0].PluginName)
	delivery := deliveryPeersFromSettings([]*PeerSettings{peer.settings})
	require.Len(t, delivery[0].Bindings, 1, "the delivery graph input must carry the new feed")

	frames := bgpFrames(t, conn.written())
	require.Len(t, frames, 2, "one ROUTE-REFRESH per negotiated family")
	for i, afi := range []byte{1, 2} {
		frame := frames[i]
		require.Len(t, frame, 23, "a ROUTE-REFRESH is a header and four octets")
		assert.Equal(t, byte(5), frame[18], "message type ROUTE-REFRESH")
		assert.Equal(t, []byte{0, afi}, frame[19:21], "AFI")
		assert.Equal(t, byte(0), frame[21], "normal route refresh subtype")
		assert.Equal(t, byte(1), frame[22], "SAFI unicast")
	}
}

// TestDerivedFeedWithoutRoutesNeedsNoCatchUp proves the case
// reload-dynamic-peer-survives.ci reaches: a peer that announced no route and
// did not advertise Route Refresh owes a new consumer nothing, so the session
// stays up and the peer is asked for nothing.
//
// VALIDATES: Session.feedCatchUp answers feedCatchUpNothing before it looks
// at the capability.
// PREVENTS: a session that sent no route restarting on a derived feed.
func TestDerivedFeedWithoutRoutesNeedsNoCatchUp(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, false, false)
	session := peer.session

	next := withDerivedRIBFeed(t, peer.settings)
	copier, reason := peerSettingsSwapPlan(peer.settings, next, session)
	require.Empty(t, reason, "a session with no route has nothing to catch up")
	peer.applyHotSwappableSettings(next, copier)

	assert.Same(t, session, peer.session, "the running session must survive the swap")
	require.Len(t, peer.settings.ProcessBindings, 1)
	assert.Empty(t, conn.written(), "nothing to re-send, so nothing is asked")
}

// TestDerivedFeedRouteArrivingBeforeApplyRestarts proves the apply re-decides:
// the plan saw no route, the peer announced one before the apply, and it cannot
// re-send it, so the session is ended with RFC 4486 Section 4 subcode 6 "Other
// Configuration Change" rather than leave the new consumer without that route.
func TestDerivedFeedRouteArrivingBeforeApplyRestarts(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.sendingInitialRoutes.Store(0)
	setSessionFacts(peer, false, false)

	next := withDerivedRIBFeed(t, peer.settings)
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.session.receivedRoute.Store(true)
	peer.applyHotSwappableSettings(next, copier)

	frames := bgpFrames(t, conn.written())
	require.Len(t, frames, 1, "one NOTIFICATION and no ROUTE-REFRESH")
	assert.Equal(t, byte(3), frames[0][18], "message type NOTIFICATION")
	assert.Equal(t, []byte{6, 6}, frames[0][19:21], "Cease, Other Configuration Change")
}

// TestNoteReceivedRouteCountsAnnouncementsOnly proves the flag the catch-up
// reads: an announcement sets it, and a withdrawal or an End-of-RIB, which
// leave no route for a consumer to hold, do not.
func TestNoteReceivedRouteCountsAnnouncementsOnly(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"end-of-rib", []byte{0, 0, 0, 0}, false},
		{"withdrawal", []byte{0, 4, 24, 10, 0, 0, 0, 0}, false},
		{"announcement", []byte{0, 0, 0, 0, 24, 10, 0, 0}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := NewSession(&PeerSettings{})
			session.noteReceivedRoute(wireu.NewWireUpdate(tc.payload, 0))
			assert.Equal(t, tc.want, session.receivedRoute.Load())
		})
	}
}

// TestDerivedFeedWithoutRouteRefreshRestarts proves the fail-closed half: a
// peer that announced routes and did not advertise Route Refresh cannot re-send
// them to a new consumer, so the change restarts the session rather than leave
// the consumer with half the peer's routes.
func TestDerivedFeedWithoutRouteRefreshRestarts(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, false)

	_, reason := peerSettingsSwapPlan(peer.settings, withDerivedRIBFeed(t, peer.settings), peer.session)
	assert.Equal(t, "ProcessBindings", reason)
}

// TestOperatorBindingChangeStillRestarts proves the swap stays narrow: a binding
// the operator typed is not derived, so a change to it restarts as before.
func TestOperatorBindingChangeStillRestarts(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	next := withDerivedRIBFeed(t, peer.settings)
	next.ProcessBindings[0].Derived = false

	_, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	assert.Equal(t, "ProcessBindings", reason)
}

// TestDerivedFeedRemovalSendsNoRefresh proves the removal half: the lost
// consumer is named for the targeted "down" (catchUpDerivedFeeds), and the peer
// is not asked to re-send anything.
func TestDerivedFeedRemovalSendsNoRefresh(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, true)
	peer.settings.ProcessBindings = withDerivedRIBFeed(t, peer.settings).ProcessBindings

	next := *peer.settings
	next.ProcessBindings = nil
	gained, lost := derivedFeedChange(peer.settings.ProcessBindings, next.ProcessBindings)
	assert.Empty(t, gained)
	assert.Equal(t, []string{"bgp-rib"}, lost)

	copier, reason := peerSettingsSwapPlan(peer.settings, &next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(&next, copier)
	assert.Empty(t, peer.settings.ProcessBindings)
	assert.Empty(t, conn.written(), "a removed feed asks the peer for nothing")
}
