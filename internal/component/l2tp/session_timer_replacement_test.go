// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- RADIUS session/idle timeout
// Related: session_timeout.go -- runSessionTimeout, runIdleTimeout
// Related: teardown.go -- teardownCapturedSession

package l2tp

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// TestSessionTimersCannotEndReplacement proves AC-3 for the two RADIUS timers:
// a session timeout or idle timeout armed for a session that ended cannot end
// the session that drew its session ID again, while each timer of the
// replacement still ends the replacement with its own cause.
//
// Method: session 7 on tunnel 11 holds both timer contexts and is cleared
// through the operator's captured teardown. A replacement takes SID 7 with its
// own contexts. The old timers then run through runSessionTimeout and
// runIdleTimeout, and the teardown their expiry makes is called directly too,
// with the old context and with the replacement's: none may end anything. The
// replacement's own timers then run and end it.
// MUTATION: dropping the row or context checks in teardownCapturedSession
// lets an old timer end the replacement with a second session-down.
func TestSessionTimersCannotEndReplacement(t *testing.T) {
	bus := newCauseBus()
	r := newReactorForPeerTeardown(t, bus)
	old := &L2TPSession{localSID: 7, state: L2TPSessionEstablished, username: "old"}
	insertEstablishedTunnel(t, r, 11, 22, netip.MustParseAddrPort("10.0.0.9:1701"), old)
	tunnel := r.tunnelsByLocalID[11]
	oldSession, oldIdle := armSessionTimers(t, r, old)

	subsystem := NewSubsystem(&Parameters{})
	subsystem.reactors = []*l2tpReactor{r}
	captured, err := subsystem.CaptureSession(SessionSelector{SID: 7, TID: 11})
	require.NoError(t, err)
	require.NoError(t, captured.Teardown(t.Context(), l2tpevents.TerminateCauseAdminReset, ""))
	require.Len(t, bus.downEvents(), 1)

	replacement := addReplacementSession(t, r, tunnel, "new")
	replacementSession, _ := armSessionTimers(t, r, replacement)

	r.runSessionTimeout(tunnel, old, time.Nanosecond, oldSession)
	r.runIdleTimeout(tunnel, old, time.Nanosecond, "", oldIdle)
	require.False(t, r.teardownCapturedSession(tunnel, old, oldSession, cdnResultAdministrative, l2tpevents.TerminateCauseSessionTimeout),
		"an expiry selected before cancellation cannot end the replacement")
	require.False(t, r.teardownCapturedSession(tunnel, old, oldIdle, cdnResultAdministrative, l2tpevents.TerminateCauseIdleTimeout),
		"an idle expiry selected before cancellation cannot end the replacement")
	require.False(t, r.teardownCapturedSession(tunnel, old, replacementSession, cdnResultAdministrative, l2tpevents.TerminateCauseSessionTimeout),
		"the replacement's live context cannot authorize ending the old row")
	r.tunnelsMu.Lock()
	require.Same(t, replacement, tunnel.lookupSession(7), "the replacement stays installed")
	r.tunnelsMu.Unlock()
	require.Len(t, bus.downEvents(), 1, "no old timer may publish a session-down")

	r.runSessionTimeout(tunnel, replacement, time.Nanosecond, replacementSession)
	downs := bus.downEvents()
	require.Len(t, downs, 2, "the replacement's own session timeout ends it")
	require.Equal(t, replacement.Birth(), downs[1].Birth)
	require.Equal(t, l2tpevents.TerminateCauseSessionTimeout, downs[1].Cause)
	require.Equal(t, "new", downs[1].Username)

	idleOwner := addReplacementSession(t, r, tunnel, "idle")
	_, idleContext := armSessionTimers(t, r, idleOwner)
	r.runIdleTimeout(tunnel, idleOwner, time.Nanosecond, "", idleContext)
	downs = bus.downEvents()
	require.Len(t, downs, 3, "the replacement's own idle timeout ends it")
	require.Equal(t, idleOwner.Birth(), downs[2].Birth)
	require.Equal(t, l2tpevents.TerminateCauseIdleTimeout, downs[2].Cause)
	r.tunnelsMu.Lock()
	require.Nil(t, tunnel.lookupSession(7))
	r.tunnelsMu.Unlock()
}

// armSessionTimers installs both timer contexts the way startSessionTimeouts
// does, without starting their goroutines, and returns them.
func armSessionTimers(t *testing.T, r *l2tpReactor, sess *L2TPSession) (sessionTimer, idleTimer context.Context) {
	t.Helper()
	sessionTimer, sessionCancel := context.WithCancel(context.Background())
	idleTimer, idleCancel := context.WithCancel(context.Background())
	t.Cleanup(sessionCancel)
	t.Cleanup(idleCancel)
	r.tunnelsMu.Lock()
	sess.sessionTimeoutContext, sess.sessionTimeoutCancel = sessionTimer, sessionCancel
	sess.idleTimeoutContext, sess.idleTimeoutCancel = idleTimer, idleCancel
	r.tunnelsMu.Unlock()
	return sessionTimer, idleTimer
}

// addReplacementSession publishes a new established admission on SID 7.
func addReplacementSession(t *testing.T, r *l2tpReactor, tunnel *L2TPTunnel, username string) *L2TPSession {
	t.Helper()
	sess := newFixtureL2TPSession(t, &L2TPSession{localSID: 7, state: L2TPSessionEstablished, username: username})
	r.tunnelsMu.Lock()
	require.Nil(t, tunnel.lookupSession(7))
	tunnel.addSession(sess)
	r.tunnelsMu.Unlock()
	return sess
}
