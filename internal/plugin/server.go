package plugin

import (
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

	"codeberg.org/thomas-mangin/ze/internal/ipc"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// logger is the plugin server subsystem logger (lazy initialization).
// Controlled by ze.log.server environment variable.
var logger = slogutil.LazyLogger("server")

// Default stage timeout for plugin registration protocol.
// Each stage must complete within this duration.
const defaultStageTimeout = 5 * time.Second

// RPCParams is the standard params format for JSON RPC requests from socket clients.
// Handlers receive Args as positional arguments and Selector as the peer filter.
type RPCParams struct {
	Selector string   `json:"selector,omitempty"` // Peer selector (optional)
	Args     []string `json:"args,omitempty"`     // Command arguments (optional)
}

// wrapHandler adapts a Handler to an ipc.RPCHandler for the RPC dispatcher.
// Creates a CommandContext from the server state and extracts args from JSON params.
func (s *Server) wrapHandler(handler Handler) ipc.RPCHandler {
	return func(_ string, params json.RawMessage) (any, error) {
		ctx := &CommandContext{
			Reactor:       s.reactor,
			Encoder:       s.encoder,
			CommitManager: s.commitManager,
			Dispatcher:    s.dispatcher,
			Subscriptions: s.subscriptions,
			Peer:          "*",
		}

		var rpcParams RPCParams
		if len(params) > 0 {
			if err := json.Unmarshal(params, &rpcParams); err != nil {
				return nil, fmt.Errorf("invalid params: %w", err)
			}
		}

		if rpcParams.Selector != "" {
			ctx.Peer = rpcParams.Selector
		}

		resp, err := handler(ctx, rpcParams.Args)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, nil
		}
		if resp.Status == statusError {
			return nil, fmt.Errorf("%v", resp.Data)
		}
		return resp.Data, nil
	}
}

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
	rpcDispatcher *ipc.RPCDispatcher // Wire method dispatch for socket clients
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
		rpcDispatcher: ipc.NewRPCDispatcher(),
		encoder:       NewJSONEncoder("6.0.0"),
		commitManager: NewCommitManager(),
		subscriptions: NewSubscriptionManager(),
		registry:      NewPluginRegistry(),
		capInjector:   NewCapabilityInjector(),
		clients:       make(map[string]*Client),
	}

	// Register default handlers (text dispatcher for plugin protocol)
	RegisterDefaultHandlers(s.dispatcher)

	// Register all builtin RPCs with wire method dispatcher (for socket clients)
	for _, reg := range AllBuiltinRPCs() {
		if err := s.rpcDispatcher.Register(reg.WireMethod, s.wrapHandler(reg.Handler)); err != nil {
			logger().Error("rpc dispatcher: registration failed", "method", reg.WireMethod, "error", err)
		}
	}

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

// encodeAlphaSerial converts a number to alpha serial by shifting digits.
// 0->a, 1->b, ..., 9->j. Example: 123 -> "bcd", 0 -> "a", 99 -> "jj".
// Used by PendingRequests for engine-initiated request serials.
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

// handleProcessStartupRPC handles the 5-stage plugin startup via YANG RPC protocol.
// Reads plugin→engine RPCs from engineConnA, sends engine→plugin callbacks via engineConnB.
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

	connA := proc.engineConnA

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

	// Progress: Registration → Config (deliver config) → Capability
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

	// Progress: Capability → Registry (deliver registry) → Ready
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
	// receives events from the very first route send — no race with the reactor.
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

	// Send OK response
	if err := connA.SendResult(s.ctx, req.ID, nil); err != nil {
		return
	}

	// Final stage transition: Ready → Running
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
				subtree := extractConfigSubtree(configTree, root)
				if subtree == nil {
					continue
				}
				jsonBytes, err := json.Marshal(subtree)
				if err != nil {
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

// handleSingleProcessCommandsRPC handles runtime commands for an RPC-mode plugin.
// Reads from engineConnA and dispatches plugin→engine RPCs (update-route, subscribe, etc.).
// Event delivery to plugins is handled directly via engineConnB.SendDeliverEvent
// in OnMessageReceived, OnPeerStateChange, etc.
func (s *Server) handleSingleProcessCommandsRPC(proc *Process) {
	defer s.cleanupProcess(proc)

	connA := proc.engineConnA

	// Plugin→engine RPC loop: read from engineConnA, dispatch.
	for {
		req, err := connA.ReadRequest(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				return // Server shutting down
			}
			logger().Debug("rpc runtime: read failed", "plugin", proc.Name(), "error", err)
			return // Connection closed (plugin exited)
		}

		s.dispatchPluginRPC(proc, connA, req)
	}
}

// dispatchPluginRPC handles a single plugin→engine RPC request.
// Unknown or empty methods get an explicit error per ze's fail-on-unknown rule.
func (s *Server) dispatchPluginRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	switch req.Method {
	case "ze-plugin-engine:update-route":
		s.handleUpdateRouteRPC(proc, connA, req)
		return
	case "ze-plugin-engine:subscribe-events":
		s.handleSubscribeEventsRPC(proc, connA, req)
		return
	case "ze-plugin-engine:unsubscribe-events":
		s.handleUnsubscribeEventsRPC(proc, connA, req)
		return
	}
	if err := connA.SendError(s.ctx, req.ID, "unknown method: "+req.Method); err != nil {
		logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", err)
	}
}

// handleUpdateRouteRPC handles ze-plugin-engine:update-route from a plugin.
// Dispatches the command string through the standard command dispatcher.
func (s *Server) handleUpdateRouteRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	var input rpc.UpdateRouteInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "invalid update-route params: "+err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	cmdCtx := &CommandContext{
		Reactor:       s.reactor,
		Encoder:       s.encoder,
		CommitManager: s.commitManager,
		Dispatcher:    s.dispatcher,
		Subscriptions: s.subscriptions,
		Process:       proc,
		Peer:          input.PeerSelector,
	}
	if cmdCtx.Peer == "" {
		cmdCtx.Peer = "*"
	}

	// Reconstruct the full command for the dispatcher.
	// The dispatcher matches commands like "bgp peer update", "bgp peer cache", etc.
	// The RPC protocol separates peer selector from command, so we prepend "bgp peer"
	// to let the dispatcher do its prefix matching. The peer selector is already set
	// on cmdCtx.Peer; the dispatcher won't extract one from tokens[2] because command
	// tokens like "update", "cache", "plugin" don't look like IPs (no dots/colons).
	dispatchCmd := "bgp peer " + input.Command

	resp, err := s.dispatcher.Dispatch(cmdCtx, dispatchCmd)
	if err != nil {
		if errors.Is(err, ErrSilent) {
			if sendErr := connA.SendResult(s.ctx, req.ID, &rpc.UpdateRouteOutput{}); sendErr != nil {
				logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
			}
			return
		}
		if sendErr := connA.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	// Extract route counts from response if available
	output := &rpc.UpdateRouteOutput{}
	if resp != nil && resp.Data != nil {
		if m, ok := resp.Data.(map[string]any); ok {
			if v, ok := m["peers-affected"]; ok {
				if n, ok := v.(float64); ok {
					output.PeersAffected = uint32(n)
				}
			}
			if v, ok := m["routes-sent"]; ok {
				if n, ok := v.(float64); ok {
					output.RoutesSent = uint32(n)
				}
			}
		}
	}

	if sendErr := connA.SendResult(s.ctx, req.ID, output); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// parseEventString splits an event string like "update direction sent" into
// (eventType, direction). If no "direction" keyword is present, returns DirectionBoth.
// This mirrors the text protocol's parseSubscription logic for RPC event strings.
func parseEventString(event string) (string, string) {
	parts := strings.Fields(event)
	if len(parts) >= 3 && parts[1] == "direction" {
		return parts[0], parts[2]
	}
	return event, DirectionBoth
}

// registerSubscriptions registers event subscriptions for a process.
// Parses event strings (e.g. "update direction sent") into EventType + Direction.
func (s *Server) registerSubscriptions(proc *Process, input *rpc.SubscribeEventsInput) {
	if input.Format != "" {
		proc.SetFormat(input.Format)
	}

	for _, event := range input.Events {
		eventType, direction := parseEventString(event)
		sub := &Subscription{
			Namespace: NamespaceBGP,
			EventType: eventType,
			Direction: direction,
		}
		if len(input.Peers) > 0 {
			sub.PeerFilter = &PeerFilter{Selector: input.Peers[0]}
		}
		s.subscriptions.Add(proc, sub)
	}
}

// handleSubscribeEventsRPC handles ze-plugin-engine:subscribe-events from a plugin.
func (s *Server) handleSubscribeEventsRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	var input rpc.SubscribeEventsInput
	if err := json.Unmarshal(req.Params, &input); err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "invalid subscribe params: "+err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	if s.subscriptions == nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "subscription manager not available"); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	s.registerSubscriptions(proc, &input)
	if sendErr := connA.SendResult(s.ctx, req.ID, nil); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// handleUnsubscribeEventsRPC handles ze-plugin-engine:unsubscribe-events from a plugin.
func (s *Server) handleUnsubscribeEventsRPC(proc *Process, connA *PluginConn, req *ipc.Request) {
	if s.subscriptions == nil {
		if sendErr := connA.SendError(s.ctx, req.ID, "subscription manager not available"); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	s.subscriptions.ClearProcess(proc)
	if sendErr := connA.SendResult(s.ctx, req.ID, nil); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
}

// registrationFromRPC converts DeclareRegistrationInput (RPC types) to PluginRegistration (engine types).
func registrationFromRPC(input *rpc.DeclareRegistrationInput) *PluginRegistration {
	reg := &PluginRegistration{
		WantsConfigRoots: input.WantsConfig,
		Done:             true,
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
		caps.Capabilities = append(caps.Capabilities, PluginCapability(cap))
	}

	return caps
}

// handlePluginFailed handles a "ready failed" message from a plugin.
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

// clientLoop reads NUL-framed JSON RPC requests and dispatches them.
func (s *Server) clientLoop(client *Client) {
	defer s.wg.Done()
	defer s.removeClient(client)
	defer client.conn.Close() //nolint:errcheck // best-effort cleanup on defer

	reader := ipc.NewFrameReader(client.conn)
	writer := ipc.NewFrameWriter(client.conn)

	for {
		select {
		case <-client.ctx.Done():
			return
		default: // proceed to read next frame
		}

		msg, err := reader.Read()
		if err != nil {
			return // Client disconnected or read error
		}

		if len(msg) == 0 {
			continue
		}

		var req ipc.Request
		if err := json.Unmarshal(msg, &req); err != nil {
			errResp := &ipc.RPCError{Error: "invalid-json"}
			if writeErr := s.writeRPCResponse(writer, errResp); writeErr != nil {
				return
			}
			continue
		}

		result := s.rpcDispatcher.Dispatch(&req)
		if writeErr := s.writeRPCResponse(writer, result); writeErr != nil {
			return
		}
	}
}

// writeRPCResponse marshals and writes an RPC response via NUL-framed writer.
// Returns error if the write fails (caller should close connection).
func (s *Server) writeRPCResponse(writer *ipc.FrameWriter, result any) error {
	respJSON, err := json.Marshal(result)
	if err != nil {
		logger().Warn("failed to marshal RPC response", "error", err)
		return err
	}
	return writer.Write(respJSON)
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
		connB := proc.ConnB()
		if connB == nil {
			continue
		}
		deliverCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()
		if err != nil {
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
		connB := proc.ConnB()
		if connB == nil {
			continue
		}
		deliverCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()
		if err != nil {
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
		connB := proc.ConnB()
		if connB == nil {
			continue
		}
		deliverCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()
		if err != nil {
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
		connB := proc.ConnB()
		if connB == nil {
			continue
		}
		deliverCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		err := connB.SendDeliverEvent(deliverCtx, output)
		cancel()
		if err != nil {
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

// EncodeNLRI encodes NLRI by routing to the appropriate family plugin via RPC.
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

	// Send RPC request and wait for response
	connB := proc.ConnB()
	if connB == nil {
		return nil, fmt.Errorf("plugin %s connection closed", pluginName)
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	hexStr, err := connB.SendEncodeNLRI(ctx, familyStr, args)
	if err != nil {
		return nil, fmt.Errorf("plugin request failed: %w", err)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("decode plugin hex response: %w", err)
	}
	return data, nil
}

// DecodeNLRI decodes NLRI by routing to the appropriate family plugin via RPC.
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

	// Send RPC request and wait for response
	connB := proc.ConnB()
	if connB == nil {
		return "", fmt.Errorf("plugin %s connection closed", pluginName)
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	jsonResult, err := connB.SendDecodeNLRI(ctx, familyStr, hexData)
	if err != nil {
		return "", fmt.Errorf("plugin request failed: %w", err)
	}

	return jsonResult, nil
}
