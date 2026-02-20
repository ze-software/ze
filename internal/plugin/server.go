// Design: docs/architecture/api/process-protocol.md — plugin process management

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
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// logger is the plugin server subsystem logger (lazy initialization).
// Controlled by ze.log.server environment variable.
var logger = slogutil.LazyLogger("server")

// Default stage timeout for plugin registration protocol.
// Each stage must complete within this duration.
// Override via ze.plugin.stage.timeout env var or per-plugin config timeout.
const defaultStageTimeout = 5 * time.Second

// stageTimeoutFromEnv reads ze.plugin.stage.timeout (or ze_plugin_stage_timeout)
// and returns the parsed duration. Falls back to defaultStageTimeout on missing
// or invalid values.
func stageTimeoutFromEnv() time.Duration {
	for _, key := range []string{"ze.plugin.stage.timeout", "ze_plugin_stage_timeout"} {
		if v := os.Getenv(key); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil {
				logger().Warn("invalid stage timeout env var", "key", key, "value", v, "error", err)
				return defaultStageTimeout
			}
			return d
		}
	}
	return defaultStageTimeout
}

// RPCParams is the standard params format for JSON RPC requests from socket clients.
// Handlers receive Args as positional arguments and Selector as the peer filter.
type RPCParams struct {
	Selector string   `json:"selector,omitempty"` // Peer selector (optional)
	Args     []string `json:"args,omitempty"`     // Command arguments (optional)
}

// Server manages API connections and command dispatch.
type Server struct {
	config        *ServerConfig
	reactor       ReactorLifecycle
	dispatcher    *Dispatcher
	rpcDispatcher *ipc.RPCDispatcher // Wire method dispatch for socket clients
	bgpHooks      *BGPHooks
	commitManager any
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

	configLoader ConfigLoader // Loads new config tree for ReloadFromDisk

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	reloadMu sync.Mutex // Prevents concurrent config reloads
}

// wrapHandler adapts a Handler to an ipc.RPCHandler for the RPC dispatcher.
// Creates a CommandContext from the server state and extracts args from JSON params.
func (s *Server) wrapHandler(handler Handler) ipc.RPCHandler {
	return func(_ string, params json.RawMessage) (any, error) {
		ctx := &CommandContext{
			Server: s,
			Peer:   "*",
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
		if resp.Status == StatusError {
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

	// Use per-plugin timeout if configured, else env var, else default.
	// Priority: config > env > default.
	timeout := proc.config.StageTimeout
	if timeout == 0 {
		timeout = stageTimeoutFromEnv()
	}

	// Deadline is stageStartTime + timeout, not now + timeout.
	// This prevents fast plugins from timing out while waiting for slow
	// plugins at the barrier — the timeout measures from when the stage
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

// Client represents a connected API client.
type Client struct {
	id     string
	conn   net.Conn
	server *Server

	ctx    context.Context
	cancel context.CancelFunc
}

// NewServer creates a new API server.
func NewServer(config *ServerConfig, reactor ReactorLifecycle) *Server {
	s := &Server{
		config:        config,
		reactor:       reactor,
		dispatcher:    NewDispatcher(),
		rpcDispatcher: ipc.NewRPCDispatcher(),
		bgpHooks:      config.BGPHooks,
		commitManager: config.CommitManager,
		subscriptions: NewSubscriptionManager(),
		registry:      NewPluginRegistry(),
		capInjector:   NewCapabilityInjector(),
		clients:       make(map[string]*Client),
	}

	// Register core handlers (text dispatcher for plugin protocol)
	RegisterDefaultHandlers(s.dispatcher)

	// Register all builtin RPCs with wire method dispatcher (for socket clients)
	for _, reg := range AllBuiltinRPCs() {
		if err := s.rpcDispatcher.Register(reg.WireMethod, s.wrapHandler(reg.Handler)); err != nil {
			logger().Error("rpc dispatcher: registration failed", "method", reg.WireMethod, "error", err)
		}
	}

	// Register additional RPCs from providers (e.g., BGP handler RPCs injected by reactor)
	for _, provider := range config.RPCProviders {
		for _, reg := range provider() {
			s.dispatcher.Register(reg.CLICommand, reg.Handler, reg.Help)
			if err := s.rpcDispatcher.Register(reg.WireMethod, s.wrapHandler(reg.Handler)); err != nil {
				logger().Error("rpc dispatcher: registration failed", "method", reg.WireMethod, "error", err)
			}
		}
	}

	return s
}

// Context returns the server's context. Used by RPC handlers that need
// a cancellable context tied to the server's lifetime (e.g., coordinator reload).
func (s *Server) Context() context.Context {
	return s.ctx
}

// Reactor returns the reactor lifecycle interface.
func (s *Server) Reactor() ReactorLifecycle {
	return s.reactor
}

// Dispatcher returns the command dispatcher.
func (s *Server) Dispatcher() *Dispatcher {
	return s.dispatcher
}

// CommitManager returns the commit manager.
func (s *Server) CommitManager() any {
	return s.commitManager
}

// Subscriptions returns the subscription manager.
func (s *Server) Subscriptions() *SubscriptionManager {
	return s.subscriptions
}

// ProcessManager returns the process manager.
// Used by BGP hook implementations to iterate plugin processes.
func (s *Server) ProcessManager() *ProcessManager {
	return s.procManager
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
	} else {
		// No plugins to start — signal immediately so WaitForPluginStartupComplete
		// does not block. SetAPIProcessCount always creates the startupComplete
		// channel, but without runPluginStartup nothing would close it.
		s.signalStartupComplete()
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
		s.reactor.RegisterCacheConsumer(proc.Name())
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

// handleSingleProcessCommandsRPC handles runtime commands for an RPC-mode plugin.
// Reads from engineConnA and dispatches plugin→engine RPCs (update-route, subscribe, etc.).
// Event delivery to plugins is handled directly via engineConnB.SendDeliverEvent
// in OnMessageReceived, OnPeerStateChange, etc.
func (s *Server) handleSingleProcessCommandsRPC(proc *Process) {
	defer s.cleanupProcess(proc)

	connA := proc.ConnA()
	if connA == nil {
		logger().Debug("rpc runtime: no connection (startup failed?)", "plugin", proc.Name())
		return
	}

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
// Generic RPCs (update-route, subscribe, unsubscribe) are handled directly.
// BGP codec RPCs (decode-nlri, encode-nlri, etc.) are delegated to BGPHooks.
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

	// Try BGP codec hook for remaining methods
	if s.bgpHooks != nil && s.bgpHooks.CodecRPCHandler != nil {
		codec := s.bgpHooks.CodecRPCHandler(req.Method)
		if codec != nil {
			s.handleCodecRPC(proc, connA, req, codec)
			return
		}
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
		Server:  s,
		Process: proc,
		Peer:    input.PeerSelector,
	}
	if cmdCtx.Peer == "" {
		cmdCtx.Peer = "*"
	}

	// Reconstruct the full command for the dispatcher.
	// Commands from "bgp peer <sel> <cmd>" arrive with the peer selector stripped
	// and command as just "<cmd>" (e.g., "update text ..."). These need "bgp peer "
	// prepended for the dispatcher to match "bgp peer update", "bgp peer teardown", etc.
	//
	// Commands that aren't peer-targeted (e.g., "bgp watchdog announce dnsr",
	// "bgp cache list") arrive with the full "bgp ..." prefix intact and must be
	// passed through directly — prepending "bgp peer " would create an unmatchable
	// "bgp peer bgp watchdog ..." command.
	var dispatchCmd string
	if strings.HasPrefix(strings.ToLower(input.Command), "bgp ") {
		dispatchCmd = input.Command
	} else {
		dispatchCmd = "bgp peer " + input.Command
	}

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
// This mirrors the text protocol's ParseSubscription logic for RPC event strings.
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

// handleCodecRPC is a shared helper for plugin→engine codec RPCs (decode-nlri, encode-nlri).
// The codec callback unmarshals params and calls the registry; it returns the result to send
// or an error to relay back to the plugin.
func (s *Server) handleCodecRPC(proc *Process, connA *PluginConn, req *ipc.Request,
	codec func(json.RawMessage) (any, error),
) {
	result, err := codec(req.Params)
	if err != nil {
		if sendErr := connA.SendError(s.ctx, req.ID, err.Error()); sendErr != nil {
			logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
		}
		return
	}

	if sendErr := connA.SendResult(s.ctx, req.ID, result); sendErr != nil {
		logger().Debug("rpc runtime: send result failed", "plugin", proc.Name(), "error", sendErr)
	}
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

	// Remove cache consumer tracking for this plugin.
	// UnregisterConsumer decrements pending counts for unacked entries
	// so they can be evicted instead of leaking.
	if proc.IsCacheConsumer() && s.reactor != nil {
		s.reactor.UnregisterCacheConsumer(proc.Name())
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

// --- BGP event delegation ---
// These methods delegate to BGPHooks when set.
// They are called by the reactor for BGP event delivery.

// OnMessageReceived handles raw BGP messages from peers.
// msg is bgptypes.RawMessage (typed as any to avoid BGP imports).
// Delegates to BGPHooks.OnMessageReceived when set.
// Returns the count of cache-consumer plugins that successfully received the event.
func (s *Server) OnMessageReceived(peer PeerInfo, msg any) int {
	if s.bgpHooks == nil || s.bgpHooks.OnMessageReceived == nil {
		return 0
	}
	if s.procManager == nil || s.subscriptions == nil {
		return 0
	}
	return s.bgpHooks.OnMessageReceived(s, peer, msg)
}

// OnPeerStateChange handles peer state transitions.
// Delegates to BGPHooks.OnPeerStateChange when set.
func (s *Server) OnPeerStateChange(peer PeerInfo, state string) {
	if s.bgpHooks == nil || s.bgpHooks.OnPeerStateChange == nil {
		return
	}
	if s.procManager == nil || s.subscriptions == nil {
		return
	}
	s.bgpHooks.OnPeerStateChange(s, peer, state)
}

// OnPeerNegotiated handles capability negotiation completion.
// neg is format.DecodedNegotiated (typed as any to avoid BGP imports).
// Delegates to BGPHooks.OnPeerNegotiated when set.
func (s *Server) OnPeerNegotiated(peer PeerInfo, neg any) {
	if s.bgpHooks == nil || s.bgpHooks.OnPeerNegotiated == nil {
		return
	}
	if s.procManager == nil || s.subscriptions == nil {
		return
	}
	s.bgpHooks.OnPeerNegotiated(s, peer, neg)
}

// OnMessageSent handles BGP messages sent to peers.
// msg is bgptypes.RawMessage (typed as any to avoid BGP imports).
// Delegates to BGPHooks.OnMessageSent when set.
func (s *Server) OnMessageSent(peer PeerInfo, msg any) {
	if s.bgpHooks == nil || s.bgpHooks.OnMessageSent == nil {
		return
	}
	if s.procManager == nil || s.subscriptions == nil {
		return
	}
	s.bgpHooks.OnMessageSent(s, peer, msg)
}

// BroadcastValidateOpen sends validate-open to all plugins that declared WantsValidateOpen.
// local and remote are *message.Open (typed as any to avoid BGP imports).
// Returns nil if all accept, or an OpenValidationError on first rejection.
func (s *Server) BroadcastValidateOpen(peerAddr string, local, remote any) error {
	if s.bgpHooks == nil || s.bgpHooks.BroadcastValidateOpen == nil {
		return nil
	}
	return s.bgpHooks.BroadcastValidateOpen(s, peerAddr, local, remote)
}

// EncodeNLRI encodes NLRI by routing to the appropriate family plugin via RPC.
// Returns error if no plugin registered or plugin not running.
func (s *Server) EncodeNLRI(family string, args []string) ([]byte, error) {
	if s.registry == nil || s.procManager == nil {
		return nil, fmt.Errorf("server not configured for plugins")
	}

	pluginName := s.registry.LookupFamily(family)
	if pluginName == "" {
		return nil, fmt.Errorf("no plugin registered for family %s", family)
	}

	proc := s.procManager.GetProcess(pluginName)
	if proc == nil {
		return nil, fmt.Errorf("plugin %s not running", pluginName)
	}

	connB := proc.ConnB()
	if connB == nil {
		return nil, fmt.Errorf("plugin %s connection closed", pluginName)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	hexStr, err := connB.SendEncodeNLRI(ctx, family, args)
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
// Returns the JSON representation of the decoded NLRI.
// Returns error if no plugin registered or plugin not running.
func (s *Server) DecodeNLRI(family, hexData string) (string, error) {
	if s.registry == nil || s.procManager == nil {
		return "", fmt.Errorf("server not configured for plugins")
	}

	pluginName := s.registry.LookupFamily(family)
	if pluginName == "" {
		return "", fmt.Errorf("no plugin registered for family %s", family)
	}

	proc := s.procManager.GetProcess(pluginName)
	if proc == nil {
		return "", fmt.Errorf("plugin %s not running", pluginName)
	}

	connB := proc.ConnB()
	if connB == nil {
		return "", fmt.Errorf("plugin %s connection closed", pluginName)
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	jsonResult, err := connB.SendDecodeNLRI(ctx, family, hexData)
	if err != nil {
		return "", fmt.Errorf("plugin request failed: %w", err)
	}

	return jsonResult, nil
}
