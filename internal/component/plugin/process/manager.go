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
	"github.com/ze-software/ze/internal/core/opererr"
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
	// ErrManagerNotStarted refuses a respawn before StartWithContext has run.
	ErrManagerNotStarted = errors.New("process manager not started")
)

// pluginMetrics holds Prometheus metrics for plugin health.
// Created by SetMetricsRegistry; nil when metrics are disabled.
type pluginMetrics struct {
	status            metrics.GaugeVec   // ze_plugin_status: current stage per plugin
	restarts          metrics.CounterVec // ze_plugin_restarts_total: cumulative restart count
	delivered         metrics.CounterVec // ze_plugin_events_delivered_total: events enqueued
	failed            metrics.CounterVec // ze_plugin_delivery_failures_total: fire-and-forget batches that failed
	slow              metrics.CounterVec // ze_plugin_delivery_slow_batches_total: batches over DeliverySlowThreshold
	queueDepth        metrics.GaugeVec   // ze_plugin_delivery_queue_depth: current queued event count per plugin
	backpressureWaits metrics.CounterVec // ze_plugin_delivery_backpressure_waits_total: producer waits at queue ceiling
}

// ProcessManager manages multiple external processes.
type ProcessManager struct {
	// configs holds exactly the plugins this manager is asked to run, one
	// entry per name: StartMore replaces a name it already holds, and
	// UnconfigureProcess drops a name once its removal is final. Protected by
	// mu.
	configs   []plugin.PluginConfig
	processes map[string]*Process

	// Respawn tracking: name -> list of respawn timestamps
	respawnTimes map[string][]time.Time

	// Cumulative respawn counts, reset only by an operator's restart
	// (Reenable, RespawnReenabled).
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

	// statusMu is a leaf lock serializing ze_plugin_status and
	// ze_plugin_delivery_queue_depth gauge updates in wireMetrics against
	// deletePluginStatusLabel so concurrent updates cannot re-create deleted
	// gauge labels after clearMetricHooks. NEVER acquire pm.mu or p.stageMu
	// while holding statusMu.
	statusMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	// respawning serializes concurrent Respawn calls for the same plugin name
	// without blocking readers or respawns of unrelated names. Protected by mu.
	respawning map[string]chan struct{}

	// stopping is true from the moment Stop signals until the next spawn clears it
	// (startConfigs). A second entry to Stop returns at once instead of waiting for
	// the engines a first entry is already waiting for. See Stop for why.
	stopping   atomic.Bool
	stopActive bool // mu fences admission while either stop owner is joining.

	// Terminal admission and unpublished owners share mu with stop ownership.
	shutdown atomic.Pointer[managerShutdown]
	spawns   sync.WaitGroup        // Add is fenced under mu before shutdown joins it.
	starting map[*Process]struct{} // Unpublished owners, protected by mu.

	// cleanupLate names the plugins whose engine did not return within
	// pluginStopGrace at the last Stop, sorted. Protected by mu.
	cleanupLate []string

	// shutdownOrderFailure retains only the first ordering refusal of this
	// terminal stop. Its bounded operator message is protected by mu.
	shutdownOrderFailure *opererr.Error

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
		status:            reg.GaugeVec("ze_plugin_status", "Current plugin stage (0=init, 6=running)", []string{reportSourcePlugin}),
		restarts:          reg.CounterVec("ze_plugin_restarts_total", "Total plugin restart attempts (process started)", []string{reportSourcePlugin}),
		delivered:         reg.CounterVec("ze_plugin_events_delivered_total", "Total events delivered to plugin", []string{reportSourcePlugin}),
		failed:            reg.CounterVec("ze_plugin_delivery_failures_total", "Event batches sent with no caller waiting that the plugin failed to take", []string{reportSourcePlugin}),
		slow:              reg.CounterVec("ze_plugin_delivery_slow_batches_total", "Event batches that took over 500ms to deliver to the plugin", []string{reportSourcePlugin}),
		queueDepth:        reg.GaugeVec("ze_plugin_delivery_queue_depth", "Current number of events queued for delivery to the plugin", []string{reportSourcePlugin}),
		backpressureWaits: reg.CounterVec("ze_plugin_delivery_backpressure_waits_total", "Producer waits when the plugin event delivery queue reached its ceiling", []string{reportSourcePlugin}),
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
	var hooks *processMetricHooks
	hooks = &processMetricHooks{
		stageChange: func(stage plugin.PluginStage) {
			pm.statusMu.Lock()
			defer pm.statusMu.Unlock()
			if proc.metricHooks.Load() != hooks {
				return
			}
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
		deliveryQueueDepth: func(depth int) {
			pm.statusMu.Lock()
			defer pm.statusMu.Unlock()
			if proc.metricHooks.Load() != hooks {
				return
			}
			m.queueDepth.With(name).Set(float64(depth))
		},
		deliveryBackpressure: func() {
			m.backpressureWaits.With(name).Inc()
		},
	}
	proc.metricHooks.Store(hooks)
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
		respawning:    make(map[string]chan struct{}),
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
	pm.mu.Lock()
	if pm.shutdown.Load() != nil {
		pm.mu.Unlock()
		return ErrDaemonStopping
	}
	if pm.stopActive {
		pm.mu.Unlock()
		return ErrDaemonStopping
	}
	pm.ctx, pm.cancel = context.WithCancel(ctx)
	configs := slices.Clone(pm.configs)
	pm.mu.Unlock()
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
	pm.mu.Lock()
	if pm.shutdown.Load() != nil {
		pm.mu.Unlock()
		return ErrDaemonStopping
	}
	if pm.stopActive {
		pm.mu.Unlock()
		return ErrDaemonStopping
	}
	if pm.ctx == nil {
		pm.mu.Unlock()
		return errProcessmanagerStartmoreCalledBeforeStartwithcontext
	}
	if len(configs) == 0 {
		pm.mu.Unlock()
		return nil
	}
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

// StartEach starts every config and answers, by plugin name, the error of each
// one that did not start. Unlike StartWithContext and StartMore, one plugin
// that fails to start stops nothing: the others stay running and held, and the
// caller decides what a failure means. A config that started is registered
// like any other process; a config that failed is not.
//
// The daemon calls it at boot for the external plugins the config declares,
// before the full config can be parsed, so a plugin that cannot be run is an
// outcome the load reports and never the end of the plugins that did run.
//
// The first start of a manager captures ctx for every later StartMore; a
// manager already started keeps its context and ctx is not read. Configs MUST
// hold no name the manager already runs.
//
// environment holds the settings of the config's `environment` block
// (config.EnvironmentEntries), which the daemon has not applied to itself at
// that point: each external plugin forked here is given them for its own
// environment (Process.SetConfigEnvironment). They are kept nowhere else, so
// a Respawn of the same plugin forks from the daemon's environment alone,
// which by then holds the block.
func (pm *ProcessManager) StartEach(ctx context.Context, configs []plugin.PluginConfig, environment []string) map[string]error {
	pm.mu.Lock()
	if pm.shutdown.Load() != nil {
		pm.mu.Unlock()
		failures := make(map[string]error, len(configs))
		for _, cfg := range configs {
			failures[cfg.Name] = ErrDaemonStopping
		}
		return failures
	}
	if pm.stopActive {
		pm.mu.Unlock()
		failures := make(map[string]error, len(configs))
		for _, cfg := range configs {
			failures[cfg.Name] = ErrDaemonStopping
		}
		return failures
	}
	if pm.ctx == nil {
		pm.ctx, pm.cancel = context.WithCancel(ctx)
	}
	for i := range configs {
		held := slices.ContainsFunc(pm.configs, func(config plugin.PluginConfig) bool {
			return config.Name == configs[i].Name
		})
		if !held {
			pm.configs = append(pm.configs, configs[i])
		}
	}
	pm.mu.Unlock()

	failures := make(map[string]error)
	for _, cfg := range configs {
		if err := pm.startProcess(cfg, environment); err != nil {
			failures[cfg.Name] = err
		}
	}
	return failures
}

// startConfigs spawns the given configs under pm.ctx and registers them in
// pm.processes. Used by both StartWithContext (initial spawn) and StartMore
// (incremental spawn). The shared map lets AllProcesses return every process
// regardless of which spawn phase created it.
func (pm *ProcessManager) startConfigs(configs []plugin.PluginConfig) error {
	// Every spawn is admitted and owned before Start; a failure remains owned
	// through its real process/engine join, including a terminal stop racing it.
	for _, cfg := range configs {
		if err := pm.startProcess(cfg, nil); err != nil {
			// Stop already started processes
			pm.Stop()
			return err
		}
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
	pm.stop(nil, false)
}

// StopInOrder seals process admission and reverses the tiers resolved from the
// sealed process snapshot. order MUST return every name once in startup order;
// nil tiers request simultaneous forced stop after its diagnostic. No Bye is
// sent. One pluginStopGrace bounds all cleanup tiers. Unlike reusable Stop,
// this operation is terminal. Reentrant calls do not wait for themselves.
// The daemon owner MUST inspect ShutdownOrderFailure after this returns and
// persist any failure before its store closes. Safe for concurrent use.
func (pm *ProcessManager) StopInOrder(order func([]plugin.PluginConfig) [][]string) {
	pm.stop(order, true)
}

func (pm *ProcessManager) stop(order func([]plugin.PluginConfig) [][]string, terminal bool) {
	engineCtx, engineCancel := context.WithTimeout(context.Background(), pluginStopGrace)
	defer engineCancel()
	// One entry owns cleanup. Reentrant calls cannot wait for their own engine;
	// during ordered stop they also MUST leave still-needed providers live.
	//
	// This is a guard, not the cure. The known re-entrant caller was a plugin engine
	// whose own shutdown reached back here (Reactor.cleanup stopping the server that
	// hosts it), and Reactor.cleanup now stops only a server it owns. The guard stays
	// because a second entry can only ever cost time: a plugin engine calling Stop on
	// its way out would otherwise wait for itself.
	// Publish the owner BEFORE any side effect can wake a failed start. That
	// start calls ordinary Stop; it must find this fence, never cancel providers.
	// No Start, process join or connect-back holds mu across a blocking operation.
	pm.mu.Lock()
	if pm.shutdown.Load() != nil {
		pm.mu.Unlock()
		return
	}
	entered := pm.stopping.Swap(true)
	if terminal {
		pm.shutdown.Store(&managerShutdown{ctx: engineCtx})
		pm.respawnRefused.Store(true)
	}
	managerCancel := pm.cancel
	if !entered {
		pm.stopActive = true
	}
	pm.mu.Unlock()
	// The terminal fence precedes diagnostics too. A reusable stop may
	// already own cleanup and never invoke this entry's resolver, but a
	// missing terminal resolver is still a real loss of dependency ordering.
	if terminal {
		if order == nil {
			pm.RecordShutdownOrderFailure(errors.New("no plugin dependency resolver was supplied"))
		}
	}
	if entered {
		return
	}
	defer func() {
		pm.mu.Lock()
		pm.stopActive = false
		pm.mu.Unlock()
	}()

	// Reusable Stop cancels the shared parent immediately. A terminal stop
	// keeps it live until every dependent has finished using its providers.
	// For internal plugins: closing engine-side net.Pipe unblocks the plugin's
	// ReadRequest, causing it to return an error and exit the event loop.
	// For external plugins: the same closed connection ends the plugin's SDK Run,
	// and on Linux its superviseExit kills the process group only when the plugin is
	// still running pluginExitGrace later (process.go); elsewhere this cancel kills
	// the group at once.
	if !terminal {
		if managerCancel != nil {
			managerCancel()
		}
	}

	// Stop TLS acceptor if running (closes listener, cancels accept loop).
	if pm.acceptor != nil {
		pm.acceptor.Stop()
	}

	late := pm.joinStarting(engineCtx)
	processes := pm.AllProcesses()
	late = append(late, pm.stopEngines(engineCtx, processes, order, terminal)...)
	if managerCancel != nil {
		managerCancel()
	}

	// Report cleanup that missed the shared deadline; never imply it finished.
	//
	// Closing the connection unblocks a plugin's read loop at once, but the read loop
	// is not the whole engine: an engine releases what it installed AFTER its loop
	// ends, and that release can be netlink round-trips. The IKE engine's is its
	// catch-all and operator XFRM policy deletes plus a backend close (engine/runEngine), and
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
	// Internal plugins wait on the engine, not their delivery group: the QEMU
	// bgp delivery loop could take over 3s after its engine released everything.
	// An external consumer instead owes its real process and stderr-relay join
	// before providers close. Both waits use the same deadline, never a new grace.
	//
	// The bound is the daemon's own shutdown budget, so there is one number to reason
	// about rather than two: cmd/ze/hub/main.go gives eng.Stop 3s and warns when it is
	// missed. An engine that returns promptly costs nothing here.
	if len(late) > 0 {
		slices.Sort(late)
		late = slices.Compact(late)
		logger().Warn("plugin engine did not finish its shutdown cleanup in time, resources it installed may be left behind",
			"code", CodeDoctorPluginStopCleanupLate, "plugins", strings.Join(late, ","), "grace", pluginStopGrace)
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
	var groupWg sync.WaitGroup
	for _, proc := range processes {
		groupWg.Add(1)
		go func(p *Process) {
			defer groupWg.Done()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), p.stopExitGrace()+500*time.Millisecond)
			defer waitCancel()
			_ = p.Wait(waitCtx)
		}(proc)
	}
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
// lookup, which is the common `bus { endpoint bgp-gr { internal bgp-gr; } }` and every
// auto-loaded plugin; only an aliased block (`endpoint gr { internal bgp-gr; }`)
// walks the processes. When two blocks run the registration the walk answers
// an error naming both, because the reference cannot say which one judges the
// route. Stage 1 refuses the second such block of a plugin that declares
// filters or insertions (BlocksServing), so the error is a guard, not a path
// a running daemon takes. Safe for concurrent use.
func (pm *ProcessManager) ProcessServing(registration string) (*Process, error) {
	if registration == "" {
		return nil, nil
	}
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
	if registration == "" {
		return nil
	}
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

// RegistrationFor returns the compiled registration identity for the
// configured endpoint name, or empty string for a foreign external program or
// an unknown endpoint. Works even when the endpoint is stopped or disabled.
// Safe for concurrent use.
func (pm *ProcessManager) RegistrationFor(name string) string {
	if name == "" {
		return ""
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for i := range pm.configs {
		if pm.configs[i].Name == name {
			return pm.configs[i].CompiledIdentity()
		}
	}
	if proc := pm.processes[name]; proc != nil {
		return proc.RegistrationName()
	}
	return ""
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
		Str("keep one block that runs ").Quoted(registration).Str(" under bus { }")
	return errors.New(tb.String())
}

// RemoveProcess unregisters a stopped process by name. The plugin stays
// configured (ConfiguredNames, Respawn): a failed startup or restart handshake
// removes its process this way and records it down, and a reload's removal
// keeps it configured until the reload is accepted and UnconfigureProcess
// drops it.
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

// UnconfigureProcess drops name from the manager's configured set once a
// reload removal is final or a rolled-back reload start ends, along with its
// respawn history, disable state, stop record, and status gauge label. It is a
// no-op when a live process is currently registered under name (for example, a
// replacement started under the same name). Safe for concurrent use.
func (pm *ProcessManager) UnconfigureProcess(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.processes[name] != nil {
		return
	}
	pm.configs = slices.DeleteFunc(pm.configs, func(cfg plugin.PluginConfig) bool {
		return cfg.Name == name
	})
	delete(pm.disabled, name)
	delete(pm.respawnTimes, name)
	delete(pm.totalRespawns, name)
	pm.clearStoppedLocked(name)
	pm.deletePluginStatusLabel(name)
}

// AddProcess registers a process by name. Used by tests to inject mock processes.
// Wires metrics callbacks if a metrics registry is configured.
// A successful call transfers cleanup ownership to the manager. On refusal the
// caller retains ownership and MUST call Stop and Wait. Safe for concurrent use.
func (pm *ProcessManager) AddProcess(name string, proc *Process) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.shutdown.Load() != nil {
		return ErrDaemonStopping
	}
	if pm.stopActive {
		return ErrDaemonStopping
	}
	pm.wireMetrics(proc)
	// The third spawn site, and it clears the re-entry guard for the same reason as
	// the other two: the manager now holds a process whose engine a Stop must wait for.
	pm.stopping.Store(false)
	pm.processes[name] = proc
	return nil
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
// one. UnconfigureProcess drops a name once a reload's removal is final or a
// rolled-back reload start ends. Safe for concurrent use.
func (pm *ProcessManager) ConfiguredNames() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	names := make([]string, 0, len(pm.configs))
	for i := range pm.configs {
		names = append(names, pm.configs[i].Name)
	}
	return names
}

// ConfiguredConfigs returns a copy of every plugin configuration this manager
// currently holds, sorted by Name. Safe for concurrent use.
func (pm *ProcessManager) ConfiguredConfigs() []plugin.PluginConfig {
	pm.mu.RLock()
	configs := slices.Clone(pm.configs)
	pm.mu.RUnlock()
	slices.SortFunc(configs, func(a, b plugin.PluginConfig) int {
		return strings.Compare(a.Name, b.Name)
	})
	return configs
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
// Respawns are serialized per plugin name (respawning) rather than across the
// whole manager, and pm.mu is released before stopping/joining the outgoing
// generation or starting the replacement (including TLS connect-back). Readers
// (GetProcess, AllProcesses, ConfiguredNames, ConfiguredConfigs) and respawns
// of unrelated plugin names never wait on another plugin's join or
// connect-back. The outgoing generation is withdrawn from pm.processes under
// pm.mu before Stop wakes its supervisor, so applyFailurePolicy sees
// GetProcess(name) != old without blocking.
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
// Returns ErrManagerNotStarted if ProcessManager was not started (ctx is nil).
func (pm *ProcessManager) Respawn(name string) (*Process, error) {
	return pm.respawn(name, false)
}

// RespawnReenabled is Respawn for an operator's explicit restart command
// (`request plugin restart`). It runs every pre-spawn admission check first
// (manager started, daemon not stopping, name configured), and only after all
// pass atomically clears any respawn-limit disable and respawn counters under
// pm.mu before admitting the replacement. A refused call leaves disable state
// and respawn history untouched. Safe for concurrent use.
func (pm *ProcessManager) RespawnReenabled(name string) (*Process, error) {
	return pm.respawn(name, true)
}

// acquireRespawnSlot serializes concurrent respawns of the same plugin name
// without blocking readers or respawns of unrelated names. The caller MUST
// call the returned release function once the respawn finishes.
func (pm *ProcessManager) acquireRespawnSlot(name string) func() {
	pm.mu.Lock()
	for {
		ch, busy := pm.respawning[name]
		if !busy {
			break
		}
		pm.mu.Unlock()
		<-ch
		pm.mu.Lock()
	}
	if pm.respawning == nil {
		pm.respawning = make(map[string]chan struct{})
	}
	done := make(chan struct{})
	pm.respawning[name] = done
	pm.mu.Unlock()
	return func() {
		pm.mu.Lock()
		delete(pm.respawning, name)
		close(done)
		pm.mu.Unlock()
	}
}

func (pm *ProcessManager) respawn(name string, reenable bool) (*Process, error) {
	releaseSlot := pm.acquireRespawnSlot(name)
	defer releaseSlot()

	pm.mu.Lock()
	pmLocked := true
	// Only rate/config bookkeeping and admission hold mu. Every early return
	// below precedes ownership transfer and releases that lock.
	defer func() {
		if pmLocked {
			pm.mu.Unlock()
		}
	}()

	// Check if ProcessManager was started.
	if pm.ctx == nil {
		return nil, ErrManagerNotStarted
	}

	if pm.respawnRefused.Load() {
		return nil, ErrDaemonStopping
	}
	if pm.shutdown.Load() != nil {
		return nil, ErrDaemonStopping
	}
	if pm.stopActive {
		return nil, ErrDaemonStopping
	}

	// Find config before mutating any disable or respawn state so an unknown
	// name is always refused without side effects.
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

	// Lift disable state and respawn counters only after all pre-spawn
	// admission checks have passed, atomically under pm.mu.
	if reenable {
		delete(pm.disabled, name)
		delete(pm.respawnTimes, name)
		delete(pm.totalRespawns, name)
	}

	// Check if already disabled.
	if pm.disabled[name] {
		return nil, ErrProcessDisabled
	}

	now := time.Now()

	// Clean up old respawn times (outside window).
	var validTimes []time.Time
	for _, t := range pm.respawnTimes[name] {
		if now.Sub(t) < RespawnWindow {
			validTimes = append(validTimes, t)
		}
	}

	// Check per-window limit.
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

	// Check cumulative limit (prevents cycling indefinitely across windows).
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

	// Record this respawn.
	validTimes = append(validTimes, now)
	pm.respawnTimes[name] = validTimes

	// Own the replacement before releasing mu. Ordered stop can now publish its
	// fence and wake connect-back without waiting for this caller's process join.
	newProc := pm.admitSpawnLocked(*cfg)
	ctx := pm.ctx
	old := pm.processes[name]
	if old != nil {
		// Intentional retirement MUST withdraw the exact outgoing generation
		// before Stop wakes its failure-policy supervisor. It stays physically
		// owned by this admitted spawn until its real join below succeeds.
		delete(pm.processes, name)
		pm.starting[old] = struct{}{}
	}
	pmLocked = false
	pm.mu.Unlock()
	defer pm.spawns.Done()
	if old != nil {
		old.clearMetricHooks()
		old.Stop()
		joinBound := old.StopJoinBound()
		joinCtx, cancel := context.WithTimeout(context.Background(), joinBound)
		joinErr := old.Wait(joinCtx)
		cancel()
		if joinErr != nil {
			// A timed-out outgoing generation MUST remain owned for terminal
			// shutdown's join, even after the replacement occupies its name.
			recordRestartOverlap(old, joinBound, joinErr)
		} else {
			pm.mu.Lock()
			delete(pm.starting, old)
			pm.mu.Unlock()
		}
	}
	if pm.shutdown.Load() != nil {
		pm.retireRejectedStart(newProc)
		return nil, ErrDaemonStopping
	}
	if err := pm.startAdmitted(newProc, ctx); err != nil {
		return nil, err
	}

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
// process (clearMetricHooks), so a dying or disabled process cannot re-create
// deleted metric labels via SetStage. MUST be called before
// deletePluginStatusLabel; caller holds pm.mu.
func (pm *ProcessManager) clearProcessCallbacks(name string) {
	if proc, ok := pm.processes[name]; ok {
		proc.clearMetricHooks()
	}
}

// deletePluginStatusLabel removes the per-plugin gauge labels (status and
// delivery queue depth) under leaf lock statusMu, serialized with wireMetrics's
// gauge callbacks so concurrent updates cannot re-create deleted gauge series
// after clearMetricHooks. Called when a plugin is disabled (respawn limit
// exceeded) or unconfigured. Only deletes gauges; restarts, delivered, failed,
// slow, and backpressure-wait counters are preserved for post-mortem
// monitoring (counters must not be deleted mid-lifetime as it breaks rate()
// and increase() queries).
func (pm *ProcessManager) deletePluginStatusLabel(name string) {
	if pm.pmetrics == nil {
		return
	}
	pm.statusMu.Lock()
	defer pm.statusMu.Unlock()
	pm.pmetrics.status.Delete(name)
	pm.pmetrics.queueDepth.Delete(name)
}

// DrainEventDelivery joins application of prior events and each live process's
// declared deferred work. It is the inbound half of `request quiesce`: the two
// reactor quiescers drain what ze owes the WIRE, and this drains what ze owes
// its PLUGINS. Processes already stopped when captured owe no work; a captured
// live process that stops during settlement fails its receipt.
//
// Without it a barrier built on quiesce is unsound in one direction. A command
// that reaches a plugin through the event rail, such as an RPKI validation
// decision returning to the adj-RIB-in, is still in the event delivery queue
// when quiesce answers, so a caller told "settled" reads state the decision
// has not reached yet.
//
// Every process is drained concurrently: they are independent, and a serial
// walk would charge the slowest plugin's timeout to each of the others.
func (pm *ProcessManager) DrainEventDelivery(ctx context.Context) error {
	pm.mu.RLock()
	procs := make([]*Process, 0, len(pm.processes))
	for _, proc := range pm.processes {
		if proc.Running() {
			procs = append(procs, proc)
		}
	}
	pm.mu.RUnlock()

	var group errgroup.Group
	for _, proc := range procs {
		group.Go(func() error { return proc.SettleEventsApplied(ctx) })
	}
	return group.Wait()
}
