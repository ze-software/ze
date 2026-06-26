// Design: docs/architecture/api/process-protocol.md — 5-stage plugin startup protocol
// Overview: server.go — Server struct and lifecycle
// Detail: startup_autoload.go — auto-loading plugins for families and event types

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	plugipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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

	// Deadline is stageStartTime + timeout, not now + timeout.
	// This prevents fast plugins from timing out while waiting for slow
	// plugins at the barrier -- the timeout measures from when the stage
	// began, not from when this plugin reached the barrier.
	deadline := coord.StageStartTime().Add(timeout)
	stageCtx, cancel := context.WithDeadline(s.ctx, deadline)
	err := coord.WaitForStage(stageCtx, waitStage)
	cancel()

	if err != nil {
		logger().Error("stage timeout", "plugin", pluginName, "waiting_for", waitStage, "error", err)
		coord.PluginFailed(proc.Index(), fmt.Sprintf("stage timeout: %v", err))
		proc.Stop()
		return false
	}
	return true
}

// stageProgression defines a two-step stage transition with an intermediate delivery.
type stageProgression struct {
	from, mid, to plugin.PluginStage
	deliver       func(*process.Process)
}

// progressThroughStages handles the common pattern of two stage transitions with delivery between.
func (s *Server) progressThroughStages(proc *process.Process, name string, p stageProgression) {
	logger().Debug("server: progressThroughStages START", "plugin", name, "from", p.from, "mid", p.mid, "to", p.to)
	// First transition: from -> mid
	if !s.stageTransition(proc, name, p.from, p.mid) {
		logger().Debug("server: progressThroughStages FAILED first transition", "plugin", name)
		return
	}
	logger().Debug("server: progressThroughStages SetStage mid", "plugin", name, "mid", p.mid)
	proc.SetStage(p.mid)

	// Deliver content
	if p.deliver != nil {
		logger().Debug("server: progressThroughStages calling deliver", "plugin", name)
		p.deliver(proc)
		logger().Debug("server: progressThroughStages deliver done", "plugin", name)
	}

	// Second transition: mid -> to
	logger().Debug("server: progressThroughStages second transition START", "plugin", name)
	if !s.stageTransition(proc, name, p.mid, p.to) {
		logger().Debug("server: progressThroughStages FAILED second transition", "plugin", name)
		return
	}
	logger().Debug("server: progressThroughStages SetStage to", "plugin", name, "to", p.to)
	proc.SetStage(p.to)
	logger().Debug("server: progressThroughStages DONE", "plugin", name)
}

// handlePluginConflict logs and handles plugin registration conflicts.
func (s *Server) handlePluginConflict(proc *process.Process, name, msg string, err error) {
	if s.coordinator != nil {
		s.coordinator.PluginFailed(proc.Index(), err.Error())
	}
	logger().Error(msg, "plugin", name, "error", err)
	proc.Stop()
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

	// Fan out the post-startup callback to every running plugin so that
	// OnAllPluginsReady handlers (e.g., bgp-rpki enabling the adj-rib-in
	// validation gate) can safely dispatch cross-plugin commands -- at this
	// point the dispatcher command registry is frozen and guaranteed to hold
	// every registered command from every startup phase. Best-effort.
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
			err := fmt.Errorf("plugin %s failed during startup at stage %s", proc.Name(), proc.Stage())
			if phaseErr == nil {
				phaseErr = err
			}
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
	if s.registry != nil {
		s.registry.Unregister(proc.Name())
	}
	if s.capInjector != nil {
		s.capInjector.RemovePluginCapabilities(proc.Name())
	}
	s.removePluginFamilies(proc.Name())
	if s.dispatcher != nil {
		s.dispatcher.Registry().UnregisterAll(proc)
		s.dispatcher.Pending().CancelAll(proc)
	}
	if s.subscriptions != nil {
		s.subscriptions.ClearProcess(proc)
	}
	if proc.IsCacheConsumer() && s.reactor != nil {
		s.reactor.UnregisterCacheConsumer(proc.Name())
	}
	runProcessCleanupHooks(proc.Name())
	proc.Stop()
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

// handleProcessStartupRPC handles the 5-stage plugin startup via YANG RPC protocol.
// Reads plugin-initiated RPCs and sends engine-initiated callbacks over a single MuxConn.
// Returns when startup is complete (StageRunning) or on error.
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
	if err := proc.InitConns(); err != nil {
		logger().Error("rpc startup: init connections failed", "plugin", proc.Name(), "error", err)
		return
	}

	conn := proc.Conn()
	if conn == nil {
		logger().Debug("rpc startup: no connection (startup failed?)", "plugin", proc.Name())
		return
	}

	// Stage 1: Read declare-registration from plugin (plugin-initiated)
	req, err := conn.ReadRequest(s.ctx)
	if err != nil {
		logger().Error("rpc startup: read registration failed", "plugin", proc.Name(), "error", err)
		return
	}
	if req.Method != "ze-plugin-engine:declare-registration" {
		if err := conn.SendError(s.ctx, req.ID, "expected declare-registration, got "+req.Method); err != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", err)
		}
		return
	}

	var regInput rpc.DeclareRegistrationInput
	if err := json.Unmarshal(req.Params, &regInput); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "invalid registration: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	// Validate doctor check declarations before conversion.
	if err := validateDoctorCheckDecls(regInput.DoctorChecks); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "invalid doctor check: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	// Validate enricher declarations before proxy registration.
	if err := validateEnricherDecls(regInput.Enrichers); err != nil {
		var tb textbuf.Buffer
		if sendErr := conn.SendError(s.ctx, req.ID, tb.Str("invalid enricher: ").Err(err).String()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	// Convert RPC input to engine registration type
	reg := registrationFromRPC(&regInput)
	reg.Name = proc.Config().Name
	proc.SetRegistration(reg)
	proc.SetCacheConsumer(regInput.CacheConsumer)
	if regInput.CacheConsumer && s.reactor != nil {
		s.reactor.RegisterCacheConsumer(proc.Name(), regInput.CacheConsumerUnordered)
	}

	// Validate declared dependencies against configured plugin set.
	// Internal deps were auto-added by expandDependencies() in the config loader.
	// External deps must be explicitly configured by the operator.
	for _, dep := range regInput.Dependencies {
		if !s.hasConfiguredPlugin(dep) {
			errMsg := fmt.Sprintf("missing dependency: plugin %q requires %q", proc.Config().Name, dep)
			if sendErr := conn.SendError(s.ctx, req.ID, errMsg); sendErr != nil {
				logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
			}
			logger().Error("rpc startup: dependency not configured", "plugin", proc.Name(), "dependency", dep)
			return
		}
	}

	// Register with registry, then register declared runtime families. If
	// family registration fails, roll back the plugin registry rows so the
	// failed declaration is invisible to later startup phases.
	startupRegistrationMu.Lock()
	if err := s.registry.Register(reg); err != nil {
		startupRegistrationMu.Unlock()
		if sendErr := conn.SendError(s.ctx, req.ID, "registration conflict: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		s.handlePluginConflict(proc, reg.Name, "plugin registration conflict", err)
		return
	}
	addedFamilies, err := registerPluginFamilies(regInput.Families)
	if err != nil {
		s.registry.Unregister(reg.Name)
		startupRegistrationMu.Unlock()
		if sendErr := conn.SendError(s.ctx, req.ID, "family registration conflict: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		s.handlePluginConflict(proc, reg.Name, "plugin family registration conflict", err)
		return
	}
	s.rememberPluginFamilies(reg.Name, addedFamilies)
	startupRegistrationMu.Unlock()

	// Register proxy enrichers for declared show enrichers.
	if len(regInput.Enrichers) > 0 {
		registerProxyEnrichers(proc.Config().Name, regInput.Enrichers, conn)
	}

	// Send OK response
	if err := conn.SendResult(s.ctx, req.ID, nil); err != nil {
		return
	}

	// Progress: Registration -> Config (deliver config) -> Capability
	s.progressThroughStages(proc, reg.Name, stageProgression{
		from: plugin.StageRegistration, mid: plugin.StageConfig, to: plugin.StageCapability,
		deliver: func(p *process.Process) { s.deliverConfigRPC(p) },
	})

	if proc.Stage() < plugin.StageCapability {
		return // Stage transition failed
	}

	// Stage 3: Read declare-capabilities from plugin (plugin-initiated)
	req, err = conn.ReadRequest(s.ctx)
	if err != nil {
		logger().Error("rpc startup: read capabilities failed", "plugin", proc.Name(), "error", err)
		return
	}
	if req.Method != "ze-plugin-engine:declare-capabilities" {
		if err := conn.SendError(s.ctx, req.ID, "expected declare-capabilities, got "+req.Method); err != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", err)
		}
		return
	}

	var capsInput rpc.DeclareCapabilitiesInput
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &capsInput); err != nil {
			if sendErr := conn.SendError(s.ctx, req.ID, "invalid capabilities: "+err.Error()); sendErr != nil {
				logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
			}
			return
		}
	}

	// Convert and register capabilities
	caps := capabilitiesFromRPC(&capsInput)
	caps.PluginName = proc.Config().Name
	proc.SetCapabilities(caps)

	if err := s.capInjector.AddPluginCapabilities(caps); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "capability conflict: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		s.handlePluginConflict(proc, caps.PluginName, "plugin capability conflict", err)
		return
	}

	// Send OK response
	if err := conn.SendResult(s.ctx, req.ID, nil); err != nil {
		return
	}

	// Progress: Capability -> Registry (deliver registry) -> Ready
	s.progressThroughStages(proc, caps.PluginName, stageProgression{
		from: plugin.StageCapability, mid: plugin.StageRegistry, to: plugin.StageReady,
		deliver: func(p *process.Process) { s.deliverRegistryRPC(p) },
	})

	if proc.Stage() < plugin.StageReady {
		return // Stage transition failed
	}

	// Stage 5: Read ready from plugin (plugin-initiated)
	req, err = conn.ReadRequest(s.ctx)
	if err != nil {
		logger().Error("rpc startup: read ready failed", "plugin", proc.Name(), "error", err)
		return
	}
	if req.Method != "ze-plugin-engine:ready" {
		if err := conn.SendError(s.ctx, req.ID, "expected ready, got "+req.Method); err != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", err)
		}
		return
	}

	// Parse optional startup subscriptions from "ready" params.
	// Registering subscriptions here (before SignalAPIReady) ensures the plugin
	// receives events from the very first route send -- no race with the reactor.
	var readyInput rpc.ReadyInput
	if req.Params != nil {
		if parseErr := json.Unmarshal(req.Params, &readyInput); parseErr != nil {
			logger().Warn("rpc startup: invalid ready params", "plugin", proc.Name(), "error", parseErr)
		}
	}

	if readyInput.Subscribe != nil && s.subscriptions != nil {
		s.registerSubscriptions(proc, readyInput.Subscribe)
		logger().Debug("rpc startup: registered startup subscriptions",
			"plugin", proc.Name(), "events", readyInput.Subscribe.Events)
	}

	// Wire direct bridge dispatch BEFORE sending OK, so the engine's
	// DispatchRPC handler is registered before the SDK calls SetReady().
	// This prevents a race where the SDK takes the bridge path before
	// the engine handler is wired.
	s.wireBridgeDispatch(proc)

	// Register plugin commands with the dispatcher BEFORE sending the ready OK.
	// Commands were declared in Stage 1 (PluginRegistry) but the dispatcher's
	// CommandRegistry (used by dispatchPlugin) needs its own entries.
	// Registering here — before the StageReady barrier — ensures all plugin
	// commands are visible by the time the barrier releases and event loops
	// can trigger inter-plugin dispatch (e.g., bgp-rs dispatching "adj-rib-in replay").
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
				if err := s.dispatcher.Registry().RegisterDeprecated(proc, oldName, canonicalName); err != nil {
					logger().Warn("deprecated alias rejected", "plugin", proc.Name(), "old", oldName, "new", canonicalName, "error", err)
					continue
				}
				logger().Debug("deprecated alias registered", "plugin", proc.Name(), "old", oldName, "new", canonicalName)
			}
		}
	}

	// Final stage transition: Ready -> Running
	// Move the barrier BEFORE the OK response below. This ensures all plugins
	// in the tier have registered their commands and reached StageReady
	// before any of them receive OK and start their runtime event loop.
	if !s.stageTransition(proc, proc.Name(), plugin.StageReady, plugin.StageRunning) {
		return
	}
	proc.SetStage(plugin.StageRunning)

	if s.reactor != nil {
		s.reactor.SignalAPIReady()
	}

	// Send OK response (last message on the pipe for bridge transport).
	if err := conn.SendResult(s.ctx, req.ID, nil); err != nil {
		return
	}

	// If plugin requested bridge transport, switch PluginConn to use bridge
	// for all future engine->plugin callbacks. The pipe is no longer used.
	if readyInput.Transport == "bridge" && proc.Bridge() != nil {
		conn.SetBridge(proc.Bridge())
		logger().Debug("rpc startup: switched to bridge transport", "plugin", proc.Name())
	}
}

// deliverConfigRPC sends configuration to a plugin via RPC (Stage 2).
// Sends ze-plugin-callback:configure RPC to the plugin.
func (s *Server) deliverConfigRPC(proc *process.Process) {
	reg := proc.Registration()
	conn := proc.Conn()
	if conn == nil {
		logger().Error("deliverConfigRPC: connection closed", "plugin", proc.Name())
		return
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

	if err := conn.SendConfigure(s.ctx, sections); err != nil {
		logger().Error("deliverConfigRPC failed", "plugin", proc.Name(), "error", err)
		s.coordinatorMu.Lock()
		coord := s.coordinator
		s.coordinatorMu.Unlock()
		if coord != nil {
			coord.PluginFailed(proc.Index(), fmt.Sprintf("configure failed: %v", err))
		}
		proc.Stop()
		if registry.IsFatalOnConfigError(proc.Name()) {
			s.startupErr = fmt.Errorf("%s: %w", proc.Name(), err)
		}
	}
}

// deliverRegistryRPC sends the command registry to a plugin via RPC (Stage 4).
// Sends ze-plugin-callback:share-registry RPC to the plugin.
func (s *Server) deliverRegistryRPC(proc *process.Process) {
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
	if err := conn.SendShareRegistry(s.ctx, commands); err != nil {
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
