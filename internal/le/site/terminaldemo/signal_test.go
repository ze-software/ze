package siteterminaldemo

import (
	"bytes"
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
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

// VALIDATES: G-2 for image-build: SIGINT or SIGTERM delivered to le while the
// renderer image builds cancels the context the docker build runs under.
// METHOD: the fixture answers `docker build` by signaling this process and
// waiting on the build's context. The test holds its own subscription to both
// signals, so a build that no signal root reaches leaves the process alive and
// answers "built" after the wait instead of the default action killing the
// test binary.
// PREVENTS: image-build rooted on context.Background, where a SIGTERM to le
// alone (no terminal Ctrl-C reaching the docker client) leaves the build
// running.
func TestImageBuildCancelsOnSignal(t *testing.T) {
	held := make(chan os.Signal, 2)
	signal.Notify(held, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(held)

	fixture := newDemoFixture(t)
	saved := imageBuildEngine
	t.Cleanup(func() { imageBuildEngine = saved })
	for _, delivered := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		build := &signalingBuild{t: t, signal: delivered}
		imageBuildEngine = func(ctx context.Context) (*Engine, error) {
			engine := fixture.engine(&bytes.Buffer{})
			engine.ctx = ctx
			engine.docker = interoplab.NewFixtureDocker(build)
			return engine, nil
		}
		_, code := runImageBuild()
		if !build.canceled {
			t.Errorf("%v: the docker build ran to the end, want its context canceled", delivered)
		}
		if code != 1 {
			t.Errorf("%v: image-build exited %d, want 1 for an interrupted build", delivered, code)
		}
	}
}

// signalingBuild is a Docker process boundary whose `docker build` signals
// this process and waits on its context; every other command succeeds.
type signalingBuild struct {
	t        *testing.T
	signal   syscall.Signal
	canceled bool
}

func (b *signalingBuild) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	if len(command.Arguments) < 2 || command.Arguments[1] != "build" {
		return interoplab.ProcessResult{}, nil
	}
	if err := syscall.Kill(os.Getpid(), b.signal); err != nil {
		b.t.Errorf("signal self: %v", err)
		return interoplab.ProcessResult{}, err
	}
	select {
	case <-ctx.Done():
		b.canceled = true
		return interoplab.ProcessResult{}, ctx.Err()
	case <-time.After(5 * time.Second):
		return interoplab.ProcessResult{Stdout: "sha256:fixture-renderer\n"}, nil
	}
}
