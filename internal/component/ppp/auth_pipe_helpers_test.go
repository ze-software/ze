package ppp

import (
	"net"
	"testing"
	"time"
)

// newAuthTestSession builds a pppSession with the minimum fields the
// auth-phase handlers (PAP, CHAP, MS-CHAPv2) need plus buffered
// lifecycle and auth-event channels. eventsOut is buffered large
// enough to absorb every EventSessionDown the handler may emit via
// s.fail without blocking; tests assert on authEventsOut and on
// peerEnd wire bytes rather than on eventsOut.
//
// Shared across pap_test.go and chap_test.go so both auth codecs
// exercise identical plumbing.
func newAuthTestSession(driverEnd net.Conn) (*pppSession, chan AuthEvent) {
	authEventsOut := make(chan AuthEvent, 4)
	eventsOut := make(chan Event, 4)
	s := &pppSession{
		tunnelID:      55,
		sessionID:     66,
		chanFile:      driverEnd,
		eventsOut:     eventsOut,
		authEventsOut: authEventsOut,
		authRespCh:    make(chan authResponseMsg, 1),
		stopCh:        make(chan struct{}),
		sessStop:      make(chan struct{}),
		done:          make(chan struct{}),
		authTimeout:   2 * time.Second,
		logger:        discardLogger(),
	}
	return s, authEventsOut
}

// peerFrameReadTimeout bounds net.Pipe reads in auth-phase handler
// tests. net.Pipe has no read deadline of its own, so the helper runs
// the Read in a goroutine and races it against this timer.
const peerFrameReadTimeout = 2 * time.Second

// readPeerFrame reads one frame from the peer end of a net.Pipe and
// returns (proto, payload). Uses a goroutine to bound the otherwise
// deadline-less net.Pipe Read by peerFrameReadTimeout.
func readPeerFrame(t *testing.T, peerEnd net.Conn) (uint16, []byte) {
	t.Helper()
	type readResult struct {
		buf []byte
		err error
	}
	ch := make(chan readResult, 1)
	go func() {
		buf := make([]byte, MaxFrameLen)
		n, err := peerEnd.Read(buf)
		ch <- readResult{buf[:n], err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("peer read: %v", r.err)
		}
		proto, payload, _, err := ParseFrame(r.buf)
		if err != nil {
			t.Fatalf("ParseFrame: %v", err)
		}
		return proto, payload
	case <-time.After(peerFrameReadTimeout):
		t.Fatal("timed out reading from peer")
		return 0, nil
	}
}
