//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- caller retry ownership.
// Related: client_owner_test.go -- each actual attempt releases lifecycle mu.

package pppoeclient

import (
	"errors"
	"io"
	"syscall"
	"testing"
)

// TestClientReadPacketRetriesOnlyTransientAttempts exercises the actual Linux
// sessionLink.readPacket entry point. It retries EAGAIN/EINTR and returns each
// independent terminal unchanged. Original-file readiness/order remains the
// shared kernel native oracle and the real eager/unit Dial roots.
func TestClientReadPacketRetriesOnlyTransientAttempts(t *testing.T) {
	for _, terminal := range []error{nil, io.EOF, io.ErrUnexpectedEOF, syscall.EBADF} {
		client := testClientProtocol(&recordingRWC{})
		calls := 0
		client.read = func(buffer []byte) (int, error) {
			calls++
			switch calls {
			case 1:
				return 0, syscall.EAGAIN
			case 2:
				return 0, syscall.EINTR
			case 3:
				if terminal != nil {
					return 0, terminal
				}
				return copy(buffer, []byte{0x80, 0x21, 1, 7, 0, 4}), nil
			default:
				t.Fatal("terminal read retried")
				return 0, io.ErrUnexpectedEOF
			}
		}
		link := &sessionLink{client: client}
		var buffer [32]byte
		n, err := link.readPacket(buffer[:])
		if !errors.Is(err, terminal) {
			t.Fatalf("terminal changed: %v, want %v", err, terminal)
		}
		if calls != 3 {
			t.Fatalf("read attempts = %d", calls)
		}
		if terminal == nil {
			if string(buffer[:n]) != string([]byte{0x80, 0x21, 1, 7, 0, 4}) {
				t.Fatalf("packet changed: %x", buffer[:n])
			}
		}
		if !link.mu.TryLock() {
			t.Fatal("retry wrapper retained lifecycle lock")
		}
		link.mu.Unlock()
	}
}
