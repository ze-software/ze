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

	"golang.org/x/sync/errgroup"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// respawnJoinMargin is how long Respawn waits, past the stopped process's own
// exit grace (stopExitGrace), for that process to end before it starts the
// replacement: the two make Process.StopJoinBound. A join that expires is
// raised as doctor-plugin-restart-overlap (recordRestartOverlap). A var for the same test seam as pluginStopGrace:
// never written outside a test.
var respawnJoinMargin = time.Second

// Respawn errors.
var (
	ErrRespawnLimitExceeded = errors.New("respawn limit exceeded")
	ErrProcessDisabled      = errors.New("process disabled due to respawn limit")
	ErrProcessNotFound      = errors.New("process not found")
	// ErrDaemonStopping refuses a respawn once the stop drain has begun: the
	// stop is terminal, and a respawned plugin would announce again what its
	// drain just made unattractive.
	ErrDaemonStopping = errors.New("respawn refused: ze is stopping")
)

// pluginMetrics holds Prometheus metrics for plugin health.
// Created by SetMetricsRegistry; nil when metrics are disabled.
type pluginMetrics struct {
	status    metrics.GaugeVec   // ze_plugin_status: current stage per plugin
	restarts  metrics.CounterVec // ze_plugin_restarts_total: cumulative restart count
	delivered metrics.CounterVec // ze_plugin_events_delivered_total: events enqueued
	failed    metrics.CounterVec // ze_plugin_delivery_failures_total: fire-and-forget batches that failed
	slow      metrics.CounterVec // ze_plugin_delivery_slow_batches_total: batches over DeliverySlowThreshold
}

// ProcessManager manages multiple external processes.
type ProcessManager struct {
	// configs holds exactly the plugins this manager is asked to run, one
	// entry per name: StartMore replaces a name it already holds. Nothing
	// drops an entry, so a plugin a reload removed stays listed. Protected by
	// mu.
	configs   []plugin.PluginConfig
	processes map[string]*Process

	// Respawn tracking: name -> list of respawn timestamps
	respawnTimes map[string][]time.Time

	// Cumulative respawn counts, reset only by an operator's restart
	// (Reenable).
	totalRespawns map[string]int

	// Disabled processes (respawn limit exceeded)
	disabled map[string]bool

	// stopped holds the record of every configured plugin that is down, and
	// why (stopped.go). A start that reaches running and a removal clear it.
	stopped map[string]PluginStop

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

	// cleanupLate names the plugins whose engine did not return within
	// pluginStopGrace at the last Stop, sorted. Protected by mu.
	cleanupLate []string

	// respawnRefused is set once by RefuseRespawns when the daemon's stop
	// drain begins, and never cleared: the stop is terminal.
	respawnRefused atomic.Bool
}

// RefuseRespawns makes every later Respawn fail with ErrDaemonStopping. The
// daemon calls it when its stop drain begins, so a plugin that exits during
// the drain stays down until teardown. It MUST be called only on the stop
// path: nothing clears it. Safe for concurrent use.
func (pm *ProcessManager) RefuseRespawns() {
	pm.respawnRefused.Store(true)
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
		failed:    reg.CounterVec("ze_plugin_delivery_failures_total", "Event batches sent with no caller waiting that the plugin failed to take", []string{reportSourcePlugin}),
		slow:      reg.CounterVec("ze_plugin_delivery_slow_batches_total", "Event batches that took over 500ms to deliver to the plugin", []string{reportSourcePlugin}),
	}
}

// wireMetrics publishes the metrics callbacks of a process (metric_hooks.go).
// No-op when metrics are disabled (pm.pmetrics is nil) or plugin name is empty.
// Called before the process is started; the publication is atomic, so a later
// call is safe too.
func (pm *ProcessManager) wireMetrics(proc *Process) {
	if pm.pmetrics == nil {
		return
	}
	name := proc.Name()
	if name == "" {
		return
	}
	m := pm.pmetrics
	proc.metricHooks.Store(&processMetricHooks{
		stageChange: func(stage plugin.PluginStage) {
			m.status.With(name).Set(float64(stage))
		},
		delivered: func() {
			m.delivered.With(name).Inc()
		},
		deliveryFailed: func() {
			m.failed.With(name).Inc()
		},
		deliverySlow: func() {
			m.slow.With(name).Inc()
		},
	})
}

// NewProcessManager creates a new process manager.
// It keeps its own copy of configs, because StartMore changes the list in
// place.
func NewProcessManager(configs []plugin.PluginConfig) *ProcessManager {
	return &ProcessManager{
		configs:       slices.Clone(configs),
		processes:     make(map[string]*Process),
		respawnTimes:  make(map[string][]time.Time),
		totalRespawns: make(map[string]int),
		disabled:      make(map[string]bool),
		stopped:       make(map[string]PluginStop),
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
	pm.mu.RLock()
	configs := slices.Clone(pm.configs)
	pm.mu.RUnlock()
	return pm.startConfigs(configs)
}

// StartMore starts the additional configs under the existing context.
// Must be called after StartWithContext (the context is captured then).
// Returns an error if the manager has not been started yet.
//
// A config whose name the manager already holds REPLACES the held one, in its
// place: a plugin a reload removed and added back runs, and is restarted
// (Respawn), with the config it was added back with, never the stale one.
func (pm *ProcessManager) StartMore(configs []plugin.PluginConfig) error {
	if pm.ctx == nil {
		return errProcessmanagerStartmoreCalledBeforeStartwithcontext
	}
	if len(configs) == 0 {
		return nil
	}
	pm.mu.Lock()
	for i := range configs {
		index := slices.IndexFunc(pm.configs, func(held plugin.PluginConfig) bool {
			return held.Name == configs[i].Name
		})
		if index < 0 {
			pm.configs = append(pm.configs, configs[i])
			continue
		}
		pm.configs[index] = configs[i]
	}
	pm.mu.Unlock()
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
// connection is the shutdown signal for internal plugins, and for external
// plugins too: on Linux each external plugin's superviseExit closes its
// connection when the context ends and kills its process group only if it is
// still running pluginExitGrace later; elsewhere the canceled context kills the
// group at once (newPluginShell, exitwait_*.go).
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
	// For external plugins: the same closed connection ends the plugin's SDK Run,
	// and on Linux its superviseExit kills the process group only when the plugin is
	// still running pluginExitGrace later (process.go); elsewhere this cancel kills
	// the group at once.
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
			"code", diagnostic.CodeDoctorPluginStopCleanupLate, "plugins", strings.Join(late, ","), "grace", pluginStopGrace)
	}
	pm.mu.Lock()
	pm.cleanupLate = late
	pm.mu.Unlock()

	// Then the goroutine-leak wait, unchanged: the group holds the delivery loop and
	// the external-process monitor as well, and this is the "call Wait after Stop"
	// half of the Process contract rather than a cleanup guarantee. Each process gets
	// 500ms past its own exit grace: the monitor of an external plugin that ignores its
	// closed connection returns only once pluginExitGrace has killed it, and an
	// internal plugin has no such grace to wait out (stopExitGrace).
	pm.mu.RLock()
	var groupWg sync.WaitGroup
	for _, proc := range pm.processes {
		groupWg.Add(1)
		go func(p *Process) {
			defer groupWg.Done()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), p.stopExitGrace()+500*time.Millisecond)
			defer waitCancel()
			_ = p.Wait(waitCtx)
		}(proc)
	}
	pm.mu.RUnlock()
	groupWg.Wait()

	pm.mu.Lock()
	pm.processes = make(map[string]*Process)
	pm.mu.Unlock()
}

// CleanupLate answers the plugins whose engine did not finish its shutdown
// cleanup within the grace at the last Stop, sorted; none before a Stop. The
// plugin server writes them to the last-stop record, so the next local
// `ze doctor` reports doctor-plugin-stop-cleanup-late. Safe for concurrent use.
func (pm *ProcessManager) CleanupLate() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return slices.Clone(pm.cleanupLate)
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

// ProcessServing answers the process that runs the registration a
// `<plugin>:<filter>` chain reference or a filter insertion names
// (Process.RegistrationName). It answers nil when no process runs it.
//
// The process filed under the registration's own name answers in one map
// lookup, which is the common `plugin { internal bgp-gr { ... } }` and every
// auto-loaded plugin; only an aliased block (`internal gr { use bgp-gr; }`)
// walks the processes. When two blocks run the registration the walk answers
// an error naming both, because the reference cannot say which one judges the
// route. Stage 1 refuses the second such block of a plugin that declares
// filters or insertions (BlocksServing), so the error is a guard, not a path
// a running daemon takes. Safe for concurrent use.
func (pm *ProcessManager) ProcessServing(registration string) (*Process, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if proc := pm.processes[registration]; proc != nil && proc.registryName == registration {
		return proc, nil
	}
	var serving *Process
	for _, proc := range pm.processes {
		if proc.registryName != registration {
			continue
		}
		if serving != nil {
			return nil, AmbiguousRegistration(registration, []string{serving.Name(), proc.Name()})
		}
		serving = proc
	}
	return serving, nil
}

// BlocksServing answers the names of every process that runs registration,
// sorted. Stage 1 asks it before a plugin's filters and insertions are
// accepted: more than one name means a chain reference to the plugin would
// be ambiguous. Safe for concurrent use.
func (pm *ProcessManager) BlocksServing(registration string) []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var names []string
	for _, proc := range pm.processes {
		if proc.registryName == registration {
			names = append(names, proc.Name())
		}
	}
	slices.Sort(names)
	return names
}

// AmbiguousRegistration is the refusal of a filter reference that more than
// one plugin block serves: what is wrong, why, and the fix.
func AmbiguousRegistration(registration string, blocks []string) error {
	sorted := slices.Clone(blocks)
	slices.Sort(sorted)
	var tb textbuf.Buffer
	tb.Str("plugin blocks ")
	for i, name := range sorted {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Quoted(name)
	}
	tb.Str(" all run ").Quoted(registration).Str(", and a filter reference ").Quoted(registration + ":<filter>").
		Str(" names the plugin, not the block, so it cannot say which one judges the route; ").
		Str("keep one block that runs ").Quoted(registration).Str(" under plugin { }")
	return errors.New(tb.String())
}

// RemoveProcess unregisters a stopped process by name. The plugin stays
// configured (ConfiguredNames, Respawn): a failed startup or restart handshake
// removes its process this way and records it down. A reload's removal does
// too, so a plugin the configuration no longer names stays configured: nothing
// drops its name from the configured set.
//
// A removed plugin is not down, so its stop record and its bus warning go
// with it, and so do the warnings it raised itself (report-issue) and its
// crash: its own source is cleared, because only the plugin could clear it.
func (pm *ProcessManager) RemoveProcess(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.processes, name)
	pm.clearStoppedLocked(name)
	report.ClearSource(name)
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

// ConfiguredNames returns the name of every plugin this manager is asked to
// run, once each, in the order it was first given them (NewProcessManager,
// then each StartMore). A name stays configured after its process ends or is
// disabled, which is what lets a reader tell a stopped plugin from an absent
// one. Nothing removes a name, so a plugin a reload removed is still listed.
// Safe for concurrent use.
func (pm *ProcessManager) ConfiguredNames() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	names := make([]string, 0, len(pm.configs))
	for i := range pm.configs {
		names = append(names, pm.configs[i].Name)
	}
	return names
}

// Disabled reports whether the respawn limit disabled the named process
// (Respawn). Safe for concurrent use.
func (pm *ProcessManager) Disabled(name string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.disabled[name]
}

// reportSourcePlugin is the report-bus source of every raise this manager
// makes. The codes are the doctor codes (diagnostic.CodeDoctorPluginCrashed,
// diagnostic.CodeDoctorPluginStopped), so `ze doctor` and the bus name one
// fault the same way.
const reportSourcePlugin = "plugin"

// ReportCrash records that a named plugin's process ended when nothing asked it
// to, with how it ended (exitStatus) and the failure policy ze applies to it
// (policy). The engine calls it for EVERY unexpected exit, before it acts on
// the policy (Server.applyFailurePolicy,
// internal/component/plugin/server/failure_policy.go).
//
// It is separate from Respawn because a crash and a restart are different
// events. A plugin that asked not to be started again still crashed, and an
// operator reading `ze doctor` needs that row whether or not ze acted on it.
//
// The warnings the plugin raised itself through report-issue are cleared
// here: their owner is gone and nothing else can clear them, so they would
// outlive it as stale faults. doctor-plugin-crashed and doctor-plugin-stopped
// then report the down.
//
// A process with no exit recorded within the wait is not said to have ended:
// the event says its connection closed and its process had not ended, and
// carries no exit-status detail.
func (pm *ProcessManager) ReportCrash(name string, exit ProcessExit, policy string) {
	report.ClearSource(name)
	var message textbuf.Buffer
	detail := map[string]any{"policy": policy}
	switch exit.kind {
	case exitNone:
		message.Str("plugin process exited unexpectedly: ").Str(name)
	case exitRecorded:
		message.Str("plugin process exited unexpectedly: ").Str(name).
			Str("; it ended with ").Str(exit.status)
		detail["exit-status"] = exit.status
	case exitUnrecorded:
		message.Str("plugin ").Str(name).
			Str(" closed its connection unexpectedly, and its process had not ended ").
			Str(exitStatusWaitMax.String()).Str(" later")
	}
	message.Str(", and ze applies its failure policy ").Str(policy)
	// A fork's exit status can carry a wait error's text, so the message is
	// fitted to the bus bound rather than refused. The subject is the plugin
	// name, at most 64 characters (ipc.PluginNameForm), and the fix is fixed.
	report.RaiseErrorFix(reportSourcePlugin, diagnostic.CodeDoctorPluginCrashed, name,
		report.FitMessage(message.String()), "ze show errors", detail)
}

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
// It asks NO question about whether this plugin may be restarted. The plugin's
// own declaration answers that, and the caller has already read it
// (rpc.FailurePolicy). The config-reload rollback path calls this for a plugin
// it has been told is broken, which is an order rather than a policy decision.
//
// Returns ErrRespawnLimitExceeded if limit exceeded within window.
// Returns ErrProcessDisabled if process was previously disabled.
// Returns ErrProcessNotFound if process name not in configuration.
// Returns ErrDaemonStopping once RefuseRespawns has been called.
// Returns error if ProcessManager was not started (ctx is nil).
func (pm *ProcessManager) Respawn(name string) (*Process, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if ProcessManager was started
	if pm.ctx == nil {
		return nil, errors.New("process manager not started")
	}

	if pm.respawnRefused.Load() {
		return nil, ErrDaemonStopping
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
		var detail textbuf.Buffer
		detail.Str("it was started ").Int(RespawnLimit).Str(" times within ").
			Str(RespawnWindow.String()).Str(", the per-window limit")
		pm.recordStoppedLocked(name, PluginStop{Cause: StopRespawnLimit, Detail: detail.String()})
		return nil, ErrRespawnLimitExceeded
	}

	// Check cumulative limit (prevents cycling indefinitely across windows)
	pm.totalRespawns[name]++
	if pm.totalRespawns[name] > MaxTotalRespawns {
		pm.disabled[name] = true
		pm.clearProcessCallbacks(name)
		pm.deletePluginStatusLabel(name)
		var detail textbuf.Buffer
		detail.Str("it was started ").Int(int64(pm.totalRespawns[name])).Str(" times, above the cumulative limit of ").
			Int(MaxTotalRespawns)
		pm.recordStoppedLocked(name, PluginStop{Cause: StopRespawnLimit, Detail: detail.String()})
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
	// Take the metrics callbacks back first, so the dying process cannot re-create
	// deleted status labels via SetStage during shutdown. The process may be live
	// (request plugin restart restarts a running plugin), so its delivery and
	// stage goroutines read the callbacks while this runs: clearMetricHooks is the
	// atomic swap they read through (metric_hooks.go).
	if proc, ok := pm.processes[name]; ok {
		proc.clearMetricHooks()
		proc.Stop()
		// An external plugin that does not exit on its closed connection is killed
		// only once pluginExitGrace has passed, so the join waits past its grace
		// (StopJoinBound). A join that still expires means the replacement starts
		// before the old process finished stopping: that is said, never dropped,
		// because nothing else records it.
		joinBound := proc.StopJoinBound()
		ctx, cancel := context.WithTimeout(context.Background(), joinBound)
		joinErr := proc.Wait(ctx)
		cancel()
		if joinErr != nil {
			recordRestartOverlap(proc, joinBound, joinErr)
		}
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

	// A respawned process has not run its handshake yet, so the stop record
	// stays until that generation reaches running (Server.superviseProcess
	// calls ClearStopped); a failed handshake replaces it.

	// Increment restart counter after successful start.
	if pm.pmetrics != nil {
		pm.pmetrics.restarts.With(name).Inc()
	}

	return newProc, nil
}

// clearProcessCallbacks takes the metrics callbacks back from the named
// process (clearMetricHooks), so a dying process cannot re-create deleted
// metric labels via SetStage or Deliver during shutdown. Caller holds pm.mu.
func (pm *ProcessManager) clearProcessCallbacks(name string) {
	if proc, ok := pm.processes[name]; ok {
		proc.clearMetricHooks()
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

// DrainEventDelivery returns when every process has delivered the events it
// held when the call started. It is the inbound half of `request quiesce`: the
// two reactor quiescers drain what ze owes the WIRE, and this drains what ze
// owes its PLUGINS.
//
// Without it a barrier built on quiesce is unsound in one direction. A command
// that reaches a plugin through the event rail, such as an RPKI validation
// decision returning to the adj-RIB-in, is still in a 64-deep channel when
// quiesce answers, so a caller told "settled" reads state the decision has not
// reached yet.
//
// Every process is drained concurrently: they are independent, and a serial
// walk would charge the slowest plugin's timeout to each of the others.
func (pm *ProcessManager) DrainEventDelivery(ctx context.Context) error {
	pm.mu.RLock()
	procs := make([]*Process, 0, len(pm.processes))
	for _, proc := range pm.processes {
		procs = append(procs, proc)
	}
	pm.mu.RUnlock()

	var group errgroup.Group
	for _, proc := range procs {
		group.Go(func() error { return proc.DrainEvents(ctx) })
	}
	return group.Wait()
}
