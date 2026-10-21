// Design: docs/architecture/update-building.md -- owed keys and the recovery worker
// Related: relay_recovery.go -- queueRecoveryOutcome: a survivor the destination may not receive is stepped past
// Related: forward_pool.go -- fwdBatchHandler settles every recovery admission
//
// VALIDATES: an owed key at an external destination whose only survivor
// carries NO_EXPORT is decided as a withdrawal, the admission settles, and the
// destination session stays up.
// PREVENTS: test/plugin/wellknown-no-export-withdraw-egress.ci, where the
// recovery worker reported "source-DOWN recovery operation was not written"
// and reset the external session.
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// TestOwedKeyNoExportSurvivorWithdrawsAtExternalDestination proves a
// policy-denied survivor settles the owed key as a withdrawal.
// RFC 1997 Section "Well-known Communities": "All routes received carrying a
// communities attribute containing this value MUST NOT be advertised outside
// a BGP confederation boundary".
// Method: the destination is external. The source announces P without
// communities, which reaches the destination, then withdraws it, which owes
// P. The RIB answers with one survivor from the same source carrying
// NO_EXPORT. Its announcement is suppressed toward the external peer, so the
// decision written is the withdrawal of P, the record leaves the owed set,
// and the session is not torn down.
func TestOwedKeyNoExportSurvivorWithdrawsAtExternalDestination(t *testing.T) {
	rail := newOwedRail(t)
	rail.destination.settings.PeerAS = 65002
	rail.destination.refreshForwardFacts()
	p := []byte{24, 192, 0, 2}
	session := rail.destination.currentSession()

	rail.forward(t, false, p)
	require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 1, "P reaches the external destination")
	rail.forward(t, true, p)
	require.Len(t, rail.owed(), 1, "the owner withdrawal is owed")

	rail.request(t)
	attributes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x00, // AS_PATH empty
		0x40, 0x03, 0x04, 192, 0, 2, 1, // NEXT_HOP
		0xC0, 0x08, 0x04, 0xFF, 0xFF, 0xFF, 0x01, // COMMUNITY NO_EXPORT
	}
	source := rail.source.Settings().Address
	rail.release <- func(request ribevents.RecoveryRequest) []ribevents.RecoveryOutcome {
		return recoverySurvivorAnswer(request, p, ribevents.RecoveryRoute{Source: source,
			NLRI: p, Attributes: attributes, NextHop: []byte{192, 0, 2, 1}, MessageID: 9})
	}

	lowEventually(t, func() bool {
		return len(recoveryWrittenUpdates(t, rail.conn.written())) == 2
	}, "the suppressed survivor's withdrawal is written")
	updates := recoveryWrittenUpdates(t, rail.conn.written())
	require.True(t, bytes.Equal(p, updates[1].WithdrawnRoutes), "the decision withdraws P")
	require.Empty(t, updates[1].NLRI, "the NO_EXPORT survivor is never announced")
	lowEventually(t, func() bool { return len(rail.owed()) == 0 }, "the decided record leaves the owed set")
	require.False(t, session.tearingDown.Load(), "a suppressed survivor does not reset the session")
	require.Same(t, session, rail.destination.currentSession())
}
