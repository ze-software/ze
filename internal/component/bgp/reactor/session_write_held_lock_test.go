package reactor

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestSendUpdateHeldTakesNoSessionMutex pins the lock order SendUpdateHeld owes.
// Its caller holds writeMu, and closeConn takes s.mu and THEN writeMu. So a
// SendUpdateHeld that waits on s.mu deadlocks against a closeConn that is
// waiting for this caller's writeMu. That was the shape of a prefix-limit
// teardown landing during the initial sync: the peer never ran again
// (test/plugin/prefix-teardown-reconnect-backoff.ci timed out under load).
//
// The test takes the two locks in closeConn's position (s.mu held) and in the
// initial sync's position (writeMu held), and asks SendUpdateHeld to finish.
func TestSendUpdateHeldTakesNoSessionMutex(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	session.HoldWrites()
	defer session.releaseWrites()

	// closeConn's position: s.mu held, next wanting writeMu.
	session.mu.Lock()
	defer session.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- session.SendUpdateHeld(&message.Update{})
	}()

	select {
	case err := <-done:
		// The session never reached Established, so the answer is the refusal.
		if !errors.Is(err, ErrInvalidState) {
			t.Fatalf("SendUpdateHeld = %v, want ErrInvalidState", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SendUpdateHeld blocked on s.mu while its caller held writeMu: lock-order inversion with closeConn")
	}
}
