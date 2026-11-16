// Design: docs/research/l2tpv2-ze-integration.md -- kernel backend lifetime
// Related: subsystem.go -- real startup and shutdown own the backend

package l2tp

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

type kernelBackendProbe struct {
	iface.Backend
	closes      int
	beforeClose func() error
}

func (b *kernelBackendProbe) Close() error {
	b.closes++
	if b.beforeClose != nil {
		return b.beforeClose()
	}
	return nil
}

// Real subsystem startup must capture one kernel owner for both listeners,
// preserve the configured global owner, and stop every PPP driver before Close.
// The worker is inert: this proves lifecycle order, not native kernel I/O.
func TestKernelBackendOwnedThroughSubsystemStop(t *testing.T) {
	previousWorker, previousProbe := newSubsystemKernelWorkerFn, probeKernelModulesFn
	newSubsystemKernelWorkerFn = func(chan<- kernelSetupFailed, chan<- kernelSetupSucceeded, *slog.Logger) (*kernelWorker, error) {
		return &kernelWorker{}, nil
	}
	probeKernelModulesFn = func() error { return nil }
	t.Cleanup(func() { newSubsystemKernelWorkerFn, probeKernelModulesFn = previousWorker, previousProbe })
	global, globalName := iface.GetBackend(), iface.ActiveBackendName()
	address := netip.MustParseAddrPort("127.0.0.1:0")
	sub := NewSubsystem(Parameters{Enabled: true, ListenAddrs: []netip.AddrPort{address, address}})
	owner := &kernelBackendProbe{}
	acquisitions := 0
	sub.openKernelBackend = func(name string) (iface.Backend, error) {
		if name != iface.DefaultBackendName() {
			t.Fatalf("requested backend %q", name)
		}
		acquisitions++
		return owner, nil
	}
	owner.beforeClose = func() error {
		for _, driver := range sub.pppDrivers {
			if driver == nil {
				return errors.New("kernel listener has no PPP driver")
			}
			select {
			case _, open := <-driver.EventsOut():
				if open {
					return errors.New("PPP event channel remained open at backend Close")
				}
			default:
				return errors.New("PPP driver was not stopped before backend Close")
			}
		}
		return nil
	}
	if err := sub.Start(context.Background(), nil, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sub.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if acquisitions != 1 || len(sub.reactors) != 2 {
		t.Fatalf("acquisitions=%d reactors=%d", acquisitions, len(sub.reactors))
	}
	for _, reactor := range sub.reactors {
		if reactor.kernelBackend != owner {
			t.Fatal("reactor did not retain the kernel owner")
		}
	}
	if iface.GetBackend() != global || iface.ActiveBackendName() != globalName {
		t.Fatal("kernel startup changed the configured backend")
	}
	if err := sub.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := sub.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if owner.closes != 1 {
		t.Fatalf("backend Close calls=%d", owner.closes)
	}
	if iface.GetBackend() != global || iface.ActiveBackendName() != globalName {
		t.Fatal("kernel shutdown changed the configured backend")
	}
}

// A failed local acquisition must report its cause and leave no owned lease;
// retry can acquire it once, and repeated preparation must reuse that owner.
func TestKernelBackendAcquisitionRetriesWithoutGlobalSwap(t *testing.T) {
	failure := errors.New("kernel factory unavailable")
	sub := NewSubsystem(Parameters{})
	owner := &kernelBackendProbe{}
	calls := 0
	sub.openKernelBackend = func(string) (iface.Backend, error) {
		calls++
		if calls == 1 {
			return nil, failure
		}
		return owner, nil
	}
	if err := sub.prepareKernelBackend(); !errors.Is(err, failure) {
		t.Fatalf("acquisition error=%v", err)
	}
	if sub.kernelBackend != nil {
		t.Fatal("failed acquisition published a backend")
	}
	if err := sub.prepareKernelBackend(); err != nil {
		t.Fatal(err)
	}
	if err := sub.prepareKernelBackend(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || sub.kernelBackend != owner {
		t.Fatalf("calls=%d owner=%v", calls, sub.kernelBackend)
	}
	if err := sub.closeKernelBackend(); err != nil {
		t.Fatal(err)
	}
	if err := sub.closeKernelBackend(); err != nil {
		t.Fatal(err)
	}
	if owner.closes != 1 {
		t.Fatalf("backend Close calls=%d", owner.closes)
	}
}
