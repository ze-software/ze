// Design: docs/architecture/l2tp/l2tp-9-observer.md -- original native root lifetime

package l2tp

import (
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
)

// These tests use real listeners, reactors and Start/Stop. Only the privileged
// module probe and native worker constructor are disabled; no native I/O or
// pending-cleanup success is claimed. Cleanup MUST join the original root.
func cqmAPIRoot(t *testing.T, addresses []netip.AddrPort) *Subsystem {
	t.Helper()
	previousProbe, previousWorker := probeKernelModulesFn, newSubsystemKernelWorkerFn
	probeKernelModulesFn = func() error { return nil }
	newSubsystemKernelWorkerFn = func(chan<- kernelSetupFailed, chan<- kernelSetupSucceeded, *slog.Logger, kernelBackendPreparation) (*kernelWorker, error) {
		return nil, nil //nolint:nilnil // an inert root: Start runs with no native worker, the state it keeps after a worker error, without faking one
	}
	t.Cleanup(func() { probeKernelModulesFn, newSubsystemKernelWorkerFn = previousProbe, previousWorker })
	previousRegistry, previousBinding := registry.GetMetricsRegistry(), l2tpMetricsPtr.Load()
	registry.SetMetricsRegistry(metrics.NewPrometheusRegistry())
	t.Cleanup(func() { registry.SetMetricsRegistry(previousRegistry); l2tpMetricsPtr.Store(previousBinding) })
	root := NewSubsystem(&Parameters{
		Enabled: true, ListenAddrs: addresses, MaxTunnels: 4, MaxSessions: 4,
		MRU: 1500, CQMEnabled: true, MaxLogins: 2, EventRingSizePerSession: 8,
		SampleRetentionSeconds: 300,
	})
	t.Cleanup(func() { require.NoError(t, root.Stop(t.Context())) })
	return root
}

// MUTATION: allocating a scope per observer/listener, or retaining the old
// scope across restart, separates or aliases the actual original native root.
func TestObserverRootSharesOriginalScopeAndMintsNextRestart(t *testing.T) {
	endpoint := netip.MustParseAddrPort("127.0.0.1:0")
	root := cqmAPIRoot(t, []netip.AddrPort{endpoint, endpoint})
	bus := newCQMNativeBus(t)
	require.NoError(t, root.Start(t.Context(), bus, nil))
	require.Len(t, root.reactors, 2)
	original := root.observer
	require.NotNil(t, original)
	generation := original.generation
	require.True(t, generation.IsValid())
	require.NotNil(t, original.metrics, "the registry binds before observer construction")
	require.Same(t, l2tpMetricsPtr.Load(), original.metrics)
	for _, reactor := range root.reactors {
		require.Equal(t, generation, reactor.params.Generation)
		require.Same(t, original, reactor.observer)
		require.True(t, reactor.listener.Addr().IsValid())
	}
	require.NoError(t, root.Stop(t.Context()))
	require.Nil(t, root.observer)
	require.Equal(t, observerClosed, original.admission)
	require.Empty(t, root.retiredReactors, "a clean empty root releases its original observer")
	require.NoError(t, root.Start(t.Context(), bus, nil))
	require.NotNil(t, root.observer)
	require.NotEqual(t, generation, root.observer.generation)
	for _, reactor := range root.reactors {
		require.Same(t, root.observer, reactor.observer)
		require.Equal(t, root.observer.generation, reactor.params.Generation)
	}
	// A valid delayed delivery from the prior scope must not allocate a ring
	// in the restarted native root, while its own original neighbor remains useful.
	birth := cqmAPIBirth(t)
	require.NoError(t, root.observer.observeEvent(generation, cqmAPIEvent(birth, ObserverEventSessionUp, time.Now(), 0), ""))
	require.Empty(t, root.observer.SessionSummaries())
	cqmAPIEstablished(t, root.observer, root.observer.generation, birth, "alice", time.Now())
	require.Len(t, root.observer.SessionEvents(birth), 3)
}

// TestNativeRootScopeExistsWithQualityMonitoringDisabled starts real listeners
// with CQM disabled so the native event scope cannot depend on that optional
// reader. MUTATION: allocating only inside the CQM branch leaves zero stamps.
func TestNativeRootScopeExistsWithQualityMonitoringDisabled(t *testing.T) {
	endpoint := netip.MustParseAddrPort("127.0.0.1:0")
	root := cqmAPIRoot(t, []netip.AddrPort{endpoint, endpoint})
	root.params.CQMEnabled = false
	require.NoError(t, root.Start(t.Context(), newCQMNativeBus(t), nil))
	require.Nil(t, root.observer)
	require.Len(t, root.reactors, 2)
	generation := root.reactors[0].params.Generation
	require.True(t, generation.IsValid())
	for _, reactor := range root.reactors {
		require.Equal(t, generation, reactor.params.Generation)
		require.Nil(t, reactor.observer)
	}
}

type cqmAPIUnwindBus struct {
	*cqmAPIBus
	root     *Subsystem
	captured *Observer
}

func (bus *cqmAPIUnwindBus) Subscribe(namespace, event string, handler func(any)) func() {
	if bus.root.observer != nil {
		bus.captured = bus.root.observer
	}
	return bus.cqmAPIBus.Subscribe(namespace, event, handler)
}

// MUTATION: leaving the observer or its subscriptions open during partial
// listener-start unwind lets a copied callback publish into an unstarted root.
func TestObserverPartialRootStartUnwindClosesOriginalSubscriptions(t *testing.T) {
	blocker, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, blocker.Close()) })
	occupied := netip.MustParseAddrPort(blocker.LocalAddr().String())
	root := cqmAPIRoot(t, []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:0"), occupied})
	bus := &cqmAPIUnwindBus{cqmAPIBus: newCQMNativeBus(t), root: root}
	require.Error(t, root.Start(t.Context(), bus, nil))
	require.NotNil(t, bus.captured, "the original observer existed before the listener refusal")
	require.Nil(t, root.observer)
	require.Empty(t, root.reactors)
	require.Equal(t, observerClosed, bus.captured.admission)
	bus.mu.Lock()
	active := bus.active
	bus.mu.Unlock()
	require.Zero(t, active, "every original subscription unwound")
	cqmAPIEstablished(t, bus.captured, bus.captured.generation, cqmAPIBirth(t), "alice", time.Now())
	require.Empty(t, bus.captured.SessionSummaries())
	require.Empty(t, bus.captured.LoginSummaries())
}
