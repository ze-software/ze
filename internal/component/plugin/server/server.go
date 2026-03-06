// Design: docs/architecture/api/process-protocol.md — plugin process management
// Detail: startup.go — 5-stage plugin startup protocol
// Detail: client.go — API client connections
// Detail: dispatch.go — command dispatch routing
// Detail: events.go — event delivery to plugins

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/core/ipc"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/syncutil"
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
	reactor       plugin.ReactorLifecycle
	dispatcher    *Dispatcher
	rpcDispatcher *ipc.RPCDispatcher // Wire method dispatch for socket clients
	rpcFallback   func(string) func(json.RawMessage) (any, error)
	commitManager any
	procManager   *process.ProcessManager
	subscriptions *SubscriptionManager // API-driven event subscriptions

	// Plugin registration protocol
	coordinator *plugin.StartupCoordinator // Stage synchronization
	registry    *plugin.PluginRegistry     // Command/capability registry
	capInjector *plugin.CapabilityInjector // Capability injection for OPEN

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
		if resp.Status == plugin.StatusError {
			return nil, fmt.Errorf("%v", resp.Data)
		}
		return resp.Data, nil
	}
}

// NewServer creates a new API server.
func NewServer(config *ServerConfig, reactor plugin.ReactorLifecycle) *Server {
	s := &Server{
		config:        config,
		reactor:       reactor,
		dispatcher:    NewDispatcher(),
		rpcDispatcher: ipc.NewRPCDispatcher(),
		rpcFallback:   config.RPCFallback,
		commitManager: config.CommitManager,
		subscriptions: NewSubscriptionManager(),
		registry:      plugin.NewPluginRegistry(),
		capInjector:   plugin.NewCapabilityInjector(),
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

// hasConfiguredPlugin returns true if a plugin with the given name is in the
// server's configured plugin list. Used by stage 1 dependency validation.
func (s *Server) hasConfiguredPlugin(name string) bool {
	for _, p := range s.config.Plugins {
		if p.Name == name {
			return true
		}
	}
	return false
}

// Context returns the server's context. Used by RPC handlers that need
// a cancellable context tied to the server's lifetime (e.g., coordinator reload).
func (s *Server) Context() context.Context {
	return s.ctx
}

// Reactor returns the reactor lifecycle interface.
func (s *Server) Reactor() plugin.ReactorLifecycle {
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
func (s *Server) ProcessManager() *process.ProcessManager {
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

		// Restrict socket permissions to owner only (CWE-732: no world access)
		if err := os.Chmod(s.config.SocketPath, 0o600); err != nil {
			listener.Close() //nolint:errcheck,gosec // cleanup on chmod failure
			return fmt.Errorf("set socket permissions: %w", err)
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
		// No plugins to start -- signal immediately so WaitForPluginStartupComplete
		// does not block. SetAPIProcessCount always creates the startupComplete
		// channel, but without runPluginStartup nothing would close it.
		s.signalStartupComplete()
	}

	return nil
}

// Stop signals the server to stop.
func (s *Server) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Wait waits for the server to stop.
func (s *Server) Wait(ctx context.Context) error {
	return syncutil.WaitGroupWait(ctx, &s.wg)
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
		default: // non-blocking shutdown check
		}

		// Accept with timeout to check for shutdown
		if ul, ok := s.listener.(*net.UnixListener); ok {
			if err := ul.SetDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
				logger().Debug("accept loop: set deadline failed", "error", err)
			}
		}

		conn, err := s.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.ctx.Done():
				return
			default: // transient error, retry accept
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
		if err := s.listener.Close(); err != nil {
			logger().Debug("cleanup: close listener", "error", err)
		}
	}

	// Close all clients
	s.mu.Lock()
	for _, client := range s.clients {
		client.cancel()
		if err := client.conn.Close(); err != nil {
			logger().Debug("cleanup: close client", "id", client.id, "error", err)
		}
	}
	s.clients = make(map[string]*Client)
	s.mu.Unlock()

	// Remove socket file
	if s.config.SocketPath != "" {
		if err := os.Remove(s.config.SocketPath); err != nil && !os.IsNotExist(err) {
			logger().Debug("cleanup: remove socket", "path", s.config.SocketPath, "error", err)
		}
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

// GetPluginCapabilitiesForPeer returns plugin-declared capabilities for a specific peer.
// Returns global capabilities plus any peer-specific capabilities (per-peer takes precedence).
func (s *Server) GetPluginCapabilitiesForPeer(peerAddr string) []plugin.InjectedCapability {
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
func (s *Server) GetSchemaDeclarations() []plugin.SchemaDeclaration {
	if s.procManager == nil {
		return nil
	}

	var declarations []plugin.SchemaDeclaration
	for _, proc := range s.procManager.AllProcesses() {
		reg := proc.Registration()
		declarations = append(declarations, reg.SchemaDeclarations...)
	}
	return declarations
}
