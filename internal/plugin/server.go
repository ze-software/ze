package plugin

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// logger is the plugin server subsystem logger (lazy initialization).
// Controlled by ze.log.server environment variable.
var logger = slogutil.LazyLogger("server")

// Default stage timeout for plugin registration protocol.
// Each stage must complete within this duration.
const defaultStageTimeout = 5 * time.Second

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

	// Use per-plugin timeout if configured, else default
	timeout := proc.config.StageTimeout
	if timeout == 0 {
		timeout = defaultStageTimeout
	}

	stageCtx, cancel := context.WithTimeout(s.ctx, timeout)
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
	// First transition: from → mid
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

	// Second transition: mid → to
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

// Server manages API connections and command dispatch.
type Server struct {
	config        *ServerConfig
	reactor       ReactorInterface
	dispatcher    *Dispatcher
	encoder       *JSONEncoder
	commitManager *CommitManager
	procManager   *ProcessManager
	subscriptions *SubscriptionManager // API-driven event subscriptions

	// Plugin registration protocol
	coordinator *StartupCoordinator // Stage synchronization
	registry    *PluginRegistry     // Command/capability registry
	capInjector *CapabilityInjector // Capability injection for OPEN

	listener net.Listener
	clients  map[string]*Client
	clientID atomic.Uint64

	running atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// Client represents a connected API client.
type Client struct {
	id     string
	conn   net.Conn
	server *Server

	ctx    context.Context
	cancel context.CancelFunc
}

// NewServer creates a new API server.
func NewServer(config *ServerConfig, reactor ReactorInterface) *Server {
	s := &Server{
		config:        config,
		reactor:       reactor,
		dispatcher:    NewDispatcher(),
		encoder:       NewJSONEncoder("6.0.0"),
		commitManager: NewCommitManager(),
		subscriptions: NewSubscriptionManager(),
		registry:      NewPluginRegistry(),
		capInjector:   NewCapabilityInjector(),
		clients:       make(map[string]*Client),
	}

	// Register default handlers
	RegisterDefaultHandlers(s.dispatcher)

	return s
}

// Running returns true if the server is running.
func (s *Server) Running() bool {
	return s.running.Load()
}

// ClientCount returns the number of connected clients.
func (s *Server) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// Start begins accepting connections.
func (s *Server) Start() error {
	return s.StartWithContext(context.Background())
}

// StartWithContext begins accepting connections with the given context.
func (s *Server) StartWithContext(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Only start socket listener if socket path is configured
	if s.config.SocketPath != "" {
		// Remove existing socket if present
		if err := os.Remove(s.config.SocketPath); err != nil && !os.IsNotExist(err) {
			return err
		}

		// Create listener
		var lc net.ListenConfig
		listener, err := lc.Listen(ctx, "unix", s.config.SocketPath)
		if err != nil {
			return err
		}

		s.listener = listener
		s.running.Store(true)

		// Start accept loop
		s.wg.Add(1)
		go s.acceptLoop()
	} else {
		s.running.Store(true)
	}

	// Start plugin phases asynchronously (non-blocking)
	// Phase 1: Explicit plugins
	// Phase 2: Auto-load plugins for unclaimed families (after Phase 1 registers)
	if len(s.config.Plugins) > 0 || len(s.config.ConfiguredFamilies) > 0 {
		s.wg.Add(1)
		go s.runPluginStartup()
	}

	return nil
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

	// Handle commands synchronously (blocks until all plugins reach StageRunning)
	s.handleProcessCommandsSync(pm)

	return nil
}

// handleProcessCommandsSync handles commands from all processes and waits for completion.
// Blocks until all plugins reach StageRunning or context is cancelled.
// After StageRunning, starts async handlers for continued operation.
func (s *Server) handleProcessCommandsSync(pm *ProcessManager) {
	// Get all processes from the manager
	pm.mu.RLock()
	processes := make([]*Process, 0, len(pm.processes))
	for _, p := range pm.processes {
		processes = append(processes, p)
	}
	pm.mu.RUnlock()

	// Start a goroutine to handle startup for each process
	var procWg sync.WaitGroup
	for _, proc := range processes {
		procWg.Add(1)
		go func(p *Process) {
			defer procWg.Done()
			s.handleProcessStartup(p)
		}(proc)
	}

	procWg.Wait()

	// After startup, start async handlers for continued operation
	for _, proc := range processes {
		go s.handleSingleProcessCommands(proc)
	}
}

// handleProcessStartup handles a process until it reaches StageRunning.
// Returns when startup is complete, allowing the sync phase to finish.
func (s *Server) handleProcessStartup(proc *Process) {
	// Initialize process to registration stage
	proc.SetStage(StageRegistration)

	cmdCtx := &CommandContext{
		Reactor:       s.reactor,
		Encoder:       s.encoder,
		CommitManager: s.commitManager,
		Dispatcher:    s.dispatcher,
		Subscriptions: s.subscriptions,
		Process:       proc,
		Peer:          "*",
	}

	for proc.Running() && s.ctx.Err() == nil {
		// Read command from process stdout with timeout
		readCtx, cancel := context.WithTimeout(s.ctx, 100*time.Millisecond)
		line, err := proc.ReadCommand(readCtx)
		cancel()

		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			return
		}

		if line == "" {
			continue
		}

		// Parse #N serial prefix
		serial, cmd := parseSerial(line)
		cmdCtx.Serial = serial

		// Handle based on current stage
		stage := proc.Stage()
		switch stage {
		case StageRegistration:
			if s.handleRegistrationLine(proc, line) {
				continue
			}
		case StageCapability:
			if s.handleCapabilityLine(proc, line) {
				continue
			}
		case StageRunning:
			// Startup complete - return from sync handler
			// Commands will be handled by async handler started after
			return
		case StageInit, StageConfig, StageRegistry, StageReady:
			// Other stages: fall through to dispatch
		}

		// Handle "ready" command (Stage 5)
		if cmd == "ready" {
			if s.coordinator != nil {
				s.coordinator.StageComplete(proc.Index(), StageReady)

				timeout := proc.config.StageTimeout
				if timeout == 0 {
					timeout = defaultStageTimeout
				}

				stageCtx, cancel := context.WithTimeout(s.ctx, timeout)
				err := s.coordinator.WaitForStage(stageCtx, StageRunning)
				cancel()
				if err != nil {
					logger().Error("stage timeout waiting for running stage", "plugin", proc.Name(), "error", err)
					s.coordinator.PluginFailed(proc.Index(), fmt.Sprintf("stage timeout: %v", err))
					return
				}
			}

			proc.SetStage(StageRunning)
			if s.reactor != nil {
				s.reactor.SignalAPIReady()
			}
			// Startup complete - return
			return
		}

		// Dispatch command (handles subscribe, etc. during startup)
		resp, err := s.dispatcher.Dispatch(cmdCtx, cmd)
		if err != nil {
			if errors.Is(err, ErrSilent) {
				continue
			}
			resp = &Response{Status: "error", Data: err.Error()}
		}

		// Send response only if serial present
		if serial != "" && resp != nil {
			resp.Serial = serial
			respJSON, _ := json.Marshal(WrapResponse(resp))
			_ = proc.WriteEvent(string(respJSON))
		}
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

// Stop signals the server to stop.
func (s *Server) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Wait waits for the server to stop.
func (s *Server) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// acceptLoop accepts incoming connections.
func (s *Server) acceptLoop() {
	defer s.wg.Done()
	defer s.cleanup()

	for {
		// Check for shutdown
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Accept with timeout to check for shutdown
		if ul, ok := s.listener.(*net.UnixListener); ok {
			_ = ul.SetDeadline(time.Now().Add(100 * time.Millisecond))
		}

		conn, err := s.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.ctx.Done():
				return
			default:
				// Transient error, continue
				continue
			}
		}

		// Handle new client
		s.handleClient(conn)
	}
}

// cleanup closes listener and removes socket.
func (s *Server) cleanup() {
	s.running.Store(false)

	// Stop processes
	if s.procManager != nil {
		s.procManager.Stop()
	}

	// Close listener
	if s.listener != nil {
		_ = s.listener.Close()
	}

	// Close all clients
	s.mu.Lock()
	for _, client := range s.clients {
		client.cancel()
		_ = client.conn.Close()
	}
	s.clients = make(map[string]*Client)
	s.mu.Unlock()

	// Remove socket file
	if s.config.SocketPath != "" {
		_ = os.Remove(s.config.SocketPath)
	}
}

// parseSerial extracts #N prefix from command line.
// Returns (serial, command) where serial is empty if no prefix.
// Only recognizes numeric serials: "#1 cmd", "#123 cmd", not "# comment".
func parseSerial(line string) (string, string) {
	if !strings.HasPrefix(line, "#") {
		return "", line
	}
	// Find first space
	idx := strings.Index(line, " ")
	if idx <= 1 {
		return "", line // No space after # or just "#"
	}
	// Check if characters between # and space are all digits
	serial := line[1:idx]
	for _, c := range serial {
		if c < '0' || c > '9' {
			return "", line // Not a numeric serial
		}
	}
	return serial, line[idx+1:]
}

// isComment returns true if line is a comment (starts with "# ").
func isComment(line string) bool {
	return strings.HasPrefix(line, "# ")
}

// encodeAlphaSerial converts a number to alpha serial by shifting digits.
// 0->a, 1->b, ..., 9->j. Example: 123 -> "bcd", 0 -> "a", 99 -> "jj".
// Used for ze-initiated requests to avoid collision with numeric process serials.
func encodeAlphaSerial(n uint64) string {
	if n == 0 {
		return "a"
	}
	var result []byte
	for n > 0 {
		digit := n % 10
		result = append([]byte{byte('a' + digit)}, result...)
		n /= 10
	}
	return string(result)
}

// isAlphaSerial returns true if serial uses alpha encoding (a-j digits).
func isAlphaSerial(serial string) bool {
	if serial == "" {
		return false
	}
	for _, c := range serial {
		if c < 'a' || c > 'j' {
			return false
		}
	}
	return true
}

// handleSingleProcessCommands handles commands from a single process.
func (s *Server) handleSingleProcessCommands(proc *Process) {
	// Cleanup on exit
	defer s.cleanupProcess(proc)

	// Initialize process to registration stage
	proc.SetStage(StageRegistration)

	cmdCtx := &CommandContext{
		Reactor:       s.reactor,
		Encoder:       s.encoder,
		CommitManager: s.commitManager,
		Dispatcher:    s.dispatcher,
		Subscriptions: s.subscriptions,
		Process:       proc, // For session state (ack, sync)
		Peer:          "*",  // Default to all peers
	}

	for proc.Running() {
		// Check for shutdown
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Read command from process stdout with timeout
		readCtx, cancel := context.WithTimeout(s.ctx, 100*time.Millisecond)
		line, err := proc.ReadCommand(readCtx)
		cancel()

		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				// Timeout, check if process is still running and try again
				continue
			}
			// Process probably exited
			return
		}

		if line == "" {
			continue
		}

		// Check for @serial response (plugin command response)
		if serial, respType, data, ok := parsePluginResponse(line); ok {
			s.handlePluginResponse(proc, serial, respType, data)
			continue
		}

		// Handle "ready failed" at any stage - plugin is signaling startup failure
		if strings.HasPrefix(line, "ready failed ") {
			s.handlePluginFailed(proc, line)
			return
		}

		// Parse #N serial prefix
		serial, cmd := parseSerial(line)
		cmdCtx.Serial = serial

		// Handle based on current stage
		stage := proc.Stage()
		switch stage {
		case StageRegistration:
			// Stage 1: Parse registration commands
			if s.handleRegistrationLine(proc, line) {
				continue
			}
			// Fall through to normal dispatch if not a registration command

		case StageCapability:
			// Stage 3: Parse capability commands
			if s.handleCapabilityLine(proc, line) {
				continue
			}
			// Fall through to normal dispatch if not a capability command

		case StageInit, StageConfig, StageRegistry, StageReady, StageRunning:
			// Other stages: fall through to normal dispatch
		}

		// Check for register/unregister before normal dispatch (legacy/runtime)
		tokens := tokenize(cmd)
		if len(tokens) > 0 {
			switch strings.ToLower(tokens[0]) {
			case "register":
				s.handleRegisterCommand(proc, serial, tokens[1:])
				continue
			case "unregister":
				s.handleUnregisterCommand(proc, serial, tokens[1:])
				continue
			}
		}

		// Handle "ready" command (Stage 5)
		if cmd == "ready" {
			// Signal Stage 5 complete
			if s.coordinator != nil {
				s.coordinator.StageComplete(proc.Index(), StageReady)

				// Use per-plugin timeout if configured, else default
				timeout := proc.config.StageTimeout
				if timeout == 0 {
					timeout = defaultStageTimeout
				}

				// Wait for all plugins to be ready before signaling reactor
				stageCtx, cancel := context.WithTimeout(s.ctx, timeout)
				err := s.coordinator.WaitForStage(stageCtx, StageRunning)
				cancel()
				if err != nil {
					logger().Error("stage timeout waiting for running stage", "plugin", proc.Name(), "error", err)
					s.coordinator.PluginFailed(proc.Index(), fmt.Sprintf("stage timeout: %v", err))
					return
				}
			}

			proc.SetStage(StageRunning)
			if s.reactor != nil {
				s.reactor.SignalAPIReady()
			}
			continue
		}

		// Dispatch command
		logger().Debug("Dispatch", "plugin", proc.Name(), "cmd", cmd)
		resp, err := s.dispatcher.Dispatch(cmdCtx, cmd)
		logger().Debug("Dispatch result", "plugin", proc.Name(), "err", err, "resp", resp)
		if err != nil {
			// ErrSilent means suppress response entirely
			if errors.Is(err, ErrSilent) {
				continue
			}
			resp = &Response{Status: "error", Data: err.Error()}
		}

		// Send response only if serial present (serial = ack)
		// IPC 2.0: wrap response
		if serial != "" && resp != nil {
			resp.Serial = serial
			respJSON, _ := json.Marshal(WrapResponse(resp))
			_ = proc.WriteEvent(string(respJSON))
		}
	}
}

// handleRegistrationLine handles Stage 1 registration commands.
// Returns true if line was handled, false if should fall through to normal dispatch.
func (s *Server) handleRegistrationLine(proc *Process, line string) bool {
	reg := proc.Registration()
	if err := reg.ParseLine(line); err != nil {
		logger().Debug("server: handleRegistrationLine PARSE ERROR", "plugin", proc.Name(), "line", line, "err", err)
		return false
	}
	if !reg.Done {
		logger().Debug("server: handleRegistrationLine parsed", "plugin", proc.Name(), "line", line)
		return true
	}

	logger().Debug("server: handleRegistrationLine DONE", "plugin", proc.Name(), "config_roots", reg.WantsConfigRoots)
	reg.Name = proc.config.Name
	if err := s.registry.Register(reg); err != nil {
		s.handlePluginConflict(proc, reg.Name, "plugin registration conflict", err)
		return true
	}

	logger().Debug("server: handleRegistrationLine calling progressThroughStages", "plugin", proc.Name())
	s.progressThroughStages(proc, reg.Name, stageProgression{
		from: StageRegistration, mid: StageConfig, to: StageCapability,
		deliver: s.deliverConfig,
	})
	logger().Debug("server: handleRegistrationLine progressThroughStages returned", "plugin", proc.Name())
	return true
}

// handleCapabilityLine handles Stage 3 capability commands.
// Returns true if line was handled, false if should fall through to normal dispatch.
func (s *Server) handleCapabilityLine(proc *Process, line string) bool {
	caps := proc.Capabilities()
	if err := caps.ParseLine(line); err != nil {
		return false
	}
	if !caps.Done {
		return true
	}

	caps.PluginName = proc.config.Name
	if err := s.capInjector.AddPluginCapabilities(caps); err != nil {
		s.handlePluginConflict(proc, caps.PluginName, "plugin capability conflict", err)
		return true
	}

	s.progressThroughStages(proc, caps.PluginName, stageProgression{
		from: StageCapability, mid: StageRegistry, to: StageReady,
		deliver: s.deliverRegistry,
	})
	return true
}

// GetPluginCapabilities returns all global plugin-declared capabilities for OPEN injection.
//
// Deprecated: Use GetPluginCapabilitiesForPeer for per-peer capability support.
func (s *Server) GetPluginCapabilities() []InjectedCapability {
	if s.capInjector == nil {
		return nil
	}
	return s.capInjector.GetCapabilities()
}

// GetPluginCapabilitiesForPeer returns plugin-declared capabilities for a specific peer.
// Returns global capabilities plus any peer-specific capabilities (per-peer takes precedence).
func (s *Server) GetPluginCapabilitiesForPeer(peerAddr string) []InjectedCapability {
	if s.capInjector == nil {
		return nil
	}
	return s.capInjector.GetCapabilitiesForPeer(peerAddr)
}

// GetDecodeFamilies returns all families that have decode plugins registered.
// Used by Session to auto-add Multiprotocol capabilities in OPEN.
// Plugins that can decode a family should advertise that family to peers.
func (s *Server) GetDecodeFamilies() []string {
	if s.registry == nil {
		return nil
	}
	return s.registry.GetDecodeFamilies()
}

// GetSchemaDeclarations returns all schema declarations from registered plugins.
// Used for two-phase config parsing to extend the schema before parsing peer config.
// Should be called after Stage 1 (Registration) completes for all plugins.
func (s *Server) GetSchemaDeclarations() []SchemaDeclaration {
	if s.procManager == nil {
		return nil
	}

	var declarations []SchemaDeclaration
	for _, proc := range s.procManager.processes {
		reg := proc.Registration()
		declarations = append(declarations, reg.SchemaDeclarations...)
	}
	return declarations
}

// deliverConfig sends configuration to a plugin (Stage 2).
// Plugins declare which config roots they want via "declare wants config <root>".
// Format: "config json <root> <json>" for each declared root.
// Supports path-based scopes: "bgp/peer" extracts configTree["bgp"]["peer"].
func (s *Server) deliverConfig(proc *Process) {
	logger().Debug("server: deliverConfig START", "plugin", proc.Name())
	reg := proc.Registration()

	// Fast path: plugin doesn't want any config
	if len(reg.WantsConfigRoots) == 0 {
		logger().Debug("server: deliverConfig FAST PATH (no config roots)", "plugin", proc.Name())
		_ = proc.WriteEvent("config done")
		return
	}

	if s.reactor == nil {
		logger().Debug("server: deliverConfig FAST PATH (no reactor)", "plugin", proc.Name())
		_ = proc.WriteEvent("config done")
		return
	}

	// Get full config tree from reactor
	configTree := s.reactor.GetConfigTree()
	if configTree == nil {
		logger().Debug("server: deliverConfig FAST PATH (no config tree)", "plugin", proc.Name())
		_ = proc.WriteEvent("config done")
		return
	}

	// Send each requested root as JSON
	for _, root := range reg.WantsConfigRoots {
		subtree := extractConfigSubtree(configTree, root)
		if subtree == nil {
			logger().Debug("server: deliverConfig root not found", "plugin", proc.Name(), "root", root)
			continue
		}

		// Serialize to JSON
		jsonBytes, err := json.Marshal(subtree)
		if err != nil {
			logger().Warn("server: deliverConfig JSON marshal failed", "plugin", proc.Name(), "root", root, "err", err)
			continue
		}

		// Format: config json <root> <json>
		line := fmt.Sprintf("config json %s %s", root, string(jsonBytes))
		_ = proc.WriteEvent(line)
		logger().Debug("server: deliverConfig sent", "plugin", proc.Name(), "root", root)
	}

	logger().Debug("server: deliverConfig DONE", "plugin", proc.Name())
	_ = proc.WriteEvent("config done")
}

// extractConfigSubtree extracts a subtree from the config based on path.
// Always returns data wrapped in its full path structure from root.
// Supports:
//   - "*" → entire tree
//   - "bgp" → {"bgp": configTree["bgp"]}
//   - "bgp/peer" → {"bgp": {"peer": configTree["bgp"]["peer"]}}
func extractConfigSubtree(configTree map[string]any, path string) any {
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

// deliverRegistry sends the command registry to a plugin (Stage 4).
func (s *Server) deliverRegistry(proc *Process) {
	reg := proc.Registration()
	allCommands := s.registry.BuildCommandInfo()
	lines := FormatRegistrySharing(reg.Name, allCommands)

	for _, line := range lines {
		_ = proc.WriteEvent(line)
	}
}

// handlePluginFailed handles a "ready failed" message from a plugin.
// This can occur at any stage and indicates startup failure.
// Signals the coordinator to abort startup for all plugins.
func (s *Server) handlePluginFailed(proc *Process, line string) {
	// Parse: "ready failed text <message>" or "ready failed b64 <message>"
	parts := strings.SplitN(line, " ", 4)
	errMsg := "plugin startup failed"
	if len(parts) >= 4 {
		errMsg = parts[3]
	}

	// Log the failure with structured logging
	logger().Error("plugin startup failed",
		"plugin", proc.Name(),
		"stage", proc.Stage().String(),
		"error", errMsg,
	)

	// Signal coordinator to abort startup
	if s.coordinator != nil {
		s.coordinator.PluginFailed(proc.Index(), errMsg)
	}

	// Stop the process
	proc.Stop()
}

// handleRegisterCommand processes a register command from a process.
func (s *Server) handleRegisterCommand(proc *Process, serial string, tokens []string) {
	def, err := parseRegisterCommand(tokens)
	if err != nil {
		if serial != "" {
			resp := &Response{Serial: serial, Status: "error", Data: err.Error()}
			respJSON, _ := json.Marshal(WrapResponse(resp))
			_ = proc.WriteEvent(string(respJSON))
		}
		return
	}

	results := s.dispatcher.Registry().Register(proc, []CommandDef{*def})
	result := results[0]

	if result.OK {
		proc.AddRegisteredCommand(def.Name)
	}

	if serial != "" {
		var resp *Response
		if result.OK {
			resp = &Response{Serial: serial, Status: "done"}
		} else {
			resp = &Response{Serial: serial, Status: "error", Data: result.Error}
		}
		respJSON, _ := json.Marshal(WrapResponse(resp))
		_ = proc.WriteEvent(string(respJSON))
	}
}

// handleUnregisterCommand processes an unregister command from a process.
func (s *Server) handleUnregisterCommand(proc *Process, serial string, tokens []string) {
	name, err := parseUnregisterCommand(tokens)
	if err != nil {
		if serial != "" {
			resp := &Response{Serial: serial, Status: "error", Data: err.Error()}
			respJSON, _ := json.Marshal(WrapResponse(resp))
			_ = proc.WriteEvent(string(respJSON))
		}
		return
	}

	s.dispatcher.Registry().Unregister(proc, []string{name})
	proc.RemoveRegisteredCommand(name)

	if serial != "" {
		resp := &Response{Serial: serial, Status: "done"}
		respJSON, _ := json.Marshal(WrapResponse(resp))
		_ = proc.WriteEvent(string(respJSON))
	}
}

// handlePluginResponse handles a response from a plugin process.
func (s *Server) handlePluginResponse(_ *Process, serial, respType, data string) {
	pending := s.dispatcher.Pending()

	switch respType {
	case statusDone:
		var respData any
		if data != "" {
			// Try to parse as JSON
			if err := json.Unmarshal([]byte(data), &respData); err != nil {
				respData = data // Use as string if not valid JSON
			}
		}
		pending.Complete(serial, &Response{Status: statusDone, Data: respData})

	case statusError:
		pending.Complete(serial, &Response{Status: statusError, Data: data})

	case "partial":
		var respData any
		if data != "" {
			if err := json.Unmarshal([]byte(data), &respData); err != nil {
				respData = data
			}
		}
		pending.Partial(serial, &Response{Status: statusDone, Partial: true, Data: respData})
	}
}

// cleanupProcess handles cleanup when a process exits.
func (s *Server) cleanupProcess(proc *Process) {
	// Unregister all commands from this process
	s.dispatcher.Registry().UnregisterAll(proc)

	// Cancel all pending requests
	s.dispatcher.Pending().CancelAll(proc)

	// Clear all subscriptions for this process
	if s.subscriptions != nil {
		s.subscriptions.ClearProcess(proc)
	}
}

// handleClient creates and manages a client connection.
func (s *Server) handleClient(conn net.Conn) {
	id := s.clientID.Add(1)
	clientID := string(rune('0'+id%10)) + conn.RemoteAddr().String()

	clientCtx, clientCancel := context.WithCancel(s.ctx)

	client := &Client{
		id:     clientID,
		conn:   conn,
		server: s,
		ctx:    clientCtx,
		cancel: clientCancel,
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	s.wg.Add(1)
	go s.clientLoop(client)
}

// clientLoop reads and processes commands from a client.
func (s *Server) clientLoop(client *Client) {
	defer s.wg.Done()
	defer s.removeClient(client)
	defer func() { _ = client.conn.Close() }()

	reader := bufio.NewReader(client.conn)

	for {
		select {
		case <-client.ctx.Done():
			return
		default:
		}

		// Read line
		line, err := reader.ReadString('\n')
		if err != nil {
			return // Client disconnected
		}

		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip comments (lines starting with "# ")
		if isComment(line) {
			continue
		}

		// Process command (handles #N serial prefix)
		s.processCommand(client, line)
	}
}

// processCommand dispatches a command and sends response.
func (s *Server) processCommand(client *Client, line string) {
	// Parse #N serial prefix
	serial, command := parseSerial(line)

	ctx := &CommandContext{
		Reactor:       s.reactor,
		Encoder:       s.encoder,
		CommitManager: s.commitManager,
		Dispatcher:    s.dispatcher,
		Subscriptions: s.subscriptions,
		Serial:        serial,
		// Note: Process is nil for socket clients - session commands are no-ops
	}

	resp, err := s.dispatcher.Dispatch(ctx, command)
	if err != nil {
		// ErrSilent means suppress response entirely
		if errors.Is(err, ErrSilent) {
			return
		}
		// Send error response
		resp = &Response{
			Status: "error",
			Data:   err.Error(),
		}
	}

	// Socket clients always get responses, serial in JSON body
	if resp != nil {
		resp.Serial = serial
		s.sendResponse(client, resp)
	}
}

// sendResponse sends a JSON response to the client.
// IPC 2.0: wraps response in {"type":"response","response":{...}}.
func (s *Server) sendResponse(client *Client, resp *Response) {
	wrapped := WrapResponse(resp)
	data, err := json.Marshal(wrapped)
	if err != nil {
		// Fallback error response (also wrapped)
		data = []byte(`{"type":"response","response":{"status":"error","data":"json marshal failed"}}`)
	}

	data = append(data, '\n')
	_, _ = client.conn.Write(data)
}

// removeClient removes a client from tracking.
func (s *Server) removeClient(client *Client) {
	s.mu.Lock()
	delete(s.clients, client.id)
	s.mu.Unlock()
}

// OnMessageReceived handles raw BGP messages from peers.
// Forwards to processes based on API subscriptions.
// Implements reactor.MessageReceiver interface.
//
// This is called for ALL message types (UPDATE, OPEN, NOTIFICATION, KEEPALIVE).
func (s *Server) OnMessageReceived(peer PeerInfo, msg RawMessage) {
	if s.procManager == nil || s.subscriptions == nil {
		logger().Debug("OnMessageReceived: no procManager or subscriptions")
		return
	}

	eventType := messageTypeToEventType(msg.Type)
	if eventType == "" {
		logger().Debug("OnMessageReceived: unknown event type", "msgType", msg.Type)
		return
	}

	logger().Debug("OnMessageReceived", "peer", peer.Address.String(), "event", eventType, "dir", msg.Direction)
	procs := s.subscriptions.GetMatching(NamespaceBGP, eventType, msg.Direction, peer.Address.String())
	logger().Debug("OnMessageReceived matched", "count", len(procs))
	for _, proc := range procs {
		output := s.formatMessageForSubscription(peer, msg, proc.Format())
		logger().Debug("OnMessageReceived writing", "proc", proc.Name(), "outputLen", len(output))
		if err := proc.WriteEvent(output); err != nil {
			logger().Warn("OnMessageReceived write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// messageTypeToEventType converts BGP message type to event type string.
// Returns empty string for unsupported types.
func messageTypeToEventType(msgType message.MessageType) string {
	switch msgType { //nolint:exhaustive // Only handle supported types
	case message.TypeUPDATE:
		return EventUpdate
	case message.TypeOPEN:
		return EventOpen
	case message.TypeNOTIFICATION:
		return EventNotification
	case message.TypeKEEPALIVE:
		return EventKeepalive
	case message.TypeROUTEREFRESH:
		return EventRefresh
	default:
		return ""
	}
}

// formatMessageForSubscription formats a BGP message for subscription-based delivery.
// Uses JSON encoding with the specified format (from process settings).
func (s *Server) formatMessageForSubscription(peer PeerInfo, msg RawMessage, format string) string {
	switch msg.Type { //nolint:exhaustive // Only handle supported types
	case message.TypeUPDATE:
		content := ContentConfig{
			Encoding: EncodingJSON,
			Format:   format,
		}
		return FormatMessage(peer, msg, content, "")

	case message.TypeOPEN:
		decoded := DecodeOpen(msg.RawBytes)
		return s.encoder.Open(peer, decoded, msg.Direction, msg.MessageID)

	case message.TypeNOTIFICATION:
		decoded := DecodeNotification(msg.RawBytes)
		return s.encoder.Notification(peer, decoded, msg.Direction, msg.MessageID)

	case message.TypeKEEPALIVE:
		return s.encoder.Keepalive(peer, msg.Direction, msg.MessageID)

	case message.TypeROUTEREFRESH:
		decoded := DecodeRouteRefresh(msg.RawBytes)
		return s.encoder.RouteRefresh(peer, decoded, msg.Direction, msg.MessageID)

	default:
		return ""
	}
}

// OnPeerStateChange handles peer state transitions.
// Called by reactor when peer state changes (not a BGP message).
// State events are separate from BGP protocol messages.
func (s *Server) OnPeerStateChange(peer PeerInfo, state string) {
	logger().Debug("OnPeerStateChange", "peer", peer.Address.String(), "state", state)
	if s.procManager == nil || s.subscriptions == nil {
		logger().Debug("OnPeerStateChange: no procManager or subscriptions")
		return
	}

	procs := s.subscriptions.GetMatching(NamespaceBGP, EventState, "", peer.Address.String())
	logger().Debug("OnPeerStateChange matched", "count", len(procs))
	for _, proc := range procs {
		output := FormatStateChange(peer, state, EncodingJSON)
		logger().Debug("OnPeerStateChange writing", "proc", proc.Name())
		if err := proc.WriteEvent(output); err != nil {
			logger().Warn("OnPeerStateChange write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// OnPeerNegotiated handles capability negotiation completion.
// Called by reactor after OPEN exchange completes successfully.
// Informs plugins of negotiated capabilities so they can adjust behavior.
func (s *Server) OnPeerNegotiated(peer PeerInfo, neg DecodedNegotiated) {
	if s.procManager == nil || s.subscriptions == nil {
		return
	}

	procs := s.subscriptions.GetMatching(NamespaceBGP, EventNegotiated, "", peer.Address.String())
	for _, proc := range procs {
		output := FormatNegotiated(peer, neg, s.encoder)
		if err := proc.WriteEvent(output); err != nil {
			logger().Warn("OnPeerNegotiated write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// OnMessageSent handles BGP messages sent to peers.
// Forwards to processes that subscribed to sent events.
// Called by reactor after successfully sending UPDATE to peer.
func (s *Server) OnMessageSent(peer PeerInfo, msg RawMessage) {
	eventType := messageTypeToEventType(msg.Type)
	logger().Debug("OnMessageSent", "peer", peer.Address.String(), "type", eventType)
	if s.procManager == nil || s.subscriptions == nil {
		logger().Debug("OnMessageSent: no procManager or subscriptions")
		return
	}

	if eventType == "" {
		return
	}

	procs := s.subscriptions.GetMatching(NamespaceBGP, eventType, DirectionSent, peer.Address.String())
	logger().Debug("OnMessageSent matched", "count", len(procs))
	for _, proc := range procs {
		output := s.formatSentMessageForSubscription(peer, msg, proc.Format())
		logger().Debug("OnMessageSent writing", "proc", proc.Name())
		if err := proc.WriteEvent(output); err != nil {
			logger().Warn("OnMessageSent write failed", "proc", proc.Name(), "err", err)
		}
	}
}

// formatSentMessageForSubscription formats a sent BGP message for subscription delivery.
// Uses FormatSentMessage which sets "type":"sent" to distinguish from received messages.
// The format parameter is the process's configured format (hex, base64, parsed, full).
func (s *Server) formatSentMessageForSubscription(peer PeerInfo, msg RawMessage, format string) string {
	content := ContentConfig{
		Encoding: EncodingJSON,
		Format:   format,
	}
	return FormatSentMessage(peer, msg, content)
}

// EncodeNLRI encodes NLRI by routing to the appropriate family plugin.
// This is the public API for external callers (CLI tools, external plugins, tests).
// Internal code paths use direct function calls for performance (e.g., update_text.go
// calls flowspec.Encode directly). This method exists for callers that don't know
// which plugin handles a family at compile time.
// Returns error if no plugin registered or plugin not running.
func (s *Server) EncodeNLRI(family nlri.Family, args []string) ([]byte, error) {
	if s.registry == nil || s.procManager == nil {
		return nil, fmt.Errorf("server not configured for plugins")
	}

	familyStr := family.String()
	pluginName := s.registry.LookupFamily(familyStr)
	if pluginName == "" {
		return nil, fmt.Errorf("no plugin registered for family %s", familyStr)
	}

	// Get the process
	proc := s.procManager.GetProcess(pluginName)
	if proc == nil {
		return nil, fmt.Errorf("plugin %s not running", pluginName)
	}

	// Build request: "encode nlri <family> <args...>"
	command := fmt.Sprintf("encode nlri %s %s", familyStr, strings.Join(args, " "))

	// Send request and wait for response
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	response, err := proc.SendRequest(ctx, command)
	if err != nil {
		return nil, fmt.Errorf("plugin request failed: %w", err)
	}

	// Parse response: "encoded hex <hex>" or "encoded error <msg>"
	if strings.HasPrefix(response, "encoded hex ") {
		hexStr := strings.TrimPrefix(response, "encoded hex ")
		data, err := hex.DecodeString(hexStr)
		if err != nil {
			return nil, fmt.Errorf("decode plugin hex response: %w", err)
		}
		return data, nil
	}
	if strings.HasPrefix(response, "encoded error ") {
		return nil, errors.New(strings.TrimPrefix(response, "encoded error "))
	}

	return nil, fmt.Errorf("unexpected plugin response: %s", response)
}

// DecodeNLRI decodes NLRI by routing to the appropriate family plugin.
// This is the public API for external callers (CLI tools, external plugins, tests).
// Internal code paths use direct function calls for performance. This method exists
// for callers that don't know which plugin handles a family at compile time.
// Returns the JSON representation of the decoded NLRI.
// Returns error if no plugin registered or plugin not running.
func (s *Server) DecodeNLRI(family nlri.Family, hexData string) (string, error) {
	if s.registry == nil || s.procManager == nil {
		return "", fmt.Errorf("server not configured for plugins")
	}

	familyStr := family.String()
	pluginName := s.registry.LookupFamily(familyStr)
	if pluginName == "" {
		return "", fmt.Errorf("no plugin registered for family %s", familyStr)
	}

	// Get the process
	proc := s.procManager.GetProcess(pluginName)
	if proc == nil {
		return "", fmt.Errorf("plugin %s not running", pluginName)
	}

	// Build request: "decode nlri <family> <hex>"
	command := fmt.Sprintf("decode nlri %s %s", familyStr, hexData)

	// Send request and wait for response
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	response, err := proc.SendRequest(ctx, command)
	if err != nil {
		return "", fmt.Errorf("plugin request failed: %w", err)
	}

	// Parse response: "decoded json <json>" or "decoded unknown"
	if strings.HasPrefix(response, "decoded json ") {
		return strings.TrimPrefix(response, "decoded json "), nil
	}
	if response == "decoded unknown" {
		return "", fmt.Errorf("plugin could not decode NLRI")
	}

	return "", fmt.Errorf("unexpected plugin response: %s", response)
}
