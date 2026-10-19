package perf

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// VALIDATES: AC-8 for `./le perf run` and `./le perf evidence`: SIGINT and
// SIGTERM cancel the root context the benchmark chain runs under, so
// perfrunner's removals run before the process exits.
// PREVENTS: a Ctrl-C that kills le by the default action and leaves the DUT,
// the sender and the network behind. The test holds its own signal.Notify, so
// a root that does not listen is a clean red, never a killed test binary.
func TestPerfVerbsCancelOnSignal(t *testing.T) {
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
