package reactor

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type startupFailingListenerFactory struct {
	err error
}

func (f startupFailingListenerFactory) Listen(context.Context, string, string) (net.Listener, error) {
	return nil, f.err
}

func TestStartWithContextCleansUpAfterAPIServerFailure(t *testing.T) {
	// VALIDATES: AC-1, AC-2 -- late API-server creation failure aborts provisional listener and cache resources.
	// PREVENTS: Failed startup leaving a bound TCP listener, live cache scanner, or uncanceled reactor context.
	apiErr := errors.New("test plugin server creation failed")
	reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
	reactor.pluginServerMaker = func(*pluginserver.ServerConfig, plugin.ReactorLifecycle) (*pluginserver.Server, error) {
		return nil, apiErr
	}

	err := reactor.StartWithContext(context.Background())
	require.ErrorIs(t, err, apiErr)
	assert.False(t, reactor.Running(), "failed startup must not mark reactor running")
	assert.Nil(t, reactor.ListenAddr(), "global listener must be cleared on failed startup")
	assert.Empty(t, reactor.ListenAddrs(), "no listener address should remain after abort cleanup")

	require.NotNil(t, reactor.ctx, "startup context should exist before abort")
	select {
	case <-reactor.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("reactor context was not canceled by abort cleanup")
	}

	stopCh := reactor.recentUpdates.stopCh
	require.NotNil(t, stopCh, "cache scanner must have been started before injected API failure")
	select {
	case <-stopCh:
	case <-time.After(time.Second):
		t.Fatal("recent update cache scanner was not stopped by abort cleanup")
	}
}

func TestStartWithContextCleansUpAfterListenerFailure(t *testing.T) {
	// VALIDATES: AC-2 -- listener startup failure after cache/context startup aborts provisional resources.
	// PREVENTS: Failed listener bind leaving the cache scanner running or reactor context ambiguous.
	listenErr := errors.New("test listener bind failed")
	reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
	reactor.SetListenerFactory(startupFailingListenerFactory{err: listenErr})

	err := reactor.StartWithContext(context.Background())
	require.ErrorIs(t, err, listenErr)
	assert.False(t, reactor.Running(), "failed listener startup must not mark reactor running")
	assert.Nil(t, reactor.ListenAddr(), "failed listener must not remain registered")

	require.NotNil(t, reactor.ctx, "startup context should exist before listener failure")
	select {
	case <-reactor.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("reactor context was not canceled after listener failure")
	}

	stopCh := reactor.recentUpdates.stopCh
	require.NotNil(t, stopCh, "cache scanner must have been started before listener failure")
	select {
	case <-stopCh:
	case <-time.After(time.Second):
		t.Fatal("recent update cache scanner was not stopped after listener failure")
	}
}

func TestStopAfterFailedStartupIsSafe(t *testing.T) {
	// VALIDATES: AC-3 -- Stop after failed startup is idempotent and does not double-close provisional resources.
	// PREVENTS: Panic or stale listener state when callers defensively Stop after StartWithContext returns an error.
	apiErr := errors.New("test plugin server creation failed")
	reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
	reactor.pluginServerMaker = func(*pluginserver.ServerConfig, plugin.ReactorLifecycle) (*pluginserver.Server, error) {
		return nil, apiErr
	}

	err := reactor.StartWithContext(context.Background())
	require.ErrorIs(t, err, apiErr)

	require.NotPanics(t, func() {
		reactor.Stop()
		reactor.Stop()
	})
	assert.Nil(t, reactor.ListenAddr(), "Stop after failed startup must not resurrect or retain listener state")
	assert.Empty(t, reactor.ListenAddrs())
}

// fakeEventBus is a minimal ze.EventBus for the startup-cleanup tests. It
// mirrors the production bus semantics that matter here: Subscribe returns a
// non-blocking, idempotent unsubscribe; Emit copies matching handlers under the
// lock and invokes them outside it.
type fakeEventBus struct {
	mu     sync.Mutex
	nextID int
	subs   map[int]fakeSub
}

type fakeSub struct {
	namespace string
	eventType string
	handler   func(any)
}

func newFakeEventBus() *fakeEventBus {
	return &fakeEventBus{subs: make(map[int]fakeSub)}
}

func (b *fakeEventBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	if handler == nil {
		return func() {}
	}
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	b.subs[id] = fakeSub{namespace: namespace, eventType: eventType, handler: handler}
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
	}
}

func (b *fakeEventBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	var matched []func(any)
	for _, s := range b.subs {
		if s.namespace == namespace && s.eventType == eventType {
			matched = append(matched, s.handler)
		}
	}
	b.mu.Unlock()
	for _, h := range matched {
		h(payload)
	}
	return 0, nil
}

func (b *fakeEventBus) activeSubscriptions() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

func TestStartWithContextReleasesEventSubscriptionsOnFailure(t *testing.T) {
	// VALIDATES: A failed startup releases the EventBus subscriptions registered
	// by SubscribeInterfaceEvents (called before any abort-guarded failure), so
	// (interface, addr-*) handlers do not leak on the bus after StartWithContext
	// returns an error. activeSubscriptions()==0 proves the production
	// unsubscribe funcs ran and removed the handlers from the bus.
	// PREVENTS: BENG-004 -- abortStartup leaving interface event handlers
	// subscribed against a half-torn-down reactor (the Task/Security-Review
	// promise that no event subscription may remain after a failed startup).
	//
	// test-relax: an earlier draft added a peer + emitted addr-added to assert no
	// listener starts after abort, but a peer LocalAddress forces a per-peer bind
	// on the privileged BGP port (179) that fails before the injected API error
	// and is unrelated to subscription release. Listener-handler behavior is
	// already covered by reactor_iface_test.go; bus.activeSubscriptions()==0 is
	// the direct de-registration proof for this leak.
	apiErr := errors.New("test plugin server creation failed")
	bus := newFakeEventBus()
	reactor := New(&Config{ListenAddr: "127.0.0.1:0"})
	reactor.SetEventBus(bus)
	reactor.pluginServerMaker = func(*pluginserver.ServerConfig, plugin.ReactorLifecycle) (*pluginserver.Server, error) {
		return nil, apiErr
	}

	require.Zero(t, bus.activeSubscriptions(), "no subscriptions before start")

	err := reactor.StartWithContext(context.Background())
	require.ErrorIs(t, err, apiErr)

	assert.Nil(t, reactor.eventBusUnsubs, "failed startup must clear tracked unsubscribe funcs")
	assert.Zero(t, bus.activeSubscriptions(), "failed startup must unsubscribe all EventBus handlers")
}
