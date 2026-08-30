package bmp

import (
	"net"
	"testing"
	"time"
)

// RFC requirement: RFC7854-4.5-2 negative -- the close is owed to a termination
// message and to nothing else, so a session carrying other valid messages stays
// open and keeps its router registered.
func TestBMPReceiverKeepsSessionWithoutTermination(t *testing.T) {
	// VALIDATES: RFC 7854 Section 4.5 -- the monitoring station closes after
	// receiving a termination message. This is the other half: absent one, the
	// session is the router's to end.
	// PREVENTS: a receiver that satisfies the close obligation by hanging up on
	// any message, or after the first one. That would pass the positive test and
	// break monitoring entirely, which is why the pair is written rather than the
	// positive alone.

	server, client := net.Pipe()

	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		bp.handleSession(server)
	}()

	buf := make([]byte, 256)
	init := &Initiation{TLVs: []TLV{makeStringTLV(InitTLVSysName, "test-router")}}
	n := writeInitiation(buf, 0, init)
	if _, err := client.Write(buf[:n]); err != nil {
		t.Fatalf("write initiation: %v", err)
	}

	// A second valid non-termination message, so the assertion is about the
	// message TYPE rather than about a session that has only ever seen one.
	n = writeInitiation(buf, 0, init)
	if _, err := client.Write(buf[:n]); err != nil {
		t.Fatalf("write second initiation: %v", err)
	}

	select {
	case <-done:
		t.Fatal("the receiver closed the session with no termination message")
	case <-time.After(300 * time.Millisecond):
	}

	if routers := bp.state.routerCount(); routers != 1 {
		t.Errorf("the receiver holds %d router session(s), want 1", routers)
	}

	// End it the way the RFC expects when no termination is sent: the router
	// goes away, and the receiver's read fails.
	if err := client.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the receiver did not end the session after the router closed")
	}
	close(bp.stopCh)
}
