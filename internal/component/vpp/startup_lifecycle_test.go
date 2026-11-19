// Design: docs/research/vpp-deployment-reference.md -- startup configuration transactions

package vpp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// The real configuration adapter waits for startup, joins the old process
// before replacement, and keeps the original config available to rollback.
func TestVPPStartupTransactionReplacesAndRollsBack(t *testing.T) {
	runtime, runs := startupLifecycleTestRuntime(t)
	tx := newVPPConfigTx(runtime)
	if err := tx.Configure(startupLifecycleSections("first")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runs:
		t.Fatal("configure started VPP before the startup phase")
	default:
	}
	started := startupLifecycleCall(runtime.Start)
	first := <-runs
	first.ready <- nil
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(startupLifecycleSections("second")); err != nil {
		t.Fatal(err)
	}
	applied := startupLifecycleCall(tx.Apply)
	second := <-runs
	select {
	case <-first.stopped:
	default:
		t.Fatal("replacement started before the old owner joined")
	}
	select {
	case err := <-applied:
		t.Fatalf("apply answered before readiness: %v", err)
	default:
	}
	if second.socket != "/run/second/api.sock" {
		t.Fatalf("replacement=%s", second.socket)
	}
	second.ready <- nil
	if err := <-applied; err != nil {
		t.Fatal(err)
	}
	rolledBack := startupLifecycleCall(tx.Rollback)
	restored := <-runs
	if restored.socket != "/run/first/api.sock" {
		t.Fatalf("rollback=%s", restored.socket)
	}
	restored.ready <- nil
	if err := <-rolledBack; err != nil {
		t.Fatal(err)
	}
}

// An apply failure must reinstall the running process itself, because the
// SDK's rollback gate does not invoke the undo of an ordinary failed apply.
func TestVPPStartupFailureRestoresRunningConfiguration(t *testing.T) {
	runtime, runs := startupLifecycleTestRuntime(t)
	tx := newVPPConfigTx(runtime)
	if err := tx.Configure(startupLifecycleSections("first")); err != nil {
		t.Fatal(err)
	}
	started := startupLifecycleCall(runtime.Start)
	(<-runs).ready <- nil
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(startupLifecycleSections("second")); err != nil {
		t.Fatal(err)
	}
	applied := startupLifecycleCall(tx.Apply)
	failed := <-runs
	want := errors.New("candidate plugin API is incompatible")
	failed.ready <- want
	restored := <-runs
	if restored.socket != "/run/first/api.sock" {
		t.Fatalf("restoration=%s", restored.socket)
	}
	select {
	case <-failed.stopped:
	default:
		t.Fatal("failed candidate was not joined before restoration")
	}
	select {
	case err := <-applied:
		t.Fatalf("apply answered before restoration: %v", err)
	default:
	}
	restored.ready <- nil
	if err := <-applied; !errors.Is(err, want) {
		t.Fatalf("apply failure=%v", err)
	}
}

// Losing both candidate and restoration opens the SDK's broken-state gate,
// which retries the original config rather than accepting an equal no-op.
func TestVPPStartupFailedRestorationRetainsBrokenTransaction(t *testing.T) {
	runtime, runs := startupLifecycleTestRuntime(t)
	tx := newVPPConfigTx(runtime)
	if err := tx.Configure(startupLifecycleSections("first")); err != nil {
		t.Fatal(err)
	}
	started := startupLifecycleCall(runtime.Start)
	(<-runs).ready <- nil
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(startupLifecycleSections("second")); err != nil {
		t.Fatal(err)
	}
	applied := startupLifecycleCall(tx.Apply)
	(<-runs).ready <- errors.New("candidate failed")
	(<-runs).ready <- errors.New("restoration failed")
	if err := <-applied; !errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("broken apply=%v", err)
	}
	rolledBack := startupLifecycleCall(tx.Rollback)
	restored := <-runs
	if restored.socket != "/run/first/api.sock" {
		t.Fatalf("repair=%s", restored.socket)
	}
	restored.ready <- nil
	if err := <-rolledBack; err != nil {
		t.Fatal(err)
	}
}

type startupLifecycleProbe struct {
	socket  string
	ready   chan error
	stopped chan struct{}
}

func startupLifecycleTestRuntime(t *testing.T) (*vppLifecycle, <-chan *startupLifecycleProbe) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	runs := make(chan *startupLifecycleProbe, 8)
	var active atomic.Int32
	runtime := newVPPLifecycle(ctx, func(ctx context.Context, settings *VPPSettings, _ bool, receipt chan<- error) error {
		if active.Add(1) != 1 {
			t.Error("two VPP process owners overlap")
		}
		probe := &startupLifecycleProbe{socket: settings.APISocket, ready: make(chan error, 1), stopped: make(chan struct{})}
		defer close(probe.stopped)
		defer active.Add(-1)
		runs <- probe
		select {
		case err := <-probe.ready:
			receipt <- err
		case <-ctx.Done():
			return nil
		}
		<-ctx.Done()
		return nil
	})
	t.Cleanup(func() {
		cancel()
		if err := runtime.Stop(); err != nil {
			t.Error(err)
		}
	})
	return runtime, runs
}

func startupLifecycleCall(call func() error) <-chan error {
	result := make(chan error, 1)
	go func() { result <- call() }()
	return result
}

func startupLifecycleSections(name string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: "vpp", Data: `{"vpp":{"enabled":"true","external":"true","api-socket":"/run/` + name + `/api.sock"}}`}}
}
