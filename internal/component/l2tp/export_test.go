package l2tp

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/pkg/ze"
)

// StartSessionEventReadersForTest subscribes this package's own readers of the
// L2TP session events to bus: the subscriber bridge, writing into registry, and
// the CQM observer. An external test reaches the readers other packages
// register through the plugin registry, and these two through this function.
// The caller MUST call the returned stop function when the test ends.
func StartSessionEventReadersForTest(bus ze.EventBus, registry *subscriber.Registry) (stop func()) {
	bridge := newSubscriberBridge(registry, bus, slog.Default())
	s := &Subsystem{observer: newObserver(testObserverConfig())}
	s.wireObserverSubscriptions(bus)
	return func() {
		bridge.stop()
		s.observer.Stop()
	}
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
