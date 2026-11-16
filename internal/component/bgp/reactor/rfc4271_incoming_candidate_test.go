// Design: docs/architecture/behavior/peer-lifecycle.md -- incoming connection ownership.
// Related: rfc4271_incoming_listener_test.go -- actual listener and outgoing-session setup.
// RFC 4271 Sections 8.2.1 and 8.2.2 -- see rfc/short/rfc4271.md.
// RFC naming: untagged -- independent DelayOpen timer regression; the admitted-connection OPEN and failure controls carry the partial RFC row's tags in their own files.
package reactor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
)

// TestIncomingCandidateDelayOpenExpires proves the second connection owns a
// DelayOpen lifecycle independently of an outgoing connection already waiting
// for its peer OPEN. Its remote sends no bytes, so a reader that merely waits
// for the remote OPEN cannot satisfy the local timer's wire action.
// The existing default-delay listener probe supplies the zero-delay control.
func TestIncomingCandidateDelayOpenExpires(t *testing.T) {
	const delay = time.Second
	r, peer, original, outgoing, timerClock := incomingListenerPeer(t, true, delay)
	delayOpenAwait(t, original.timers.IsDelayOpenTimerRunning)
	timerClock.Add(delay)
	delayOpenReadType(t, outgoing, 1)
	delayOpenAwait(t, func() bool { return original.State() == fsm.StateOpenSent })

	incoming := incomingListenerDial(t, r)
	delayOpenNoBytes(t, incoming)
	delayOpenReadType(t, incoming, 1)
	if peer.currentSession() != original {
		t.Fatal("a candidate without a remote OPEN replaced the outgoing session")
	}
	if original.State() != fsm.StateOpenSent {
		t.Fatalf("outgoing state %s, want OpenSent while incoming waits for OPEN", original.State())
	}
}
