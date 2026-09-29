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
	init := &Initiation{TLVs: []TLV{
		makeStringTLV(InitTLVSysName, "test-router"),
		makeStringTLV(InitTLVSysDescr, "test router"),
	}}
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

// RFC requirement: RFC7854-4.5-5 positive -- a router that closes the TCP
// session with no Termination message ends the receiver's session, and the
// receiver drops that router and the peer its Peer Up recorded.
func TestBMPReceiverEndsTheSessionWhenTheRouterLeavesWithoutTermination(t *testing.T) {
	// VALIDATES: RFC 7854 Section 4.5 -- "a monitoring station must always be
	// prepared for the session to terminate with no message". The router sends
	// an Initiation and a Peer Up, then closes the connection without a
	// Termination message.
	// PREVENTS: a receiver that cleans up only on a Termination message, so a
	// router that disappears leaves a stale router and a stale peer behind.

	server, client := net.Pipe()

	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}
	defer close(bp.stopCh)

	done := make(chan struct{})
	go func() {
		defer close(done)
		bp.handleSession(server)
	}()

	buf := make([]byte, 1024)
	init := &Initiation{TLVs: []TLV{
		makeStringTLV(InitTLVSysName, "test-router"),
		makeStringTLV(InitTLVSysDescr, "test router"),
	}}
	n := writeInitiation(buf, 0, init)
	if _, err := client.Write(buf[:n]); err != nil {
		t.Fatalf("write initiation: %v", err)
	}
	pu := &PeerUp{
		Peer:            testPeerHeader(),
		LocalPort:       179,
		RemotePort:      54321,
		SentOpenMsg:     makeBGPOpen(65001, 0x01020304),
		ReceivedOpenMsg: makeBGPOpen(65002, 0x05060708),
	}
	n = writePeerUp(buf, 0, pu)
	if _, err := client.Write(buf[:n]); err != nil {
		t.Fatalf("write peer up: %v", err)
	}
	// net.Pipe returns from Write only when the receiver has read the bytes.
	// The receiver reads this message after it processed the Peer Up, so the
	// Peer Up is recorded when this Write returns.
	n = writeInitiation(buf, 0, init)
	if _, err := client.Write(buf[:n]); err != nil {
		t.Fatalf("write second initiation: %v", err)
	}

	bp.state.mu.Lock()
	peers := len(bp.state.peers)
	bp.state.mu.Unlock()
	if peers != 1 {
		t.Fatalf("the receiver holds %d peer(s) before the close, want 1", peers)
	}
	if routers := bp.state.routerCount(); routers != 1 {
		t.Fatalf("the receiver holds %d router(s) before the close, want 1", routers)
	}

	if err := client.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the receiver did not end the session after the router closed without a termination message")
	}

	if routers := bp.state.routerCount(); routers != 0 {
		t.Errorf("the receiver holds %d router(s) after the router left, want 0", routers)
	}
	bp.state.mu.Lock()
	peers = len(bp.state.peers)
	bp.state.mu.Unlock()
	if peers != 0 {
		t.Errorf("the receiver holds %d peer(s) after the router left, want 0", peers)
	}
}
