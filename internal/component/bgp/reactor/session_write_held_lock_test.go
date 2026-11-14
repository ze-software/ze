package reactor

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// TestSendUpdateHeldTakesNoSessionMutex pins the lock order sendUpdateHeld owes.
// Its caller holds writeMu, and the session's lock order is s.mu and THEN
// writeMu. So a sendUpdateHeld that waits on s.mu deadlocks against a holder of
// s.mu that is waiting for this caller's writeMu. connectionEstablished is such
// a holder today. closeConn was one when this test was written, and that was
// the shape of a prefix-limit teardown landing during the initial sync: the
// peer never ran again (test/plugin/prefix-teardown-reconnect-backoff.ci timed
// out under load). The close no longer holds s.mu while it waits for the write
// path (closeConnWithin), and the order still binds every other holder.
//
// The test takes the two locks in that holder's position (s.mu held) and in the
// initial sync's position (writeMu held), and asks sendUpdateHeld to finish.
func TestSendUpdateHeldTakesNoSessionMutex(t *testing.T) {
	settings := NewPeerSettings(
		netip.MustParseAddr("192.0.2.1"),
		65001, 65002, 0x01020301,
	)
	session := NewSession(settings)

	session.holdWrites()
	defer session.releaseWrites()

	// The position of a holder of s.mu that next wants writeMu.
	session.mu.Lock()
	defer session.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- session.sendUpdateHeld(&message.Update{})
	}()

	select {
	case err := <-done:
		// The session never reached Established, so the answer is the refusal.
		if !errors.Is(err, ErrInvalidState) {
			t.Fatalf("sendUpdateHeld = %v, want ErrInvalidState", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sendUpdateHeld blocked on s.mu while its caller held writeMu: lock-order inversion with closeConn")
	}
}
