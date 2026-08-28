// Design: docs/architecture/api/process-protocol.md — multi-process coordination and respawn
// Overview: process.go — Process struct and lifecycle
// Related: delivery.go — event delivery pipeline

package process

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
)

var errProcessmanagerStartmoreCalledBeforeStartwithcontext = errors.New("ProcessManager: StartMore called before StartWithContext")

const (
	// RespawnLimit is max respawns per RespawnWindow before disabling.
	// ExaBGP: respawn_number=5 per ~63 seconds.
	RespawnLimit = 5

	// RespawnWindow is the time window for respawn limit tracking.
	RespawnWindow = 60 * time.Second

	// MaxTotalRespawns is the cumulative respawn limit before permanent disable.
	// Prevents a permanently broken plugin from cycling indefinitely across windows.
	MaxTotalRespawns = 20
)

// pluginStopGrace is how long Stop gives every plugin goroutine to finish once the
// connections are closed. It is the daemon's own shutdown budget (cmd/ze/hub/main.go
// gives eng.Stop 3s and warns when that is missed), so the two bounds are one number:
// a wait shorter than the budget it sits inside discards cleanup the daemon was still
// willing to wait for. See Stop for what a plugin does after its read loop ends, and
// what was measured when this wait expired first.
//
// It is a var rather than a const so a test can drive the EXPIRED branch without
// spending the real grace on it. Same test seam as hasCaps in
// internal/test/runner/caps.go: never written outside a test.
var pluginStopGrace = 3 * time.Second

// Respawn errors.
var (
	ErrRespawnLimitExceeded = errors.New("respawn limit exceeded")
	ErrProcessDisabled      = errors.New("process disabled due to respawn limit")
	ErrProcessNotFound      = errors.New("process not found")
	// ErrRespawnNotEnabled reports that the plugin config enables no respawn, so
	// nothing was restarted. It is an error rather than a silent nil because the
	// caller asks for a restart of a plugin it already knows is broken. To answer
	// "done" while the broken process keeps running is the fail-open direction a
	// guard must never take (ai/rules/evidence.md).
	ErrRespawnNotEnabled = errors.New("respawn not enabled for plugin")
)

// pluginMetrics holds Prometheus metrics for plugin health.
// Created by SetMetricsRegistry; nil when metrics are disabled.
type pluginMetrics struct {
	status    metrics.GaugeVec   // ze_plugin_status: current stage per plugin
	restarts  metrics.CounterVec // ze_plugin_restarts_total: cumulative restart count
	delivered metrics.CounterVec // ze_plugin_events_delivered_total: events enqueued
}

// ProcessManager manages multiple external processes.
type ProcessManager struct {
	configs   []plugin.PluginConfig
	processes map[string]*Process

	// Respawn tracking: name -> list of respawn timestamps
	respawnTimes map[string][]time.Time

	// Cumulative respawn counts (never reset)
	totalRespawns map[string]int

	// Disabled processes (respawn limit exceeded)
	disabled map[string]bool

	// TLS acceptor for external plugin connect-back (nil = use socketpairs).
	acceptor *ipc.PluginAcceptor

	// Plugin health metrics (nil when metrics disabled).
	pmetrics *pluginMetrics

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex

	// stopping is true from the moment Stop signals until the next spawn clears it
	// (startConfigs). A second entry to Stop returns at once instead of waiting for
	// the engines a first entry is already waiting for. See Stop for why.
	stopping atomic.Bool
}

// SetAcceptor sets the TLS acceptor for external plugin connect-back.
// Must be called before StartWithContext.
func (pm *ProcessManager) SetAcceptor(a *ipc.PluginAcceptor) {
	pm.acceptor = a
}

// SetMetricsRegistry creates plugin health metrics from the given registry.
// Must be called before StartWithContext. Nil registry disables metrics.
// Idempotent: second call is a no-op.
func (pm *ProcessManager) SetMetricsRegistry(reg metrics.Registry) {
	if reg == nil || pm.pmetrics != nil {
		return
	}
	pm.pmetrics = &pluginMetrics{
		status:    reg.GaugeVec("ze_plugin_status", "Current plugin stage (0=init, 6=running)", []string{reportSourcePlugin}),
		restarts:  reg.CounterVec("ze_plugin_restarts_total", "Total plugin restart attempts (process started)", []string{reportSourcePlugin}),
		delivered: reg.CounterVec("ze_plugin_events_delivered_total", "Total events delivered to plugin", []string{reportSourcePlugin}),
	}
}

// wireMetrics sets metrics callbacks on a process.
// No-op when metrics are disabled (pm.pmetrics is nil) or plugin name is empty.
// Must be called before the process is started.
func (pm *ProcessManager) wireMetrics(proc *Process) {
	if pm.pmetrics == nil {
		return
	}
	name := proc.Name()
	if name == "" {
		return
	}
	m := pm.pmetrics
	proc.onStageChange = func(stage plugin.PluginStage) {
		m.status.With(name).Set(float64(stage))
	}
	proc.deliveryInc = func() {
		m.delivered.With(name).Inc()
	}
}

// NewProcessManager creates a new process manager.
func NewProcessManager(configs []plugin.PluginConfig) *ProcessManager {
	return &ProcessManager{
		configs:       configs,
		processes:     make(map[string]*Process),
		respawnTimes:  make(map[string][]time.Time),
		totalRespawns: make(map[string]int),
		disabled:      make(map[string]bool),
	}
}

// Start starts all configured processes.
func (pm *ProcessManager) Start() error {
	return pm.StartWithContext(context.Background())
}

// StartWithContext starts all configured processes with the given context.
// On the first call it captures the context for use by later StartMore calls.
// Subsequent calls re-spawn pm.configs (legacy single-shot behavior); use
// StartMore to add additional configs while keeping the originals running.
func (pm *ProcessManager) StartWithContext(ctx context.Context) error {
	pm.ctx, pm.cancel = context.WithCancel(ctx)
	return pm.startConfigs(pm.configs)
}

// StartMore starts the additional configs under the existing context.
// Must be called after StartWithContext (the context is captured then).
// Returns an error if the manager has not been started yet.
func (pm *ProcessManager) StartMore(configs []plugin.PluginConfig) error {
	if pm.ctx == nil {
		return errProcessmanagerStartmoreCalledBeforeStartwithcontext
	}
	if len(configs) == 0 {
		return nil
	}
	pm.configs = append(pm.configs, configs...)
	return pm.startConfigs(configs)
}

// startConfigs spawns the given configs under pm.ctx and registers them in
// pm.processes. Used by both StartWithContext (initial spawn) and StartMore
// (incremental spawn). The shared map lets AllProcesses return every process
// regardless of which spawn phase created it.
func (pm *ProcessManager) startConfigs(configs []plugin.PluginConfig) error {
	// A manager holding live processes MUST have a Stop that waits for them, so
	// every spawn clears the re-entry guard Stop sets. Leaving it set after a spawn
	// gives the next Stop the second-entry path and skips the engine wait, which is
	// the release that wait exists to land.
	//
	// This is one of THREE spawn sites, not the only one: Respawn and AddProcess also
	// put a process in pm.processes and each clears the flag itself. Stop cancels
	// pm.ctx, but Process.startInternal (process.go) never reads that context, so a
	// respawn racing a shutdown starts a live engine under a canceled context.
	pm.stopping.Store(false)
	for _, cfg := range configs {
		proc := NewProcess(cfg)
		pm.wireMetrics(proc)
		// Pass TLS acceptor to external plugins for connect-back.
		if pm.acceptor != nil && !cfg.Internal {
			proc.SetAcceptor(pm.acceptor)
		}
		if err := proc.StartWithContext(pm.ctx); err != nil {
			// Stop already started processes
			pm.Stop()
			return err
		}

		pm.mu.Lock()
		pm.processes[cfg.Name] = proc
		pm.mu.Unlock()
	}

	return nil
}

// Stop stops all processes.
// Cancels context and closes connections immediately, which unblocks plugin
// reads on net.Pipe and causes prompt exit. No bye round-trip — closing the
// connection is the shutdown signal for internal plugins, and context
// cancellation kills external plugins via exec.CommandContext.
func (pm *ProcessManager) Stop() {
	// One entry owns the engine wait below. A second entry returns at once: the
	// first has already canceled the context and closed every connection, which is
	// the whole shutdown signal, and waiting again for the same engines would only
	// pile one grace on top of another.
	//
	// This is a guard, not the cure. The known re-entrant caller was a plugin engine
	// whose own shutdown reached back here (Reactor.cleanup stopping the server that
	// hosts it), and Reactor.cleanup now stops only a server it owns. The guard stays
	// because a second entry can only ever cost time: a plugin engine calling Stop on
	// its way out would otherwise wait for itself.
	if pm.stopping.Swap(true) {
		return
	}

	// Cancel context and close all connections immediately.
	// For internal plugins: closing engine-side net.Pipe unblocks the plugin's
	// ReadRequest, causing it to return an error and exit the event loop.
	// For external plugins: context cancellation kills the subprocess.
	if pm.cancel != nil {
		pm.cancel()
	}

	// Stop TLS acceptor if running (closes listener, cancels accept loop).
	if pm.acceptor != nil {
		pm.acceptor.Stop()
	}

	pm.mu.Lock()
	for _, proc := range pm.processes {
		proc.Stop()
	}
	pm.mu.Unlock()

	// Wait for each plugin ENGINE to return, and SAY SO when one does not.
	//
	// Closing the connection unblocks a plugin's read loop at once, but the read loop
	// is not the whole engine: an engine releases what it installed AFTER its loop
	// ends, and that release can be netlink round-trips. The IKE engine's is eight
	// XFRM policy deletes plus a backend close (engine/runEngine, bypass.go), and
	// those policies are node-wide, so losing the release leaves them installed for a
	// daemon that no longer exists.
	//
	// This wait is what decides whether that release lands. It waited on the whole
	// goroutine group for 500ms and discarded its own timeout, so a slower box simply
	// lost the cleanup with nothing said: MEASURED 2026-08-17, a release delayed 700ms
	// left all eight IKE bypass policies in the kernel, and the same test failed the
	// same way in the QEMU VM at the real speed of that release
	// (test/ipsec/ipsec-teardown-leaves-nothing.ci).
	//
	// It waits on the ENGINE and not on the group, because the group also holds the
	// event delivery loop, which can still be draining a batch when the engine has
	// long finished: in the QEMU VM the bgp plugin's group needed more than 3s while
	// its engine did not, so waiting on the group would charge every daemon stop for a
	// signal that says nothing about released resources.
	//
	// The bound is the daemon's own shutdown budget, so there is one number to reason
	// about rather than two: cmd/ze/hub/main.go gives eng.Stop 3s and warns when it is
	// missed. An engine that returns promptly costs nothing here.
	pm.mu.RLock()
	engineCtx, engineCancel := context.WithTimeout(context.Background(), pluginStopGrace)
	var waitWg sync.WaitGroup
	var lateMu sync.Mutex
	var late []string
	for _, proc := range pm.processes {
		waitWg.Add(1)
		go func(p *Process) {
			defer waitWg.Done()
			if err := p.WaitEngine(engineCtx); err != nil {
				lateMu.Lock()
				late = append(late, p.config.Name)
				lateMu.Unlock()
			}
		}(proc)
	}
	pm.mu.RUnlock()
	waitWg.Wait()
	engineCancel()
	if len(late) > 0 {
		slices.Sort(late)
		logger().Warn("plugin engine did not finish its shutdown cleanup in time, resources it installed may be left behind",
			"plugins", strings.Join(late, ","), "grace", pluginStopGrace)
	}

	// Then the goroutine-leak wait, unchanged: the group holds the delivery loop and
	// the external-process monitor as well, and this is the "call Wait after Stop"
	// half of the Process contract rather than a cleanup guarantee.
	pm.mu.RLock()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	var groupWg sync.WaitGroup
	for _, proc := range pm.processes {
		groupWg.Add(1)
		go func(p *Process) {
			defer groupWg.Done()
			_ = p.Wait(waitCtx)
		}(proc)
	}
	pm.mu.RUnlock()
	groupWg.Wait()
	waitCancel()

	pm.mu.Lock()
	pm.processes = make(map[string]*Process)
	pm.mu.Unlock()
}

// Wait waits for all processes to stop.
func (pm *ProcessManager) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		pm.mu.RLock()
		for _, proc := range pm.processes {
			_ = proc.Wait(ctx)
		}
		pm.mu.RUnlock()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetProcess returns a process by name, or nil if not found.
func (pm *ProcessManager) GetProcess(name string) *Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.processes[name]
}

// RemoveProcess unregisters a stopped process by name.
// Used during config reload to clean up auto-stopped plugins.
func (pm *ProcessManager) RemoveProcess(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.processes, name)
}

// AddProcess registers a process by name. Used by tests to inject mock processes.
// Wires metrics callbacks if a metrics registry is configured.
func (pm *ProcessManager) AddProcess(name string, proc *Process) {
	pm.wireMetrics(proc)
	pm.mu.Lock()
	defer pm.mu.Unlock()
	// The third spawn site, and it clears the re-entry guard for the same reason as
	// the other two: the manager now holds a process whose engine a Stop must wait for.
	pm.stopping.Store(false)
	pm.processes[name] = proc
}

// AllProcesses returns a snapshot of all processes.
// Caller may iterate and filter the returned slice without holding the lock.
func (pm *ProcessManager) AllProcesses() []*Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Process, 0, len(pm.processes))
	for _, proc := range pm.processes {
		result = append(result, proc)
	}
	return result
}

// processCount returns the number of running processes.
func (pm *ProcessManager) processCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	count := 0
	for _, proc := range pm.processes {
		if proc.Running() {
			count++
		}
	}
	return count
}

// isRunning returns true if the named process is running.
func (pm *ProcessManager) isRunning(name string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	proc, ok := pm.processes[name]
	if !ok {
		return false
	}
	return proc.Running()
}

// isDisabled returns true if the named process is disabled due to respawn limit.
func (pm *ProcessManager) isDisabled(name string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.disabled[name]
}

const reportSourcePlugin = "plugin"
const reportCodePluginCrash = "plugin-crash"
const reportCodePluginDown = "plugin-down"

// Respawn restarts a process, enforcing respawn limits. It returns the NEW
// process on success, and a nil process with a non-nil error on every other
// path.
//
// The new process is RETURNED because a restart is only half done when this
// returns. The process is spawned, and it has run no startup handshake, so it
// carries no registration, no delivered config, no subscriptions and no
// exclusive-role claim set. The engine owns that handshake
// (Server.restartPlugin, internal/component/plugin/server/reload_tx.go), and it
// cannot run one on a process this method keeps to itself.
//
// Returns ErrRespawnNotEnabled if the plugin config enables no respawn.
// Returns ErrRespawnLimitExceeded if limit exceeded within window.
// Returns ErrProcessDisabled if process was previously disabled.
// Returns ErrProcessNotFound if process name not in configuration.
// Returns error if ProcessManager was not started (ctx is nil).
func (pm *ProcessManager) Respawn(name string) (*Process, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if ProcessManager was started
	if pm.ctx == nil {
		return nil, errors.New("process manager not started")
	}

	// Check if already disabled
	if pm.disabled[name] {
		return nil, ErrProcessDisabled
	}

	// Find config
	var cfg *plugin.PluginConfig
	for i := range pm.configs {
		if pm.configs[i].Name == name {
			cfg = &pm.configs[i]
			break
		}
	}
	if cfg == nil {
		return nil, ErrProcessNotFound
	}

	// Validated: this is a real crash of a known, enabled plugin (AC-17).
	report.RaiseError(reportSourcePlugin, reportCodePluginCrash, name,
		"plugin process exited unexpectedly: "+name, nil)

	// Check respawn enabled
	if !cfg.RespawnEnabled && !cfg.Respawn {
		return nil, ErrRespawnNotEnabled
	}

	now := time.Now()

	// Clean up old respawn times (outside window)
	var validTimes []time.Time
	for _, t := range pm.respawnTimes[name] {
		if now.Sub(t) < RespawnWindow {
			validTimes = append(validTimes, t)
		}
	}

	// Check per-window limit
	if len(validTimes) >= RespawnLimit {
		pm.disabled[name] = true
		pm.clearProcessCallbacks(name)
		pm.deletePluginStatusLabel(name)
		report.RaiseWarning(reportSourcePlugin, reportCodePluginDown, name,
			"plugin disabled (respawn limit exceeded): "+name,
			map[string]any{"limit": RespawnLimit, "window_seconds": int(RespawnWindow.Seconds())})
		logger().Warn("respawn limit exceeded, process disabled",
			"process", name, "limit", RespawnLimit, "window", RespawnWindow)
		return nil, ErrRespawnLimitExceeded
	}

	// Check cumulative limit (prevents cycling indefinitely across windows)
	pm.totalRespawns[name]++
	if pm.totalRespawns[name] > MaxTotalRespawns {
		pm.disabled[name] = true
		pm.clearProcessCallbacks(name)
		pm.deletePluginStatusLabel(name)
		report.RaiseWarning(reportSourcePlugin, reportCodePluginDown, name,
			"plugin disabled (cumulative respawn limit exceeded): "+name,
			map[string]any{"total_respawns": pm.totalRespawns[name], "limit": MaxTotalRespawns})
		logger().Warn("cumulative respawn limit exceeded, process disabled",
			"process", name, "total", pm.totalRespawns[name], "limit", MaxTotalRespawns)
		return nil, ErrRespawnLimitExceeded
	}

	// Record this respawn
	validTimes = append(validTimes, now)
	pm.respawnTimes[name] = validTimes

	// Stop the process this respawn replaces and JOIN its goroutines, whether or not
	// its engine is still running. The line further down overwrites pm.processes[name],
	// and that map entry is the only handle anything holds on the old process, so this
	// is the last moment it can be joined.
	//
	// Running() reports the ENGINE, and the engine is not the only goroutine a Process
	// owns: StartWithContext starts the event delivery loop first (process.go), and
	// that loop ends only when Stop closes the event channel (delivery.go, deliveryLoop
	// ranges over it). A respawn follows a crash, so an engine that has already
	// returned is the COMMON case here and was the one the old Running() guard skipped.
	// Every such cycle left a delivery loop blocked on a channel nobody could reach,
	// and ProcessManager.Stop walks pm.processes, so nothing ever joined it.
	//
	// Nil out the metrics callbacks first, so the dying process cannot re-create
	// deleted status labels via SetStage during shutdown.
	if proc, ok := pm.processes[name]; ok {
		proc.onStageChange = nil
		proc.deliveryInc = nil
		proc.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = proc.Wait(ctx)
		cancel()
	}

	// Start new process with acceptor if configured.
	newProc := NewProcess(*cfg)
	pm.wireMetrics(newProc)
	if pm.acceptor != nil && !cfg.Internal {
		newProc.SetAcceptor(pm.acceptor)
	}
	if err := newProc.StartWithContext(pm.ctx); err != nil {
		return nil, err
	}
	// A spawn clears the re-entry guard, exactly as startConfigs does. Stop cancels
	// pm.ctx before it waits, and startInternal (process.go) does not read that
	// context, so a Respawn landing after a Stop's engine wait leaves a LIVE engine
	// behind a flag that would send the next Stop down the second-entry path and
	// skip its release.
	pm.stopping.Store(false)
	pm.processes[name] = newProc

	// Successful restart clears the plugin-down warning.
	report.ClearWarning(reportSourcePlugin, reportCodePluginDown, name)

	// Increment restart counter after successful start.
	if pm.pmetrics != nil {
		pm.pmetrics.restarts.With(name).Inc()
	}

	return newProc, nil
}

// clearProcessCallbacks nils out metrics callbacks on the named process.
// Prevents a dying process from re-creating deleted metric labels via
// SetStage or Deliver during shutdown.
func (pm *ProcessManager) clearProcessCallbacks(name string) {
	if proc, ok := pm.processes[name]; ok {
		proc.onStageChange = nil
		proc.deliveryInc = nil
	}
}

// deletePluginStatusLabel removes the status gauge label for a plugin.
// Called when a plugin is disabled (respawn limit exceeded).
// Only deletes the status gauge; restarts and delivered counters are
// preserved for post-mortem monitoring (counters must not be deleted
// mid-lifetime as it breaks rate() and increase() queries).
func (pm *ProcessManager) deletePluginStatusLabel(name string) {
	if pm.pmetrics == nil {
		return
	}
	pm.pmetrics.status.Delete(name)
}
