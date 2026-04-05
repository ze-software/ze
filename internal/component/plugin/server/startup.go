// Design: docs/architecture/api/process-protocol.md — 5-stage plugin startup protocol
// Overview: server.go — Server struct and lifecycle
// Detail: startup_autoload.go — auto-loading plugins for families and event types

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
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

	// Phase 1: Explicit plugins (from config plugin { external ... } section).
	if len(s.config.Plugins) > 0 {
		logger().Debug("starting explicit plugins", "count", len(s.config.Plugins))
		if err := s.runPluginPhase(s.config.Plugins); err != nil {
			logger().Error("explicit plugin startup failed", "error", err)
			s.signalStartupComplete()
			return
		}
	}

	// Phase 2: Auto-load plugins for config paths
	// Config has fib { kernel { } } but no explicit plugin declaration.
	autoLoadConfigPaths := s.getConfigPathPlugins()
	if len(autoLoadConfigPaths) > 0 {
		logger().Debug("auto-loading plugins for config paths",
			"count", len(autoLoadConfigPaths))

		if s.reactor != nil {
			s.reactor.AddAPIProcessCount(len(autoLoadConfigPaths))
		}

		if err := s.runPluginPhase(autoLoadConfigPaths); err != nil {
			logger().Error("auto-load config path plugin startup failed", "error", err)
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

	if s.reactor != nil {
		s.reactor.SignalPluginStartupComplete()
	}
	s.startupDoneOnce.Do(func() { close(s.startupDone) })
}

// WaitForStartupComplete blocks until all plugin startup phases are done.
// Returns immediately if no plugins were started.
func (s *Server) WaitForStartupComplete(ctx context.Context) error {
	select {
	case <-s.startupDone:
		return nil
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
		return fmt.Errorf("no ProcessSpawner set — call SetProcessSpawner before Start")
	}
	if err := s.spawner.SpawnMore(plugins); err != nil {
		return err
	}
	pm, ok := s.spawner.GetProcessManager().(*process.ProcessManager)
	if !ok || pm == nil {
		return fmt.Errorf("spawner did not produce a valid ProcessManager")
	}
	s.procManager.Store(pm)

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

	// Collect ALL processes for async handler startup after all tiers.
	var allProcesses []*process.Process

	// Step (c): For each tier, create a coordinator and run the 5-stage handshake.
	for tierIdx, tierNames := range tiers {
		// Build process slice for this tier by looking up names in PM.
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

		// Assign tier-local indices for coordinator barrier synchronization.
		for i, proc := range tierProcs {
			proc.SetIndex(i)
		}

		// Create coordinator for this tier.
		newCoord := plugin.NewStartupCoordinator(len(tierProcs))
		newCoord.SetStartTime(time.Now())
		s.coordinatorMu.Lock()
		s.coordinator = newCoord
		s.coordinatorMu.Unlock()

		logger().Debug("starting tier handshake", "tier", tierIdx, "plugins", tierNames)

		// Launch handshake goroutines for this tier's processes.
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

		logger().Debug("tier handshake complete", "tier", tierIdx)
	}

	// Step (d): After ALL tiers complete, start async handlers for ALL processes.
	// Tracked in wg so Server.Wait() blocks until all handlers exit.
	s.coordinatorMu.Lock()
	s.coordinator = nil
	s.coordinatorMu.Unlock()
	for _, proc := range allProcesses {
		s.wg.Add(1)
		go func(p *process.Process) {
			defer s.wg.Done()
			s.handleSingleProcessCommandsRPC(p)
		}(proc)
	}

	return nil
}

// handleProcessStartupRPC handles the 5-stage plugin startup via YANG RPC protocol.
// Reads plugin-initiated RPCs and sends engine-initiated callbacks over a single MuxConn.
// Returns when startup is complete (StageRunning) or on error.
func (s *Server) handleProcessStartupRPC(proc *process.Process) {
	proc.SetStage(plugin.StageRegistration)

	// Signal coordinator on early exit if startup didn't complete.
	// Without this, other plugins hang at WaitForStage until timeout.
	defer func() {
		if proc.Stage() < plugin.StageRunning && s.coordinator != nil {
			s.coordinator.PluginFailed(proc.Index(), "startup incomplete")
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

	// Register with registry
	if err := s.registry.Register(reg); err != nil {
		if sendErr := conn.SendError(s.ctx, req.ID, "registration conflict: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		s.handlePluginConflict(proc, reg.Name, "plugin registration conflict", err)
		return
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
			defs[i] = CommandDef{Name: name}
		}
		results := s.dispatcher.Registry().Register(proc, defs)
		for _, r := range results {
			if !r.OK {
				logger().Debug("command registration conflict", "plugin", proc.Name(), "command", r.Name, "error", r.Error)
			} else {
				logger().Debug("command registered", "plugin", proc.Name(), "command", r.Name)
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

	// Send OK response
	if err := conn.SendResult(s.ctx, req.ID, nil); err != nil {
		return
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
			for _, root := range reg.WantsConfigRoots {
				subtree := ExtractConfigSubtree(configTree, root)
				if subtree == nil {
					continue
				}
				jsonBytes, err := json.Marshal(subtree)
				if err != nil {
					logger().Error("deliverConfigRPC: marshal config subtree", "plugin", proc.Name(), "root", root, "error", err)
					continue
				}
				sections = append(sections, rpc.ConfigSection{Root: root, Data: string(jsonBytes)})
			}
		}
	}

	if err := conn.SendConfigure(s.ctx, sections); err != nil {
		logger().Error("deliverConfigRPC failed", "plugin", proc.Name(), "error", err)
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
	if path == "*" {
		return configTree
	}

	// Split path by "/" and filter empty parts
	rawParts := strings.Split(path, "/")
	var parts []string
	for _, p := range rawParts {
		if p != "" {
			parts = append(parts, p)
		}
	}

	if len(parts) == 0 {
		return configTree
	}

	// Navigate to the leaf data
	var current any = configTree
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
		if current == nil {
			return nil
		}
	}

	// Wrap the leaf data in its path structure (from leaf to root)
	result := current
	for i := len(parts) - 1; i >= 0; i-- {
		result = map[string]any{parts[i]: result}
	}
	return result
}

// registrationFromRPC converts DeclareRegistrationInput (RPC types) to PluginRegistration (engine types).
func registrationFromRPC(input *rpc.DeclareRegistrationInput) *plugin.PluginRegistration {
	reg := &plugin.PluginRegistration{
		WantsConfigRoots:  input.WantsConfig,
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
	}

	if input.Schema != nil {
		reg.PluginSchema = &plugin.PluginSchemaDecl{
			Module:    input.Schema.Module,
			Namespace: input.Schema.Namespace,
			Handlers:  input.Schema.Handlers,
			Yang:      input.Schema.YANGText,
		}
	}

	for _, ch := range input.ConnectionHandlers {
		reg.ConnectionHandlers = append(reg.ConnectionHandlers, plugin.ConnectionHandler{
			Type:    ch.Type,
			Port:    ch.Port,
			Address: ch.Address,
		})
	}

	for _, f := range input.Filters {
		nlri := true
		if f.NLRI != nil {
			nlri = *f.NLRI
		}
		onError := "reject"
		if f.OnError != "" {
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

// validHandoffPort reports whether port is in the valid range for connection handoff (1-65535).
