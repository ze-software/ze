package gr

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// removalTestPeer is the peer address used across the removal tests.
const removalTestPeer = "10.0.0.1"

// scrapeMetrics renders the registry's /metrics text so tests can assert on the
// presence or absence of a specific per-peer series (behavioral, not count-only).
func scrapeMetrics(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, req)
	return w.Body.String()
}

// newGRPluginWithGRCap builds a grPlugin with a stored GR capability for
// removalTestPeer, mirroring the state after an OPEN with graceful-restart.
func newGRPluginWithGRCap() *grPlugin {
	gp := &grPlugin{
		peerCaps:     make(map[string]*grPeerCap),
		peerLLGRCaps: make(map[string]*llgrPeerCap),
		removedPeers: make(map[string]bool),
		state:        newGRStateManager(nil),
	}
	gp.peerCaps[removalTestPeer] = &grPeerCap{
		RestartTime: 120,
		Families:    []grCapFamily{{Family: family.IPv4Unicast, ForwardState: true}},
	}
	return gp
}

// seedPeerMetricLabels creates the per-peer GR series (as a prior restart-timer
// expiry and a stale-route gauge would), so a later removal can be observed to
// delete them.
func seedPeerMetricLabels() {
	if m := grMetricsPtr.Load(); m != nil {
		m.timerExpired.With(removalTestPeer).Inc()
		m.staleRoutes.With(removalTestPeer).Set(3)
	}
}

// TestHandleEventStateRemoved_SkipsActivationAndDeletesLabels verifies that a
// SessionStateDown with reason "removed" (peer deconfigured) makes GR release the
// peer instead of activating route retention, and deletes the peer's per-peer
// Prometheus series so removed peers do not linger in /metrics.
//
// VALIDATES: handleStateEvent on down+reason="removed" -> onPeerRemoved (no GR
// activation, per-peer labels deleted). Fixes deferral L27.
// PREVENTS: GR retaining routes for a deconfigured peer, and ze_gr_* series
// leaking per removed peer (GR has no other peer-removal callback).
func TestHandleEventStateRemoved_SkipsActivationAndDeletesLabels(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetMetricsRegistry(reg)

	gp := newGRPluginWithGRCap()
	seedPeerMetricLabels()

	before := scrapeMetrics(t, reg)
	require.Contains(t, before, `ze_gr_timer_expired_total{peer="10.0.0.1"}`,
		"precondition: per-peer timer_expired series should exist before removal")

	event := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"10.0.0.1","as":65001}},"state":"down","reason":"removed"}}`
	require.NoError(t, gp.handleEvent(event))

	assert.False(t, gp.state.peerActive("10.0.0.1"),
		"removed peer must NOT activate GR route retention")

	after := scrapeMetrics(t, reg)
	assert.NotContains(t, after, `ze_gr_timer_expired_total{peer="10.0.0.1"}`,
		"per-peer timer_expired series must be deleted on peer removal")
	assert.NotContains(t, after, `ze_gr_stale_routes{peer="10.0.0.1"}`,
		"per-peer stale_routes series must be deleted on peer removal")
}

// TestStateRemovedTombstonePreventsLaterActivation verifies that once a peer is
// removed, a subsequent (racing) session-down event for that same peer does not
// resurrect GR state. removePeer and the FSM teardown can both emit a down; the
// removal must win regardless of delivery order.
//
// VALIDATES: onPeerRemoved records a tombstone that suppresses activation on the
// next down for that peer.
// PREVENTS: a late "connection lost" down re-activating GR (and recreating the
// metric series) for a peer that was just deconfigured.
func TestStateRemovedTombstonePreventsLaterActivation(t *testing.T) {
	gp := newGRPluginWithGRCap()

	removed := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"10.0.0.1","as":65001}},"state":"down","reason":"removed"}}`
	require.NoError(t, gp.handleEvent(removed))
	require.False(t, gp.state.peerActive("10.0.0.1"))

	// A racing teardown down arrives after removal.
	lateDown := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"10.0.0.1","as":65001}},"state":"down","reason":"connection lost"}}`
	require.NoError(t, gp.handleEvent(lateDown))

	assert.False(t, gp.state.peerActive("10.0.0.1"),
		"a down after removal must not re-activate GR for the removed peer")
}

// TestStructuredStateRemoved_SkipsActivation verifies the structured (DirectBridge)
// delivery path also treats reason "removed" as terminal.
//
// VALIDATES: handleStructuredState on down+reason="removed" -> onPeerRemoved.
// PREVENTS: the structured path diverging from the JSON path on peer removal.
func TestStructuredStateRemoved_SkipsActivation(t *testing.T) {
	gp := newGRPluginWithGRCap()

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindState,
		State:       rpc.SessionStateDown,
		Reason:      rpc.ReasonPeerRemoved,
		RawMessage:  &bgptypes.RawMessage{},
	}
	gp.handleStructuredEvent(se)

	assert.False(t, gp.state.peerActive("10.0.0.1"),
		"structured removed down must NOT activate GR")
}

// TestReAddAfterRemovalActivatesGR verifies the tombstone is cleared when a
// removed peer comes back up, so a genuine later restart still gets GR.
//
// VALIDATES: onSessionReestablished (state "up") clears the removal tombstone.
// PREVENTS: a re-added peer being permanently denied GR because of a stale
// tombstone from a prior removal.
func TestReAddAfterRemovalActivatesGR(t *testing.T) {
	gp := newGRPluginWithGRCap()

	removed := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"10.0.0.1","as":65001}},"state":"down","reason":"removed"}}`
	require.NoError(t, gp.handleEvent(removed))

	// Peer re-added: it re-advertises its GR capability in a fresh OPEN
	// (removal cleared the cached cap), then the session comes up.
	reopen := `{"type":"bgp","bgp":{"message":{"type":"open","direction":"received"},"peer":{"remote":{"address":"10.0.0.1","as":65001}},"open":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[{"code":64,"name":"graceful-restart","value":"00780001018000020180"}]}}}`
	require.NoError(t, gp.handleEvent(reopen))
	up := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"10.0.0.1","as":65001}},"state":"up"}}`
	require.NoError(t, gp.handleEvent(up))

	// A genuine session drop should now activate GR again.
	drop := `{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{"remote":{"address":"10.0.0.1","as":65001}},"state":"down","reason":"tcp-failure"}}`
	require.NoError(t, gp.handleEvent(drop))

	assert.True(t, gp.state.peerActive("10.0.0.1"),
		"after re-add, a real drop must activate GR (tombstone cleared, cap re-advertised)")
}
