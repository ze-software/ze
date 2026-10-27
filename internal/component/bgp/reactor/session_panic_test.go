package reactor

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

	var callbackRan bool
	session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		_ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
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
