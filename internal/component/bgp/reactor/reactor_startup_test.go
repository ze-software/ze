package reactor

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

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
	reactor := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
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
	reactor := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
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
	reactor := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
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
	// an earlier draft added a peer + emitted addr-added to assert no
	// listener starts after abort, but a peer LocalAddress forces a per-peer bind
	// on the privileged BGP port (179) that fails before the injected API error
	// and is unrelated to subscription release. Listener-handler behavior is
	// already covered by reactor_iface_test.go; bus.activeSubscriptions()==0 is
	// the direct de-registration proof for this leak.
	apiErr := errors.New("test plugin server creation failed")
	bus := newFakeEventBus()
	reactor := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
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

// TestExternalServerDerivedFromMode proves the borrow/self-host decision is fixed
// at construction from Config.Standalone, not inferred at runtime from r.api.
//
// VALIDATES: P3 AC-3 -- externalServer == !Config.Standalone.
// PREVENTS: reverting to `externalServer = r.api != nil`, which let production
// silently self-host whenever a server was not injected.
func TestExternalServerDerivedFromMode(t *testing.T) {
	if r := New(&Config{}); !r.externalServer {
		t.Error("borrow-mode reactor (Standalone=false) should have externalServer=true")
	}
	if r := New(&Config{Standalone: true}); r.externalServer {
		t.Error("standalone reactor (Standalone=true) should have externalServer=false")
	}
}

// TestReactorBorrowModeErrorsWithoutServer proves a production (borrow) reactor
// started without an injected plugin server fails with a clear error and NEVER
// falls back to self-hosting (pluginServerMaker is never called).
//
// VALIDATES: P3 AC-1 -- borrow mode + no server -> errBorrowModeNoServer; the
// server maker is not invoked.
// PREVENTS: a production wiring bug (missing SetPluginServer) silently starting a
// second, reactor-owned server instead of erroring.
func TestReactorBorrowModeErrorsWithoutServer(t *testing.T) {
	reactor := New(&Config{ListenAddr: "127.0.0.1:0"}) // Standalone=false (borrow)
	reactor.pluginServerMaker = func(*pluginserver.ServerConfig, plugin.ReactorLifecycle) (*pluginserver.Server, error) {
		t.Fatal("borrow mode must not call pluginServerMaker (must not self-host)")
		return nil, errors.New("unreachable: borrow mode must not create a server")
	}

	err := reactor.StartWithContext(context.Background())
	require.ErrorIs(t, err, errBorrowModeNoServer)
	assert.False(t, reactor.Running(), "failed borrow-mode start must not mark reactor running")
	// The global listener binds before startAPIServer, so the borrow-guard abort
	// must release it (abortStartup) -- prove that on the borrow-guard path, not
	// just the standalone maker-failure path.
	assert.Nil(t, reactor.ListenAddr(), "borrow-guard abort must release the bound global listener")
	assert.Empty(t, reactor.ListenAddrs(), "no listener address should remain after borrow-guard abort")
}

// TestReactorStandaloneSelfHosts proves a standalone reactor reaches the
// self-host branch and creates its own server via pluginServerMaker (where a
// borrow-mode reactor would have errored before this).
//
// VALIDATES: P3 AC-2 -- standalone mode self-hosts (calls the server maker).
// PREVENTS: standalone consumers (ze-chaos sim, integration harness)
// accidentally getting the borrow guard.
func TestReactorStandaloneSelfHosts(t *testing.T) {
	reactor := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	makerCalled := false
	sentinel := errors.New("maker reached: standalone self-host path")
	reactor.pluginServerMaker = func(*pluginserver.ServerConfig, plugin.ReactorLifecycle) (*pluginserver.Server, error) {
		makerCalled = true
		return nil, sentinel
	}

	err := reactor.StartWithContext(context.Background())
	require.ErrorIs(t, err, sentinel)
	assert.True(t, makerCalled, "standalone mode must call pluginServerMaker to self-host")
}
