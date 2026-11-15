// Design: docs/architecture/bgp/structural-forwarding.md -- common socket relay proof driver
// Related: rfc4271_atomic_aggregate_relay_proof_test.go -- received NLRI preservation
// Related: next_hop_auto_relay_test.go -- cached and native rail matrix
// Related: rfc7607_egress_test.go -- final outbound AS-zero admission
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Receive through the session-to-cache path and dispatch through the chosen
// relay. The fixture's real fwdBatchHandler commits the Session's Adj-RIB-Out.
func rfc4271ProofRelay(t *testing.T, f *aigpReplayFixture, body []byte, rs bool) {
	t.Helper()
	f.r.rsForwardingEnabled = false
	id := f.receive(t, body)
	if rs {
		update, present := f.r.recentUpdates.Get(id)
		require.True(t, present)
		f.r.rsForwardingEnabled = true
		skipped, dispatched := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
		f.r.rsForwardingEnabled = false
		require.Empty(t, skipped, "native route-server relay must own the destination")
		require.Equal(t, 1, dispatched, "native route-server relay must dispatch to the live socket worker")
		// The native rail can write directly into the destination's buffered
		// writer. Session.Run flushes its source's dirty destinations at the
		// read boundary; this direct-call fixture must serve that same boundary.
		f.source.session.flushFwdDirty()
		require.Empty(t, f.source.session.fwdDirty, "the native relay read boundary flushes every dirty destination")
	} else {
		f.forward(t, id)
	}
	// Wait for the destination FIFO itself, rather than the AIGP fixture's
	// optional completion marker. The caller's socket assertions still
	// require the actual announcement or withdrawal after this barrier.
	f.drain(t)
}

func nextHopAutoRailName(native bool) string {
	if native {
		return "native-rs"
	}
	return "cached"
}
