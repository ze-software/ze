// Design: docs/architecture/update-building.md -- owed keys and the recovery worker
// Related: forward_pool.go -- fwdBatchHandler, closeRecoveryWrites
// Related: recovery_owed.go -- recoveryWorker.run
//
// VALIDATES: a recovery admission that reaches a destination with no
// Established session is settled with errRecoveryDestinationClosed on every
// early return of the batch handler.
// PREVENTS: an admission released unsettled, which the worker read as
// errRecoveryNotWritten ("source-DOWN recovery operation was not written")
// and answered by resetting a session that was already closing.
package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// TestRecoveryAdmissionSettlesWhenDestinationClosed proves each early return
// of fwdBatchHandler names its outcome for a recovery admission.
// Method: one admission is handed to the production batch handler, then to
// the pool's release, for a destination with no session, with a session that
// left Established, and with a session that has no connection. Each admission
// must carry errRecoveryDestinationClosed and nothing reaches the socket.
func TestRecoveryAdmissionSettlesWhenDestinationClosed(t *testing.T) {
	cases := []struct {
		name  string
		close func(t *testing.T, destination *Peer, session *Session)
	}{
		{name: "no session", close: func(_ *testing.T, destination *Peer, _ *Session) {
			destination.mu.Lock()
			destination.session = nil
			destination.mu.Unlock()
		}},
		{name: "left established", close: func(t *testing.T, _ *Peer, session *Session) {
			t.Helper()
			require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionFails))
			require.NotEqual(t, fsm.StateEstablished, session.fsm.State())
		}},
		{name: "no connection", close: func(_ *testing.T, _ *Peer, session *Session) {
			session.mu.Lock()
			session.conn = nil
			session.mu.Unlock()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _, destination, conn, _ := newSyncOrderRail(t)
			session := destination.currentSession()
			tc.close(t, destination, session)

			admission := newRecoveryAdmission(t.Context(), destination, session, (&reactorAPIAdapter{r: r}).recoverySources(), nil)
			items := []fwdItem{{peer: destination, recovery: admission}}
			fwdBatchHandler(fwdKey{peerAddr: destination.Settings().PeerKey()}, items)
			r.fwdPool.releaseItem(&items[0])

			<-admission.done
			require.ErrorIs(t, admission.err, errRecoveryDestinationClosed)
			require.Empty(t, conn.written(), "a closed destination is written nothing")
		})
	}
}

// TestRecoveryWorkerStopsQuietlyWhenDestinationClosed proves the worker does
// not reset a session that closed under its admission.
// Method: P is owed at the destination and the worker's lookup is held. The
// session loses its connection, then the RIB answers. The admission settles
// with errRecoveryDestinationClosed; the worker ends on its own, and the
// session is not torn down by it: its close owns the owed set.
func TestRecoveryWorkerStopsQuietlyWhenDestinationClosed(t *testing.T) {
	rail := newOwedRail(t)
	p := []byte{24, 192, 0, 2}
	session := rail.destination.currentSession()

	rail.forward(t, false, p)
	rail.forward(t, true, p)
	rail.request(t)
	session.mu.Lock()
	session.conn = nil
	session.mu.Unlock()
	rail.release <- nil

	worker := session.recovery.Load()
	require.NotNil(t, worker)
	select {
	case <-worker.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the worker kept running after its destination closed")
	}
	require.False(t, session.tearingDown.Load(), "the worker does not reset a closing session")
}
