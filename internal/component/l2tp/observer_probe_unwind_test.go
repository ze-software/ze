// Design: docs/architecture/l2tp/l2tp-9-observer.md -- partial native root ownership

package l2tp

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// cqmProbeUnwindBus retains the original callback and cleanup handles from two
// bounded Start attempts. The test MUST force cleanup even on the broken source;
// each native unsubscribe MUST tolerate its earlier production release.
type cqmProbeUnwindBus struct {
	*cqmAPIBus
	root     *Subsystem
	original *Observer
	copiedUp func(any)
	unsubs   []func()
}

func (bus *cqmProbeUnwindBus) Subscribe(namespace, event string, handler func(any)) func() {
	if bus.original == nil {
		bus.original = bus.root.observer
	}
	if bus.original != nil {
		if bus.root.observer == bus.original {
			if event == l2tpevents.SessionUpEvent {
				bus.copiedUp = handler
			}
		}
	}
	unsub := bus.cqmAPIBus.Subscribe(namespace, event, handler)
	bus.unsubs = append(bus.unsubs, unsub)
	return unsub
}

// TestObserverProbeRefusalUnwindsBeforeStopAndRetry refuses the actual module
// probe after native subscriptions exist, then retries with a real UDP root.
// MUTATION: returning the probe error without unwind strands the first observer,
// admits its copied callback and leaves its subscriptions alive after retry.
func TestObserverProbeRefusalUnwindsBeforeStopAndRetry(t *testing.T) {
	// These original package seams and cached environment values are process-wide;
	// this test deliberately cannot run in parallel.
	useEnv(t, map[string]string{"ze.l2tp.skip-kernel-probe": "false"})
	root := cqmAPIRoot(t, []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:0")})
	bus := &cqmProbeUnwindBus{cqmAPIBus: newCQMNativeBus(t), root: root}
	t.Cleanup(func() {
		assert.NoError(t, root.Stop(t.Context()))
		if bus.original != nil {
			bus.original.Stop()
		}
		for _, unsub := range bus.unsubs {
			unsub()
		}
	})
	probeRefusal := errors.New("original module probe refused")
	probeKernelModulesFn = func() error { return probeRefusal }
	require.ErrorIs(t, root.Start(t.Context(), bus, nil), probeRefusal)
	require.NotNil(t, bus.original, "the observer existed before the actual probe refusal")
	require.NotNil(t, bus.copiedUp, "the original native subscription supplied this callback")
	generation := bus.original.generation
	assert.Nil(t, root.observer, "failed Start releases its only observer owner before returning")
	assert.Nil(t, root.replayUnsub, "failed Start releases its replay owner before returning")
	assert.Equal(t, observerClosed, bus.original.admission)
	bus.mu.Lock()
	active := bus.active
	bus.mu.Unlock()
	assert.Zero(t, active, "all five partially acquired subscriptions must unwind immediately")

	birth := cqmAPIBirth(t)
	bus.copiedUp(&l2tpevents.SessionUpPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: generation,
		TunnelID: 17, SessionID: 7,
	})
	assert.Empty(t, bus.original.SessionSummaries(), "even a copied original callback is closed")
	require.NoError(t, root.Stop(t.Context()))
	bus.mu.Lock()
	active = bus.active
	bus.mu.Unlock()
	assert.Zero(t, active, "Stop after refusal must leave no callback owner")

	probeKernelModulesFn = func() error { return nil }
	require.NoError(t, root.Start(t.Context(), bus, nil))
	require.NotNil(t, root.observer)
	current := root.observer
	require.NotSame(t, bus.original, current)
	require.NotEqual(t, generation, current.generation)
	require.Len(t, root.reactors, 1)
	require.True(t, root.reactors[0].listener.Addr().IsValid(), "retry binds a real UDP listener")
	require.Same(t, current, root.reactors[0].observer)
	bus.mu.Lock()
	active = bus.active
	bus.mu.Unlock()
	assert.Equal(t, 9, active, "only the retry's replay, CQM and subscriber bridge subscriptions remain")
	_, err := l2tpevents.EchoRTT.Emit(bus, &l2tpevents.EchoRTTPayload{
		Birth: birth, SubscriberID: birth.String(), Generation: current.generation,
		TunnelID: 17, SessionID: 7, Username: "alice", RTT: 11 * time.Millisecond,
	})
	require.NoError(t, err)
	require.Len(t, current.SessionEvents(birth), 1, "the current root still admits useful delivery")
	assert.Empty(t, bus.original.SessionSummaries(), "retry never reopens the refused original")
	require.NoError(t, root.Stop(t.Context()))
	assert.Equal(t, observerClosed, current.admission)
	bus.mu.Lock()
	active = bus.active
	bus.mu.Unlock()
	assert.Zero(t, active, "the successful retry releases every current subscription exactly once")
}
