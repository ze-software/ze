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

	"github.com/ze-software/ze/internal/component/config/yang"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/ipc"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/syncutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errFilterUpdateNoProcessManager = errors.New("filter-update: no process manager")

// logger is the plugin server subsystem logger (lazy initialization).
// Controlled by ze.log.plugin.server environment variable.
var logger = slogutil.LazyLogger("plugin.server")

// Default stall timeout for the plugin registration protocol.
//
// This is NOT a budget for how long a stage may take. It bounds how long the
// whole tier may go with NO plugin completing a stage: any completion restarts
// the window (see StartupCoordinator.WaitForStageProgress). A tier of 20+
// plugins on a loaded host takes far longer than this in total and must still
// start; only a genuinely wedged plugin should trip it.
//
// Override via ze.plugin.stage.timeout env var or per-plugin config timeout.
const defaultStageTimeout = 5 * time.Second

// Env var registration for stage stall timeout.
var _ = env.MustRegister(env.EnvEntry{Key: "ze.plugin.stage.timeout", Type: "duration", Default: "5s", Description: "Plugin registration protocol: how long a startup stage may stall with no plugin making progress"})

// stageTimeoutFromEnv reads ze.plugin.stage.timeout and returns the parsed duration.
// Falls back to defaultStageTimeout on missing or invalid values.
func stageTimeoutFromEnv() time.Duration {
	return env.GetDuration("ze.plugin.stage.timeout", defaultStageTimeout)
}

// rpcParams is the standard params format for JSON RPC requests from socket clients.
// Handlers receive Args as positional arguments and Selector as the peer filter.
// Identity (Username) is never accepted from client JSON; it must be injected by
// the transport layer (SSH session, plugin process manager, TLS auth).
type rpcParams struct {
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
	spawner           plugin.ProcessSpawner         // PluginManager for process lifecycle
	subscriptions     *SubscriptionManager          // API-driven event subscriptions (plugin processes)
	deliveryGraph     atomic.Pointer[DeliveryGraph] // config-derived peer-to-process edges (delivery_graph.go)
	engineSubscribers *engineEventSubscribers       // Engine-side stream subscribers (orchestrator etc.)
	monitors          *MonitorManager               // CLI monitor subscriptions
	quiescers         QuiescerRegistry              // Subsystem drains invoked by `request quiesce`

	// Plugin registration protocol
	coordinator       *plugin.StartupCoordinator             // Stage synchronization
	coordinatorMu     sync.Mutex                             // Protects coordinator reads/writes
	registry          *plugin.PluginRegistry                 // Command/capability registry
	capInjector       *plugin.CapabilityInjector             // Capability injection for OPEN
	runtimeFamilies   map[string][]family.FamilyRegistration // dynamic families committed by each plugin
	runtimeFamiliesMu sync.Mutex

	eventRing *EventRing // Global event history ring (diag-2)

	running atomic.Bool

	loadedPlugins   map[string]bool // tracks all plugins loaded across startup phases
	loadedPluginsMu sync.Mutex      // protects loadedPlugins

	// advertisedClaims records every exclusive-role token the engine told a
	// plugin was claimed (Stage-2 configure), mapped to the claimants it was
	// derived from. Read back in signalStartupComplete to prove each claimant
	// actually reached Running -- see startup_claims.go.
	advertisedClaims   map[string]map[string]bool
	advertisedClaimsMu sync.Mutex

	startupDone     chan struct{} // closed when signalStartupComplete runs
	startupDoneOnce sync.Once
	startupErr      error // non-nil when a config-path plugin fails during startup

	configLoader          ConfigLoader   // Loads new config tree for ReloadFromDisk
	fullReload            FullReloadFunc // Runs hub-level reload for daemon-reload RPC
	rebootFunc            func()         // Set by daemon; called on "daemon reboot" RPC
	shutdownFunc          func()         // Set by daemon; reactor-independent daemon shutdown (used when no BGP reactor is present)
	shutdownRequested     chan struct{}
	shutdownRequestedOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	txLock txLock // Transaction exclusion (one config transaction at a time)

	// reloadGen is the monotonic "reload processed" fence surfaced by
	// `show reload-status`. Advanced by MarkReloadProcessed once a whole
	// reload sequence completes, applied or rejected. See reload_generation.go.
	reloadGen reloadGeneration

	// Forked route-installing plugins (OSPF, IS-IS) insert into the engine Loc-RIB
	// via the route-install RPC. installedByPlugin tracks each plugin's live routes
	// (keyed by plugin name) so a disconnect withdraws them (AC-8: no stale routes
	// when a forked plugin dies without withdrawing). Lazily initialized.
	routeMu           sync.Mutex
	installedByPlugin map[string]map[routeKey]struct{}
	routeMetricOnce   sync.Once
	routeInstallRPCs  metrics.CounterVec // ze_route_install_rpc_total{plugin,op,result}
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
			// Inject the reserved trusted internal identity (spec-fixit-authz-admin-
			// fallthrough O-4). Store.Authorize now fails closed on an empty username,
			// so without this every RPC method would be denied on a box with
			// authorization configured. The reserved prefix is un-typeable, so no
			// authenticated client can present it.
			Username: internalRPCIdentity,
			// Sender stays unset, and a command that puts a message on a peer's
			// wire is refused here by name (send_permission.go). An RPCHandler is
			// handed the method and the params and no caller identity, so this
			// path cannot say whether a process or an operator called it. Nothing
			// dispatches through s.rpcDispatcher today. Whoever wires it must
			// carry the caller down to here and state the sender.
		}

		var rpcParams rpcParams
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
			// The code stays "unauthorized" (wire contract); only the human
			// text carries the operator-facing wording.
			return nil, rpc.NewCodedError("unauthorized", plugin.UnauthorizedMessage)
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
		subscriptions:     newSubscriptionManager(),
		engineSubscribers: newEngineEventSubscribers(),
		monitors:          NewMonitorManager(),
		registry:          plugin.NewPluginRegistry(),
		capInjector:       plugin.NewCapabilityInjector(),
		eventRing:         NewEventRing(defaultEventRingCapacity),
		startupDone:       make(chan struct{}),
		shutdownRequested: make(chan struct{}),
		loadedPlugins:     make(map[string]bool),
		runtimeFamilies:   make(map[string][]family.FamilyRegistration),
	}

	// Register the reactor's forward pool as a quiescer so `request quiesce`
	// drains queued routes to peer sockets (see quiesce.go).
	registerReactorQuiescer(s, reactor)

	// Wire the plugin write-watchdog counter. When a write on a non-deadline
	// transport (SSH channel, io.Pipe) stalls past the watchdog window, the rpc
	// layer closes the connection (fail-fast) and invokes this hook. The hook
	// is a process-wide package global in rpc, so the last server with a
	// registry wins -- fine, since production runs a single plugin server.
	if config != nil && config.MetricsRegistry != nil {
		wd := config.MetricsRegistry.CounterVec(
			"ze_plugin_write_watchdog_total",
			"plugin RPC connections closed because a write stalled past the write-watchdog window",
			[]string{"transport"})
		rpc.SetWriteWatchdogHook(func(transport, _ string) {
			wd.With(transport).Inc()
		})
	}

	// Register plugin-declared event and send types before any subscriptions.
	plugin.RegisterPluginEventTypes()
	plugin.RegisterPluginSendTypes()

	// Build WireMethod -> CLI path mapping from shared YANG loader.
	loader, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("YANG command tree: %w", err)
	}
	wireToPaths := yang.WireMethodToPaths(loader)
	wireToPath := yang.WireMethodToPath(loader)
	pathToDesc := yang.PathToDescription(loader)
	pathToArgDefs := yang.PathToArgDefs(loader)

	// Register core handlers (text dispatcher for plugin protocol),
	// including all YANG command aliases.
	cmdTree := yang.BuildCommandTree(loader)
	loadBuiltinsWithAliases(s.dispatcher, wireToPaths, pathToDesc, pathToArgDefs, cmdTree)

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

// EventRing returns the global event history ring for CLI queries.
func (s *Server) EventRing() *EventRing {
	return s.eventRing
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
	if setter, ok := sp.(interface{ SetMetricsRegistry(metrics.Registry) }); ok && s.config.MetricsRegistry != nil {
		setter.SetMetricsRegistry(s.config.MetricsRegistry)
	}
}

// SetMetricsRegistry forwards a registry that becomes available after the
// server and its process spawner were created. BGP creates the registry during
// plugin configuration, after external processes have already been spawned.
func (s *Server) SetMetricsRegistry(reg metrics.Registry) {
	if s.config != nil {
		s.config.MetricsRegistry = reg
	}
	if setter, ok := s.spawner.(interface{ SetMetricsRegistry(metrics.Registry) }); ok && reg != nil {
		setter.SetMetricsRegistry(reg)
	}
}

// CallFilterUpdate sends a filter-update RPC to a named plugin and returns the response.
// Returns an error if the plugin is not found, not connected, or the RPC fails.
func (s *Server) CallFilterUpdate(ctx context.Context, pluginName string, input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	pm := s.procManager.Load()
	if pm == nil {
		return nil, errFilterUpdateNoProcessManager
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

// CallDoctorCheck invokes a plugin's doctor check callback and returns diagnostics.
func (s *Server) CallDoctorCheck(ctx context.Context, pluginName, checkName string) (*rpc.DoctorCheckOutput, error) {
	pm := s.procManager.Load()
	if pm == nil {
		return nil, fmt.Errorf("doctor-check: no process manager")
	}
	proc := pm.GetProcess(pluginName)
	if proc == nil {
		return nil, fmt.Errorf("doctor-check: unknown plugin %q", pluginName)
	}
	conn := proc.Conn()
	if conn == nil {
		return nil, fmt.Errorf("doctor-check: plugin %q not connected", pluginName)
	}
	return conn.SendDoctorCheck(ctx, checkName)
}

// DoctorCheckPlugins returns plugin names and their doctor check registrations.
func (s *Server) DoctorCheckPlugins() map[string][]plugin.DoctorCheckRegistration {
	pm := s.procManager.Load()
	if pm == nil {
		return nil
	}
	result := make(map[string][]plugin.DoctorCheckRegistration)
	for _, proc := range pm.AllProcesses() {
		reg := proc.Registration()
		if reg == nil || len(reg.DoctorChecks) == 0 {
			continue
		}
		checks := make([]plugin.DoctorCheckRegistration, len(reg.DoctorChecks))
		copy(checks, reg.DoctorChecks)
		result[proc.Name()] = checks
	}
	return result
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
	var wildcard *plugin.FilterRegistration
	for i := range reg.Filters {
		f := &reg.Filters[i]
		if f.Name == filterName {
			if f.OnError == rpc.OnErrorAccept {
				return rpc.OnErrorAccept
			}
			return rpc.OnErrorReject
		}
		if f.Name == "*" {
			wildcard = f
		}
	}
	if wildcard != nil && wildcard.OnError == rpc.OnErrorAccept {
		return rpc.OnErrorAccept
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
	var wildcard *plugin.FilterRegistration
	for i := range reg.Filters {
		f := &reg.Filters[i]
		if f.Name == filterName {
			return f.Attributes, f.Raw
		}
		if f.Name == "*" {
			wildcard = f
		}
	}
	if wildcard != nil {
		return wildcard.Attributes, wildcard.Raw
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

// replyContext carries a RESPONSE to a request the engine has already run. It
// is s.ctx with Stop cancellation removed because a reply and the next request
// are owed different things: reading the next request stops with the server,
// while an admitted reply must still be attempted.
//
// Daemon termination commands defer their lifecycle action until the response
// transport completes, but parent cancellation or another concurrent stop can
// still cancel s.ctx between handler completion and the write. Without this
// context split, writeAppended (pkg/plugin/rpc/conn.go) returns ctx.Err()
// without writing the response. The write remains bounded by
// defaultWriteDeadline, or by the socket error when the plugin is gone.
func (s *Server) replyContext() context.Context {
	return context.WithoutCancel(s.ctx)
}

func (s *Server) signalShutdownRequested() {
	s.shutdownRequestedOnce.Do(func() {
		if s.shutdownRequested != nil {
			close(s.shutdownRequested)
		}
	})
}

// Stop signals the server to stop and cleans up resources.
//
// An in-flight config transaction is stood down FIRST, and given a bounded
// moment to unwind (stopTransaction, reload.go). cleanup closes every plugin
// connection, and the transaction is still using them. Closed under it, the
// bridge reads each connection as a crashed plugin. That elects a rollback,
// which restarts plugins the next line is about to kill.
func (s *Server) Stop() {
	s.running.Store(false)
	s.stopTransaction(txShutdownGrace)
	if s.cancel != nil {
		s.cancel()
	}
	s.cleanup()
}

// Wait reports an accepted explicit shutdown request after its response
// boundary, so the daemon can call Stop without closing the requesting
// connection too early. After direct Stop or parent cancellation, it drains
// runtime handlers. An empty runtime plugin set does not stop the server.
func (s *Server) Wait(ctx context.Context) error {
	var serverDone <-chan struct{}
	if s.ctx != nil {
		serverDone = s.ctx.Done()
	}
	if serverDone == nil && s.shutdownRequested == nil {
		return syncutil.WaitGroupWait(ctx, &s.wg)
	}
	if !s.running.Load() {
		return syncutil.WaitGroupWait(ctx, &s.wg)
	}
	select {
	case <-s.shutdownRequested:
		return nil
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.shutdownRequested:
		return nil
	case <-serverDone:
		select {
		case <-s.shutdownRequested:
			return nil
		default:
		}
		return syncutil.WaitGroupWait(ctx, &s.wg)
	}
}

// cleanup stops processes.
func (s *Server) cleanup() {
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

// GetPluginCapabilitiesForSelectors returns plugin-declared capabilities for one
// peer that several selectors can name, resolved in the order given: for each
// capability code the first selector that declares it wins, then the globals.
//
// Callers MUST prefer this over probing selectors one at a time. Every answer
// carries the global capabilities, so a caller that stops at the first non-empty
// result stops at the globals and never reaches its later selectors.
func (s *Server) GetPluginCapabilitiesForSelectors(selectors ...string) []plugin.InjectedCapability {
	if s.capInjector == nil {
		return nil
	}
	return s.capInjector.GetCapabilitiesForSelectors(selectors...)
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
