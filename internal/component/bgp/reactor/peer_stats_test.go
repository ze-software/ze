package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPeerWithMetrics creates a Peer wired to a spy metrics registry.
// Returns the peer (addr "192.0.2.1") and the spy registry for assertions.
func newPeerWithMetrics() (*Peer, *spyRegistry) {
	reg := newSpyRegistry()
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)
	peer.reactor = &Reactor{
		rmetrics: initReactorMetrics(reg, "test", "1.2.3.4", "65000"),
	}
	return peer, reg
}

// TestPeerStatsInitialZero verifies counters start at zero.
//
// VALIDATES: New peers have zero update, keepalive, and EOR counters.
// PREVENTS: Counters starting with garbage values.
func TestPeerStatsInitialZero(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	stats := peer.Stats()
	assert.Equal(t, uint32(0), stats.UpdatesReceived)
	assert.Equal(t, uint32(0), stats.UpdatesSent)
	assert.Equal(t, uint32(0), stats.KeepalivesReceived)
	assert.Equal(t, uint32(0), stats.KeepalivesSent)
	assert.Equal(t, uint32(0), stats.EORReceived)
	assert.Equal(t, uint32(0), stats.EORSent)
}

// TestPeerStatsIncrement verifies counter increment methods.
//
// VALIDATES: IncrUpdatesReceived/Sent, IncrKeepalivesReceived/Sent, IncrEORReceived/Sent update counters.
// PREVENTS: Counters not being updated or updating the wrong counter.
func TestPeerStatsIncrement(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	peer.IncrUpdatesReceived()
	peer.IncrUpdatesReceived()
	peer.IncrUpdatesSent()
	peer.IncrKeepalivesReceived()
	peer.IncrKeepalivesReceived()
	peer.IncrKeepalivesReceived()
	peer.IncrKeepalivesSent()
	peer.IncrKeepalivesSent()
	peer.IncrEORReceived()
	peer.IncrEORSent()

	stats := peer.Stats()
	assert.Equal(t, uint32(2), stats.UpdatesReceived)
	assert.Equal(t, uint32(1), stats.UpdatesSent)
	assert.Equal(t, uint32(3), stats.KeepalivesReceived)
	assert.Equal(t, uint32(2), stats.KeepalivesSent)
	assert.Equal(t, uint32(1), stats.EORReceived)
	assert.Equal(t, uint32(1), stats.EORSent)
}

// TestPeerEstablishedAt verifies per-peer uptime tracking.
//
// VALIDATES: SetEstablishedNow records a timestamp, EstablishedAt returns it.
// PREVENTS: Uptime using reactor start time instead of per-peer session time.
func TestPeerEstablishedAt(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	require.True(t, peer.EstablishedAt().IsZero(), "should be zero before establishment")

	peer.SetEstablishedNow()

	require.False(t, peer.EstablishedAt().IsZero(), "should be non-zero after establishment")
}

// TestPeerStatsClearOnReset verifies counters can be cleared.
//
// VALIDATES: ClearStats resets all counters and established timestamp.
// PREVENTS: Stale counters surviving session reset.
func TestPeerStatsClearOnReset(t *testing.T) {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	peer := NewPeer(settings)

	peer.IncrUpdatesReceived()
	peer.IncrUpdatesSent()
	peer.IncrKeepalivesReceived()
	peer.IncrKeepalivesSent()
	peer.IncrEORReceived()
	peer.IncrEORSent()
	peer.SetEstablishedNow()

	peer.ClearStats()

	stats := peer.Stats()
	assert.Equal(t, uint32(0), stats.UpdatesReceived)
	assert.Equal(t, uint32(0), stats.UpdatesSent)
	assert.Equal(t, uint32(0), stats.KeepalivesReceived)
	assert.Equal(t, uint32(0), stats.KeepalivesSent)
	assert.Equal(t, uint32(0), stats.EORReceived)
	assert.Equal(t, uint32(0), stats.EORSent)
	assert.True(t, peer.EstablishedAt().IsZero())
}

// TestUpdatePeerStateMetric_EstablishedTransition verifies that transitioning
// to Established increments the sessionsEstablished counter and sets the
// peerState gauge.
//
// VALIDATES: updatePeerStateMetric increments sessionsEstablished on transition to Established.
// PREVENTS: Established transitions not being counted in Prometheus metrics.
func TestUpdatePeerStateMetric_EstablishedTransition(t *testing.T) {
	peer, reg := newPeerWithMetrics()
	addr := peer.peerAddrLabel()

	peer.updatePeerStateMetric(PeerStateActive, PeerStateEstablished)

	// peerState gauge should reflect Established (3).
	peerStateVec := reg.gaugeVec("ze_peer_state")
	require.NotNil(t, peerStateVec)
	g := peerStateVec.get(addr)
	require.NotNil(t, g, "peerState gauge should be set for peer")
	assert.Equal(t, float64(PeerStateEstablished), g.Value())

	// sessionsEstablished counter should be 1.
	estVec := reg.counterVecs["ze_peer_sessions_established_total"]
	require.NotNil(t, estVec)
	c := estVec.get(addr)
	require.NotNil(t, c, "sessionsEstablished counter should exist for peer")
	assert.Equal(t, 1.0, c.Value())

	// stateTransitions counter should be 1 for Active->Established.
	transVec := reg.counterVecs["ze_peer_state_transitions_total"]
	require.NotNil(t, transVec)
	tc := transVec.get(addr, "Active", "Established")
	require.NotNil(t, tc, "stateTransitions counter should exist for Active->Established")
	assert.Equal(t, 1.0, tc.Value())

	// sessionFlaps should NOT have been incremented.
	flapVec := reg.counterVecs["ze_peer_session_flaps_total"]
	require.NotNil(t, flapVec)
	fc := flapVec.get(addr)
	assert.Nil(t, fc, "sessionFlaps should not be set on transition TO Established")
}

// TestUpdatePeerStateMetric_Flap verifies that transitioning FROM Established
// to a non-Established state increments sessionFlaps and resets sessionDuration.
//
// VALIDATES: updatePeerStateMetric increments sessionFlaps when leaving Established.
// PREVENTS: Session flaps not being tracked, stale duration gauge after flap.
func TestUpdatePeerStateMetric_Flap(t *testing.T) {
	peer, reg := newPeerWithMetrics()
	addr := peer.peerAddrLabel()

	// First reach Established so the counters are primed.
	peer.updatePeerStateMetric(PeerStateActive, PeerStateEstablished)

	// Now flap: Established -> Stopped.
	peer.updatePeerStateMetric(PeerStateEstablished, PeerStateStopped)

	// sessionFlaps should be 1.
	flapVec := reg.counterVecs["ze_peer_session_flaps_total"]
	require.NotNil(t, flapVec)
	fc := flapVec.get(addr)
	require.NotNil(t, fc, "sessionFlaps counter should exist after flap")
	assert.Equal(t, 1.0, fc.Value())

	// sessionDuration should be reset to 0.
	durVec := reg.gaugeVec("ze_peer_session_duration_seconds")
	require.NotNil(t, durVec)
	dg := durVec.get(addr)
	require.NotNil(t, dg, "sessionDuration gauge should exist after flap")
	assert.Equal(t, 0.0, dg.Value())

	// peerState gauge should reflect Stopped (0).
	peerStateVec := reg.gaugeVec("ze_peer_state")
	require.NotNil(t, peerStateVec)
	g := peerStateVec.get(addr)
	require.NotNil(t, g)
	assert.Equal(t, float64(PeerStateStopped), g.Value())
}

// TestUpdatePeerStateMetric_NonEstablished verifies that transitions between
// non-Established states do not increment sessionsEstablished or sessionFlaps.
//
// VALIDATES: updatePeerStateMetric only counts established/flap for relevant transitions.
// PREVENTS: False established or flap counts on non-Established state changes.
func TestUpdatePeerStateMetric_NonEstablished(t *testing.T) {
	peer, reg := newPeerWithMetrics()
	addr := peer.peerAddrLabel()

	peer.updatePeerStateMetric(PeerStateStopped, PeerStateConnecting)
	peer.updatePeerStateMetric(PeerStateConnecting, PeerStateActive)
	peer.updatePeerStateMetric(PeerStateActive, PeerStateStopped)

	// sessionsEstablished should not have been touched.
	estVec := reg.counterVecs["ze_peer_sessions_established_total"]
	require.NotNil(t, estVec)
	c := estVec.get(addr)
	assert.Nil(t, c, "sessionsEstablished should not be set for non-Established transitions")

	// sessionFlaps should not have been touched.
	flapVec := reg.counterVecs["ze_peer_session_flaps_total"]
	require.NotNil(t, flapVec)
	fc := flapVec.get(addr)
	assert.Nil(t, fc, "sessionFlaps should not be set when never reaching Established")

	// stateTransitions should have 3 entries.
	transVec := reg.counterVecs["ze_peer_state_transitions_total"]
	require.NotNil(t, transVec)
	assert.NotNil(t, transVec.get(addr, "Stopped", "Connecting"))
	assert.NotNil(t, transVec.get(addr, "Connecting", "Active"))
	assert.NotNil(t, transVec.get(addr, "Active", "Stopped"))

	// peerState gauge should reflect last state: Stopped (0).
	peerStateVec := reg.gaugeVec("ze_peer_state")
	require.NotNil(t, peerStateVec)
	g := peerStateVec.get(addr)
	require.NotNil(t, g)
	assert.Equal(t, float64(PeerStateStopped), g.Value())
}

// TestIncrNotificationSent verifies that IncrNotificationSent increments
// both the notifSent counter (with code/subcode labels) and peerMsgSent.
//
// VALIDATES: IncrNotificationSent records notification code, subcode, and message type.
// PREVENTS: Notification send events missing from metrics, wrong label mapping.
func TestIncrNotificationSent(t *testing.T) {
	peer, reg := newPeerWithMetrics()
	addr := peer.peerAddrLabel()

	// Send a "cease" notification (code=6, subcode=2).
	peer.IncrNotificationSent(6, 2)

	// notifSent counter should be 1 with correct labels.
	notifVec := reg.counterVecs["ze_peer_notifications_sent_total"]
	require.NotNil(t, notifVec)
	nc := notifVec.get(addr, "cease", "2")
	require.NotNil(t, nc, "notifSent counter should exist with cease/2 labels")
	assert.Equal(t, 1.0, nc.Value())

	// peerMsgSent counter should be 1 for "notification" type.
	msgVec := reg.counterVecs["ze_peer_messages_sent_total"]
	require.NotNil(t, msgVec)
	mc := msgVec.get(addr, "notification")
	require.NotNil(t, mc, "peerMsgSent counter should exist for notification type")
	assert.Equal(t, 1.0, mc.Value())

	// Unknown code should map to "other".
	peer.IncrNotificationSent(255, 0)
	oc := notifVec.get(addr, "other", "0")
	require.NotNil(t, oc, "unknown code should map to 'other'")
	assert.Equal(t, 1.0, oc.Value())
}

// TestIncrNotificationReceived verifies that IncrNotificationReceived increments
// both the notifRecv counter (with code/subcode labels) and peerMsgRecv.
//
// VALIDATES: IncrNotificationReceived records notification code, subcode, and message type.
// PREVENTS: Notification receive events missing from metrics, wrong label mapping.
func TestIncrNotificationReceived(t *testing.T) {
	peer, reg := newPeerWithMetrics()
	addr := peer.peerAddrLabel()

	// Receive an "open" notification (code=2, subcode=4).
	peer.IncrNotificationReceived(2, 4)

	// notifRecv counter should be 1 with correct labels.
	notifVec := reg.counterVecs["ze_peer_notifications_received_total"]
	require.NotNil(t, notifVec)
	nc := notifVec.get(addr, "open", "4")
	require.NotNil(t, nc, "notifRecv counter should exist with open/4 labels")
	assert.Equal(t, 1.0, nc.Value())

	// peerMsgRecv counter should be 1 for "notification" type.
	msgVec := reg.counterVecs["ze_peer_messages_received_total"]
	require.NotNil(t, msgVec)
	mc := msgVec.get(addr, "notification")
	require.NotNil(t, mc, "peerMsgRecv counter should exist for notification type")
	assert.Equal(t, 1.0, mc.Value())

	// Unknown code should map to "other".
	peer.IncrNotificationReceived(99, 7)
	oc := notifVec.get(addr, "other", "7")
	require.NotNil(t, oc, "unknown code should map to 'other'")
	assert.Equal(t, 1.0, oc.Value())
}
