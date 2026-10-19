// Related: actions.go -- the signal root these tests drive

package verifyevidence

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// VALIDATES: SIGINT and SIGTERM cancel the context `./le verify evidence
// release-candidate` runs under, so RunOneShot removes the container.
// METHOD: the test holds its own signal.Notify, so a root that does not listen
// is a clean red rather than a killed test binary.
// PREVENTS: Ctrl-C killing le by the default action and leaving the
// privileged build container running in the daemon.
func TestReleaseCandidateCancelsOnSignal(t *testing.T) {
	held := make(chan os.Signal, 2)
	signal.Notify(held, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(held)

	for _, delivered := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		payload, code := signalRooted(func(ctx context.Context) (any, int) {
			if err := syscall.Kill(os.Getpid(), delivered); err != nil {
				t.Errorf("signal self: %v", err)
				return "not signaled", 0
			}
			select {
			case <-ctx.Done():
				return "canceled", 1
			case <-time.After(5 * time.Second):
				return "not canceled", 0
			}
		})
		if payload != "canceled" || code != 1 {
			t.Errorf("%v: payload %v code %d, want the root context canceled", delivered, payload, code)
		}
	}
}
