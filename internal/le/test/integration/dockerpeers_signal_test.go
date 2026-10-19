// Related: dockerpeers.go -- the docker-peers and live-rpki children this test signals

package testintegration

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// sleeperArgv is a child holding a grandchild, both sleeping 30 s, so a stop
// that reaches only the direct child still leaves the run waiting.
var sleeperArgv = []string{"sh", "-c", "sleep 30 & sleep 30"}

// VALIDATES: AC-8 and AC-22 for docker-peers and live-rpki: SIGINT and SIGTERM
// reach the `go test` child through the signal-cancelled root, so le kills the
// child's process group and answers non-zero at once, rather than waiting out
// the child's own 900 s bound.
// METHOD: the verb's runner drives a child that would sleep for 30 s; the test
// signals itself 300 ms in and requires the runner back within 10 s, non-zero.
// PREVENTS: the root's NotifyContext swallowing the signal while the child,
// run on context.Background, runs on.
func TestDockerChildStopsOnSignal(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	for _, delivered := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		run := nativeRunner(root, func(ctx context.Context, root string) (any, int) {
			timer := time.AfterFunc(300*time.Millisecond, func() {
				if err := syscall.Kill(os.Getpid(), delivered); err != nil {
					t.Errorf("signal self: %v", err)
				}
			})
			defer timer.Stop()
			return runRequiringDocker(ctx, dockerPeersVerb, sleeperArgv, root)
		})
		began := time.Now()
		_, code := run()
		if elapsed := time.Since(began); elapsed > 10*time.Second {
			t.Errorf("%v: the runner came back after %v, want the child stopped on the signal", delivered, elapsed)
		}
		if code == 0 {
			t.Errorf("%v: code 0, want a non-zero answer for a signalled run", delivered)
		}
	}
}
