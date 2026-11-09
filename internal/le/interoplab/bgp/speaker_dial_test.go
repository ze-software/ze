package bgp

import (
	"errors"
	"net"
	"syscall"
	"testing"
	"time"
)

// closedLoopbackAddress reserves a loopback port and releases it, so a dial
// to the returned address is refused until somebody listens on it again.
func closedLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a loopback port: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release the loopback port: %v", err)
	}
	return address
}

type speakerDialResult struct {
	connection net.Conn
	err        error
}

// Goal: the speaker connects to a peer whose listener comes up after the
// first dial, as Ze's does when a scenario starts the speaker with no
// --connect-delay.
// Method: dial a released loopback port with a 5 s deadline, open a listener
// on that port 300 ms later, and require both the dial and the Accept to
// succeed.
// MUTATION: replace the loop in dialSpeakerPeer with a single dialer.DialContext and return its error; this test then fails with ECONNREFUSED.
func TestDialSpeakerPeerWaitsForLateListener(t *testing.T) {
	address := closedLoopbackAddress(t)
	results := make(chan speakerDialResult, 1)
	go func() {
		connection, err := dialSpeakerPeer(address, time.Now().Add(5*time.Second))
		results <- speakerDialResult{connection: connection, err: err}
	}()

	time.Sleep(300 * time.Millisecond)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("listen on %s: %v", address, err)
	}
	defer func() { _ = listener.Close() }()

	tcpListener, isTCP := listener.(*net.TCPListener)
	if !isTCP {
		t.Fatalf("listener on %s is %T, want *net.TCPListener", address, listener)
	}
	if err := tcpListener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set accept deadline: %v", err)
	}
	accepted, acceptErr := listener.Accept()
	if acceptErr == nil {
		defer func() { _ = accepted.Close() }()
	}

	result := <-results
	if result.err != nil {
		t.Fatalf("dialSpeakerPeer(%s): %v", address, result.err)
	}
	defer func() { _ = result.connection.Close() }()
	if acceptErr != nil {
		t.Fatalf("accept on %s: %v", address, acceptErr)
	}
}

// Goal: a peer that never listens fails the dial at the deadline with the
// refusal it answered, not a context error, and no earlier or much later.
// Method: dial a released loopback port with a 600 ms deadline, require
// ECONNREFUSED and an elapsed time between 550 ms and 3 s.
func TestDialSpeakerPeerReturnsLastErrorAtDeadline(t *testing.T) {
	address := closedLoopbackAddress(t)
	start := time.Now()
	connection, err := dialSpeakerPeer(address, start.Add(600*time.Millisecond))
	elapsed := time.Since(start)
	if err == nil {
		_ = connection.Close()
		t.Fatalf("dialSpeakerPeer(%s) connected to a port nobody listens on", address)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("dialSpeakerPeer(%s) error = %v, want ECONNREFUSED", address, err)
	}
	if elapsed < 550*time.Millisecond {
		t.Fatalf("dialSpeakerPeer gave up after %v, before the 600 ms deadline", elapsed)
	}
	if elapsed >= 3*time.Second {
		t.Fatalf("dialSpeakerPeer returned after %v, long past the 600 ms deadline", elapsed)
	}
}
