// Design: docs/architecture/api/process-protocol.md -- whole-daemon shutdown
// Related: manager_shutdown.go -- admitted start frames and terminal publication
package process

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

// Refused terminal respawns cannot spend restart allowance or disable the
// stopped plugin. Repeating beyond the real bound distinguishes an early
// refusal from a late refusal after the rate bookkeeping has changed.
// MUTATION: remove the three early Respawn refusal guards together; the late
// terminal guard still refuses five calls, then the sixth reports a rate limit.
func TestTerminalRespawnDoesNotConsumeRestartBudget(t *testing.T) {
	const name = "test-terminal-respawn-budget"
	registerRunEngine(t, name, func(conn net.Conn) int { drainUntilClosed(conn); return 0 })
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	first := pm.GetProcess(name)
	if first == nil {
		t.Fatal("control: real initial engine was not published")
	}
	pm.StopInOrder(shutdownTestTiers)
	select {
	case <-first.EngineDone():
	default:
		t.Fatal("ordered stop did not join the initial engine")
	}
	for attempt := range RespawnLimit + 1 {
		proc, err := pm.Respawn(name)
		if !errors.Is(err, ErrDaemonStopping) {
			t.Errorf("terminal Respawn %d = %v, want stopping refusal", attempt+1, err)
		}
		if proc != nil {
			t.Error("terminal Respawn returned a replacement")
		}
	}
	if pm.Disabled(name) {
		t.Error("terminal refusals disabled the plugin through its restart limit")
	}
	pm.mu.RLock()
	total, window := pm.totalRespawns[name], len(pm.respawnTimes[name])
	pm.mu.RUnlock()
	if total != 0 {
		t.Errorf("terminal refusals spent %d cumulative restarts", total)
	}
	if window != 0 {
		t.Errorf("terminal refusals spent %d window restarts", window)
	}
	if got := pm.AllProcesses(); len(got) != 0 {
		t.Errorf("terminal refusal published %d processes", len(got))
	}
}

// Ordinary Stop fences Respawn while its actual original engine is finalizing.
// Unlike a terminal stop, neither RefuseRespawns nor shutdown is set here, so
// this case distinguishes the independent reusable-stop guard.
// MUTATION: remove only Respawn's early stopActive refusal; a replacement is
// returned instead of ErrDaemonStopping after the outgoing real join's bound.
func TestReusableStopRefusesRespawnWhileItsEngineIsJoining(t *testing.T) {
	const name = "test-reusable-stop-respawn"
	entered, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	var generations atomic.Int64
	registerRunEngine(t, name, func(conn net.Conn) int {
		generation := generations.Add(1)
		drainUntilClosed(conn)
		if generation == 1 {
			close(entered)
			<-release
		}
		return 0
	})
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		select {
		case <-stopped:
		case <-time.After(pluginStopGrace + time.Second):
			t.Error("ordinary stop did not return after finalizer release")
		}
	})
	go func() { pm.Stop(); close(stopped) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("ordinary stop did not reach the real engine finalizer")
	}
	if pm.shutdown.Load() != nil {
		t.Fatal("control: ordinary stop unexpectedly set a terminal fence")
	}
	if pm.respawnRefused.Load() {
		t.Fatal("control: ordinary stop unexpectedly refused all later respawns")
	}
	proc, err := pm.Respawn(name)
	// Keep a wrongly returned generation physically owned even if ordinary
	// Stop's original snapshot cannot contain that replacement.
	if proc != nil {
		t.Cleanup(func() {
			proc.Stop()
			ctx, cancel := context.WithTimeout(context.Background(), pluginStopGrace)
			defer cancel()
			if err := proc.Wait(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	if !errors.Is(err, ErrDaemonStopping) {
		t.Errorf("Respawn during ordinary cleanup = %v, want stopping refusal", err)
	}
	if proc != nil {
		t.Error("Respawn during ordinary cleanup returned a replacement")
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-stopped:
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("ordinary stop did not join the released engine")
	}
}

// The listener gate is inside the real external Start before it can admit a
// child or its monitor. Close observes the actual terminal acceptor stop.
type admissionFrameListener struct {
	net.Listener
	entered   chan struct{}
	release   chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func (l *admissionFrameListener) Addr() net.Addr {
	close(l.entered)
	<-l.release
	return l.Listener.Addr()
}

func (l *admissionFrameListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return l.Listener.Close()
}

// A canceled real external Start still owns its admitted call frame after its
// current worker group joins. Process.Wait alone cannot substitute for that
// frame: Start can still return, clean up, and change manager ownership.
// MUTATION: skip joinStarting's wait on pm.spawns while retaining Add/Done;
// the actual provider finalizer then runs while listener.Addr still holds Start.
func TestOrderedStopJoinsStartFrameAfterItsCurrentWorkersFinish(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	wrapped := &admissionFrameListener{
		Listener: listener, entered: make(chan struct{}),
		release: make(chan struct{}), closed: make(chan struct{}),
	}
	acceptor := ipc.NewPluginAcceptor(wrapped, "a-plugin-secret-of-at-least-32-chars", nil)
	t.Cleanup(acceptor.Stop)
	const provider = "test-start-frame-provider"
	providerExited := make(chan struct{})
	registerRunEngine(t, provider, func(conn net.Conn) int {
		drainUntilClosed(conn)
		close(providerExited)
		return 0
	})
	pm := NewProcessManager([]plugin.PluginConfig{{Name: provider, Internal: true, Encoder: "json"}})
	pm.SetAcceptor(acceptor)
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	started := make(chan error, 1)
	stopped, ownerStopped := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	terminalStarted, ownerStopStarted := false, false
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(wrapped.release) })
		if terminalStarted {
			select {
			case <-stopped:
			case <-time.After(pluginStopGrace + time.Second):
				t.Error("ordered stop did not finish after releasing Start")
			}
		}
		select {
		case err := <-started:
			if err != nil {
				t.Logf("StartMore exited during cleanup: %v", err)
			}
		case <-time.After(pluginStopGrace + time.Second):
			t.Error("Start caller did not finish after gate release")
		}
		if ownerStopStarted {
			select {
			case <-ownerStopped:
			case <-time.After(time.Second):
				t.Error("Process.Stop caller did not finish after gate release")
			}
		}
	})
	const starting = "test-start-frame-external"
	go func() {
		started <- pm.StartMore([]plugin.PluginConfig{{Name: starting, Encoder: "json", Run: "exec sleep 30"}})
		close(started)
	}()
	select {
	case <-wrapped.entered:
	case <-time.After(time.Second):
		t.Fatal("external Start did not enter listener.Addr")
	}
	pm.mu.RLock()
	var unpublished *Process
	for proc := range pm.starting {
		if proc.Name() == starting {
			unpublished = proc
		}
	}
	pm.mu.RUnlock()
	if unpublished == nil {
		t.Fatal("real Start had no admitted owner")
	}
	// Stop cancels and closes delivery before its connection cleanup waits on
	// Start's mutex. The real Wait now joins the existing delivery worker;
	// no synthetic WaitGroup count or Done substitutes for physical completion.
	ownerStopStarted = true
	go func() { unpublished.Stop(); close(ownerStopped) }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err = unpublished.Wait(ctx)
	cancel()
	if err != nil {
		t.Fatalf("current Start workers did not finish: %v", err)
	}
	terminalStarted = true
	go func() { pm.StopInOrder(shutdownTestTiers); close(stopped) }()
	select {
	case <-wrapped.closed:
	case <-time.After(time.Second):
		t.Fatal("terminal stop did not close its acceptor")
	}
	select {
	case <-providerExited:
		t.Error("provider finalized while the real admitted Start frame was held")
	case <-stopped:
		t.Error("ordered stop returned while the real admitted Start frame was held")
	case <-time.After(100 * time.Millisecond):
		// This bounds only the negative observation before releasing the gate.
		// A mutant RED must be one of the physical outcomes above, not timeout.
	}
	releaseOnce.Do(func() { close(wrapped.release) })
	select {
	case err := <-started:
		if err == nil {
			t.Error("canceled held Start unexpectedly succeeded")
		}
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("released real Start did not return")
	}
	select {
	case <-ownerStopped:
	case <-time.After(time.Second):
		t.Fatal("released real Process.Stop did not return")
	}
	select {
	case <-stopped:
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("ordered stop did not join the released Start frame")
	}
	if pm.GetProcess(starting) != nil {
		t.Error("canceled held Start published a process")
	}
}
