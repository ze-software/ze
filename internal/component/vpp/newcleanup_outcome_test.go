// Design: docs/research/vpp-deployment-reference.md -- joined VPP shutdown outcomes
// Related: vpp.go -- the real outer run loop and deferred driver restoration

package vpp

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestVPPManagerCleanupCauseSurvivesCancellation drives a real canceled context
// and a joined cleanup cause through the outer run loop, without native I/O.
func TestVPPManagerCleanupCauseSurvivesCancellation(t *testing.T) {
	manager := newCleanupOutcomeManager(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	want := errors.New("original managed descriptor close failed")
	calls := 0
	err := manager.runWithOps(ctx, managerRunOps{
		bindAll: manager.dpdk.BindAll,
		runOnce: func(context.Context, string) error {
			calls++
			cancel()
			return errors.Join(ctx.Err(), want)
		},
		unbindAll: manager.dpdk.UnbindAll,
	})
	if calls != 1 {
		t.Fatalf("native run calls=%d, want 1", calls)
	}
	if !errors.Is(err, want) {
		t.Fatalf("requested shutdown discarded original cleanup cause: %v", err)
	}
}

// TestVPPManagerCleanupCleanCancellation preserves successful requested shutdown.
// The same outer loop must call the selected run once and return success.
func TestVPPManagerCleanupCleanCancellation(t *testing.T) {
	manager := newCleanupOutcomeManager(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	err := manager.runWithOps(ctx, managerRunOps{
		bindAll: manager.dpdk.BindAll,
		runOnce: func(context.Context, string) error {
			calls++
			cancel()
			return ctx.Err()
		},
		unbindAll: manager.dpdk.UnbindAll,
	})
	if err != nil {
		t.Fatalf("clean requested shutdown failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("native run calls=%d, want 1", calls)
	}
}

// TestVPPManagerCleanupDeferredRestoreFailure reaches actual binder cleanup
// from Run's defer and requires that failure to reach the caller, not only logs.
func TestVPPManagerCleanupDeferredRestoreFailure(t *testing.T) {
	fixture := newDPDKCleanupFixture(t)
	fixture.bind(t)
	want := errors.New("original vfio unbind failed")
	fixture.unbindFailure = want
	manager := newCleanupOutcomeManager(t)
	manager.dpdk = fixture.binder
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err := manager.runWithOps(ctx, managerRunOps{
		bindAll: manager.dpdk.BindAll,
		runOnce: func(context.Context, string) error {
			cancel()
			return ctx.Err()
		},
		unbindAll: func() error { return fixture.binder.unbindAllWithOps(fixture.ops) },
	})
	if fixture.unbindCalls != 1 {
		t.Fatalf("original driver cleanup calls=%d, want 1", fixture.unbindCalls)
	}
	if err == nil {
		t.Fatal("Run discarded the original driver's cleanup failure")
	}
	if !strings.Contains(err.Error(), want.Error()) {
		t.Fatalf("Run returned another cause: %v", err)
	}
	original, ok := fixture.binder.savedDrivers[dpdkCleanupPCI]
	if !ok {
		t.Fatal("failed cleanup lost its original driver receipt")
	}
	if original.driver != "ixgbe" || original.override != dpdkCleanupOverride {
		t.Fatal("failed cleanup lost its original driver receipt")
	}
}

// TestVPPManagerCleanupExactChildCancellation preserves the clean outcome after
// the actual wait helper kills and reaps the specific child it was given.
func TestVPPManagerCleanupExactChildCancellation(t *testing.T) {
	manager := newCleanupOutcomeManager(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	child := newCleanupOutcomeChild(t)
	err := manager.runWithOps(ctx, managerRunOps{
		bindAll: manager.dpdk.BindAll,
		runOnce: func(context.Context, string) error {
			cancel()
			return waitVPPConnection(ctx, child, make(chan error))
		},
		unbindAll: manager.dpdk.UnbindAll,
	})
	if child.ProcessState == nil {
		t.Fatal("requested shutdown returned before its original child was reaped")
	}
	if err != nil {
		t.Fatalf("the exact child's clean requested shutdown failed: %v", err)
	}
}

// TestVPPManagerCleanupJoinedChildFailure preserves a separate cleanup error
// even beside the actual expected cancellation/kill result of that same child.
func TestVPPManagerCleanupJoinedChildFailure(t *testing.T) {
	manager := newCleanupOutcomeManager(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	child := newCleanupOutcomeChild(t)
	want := errors.New("original child descriptor release failed")
	err := manager.runWithOps(ctx, managerRunOps{
		bindAll: manager.dpdk.BindAll,
		runOnce: func(context.Context, string) error {
			cancel()
			return errors.Join(waitVPPConnection(ctx, child, make(chan error)), want)
		},
		unbindAll: manager.dpdk.UnbindAll,
	})
	if child.ProcessState == nil {
		t.Fatal("failed shutdown returned before its original child was reaped")
	}
	if !errors.Is(err, want) {
		t.Fatalf("expected child cancellation concealed the separate cleanup failure: %v", err)
	}
}

// An unresolved native cleanup failure must terminate before another child run.
func TestVPPManagerCleanupStopsBeforeRestart(t *testing.T) {
	manager := newCleanupOutcomeManager(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	want := errors.New("original child descriptor cleanup refused")
	onceCalls := 0
	err := manager.runWithOps(ctx, managerRunOps{
		bindAll: manager.dpdk.BindAll,
		runOnce: func(context.Context, string) error {
			onceCalls++
			if onceCalls == 1 {
				return nativeCleanupError{cause: want}
			}
			cancel()
			return ctx.Err()
		},
		unbindAll: func() error { return nil },
	})
	if onceCalls != 1 {
		t.Fatalf("unresolved cleanup restarted the native run: %d", onceCalls)
	}
	if !errors.Is(err, want) {
		t.Fatalf("terminal cleanup cause disappeared: %v", err)
	}
}

func newCleanupOutcomeManager(t *testing.T) *VPPManager {
	t.Helper()
	settings := &VPPSettings{
		Enabled: true, External: true, APISocket: "/unused/cleanup-api.sock",
		Memory: MemorySettings{MainHeap: "1G", HugepageSize: "2M", Buffers: 128000},
		Stats:  StatsSettings{SegmentSize: "512M", SocketPath: "/unused/cleanup-stats.sock", PollInterval: 30},
	}
	return NewVPPManager(settings, t.TempDir(), "/unused/cleanup-vpp")
}

// newCleanupOutcomeChild borrows the existing bounded child fixture. Its only
// external resources are its own process and pipes, joined by waitVPPConnection.
func newCleanupOutcomeChild(t *testing.T) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	lifetime, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	child := exec.CommandContext(lifetime, executable, "-test.run=^TestVPPManagedWaitChild$", "--", "vpp-owned-wait-child")
	output, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := input.Close(); err != nil {
			if !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		}
		if child.ProcessState == nil {
			if err := child.Process.Kill(); err != nil {
				if !errors.Is(err, os.ErrProcessDone) {
					t.Error(err)
				}
			}
			if err := child.Wait(); err != nil {
				t.Logf("cleanup reaped the test-owned child: %v", err)
			}
		}
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatalf("original child readiness: %v", err)
	}
	if line != "READY\n" {
		t.Fatalf("original child readiness=%q", line)
	}
	return child
}
