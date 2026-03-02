// Design: docs/architecture/api/process-protocol.md — text-mode 5-stage startup
// Overview: server_startup.go — JSON-mode handleProcessStartupRPC
// Related: subsystem_text.go — text-mode completeTextProtocol (subsystem path)

package server

import (
	"encoding/json"
	"fmt"

	plugin "codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/process"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// handleTextProcessStartup runs the 5-stage text handshake for an external process.
// This is the text-mode equivalent of the JSON path in handleProcessStartupRPC.
//
// Unlike completeTextProtocol() (subsystem path), this version interleaves
// coordinator barriers between stages so that multiple plugins synchronize
// their startup progress.
//
// Socket A (tcA): engine reads plugin stages 1, 3, 5 and sends "ok" responses.
// Socket B (tcB): engine sends stages 2, 4 and reads "ok" responses from plugin.
// After stage 5, tcB is assigned to the process for text event delivery.
func (s *Server) handleTextProcessStartup(proc *process.Process, tcA, tcB *rpc.TextConn) {
	name := proc.Name()

	// --- Stage 1: Read registration from Socket A ---
	regText, err := tcA.ReadMessage(s.ctx)
	if err != nil {
		logger().Error("text startup: read registration failed", "plugin", name, "error", err)
		return
	}
	regInput, err := rpc.ParseRegistrationText(regText)
	if err != nil {
		_ = tcA.WriteLine(s.ctx, "error "+err.Error()) //nolint:errcheck // best-effort error response
		logger().Error("text startup: parse registration failed", "plugin", name, "error", err)
		return
	}

	reg := registrationFromRPC(&regInput)
	reg.Name = proc.Config().Name
	proc.SetRegistration(reg)
	proc.SetCacheConsumer(regInput.CacheConsumer)
	if regInput.CacheConsumer && s.reactor != nil {
		s.reactor.RegisterCacheConsumer(proc.Name(), regInput.CacheConsumerUnordered)
	}

	// Validate declared dependencies.
	for _, dep := range regInput.Dependencies {
		if !s.hasConfiguredPlugin(dep) {
			errMsg := fmt.Sprintf("missing dependency: plugin %q requires %q", proc.Config().Name, dep)
			_ = tcA.WriteLine(s.ctx, "error "+errMsg) //nolint:errcheck // best-effort error response
			logger().Error("text startup: dependency not configured", "plugin", name, "dependency", dep)
			return
		}
	}

	if err := s.registry.Register(reg); err != nil {
		_ = tcA.WriteLine(s.ctx, "error registration conflict: "+err.Error()) //nolint:errcheck // best-effort
		s.handlePluginConflict(proc, reg.Name, "plugin registration conflict", err)
		return
	}

	if err := tcA.WriteLine(s.ctx, "ok"); err != nil {
		logger().Error("text startup: stage 1 respond failed", "plugin", name, "error", err)
		return
	}

	// --- Coordinator: Registration → Config → Capability ---
	s.progressThroughStages(proc, name, stageProgression{
		from: plugin.StageRegistration, mid: plugin.StageConfig, to: plugin.StageCapability,
		deliver: func(p *process.Process) { s.deliverConfigText(p, tcB) },
	})
	if proc.Stage() < plugin.StageCapability {
		return
	}

	// --- Stage 3: Read capabilities from Socket A ---
	capsText, err := tcA.ReadMessage(s.ctx)
	if err != nil {
		logger().Error("text startup: read capabilities failed", "plugin", name, "error", err)
		return
	}
	capsInput, err := rpc.ParseCapabilitiesText(capsText)
	if err != nil {
		_ = tcA.WriteLine(s.ctx, "error "+err.Error()) //nolint:errcheck // best-effort error response
		logger().Error("text startup: parse capabilities failed", "plugin", name, "error", err)
		return
	}

	caps := capabilitiesFromRPC(&capsInput)
	caps.PluginName = proc.Config().Name
	proc.SetCapabilities(caps)

	if err := s.capInjector.AddPluginCapabilities(caps); err != nil {
		_ = tcA.WriteLine(s.ctx, "error capability conflict: "+err.Error()) //nolint:errcheck // best-effort
		s.handlePluginConflict(proc, caps.PluginName, "plugin capability conflict", err)
		return
	}

	if err := tcA.WriteLine(s.ctx, "ok"); err != nil {
		logger().Error("text startup: stage 3 respond failed", "plugin", name, "error", err)
		return
	}

	// --- Coordinator: Capability → Registry → Ready ---
	s.progressThroughStages(proc, name, stageProgression{
		from: plugin.StageCapability, mid: plugin.StageRegistry, to: plugin.StageReady,
		deliver: func(p *process.Process) { s.deliverRegistryText(p, tcB) },
	})
	if proc.Stage() < plugin.StageReady {
		return
	}

	// --- Stage 5: Read ready from Socket A ---
	readyText, err := tcA.ReadMessage(s.ctx)
	if err != nil {
		logger().Error("text startup: read ready failed", "plugin", name, "error", err)
		return
	}
	readyInput, err := rpc.ParseReadyText(readyText)
	if err != nil {
		_ = tcA.WriteLine(s.ctx, "error "+err.Error()) //nolint:errcheck // best-effort error response
		logger().Error("text startup: parse ready failed", "plugin", name, "error", err)
		return
	}

	if readyInput.Subscribe != nil && s.subscriptions != nil {
		s.registerSubscriptions(proc, readyInput.Subscribe)
		logger().Debug("text startup: registered startup subscriptions",
			"plugin", name, "events", readyInput.Subscribe.Events)
	}

	// Register plugin commands with the dispatcher BEFORE sending the ready OK.
	// See server_startup.go for the race condition this prevents.
	if reg := proc.Registration(); reg != nil && len(reg.Commands) > 0 {
		defs := make([]CommandDef, len(reg.Commands))
		for i, cmdName := range reg.Commands {
			defs[i] = CommandDef{Name: cmdName}
		}
		results := s.dispatcher.Registry().Register(proc, defs)
		for _, r := range results {
			if !r.OK {
				logger().Debug("command registration conflict", "plugin", name, "command", r.Name, "error", r.Error)
			} else {
				logger().Debug("command registered", "plugin", name, "command", r.Name)
			}
		}
	}

	if err := tcA.WriteLine(s.ctx, "ok"); err != nil {
		logger().Error("text startup: stage 5 respond failed", "plugin", name, "error", err)
		return
	}

	// --- Final: Ready → Running ---
	if !s.stageTransition(proc, name, plugin.StageReady, plugin.StageRunning) {
		return
	}
	proc.SetStage(plugin.StageRunning)

	// Assign textConnB for event delivery (deliverBatch checks TextConnB()).
	proc.SetTextConnB(tcB)

	if s.reactor != nil {
		s.reactor.SignalAPIReady()
	}
}

// deliverConfigText sends configuration to a plugin via text protocol (Stage 2).
func (s *Server) deliverConfigText(proc *process.Process, tcB *rpc.TextConn) {
	reg := proc.Registration()

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
					logger().Error("deliverConfigText: marshal config subtree", "plugin", proc.Name(), "root", root, "error", err)
					continue
				}
				sections = append(sections, rpc.ConfigSection{Root: root, Data: string(jsonBytes)})
			}
		}
	}

	configText, err := rpc.FormatConfigureText(rpc.ConfigureInput{Sections: sections})
	if err != nil {
		logger().Error("deliverConfigText: format failed", "plugin", proc.Name(), "error", err)
		return
	}
	if err := tcB.WriteMessage(s.ctx, configText); err != nil {
		logger().Error("deliverConfigText: write failed", "plugin", proc.Name(), "error", err)
		return
	}
	resp, err := tcB.ReadLine(s.ctx)
	if err != nil {
		logger().Error("deliverConfigText: read response failed", "plugin", proc.Name(), "error", err)
		return
	}
	if resp != "ok" {
		logger().Error("deliverConfigText: plugin responded with error", "plugin", proc.Name(), "response", resp)
	}
}

// deliverRegistryText sends the command registry to a plugin via text protocol (Stage 4).
func (s *Server) deliverRegistryText(proc *process.Process, tcB *rpc.TextConn) {
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

	registryText, err := rpc.FormatRegistryText(rpc.ShareRegistryInput{Commands: commands})
	if err != nil {
		logger().Error("deliverRegistryText: format failed", "plugin", proc.Name(), "error", err)
		return
	}
	if err := tcB.WriteMessage(s.ctx, registryText); err != nil {
		logger().Error("deliverRegistryText: write failed", "plugin", proc.Name(), "error", err)
		return
	}
	resp, err := tcB.ReadLine(s.ctx)
	if err != nil {
		logger().Error("deliverRegistryText: read response failed", "plugin", proc.Name(), "error", err)
		return
	}
	if resp != "ok" {
		logger().Error("deliverRegistryText: plugin responded with error", "plugin", proc.Name(), "response", resp)
	}
}
