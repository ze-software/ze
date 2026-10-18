package siteterminaldemo

import (
	"bytes"
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

// VALIDATES: AC-8 for the terminal-demo render: SIGINT or SIGTERM delivered
// while a renderer action runs cancels the root context the demo container
// runs under, so RunOneShot removes the container.
// PREVENTS: an interrupt killing le while the render container keeps running.
// Method: the action body signals this process and waits on its context, in
// the shape of TestNativeRunnerCancelsOnSIGINT.
func TestRendererActionCancelsOnSIGINT(t *testing.T) {
	for _, signal := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		payload, code := signalRooted(func(ctx context.Context) (any, int) {
			if err := syscall.Kill(os.Getpid(), signal); err != nil {
				t.Fatalf("signal self: %v", err)
			}
			select {
			case <-ctx.Done():
				return "canceled", 1
			case <-time.After(10 * time.Second):
				return "not canceled", 0
			}
		})
		if payload != "canceled" || code != 1 {
			t.Errorf("%v: payload %v code %d, want the root context canceled", signal, payload, code)
		}
	}
}

// VALIDATES: the root context reaches the demo container's RunOneShot.
// PREVENTS: a signal root that cancels a context nothing reads.
func TestDemoContainerRunsUnderTheEngineContext(t *testing.T) {
	fixture := newDemoFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	engine := fixture.engine(&bytes.Buffer{})
	engine.ctx = ctx
	if _, _, err := executeRenderer(engine, "validate", ""); err == nil {
		t.Fatal("validation under a canceled root context succeeded")
	}
	if fixture.canceledRuns == 0 {
		t.Error("no demo container saw the canceled root context")
	}
}
