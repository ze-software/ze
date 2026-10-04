// Design: docs/guide/graceful-restart.md -- Receiving Speaker procedures
// RFC: rfc/short/rfc4724.md -- Section 4.2, consecutive restarts
// Overview: gr.go -- handleStateEvent, the session-down sequence
package gr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Drive both drops through GR's production callbacks and the real RIB engine.
// Refresh only the second route between drops, leaving the first genuinely stale.
func rfc4724SecondRestart(t *testing.T) *realGRRIB {
	t.Helper()
	gp, rib := newGRWithRealRIB(t)
	gp.peerCaps[testPeer] = testCap(120, famIPv4, famIPv6)
	rib.received("18c6336418c63365", grReceivedAttrs)
	gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})
	rib.down()
	rib.requireStale("198.51.100.0/24", 1)
	rib.requireStale("198.51.101.0/24", 1)
	gp.handleStateEvent(testPeer, map[string]any{"state": "up"})
	rib.up(testPeer)
	rib.received("18c63365", grReceivedAttrs)
	gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})
	rib.down()
	return rib
}

// VALIDATES: RFC 4724 Section 4.2 -- "a route (from the peer) previously marked
// as stale MUST be deleted" on a consecutive restart.
// PREVENTS: retaining the old generation for a second Restart Time.
//
// RFC requirement: RFC4724-4.2-4 positive -- a second session drop before End-of-RIB deletes the route still stale from the first restart.
func TestRFC4724ConsecutiveRestartDeletesThePreviouslyStaleRoutes(t *testing.T) {
	rib := rfc4724SecondRestart(t)
	require.Empty(t, rib.routes("198.51.100.0/24"))
	rib.requireStale("198.51.101.0/24", 1)
}

// VALIDATES: deletion precedes the new marking: the refreshed route survives
// and is retained as stale for the new restart, unlike its unrefreshed sibling.
// PREVENTS: marking first and then purging every route, including fresh ones.
//
// RFC requirement: RFC4724-4.2-4 negative -- on the second drop, deletion of previously stale routes does not reach routes refreshed before the new restart.
func TestRFC4724ConsecutiveRestartDeletesBeforeMarking(t *testing.T) {
	rib := rfc4724SecondRestart(t)
	rib.requireStale("198.51.101.0/24", 1)
	require.Empty(t, rib.routes("198.51.100.0/24"))
}
