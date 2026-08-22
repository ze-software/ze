// Design: docs/architecture/api/process-protocol.md — 5-stage plugin startup protocol
// Overview: server.go — Server struct and lifecycle
// Detail: startup_autoload.go — auto-loading plugins for families and event types
// Detail: startup_failure.go — the error reported when a plugin never reaches StageRunning

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var (
	errNoProcessspawnerSetCallSetprocessspawner = errors.New("no ProcessSpawner set — call SetProcessSpawner before Start")
	errSpawnerDidNotProduceAValid               = errors.New("spawner did not produce a valid ProcessManager")
	errFamilyDeclarationMissingName             = errors.New("family declaration missing name")

	startupRegistrationMu sync.Mutex
)

// Family mode constants (mirrored from root registration.go — unexported, not cross-package accessible).
const (
	familyModeDecode = "decode"
	familyModeBoth   = "both"
)

// stageTransition handles coordinator stage completion and waiting.
// Returns true if transition succeeded, false if failed (caller should return true to stop processing).
func (s *Server) stageTransition(proc *process.Process, pluginName string, completeStage, waitStage plugin.PluginStage) bool {
	s.coordinatorMu.Lock()
	coord := s.coordinator
	s.coordinatorMu.Unlock()
	if coord == nil {
		return true
	}

	logger().Debug("server: stageTransition START", "plugin", pluginName, "complete", completeStage, "wait_for", waitStage)
	logger().Debug("server: stageTransition calling StageComplete", "plugin", pluginName, "index", proc.Index())
	coord.StageComplete(proc.Index(), completeStage)
	logger().Debug("server: stageTransition StageComplete returned", "plugin", pluginName)

	// Use per-plugin timeout if configured, else env var, else default.
	// Priority: config > env > default.
	timeout := proc.Config().StageTimeout
	if timeout == 0 {
		timeout = stageTimeoutFromEnv()
	}

	// The timeout bounds a STALL, not the stage. It expires only when no
	// plugin in the tier completes a stage for that long.
	//
	// It used to be a flat wall-clock budget (stageStartTime + timeout) for
	// the whole stage, shared by every plugin in the tier. A tier is routinely
	// 20+ plugins (bgp plus every bgp-* plugin), and on a loaded host they all
	// slow down together: the shared budget expired while every plugin was
	// still handshaking normally, and the engine then stopped all of them
	// ("startup barrier aborted"), so a busy router dropped its own plugins.
	// Raising the constant only moves the load level at which that happens.
	//
	// Still bounded: progress events are finite (a plugin completes each stage
	// once; repeats are ignored), so a wedged tier trips the barrier within
	// `timeout` of the last progress, and s.ctx ends the wait on shutdown.
	err := coord.WaitForStageProgress(s.ctx, waitStage, timeout)

	if err != nil {
		logger().Error("stage stalled", "plugin", pluginName, "waiting_for", waitStage, "error", err)
		coord.PluginFailed(proc.Index(), fmt.Sprintf("stage timeout: %v", err))
		proc.Stop()
		return false
	}
	return true
}

// runPluginStartup handles five-phase plugin startup:
// Phase 1: Start explicit plugins, wait for registration.
// Phase 2: Auto-load plugins for config paths (e.g., fib { kernel {} } triggers fib-kernel).
// Phase 3: Auto-load plugins for unclaimed families.
// Phase 4: Auto-load plugins for custom event types (e.g., update-rpki triggers bgp-rpki-decorator).
// Phase 5: Auto-load plugins for custom send types (e.g., enhanced-refresh triggers bgp-route-refresh).
func (s *Server) runPluginStartup() {
	defer s.wg.Done()

	// Phase 1: Auto-load plugins for config paths (e.g., bgp, interface, fib).
	// Config-path plugins run first because they establish infrastructure
	// (like the BGP reactor) that explicit and family plugins depend on.
	autoLoadConfigPaths := s.getConfigPathPlugins()
	if len(autoLoadConfigPaths) > 0 {
		logger().Debug("auto-loading plugins for config paths",
			"count", len(autoLoadConfigPaths))

		if s.reactor != nil {
			s.reactor.AddAPIProcessCount(len(autoLoadConfigPaths))
		}

		if err := s.runPluginPhase(autoLoadConfigPaths); err != nil {
			logger().Error("auto-load config path plugin startup failed", "error", err)
			if s.reactor != nil {
				s.reactor.AddAPIProcessCount(-len(autoLoadConfigPaths))
			}
			s.startupErr = fmt.Errorf("config-path plugin startup failed: %w", err)
			s.signalStartupComplete()
			return
		}
	}

	// Phase 2: Explicit plugins (from config plugin { external ... } section).
	// These run after config-path plugins so infrastructure is available.
	if len(s.config.Plugins) > 0 {
		logger().Debug("starting explicit plugins", "count", len(s.config.Plugins))
		if err := s.runPluginPhase(s.config.Plugins); err != nil {
			logger().Error("explicit plugin startup failed", "error", err)
			s.signalStartupComplete()
			return
		}
	}

	// Phase 3: Auto-load plugins for unclaimed families
	// Now registry has families from explicit plugins - use family-based check
	autoLoadFamilies := s.getUnclaimedFamilyPlugins()
	if len(autoLoadFamilies) > 0 {
		logger().Debug("auto-loading plugins for unclaimed families",
			"count", len(autoLoadFamilies))

		if s.reactor != nil {
			s.reactor.AddAPIProcessCount(len(autoLoadFamilies))
		}

		if err := s.runPluginPhase(autoLoadFamilies); err != nil {
			logger().Error("auto-load family plugin startup failed", "error", err)
			if s.reactor != nil {
				s.reactor.AddAPIProcessCount(-len(autoLoadFamilies))
			}
			s.signalStartupComplete()
			return
		}
	}

	// Phase 4: Auto-load plugins for custom event types
	// Config has receive [ update-rpki ] but no explicit decorator plugin configured.
	autoLoadEvents := s.getUnclaimedEventTypePlugins()
	if len(autoLoadEvents) > 0 {
		logger().Debug("auto-loading plugins for custom event types",
			"count", len(autoLoadEvents))

		if s.reactor != nil {
			s.reactor.AddAPIProcessCount(len(autoLoadEvents))
		}

		if err := s.runPluginPhase(autoLoadEvents); err != nil {
			logger().Error("auto-load event plugin startup failed", "error", err)
			if s.reactor != nil {
				s.reactor.AddAPIProcessCount(-len(autoLoadEvents))
			}
			s.signalStartupComplete()
			return
		}
	}

	// Phase 5: Auto-load plugins for custom send types
	// Config has send [ enhanced-refresh ] but no explicit route-refresh plugin configured.
	autoLoadSendTypes := s.getUnclaimedSendTypePlugins()
	if len(autoLoadSendTypes) > 0 {
		logger().Debug("auto-loading plugins for custom send types",
			"count", len(autoLoadSendTypes))

		if s.reactor != nil {
			s.reactor.AddAPIProcessCount(len(autoLoadSendTypes))
		}

		if err := s.runPluginPhase(autoLoadSendTypes); err != nil {
			logger().Error("auto-load send type plugin startup failed", "error", err)
			if s.reactor != nil {
				s.reactor.AddAPIProcessCount(-len(autoLoadSendTypes))
			}
			s.signalStartupComplete()
			return
		}
	}

	// Signal that all plugin phases are complete
	s.signalStartupComplete()
}

// signalStartupComplete freezes registries for lock-free dispatch and
// notifies reactor that plugin startup is done.
func (s *Server) signalStartupComplete() {
	// Freeze registries: all registrations are complete, no writers after this point.
	if s.dispatcher != nil {
		if sm := s.dispatcher.Subsystems(); sm != nil {
			sm.Freeze()
		}
		if cr := s.dispatcher.Registry(); cr != nil {
			cr.Freeze()
		}
	}

	// Every phase has settled and failed plugins are rolled back, so this is the
	// first point with complete knowledge of which plugins are running -- and it
	// is still before StartPeers. Check that every exclusive role advertised at
	// Stage 2 has a live claimant; a plugin that stood down for a claimant which
	// never came up would silently perform nothing (startup_claims.go).
	s.verifyAdvertisedClaims()

	// Fan out the post-startup callback to every running plugin so that
	// OnAllPluginsReady handlers (e.g., bgp-rpki enabling the adj-rib-in
	// validation gate) can safely dispatch cross-plugin commands -- at this
	// point the dispatcher command registry is frozen and guaranteed to hold
	// every registered command from every startup phase. Best-effort.
	//
	// Nothing that must be in place before the first peer-up may be decided
	// here: this fan-out is not ordered against StartPeers below. Such state is
	// declared instead and delivered on the Stage-2 configure callback -- see
	// sendPostStartupToAll's doc comment and startup_claims.go.
	s.sendPostStartupToAll()

	if s.reactor != nil {
		s.reactor.SignalPluginStartupComplete()
	}
	s.startupDoneOnce.Do(func() { close(s.startupDone) })
}

// sendPostStartupToAll delivers the post-startup callback to every running
// plugin. Each delivery runs in its own goroutine with a bounded timeout, so
// one slow or broken plugin cannot delay notification to the rest. Errors are
// logged at Debug level because they are expected during shutdown races
// (connection closed before callback arrives).
//
// It deliberately does NOT wait. Waiting was tried (2026-07-25) to make
// OnAllPluginsReady handlers ordered before peer startup, so that a handler
// configuring how peer-up is processed could not lose a race against session
// establishment. It DEADLOCKS: this function is called immediately before
// SignalPluginStartupComplete -> StartPeers, and a handler that waits on peer
// activity (a test observer waiting for routes to reach Adj-RIB-In, for
// instance) then blocks the very peers it is waiting for, until the
// postStartupTimeout fires. Three functional tests failed that way.
//
// So the ordering between a post-startup handler and peer startup is NOT
// guaranteed, and anything that needs to be in place before the first peer-up
// must not rely on this callback. That state is DECLARED instead: a plugin puts
// it in its registration (registry.Registration.Claims) and the engine delivers
// the resolved set on the Stage-2 configure callback, which is part of the
// sequential handshake and therefore completes before peers start. See
// startup_claims.go. Do not move such a decision back onto this fan-out.
func (s *Server) sendPostStartupToAll() {
	pm := s.procManager.Load()
	if pm == nil {
		return
	}
	for _, proc := range pm.AllProcesses() {
		if proc == nil || !proc.Running() {
			continue
		}
		conn := proc.Conn()
		if conn == nil {
			continue
		}
		name := proc.Name()
		go func(c *plugipc.PluginConn, pluginName string) {
			ctx, cancel := context.WithTimeout(s.ctx, postStartupTimeout)
			defer cancel()
			if err := c.SendPostStartup(ctx); err != nil {
				logger().Debug("post-startup callback failed", "plugin", pluginName, "error", err)
			}
		}(conn, name)
	}
}

// postStartupTimeout bounds the wait for a plugin to process the post-startup
// callback. Plugins with expensive OnAllPluginsReady handlers (e.g., those
// that issue a DispatchCommand to another plugin) must complete within this
// window or the callback is abandoned; this prevents one slow handler from
// blocking engine bookkeeping.
const postStartupTimeout = 10 * time.Second

// WaitForStartupComplete blocks until all plugin startup phases are done.
// Returns a non-nil error if a config-path plugin failed during startup
// (e.g., invalid BGP config) or if the context deadline is exceeded.
func (s *Server) WaitForStartupComplete(ctx context.Context) error {
	select {
	case <-s.startupDone:
		return s.startupErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runPluginPhase starts a batch of plugins with tier-ordered handshake.
//
// All processes are started at once (single ProcessManager), but the 5-stage
// handshake is sequenced by dependency tiers. Tier 0 (no dependencies) completes
// fully — including command registration — before tier 1 begins, ensuring that
// dependent plugins can dispatch commands to their dependencies immediately.
//
// Async handlers (handleSingleProcessCommandsRPC) start only after ALL tiers
// complete, because they read from the same connections used during startup.
func (s *Server) runPluginPhase(plugins []plugin.PluginConfig) error {
	if len(plugins) == 0 {
		return nil
	}

	// Step (a): Spawn processes via PluginManager (ProcessSpawner).
	if s.spawner == nil {
		return errNoProcessspawnerSetCallSetprocessspawner
	}
	if err := s.spawner.SpawnMore(plugins); err != nil {
		return err
	}
	pm, ok := s.spawner.GetProcessManager().(*process.ProcessManager)
	if !ok || pm == nil {
		return errSpawnerDidNotProduceAValid
	}
	s.procManager.Store(pm)

	// Track loaded plugins so later phases don't re-load them.
	for _, p := range plugins {
		s.markPluginLoaded(p.Name)
	}

	// Step (b): Compute dependency tiers from plugin configs.
	names := make([]string, len(plugins))
	for i, p := range plugins {
		names[i] = p.Name
	}
	tiers, err := registry.TopologicalTiers(names)
	if err != nil {
		logger().Error("tier computation failed", "error", err)
		pm.Stop()
		return err
	}

	logger().Debug("plugin startup tiers computed", "tiers", tiers)

	var allProcesses []*process.Process
	var phaseErr error

	// Step (c): For each tier, create a coordinator and run the 5-stage handshake.
	for tierIdx, tierNames := range tiers {
		tierProcs := make([]*process.Process, 0, len(tierNames))
		for _, name := range tierNames {
			proc := pm.GetProcess(name)
			if proc == nil {
				logger().Error("tier process not found in PM", "plugin", name, "tier", tierIdx)
				continue
			}
			tierProcs = append(tierProcs, proc)
		}

		if len(tierProcs) == 0 {
			continue
		}

		for i, proc := range tierProcs {
			proc.SetIndex(i)
		}

		newCoord := plugin.NewStartupCoordinator(len(tierProcs))
		newCoord.SetStartTime(time.Now())
		s.coordinatorMu.Lock()
		s.coordinator = newCoord
		s.coordinatorMu.Unlock()

		logger().Debug("starting tier handshake", "tier", tierIdx, "plugins", tierNames)

		var procWg sync.WaitGroup
		for _, proc := range tierProcs {
			procWg.Add(1)
			go func(p *process.Process) {
				defer procWg.Done()
				s.handleProcessStartupRPC(p)
			}(proc)
		}
		procWg.Wait()

		allProcesses = append(allProcesses, tierProcs...)
		for _, proc := range tierProcs {
			if proc.Stage() >= plugin.StageRunning {
				continue
			}
			err := startupFailureError(proc)
			phaseErr = preferDiagnosedError(phaseErr, err)
			logger().Error("plugin startup failed", "plugin", proc.Name(), "stage", proc.Stage(), "error", err)
			s.rollbackStartupProcess(proc)
		}

		logger().Debug("tier handshake complete", "tier", tierIdx)
		if phaseErr != nil {
			break
		}
	}

	if phaseErr != nil {
		s.rollbackNonRunningStartupProcesses(pm, plugins)
	}

	s.coordinatorMu.Lock()
	s.coordinator = nil
	s.coordinatorMu.Unlock()

	// Step (d): Start async handlers only for processes that committed.
	for _, proc := range allProcesses {
		if proc.Stage() < plugin.StageRunning {
			continue
		}
		s.wg.Add(1)
		go func(p *process.Process) {
			defer s.wg.Done()
			s.handleSingleProcessCommandsRPC(p)
		}(proc)
	}

	return phaseErr
}

func (s *Server) unmarkPluginLoaded(name string) {
	s.loadedPluginsMu.Lock()
	delete(s.loadedPlugins, name)
	s.loadedPluginsMu.Unlock()
}

func (s *Server) rememberPluginFamilies(name string, regs []family.FamilyRegistration) {
	if len(regs) == 0 {
		return
	}
	s.runtimeFamiliesMu.Lock()
	if s.runtimeFamilies == nil {
		s.runtimeFamilies = make(map[string][]family.FamilyRegistration)
	}
	s.runtimeFamilies[name] = append(s.runtimeFamilies[name], regs...)
	s.runtimeFamiliesMu.Unlock()
}

func (s *Server) removePluginFamilies(name string) {
	s.runtimeFamiliesMu.Lock()
	regs := append([]family.FamilyRegistration(nil), s.runtimeFamilies[name]...)
	delete(s.runtimeFamilies, name)
	s.runtimeFamiliesMu.Unlock()
	family.UnregisterFamilyBatch(regs)
}

func (s *Server) rollbackStartupProcess(proc *process.Process) {
	proc.Stop()
	if proc.Stage() >= plugin.StageRunning {
		proc.WaitRuntimeCleanup()
	} else {
		s.cleanupProcess(proc)
	}

	if s.registry != nil {
		s.registry.Unregister(proc.Name())
	}
	if s.capInjector != nil {
		s.capInjector.RemovePluginCapabilities(proc.Name())
	}
	s.removePluginFamilies(proc.Name())
	// The pipe aliases leave with the plugin, on both paths that reach here: a
	// startup that failed at any stage, and a running plugin the operator
	// removed from the config. A name that outlived its plugin answers a
	// command nobody serves, and it refuses that plugin its own name when it
	// starts again.
	command.UnregisterPluginAliases(proc.Name())
	if pm := s.procManager.Load(); pm != nil {
		pm.RemoveProcess(proc.Name())
	}
	s.unmarkPluginLoaded(proc.Name())
}

func (s *Server) rollbackNonRunningStartupProcesses(pm *process.ProcessManager, plugins []plugin.PluginConfig) {
	for _, cfg := range plugins {
		proc := pm.GetProcess(cfg.Name)
		if proc == nil {
			s.unmarkPluginLoaded(cfg.Name)
			continue
		}
		if proc.Stage() >= plugin.StageRunning {
			continue
		}
		logger().Error("plugin startup failed before tier handshake completed",
			"plugin", proc.Name(), "stage", proc.Stage())
		s.rollbackStartupProcess(proc)
	}
}

// handleProcessStartupRPC runs the shared 5-stage startup handshake for a
// tier-managed engine plugin. It initializes the process connection, then drives
// runStartupHandshake with an engineStartupSink that performs the full engine
// registration set and synchronizes the tier through the StartupCoordinator
// barrier. Startup completion is reported through proc.Stage() (set by the
// sink's Transition), not the returned error -- which is why the driver's error
// is only logged here and the outer runPluginPhase inspects the process stage.
func (s *Server) handleProcessStartupRPC(proc *process.Process) {
	proc.SetStage(plugin.StageRegistration)

	// Signal coordinator on early exit if startup didn't complete.
	// Without this, other plugins hang at WaitForStage until timeout.
	defer func() {
		if proc.Stage() < plugin.StageRunning {
			s.coordinatorMu.Lock()
			coord := s.coordinator
			s.coordinatorMu.Unlock()
			if coord != nil {
				coord.PluginFailed(proc.Index(), "startup incomplete")
			}
		}
	}()

	// Initialize connections from raw sockets (creates PluginConn wrappers).
	//
	// Every failure path below records its cause on the process as well as
	// logging it: startup completion is reported through proc.Stage(), which
	// says WHERE the handshake stopped but never WHY, and runPluginPhase turns
	// that stage into the error the operator sees. Without the recording, the
	// cause died here (the handshake one at Debug level, below the default WARN)
	// and ze exited with an unactionable "failed during startup at stage Config".
	if err := proc.InitConns(); err != nil {
		logger().Error("rpc startup: init connections failed", "plugin", proc.Name(), "error", err)
		proc.SetStartupError(fmt.Errorf("init connections: %w", err))
		return
	}
	if proc.Conn() == nil {
		logger().Debug("rpc startup: no connection (startup failed?)", "plugin", proc.Name())
		proc.SetStartupError(errStartupConnClosed)
		return
	}

	if err := runStartupHandshake(s.ctx, &engineStartupSink{s: s, proc: proc}); err != nil {
		logger().Debug("rpc startup: handshake ended before running", "plugin", proc.Name(), "error", err)
		proc.SetStartupError(err)
	}
}

// engineStartupSink adapts the engine Server to the shared startup driver. It
// performs the full set of engine-side registrations between stages and
// synchronizes concurrent plugins in a tier through the Server's
// StartupCoordinator barrier (a nil coordinator, as in ad-hoc sessions, makes
// every Transition an immediate success).
type engineStartupSink struct {
	s    *Server
	proc *process.Process
}

func (e *engineStartupSink) conn() *plugipc.PluginConn { return e.proc.Conn() }

// OnRegistration validates and applies the plugin's declare-registration:
// doctor-check and enricher declarations, cache-consumer state, dependency
// checks, the PluginRegistry row, declared runtime families (rolled back on
// conflict), and proxy enrichers. A returned error's text is the exact message
// the driver relays to the plugin; the failed process is torn down by
// rollbackStartupProcess after the tier handshake completes.
func (e *engineStartupSink) onRegistration(input *rpc.DeclareRegistrationInput) error {
	s, proc := e.s, e.proc

	// Validate doctor check declarations before conversion.
	if err := validateDoctorCheckDecls(input.DoctorChecks); err != nil {
		return fmt.Errorf("invalid doctor check: %w", err)
	}
	// Validate enricher declarations before proxy registration.
	if err := validateEnricherDecls(input.Enrichers); err != nil {
		return fmt.Errorf("invalid enricher: %w", err)
	}
	// Validate pipe alias declarations before conversion. The pipe resolver
	// reads the alias registry in this process. A declaration that reaches that
	// registry is live for every operator, so a refusal happens before it. The
	// commands travel with the pipes because an alias may sit only on a command
	// path this same message declares.
	if err := validatePipeDecls(input.Pipes, input.Commands); err != nil {
		return fmt.Errorf("invalid pipe alias: %w", err)
	}

	// Convert RPC input to engine registration type.
	reg := registrationFromRPC(input)
	reg.Name = proc.Config().Name
	proc.SetRegistration(reg)
	proc.SetCacheConsumer(input.CacheConsumer)
	if input.CacheConsumer && s.reactor != nil {
		s.reactor.RegisterCacheConsumer(proc.Name(), input.CacheConsumerUnordered)
	}

	// Validate declared dependencies against the configured plugin set.
	// Internal deps were auto-added by expandDependencies() in the config loader.
	// External deps must be explicitly configured by the operator.
	for _, dep := range input.Dependencies {
		if !s.hasConfiguredPlugin(dep) {
			logger().Error("rpc startup: dependency not configured", "plugin", proc.Name(), "dependency", dep)
			return fmt.Errorf("missing dependency: plugin %q requires %q", proc.Config().Name, dep)
		}
	}

	// Register the registry rows, then the pipe aliases, then the declared
	// runtime families. Each failure below unwinds what the ones above it
	// wrote, in the reverse order, so a refused declaration is invisible to
	// later startup phases. A failure in a LATER STAGE unwinds the same three
	// through rollbackStartupProcess.
	startupRegistrationMu.Lock()
	if err := s.registry.Register(reg); err != nil {
		startupRegistrationMu.Unlock()
		logger().Error("plugin registration conflict", "plugin", reg.Name, "error", err)
		return fmt.Errorf("registration conflict: %w", err)
	}
	if err := registerPluginPipes(reg.Name, input.Pipes, input.Commands); err != nil {
		s.registry.Unregister(reg.Name)
		startupRegistrationMu.Unlock()
		logger().Error("plugin pipe alias refused", "plugin", reg.Name, "error", err)
		return fmt.Errorf("pipe alias refused: %s: %w", reg.Name, err)
	}
	addedFamilies, err := registerPluginFamilies(input.Families)
	if err != nil {
		command.UnregisterPluginAliases(reg.Name)
		s.registry.Unregister(reg.Name)
		startupRegistrationMu.Unlock()
		logger().Error("plugin family registration conflict", "plugin", reg.Name, "error", err)
		return fmt.Errorf("family registration conflict: %w", err)
	}
	s.rememberPluginFamilies(reg.Name, addedFamilies)
	startupRegistrationMu.Unlock()

	// Register proxy enrichers for declared show enrichers.
	if len(input.Enrichers) > 0 {
		registerProxyEnrichers(proc.Config().Name, input.Enrichers, proc.Conn())
	}
	return nil
}

// DeliverConfig delivers the real config sections for the plugin's requested
// roots. A send failure is fatal to this plugin's startup (barrier signaled,
// process stopped, startupErr set for fatal-on-config plugins) and aborts the
// driver; see deliverConfigRPC.
func (e *engineStartupSink) deliverConfig(ctx context.Context) error {
	return e.s.deliverConfigRPC(ctx, e.proc)
}

// OnCapabilities converts and registers the plugin's declared capabilities into
// the capability injector. A conflict returns the exact plugin-facing message.
//
// Stage 3 records no answer shape. One answer has one encoding, so a command
// answer is a head, its records and a terminator whatever this plugin declared
// here (WriteRecordAnswer, pkg/plugin/rpc/answer_write.go).
func (e *engineStartupSink) onCapabilities(input *rpc.DeclareCapabilitiesInput) error {
	caps := capabilitiesFromRPC(input)
	caps.PluginName = e.proc.Config().Name
	e.proc.SetCapabilities(caps)
	if err := e.s.capInjector.AddPluginCapabilities(caps); err != nil {
		logger().Error("plugin capability conflict", "plugin", caps.PluginName, "error", err)
		return fmt.Errorf("capability conflict: %w", err)
	}
	return nil
}

// DeliverRegistry shares the command registry with the plugin. The engine treats
// a share-registry send failure as non-fatal (logged inside deliverRegistryRPC);
// startup continues to the Ready stage, so this never aborts the driver.
func (e *engineStartupSink) deliverRegistry(ctx context.Context) error {
	e.s.deliverRegistryRPC(ctx, e.proc)
	return nil
}

// OnReady registers startup subscriptions, wires bridge dispatch, and registers
// the plugin's commands with the dispatcher -- all before the Ready->Running
// barrier so every command is visible when the barrier releases.
func (e *engineStartupSink) onReady(input *rpc.ReadyInput) error {
	s, proc := e.s, e.proc

	// Registering subscriptions here (before SignalAPIReady) ensures the plugin
	// receives events from the very first route send -- no race with the reactor.
	if input.Subscribe != nil && s.subscriptions != nil {
		s.registerSubscriptions(proc, input.Subscribe)
		logger().Debug("rpc startup: registered startup subscriptions",
			"plugin", proc.Name(), "events", input.Subscribe.Events)
	}

	// Wire direct bridge dispatch BEFORE the OK is sent (in the driver), so the
	// engine's DispatchRPC handler is registered before the SDK calls SetReady()
	// -- preventing a race where the SDK takes the bridge path before the engine
	// handler is wired.
	s.wireBridgeDispatch(proc)

	// Register plugin commands with the dispatcher. Commands were declared in
	// Stage 1 (PluginRegistry) but the dispatcher's CommandRegistry (used by
	// dispatchPlugin) needs its own entries. Registering here -- before the
	// StageReady barrier -- ensures all plugin commands are visible by the time
	// the barrier releases and event loops can trigger inter-plugin dispatch
	// (e.g., bgp-rs dispatching "adj-rib-in replay").
	if reg := proc.Registration(); reg == nil {
		logger().Debug("no registration for plugin", "plugin", proc.Name())
	} else {
		logger().Debug("plugin registration", "plugin", proc.Name(), "commands", reg.Commands, "families", reg.Families)
	}
	if reg := proc.Registration(); reg != nil && len(reg.Commands) > 0 {
		defs := make([]CommandDef, len(reg.Commands))
		for i, name := range reg.Commands {
			defs[i] = CommandDef{Name: name, Description: reg.CommandDescriptions[name], Hidden: reg.CommandHidden[name], Completable: reg.CommandCompletable[name]}
		}
		results := s.dispatcher.Registry().Register(proc, defs)
		for _, r := range results {
			if !r.OK {
				logger().Warn("command registration rejected", "plugin", proc.Name(), "command", r.Name, "error", r.Error)
			} else {
				logger().Debug("command registered", "plugin", proc.Name(), "command", r.Name)
			}
		}
		for canonicalName, oldNames := range reg.CommandDeprecatedNames {
			for _, oldName := range oldNames {
				if err := s.dispatcher.Registry().registerDeprecated(proc, oldName, canonicalName); err != nil {
					logger().Warn("deprecated alias rejected", "plugin", proc.Name(), "old", oldName, "new", canonicalName, "error", err)
					continue
				}
				logger().Debug("deprecated alias registered", "plugin", proc.Name(), "old", oldName, "new", canonicalName)
			}
		}
	}
	return nil
}

// OnRunning signals the reactor that a plugin's API is ready, after the
// Ready->Running barrier and before the final OK.
func (e *engineStartupSink) onRunning() {
	if e.s.reactor != nil {
		e.s.reactor.SignalAPIReady()
	}
}

// PostReady switches the PluginConn to bridge transport when the plugin
// requested it, after the final OK (the last message on the pipe).
func (e *engineStartupSink) postReady(input *rpc.ReadyInput) {
	if input.Transport == "bridge" && e.proc.Bridge() != nil {
		e.proc.Conn().SetBridge(e.proc.Bridge())
		logger().Debug("rpc startup: switched to bridge transport", "plugin", e.proc.Name())
	}
}

// Transition advances the tier barrier from one stage to the next and records
// the new process stage. A nil coordinator (ad-hoc session) makes stageTransition
// an immediate success.
func (e *engineStartupSink) transition(from, to plugin.PluginStage) bool {
	if !e.s.stageTransition(e.proc, e.proc.Name(), from, to) {
		return false
	}
	e.proc.SetStage(to)
	return true
}

// deliverConfigRPC sends configuration to a plugin via RPC (Stage 2).
// Sends ze-plugin-callback:configure RPC to the plugin. Returns a non-nil error
// (aborting the shared driver) only when the send fails; a build-sections error
// is logged and delivery proceeds with whatever sections were built.
func (s *Server) deliverConfigRPC(ctx context.Context, proc *process.Process) error {
	reg := proc.Registration()
	conn := proc.Conn()
	if conn == nil {
		logger().Error("deliverConfigRPC: connection closed", "plugin", proc.Name())
		return errStartupConnClosed
	}

	var sections []rpc.ConfigSection

	if len(reg.WantsConfigRoots) > 0 && s.reactor != nil {
		configTree := s.reactor.GetConfigTree()
		if configTree != nil {
			var err error
			sections, err = config.BuildPluginConfigSections(configTree, reg.WantsConfigRoots)
			if err != nil {
				logger().Error("deliverConfigRPC: build config sections", "plugin", proc.Name(), "error", err)
			}
		}
	}

	// Exclusive roles other plugins have claimed. Delivered here, on Stage 2,
	// because the sequential handshake puts it before the plugin's Stage-5 ready
	// and therefore before the reactor starts peers -- see startup_claims.go.
	claims := s.advertiseClaims(proc.Name())
	if len(claims) > 0 {
		logger().Debug("delivering claimed exclusive roles at stage 2",
			"plugin", proc.Name(), "roles", claims)
	}

	if err := conn.SendConfigure(ctx, sections, claims); err != nil {
		logger().Error("deliverConfigRPC failed", "plugin", proc.Name(), "error", err)
		s.coordinatorMu.Lock()
		coord := s.coordinator
		s.coordinatorMu.Unlock()
		if coord != nil {
			var tb textbuf.Buffer
			coord.PluginFailed(proc.Index(), tb.Str("configure failed: ").Err(err).String())
		}
		proc.Stop()
		if registry.IsFatalOnConfigError(proc.Name()) {
			s.startupErr = fmt.Errorf("%s: %w", proc.Name(), err)
		}
		return err
	}
	return nil
}

// deliverRegistryRPC sends the command registry to a plugin via RPC (Stage 4).
// Sends ze-plugin-callback:share-registry RPC to the plugin. A send failure is
// logged but non-fatal: engine startup proceeds to the Ready stage.
func (s *Server) deliverRegistryRPC(ctx context.Context, proc *process.Process) {
	allCommands := s.registry.BuildCommandInfo()

	totalCmds := 0
	for _, cmds := range allCommands {
		totalCmds += len(cmds)
	}
	commands := make([]rpc.RegistryCommand, 0, totalCmds)
	for pluginName, cmds := range allCommands {
		for _, cmd := range cmds {
			commands = append(commands, rpc.RegistryCommand{
				Name:     cmd.Command,
				Plugin:   pluginName,
				Encoding: cmd.Encoding,
			})
		}
	}

	conn := proc.Conn()
	if conn == nil {
		logger().Error("deliverRegistryRPC: connection closed", "plugin", proc.Name())
		return
	}
	if err := conn.SendShareRegistry(ctx, commands); err != nil {
		logger().Error("deliverRegistryRPC failed", "plugin", proc.Name(), "error", err)
	}
}

// ExtractConfigSubtree extracts a subtree from the config based on path.
// Always returns data wrapped in its full path structure from root.
// Supports:
//   - "*" -> entire tree
//   - "bgp" -> {"bgp": configTree["bgp"]}
//   - "bgp/peer" -> {"bgp": {"peer": configTree["bgp"]["peer"]}}
func ExtractConfigSubtree(configTree map[string]any, path string) any {
	return config.ExtractConfigSubtree(configTree, path)
}

// registerPluginFamilies registers each declared family with the nlri registry.
// Called after a plugin completes its declare-registration RPC, so that
// Family.String() and LookupFamily() work for families introduced by external
// plugins at runtime.
//
// The plugin provides a canonical "afi/safi" name (e.g., "ipv4/flow"). When the
// plugin also supplies AFI/SAFI numbers (RFC 4760 wire-format values), the family
// is registered with the registry. When AFI/SAFI are unset (0), this means the
// plugin uses the older protocol that doesn't carry numeric AFI/SAFI; in that
// case, the family must already be registered (typically by an internal plugin's
// init) and this call is a no-op.
//
// Re-registration with identical values is a no-op. Conflicting AFI or SAFI
// names return an error, which propagates as a registration failure.
func registerPluginFamilies(families []rpc.FamilyDecl) ([]family.FamilyRegistration, error) {
	registrations := make([]family.FamilyRegistration, 0, len(families))
	for _, fam := range families {
		if fam.Name == "" {
			return nil, errFamilyDeclarationMissingName
		}
		parts := strings.SplitN(fam.Name, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid family name %q (expected afi/safi)", fam.Name)
		}
		// Older plugins don't send AFI/SAFI numbers. If both are unset, the
		// family must already be registered by an internal plugin's init -- skip.
		if fam.AFI == 0 && fam.SAFI == 0 {
			if _, ok := family.LookupFamily(fam.Name); ok {
				continue
			}
			return nil, fmt.Errorf("family %q: plugin sent AFI=0 SAFI=0 and family is not pre-registered", fam.Name)
		}
		registrations = append(registrations, family.FamilyRegistration{
			AFI: family.AFI(fam.AFI), SAFI: family.SAFI(fam.SAFI), AFIName: parts[0], SAFIName: parts[1],
		})
	}
	added, err := family.RegisterFamilyBatch(registrations)
	if err != nil {
		return nil, fmt.Errorf("family registration: %w", err)
	}
	return added, nil
}

// registrationFromRPC converts DeclareRegistrationInput (RPC types) to PluginRegistration (engine types).
func registrationFromRPC(input *rpc.DeclareRegistrationInput) *plugin.PluginRegistration {
	reg := &plugin.PluginRegistration{
		WantsConfigRoots:  input.WantsConfig,
		ConfigOperations:  input.ConfigOperations,
		VerifyBudget:      input.VerifyBudget,
		ApplyBudget:       input.ApplyBudget,
		WantsValidateOpen: input.WantsValidateOpen,
		Claims:            input.Claims,
		Done:              true,
	}

	for _, fam := range input.Families {
		switch fam.Mode {
		case familyModeBoth:
			reg.Families = append(reg.Families, fam.Name)
			reg.DecodeFamilies = append(reg.DecodeFamilies, fam.Name)
		case familyModeDecode:
			reg.DecodeFamilies = append(reg.DecodeFamilies, fam.Name)
		default: // "encode" or unspecified
			reg.Families = append(reg.Families, fam.Name)
		}
	}

	for _, cmd := range input.Commands {
		reg.Commands = append(reg.Commands, cmd.Name)
		if cmd.Description != "" {
			if reg.CommandDescriptions == nil {
				reg.CommandDescriptions = make(map[string]string, len(input.Commands))
			}
			reg.CommandDescriptions[cmd.Name] = cmd.Description
		}
		if cmd.Hidden {
			if reg.CommandHidden == nil {
				reg.CommandHidden = make(map[string]bool, len(input.Commands))
			}
			reg.CommandHidden[cmd.Name] = true
		}
		if cmd.Completable {
			if reg.CommandCompletable == nil {
				reg.CommandCompletable = make(map[string]bool, len(input.Commands))
			}
			reg.CommandCompletable[cmd.Name] = true
		}
		if len(cmd.DeprecatedNames) > 0 {
			if reg.CommandDeprecatedNames == nil {
				reg.CommandDeprecatedNames = make(map[string][]string, len(input.Commands))
			}
			reg.CommandDeprecatedNames[cmd.Name] = cmd.DeprecatedNames
		}
	}

	if input.Schema != nil {
		reg.PluginSchema = &plugin.PluginSchemaDecl{
			Module:    input.Schema.Module,
			Namespace: input.Schema.Namespace,
			Handlers:  input.Schema.Handlers,
			Yang:      input.Schema.YANGText,
		}
	}

	for _, f := range input.Filters {
		nlri := true
		if f.NLRI != nil {
			nlri = *f.NLRI
		}
		onError := rpc.OnErrorReject
		if f.OnError != rpc.OnErrorUnspecified {
			onError = f.OnError
		}
		reg.Filters = append(reg.Filters, plugin.FilterRegistration{
			Name:       f.Name,
			Direction:  f.Direction,
			Attributes: f.Attributes,
			NLRI:       nlri,
			Raw:        f.Raw,
			OnError:    onError,
			Overrides:  f.Overrides,
		})
	}

	for _, dc := range input.DoctorChecks {
		platforms := dc.Platforms
		if len(platforms) == 0 {
			platforms = []string{"any"}
		}
		reg.DoctorChecks = append(reg.DoctorChecks, plugin.DoctorCheckRegistration{
			Name:         dc.Name,
			Phase:        dc.Phase,
			Order:        dc.Order,
			Dependencies: dc.Dependencies,
			Platforms:    platforms,
			Codes:        dc.Codes,
		})
	}

	return reg
}

// capabilitiesFromRPC converts DeclareCapabilitiesInput (RPC types) to PluginCapabilities (engine types).
func capabilitiesFromRPC(input *rpc.DeclareCapabilitiesInput) *plugin.PluginCapabilities {
	caps := &plugin.PluginCapabilities{
		Done: true,
	}

	for _, cap := range input.Capabilities {
		caps.Capabilities = append(caps.Capabilities, plugin.PluginCapability{
			Code:     cap.Code,
			Encoding: cap.Encoding,
			Payload:  cap.Payload,
			Peers:    cap.Peers,
		})
	}

	return caps
}

// validateDoctorCheckDecls validates doctor check declarations from Stage 1 registration.
func validateDoctorCheckDecls(checks []rpc.DoctorCheckDecl) error {
	const maxChecks = 16
	const maxNameLen = 128
	const maxOrder = 9999
	const maxCodes = 16

	if len(checks) > maxChecks {
		return fmt.Errorf("too many doctor checks: %d (max %d)", len(checks), maxChecks)
	}
	seen := make(map[string]struct{}, len(checks))
	for _, dc := range checks {
		if dc.Name == "" || len(dc.Name) > maxNameLen {
			return fmt.Errorf("invalid doctor check name %q (must be 1-%d chars)", dc.Name, maxNameLen)
		}
		if !isLowerKebab(dc.Name) {
			return fmt.Errorf("invalid doctor check name %q (must be kebab-case)", dc.Name)
		}
		if _, exists := seen[dc.Name]; exists {
			return fmt.Errorf("duplicate doctor check name %q", dc.Name)
		}
		seen[dc.Name] = struct{}{}
		if !dc.Phase.Valid() {
			return fmt.Errorf("invalid doctor check phase %q for %q", dc.Phase, dc.Name)
		}
		if dc.Order < 0 || dc.Order > maxOrder {
			return fmt.Errorf("invalid doctor check order %d for %q (must be 0-%d)", dc.Order, dc.Name, maxOrder)
		}
		if len(dc.Codes) == 0 || len(dc.Codes) > maxCodes {
			return fmt.Errorf("invalid doctor check codes count %d for %q (must be 1-%d)", len(dc.Codes), dc.Name, maxCodes)
		}
		for _, code := range dc.Codes {
			if !strings.HasPrefix(code, "doctor-") {
				return fmt.Errorf("invalid doctor check code %q for %q (must start with \"doctor-\")", code, dc.Name)
			}
		}
	}
	return nil
}

// validatePipeDecls validates pipe alias declarations from Stage 1 registration.
// It reads the shape of each declaration, and it confirms the plugin declared
// the command path the alias sits on in the same message. Whether a NAME is
// already taken is decided by the alias registry, which is the only holder of
// that answer.
//
// An alias sits on a command PATH, and a path the plugin did not declare belongs
// to whoever did. The check reads the declared command names and refuses every
// path it does not find there, so a plugin that declares no command declares no
// alias either.
func validatePipeDecls(pipes []rpc.PipeDecl, commands []rpc.CommandDecl) error {
	const maxPipes = 32
	const maxNameLen = 64

	if len(pipes) > maxPipes {
		return fmt.Errorf("too many pipe aliases: %d (max %d)", len(pipes), maxPipes)
	}
	declared := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		if path := commandPathKey(c.Name); path != "" {
			declared[path] = struct{}{}
		}
	}
	for _, p := range pipes {
		if strings.TrimSpace(p.Command) == "" {
			return fmt.Errorf("pipe alias %q declares no command path", p.Name)
		}
		if p.Name == "" || len(p.Name) > maxNameLen {
			return fmt.Errorf("invalid pipe alias name %q (must be 1-%d chars)", p.Name, maxNameLen)
		}
		if !isLowerKebab(p.Name) {
			return fmt.Errorf("invalid pipe alias name %q (must be kebab-case)", p.Name)
		}
		if strings.TrimSpace(p.Expansion) == "" {
			return fmt.Errorf("pipe alias %q on %q expands to nothing", p.Name, p.Command)
		}
		if _, owned := declared[commandPathKey(p.Command)]; !owned {
			return fmt.Errorf("pipe alias %q sits on %q, a command this plugin does not declare", p.Name, p.Command)
		}
	}
	return nil
}

// commandPathKey is the one spelling of a command path the ownership check
// compares. It matches what the alias registry stores: lowercase, with one
// space between words.
//
// Two spellings of one path MUST answer the same here. A path that carries
// nothing but spaces answers the empty string, which no declared command can
// match, so it is refused rather than confirmed.
func commandPathKey(command string) string {
	return strings.ToLower(textbuf.Join(strings.Fields(command), " "))
}

// registerPluginPipes writes the declared pipe aliases into the alias registry
// the pipe resolver reads, under the plugin's name. The whole declaration is
// handed over in one call, because the registry refuses the batch rather than
// half of it.
//
// The plugin's own command names travel with it. The registry derives from them
// the empty declaration that stops an alias reaching a command below the one it
// sits on, so a declaring author never has to know how a command resolves an
// alias. The names are the same list validatePipeDecls read to decide the
// plugin owns the paths it named.
//
// The plugin name is what UnregisterPluginAliases takes the declaration back
// under, on the rollback path and when the plugin stops.
func registerPluginPipes(owner string, pipes []rpc.PipeDecl, commands []rpc.CommandDecl) error {
	if len(pipes) == 0 {
		return nil
	}
	declared := make([]command.PluginAlias, 0, len(pipes))
	for _, p := range pipes {
		declared = append(declared, command.PluginAlias{
			Command: p.Command,
			Alias: command.Alias{
				Name:        p.Name,
				Description: p.Description,
				Expansion:   p.Expansion,
			},
		})
	}
	declaredCommands := make([]string, 0, len(commands))
	for _, c := range commands {
		declaredCommands = append(declaredCommands, c.Name)
	}
	return command.RegisterPluginAliases(owner, declaredCommands, declared)
}

// isLowerKebab reports whether s is a valid lower-kebab-case identifier.
func isLowerKebab(s string) bool {
	if s == "" {
		return false
	}
	prevHyphen := true
	for i := range len(s) {
		c := s[i]
		if c == '-' {
			if prevHyphen {
				return false
			}
			prevHyphen = true
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			prevHyphen = false
			continue
		}
		return false
	}
	return !prevHyphen
}
