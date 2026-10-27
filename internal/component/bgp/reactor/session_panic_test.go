package reactor

import (
	"errors"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// runOutcome is how Session.Run ended: the error it returned, or the panic that
// escaped it. The test goroutine recovers an escaped panic only so the failure
// is reported here instead of ending the whole test binary.
type runOutcome struct {
	err     error
	escaped any
}

// readPanicSession drives one received UPDATE through the real Session.Run read
// path over net.Pipe, with the onMessageReceived seam panicking on it, and
// asserts the whole containment policy:
//
//   - the panic stops inside Run, which returns it as a *taskguard.Panic so
//     runOnce's sessionDown and the run loop's backoff follow (AC-1);
//   - the peer receives a NOTIFICATION Cease (6) / Unspecific (0) before the
//     socket closes (AC-1, RFC 4271 Section 6.7 and Section 4.5);
//   - the connection is closed;
//   - the read buffer the panicking message held is back in its pool (AC-3);
//   - a recovered-panic crash record is listed for `ze show crashes` (AC-2).
//
// coalesce selects which of the two read functions Run drives.
func readPanicSession(t *testing.T, coalesce bool) {
	t.Helper()
	restore := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restore)

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)
	session.coalesceEnabled = coalesce
	client, wire, drainErr := pipeSession(t, session)
	defer func() { _ = client.Close() }()

	_, inUseBefore := bufMuxStd.Stats()
	panicsBefore, declared := bgpPanics().guard.Panics("read")
	require.True(t, declared, "the BGP guard declares the read task")

	var callbackRan bool
	// Only the received UPDATE panics: the callback also sees the NOTIFICATION
	// the boundary sends (DirectionSent), and a panic there would be a second
	// defect in the teardown, not the read-path one under test.
	session.onMessageReceived = func(_ netip.Addr, kind msgtype.MessageType, _ []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if kind != msgtype.TypeUPDATE || direction != rpc.DirectionReceived {
			return false
		}
		callbackRan = true
		panic("injected read-path panic")
	}

	resultCh := make(chan runOutcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultCh <- runOutcome{escaped: r}
			}
		}()
		resultCh <- runOutcome{err: session.Run(t.Context())}
	}()
	go func() { _, _ = client.Write(eorUpdate()) }()

	var outcome runOutcome
	select {
	case outcome = <-resultCh:
	case <-time.After(runExitDeadline):
		t.Fatal("Run did not return within " + runExitDeadline.String() + " of the panicking UPDATE")
	}

	require.True(t, callbackRan, "precondition: the UPDATE never reached onMessageReceived, so nothing panicked")
	require.Nil(t, outcome.escaped,
		"AC-1: the panic escaped Session.Run: the read path has no inner boundary, so no NOTIFICATION can be sent")

	var contained *taskguard.Panic
	require.True(t, errors.As(outcome.err, &contained),
		"AC-1: Run must return the contained panic as its error so sessionDown and the backoff follow, got %v", outcome.err)
	require.Equal(t, "UPDATE", contained.MessageType, "AC-2: the panic names the message being processed")
	require.Equal(t, "bgp", contained.Component, "AC-2: the panic names the component")
	require.Equal(t, "read", contained.Task, "AC-2: the panic names the task")
	require.Equal(t, "192.0.2.1", contained.Unit, "AC-2: the panic names the peer")
	panicsAfter, _ := bgpPanics().guard.Panics("read")
	require.Equal(t, panicsBefore+1, panicsAfter, "AC-2: ze_task_panics_total{component=bgp,task=read} counts the panic once")

	requireConnClosed(t, drainErr)

	// RFC 4271 Section 6.7: "In the absence of any fatal errors (that are
	// indicated in this section), a BGP peer MAY choose, at any given time, to
	// close its BGP connection by sending the NOTIFICATION message with the
	// Error Code Cease."
	// RFC 4271 Section 4.5: "If no appropriate Error Subcode is defined, then a
	// zero (Unspecific) value is used for the Error Subcode field."
	sent := collectWire(wire)
	require.GreaterOrEqual(t, len(sent), 21, "AC-1: expected a NOTIFICATION on the wire, got %d bytes", len(sent))
	require.Equal(t, msgtype.TypeNOTIFICATION, msgtype.MessageType(sent[18]), "message type")
	require.Equal(t, uint8(message.NotifyCease), sent[19], "NOTIFICATION error code")
	require.Equal(t, uint8(0), sent[20], "NOTIFICATION error subcode: Unspecific")
	require.Equal(t, 21, int(sent[16])<<8|int(sent[17]), "the NOTIFICATION carries no Data: the panic stays local")

	_, inUseAfter := bufMuxStd.Stats()
	require.Equal(t, inUseBefore, inUseAfter, "AC-3: the panicking message's read buffer was not returned to the pool")

	crashes := crashlog.ListCrashes()
	require.Len(t, crashes, 1, "AC-2: one crash record for the contained panic")
	require.Equal(t, crashlog.KindRecovered, crashes[0].Kind, "AC-2: the record is listed as a recovered panic")
	require.Equal(t, contained.Report, crashes[0].Name, "AC-2: the panic names the record it wrote")
}

// TestSessionReadPanicSendsCeaseAndCloses proves the containment policy on the
// single-message read path (readAndProcessMessage).
//
// VALIDATES: AC-1, AC-2, AC-3 -- see readPanicSession.
// PREVENTS: a read-path panic unwinding through Run's closeConn defer to
// safeRunOnce, which closes the socket with no NOTIFICATION, no metric and no
// crash record.
func TestSessionReadPanicSendsCeaseAndCloses(t *testing.T) {
	readPanicSession(t, false)
}

// TestSessionReadPanicCoalesced proves the same policy on the coalesced read
// path (readAndProcessCoalesced), which Run selects when coalescing is on.
//
// VALIDATES: AC-1, AC-2, AC-3 on the coalesced path.
// PREVENTS: a boundary placed in one read function and not the other.
func TestSessionReadPanicCoalesced(t *testing.T) {
	readPanicSession(t, true)
}

// TestSessionReadPanicWriteLockHeld proves the read task's boundary never
// blocks on the write path. A writer holds writeMu for longer than the
// NOTIFICATION's deadline while the read path panics; the guard gives up on the
// lock after holdExpiryNotifyDeadline, says no NOTIFICATION was sent, and
// closes the socket while the lock is still held.
//
// The panicking frame itself cannot be that holder: every writeMu hold on the
// read path is released by defer during the unwind (lock audit A-1, spec
// "Design Insights"), so the holder left is another writer, such as a forward
// worker stuck in a write.
//
// VALIDATES: AC-6.
// PREVENTS: a guard that waits on writeMu and leaks the session behind a
// stuck writer (R-4).
func TestSessionReadPanicWriteLockHeld(t *testing.T) {
	restore := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restore)
	logs := captureSessionLog(t, slog.LevelError)

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	session := NewSession(settings)
	client, wire, drainErr := pipeSession(t, session)
	defer func() { _ = client.Close() }()

	session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		panic("BUG: injected read-path panic")
	}

	// The stuck writer: it holds the write path until the test releases it.
	session.writeMu.Lock()
	writerHolds := true
	defer func() {
		if writerHolds {
			session.writeMu.Unlock()
		}
	}()

	resultCh := make(chan runOutcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultCh <- runOutcome{escaped: r}
			}
		}()
		resultCh <- runOutcome{err: session.Run(t.Context())}
	}()
	go func() { _, _ = client.Write(eorUpdate()) }()

	// The socket closes while writeMu is still held: the guard did not wait.
	requireConnClosed(t, drainErr)
	require.Empty(t, collectWire(wire), "AC-6: nothing may reach the wire while another writer holds the write path")
	require.Contains(t, logs.String(), "no NOTIFICATION was sent to the peer",
		"AC-6: the log says the peer was not told why the session ended")

	writerHolds = false
	session.writeMu.Unlock()

	var outcome runOutcome
	select {
	case outcome = <-resultCh:
	case <-time.After(runExitDeadline):
		t.Fatal("Run did not return within " + runExitDeadline.String() + " of the writer releasing the write path")
	}
	require.Nil(t, outcome.escaped, "AC-6: the panic escaped Session.Run")
	var contained *taskguard.Panic
	require.True(t, errors.As(outcome.err, &contained), "AC-6: Run returns the contained panic, got %v", outcome.err)
}

// TestPeerReadPanicIsolation proves a read-path panic on one peer ends only
// that peer's session, through the real reactor: two peers established over
// their sockets, peer A's read path panics on an UPDATE.
//
// VALIDATES: AC-4 (peer B keeps exchanging messages; peer A is told Cease /
// Unspecific and reconnects through the run loop), AC-1 at the reactor's own
// entry point.
// PREVENTS: a contained panic that leaves a reactor-wide lock held, which would
// stall every other peer (lock audit A-1).
func TestPeerReadPanicIsolation(t *testing.T) {
	restore := crashlog.SetCrashDirForTest(t.TempDir())
	t.Cleanup(restore)

	peers := lowLiveRouter(t,
		lowLiveSettings("10.0.0.1", 65000, 65002),
		lowLiveSettings("10.0.0.2", 65000, 65003))
	a, b := peers[0], peers[1]

	// The seam: peer A's current session hands every received message to a
	// callback that panics on an UPDATE. Written before A's UPDATE is sent, and
	// the pipe's write-to-read hand-off orders it before the read goroutine's use.
	sessionA := a.peer.currentSession()
	forward := sessionA.onMessageReceived
	sessionA.onMessageReceived = func(addr netip.Addr, kind msgtype.MessageType, raw []byte,
		update *wireu.WireUpdate, ctxID bgpctx.ContextID, direction rpc.MessageDirection,
		buf BufHandle, meta map[string]any, source string, sourceID uint64) bool {
		if kind == msgtype.TypeUPDATE && direction == rpc.DirectionReceived {
			panic("BUG: injected read-path panic on peer A")
		}
		return forward(addr, kind, raw, update, ctxID, direction, buf, meta, source, sourceID)
	}
	panicsBefore, _ := bgpPanics().guard.Panics("read")

	ownershipLiveSend(t, a, nil, firstASAttrs(4, 65002), netip.MustParsePrefix("10.20.0.0/24")) // RFC 4271 Section 4.3.
	lowEventually(t, func() bool { return ceaseSent(a, message.NotifyCeaseUnspecific) }, "AC-1: peer A told Cease / Unspecific")
	lowEventually(t, func() bool { return a.peer.State() != PeerStateEstablished }, "AC-1: peer A left Established")

	// Peer B is untouched: it stays Established and its next UPDATE is read.
	updatesBefore := b.peer.counters.updatesReceived.Load()
	ownershipLiveSend(t, b, nil, firstASAttrs(4, 65003), netip.MustParsePrefix("10.30.0.0/24")) // RFC 4271 Section 4.3.
	lowEventually(t, func() bool { return b.peer.counters.updatesReceived.Load() > updatesBefore }, "AC-4: peer B's UPDATE read")
	b.send(t, message.PackTo(message.NewKeepalive(), nil))
	require.Equal(t, PeerStateEstablished, b.peer.State(), "AC-4: peer B stays Established")
	require.False(t, ceaseSent(b, message.NotifyCeaseUnspecific), "AC-4: peer B was sent no Cease")

	panicsAfter, _ := bgpPanics().guard.Panics("read")
	require.Equal(t, panicsBefore+1, panicsAfter, "AC-2: one contained panic counted")

	// Peer A reconnects: its run loop starts a new session after its backoff.
	require.Eventually(t, func() bool {
		current := a.peer.currentSession()
		return current != nil && current != sessionA && a.peer.SessionState() == fsm.StateActive
	}, 4*DefaultReconnectMin, 10*time.Millisecond, "AC-1: peer A's run loop starts a new session")
}

// ceaseSent reports whether the remote received a NOTIFICATION Cease with
// subcode.
//
// RFC 4271 Section 4.5: the NOTIFICATION body is Error code (1 octet), Error
// subcode (1 octet), Data (variable).
//
//	Byte offset: | 0..15 Marker | 16..17 Length | 18 Type | 19 Error code | 20 Error subcode |
func ceaseSent(v *lowLivePeer, subcode uint8) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, frame := range v.frames {
		if len(frame) < message.HeaderLen+2 {
			continue
		}
		if frame[18] != byte(msgtype.TypeNOTIFICATION) {
			continue
		}
		if frame[19] == byte(message.NotifyCease) && frame[20] == subcode {
			return true
		}
	}
	return false
}
