// Tests for the drain caller lifecycle through superviseProcess, with real
// plugin generations started by runPluginPhase (drain.go startDrainCaller,
// failure_policy.go superviseProcess).
// VALIDATES: spec-daemon-drain-before-stop AC-35 (a)-(d), and that a plugin
// with no DrainBudget gets no caller and is not in a drain round.
// PREVENTS: a caller that outlives its generation, a respawn meeting the old
// caller, and a drain sent to a plugin that never declared one.

package server

import (
	"context"
	"net"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// drainPluginSpec describes one lifecycle test plugin.
type drainPluginSpec struct {
	name   string
	budget int // seconds; 0 registers no OnDrain and declares no budget
	policy rpc.FailurePolicy
	hold   bool // the drain handler waits for its context instead of answering
}

// drainPluginProbe is what a lifecycle test plugin reports.
type drainPluginProbe struct {
	starts  atomic.Int64
	entered chan struct{} // one send per drain handler entry, never blocking
	// respawned runs at the start of every generation after the first, which
	// is inside applyFailurePolicy's restart, so it sees what the policy saw.
	respawned atomic.Pointer[func()]
}

// registerDrainPlugin registers a plugin whose FIRST generation ends its own
// engine when crash closes; later generations keep running.
func registerDrainPlugin(t *testing.T, spec drainPluginSpec, crash <-chan struct{}) *drainPluginProbe {
	t.Helper()
	probe := &drainPluginProbe{entered: make(chan struct{}, 4)}

	require.NoError(t, registry.Register(registry.Registration{
		Name:        spec.name,
		Description: "drain lifecycle test plugin",
		RunEngine: func(conn net.Conn) int {
			generation := probe.starts.Add(1)
			p := sdk.NewWithConn(spec.name, conn)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if generation == 1 && crash != nil {
				go crashOnSignal(crash, cancel)
			}
			if check := probe.respawned.Load(); generation > 1 && check != nil {
				(*check)()
			}
			if spec.budget > 0 {
				p.OnDrain(func(ctx context.Context, _ rpc.DrainInput) error {
					select {
					case probe.entered <- struct{}{}:
					default:
					}
					if spec.hold {
						<-ctx.Done()
						return ctx.Err()
					}
					return nil
				})
			}
			if err := p.Run(ctx, sdk.Registration{FailurePolicy: spec.policy, DrainBudget: spec.budget}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	}))
	return probe
}

func drainCallerOf(s *Server, proc *process.Process) *drainCaller {
	s.drainMu.Lock()
	defer s.drainMu.Unlock()
	return s.drainCallers[proc]
}

func drainCallerCount(s *Server) int {
	s.drainMu.Lock()
	defer s.drainMu.Unlock()
	return len(s.drainCallers)
}

func isClosed(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// TestDrainCallerLifecycle proves AC-35 (a)-(d) over real generations: one
// caller for a drain-registered generation, none for a generation without a
// budget; when the generation ends with its caller idle, the caller is out of
// the set and has returned by the time the failure policy respawns the
// plugin; the respawn gets one new caller and the old one stays returned.
func TestDrainCallerLifecycle(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	crash := make(chan struct{})
	life := drainPluginSpec{name: "drain-life", budget: 2, policy: rpc.FailureRestart}
	none := drainPluginSpec{name: "drain-none", policy: rpc.FailureIgnore}
	probe := registerDrainPlugin(t, life, crash)
	registerDrainPlugin(t, none, nil)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: life.name, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: none.name, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	pm := s.procManager.Load()
	require.NotNil(t, pm)
	first := pm.GetProcess(life.name)
	require.NotNil(t, first)
	noBudget := pm.GetProcess(none.name)
	require.NotNil(t, noBudget)

	// (a) and (b).
	require.Eventually(t, func() bool { return drainCallerOf(s, first) != nil }, failureWait, 10*time.Millisecond,
		"a generation that declared a DrainBudget has no drain caller")
	assert.Equal(t, 1, drainCallerCount(s), "exactly one caller: the plugin without a budget must have none")
	assert.Nil(t, drainCallerOf(s, noBudget), "a generation with no DrainBudget got a drain caller")
	old := drainCallerOf(s, first)

	// (c) idle, then (d). The check runs inside the respawn, so it observes
	// the caller set as applyFailurePolicy left it, not after it returned.
	var oldReturned, oldRemoved atomic.Bool
	check := func() {
		oldReturned.Store(isClosed(old.done))
		oldRemoved.Store(drainCallerOf(s, first) == nil)
	}
	probe.respawned.Store(&check)
	close(crash)
	require.Eventually(t, func() bool { return pm.GetProcess(life.name) != first }, failureWait, 10*time.Millisecond,
		"the plugin exited and the failure policy never respawned it")
	assert.True(t, oldReturned.Load(), "the old caller had not returned when the respawn ran")
	assert.True(t, oldRemoved.Load(), "the ended generation's caller was still in the set when the respawn ran")

	second := pm.GetProcess(life.name)
	require.Eventually(t, func() bool { return drainCallerOf(s, second) != nil }, failureWait, 10*time.Millisecond,
		"the respawned generation has no drain caller")
	assert.NotSame(t, old, drainCallerOf(s, second), "the respawn reused the old caller")
	assert.Equal(t, 1, drainCallerCount(s), "a respawn must leave one caller for the plugin")
	assert.Equal(t, int64(2), probe.starts.Load())
}

// TestDrainCallerLifecycleEndsDuringCall proves AC-35 (c) for a caller that
// waits on a drain call when its generation ends: the call returns with the
// connection, the round names the plugin failed well before the deadline,
// and the caller is out of the set and returned.
func TestDrainCallerLifecycleEndsDuringCall(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	crash := make(chan struct{})
	hold := drainPluginSpec{name: "drain-hold", budget: 5, policy: rpc.FailureIgnore, hold: true}
	probe := registerDrainPlugin(t, hold, crash)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: hold.name, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	pm := s.procManager.Load()
	require.NotNil(t, pm)
	proc := pm.GetProcess(hold.name)
	require.NotNil(t, proc)
	require.Eventually(t, func() bool { return drainCallerOf(s, proc) != nil }, failureWait, 10*time.Millisecond,
		"a generation that declared a DrainBudget has no drain caller")
	caller := drainCallerOf(s, proc)

	round := s.BeginDrain()
	require.Equal(t, 1, round.Plugins())
	deadline := time.Now().Add(30 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	results := make(chan DrainResult, 1)
	go func() { results <- round.Run(ctx, stopInput(deadline)) }()

	select {
	case <-probe.entered:
	case <-time.After(failureWait):
		t.Fatal("the drain never reached the plugin's handler")
	}
	close(crash)

	select {
	case result := <-results:
		assert.True(t, slices.Equal(result.Failed, []string{hold.name}), "failed %v, want %s", result.Failed, hold.name)
		assert.Empty(t, result.Late, "a generation that ended mid-call was reported late")
		assert.Empty(t, result.Answered)
	case <-time.After(failureWait):
		t.Fatal("the round still waits after the plugin's generation ended")
	}
	require.Eventually(t, func() bool { return drainCallerOf(s, proc) == nil && isClosed(caller.done) },
		failureWait, 10*time.Millisecond, "the ended generation's caller is still in the set or running")
}

// TestDrainSkipsPluginsWithoutBudget proves a drain round holds only the
// plugins that declared a DrainBudget: the round over one plugin with a budget
// and one without counts one plugin and answers for that one alone.
func TestDrainSkipsPluginsWithoutBudget(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	with := drainPluginSpec{name: "drain-with", budget: 2, policy: rpc.FailureIgnore}
	without := drainPluginSpec{name: "drain-without", policy: rpc.FailureIgnore}
	probe := registerDrainPlugin(t, with, nil)
	registerDrainPlugin(t, without, nil)

	s, _ := newLifecycleStartupServer(t)
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
		{Name: with.name, Internal: true, Encoder: plugin.EncodingJSON},
		{Name: without.name, Internal: true, Encoder: plugin.EncodingJSON},
	}))
	pm := s.procManager.Load()
	require.NotNil(t, pm)
	proc := pm.GetProcess(with.name)
	require.NotNil(t, proc)
	require.Eventually(t, func() bool { return drainCallerOf(s, proc) != nil }, failureWait, 10*time.Millisecond,
		"the plugin with a budget has no drain caller")

	round := s.BeginDrain()
	assert.Equal(t, 1, round.Plugins(), "the round must hold only the plugin with a budget")
	deadline := time.Now().Add(10 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	result := round.Run(ctx, stopInput(deadline))
	assert.Equal(t, []string{with.name}, result.Answered)
	assert.Empty(t, result.Failed)
	assert.Len(t, probe.entered, 1, "the plugin with a budget must receive exactly one drain")
}
