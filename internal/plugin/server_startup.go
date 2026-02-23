// Design: docs/architecture/api/process-protocol.md — 5-stage plugin startup protocol
// Related: server.go — Server struct and lifecycle

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// stageTransition handles coordinator stage completion and waiting.
// Returns true if transition succeeded, false if failed (caller should return true to stop processing).
func (s *Server) stageTransition(proc *Process, pluginName string, completeStage, waitStage PluginStage) bool {
	if s.coordinator == nil {
		return true
	}

	logger().Debug("server: stageTransition START", "plugin", pluginName, "complete", completeStage, "wait_for", waitStage)
	logger().Debug("server: stageTransition calling StageComplete", "plugin", pluginName, "index", proc.Index())
	s.coordinator.StageComplete(proc.Index(), completeStage)
	logger().Debug("server: stageTransition StageComplete returned", "plugin", pluginName)

	// Use per-plugin timeout if configured, else env var, else default.
	// Priority: config > env > default.
	timeout := proc.config.StageTimeout
	if timeout == 0 {
		timeout = stageTimeoutFromEnv()
	}

	// Deadline is stageStartTime + timeout, not now + timeout.
	// This prevents fast plugins from timing out while waiting for slow
	// plugins at the barrier -- the timeout measures from when the stage
	// began, not from when this plugin reached the barrier.
	deadline := s.coordinator.StageStartTime().Add(timeout)
	stageCtx, cancel := context.WithDeadline(s.ctx, deadline)
	err := s.coordinator.WaitForStage(stageCtx, waitStage)
	cancel()

	if err != nil {
		logger().Error("stage timeout", "plugin", pluginName, "waiting_for", waitStage, "error", err)
		s.coordinator.PluginFailed(proc.Index(), fmt.Sprintf("stage timeout: %v", err))
		proc.Stop()
		return false
	}
	return true
}

// stageProgression defines a two-step stage transition with an intermediate delivery.
type stageProgression struct {
	from, mid, to PluginStage
	deliver       func(*Process)
}

// progressThroughStages handles the common pattern of two stage transitions with delivery between.
func (s *Server) progressThroughStages(proc *Process, name string, p stageProgression) {
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
func (s *Server) handlePluginConflict(proc *Process, name, msg string, err error) {
	if s.coordinator != nil {
		s.coordinator.PluginFailed(proc.Index(), err.Error())
	}
	logger().Error(msg, "plugin", name, "error", err)
	proc.Stop()
}

// runPluginStartup handles two-phase plugin startup:
// Phase 1: Start explicit plugins, wait for registration
// Phase 2: Check unclaimed families, start auto-load plugins.
func (s *Server) runPluginStartup() {
	defer s.wg.Done()

	// Phase 1: Explicit plugins
	if len(s.config.Plugins) > 0 {
		logger().Debug("starting explicit plugins", "count", len(s.config.Plugins))
		if err := s.runPluginPhase(s.config.Plugins); err != nil {
			logger().Error("explicit plugin startup failed", "error", err)
			return
		}
	}

	// Phase 2: Auto-load plugins for unclaimed families
	// Now registry has families from explicit plugins - use family-based check
	autoLoadPlugins := s.getUnclaimedFamilyPlugins()
	if len(autoLoadPlugins) > 0 {
		logger().Debug("auto-loading plugins for unclaimed families",
			"count", len(autoLoadPlugins))

		// Tell reactor to wait for additional plugins
		if s.reactor != nil {
			s.reactor.AddAPIProcessCount(len(autoLoadPlugins))
		}

		if err := s.runPluginPhase(autoLoadPlugins); err != nil {
			logger().Error("auto-load plugin startup failed", "error", err)
			s.signalStartupComplete()
			return
		}
	}

	// Signal that all plugin phases are complete
	s.signalStartupComplete()
}

// signalStartupComplete notifies reactor that plugin startup is done.
func (s *Server) signalStartupComplete() {
	if s.reactor != nil {
		s.reactor.SignalPluginStartupComplete()
	}
}

// runPluginPhase starts a batch of plugins and waits for them to complete startup.
func (s *Server) runPluginPhase(plugins []PluginConfig) error {
	if len(plugins) == 0 {
		return nil
	}

	// Create coordinator for this phase
	s.coordinator = NewStartupCoordinator(len(plugins))

	// Create process manager for this phase
	pm := NewProcessManager(plugins)
	s.procManager = pm

	if err := pm.StartWithContext(s.ctx); err != nil {
		return err
	}

	// Set the Registration stage start time to NOW, after all processes are forked.
	// This ensures the timeout includes fork time, not time before processes exist.
	s.coordinator.SetStartTime(time.Now())

	// Handle commands synchronously (blocks until all plugins reach StageRunning)
	s.handleProcessCommandsSync(pm)

	return nil
}

// handleProcessCommandsSync handles commands from all processes and waits for completion.
// Blocks until all plugins reach StageRunning or context is canceled.
// After StageRunning, starts async handlers for continued operation.
func (s *Server) handleProcessCommandsSync(pm *ProcessManager) {
	// Get all processes from the manager
	pm.mu.RLock()
	processes := make([]*Process, 0, len(pm.processes))
	for _, p := range pm.processes {
		processes = append(processes, p)
	}
	pm.mu.RUnlock()

	// Start a goroutine to handle startup for each process via YANG RPC protocol.
	var procWg sync.WaitGroup
	for _, proc := range processes {
		procWg.Add(1)
		go func(p *Process) {
			defer procWg.Done()
			s.handleProcessStartupRPC(p)
		}(proc)
	}

	procWg.Wait()

	// After startup, start async handlers for continued operation.
	for _, proc := range processes {
		go s.handleSingleProcessCommandsRPC(proc)
	}
}

// getUnclaimedFamilyPlugins returns plugins to auto-load for configured families
// that are NOT claimed by any explicit plugin.
// Uses registry.LookupFamily for family-based detection (not name-based).
func (s *Server) getUnclaimedFamilyPlugins() []PluginConfig {
	seen := make(map[string]bool)
	var plugins []PluginConfig

	for _, family := range s.config.ConfiguredFamilies {
		// Family-based check: skip if already claimed by explicit plugin
		if s.registry.LookupFamily(family) != "" {
			logger().Debug("family already claimed, skipping auto-load",
				"family", family, "claimed_by", s.registry.LookupFamily(family))
			continue
		}

		// Get internal plugin for this family
		pluginName := GetPluginForFamily(family)
		if pluginName == "" {
			continue // No internal plugin for this family
		}

		// Avoid duplicates
		if seen[pluginName] {
			continue
		}
		seen[pluginName] = true

		logger().Debug("auto-loading plugin for unclaimed family",
			"plugin", pluginName, "family", family)

		plugins = append(plugins, PluginConfig{
			Name:     pluginName,
			Encoder:  "json",
			Internal: true,
		})
	}

	return plugins
}

// handleProcessStartupRPC handles the 5-stage plugin startup via YANG RPC protocol.
// Reads plugin->engine RPCs from engineConnA, sends engine->plugin callbacks via engineConnB.
// Returns when startup is complete (StageRunning) or on error.
func (s *Server) handleProcessStartupRPC(proc *Process) {
	proc.SetStage(StageRegistration)

	// Signal coordinator on early exit if startup didn't complete.
	// Without this, other plugins hang at WaitForStage until timeout.
	defer func() {
		if proc.Stage() < StageRunning && s.coordinator != nil {
			s.coordinator.PluginFailed(proc.Index(), "startup incomplete")
		}
	}()

	connA := proc.ConnA()
	if connA == nil {
		logger().Debug("rpc startup: no connection (startup failed?)", "plugin", proc.Name())
		return
	}

	// Stage 1: Read declare-registration from plugin (Socket A)
	req, err := connA.ReadRequest(s.ctx)
	if err != nil {
		logger().Error("rpc startup: read registration failed", "plugin", proc.Name(), "error", err)
		return
	}
	if req.Method != "ze-plugin-engine:declare-registration" {
		if err := connA.SendError(s.ctx, req.ID, "expected declare-registration, got "+req.Method); err != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", err)
		}
		return
	}

	var regInput rpc.DeclareRegistrationInput
	if err := json.Unmarshal(req.Params, &regInput); err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "invalid registration: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	// Convert RPC input to engine registration type
	reg := registrationFromRPC(&regInput)
	reg.Name = proc.config.Name
	proc.registration = reg
	proc.SetCacheConsumer(regInput.CacheConsumer)
	if regInput.CacheConsumer && s.reactor != nil {
		s.reactor.RegisterCacheConsumer(proc.Name(), regInput.CacheConsumerUnordered)
	}

	// Register with registry
	if err := s.registry.Register(reg); err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "registration conflict: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		s.handlePluginConflict(proc, reg.Name, "plugin registration conflict", err)
		return
	}

	// Send OK response
	if err := connA.SendResult(s.ctx, req.ID, nil); err != nil {
		return
	}

	// Progress: Registration -> Config (deliver config) -> Capability
	s.progressThroughStages(proc, reg.Name, stageProgression{
		from: StageRegistration, mid: StageConfig, to: StageCapability,
		deliver: func(p *Process) { s.deliverConfigRPC(p) },
	})

	if proc.Stage() < StageCapability {
		return // Stage transition failed
	}

	// Stage 3: Read declare-capabilities from plugin (Socket A)
	req, err = connA.ReadRequest(s.ctx)
	if err != nil {
		logger().Error("rpc startup: read capabilities failed", "plugin", proc.Name(), "error", err)
		return
	}
	if req.Method != "ze-plugin-engine:declare-capabilities" {
		if err := connA.SendError(s.ctx, req.ID, "expected declare-capabilities, got "+req.Method); err != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", err)
		}
		return
	}

	var capsInput rpc.DeclareCapabilitiesInput
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &capsInput); err != nil {
			if sendErr := connA.SendError(s.ctx, req.ID, "invalid capabilities: "+err.Error()); sendErr != nil {
				logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
			}
			return
		}
	}

	// Convert and register capabilities
	caps := capabilitiesFromRPC(&capsInput)
	caps.PluginName = proc.config.Name
	proc.capabilities = caps

	if err := s.capInjector.AddPluginCapabilities(caps); err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "capability conflict: "+err.Error()); sendErr != nil {
			logger().Debug("rpc startup: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		s.handlePluginConflict(proc, caps.PluginName, "plugin capability conflict", err)
		return
	}

	// Send OK response
	if err := connA.SendResult(s.ctx, req.ID, nil); err != nil {
		return
	}

	// Progress: Capability -> Registry (deliver registry) -> Ready
	s.progressThroughStages(proc, caps.PluginName, stageProgression{
		from: StageCapability, mid: StageRegistry, to: StageReady,
		deliver: func(p *Process) { s.deliverRegistryRPC(p) },
	})

	if proc.Stage() < StageReady {
		return // Stage transition failed
	}

	// Stage 5: Read ready from plugin (Socket A)
	req, err = connA.ReadRequest(s.ctx)
	if err != nil {
		logger().Error("rpc startup: read ready failed", "plugin", proc.Name(), "error", err)
		return
	}
	if req.Method != "ze-plugin-engine:ready" {
		if err := connA.SendError(s.ctx, req.ID, "expected ready, got "+req.Method); err != nil {
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

	// Send OK response
	if err := connA.SendResult(s.ctx, req.ID, nil); err != nil {
		return
	}

	// Final stage transition: Ready -> Running
	if !s.stageTransition(proc, proc.Name(), StageReady, StageRunning) {
		return
	}
	proc.SetStage(StageRunning)
	if s.reactor != nil {
		s.reactor.SignalAPIReady()
	}
}

// deliverConfigRPC sends configuration to a plugin via RPC (Stage 2).
// Uses engineConnB to send ze-plugin-callback:configure RPC.
func (s *Server) deliverConfigRPC(proc *Process) {
	reg := proc.Registration()
	connB := proc.ConnB()
	if connB == nil {
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

	if err := connB.SendConfigure(s.ctx, sections); err != nil {
		logger().Error("deliverConfigRPC failed", "plugin", proc.Name(), "error", err)
	}
}

// deliverRegistryRPC sends the command registry to a plugin via RPC (Stage 4).
// Uses engineConnB to send ze-plugin-callback:share-registry RPC.
func (s *Server) deliverRegistryRPC(proc *Process) {
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

	connB := proc.ConnB()
	if connB == nil {
		logger().Error("deliverRegistryRPC: connection closed", "plugin", proc.Name())
		return
	}
	if err := connB.SendShareRegistry(s.ctx, commands); err != nil {
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
func registrationFromRPC(input *rpc.DeclareRegistrationInput) *PluginRegistration {
	reg := &PluginRegistration{
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
		reg.PluginSchema = &PluginSchemaDecl{
			Module:    input.Schema.Module,
			Namespace: input.Schema.Namespace,
			Handlers:  input.Schema.Handlers,
			Yang:      input.Schema.YANGText,
		}
	}

	return reg
}

// capabilitiesFromRPC converts DeclareCapabilitiesInput (RPC types) to PluginCapabilities (engine types).
func capabilitiesFromRPC(input *rpc.DeclareCapabilitiesInput) *PluginCapabilities {
	caps := &PluginCapabilities{
		Done: true,
	}

	for _, cap := range input.Capabilities {
		caps.Capabilities = append(caps.Capabilities, PluginCapability{
			Code:     cap.Code,
			Encoding: cap.Encoding,
			Payload:  cap.Payload,
			Peers:    cap.Peers,
		})
	}

	return caps
}
