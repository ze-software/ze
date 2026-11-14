// Design: docs/architecture/core-design.md -- what the operator sees when a write passes its budget
// Related: session_write_budget.go -- recordWriteFailure and reportWriteBudgetExpiry under test
// Related: session_write_budget_test.go -- budgetEstablished, the fixture these tests drive
// Related: peer_close.go -- sessionDroppedOwed

package reactor

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// budgetExpire makes one UPDATE writer of p wait out a 200 ms write budget
// against a peer end that stopped reading, and returns once Run has returned.
// The caller sets the budget (setWriteBudget) before it builds p.
func budgetExpire(t *testing.T, p *budgetPeer) {
	t.Helper()
	p.reader.stop(t)

	written := make(chan error, 1)
	go func() { written <- p.session.SendUpdate(budgetAnnounce(100)) }()

	select {
	case err := <-written:
		require.ErrorIs(t, err, os.ErrDeadlineExceeded,
			"precondition: the writer did not fail on its write budget, so nothing expired")
	case <-time.After(budgetShort + wedgeSlack):
		t.Fatal("precondition: the writer was still blocked after its budget, so nothing expired")
	}

	select {
	case <-p.runResult:
	case <-time.After(runExitDeadline):
		t.Fatal("precondition: Run did not return after the write passed its budget")
	}
}

// TestWriteBudgetExpiryReason proves a write that waits out its budget records
// a close reason that names the cause in plain words and keeps the write's own
// error.
//
// VALIDATES: AC-10 -- the session's close reason is ErrWriteBudgetExpired, it
// gives the budget that passed, and it still answers for the deadline error.
// PREVENTS: a session-end reason that reads "i/o timeout", which tells an
// operator nothing about the peer.
func TestWriteBudgetExpiryReason(t *testing.T) {
	setWriteBudget(t, budgetShort)
	p := budgetEstablished(t)

	budgetExpire(t, p)

	reason := closeCause(p.session)
	require.ErrorIs(t, reason, ErrWriteBudgetExpired,
		"the close reason must say the peer accepted no data for the write budget")
	require.ErrorIs(t, reason, os.ErrDeadlineExceeded,
		"the close reason must keep the error of the write that failed")
	require.Contains(t, reason.Error(), budgetShort.String(),
		"the close reason must give the budget that passed")
}

// TestWriteBudgetExpiryLogsOneCodedWarning proves the expiry is logged once at
// WARN with the attributes an operator and the doctor read. The session logger
// is swapped for a JSON handler, and the test reads the records it wrote.
//
// VALIDATES: AC-10 -- one WARN line names the peer and the budget, says the
// session was closed and that ze connects again, and carries its doctor code
// and a fix.
// PREVENTS: an expiry reported by each writer in its own words, or under no
// code `ze show diagnostic` can explain.
func TestWriteBudgetExpiryLogsOneCodedWarning(t *testing.T) {
	var sink syncBuffer
	restore := swapSessionLogger(func() *slog.Logger {
		return slog.New(slog.NewJSONHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn}))
	})
	t.Cleanup(restore)

	setWriteBudget(t, budgetShort)
	p := budgetEstablished(t)

	budgetExpire(t, p)

	var coded []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(sink.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record), "log line: %s", line)
		if record["code"] == diagnostic.CodeDoctorBGPPeerNotReading {
			coded = append(coded, record)
		}
	}
	require.Len(t, coded, 1, "exactly one line carries the code; log:\n%s", sink.String())

	line := coded[0]
	require.Equal(t, "WARN", line["level"])
	require.Equal(t, "192.0.2.1", line["peer"], "the line must name the peer")
	require.EqualValues(t, budgetShort, line["budget"], "the line must give the budget that passed")
	require.Equal(t, writeBudgetExpiryFix, line["fix"])
	require.Contains(t, line["fix"], "ze.fwd.write.deadline", "the fix must name the setting of the budget")
	require.Contains(t, line["msg"], "the peer accepted no data for the write budget")
	require.Contains(t, line["msg"], "session closed")
	require.Contains(t, line["msg"], "connects to it again")
}

// TestWriteBudgetExpiryRaisesOneEvent proves the expiry is raised once on the
// report bus under its own code, with the fix, so `show doctor` and
// `show errors` report it.
//
// VALIDATES: AC-10 and A-8 -- one error event, source bgp, the peer as its
// subject, the code the reactor registered as an event code.
// PREVENTS: a transient problem that only the log holds.
func TestWriteBudgetExpiryRaisesOneEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	setWriteBudget(t, budgetShort)
	p := budgetEstablished(t)

	budgetExpire(t, p)

	events := report.Errors(0)
	require.Len(t, events, 1, "one close raises one event")
	event := events[0]
	require.Equal(t, reportSourceBGP, event.Source)
	require.Equal(t, diagnostic.CodeDoctorBGPPeerNotReading, event.Code)
	require.Equal(t, "192.0.2.1", event.Subject)
	require.Equal(t, writeBudgetExpiryFix, event.Fix)
	require.LessOrEqual(t, len(writeBudgetExpiryFix), report.FixMax, "the bus refuses a longer fix")
	require.Equal(t, budgetShort.String(), event.Detail["budget"])
	require.Contains(t, event.Message, "192.0.2.1")
	require.Contains(t, event.Message, budgetShort.String())
	require.Contains(t, diagnostic.EventCodes(), diagnostic.CodeDoctorBGPPeerNotReading,
		"the reactor must register the code it raises, or show doctor reports it under no code of its own")
}

// TestWriteFailureOtherThanBudgetIsNotAnExpiry proves a write that fails for
// another reason is not reported as a budget expiry. The peer end is closed, so
// the write fails at once with no deadline involved.
//
// VALIDATES: the classification in recordWriteFailure: only a deadline error
// met after the armed budget ran out is an expiry.
// PREVENTS: every failed write, a reset or a closed pipe, reported as a peer
// that stopped reading.
func TestWriteFailureOtherThanBudgetIsNotAnExpiry(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	setWriteBudget(t, budgetShort)
	p := budgetEstablished(t)
	require.NoError(t, p.reader.client.Close())

	require.Error(t, p.session.SendUpdate(budgetAnnounce(100)),
		"precondition: a write after the peer end closed must fail")
	select {
	case <-p.runResult:
	case <-time.After(runExitDeadline):
		t.Fatal("precondition: Run did not return after the peer end closed")
	}

	require.NotErrorIs(t, closeCause(p.session), ErrWriteBudgetExpired,
		"a closed connection is not a write budget expiry")
	for _, event := range report.Errors(0) {
		require.NotEqual(t, diagnostic.CodeDoctorBGPPeerNotReading, event.Code)
	}
}

// TestCanceledWriteIsNotABudgetExpiry proves the deadline error of a write its
// own request ended is not read as a budget expiry. The classification is
// called with the state a canceled request leaves: a budget armed and not yet
// passed, and a deadline error.
//
// VALIDATES: writeBudgetPassed reads the armed deadline, not the error alone.
// PREVENTS: a canceled API request reported as a peer that stopped reading.
func TestCanceledWriteIsNotABudgetExpiry(t *testing.T) {
	session := NewSession(NewPeerSettings(mustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301))

	session.writeMu.Lock()
	defer session.writeMu.Unlock()

	session.writeBudgetArmed = true
	session.writeBudget = time.Hour
	session.writeBudgetEnd = session.clock.Now().Add(time.Hour)
	require.False(t, session.writeBudgetPassed(os.ErrDeadlineExceeded),
		"the budget has not passed, so the deadline was expired early by the request")

	session.writeBudgetEnd = session.clock.Now().Add(-time.Second)
	require.True(t, session.writeBudgetPassed(os.ErrDeadlineExceeded),
		"control: the same error after the armed deadline is an expiry")
	require.False(t, session.writeBudgetPassed(os.ErrClosed),
		"an error that is no deadline error is never an expiry")

	session.writeBudgetArmed = false
	require.False(t, session.writeBudgetPassed(os.ErrDeadlineExceeded),
		"a holder that armed no budget met none")
}

// TestWriteBudgetExpiryRecordsPeerLastError proves the expiry reaches the
// peer's last-error record. The hook the peer attaches is set alone on the
// running session, because the other hooks are read by its goroutines.
//
// VALIDATES: AC-10 -- the peer's last error reports the same cause.
// PREVENTS: `show bgp peer` answering what a healthy peer answers after ze
// closed the session on a peer that stopped reading.
func TestWriteBudgetExpiryRecordsPeerLastError(t *testing.T) {
	setWriteBudget(t, budgetShort)
	p := budgetEstablished(t)
	peer, _ := newPeerWithMetrics()
	require.Equal(t, plugin.CloseCauseNone, peer.Stats().LastCloseCause, "precondition: no record yet")
	p.session.onWriteBudgetExpired = peer.recordWriteBudgetClose

	budgetExpire(t, p)

	stats := peer.Stats()
	require.Equal(t, plugin.CloseCauseWriteBudget, stats.LastCloseCause)
	require.False(t, stats.LastCloseCauseTime.IsZero(), "the record must say when")
}

// TestAttachStatsCallbacksWiresWriteBudgetClose proves the peer attaches its
// last-error record to the session's expiry hook.
//
// VALIDATES: the wiring TestWriteBudgetExpiryRecordsPeerLastError sets by hand.
// PREVENTS: a record method no session ever calls.
func TestAttachStatsCallbacksWiresWriteBudgetClose(t *testing.T) {
	peer, _ := newPeerWithMetrics()
	session := NewSession(NewPeerSettings(mustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301))

	peer.attachStatsCallbacks(session)
	require.NotNil(t, session.onWriteBudgetExpired)
	session.onWriteBudgetExpired()

	require.Equal(t, plugin.CloseCauseWriteBudget, peer.Stats().LastCloseCause)
}

// TestSessionDroppedOwed proves which session ends owe the session-dropped
// event, so one close gives one event.
//
// VALIDATES: a close on a write budget expiry owes no session-dropped event,
// because the session raised the expiry under its own code; every other close
// with no NOTIFICATION still owes it.
// PREVENTS: two events for one close, and a session-dropped event lost for a
// transport failure that nothing else reports.
func TestSessionDroppedOwed(t *testing.T) {
	expiry := errors.Join(ErrWriteBudgetExpired, os.ErrDeadlineExceeded)
	cases := []struct {
		name     string
		notified bool
		cause    error
		owed     bool
	}{
		{"transport failure, no cause recorded", false, nil, true},
		{"hold timer expiry with no NOTIFICATION", false, ErrHoldTimerExpired, true},
		{"a deadline error that is no budget expiry", false, os.ErrDeadlineExceeded, true},
		{"NOTIFICATION exchanged", true, nil, false},
		{"write budget expiry", false, expiry, false},
		{"write budget expiry after a NOTIFICATION", true, expiry, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.owed, sessionDroppedOwed(tc.notified, tc.cause))
		})
	}
}
