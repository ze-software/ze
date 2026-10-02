// Design: docs/guide/graceful-restart.md -- Receiving Speaker procedures
// RFC: rfc/short/rfc4724.md -- Section 4.2, consecutive restarts
// Overview: gr.go -- handleStateEvent, the session-down sequence
// Related: rfc4724_dispatch_test.go -- rfc4724RetainingPlugin

package gr

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc4724SecondRestart drives a consecutive restart: the peer's session drops, is
// re-established with the Forwarding State bit set for both families (so the stale routes
// stay, waiting for End-of-RIB), and drops again before any End-of-RIB. It returns the
// commands the second drop dispatches.
func rfc4724SecondRestart(t *testing.T) []string {
	t.Helper()
	gp, sent := rfc4724RetainingPlugin(t, testCap(120, famIPv4, famIPv6))
	gp.handleStateEvent(testPeer, map[string]any{"state": "up"})
	require.Empty(t, sent(), "re-established with F set: the stale routes are kept")

	gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})
	return sent()
}

// TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes drops the session a second
// time while routes from the first restart are still marked stale.
//
// VALIDATES: RFC 4724 Section 4.2 -- "To deal with possible consecutive restarts, a route
// (from the peer) previously marked as stale MUST be deleted." The second drop dispatches
// "request bgp rib purge-stale <peer>" for the whole peer; TestRIBPurgeStaleCommand
// (plugins/rib) proves that command deletes every stale route of the peer.
// PREVENTS: a second restart that re-marks the old stale routes and so keeps them for a
// second Restart Time.
//
// RFC requirement: RFC4724-4.2-4 positive -- on a second session drop before any End-of-RIB, the plugin dispatches purge-stale for the whole peer.
func TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes(t *testing.T) {
	got := rfc4724SecondRestart(t)

	assert.Contains(t, got, "request bgp rib purge-stale "+testPeer,
		"the routes still stale from the first restart are deleted")
}

// TestRFC4724ConsecutiveRestartDeletesBeforeMarking reads the order of the second drop's
// commands.
//
// VALIDATES: RFC 4724 Section 4.2 -- the deletion applies to routes "previously marked as
// stale": it runs before the new mark-stale, so the routes the peer refreshed in the
// re-established session are retained and marked for the new restart, not deleted with the
// old ones (TestRIBPurgeStalePreservesFresh, plugins/rib, proves purge-stale spares a route
// not marked stale).
// PREVENTS: mark-stale first and purge-stale second, which deletes every route of the peer on
// a consecutive restart.
//
// RFC requirement: RFC4724-4.2-4 negative -- on the second drop, purge-stale for the peer is dispatched before retain-routes and before mark-stale, so it never reaches the routes marked for the new restart.
func TestRFC4724ConsecutiveRestartDeletesBeforeMarking(t *testing.T) {
	got := rfc4724SecondRestart(t)

	purge := slices.Index(got, "request bgp rib purge-stale "+testPeer)
	retain := slices.Index(got, "request bgp rib retain-routes "+testPeer)
	mark := slices.Index(got, "request bgp rib mark-stale "+testPeer+" 120")
	require.NotEqual(t, -1, purge, "the previously stale routes are deleted")
	require.NotEqual(t, -1, retain, "the second drop retains the routes")
	require.NotEqual(t, -1, mark, "the second drop marks the routes stale")
	assert.Less(t, purge, retain, "the old stale routes go before the routes are retained")
	assert.Less(t, purge, mark, "the old stale routes go before the new marking")
}
