// Design: docs/architecture/api/process-protocol.md — plugin process management
// Detail: startup.go — 5-stage plugin startup protocol
// Detail: dispatch.go — command dispatch routing
// Detail: events.go — event delivery to plugins
// Detail: monitor.go — CLI monitor client management

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/ipc"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/core/syncutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// logger is the plugin server subsystem logger (lazy initialization).
// Controlled by ze.log.plugin.server environment variable.
var logger = slogutil.LazyLogger("plugin.server")

// Default stage timeout for plugin registration protocol.
// Each stage must complete within this duration.
// Override via ze.plugin.stage.timeout env var or per-plugin config timeout.
const defaultStageTimeout = 5 * time.Second

// Env var registration for stage timeout.
var _ = env.MustRegister(env.EnvEntry{Key: "ze.plugin.stage.timeout", Type: "duration", Default: "5s", Description: "Per-stage timeout for plugin registration protocol"})

// stageTimeoutFromEnv reads ze.plugin.stage.timeout and returns the parsed duration.
// Falls back to defaultStageTimeout on missing or invalid values.
func stageTimeoutFromEnv() time.Duration {
	return env.GetDuration("ze.plugin.stage.timeout", defaultStageTimeout)
}

// RPCParams is the standard params format for JSON RPC requests from socket clients.
// Handlers receive Args as positional arguments and Selector as the peer filter.
// Identity (Username) is never accepted from client JSON -- it MUST be injected by
// the transport layer (SSH session, plugin process manager, TLS auth).
type RPCParams struct {
	Selector string   `json:"selector,omitempty"` // Peer selector (optional)
	Args     []string `json:"args,omitempty"`     // Command arguments (optional)
}

// Server manages API connections and command dispatch.
type Server struct {
	config            *ServerConfig
	reactor           plugin.ReactorLifecycle
	dispatcher        *Dispatcher
	rpcDispatcher     *ipc.RPCDispatcher                            // Wire method dispatch for socket clients
	rpcHandlers       map[string]func(json.RawMessage) (any, error) // Lazily collected from registry
	rpcHandlersOnce   sync.Once
	commitManager     any
	procManager       atomic.Pointer[process.ProcessManager]
	spawner           plugin.ProcessSpawner   // PluginManager for process lifecycle
	subscriptions     *SubscriptionManager    // API-driven event subscriptions (plugin processes)
	engineSubscribers *engineEventSubscribers // Engine-side stream subscribers (orchestrator etc.)
	monitors          *MonitorManager         // CLI monitor subscriptions

	// Plugin registration protocol
	coordinator   *plugin.StartupCoordinator // Stage synchronization
	coordinatorMu sync.Mutex                 // Protects coordinator reads/writes
	registry      *plugin.PluginRegistry     // Command/capability registry
	capInjector   *plugin.CapabilityInjector // Capability injection for OPEN

	running atomic.Bool

	loadedPlugins   map[string]bool // tracks all plugins loaded across startup phases
	loadedPluginsMu sync.Mutex      // protects loadedPlugins

	startupDone     chan struct{} // closed when signalStartupComplete runs
	startupDoneOnce sync.Once
	startupErr      error // non-nil when a config-path plugin fails during startup

	configLoader ConfigLoader // Loads new config tree for ReloadFromDisk
	rebootFunc   func()       // Set by daemon; called on "daemon reboot" RPC

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	txLock txLock // Transaction exclusion (one config transaction at a time)
}

// wrapHandler adapts a Handler to an ipc.RPCHandler for the RPC dispatcher.
// Creates a CommandContext from the server state and extracts args from JSON params.
// The cliCommand and readOnly parameters enable authorization checks on the RPC path
// (same checks that Dispatch() applies on the text protocol path).
func (s *Server) wrapHandler(handler Handler, cliCommand string, readOnly bool) ipc.RPCHandler {
	return func(_ string, params json.RawMessage) (any, error) {
		ctx := &CommandContext{
			Server:         s,
			RequestContext: s.Context(),
			Peer:           "*",
		}

		var rpcParams RPCParams
		if len(params) > 0 {
			if err := json.Unmarshal(params, &rpcParams); err != nil {
				return nil, rpc.NewCodedError("invalid-params", fmt.Sprintf("invalid params: %v", err))
			}
		}

		if rpcParams.Selector != "" {
			ctx.Peer = rpcParams.Selector
		}
		// Username is NOT read from client params — identity must be injected
		// by the transport layer (see loader.go SSH wiring, dispatch.go plugin wiring).

		// Authorization check — same path as Dispatch() in command.go
		if s.dispatcher != nil && !s.dispatcher.isAuthorized(ctx, cliCommand, readOnly) {
			return nil, rpc.NewCodedError("unauthorized", "unauthorized")
		}

		resp, err := handler(ctx, rpcParams.Args)
		if err != nil {
			// Use CLI-facing command name, not internal plugin command name
			if errors.Is(err, ErrUnknownCommand) {
				return nil, rpc.NewCodedError("command-not-available",
					fmt.Sprintf("command %q not available (plugin may not be running)", cliCommand))
			}
			return nil, err
		}
		if resp == nil {
			return nil, nil
		}
		if resp.Status == plugin.StatusError {
			return nil, rpc.NewCodedError("handler-error", fmt.Sprintf("%v", resp.Data))
		}
		return resp.Data, nil
	}
}

// NewServer creates a new API server.
func NewServer(config *ServerConfig, reactor plugin.ReactorLifecycle) (*Server, error) {
	s := &Server{
		config:            config,
		reactor:           reactor,
		dispatcher:        NewDispatcher(),
		rpcDispatcher:     ipc.NewRPCDispatcher(),
		subscriptions:     NewSubscriptionManager(),
		engineSubscribers: newEngineEventSubscribers(),
		monitors:          NewMonitorManager(),
		registry:          plugin.NewPluginRegistry(),
		capInjector:       plugin.NewCapabilityInjector(),
		startupDone:       make(chan struct{}),
		loadedPlugins:     make(map[string]bool),
	}

	// Register plugin-declared event and send types before any subscriptions.
	plugin.RegisterPluginEventTypes()
	plugin.RegisterPluginSendTypes()

	// Build WireMethod -> CLI path mapping from shared YANG loader.
	loader, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("YANG command tree: %w", err)
	}
	wireToPath := yang.WireMethodToPath(loader)
	pathToDesc := yang.PathToDescription(loader)

	// Register core handlers (text dispatcher for plugin protocol)
	RegisterDefaultHandlers(s.dispatcher, wireToPath, pathToDesc)

	// Register all builtin RPCs with wire method dispatcher (for socket clients)
	for _, reg := range AllBuiltinRPCs() {
		if reg.Handler == nil {
			continue // Skip editor-internal RPCs with nil handlers
		}
		cliPath := wireToPath[reg.WireMethod] // YANG-derived CLI path for authz/errors
		if cliPath == "" {
			continue // Skip RPCs without YANG path (no authz possible)
		}
		if err := s.rpcDispatcher.Register(reg.WireMethod, s.wrapHandler(reg.Handler, cliPath, IsReadOnlyPath(cliPath))); err != nil {
			logger().Error("rpc dispatcher: registration failed", "method", reg.WireMethod, "error", err)
		}
	}

	return s, nil
}

// ConfigPath returns the path to the config file. Empty if not set.
func (s *Server) ConfigPath() string {
	if s.config == nil {
		return ""
	}
	return s.config.ConfigPath
}

// hasConfiguredPlugin returns true if a plugin with the given name is in the
// server's configured plugin list. Used by stage 1 dependency validation.
// hasConfiguredPlugin checks whether a plugin with the given registry name is
// already explicitly configured. Matches by config name OR by checking if the
// Run command invokes the plugin (e.g., config name "adj-rib-in" with
// Run "ze plugin bgp-adj-rib-in" matches registry name "bgp-adj-rib-in").
func (s *Server) hasConfiguredPlugin(name string) bool {
	if name == "" || s.config == nil {
		return false
	}
	for _, p := range s.config.Plugins {
		if p.Name == name {
			return true
		}
		// External plugins: config name may differ from registry name.
		// Check if the run command invokes this exact plugin name.
		// Use word-level matching to avoid "bgp-rib" falsely matching "bgp".
		if p.Run != "" && slices.Contains(strings.Fields(p.Run), name) {
			return true
		}
	}
	return false
}

// markPluginLoaded records that a plugin was loaded in a startup phase.
// Used to prevent re-loading across phases (each phase creates a new ProcessManager).
func (s *Server) markPluginLoaded(name string) {
	s.loadedPluginsMu.Lock()
	s.loadedPlugins[name] = true
	s.loadedPluginsMu.Unlock()
}

// isPluginLoaded returns true if a plugin was loaded in any previous startup phase.
func (s *Server) isPluginLoaded(name string) bool {
	s.loadedPluginsMu.Lock()
	defer s.loadedPluginsMu.Unlock()
	return s.loadedPlugins[name]
}

// HasProcesses returns true if any plugin processes were loaded during startup.
// Used by the main loop to decide whether to listen for server-done (all processes
// exited). Without this, configs with no plugins cause immediate daemon exit.
func (s *Server) HasProcesses() bool {
	s.loadedPluginsMu.Lock()
	defer s.loadedPluginsMu.Unlock()
	return len(s.loadedPlugins) > 0
}

// Context returns the server's context. Used by RPC handlers that need
// a cancellable context tied to the server's lifetime (e.g., coordinator reload).
func (s *Server) Context() context.Context {
	return s.ctx
}

// UpdateProtocolConfig sets protocol-specific auto-load configuration after the
// reactor has parsed settings. Called by the protocol plugin's RunEngine after
// creating the reactor, so that family/event/send auto-load phases have the data.
func (s *Server) UpdateProtocolConfig(families, customEvents, customSendTypes []string) {
	s.config.ConfiguredFamilies = families
	s.config.ConfiguredCustomEvents = customEvents
	s.config.ConfiguredCustomSendTypes = customSendTypes
}

// ReactorAny returns the reactor as any, satisfying registry.PluginServerAccessor.
func (s *Server) ReactorAny() any {
	return s.reactor
}

// ReactorFor returns a named protocol reactor from the Coordinator, or nil.
// This allows plugins to access non-BGP reactors (e.g., OSPF, IS-IS) by name.
func (s *Server) ReactorFor(name string) any {
	if c, ok := s.reactor.(*plugin.Coordinator); ok {
		return c.Reactor(name)
	}
	return nil
}

func (s *Server) Reactor() plugin.ReactorLifecycle {
	// When the reactor is a Coordinator, return the underlying reactor adapter
	// (which implements both ReactorLifecycle and BGPReactor) so that type
	// assertions to BGPReactor succeed.
	if c, ok := s.reactor.(*plugin.Coordinator); ok {
		return c.FullReactor()
	}
	return s.reactor
}

// Dispatcher returns the command dispatcher.
func (s *Server) Dispatcher() *Dispatcher {
	return s.dispatcher
}

// getRPCHandlers returns the collected RPC handlers, lazily initializing on first call.
// This allows handlers registered after server creation (e.g., from bgp/server init())
// to be included.
func (s *Server) getRPCHandlers() map[string]func(json.RawMessage) (any, error) {
	s.rpcHandlersOnce.Do(func() {
		if s.rpcHandlers == nil {
			s.rpcHandlers = registry.CollectRPCHandlers()
		}
	})
	return s.rpcHandlers
}

// CommitManager returns the commit manager.
func (s *Server) CommitManager() any {
	return s.commitManager
}

// SetCommitManager sets the commit manager. Called by the BGP plugin during
// configuration to inject a CommitManager created with BGP-specific types.
// MUST be called before any RPC dispatch (i.e., during init-time registration).
// NOT safe for concurrent use with CommitManager().
func (s *Server) SetCommitManager(cm any) {
	s.commitManager = cm
}

// Subscriptions returns the subscription manager.
func (s *Server) Subscriptions() *SubscriptionManager {
	return s.subscriptions
}

// Monitors returns the monitor manager for CLI monitor sessions.
func (s *Server) Monitors() *MonitorManager {
	return s.monitors
}

// ProcessManager returns the process manager.
// Used by BGP hook implementations to iterate plugin processes.
func (s *Server) ProcessManager() *process.ProcessManager {
	return s.procManager.Load()
}

// SetProcessSpawner sets the PluginManager as the process spawner.
// When set, runPluginPhase delegates process creation to the spawner
// instead of creating ProcessManager directly.
// Must be called before Start.
// If the spawner supports SetMetricsRegistry (e.g., PluginManager),
// the server's metrics registry is forwarded for plugin health metrics.
func (s *Server) SetProcessSpawner(sp plugin.ProcessSpawner) {
	s.spawner = sp
	// Thread metrics registry to the spawner if it supports it.
	if setter, ok := sp.(interface{ SetMetricsRegistry(any) }); ok && s.config.MetricsRegistry != nil {
		setter.SetMetricsRegistry(s.config.MetricsRegistry)
	}
}

// CallFilterUpdate sends a filter-update RPC to a named plugin and returns the response.
// Returns an error if the plugin is not found, not connected, or the RPC fails.
func (s *Server) CallFilterUpdate(ctx context.Context, pluginName string, input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	pm := s.procManager.Load()
	if pm == nil {
		return nil, fmt.Errorf("filter-update: no process manager")
	}
	proc := pm.GetProcess(pluginName)
	if proc == nil {
		return nil, fmt.Errorf("filter-update: unknown plugin %q", pluginName)
	}
	conn := proc.Conn()
	if conn == nil {
		return nil, fmt.Errorf("filter-update: plugin %q not connected", pluginName)
	}
	return conn.SendFilterUpdate(ctx, input)
}

// FilterOnError returns the declared on-error mode for a named filter.
// Returns rpc.OnErrorReject (fail-closed) if the plugin or filter is not found.
func (s *Server) FilterOnError(pluginName, filterName string) rpc.OnErrorPolicy {
	pm := s.procManager.Load()
	if pm == nil {
		return rpc.OnErrorReject
	}
	proc := pm.GetProcess(pluginName)
	if proc == nil {
		return rpc.OnErrorReject
	}
	reg := proc.Registration()
	if reg == nil {
		return rpc.OnErrorReject
	}
	for _, f := range reg.Filters {
		if f.Name == filterName {
			if f.OnError == rpc.OnErrorAccept {
				return rpc.OnErrorAccept
			}
			return rpc.OnErrorReject
		}
	}
	return rpc.OnErrorReject
}

// FilterInfo returns declaration info for a named filter: declared attributes and raw flag.
// Returns nil attributes and false if the plugin or filter is not found.
func (s *Server) FilterInfo(pluginName, filterName string) (declaredAttrs []string, raw bool) {
	pm := s.procManager.Load()
	if pm == nil {
		return nil, false
	}
	proc := pm.GetProcess(pluginName)
	if proc == nil {
		return nil, false
	}
	reg := proc.Registration()
	if reg == nil {
		return nil, false
	}
	for _, f := range reg.Filters {
		if f.Name == filterName {
			return f.Attributes, f.Raw
		}
	}
	return nil, false
}

// Running returns true if the server is running.
func (s *Server) Running() bool {
	return s.running.Load()
}

// Start begins accepting connections.
func (s *Server) Start() error {
	return s.StartWithContext(context.Background())
}

// StartWithContext begins accepting connections with the given context.
// External access is via SSH; the plugin server handles only in-process dispatch.
func (s *Server) StartWithContext(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running.Store(true)

	// Start plugin phases asynchronously (non-blocking)
	// Phase 1: Explicit plugins
	// Phase 2: Auto-load plugins for config paths (ConfigRoots matching)
	// Phase 3: Auto-load plugins for unclaimed families
	// Phase 4: Auto-load plugins for custom event types (e.g., update-rpki)
	// Phase 5: Auto-load plugins for custom send types (e.g., enhanced-refresh)
	if len(s.config.Plugins) > 0 || len(s.config.ConfiguredPaths) > 0 || len(s.config.ConfiguredFamilies) > 0 || len(s.config.ConfiguredCustomEvents) > 0 || len(s.config.ConfiguredCustomSendTypes) > 0 {
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

// Stop signals the server to stop and cleans up resources.
func (s *Server) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.cleanup()
}

// Wait waits for the server to stop.
func (s *Server) Wait(ctx context.Context) error {
	return syncutil.WaitGroupWait(ctx, &s.wg)
}

// cleanup stops processes.
func (s *Server) cleanup() {
	s.running.Store(false)

	// Stop processes
	if pm := s.procManager.Load(); pm != nil {
		pm.Stop()
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

// AllPluginCapabilities returns all stored capabilities (global + all per-peer).
// Used by the restart handler to compute max restart-time for the GR marker.
func (s *Server) AllPluginCapabilities() []plugin.InjectedCapability {
	if s.capInjector == nil {
		return nil
	}
	return s.capInjector.AllCapabilities()
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
	pm := s.procManager.Load()
	if pm == nil {
		return nil
	}

	var declarations []plugin.SchemaDeclaration
	for _, proc := range pm.AllProcesses() {
		reg := proc.Registration()
		declarations = append(declarations, reg.SchemaDeclarations...)
	}
	return declarations
}
