// Related: actions.go -- the signal root these tests drive

package verifyevidence

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
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

// VALIDATES: the kernel check, the step before the container, runs under the
// signal root: a SIGINT ends its cross-compile and the run answers the
// interruption at once.
// METHOD: the root is signaled before Run, so the production dockerKernel
// meets an ended context. Rooted on context.Background instead, it starts the
// real cross-compile, which a terminal Ctrl-C no longer reaches (the
// compilers head a process group of their own), and answers no interruption.
// PREVENTS: a Ctrl-C during `./le verify evidence release-candidate` waiting
// out a ten-minute cross-compile.
func TestReleaseCandidateKernelStepCancelsOnSignal(t *testing.T) {
	held := make(chan os.Signal, 1)
	signal.Notify(held, syscall.SIGINT)
	defer signal.Stop(held)

	// The step meets an ended context and dials no daemon. Naming one keeps
	// the verdict off the host: on macOS an unset DOCKER_HOST makes
	// dockerhost.Environment refuse, before the context is read, whenever the
	// `ze` Colima profile is stopped.
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/ze-test-docker-stub.sock")
	tree := t.TempDir()
	runner := &Runner{
		Tree:     tree,
		Look:     func(string) error { return nil },
		Ask:      func(...string) (string, error) { return "", nil },
		Docker:   interoplab.NewFixtureDocker(nil),
		Progress: io.Discard,
		Kernel:   dockerKernel(tree),
	}
	var err error
	var took time.Duration
	signalRooted(func(ctx context.Context) (any, int) {
		if killErr := syscall.Kill(os.Getpid(), syscall.SIGINT); killErr != nil {
			t.Errorf("signal self: %v", killErr)
			return nil, 0
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			t.Error("the root was not canceled by SIGINT")
			return nil, 0
		}
		start := time.Now()
		_, err = runner.Run(ctx)
		took = time.Since(start)
		return nil, 1
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run after SIGINT answered %v, want the interruption (context.Canceled)", err)
	}
	if took > 5*time.Second {
		t.Fatalf("run after SIGINT took %v, want it to stop at once", took)
	}
}
