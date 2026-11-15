// Design: docs/architecture/bgp/structural-forwarding.md -- exclusion withdraws prior state
// Related: rfc4271_atomic_aggregate_relay_proof_test.go -- live relay helpers
// Related: forward_withhold_withdraw_test.go -- the conversion-only controls
// RFC: rfc/short/rfc4271.md -- Section 9.1.3

package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// The same destination first receives the route, then its export filter
// rejects a newer generation. Reading only a conversion helper would not
// establish that a real writer withdraws the previously advertised route.
// RFC requirement: RFC4271-9.2-3 positive -- after a route was advertised,
// an export reject on either relay rail writes an UPDATE withdrawing exactly
// that prefix, with no replacement announcement on that destination.
func TestRFC4271ExportExclusionWithdrawsPreviouslyAdvertisedRoute(t *testing.T) {
	want := []byte{24, 10, 42, 0}
	for _, rs := range []bool{false, true} {
		f := newAIGPReplayFixture(t, nil)
		rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(want, false, 10), rs)
		initial := aigpSocketBodies(t, f.conn)
		require.Len(t, initial, 1)
		announced, err := message.UnpackUpdate(initial[0])
		require.NoError(t, err)
		require.Equal(t, want, announced.NLRI)
		require.Empty(t, announced.WithdrawnRoutes)

		exportLibraryAnswer(t, f.r, libraryReject, f.destination)
		rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(want, false, 20), rs)
		written := aigpSocketBodies(t, f.conn)
		require.Len(t, written, 2, "rs=%v: rejection must reach the socket after the announcement", rs)
		withdrawn, err := message.UnpackUpdate(written[1])
		require.NoError(t, err)
		require.Equal(t, want, withdrawn.WithdrawnRoutes)
		require.Empty(t, withdrawn.NLRI)
		require.Empty(t, withdrawn.PathAttributes)
	}
}

// An accepting export filter must still allow the new generation. Its MED
// changes so a legitimate duplicate suppression cannot hide the control.
// RFC requirement: RFC4271-9.2-3 negative -- a previously advertised route
// whose export filter accepts the newer generation remains announced, with
// the changed MED, rather than being withdrawn by an overbroad reject rule.
func TestRFC4271AcceptedGenerationRemainsAnnounced(t *testing.T) {
	want := []byte{24, 10, 43, 0}
	for _, rs := range []bool{false, true} {
		f := newAIGPReplayFixture(t, nil)
		rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(want, false, 10), rs)
		exportLibraryAnswer(t, f.r, medLibraryAnswer(77), f.destination)
		rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(want, false, 20), rs)
		written := aigpSocketBodies(t, f.conn)
		require.Len(t, written, 2, "rs=%v: an accepted changed generation is advertised", rs)
		latest, err := message.UnpackUpdate(written[1])
		require.NoError(t, err)
		require.Equal(t, want, latest.NLRI)
		require.Empty(t, latest.WithdrawnRoutes)
		med := payloadMED(written[1])
		require.True(t, med.present)
		require.Len(t, med.raw, 4)
		require.Equal(t, uint32(77), binary.BigEndian.Uint32(med.raw))
	}
}
