package ppp

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// VALIDATES: readFrames forwards full-sized frames onto the output
//
//	channel and drops reads shorter than 2 bytes (the PPP
//	Protocol field minimum that ParseFrame requires), logging
//	a warning and continuing to read instead of tearing the
//	session down.
//
// PREVENTS: regression where a hostile or sloppy peer could inject
//
//	a 1-byte frame that propagates to the auth handlers,
//	the LCP FSM, or ParseFrame and either panics on a
//	sub-header slice access or surfaces as a spurious
//	wire-format error.
func TestReadFramesDropsUndersizedAndForwardsRest(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	defer closeConn(peerEnd)

	s, _ := newAuthTestSession(driverEnd)

	// newAuthTestSession has already started its own readFrames
	// goroutine feeding s.framesIn; reuse that plumbing by writing
	// frames from the peer side and reading what surfaces.
	go func() {
		// 1 byte: below the 2-byte Protocol field minimum. readFrames
		// MUST drop this without exiting and without forwarding.
		if _, err := peerEnd.Write([]byte{0xAA}); err != nil {
			t.Errorf("peer write (short): %v", err)
			return
		}
		// Full LCP frame: 2-byte Protocol field + minimal LCP packet.
		buf := make([]byte, MaxFrameLen)
		off := WriteFrame(buf, 0, ProtoLCP, nil)
		off += WriteLCPPacket(buf, off, LCPConfigureRequest, 1, nil)
		if _, err := peerEnd.Write(buf[:off]); err != nil {
			t.Errorf("peer write (lcp): %v", err)
			return
		}
	}()

	select {
	case frame, ok := <-s.framesIn:
		if !ok {
			t.Fatal("framesIn closed; readFrames exited before forwarding the LCP frame")
		}
		defer putFrameBuf(frame)
		proto, _, _, perr := ParseFrame(frame)
		if perr != nil {
			t.Fatalf("ParseFrame: %v", perr)
		}
		if proto != ProtoLCP {
			t.Errorf("forwarded frame proto = 0x%04x, want ProtoLCP (0x%04x)",
				proto, ProtoLCP)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readFrames did not forward the LCP frame within 2s -- likely blocked on the undersized read")
	}

	// readFrames MUST still be running (no forwarded garbage frame
	// from the 1-byte write). Peek the channel: it should be empty.
	select {
	case extra, ok := <-s.framesIn:
		if ok {
			t.Errorf("framesIn delivered unexpected extra frame after LCP: %x", extra)
			putFrameBuf(extra)
		}
	default:
	}
}

// VALIDATES: readFrames exits when chanFile.Read returns an error
//
//	(e.g. the peer closed the pipe) and closes the frames
//	channel so downstream consumers observe the teardown via
//	receive-ok=false.
//
// PREVENTS: regression where a closed transport leaks the reader
//
//	goroutine and leaves handlers parked on framesIn.
func TestReadFramesExitsOnReadError(t *testing.T) {
	peerEnd, driverEnd := net.Pipe()
	s, _ := newAuthTestSession(driverEnd)

	closeConn(peerEnd)

	select {
	case _, ok := <-s.framesIn:
		if ok {
			t.Fatal("framesIn delivered a frame after peer close; want channel closed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("framesIn still open 2s after peer close; readFrames goroutine leaked")
	}
}

// Sanity check so the helpers above compile against the package's
// error vocabulary; not an assertion.
var _ = errors.Is(io.EOF, io.EOF)
