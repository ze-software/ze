package bmp

import (
	"net"
	"testing"
	"time"
)

// RFC requirement: RFC7854-4.5-2 positive -- the monitoring station closes the
// TCP session once it has read a termination message, without waiting for the
// monitored router to close first.
func TestBMPReceiverClosesAfterTermination(t *testing.T) {
	// VALIDATES: RFC 7854 Section 4.5 -- "Likewise, the monitoring station MUST
	// close the TCP session after receiving a termination message."
	// PREVENTS: a receiver that reads the termination, keeps the session in its
	// table and waits for the router to hang up. A router that sends a
	// termination and then holds the connection open would keep a session slot
	// for as long as it liked, which is a resource the collector cannot reclaim.

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
	term := &Termination{TLVs: []TLV{makeStringTLV(TermTLVString, "done")}}
	n = writeTermination(buf, 0, term)
	if _, err := client.Write(buf[:n]); err != nil {
		t.Fatalf("write termination: %v", err)
	}

	// The router end is deliberately NOT closed. That is the whole test: the
	// session must end because the receiver read a termination, not because the
	// far end went away. The wait is far below sessionReadDeadline (30 s), so a
	// receiver that loops on waiting for the next read cannot pass by timing out.
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the receiver did not close the session after reading a termination message")
	}

	if routers := bp.state.routerCount(); routers != 0 {
		t.Errorf("the receiver holds %d router session(s) after termination, want 0", routers)
	}

	if err := client.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	close(bp.stopCh)
}
