// Design: docs/architecture/api/process-protocol.md -- plugin shutdown ordering
// Detail: manager.go -- ProcessManager.Stop, the engine wait this test measures
// Related: manager_stop_wait_test.go -- registerRunEngine, drainUntilClosed, recordHandler

package process

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// VALIDATES: spec-fixit-shutdown-waits-out-a-deadlock AC-1 -- "A daemon with the bgp
// plugin loaded is stopped cleanly ... It stops well inside pluginStopGrace, and the
// measured 3.0s + 0.5s pair is gone" -- and AC-2, no "did not finish its shutdown
// cleanup in time" warning on that stop.
//
// The engine below is the shutdown tail of the bgp plugin, modeled at this layer. The
// real one is runBGPEngine (internal/component/bgp/plugin/register.go): after its read
// loop ends it calls Reactor.Stop and then Reactor.Wait. Reactor.cleanup Phase 1
// (internal/component/bgp/reactor/reactor.go) stops r.api, the plugin server that hosts
// this engine, and Server.Stop calls s.cleanup, which calls this manager's Stop
// (internal/component/plugin/server/server.go). So the engine's own shutdown re-enters
// the Stop that is already waiting for that engine to return, and only pluginStopGrace
// plus the 500ms group wait break it.
//
// PREVENTS: the cycle being declared fixed while a clean stop still pays for it. MEASURED
// 2026-08-19 over four traced runs of test/plugin/system-cpu-show.ci: 3001ms then 501ms,
// matching both constants, on a daemon no peer ever connected to.
func TestCleanShutdownDoesNotWaitOutTheEngineGrace(t *testing.T) {
	var pm *ProcessManager
	registerRunEngine(t, "test-stop-clean-shutdown", func(conn net.Conn) int {
		drainUntilClosed(conn)
		pm.Stop()
		return 0
	})

	var mu sync.Mutex
	var lines []string
	origLogger := logger
	logger = func() *slog.Logger {
		return slog.New(recordHandler{mu: &mu, lines: &lines})
	}
	t.Cleanup(func() { logger = origLogger })

	pm = NewProcessManager([]plugin.PluginConfig{{Name: "test-stop-clean-shutdown", Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())

	start := time.Now()
	pm.Stop()
	elapsed := time.Since(start)

	mu.Lock()
	got := strings.Join(lines, "\n")
	mu.Unlock()

	assert.Less(t, elapsed, pluginStopGrace,
		"a clean stop spent the engine grace (%s of %s): the engine cannot return, because returning needs a cleanup that waits for the engine",
		elapsed, pluginStopGrace)
	assert.NotContains(t, got, "may be left behind",
		"a clean stop warned that a plugin outlasted its grace, so the daemon reports a possible leak on every stop")
}

// VALIDATES: spec-fixit-shutdown-waits-out-a-deadlock AC-4 -- "ProcessManager.Stop is
// entered twice for one manager: the engine wait happens once; the second entry does
// not re-wait."
//
// This is a GUARD, not the cure. The cure is the ownership guard in Reactor.cleanup
// (internal/component/bgp/reactor/reactor.go), which removes the only known re-entrant
// caller. The guard stays because a second entry can only ever cost time: the first has
// already canceled the context and closed every connection, which is the whole shutdown
// signal, so a second wait charges a second grace for the same engines.
//
// PREVENTS: a plugin engine that calls Stop on its way out waiting for itself. The
// second entry is taken while the first is still inside the engine wait, which is the
// only shape where re-entry costs anything: a sequential second Stop finds an empty
// process map and is fast whatever this guard does.
func TestStopIsIdempotentForTheEngineWait(t *testing.T) {
	readLoopEnded := make(chan struct{})
	release := make(chan struct{})
	registerRunEngine(t, "test-stop-idempotent", func(conn net.Conn) int {
		drainUntilClosed(conn)
		close(readLoopEnded)
		<-release
		return 0
	})

	var mu sync.Mutex
	var lines []string
	origLogger := logger
	logger = func() *slog.Logger {
		return slog.New(recordHandler{mu: &mu, lines: &lines})
	}
	origGrace := pluginStopGrace
	pluginStopGrace = 2 * time.Second
	t.Cleanup(func() {
		logger = origLogger
		pluginStopGrace = origGrace
	})

	pm := NewProcessManager([]plugin.PluginConfig{{Name: "test-stop-idempotent", Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		pm.Stop()
	}()

	// The read loop ends only once the first Stop has closed the connection, so from
	// here the first entry owns the engine wait.
	<-readLoopEnded

	start := time.Now()
	pm.Stop()
	second := time.Since(start)

	close(release)
	<-firstDone

	mu.Lock()
	got := strings.Join(lines, "\n")
	mu.Unlock()

	assert.Less(t, second, 250*time.Millisecond,
		"the second entry re-waited for the same engines (%s): one entry owns the wait", second)
	assert.NotContains(t, got, "may be left behind",
		"the engine was charged a second grace it had already been given")
}

// VALIDATES: spec-fixit-shutdown-waits-out-a-deadlock AC-3 -- an engine that genuinely
// does not return still hits pluginStopGrace, and the warning still names that plugin.
// This spec removes the CYCLE, never the guard, and the timeouts are unchanged.
//
// It asserts the BOUND, which is what makes it different from
// TestStopNamesThePluginThatMissesItsCleanupGrace in manager_stop_wait_test.go: that
// one proves the stuck plugin is named, this one proves the wait was really spent and
// really ended. A Stop that stopped waiting would pass neither, and a Stop that hung
// would pass only the first.
func TestAStuckEngineStillHitsItsGrace(t *testing.T) {
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	registerRunEngine(t, "test-stop-stuck-engine-grace", func(conn net.Conn) int {
		drainUntilClosed(conn)
		<-stuck
		return 0
	})

	var mu sync.Mutex
	var lines []string
	origLogger := logger
	logger = func() *slog.Logger {
		return slog.New(recordHandler{mu: &mu, lines: &lines})
	}
	origGrace := pluginStopGrace
	// Above the 500ms group wait that follows the engine wait, so the measurement
	// below reads the ENGINE wait and not the wait after it.
	pluginStopGrace = 900 * time.Millisecond
	t.Cleanup(func() {
		logger = origLogger
		pluginStopGrace = origGrace
	})

	pm := NewProcessManager([]plugin.PluginConfig{{Name: "test-stop-stuck-engine-grace", Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())

	start := time.Now()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		pm.Stop()
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop never returned: a stuck engine is no longer bounded by anything")
	}
	elapsed := time.Since(start)

	mu.Lock()
	got := strings.Join(lines, "\n")
	mu.Unlock()

	assert.GreaterOrEqual(t, elapsed, pluginStopGrace,
		"Stop returned in %s, before the %s grace: a stuck engine is no longer given the time its release needs",
		elapsed, pluginStopGrace)
	assert.Contains(t, got, "test-stop-stuck-engine-grace",
		"the plugin that outlasted its grace was not named, so a lost cleanup leaves no trace")
	assert.Contains(t, got, "may be left behind")
}

// nodeState models state a plugin installs OUTSIDE its own process: the IKE engine's
// eight XFRM bypass policies are the real case, and the kernel keeps those whether or
// not the daemon that installed them still exists (bypass.go, engine/runEngine).
// Nothing in the shutdown path holds a reference to this map, so only the engine's own
// release can empty it.
var nodeState = struct {
	mu      sync.Mutex
	entries map[string]bool
}{entries: make(map[string]bool)}

func installNodeState(prefix string, count int) {
	nodeState.mu.Lock()
	defer nodeState.mu.Unlock()
	for i := range count {
		nodeState.entries[fmt.Sprintf("%s-%d", prefix, i)] = true
	}
}

func releaseNodeState(prefix string) {
	nodeState.mu.Lock()
	defer nodeState.mu.Unlock()
	maps.DeleteFunc(nodeState.entries, func(k string, _ bool) bool {
		return strings.HasPrefix(k, prefix)
	})
}

func nodeStateResidue(prefix string) []string {
	nodeState.mu.Lock()
	defer nodeState.mu.Unlock()
	var residue []string
	for k := range nodeState.entries {
		if strings.HasPrefix(k, prefix) {
			residue = append(residue, k)
		}
	}
	return residue
}

// VALIDATES: spec-fixit-shutdown-waits-out-a-deadlock AC-5 -- "a daemon that installed
// resources through a plugin is stopped: everything the engine installed is released
// before exit." A faster shutdown that leaks is a worse defect than the wait this spec
// removes, so the speed and the release are asserted in ONE stop.
//
// The engine here has the shape of the bgp one: its read loop ends, its own shutdown
// re-enters ProcessManager.Stop (runBGPEngine -> Reactor.Stop -> Reactor.cleanup ->
// Server.Stop -> ProcessManager.Stop), and only THEN does it release. The release is
// deliberately slower than the 500ms group wait, so a Stop that stopped waiting for
// engines would return with the state still installed.
//
// PREVENTS: trading the 3.5s wait for a leak. MEASURED 2026-08-17 on the real thing:
// with the release delayed 700ms, all eight IKE bypass policies stayed in the kernel
// after both daemons had exited (test/ipsec/ipsec-teardown-leaves-nothing.ci reported
// RESIDUE: policies=8).
func TestEngineReleasesWhatItInstalledOnStop(t *testing.T) {
	const prefix = "test-stop-release"
	const installed = 8
	var pm *ProcessManager
	registerRunEngine(t, "test-stop-engine-release", func(conn net.Conn) int {
		installNodeState(prefix, installed)
		drainUntilClosed(conn)
		// The bgp engine's tail: its own shutdown reaches back into this manager.
		pm.Stop()
		// The release an engine runs on its way out, slower than the 500ms group
		// wait that follows the engine wait.
		time.Sleep(slowReleaseDelay)
		releaseNodeState(prefix)
		return 0
	})
	t.Cleanup(func() { releaseNodeState(prefix) })

	var mu sync.Mutex
	var lines []string
	origLogger := logger
	logger = func() *slog.Logger {
		return slog.New(recordHandler{mu: &mu, lines: &lines})
	}
	t.Cleanup(func() { logger = origLogger })

	pm = NewProcessManager([]plugin.PluginConfig{{Name: "test-stop-engine-release", Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())
	require.Eventually(t, func() bool {
		return len(nodeStateResidue(prefix)) == installed
	}, 5*time.Second, 10*time.Millisecond, "the plugin never installed its state, so this stop proves nothing")

	start := time.Now()
	pm.Stop()
	elapsed := time.Since(start)

	mu.Lock()
	got := strings.Join(lines, "\n")
	mu.Unlock()

	assert.Empty(t, nodeStateResidue(prefix),
		"Stop returned with state the plugin installed still in place: the shutdown got faster by leaking")
	assert.Less(t, elapsed, pluginStopGrace,
		"the release landed, but the stop still spent the engine grace (%s of %s)", elapsed, pluginStopGrace)
	assert.NotContains(t, got, "may be left behind",
		"the daemon warned about a leak on a stop that leaked nothing")
}

// VALIDATES: the re-entry guard is per GENERATION, not for the life of the manager.
// Stop sets it and every spawn clears it (startConfigs), so a manager that is started
// again gets a Stop that waits for its engines rather than the second-entry path.
//
// PREVENTS: a flag that only ever goes one way. A guard that stays set after a
// respawn would turn every later Stop into an immediate return, which is exactly the
// release this wait exists to land being skipped -- a leak wearing the fix's clothes
// (ai/rules/evidence.md: a guard must fail closed).
func TestARestartedManagerStillWaitsForItsEngines(t *testing.T) {
	var released atomic.Bool
	registerRunEngine(t, "test-stop-restart-waits", func(conn net.Conn) int {
		drainUntilClosed(conn)
		time.Sleep(slowReleaseDelay)
		released.Store(true)
		return 0
	})

	pm := NewProcessManager([]plugin.PluginConfig{{Name: "test-stop-restart-waits", Internal: true, Encoder: "json"}})
	require.NoError(t, pm.Start())
	pm.Stop()
	require.True(t, released.Load(), "the first stop did not wait for the release")

	released.Store(false)
	require.NoError(t, pm.Start())
	pm.Stop()

	assert.True(t, released.Load(),
		"the second stop returned before the engine released: the guard was never cleared by the respawn")
}

// VALIDATES: the re-entry guard is cleared by EVERY spawn site, not only by the one
// that starts a manager. Respawn (manager.go) is the second: it is reached from
// Server.restartPlugin (internal/component/plugin/server/reload_tx.go) when a reload
// rollback reports a broken plugin, and it puts a live process in pm.processes
// without going through startConfigs.
//
// PREVENTS: the guard becoming the leak it was added to avoid. Stop cancels pm.ctx
// before it waits, and Process.startInternal (internal/component/plugin/process/process.go)
// never reads that context: it makes the pipe, sets running, and starts the engine
// goroutine whatever the context says. So a respawn landing after a Stop's engine wait
// leaves a LIVE engine behind a flag that is still set, and the next Stop takes the
// second-entry path and returns before that engine releases what it installed.
func TestARespawnedPluginStillGetsItsEngineWait(t *testing.T) {
	var released atomic.Bool
	registerRunEngine(t, "test-stop-respawn-waits", func(conn net.Conn) int {
		drainUntilClosed(conn)
		time.Sleep(slowReleaseDelay)
		released.Store(true)
		return 0
	})

	// RespawnEnabled is what Server.restartPlugin's plugins carry: without it Respawn
	// returns nil having done nothing (manager.go, the "Respawn not enabled" branch).
	pm := NewProcessManager([]plugin.PluginConfig{{
		Name: "test-stop-respawn-waits", Internal: true, Encoder: "json", RespawnEnabled: true,
	}})
	require.NoError(t, pm.Start())
	pm.Stop()
	require.True(t, released.Load(), "the first stop did not wait for the release")

	// The reload-rollback path, run against a manager Stop has already flagged.
	released.Store(false)
	_, respawnErr := pm.Respawn("test-stop-respawn-waits")
	require.NoError(t, respawnErr)
	require.Eventually(t, func() bool {
		proc := pm.GetProcess("test-stop-respawn-waits")
		return proc != nil && proc.Running()
	}, 5*time.Second, 10*time.Millisecond, "the respawn never produced a running process, so this stop proves nothing")

	pm.Stop()

	assert.True(t, released.Load(),
		"the stop after a respawn returned before the respawned engine released: Respawn left the re-entry guard set")
}

// VALIDATES: Respawn JOINS the process it replaces, so no goroutine outlives the map
// entry that owned it.
//
// PREVENTS: one leaked event delivery loop per crash-and-respawn cycle. Running()
// reports the ENGINE, and StartWithContext (process.go) starts the delivery loop before
// it, so a plugin whose engine has returned still owns a goroutine ranging over its
// event channel (delivery.go, deliveryLoop) that only Stop can end. Respawn used to
// skip Stop for exactly that process, then overwrite pm.processes[name], the only
// handle on it. ProcessManager.Stop walks that map, so the loop blocked for the life of
// the daemon and nothing could reach it again. The same leak in a test lets the dropped
// goroutine read a package var while a later test swaps it, which is how this was
// found: the plugin logger, raced by a stopped plugin's engine.
func TestRespawnJoinsTheProcessItReplaces(t *testing.T) {
	registerRunEngine(t, "test-respawn-join", func(_ net.Conn) int {
		// Returns at once, which is what the manager sees when a plugin crashes.
		return 0
	})

	pm := NewProcessManager([]plugin.PluginConfig{{
		Name: "test-respawn-join", Internal: true, Encoder: "json", RespawnEnabled: true,
	}})
	require.NoError(t, pm.Start())
	t.Cleanup(pm.Stop)

	first := pm.GetProcess("test-respawn-join")
	require.NotNil(t, first)
	require.Eventually(t, func() bool { return !first.Running() }, 5*time.Second, time.Millisecond,
		"the engine must have returned before the respawn, because that is what a crash looks like")

	_, err := pm.Respawn("test-respawn-join")
	require.NoError(t, err)

	// The respawn has already dropped this process from pm.processes, so this is the
	// last handle on it that exists. Every goroutine it owns must be done by now.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, first.Wait(ctx),
		"Respawn replaced a process that still had a goroutine running, and no later Stop can reach it")
}
