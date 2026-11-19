// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- joined cleanup outcomes
// Related: connected_cleanup_test.go -- retained worker and packet ordering

package pppoeclient

import (
	"errors"
	"testing"
)

// TestConnectedCleanupReturnsOriginalCauses checks the new direct result API
// with healthy cleanup and independently injected channel, transport and final
// owner causes. The private protocol fixture grants no native PPP authority.
func TestConnectedCleanupReturnsOriginalCauses(t *testing.T) {
	channelCause := errors.New("original channel close")
	transportCause := errors.New("original transport close")
	ownerCause := errors.New("original owner cleanup")
	for _, failed := range []bool{false, true} {
		name := "healthy"
		if failed {
			name = "each cause retained"
		}
		t.Run(name, func(t *testing.T) {
			client := testClientProtocol(&recordingRWC{})
			client.channelClose = func() error {
				if failed {
					return channelCause
				}
				return nil
			}
			link := &sessionLink{
				client:  client,
				stopped: make(chan struct{}),
				closeTransport: func() error {
					if failed {
						return transportCause
					}
					return nil
				},
			}
			done := make(chan struct{})
			close(done)
			calls := 0
			err := link.cleanupConnected(done, func() error {
				calls++
				select {
				case <-link.stopped:
				default:
					t.Fatal("owner cleanup preceded transport stop")
				}
				if !link.mu.TryLock() {
					t.Fatal("lifecycle lock spans owner callback")
				}
				link.mu.Unlock()
				if failed {
					return ownerCause
				}
				return nil
			})
			if calls != 1 {
				t.Fatalf("owner callback calls = %d", calls)
			}
			if !failed {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			for _, cause := range []error{channelCause, transportCause, ownerCause} {
				if !errors.Is(err, cause) {
					t.Errorf("cleanup lost %v: %v", cause, err)
				}
			}
		})
	}
}
