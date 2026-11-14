// Design: docs/architecture/core-design.md -- what the operator sees when a write passes its budget
// Related: session_write_budget.go -- recordWriteFailure and reportWriteBudgetExpiry under test
// Related: session_write_budget_test.go -- budgetEstablished, the fixture these tests drive
// Related: peer_close.go -- sessionDroppedOwed

package reactor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
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

// TestRequestDeadlineIsNotABudgetExpiry proves a write that met its request's
// own deadline is not reported as a write budget expiry. The session budget is
// a minute, the request gives its write 200 ms, and the peer end reads nothing:
// the write is still blocked when the request's time is up.
//
// VALIDATES: the request answers context.DeadlineExceeded and the session
// closes, as before the write budget existed, with no close reason of a budget
// expiry and no doctor-bgp-peer-not-reading line or event.
// PREVENTS: "accepted no data for 200ms, the write budget" with a fix that
// names ze.fwd.write.deadline, a setting that did not set that bound.
func TestRequestDeadlineIsNotABudgetExpiry(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	sink := closeLogSink(t)

	setWriteBudget(t, time.Minute)
	p := budgetEstablished(t)
	p.reader.stop(t)

	const requestTime = budgetShort
	ctx, cancel := context.WithTimeout(t.Context(), requestTime)
	defer cancel()

	written := make(chan error, 1)
	go func() {
		written <- p.session.sendUpdateCounted(ctx, budgetAnnounce(100), nil, false, nil, (*Session).writeUpdate)
	}()
	select {
	case err := <-written:
		require.ErrorIs(t, err, context.DeadlineExceeded,
			"a request that met its own deadline answers that deadline")
	case <-time.After(requestTime + wedgeSlack):
		t.Fatal("precondition: the writer was still blocked after its request's deadline")
	}
	select {
	case <-p.runResult:
	case <-time.After(runExitDeadline):
		t.Fatal("Run did not return after the request gave up a connection with a partial frame on it")
	}

	require.NotErrorIs(t, closeCause(p.session), ErrWriteBudgetExpired,
		"the request's deadline is not the write budget")
	for _, event := range report.Errors(0) {
		require.NotEqual(t, diagnostic.CodeDoctorBGPPeerNotReading, event.Code,
			"no budget passed, so the peer is not reported as one that stopped reading")
	}
	require.Empty(t, closeRecordsWith(closeLogRecords(t, sink), "code", diagnostic.CodeDoctorBGPPeerNotReading),
		"log:\n%s", sink.String())
}

// TestWriteBoundRecordsWhoseItIs proves the acquisition records, where it arms
// the socket's deadline, whether the bound is the session budget or the
// request's own deadline, and that the classification reads the record.
//
// VALIDATES: acquireWriteContext marks the bound as the request's only when
// the request's deadline is the earlier one, and writeBudgetPassed answers
// false for a deadline error under such a bound, after it passed included.
// PREVENTS: a classification that reads the armed duration alone, which
// cannot tell a 200 ms request from a 200 ms budget.
func TestWriteBoundRecordsWhoseItIs(t *testing.T) {
	session := NewSession(NewPeerSettings(mustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301))
	client, _, _ := pipeSession(t, session)
	defer func() { _ = client.Close() }()

	short, cancelShort := context.WithTimeout(t.Context(), time.Second)
	defer cancelShort()
	require.NoError(t, session.acquireWriteContext(short, time.Hour))
	require.True(t, session.writeBoundRequested, "the request's deadline is the earlier bound")
	require.LessOrEqual(t, session.writeBudget, time.Second, "the armed bound is the time left to the request")
	session.writeBudgetEnd = session.clock.Now().Add(-time.Second)
	require.False(t, session.writeBudgetPassed(os.ErrDeadlineExceeded),
		"a deadline error under the request's own bound is no budget expiry, even after that bound passed")
	session.releaseWrites()

	long, cancelLong := context.WithTimeout(t.Context(), time.Hour)
	defer cancelLong()
	require.NoError(t, session.acquireWriteContext(long, time.Second))
	require.False(t, session.writeBoundRequested, "the session budget is the earlier bound")
	require.Equal(t, time.Second, session.writeBudget)
	session.writeBudgetEnd = session.clock.Now().Add(-time.Second)
	require.True(t, session.writeBudgetPassed(os.ErrDeadlineExceeded),
		"control: the same error after the session budget passed is an expiry")
	session.releaseWrites()

	require.NoError(t, session.acquireWriteContext(t.Context(), time.Second))
	require.False(t, session.writeBoundRequested, "a request with no deadline is bounded by the session budget")
	session.releaseWrites()
}

// TestWriteBudgetExpiryReachesPeerInfo proves the cause of a write budget
// expiry reaches the snapshot every command surface reads. The session waits
// out its budget, the peer records it through the hook it attaches, and the
// reactor's PeerInfo builder (reactorAPIAdapter.Peers) is asked for the peer.
//
// VALIDATES: AC-10 -- PeerInfo carries the close cause, when it happened and
// no NOTIFICATION record, which are the three fields `show bgp peer` reads for
// last-error (lastErrorString in plugins/cmd/peer/summary.go, whose own test
// TestLastErrorNamesCloseWithoutNotification starts from this PeerInfo), and
// the cause renders as the words that test pins.
// PREVENTS: a record the peer keeps and the builder does not copy, so that
// last-error answers "" for a peer ze closed because it stopped reading.
func TestWriteBudgetExpiryReachesPeerInfo(t *testing.T) {
	// New resolves ze.fwd.write.deadline into the package's write budget
	// (initFwdWriteDeadline), so the reactor is built BEFORE the test sets its
	// own budget: built after, it would put the 30-second default back.
	r := New(&Config{})
	setWriteBudget(t, budgetShort)
	p := budgetEstablished(t)

	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	peer := NewPeer(settings)
	r.peers[settings.PeerKey()] = peer
	p.session.onWriteBudgetExpired = peer.recordWriteBudgetClose

	adapter := &reactorAPIAdapter{r: r}
	before := adapter.Peers()
	require.Len(t, before, 1)
	require.Equal(t, plugin.CloseCauseNone, before[0].LastCloseCause, "precondition: no record yet")

	budgetExpire(t, p)

	infos := adapter.Peers()
	require.Len(t, infos, 1)
	info := infos[0]
	require.Equal(t, plugin.CloseCauseWriteBudget, info.LastCloseCause,
		"the builder must copy the peer's close cause into PeerInfo")
	require.False(t, info.LastCloseCauseTime.IsZero(), "the builder must copy when the close happened")
	require.Equal(t, plugin.NotifNone, info.LastNotifDirection,
		"no NOTIFICATION was built, so the close cause is the last error")
	require.Equal(t, "peer accepted no data for the write budget (no NOTIFICATION sent)",
		info.LastCloseCause.String(), "the words last-error renders")
}

// TestBudgetExpiryInForwardFlushLogsOnce proves a write budget expiry met by
// the deferred flush of the route-server fast path is logged once. The
// destination holds octets in its write buffer, as the fast path leaves them,
// its peer end reads nothing, and a source session flushes it.
//
// VALIDATES: the flush logs the coded line of the expiry and no second line
// for the same failed flush.
// PREVENTS: "forward flush failed; destination session retired" beside the
// doctor-bgp-peer-not-reading line: one close, two lines, one of them with no
// code and no fix.
func TestBudgetExpiryInForwardFlushLogsOnce(t *testing.T) {
	sink := closeLogSink(t)

	setWriteBudget(t, budgetShort)
	p := budgetEstablished(t)
	p.reader.stop(t)

	p.session.writeMu.Lock()
	_, err := p.session.bufWriter.Write(message.PackTo(message.NewKeepalive(), nil))
	p.session.writeMu.Unlock()
	require.NoError(t, err)

	source := NewSession(NewPeerSettings(mustParseAddr("192.0.2.2"), 65001, 65003, 0x01020302))
	source.fwdDirty = []fwdDirtyTarget{{session: p.session}}
	flushed := make(chan struct{})
	go func() {
		defer close(flushed)
		source.flushFwdDirty()
	}()
	select {
	case <-flushed:
	case <-time.After(budgetShort + wedgeSlack):
		t.Fatal("precondition: the flush was still blocked after its write budget")
	}
	select {
	case <-p.runResult:
	case <-time.After(runExitDeadline):
		t.Fatal("precondition: Run did not return after the flush passed its budget")
	}

	records := closeLogRecords(t, sink)
	require.Len(t, closeRecordsWith(records, "code", diagnostic.CodeDoctorBGPPeerNotReading), 1,
		"the expiry has its coded line; log:\n%s", sink.String())
	require.Empty(t, closeRecordsWith(records, "msg", "forward flush failed; destination session retired"),
		"the coded line is the record of this close; log:\n%s", sink.String())
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
