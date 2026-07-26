package server

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	pluginrpc "github.com/ze-software/ze/pkg/plugin/rpc"
)

// barrierReactor records the peer-up barrier calls the event dispatcher makes.
//
// It embeds *plugin.Coordinator purely to satisfy the rest of
// plugin.ReactorLifecycle: with no protocol reactor registered every other
// method is a no-op, so the recording below is the only behavior under test.
type barrierReactor struct {
	*plugin.Coordinator

	mu    sync.Mutex
	steps []string
}

func (b *barrierReactor) SetPeerUpBarrier(peerAddr string, expected int) {
	b.record("arm:" + peerAddr + ":" + itoa(expected))
}

func (b *barrierReactor) SignalPeerUpBarrier(peerAddr string) {
	b.record("ack:" + peerAddr)
}

func (b *barrierReactor) record(step string) {
	b.mu.Lock()
	b.steps = append(b.steps, step)
	b.mu.Unlock()
}

func (b *barrierReactor) recorded() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.steps...)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// newBarrierTestServer returns a server whose reactor records barrier calls, and
// the recorder.
func newBarrierTestServer(t *testing.T) (*pluginserver.Server, *barrierReactor) {
	t.Helper()

	rec := &barrierReactor{Coordinator: plugin.NewCoordinator(nil)}
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, rec)
	require.NoError(t, err)
	require.NoError(t, srv.StartWithContext(t.Context()))
	t.Cleanup(srv.Stop)
	return srv, rec
}

// subscribeState registers proc for peer state events, the subscription
// onPeerStateChange selects on.
func subscribeState(srv *pluginserver.Server, proc *process.Process) {
	srv.Subscriptions().Add(proc, &pluginserver.Subscription{
		Namespace: events.LookupNamespaceID("bgp"),
		EventType: events.LookupEventTypeID(bgpevents.EventState),
		Direction: events.DirUnspecified,
	})
}

// recordingResponder answers delivery RPCs like mockPluginResponder, and appends
// a step to rec so the test can assert the barrier was armed BEFORE the first
// delivery rather than merely at some point.
func recordingResponder(ctx context.Context, conn *plugipc.PluginConn, rec *barrierReactor, name string) {
	for {
		req, err := conn.ReadRequest(ctx)
		if err != nil {
			return
		}
		rec.record("deliver:" + name)
		if err := conn.SendResult(ctx, req.ID, nil); err != nil {
			return
		}
	}
}

// VALIDATES: the dispatcher arms the peer-up barrier with the number of
// barrier-declaring plugins BEFORE it delivers the peer-up event to any of them,
// and acknowledges each one only after its delivery returns.
// PREVENTS: the ordering silently inverting. Arming after the deliveries would
// leave the count set on a barrier whose acknowledgements have already been
// spent, and acknowledging before delivery would make the End-of-RIB claim a
// registration that had not happened yet
// (ai/rules/fail-closed-guards.md).
func TestOnPeerStateChangeArmsBarrierBeforeDelivering(t *testing.T) {
	barrierName, plainName := registerBarrierTestPlugins(t)
	srv, rec := newBarrierTestServer(t)

	barrierProc, barrierConn := newTestProcWithConn(t, barrierName)
	subscribeState(srv, barrierProc)
	go recordingResponder(t.Context(), barrierConn, rec, barrierName)

	plainProc, plainConn := newTestProcWithConn(t, plainName)
	subscribeState(srv, plainProc)
	go recordingResponder(t.Context(), plainConn, rec, plainName)

	onPeerStateChange(srv, testPeerInfo(), pluginrpc.SessionStateUp, "")

	steps := rec.recorded()
	require.Equal(t, "arm:10.0.0.1:1", steps[0],
		"the barrier must be armed with the one barrier-declaring plugin, before any delivery")
	require.Contains(t, steps, "ack:10.0.0.1",
		"the barrier plugin's delivery must be acknowledged")
	require.Equal(t, 1, countStep(steps, "ack:10.0.0.1"),
		"only the barrier-declaring plugin acknowledges: a plain plugin must not satisfy the barrier")
	require.Less(t, indexOf(steps, "deliver:"+barrierName), indexOf(steps, "ack:10.0.0.1"),
		"the acknowledgement must follow the delivery it acknowledges")
}

// VALIDATES: a teardown arms no barrier and acknowledges nothing.
// PREVENTS: a peer-down event arming a barrier whose acknowledgements can never
// arrive, which would delay the NEXT session's End-of-RIB to the timeout.
func TestOnPeerStateChangeDownArmsNoBarrier(t *testing.T) {
	barrierName, _ := registerBarrierTestPlugins(t)
	srv, rec := newBarrierTestServer(t)

	proc, conn := newTestProcWithConn(t, barrierName)
	subscribeState(srv, proc)
	go recordingResponder(t.Context(), conn, rec, barrierName)

	onPeerStateChange(srv, testPeerInfo(), pluginrpc.SessionStateDown, "tcp-failure")

	for _, step := range rec.recorded() {
		require.NotContains(t, step, "arm:", "a teardown must not arm the peer-up barrier")
		require.NotContains(t, step, "ack:", "a teardown carries no barrier acknowledgement")
	}
}

// VALIDATES: when a barrier plugin's delivery fails, it is not acknowledged, and
// the dispatcher lowers the expected count to what it actually obtained.
// PREVENTS: two opposite failures. Acknowledging a failed delivery would make
// the End-of-RIB claim a registration that never happened; leaving the count
// high would hold the marker for the full barrier timeout even though no further
// acknowledgement can ever arrive, since every delivery has already been
// attempted by the time this returns.
func TestOnPeerStateChangeFailedDeliveryReleasesBarrier(t *testing.T) {
	barrierName, _ := registerBarrierTestPlugins(t)
	srv, rec := newBarrierTestServer(t)

	proc, conn := newTestProcWithConn(t, barrierName)
	subscribeState(srv, proc)
	// No responder, and the plugin side is closed: the delivery RPC fails.
	require.NoError(t, conn.Close())

	onPeerStateChange(srv, testPeerInfo(), pluginrpc.SessionStateUp, "")

	steps := rec.recorded()
	require.Equal(t, "arm:10.0.0.1:1", steps[0])
	require.NotContains(t, steps, "ack:10.0.0.1",
		"a failed delivery is not an acknowledgement")
	require.Equal(t, "arm:10.0.0.1:0", steps[len(steps)-1],
		"the expected count must be lowered to what was acknowledged so the end-of-rib is not held")
}

func countStep(steps []string, want string) int {
	n := 0
	for _, s := range steps {
		if s == want {
			n++
		}
	}
	return n
}

func indexOf(steps []string, want string) int {
	for i, s := range steps {
		if s == want {
			return i
		}
	}
	return -1
}

// registerBarrierTestPlugins puts one barrier-declaring and one plain plugin in
// the registry. Names are test-local so they cannot collide with a real plugin;
// the registry has no unregister, which is why they must stay distinctive.
func registerBarrierTestPlugins(t *testing.T) (barrier, plain string) {
	t.Helper()

	barrier = "test-barrier-declaring-plugin"
	plain = "test-plain-plugin"

	base := func(name string, declares bool) registry.Registration {
		return registry.Registration{
			Name:          name,
			Description:   "peer-up barrier test fixture",
			PeerUpBarrier: declares,
			RunEngine:     func(_ net.Conn) int { return 0 },
			CLIHandler:    func(_ []string) int { return 0 },
		}
	}
	if err := registry.Register(base(barrier, true)); err != nil {
		require.ErrorIs(t, err, registry.ErrDuplicateName, "unexpected registration failure")
	}
	if err := registry.Register(base(plain, false)); err != nil {
		require.ErrorIs(t, err, registry.ErrDuplicateName, "unexpected registration failure")
	}
	return barrier, plain
}

// VALIDATES: the peer-up barrier expects exactly the barrier-declaring plugins
// among those the peer-up event is delivered to, and expects nothing on any
// other state transition.
// PREVENTS: two opposite failures. Counting a plugin that will not acknowledge
// (one that does not declare the barrier, or a teardown that delivers no
// acknowledgement at all) makes every peer's End-of-RIB wait out the barrier
// timeout. Counting none when bgp-rs is present drops the guarantee that
// "End-of-RIB sent" means "the route server has registered this peer", which is
// what a peer waiting on the marker before it sends relies on.
func TestCountPeerUpBarrier(t *testing.T) {
	barrierName, plainName := registerBarrierTestPlugins(t)

	barrierProc := process.NewProcess(plugin.PluginConfig{Name: barrierName})
	plainProc := process.NewProcess(plugin.PluginConfig{Name: plainName})
	unknownProc := process.NewProcess(plugin.PluginConfig{Name: "test-never-registered-plugin"})

	tests := []struct {
		name  string
		procs []*process.Process
		state pluginrpc.SessionState
		want  int
	}{
		{
			name:  "counts only the declaring plugin",
			procs: []*process.Process{plainProc, barrierProc, unknownProc},
			state: pluginrpc.SessionStateUp,
			want:  1,
		},
		{
			name:  "no declaring plugin expects nothing",
			procs: []*process.Process{plainProc, unknownProc},
			state: pluginrpc.SessionStateUp,
			want:  0,
		},
		{
			name:  "every delivery counts when all declare",
			procs: []*process.Process{barrierProc, barrierProc},
			state: pluginrpc.SessionStateUp,
			want:  2,
		},
		{
			name:  "teardown arms no barrier",
			procs: []*process.Process{barrierProc},
			state: pluginrpc.SessionStateDown,
			want:  0,
		},
		{
			name:  "no subscribers expects nothing",
			procs: nil,
			state: pluginrpc.SessionStateUp,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, countPeerUpBarrier(tt.procs, tt.state))
		})
	}
}

// VALIDATES: a plugin that declares the barrier is recognized by name, and an
// unregistered name is not.
// PREVENTS: the registry lookup answering "yes" for a plugin that is not loaded,
// which would expect an acknowledgement that can never arrive and delay every
// peer's End-of-RIB to the timeout.
func TestRequiresPeerUpBarrierByName(t *testing.T) {
	barrierName, plainName := registerBarrierTestPlugins(t)

	require.True(t, registry.RequiresPeerUpBarrier(barrierName))
	require.False(t, registry.RequiresPeerUpBarrier(plainName))
	require.False(t, registry.RequiresPeerUpBarrier("test-never-registered-plugin"))
}
