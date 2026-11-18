package l2tp

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/pkg/ze"
)

// StartSessionEventReadersForTest subscribes this package's own readers of the
// L2TP session events to bus: the subscriber bridge, writing into registry, and
// the CQM observer. An external test reaches the readers other packages
// register through the plugin registry, and these two through this function.
// The caller MUST call the returned stop function when the test ends.
func StartSessionEventReadersForTest(bus ze.EventBus, registry *subscriber.Registry, generation l2tpevents.ObservationGeneration) (stop func(), err error) {
	cfg := testObserverConfig()
	cfg.Generation = generation
	observer, err := newObserver(cfg)
	if err != nil {
		return nil, err
	}
	bridge := newSubscriberBridge(registry, bus, slog.Default())
	s := &Subsystem{observer: observer}
	s.wireObserverSubscriptions(bus)
	return func() {
		bridge.stop()
		s.observer.Stop()
	}, nil
}

// SetProbeKernelModulesForTest replaces the kernel module probe for the
// duration of a test. Returns a restore function the caller MUST defer
// (or pass to t.Cleanup) so subsequent tests see the production probe.
//
// NOT safe for concurrent use. Tests calling this MUST NOT call
// t.Parallel(); the helper mutates a package-level var without
// synchronization. The ze test suite runs sequentially under
// `-race -count=1`, which makes this acceptable.
func SetProbeKernelModulesForTest(fn func() error) func() {
	old := probeKernelModulesFn
	probeKernelModulesFn = fn
	return func() { probeKernelModulesFn = old }
}

// listenerKernelBackend owns no OS resource. The listener-only fixtures never
// configure a PPP interface; an unexpected interface operation still fails
// through the embedded interface rather than silently succeeding.
type listenerKernelBackend struct{ iface.Backend }

func (*listenerKernelBackend) Close() error { return nil }

// UseListenerKernelBackendForTest supplies only the per-subsystem interface
// factory needed by listener-only cases. Real UDP binding, reactor startup and
// shutdown remain in use. The configured global interface backend is untouched.
// The caller MUST install it before Start and MUST call the returned restore
// after Stop. It changes no other subsystem or process-wide registry.
func UseListenerKernelBackendForTest(sub *Subsystem) func() {
	previous := sub.openKernelBackend
	sub.openKernelBackend = func(string) (iface.Backend, error) {
		return &listenerKernelBackend{}, nil
	}
	return func() { sub.openKernelBackend = previous }
}
