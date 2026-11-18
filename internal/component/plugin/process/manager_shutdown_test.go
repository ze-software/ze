// Design: docs/architecture/api/process-protocol.md -- whole-daemon shutdown
// Related: manager_shutdown.go -- terminal admission and unpublished owner joins
package process

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
)

func shutdownTestTiers(configs []plugin.PluginConfig) [][]string {
	names := make([]string, 0, len(configs))
	for _, cfg := range configs {
		names = append(names, cfg.Name)
	}
	return [][]string{names}
}

// Every real process-start API refuses after terminal stop. A refused injected
// process stays caller-owned and live until that caller explicitly stops it;
// neither an expired stop context nor a hidden asynchronous cleanup owns it.
func TestOrderedStopRefusesLaterProcessStarts(t *testing.T) {
	const name = "test-ordered-stop-terminal"
	registerRunEngine(t, name, func(conn net.Conn) int { drainUntilClosed(conn); return 0 })
	cfg := plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"}
	pm := NewProcessManager([]plugin.PluginConfig{cfg})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	pm.StopInOrder(shutdownTestTiers)
	if err := pm.StartMore([]plugin.PluginConfig{cfg}); !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("StartMore = %v", err)
	}
	if err := pm.StartWithContext(t.Context()); !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("StartWithContext = %v", err)
	}
	if err := pm.StartEach(t.Context(), []plugin.PluginConfig{cfg}, nil)[name]; !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("StartEach = %v", err)
	}
	if _, err := pm.Respawn(name); !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("Respawn = %v", err)
	}
	injected := NewProcess(cfg)
	if err := injected.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		injected.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := injected.Wait(ctx); err != nil {
			t.Error(err)
		}
	})
	if err := pm.AddProcess(name, injected); !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("AddProcess = %v", err)
	}
	select {
	case <-injected.EngineDone():
		t.Fatal("refused AddProcess took caller-owned cleanup")
	default:
	}
	if got := pm.AllProcesses(); len(got) != 0 {
		t.Fatalf("ordered stop admitted %d later processes", len(got))
	}
	if late := pm.CleanupLate(); len(late) != 0 {
		t.Fatalf("refusal invented unfinished manager cleanup: %v", late)
	}
	injected.Stop()
	if err := injected.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-injected.EngineDone():
	default:
		t.Fatal("caller's Wait did not join the refused engine")
	}
}

// Ordinary Stop remains reusable, and a second stop must join the replacement
// engine rather than inherit the previous stop's reentry guard.
func TestStandaloneStopStillJoinsAReusedManager(t *testing.T) {
	const name = "test-standalone-stop-reuse"
	registerRunEngine(t, name, func(conn net.Conn) int { drainUntilClosed(conn); return 0 })
	cfg := plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"}
	pm := NewProcessManager([]plugin.PluginConfig{cfg})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	pm.Stop()
	if err := pm.StartMore([]plugin.PluginConfig{cfg}); err != nil {
		t.Fatal(err)
	}
	proc := pm.GetProcess(name)
	if proc == nil {
		t.Fatal("reused manager published no process")
	}
	pm.Stop()
	select {
	case <-proc.EngineDone():
	default:
		t.Fatal("second Stop did not join the replacement engine")
	}
}

// Reentrant legacy Stop and Respawn cannot steal a terminal stop's ownership
// while its resolver still holds a live provider. The resolver gate controls
// the interval and checks positive finalization after it releases the owner.
func TestLegacyStopCannotCancelAnOrderedOwner(t *testing.T) {
	const name = "test-ordered-owner-provider"
	registerRunEngine(t, name, func(conn net.Conn) int { drainUntilClosed(conn); return 0 })
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	provider := pm.GetProcess(name)
	entered, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		select {
		case <-stopped:
		case <-time.After(pluginStopGrace + time.Second):
			t.Error("ordered owner did not return after releasing gate")
		}
	})
	go func() {
		pm.StopInOrder(func(configs []plugin.PluginConfig) [][]string {
			close(entered)
			<-release
			return shutdownTestTiers(configs)
		})
		close(stopped)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("ordered owner never reached resolver")
	}
	pm.Stop()
	if _, err := pm.Respawn(name); !errors.Is(err, ErrDaemonStopping) {
		t.Errorf("Respawn during ordered cleanup = %v", err)
	}
	select {
	case <-provider.Done():
		t.Error("legacy entry canceled an ordered owner's provider")
	default:
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("ordered stop did not finish")
	}
	select {
	case <-provider.EngineDone():
	default:
		t.Fatal("ordered stop did not join its provider engine")
	}
}

// Concurrent StartWithContext and terminal Stop share one admission decision:
// a published engine is joined, or the start is refused, with no late slot.
func TestConcurrentProcessStartAndOrderedStopRemainTerminal(t *testing.T) {
	const name = "test-ordered-start-race"
	registerRunEngine(t, name, func(conn net.Conn) int { drainUntilClosed(conn); return 0 })
	pm := NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: "json"}})
	started := make(chan error, 1)
	go func() { started <- pm.StartWithContext(t.Context()) }()
	pm.StopInOrder(shutdownTestTiers)
	select {
	case err := <-started:
		if err != nil && !errors.Is(err, ErrDaemonStopping) {
			t.Fatal(err)
		}
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("start owner did not settle")
	}
	if got := pm.AllProcesses(); len(got) != 0 {
		t.Fatalf("terminal manager holds %d processes", len(got))
	}
	if err := pm.Start(); !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("later Start = %v", err)
	}
}

// The real external child has forked but is blocked in TLS connect-back.
// Ordered stop must wake and join that unpublished owner before resolving tiers
// or touching a published provider. The pid file proves the child really ran.
func TestOrderedStopJoinsUnpublishedExternalStartBeforeProviders(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	acceptor := ipc.NewPluginAcceptor(listener, "a-plugin-secret-of-at-least-32-chars", nil)
	t.Cleanup(acceptor.Stop)
	const provider = "test-ordered-unpublished-provider"
	registerRunEngine(t, provider, func(conn net.Conn) int { drainUntilClosed(conn); return 0 })
	pm := NewProcessManager([]plugin.PluginConfig{{Name: provider, Internal: true, Encoder: "json"}})
	pm.SetAcceptor(acceptor)
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	providerProc := pm.GetProcess(provider)
	if providerProc == nil {
		t.Fatal("provider did not start")
	}
	pidPath := filepath.Join(t.TempDir(), "starting.pid")
	const starting = "test-ordered-unpublished-external"
	started := make(chan error, 1)
	go func() {
		started <- pm.StartMore([]plugin.PluginConfig{{Name: starting, Encoder: "json", Run: "echo $$ > " + pidPath + "; exec sleep 30"}})
	}()
	_ = waitForPluginPid(t, pidPath)
	pm.mu.RLock()
	var unpublished *Process
	for proc := range pm.starting {
		if proc.Name() == starting {
			unpublished = proc
		}
	}
	pm.mu.RUnlock()
	if unpublished == nil {
		t.Fatal("forked child has no admitted owner")
	}
	resolved := false
	pm.StopInOrder(func(configs []plugin.PluginConfig) [][]string {
		resolved = true
		if len(configs) != 1 {
			t.Errorf("sealed snapshot contains %d configs, want provider only", len(configs))
		}
		select {
		case <-providerProc.Done():
			t.Error("provider stopped before unpublished owner joined")
		default:
		}
		pm.mu.RLock()
		owners := len(pm.starting)
		pm.mu.RUnlock()
		if owners != 0 {
			t.Errorf("tier resolution ran with %d unpublished owners", owners)
		}
		if !shutdownExitRecorded(unpublished) {
			t.Error("tier resolution ran before the unpublished child was reaped")
		}
		return shutdownTestTiers(configs)
	})
	if !resolved {
		t.Fatal("ordered stop never resolved snapshot")
	}
	select {
	case err := <-started:
		if err == nil {
			t.Error("unpublished connect-back succeeded after stop")
		}
	case <-time.After(pluginStartBudget):
		t.Fatal("external owner did not return")
	}
	if proc := pm.GetProcess(starting); proc != nil {
		t.Fatal("rejected external start became a manager slot")
	}
	if !shutdownExitRecorded(unpublished) {
		t.Fatal("ordered stop left unpublished child unreaped")
	}
	if err := providerProc.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// Close is the exact synchronous side effect of PluginAcceptor.Stop after it
// has canceled connect-back waiters. Only the first Close invokes the gate:
// the real failed StartMore path may itself call reusable Stop and close again.
type shutdownCloseListener struct {
	net.Listener
	closed  atomic.Bool
	onClose func()
}

func (l *shutdownCloseListener) Close() error {
	if l.closed.CompareAndSwap(false, true) {
		l.onClose()
	}
	return l.Listener.Close()
}

// The actual failed-start caller, not a synthetic concurrent Stop, arbitrates
// against ordered shutdown while acceptor.Stop is still on its stack.
// DISCRIMINATES: move the acceptor stop above publication of the terminal fence;
// this Close hook sees no owner and failed StartMore can stop the provider.
func TestOrderedStopOwnsFailureBeforeAcceptorWake(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	const provider = "test-failure-arbitration-provider"
	registerRunEngine(t, provider, func(conn net.Conn) int { drainUntilClosed(conn); return 0 })
	pm := NewProcessManager([]plugin.PluginConfig{{Name: provider, Internal: true, Encoder: "json"}})
	started := make(chan error, 1)
	wrapped := &shutdownCloseListener{Listener: listener}
	wrapped.onClose = func() {
		if pm.shutdown.Load() == nil {
			t.Error("acceptor woke failed starts before publishing terminal ownership")
		}
		select {
		case err := <-started:
			if err == nil {
				t.Error("nonconnecting child start succeeded")
			}
		case <-time.After(time.Second):
			t.Error("failed StartMore did not return through the owner fence")
		}
		if proc := pm.GetProcess(provider); proc != nil {
			select {
			case <-proc.Done():
				t.Error("failed StartMore's ordinary Stop canceled the ordered owner's provider")
			default:
			}
		}
	}
	acceptor := ipc.NewPluginAcceptor(wrapped, "a-plugin-secret-of-at-least-32-chars", nil)
	pm.SetAcceptor(acceptor)
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pm.Stop)
	pidPath := filepath.Join(t.TempDir(), "arbitration.pid")
	go func() {
		started <- pm.StartMore([]plugin.PluginConfig{{Name: "arbitration-failure", Run: "echo $$ > " + pidPath + "; exec sleep 30", Encoder: "json"}})
	}()
	_ = waitForPluginPid(t, pidPath)
	pm.StopInOrder(shutdownTestTiers)
	if late := pm.CleanupLate(); len(late) != 0 {
		t.Errorf("arbitration left late cleanup: %v", late)
	}
}

// Reusable Stop also owns an admission fence while its old engines are being
// joined. Reuse begins after that owner returns, never by resetting its guard
// underneath cleanup and publishing a slot the owner is about to discard.
func TestStandaloneStopFencesAdmissionUntilOwnersJoin(t *testing.T) {
	const name = "test-standalone-active-stop"
	entered, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	registerRunEngine(t, name, func(conn net.Conn) int { drainUntilClosed(conn); close(entered); <-release; return 0 })
	cfg := plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"}
	pm := NewProcessManager([]plugin.PluginConfig{cfg})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		select {
		case <-stopped:
		case <-time.After(pluginStopGrace + time.Second):
			t.Error("reusable owner did not return")
		}
	})
	go func() { pm.Stop(); close(stopped) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("reusable Stop did not enter engine cleanup")
	}
	if err := pm.StartMore([]plugin.PluginConfig{cfg}); !errors.Is(err, ErrDaemonStopping) {
		t.Errorf("StartMore during reusable Stop = %v", err)
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("reusable Stop did not join released cleanup")
	}
	if pm.shutdown.Load() != nil {
		t.Fatal("ordinary Stop published a terminal fence")
	}
}

// The process monitor records exit only after cmd.Wait has reaped the child.
// This is a synchronized observation of an already-completed boundary: it
// never polls a pid, calls Wait, or waits away a missing production join.
func shutdownExitRecorded(proc *Process) bool {
	proc.exit.mu.Lock()
	defer proc.exit.mu.Unlock()
	return proc.exit.status != ""
}

// Even a caller with no graph resolver requests a terminal simultaneous stop;
// nil must not select the reusable lifecycle or reopen admission afterwards.
func TestOrderedStopWithoutResolverStillSealsAdmission(t *testing.T) {
	pm := NewProcessManager(nil)
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	pm.StopInOrder(nil)
	if err := pm.Start(); !errors.Is(err, ErrDaemonStopping) {
		t.Fatalf("Start after terminal stop without resolver = %v", err)
	}
}

// Only the two WaitEngine calls receive this context. The second Done lookup
// proves both same-tier joins have entered, so the healthy finalizer can report
// whether it had the shared budget left. No scheduling sleep selects the order.
type shutdownJoinContext struct {
	context.Context
	entered chan struct{}
	waits   atomic.Int64
}

func (c *shutdownJoinContext) Done() <-chan struct{} {
	if c.waits.Add(1) == 2 {
		close(c.entered)
	}
	return c.Context.Done()
}

// A stuck first consumer must not prevent a healthy same-tier engine from
// reaching its join while the one shared deadline is still live.
// MUTATION: join each tier serially; the healthy finalizer then sees expiry.
func TestOrderedStopJoinsSameTierBeforeSharedDeadline(t *testing.T) {
	const stuck = "test-same-tier-stuck"
	const healthy = "test-same-tier-healthy"
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	joins := &shutdownJoinContext{Context: ctx, entered: make(chan struct{})}
	release := make(chan struct{})
	live := make(chan bool, 1)
	registerRunEngine(t, stuck, func(conn net.Conn) int { drainUntilClosed(conn); <-release; return 0 })
	registerRunEngine(t, healthy, func(conn net.Conn) int { drainUntilClosed(conn); <-joins.entered; live <- ctx.Err() == nil; return 0 })
	pm := NewProcessManager([]plugin.PluginConfig{{Name: stuck, Internal: true}, {Name: healthy, Internal: true}})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	healthyProc := pm.GetProcess(healthy)
	if healthyProc == nil {
		t.Fatal("healthy engine was not published")
	}
	// Complete the shared budget only after the healthy engine's real final
	// boundary. The timeout is the guard that breaks a serial join's deadlock.
	joinedHealthy := make(chan struct{})
	go func() { <-healthyProc.EngineDone(); cancel(); close(joinedHealthy) }()
	t.Cleanup(func() {
		close(release)
		pm.Stop()
		select {
		case <-joinedHealthy:
		case <-time.After(time.Second):
			t.Error("healthy completion waiter did not return")
		}
	})
	late := pm.stopEngines(joins, pm.AllProcesses(), func([]plugin.PluginConfig) [][]string { return [][]string{{stuck, healthy}} }, true)
	select {
	case beforeExpiry := <-live:
		if !beforeExpiry {
			t.Error("healthy engine reached its same-tier join only after shared expiry")
		}
	case <-time.After(time.Second):
		t.Fatal("healthy finalizer never observed its join")
	}
	if len(late) != 1 {
		t.Fatalf("late engines = %v, want only %s", late, stuck)
	}
	if late[0] != stuck {
		t.Fatalf("late engine = %s, want %s", late[0], stuck)
	}
}

// Err runs only after WaitEngine selected the already-canceled context. The
// engine completes before that error reaches its caller, making the completion
// versus cancellation boundary deterministic rather than a random select test.
type shutdownCompletedContext struct {
	context.Context
	complete func()
}

func (c shutdownCompletedContext) Err() error {
	c.complete()
	return c.Context.Err()
}

// A completed engine must not be reported late when its real EngineDone signal
// closes before the canceled wait's error is observed.
// MUTATION: remove waitShutdownProcess's EngineDone observation after error.
func TestOrderedStopRecognizesCompletedEngineAtExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	proc := NewProcess(plugin.PluginConfig{Name: "test-completed-at-deadline", Internal: true})
	proc.engineDone = make(chan struct{})
	completed := shutdownCompletedContext{Context: ctx, complete: func() { close(proc.engineDone) }}
	if err := waitShutdownProcess(completed, proc, true); err != nil {
		t.Fatalf("completed engine reported late: %v", err)
	}
}
