// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: startup_driver.go — shared 5-stage handshake driver (hubStartupSink)
// Related: ../acceptor.go — NewHubAcceptor, the shared TLS acceptor the hub injects here

package server

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var errSubsystemConnectionClosedBeforeProtocol = errors.New("subsystem connection closed before protocol")

// frozenSubsystems holds an immutable snapshot of the SubsystemManager's handler
// map. Created by Freeze() after startup, used by Get() and FindHandler()
// on the hot path to avoid RLock.
type frozenSubsystems struct {
	handlers map[string]*SubsystemHandler
}

// ErrSubsystemNotRunning is returned when a command targets a non-running subsystem.
var ErrSubsystemNotRunning = errors.New("subsystem not running")

// ErrSubsystemConnectionClosed is returned when the subsystem's connection is no longer available.
var ErrSubsystemConnectionClosed = errors.New("subsystem connection closed")

// SubsystemConfig describes a forked subsystem process.
type SubsystemConfig struct {
	Name       string   // Subsystem name (cache, route, session)
	Binary     string   // Path to binary or full command
	Commands   []string // Commands this subsystem handles (for pre-registration)
	ConfigPath string   // Config file path (passed to child process)
}

// SubsystemHandler wraps a forked process that handles a subset of commands.
// It spawns the subprocess, completes the 5-stage protocol, and routes
// commands to it via pipes.
//
// A subsystem is ALWAYS an external fork (Start builds a PluginConfig with no
// Internal flag), so Start requires a TLS acceptor for the child's connect-back.
// Callers MUST call SetAcceptor before Start; SubsystemManager does this for
// every handler it owns.
type SubsystemHandler struct {
	config   SubsystemConfig
	proc     *process.Process
	acceptor *pluginipc.PluginAcceptor // TLS connect-back listener; MUST be set before Start
	commands []string                  // Commands declared during Stage 1
	schema   *plugin.PluginSchemaDecl  // YANG schema declared during Stage 1
	mu       sync.RWMutex
}

// NewSubsystemHandler creates a handler backed by a forked process.
//
// The returned handler has no TLS acceptor: the caller MUST call SetAcceptor
// before Start, or Start fails closed. Prefer SubsystemManager.NewHandler,
// which injects the manager's acceptor.
func NewSubsystemHandler(config SubsystemConfig) *SubsystemHandler {
	return &SubsystemHandler{
		config: config,
	}
}

// SetAcceptor sets the TLS acceptor the forked subsystem connects back to.
// MUST be called before Start.
func (h *SubsystemHandler) SetAcceptor(a *pluginipc.PluginAcceptor) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.acceptor = a
}

// Name returns the subsystem name.
func (h *SubsystemHandler) Name() string {
	return h.config.Name
}

// Commands returns the commands this subsystem handles.
func (h *SubsystemHandler) Commands() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]string, len(h.commands))
	copy(result, h.commands)
	return result
}

// Schema returns the YANG schema declared by this subsystem, or nil if none.
func (h *SubsystemHandler) Schema() *plugin.PluginSchemaDecl {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.schema
}

// Start spawns the subsystem process and completes the 5-stage protocol.
func (h *SubsystemHandler) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Fail closed: a subsystem is always an external fork, and an external
	// plugin with no acceptor has nowhere to connect back to. Refuse here, where
	// the subsystem is still nameable, rather than letting the process layer
	// report a bare plugin name (ai/rules/evidence.md).
	//
	// The wording is operator-facing on purpose: this error reaches the console
	// through cmd/ze/hub/main.go, and a reader there cannot act on Go symbols.
	// For maintainers: the owner must build one with plugin.NewHubAcceptor and
	// install it via SubsystemManager.SetAcceptor before any Start. The
	// "no TLS acceptor configured" phrase is deliberately shared with
	// process.startExternal so one log/test needle catches either layer.
	if h.acceptor == nil {
		return fmt.Errorf("start subsystem %s: no TLS acceptor configured; "+
			"the hub did not open the plugin connect-back listener before forking -- "+
			"this is an internal wiring fault, not a config error; "+
			"restart ze and report it with the config that triggered it", h.config.Name)
	}

	// Build command:
	// - If Binary contains spaces (full command), use as-is
	// - Otherwise, add --mode=<name>
	var tb textbuf.Buffer
	tb.Str(h.config.Binary)
	if !strings.Contains(h.config.Binary, " ") {
		tb.Str(" --mode=").Str(h.config.Name)
	}

	// Append config path if provided
	if h.config.ConfigPath != "" {
		tb.Str(" --config ").Str(h.config.ConfigPath)
	}
	cmd := tb.String()

	// Create process config
	procConfig := plugin.PluginConfig{
		Name: tb.Reset().Str("subsystem-").Str(h.config.Name).String(),
		Run:  cmd,
	}

	h.proc = process.NewProcess(procConfig)
	h.proc.SetAcceptor(h.acceptor)
	if err := h.proc.StartWithContext(ctx); err != nil {
		return fmt.Errorf("start subsystem %s: %w", h.config.Name, err)
	}

	// Complete 5-stage protocol with timeout
	stageCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := h.completeProtocol(stageCtx); err != nil {
		h.proc.Stop()
		return fmt.Errorf("subsystem %s protocol: %w", h.config.Name, err)
	}

	return nil
}

// completeProtocol runs the shared 5-stage startup protocol with the subsystem
// process. It initializes the connection, then drives runStartupHandshake with a
// hubStartupSink that harvests the plugin's declared commands and schema,
// delivers nil config and nil registry, and runs with no barrier.
func (h *SubsystemHandler) completeProtocol(ctx context.Context) error {
	// Initialize connections from raw sockets (creates PluginConn wrappers).
	if err := h.proc.InitConns(); err != nil {
		return fmt.Errorf("init connections: %w", err)
	}
	if h.proc.Conn() == nil {
		return errSubsystemConnectionClosedBeforeProtocol
	}
	return runStartupHandshake(ctx, &hubStartupSink{h: h})
}

// hubStartupSink is the minimal startup sink for forked hub subsystems. The hub
// orchestrator has no reactor, capability injector, dispatcher command registry
// or config tree, so it only harvests the plugin's declared commands (for command
// routing via FindHandler) and YANG schema (for RegisterSchemas), delivers nil
// config and nil registry, and runs a single connection with no barrier.
type hubStartupSink struct {
	h *SubsystemHandler
}

func (hs *hubStartupSink) conn() *pluginipc.PluginConn { return hs.h.proc.Conn() }

// onRegistration harvests the declared commands and YANG schema into the
// handler-local fields that FindHandler and RegisterSchemas read.
func (hs *hubStartupSink) onRegistration(input *rpc.DeclareRegistrationInput) error {
	h := hs.h
	for _, cmd := range input.Commands {
		h.commands = append(h.commands, cmd.Name)
	}
	if input.Schema != nil {
		if h.schema == nil {
			h.schema = &plugin.PluginSchemaDecl{}
		}
		h.schema.Yang = input.Schema.YANGText
		h.schema.Module = input.Schema.Module
		h.schema.Namespace = input.Schema.Namespace
		h.schema.Handlers = input.Schema.Handlers
	}
	return nil
}

// deliverConfig delivers nil config: the hub has no config tree to share.
func (hs *hubStartupSink) deliverConfig(ctx context.Context) error {
	if err := hs.h.proc.Conn().SendConfigure(ctx, nil, nil); err != nil {
		return fmt.Errorf("stage 2 configure: %w", err)
	}
	return nil
}

// onCapabilities is a no-op: the hub has no capability injector.
func (hs *hubStartupSink) onCapabilities(*rpc.DeclareCapabilitiesInput) error { return nil }

// deliverRegistry delivers nil registry: the hub has no command registry to share.
func (hs *hubStartupSink) deliverRegistry(ctx context.Context) error {
	if err := hs.h.proc.Conn().SendShareRegistry(ctx, nil); err != nil {
		return fmt.Errorf("stage 4 share-registry: %w", err)
	}
	return nil
}

// onReady, onRunning, and postReady are no-ops: the hub registers no
// subscriptions, wires no bridge, and signals no reactor.
func (hs *hubStartupSink) onReady(*rpc.ReadyInput) error { return nil }
func (hs *hubStartupSink) onRunning()                    {}
func (hs *hubStartupSink) postReady(*rpc.ReadyInput)     {}

// transition is an unconditional success: the hub runs one connection at a time
// with no barrier.
func (hs *hubStartupSink) transition(_, _ plugin.PluginStage) bool { return true }

// Signal sends an OS signal to the subsystem's external process.
// Returns an error if the process is not running or is internal (goroutine).
func (h *SubsystemHandler) Signal(sig os.Signal) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.proc == nil || !h.proc.Running() {
		return fmt.Errorf("subsystem %s: not running", h.config.Name)
	}
	if h.proc.Cmd() == nil || h.proc.Cmd().Process == nil {
		return fmt.Errorf("subsystem %s: internal plugin, cannot signal", h.config.Name)
	}
	return h.proc.Cmd().Process.Signal(sig)
}

// Stop terminates the subsystem process.
func (h *SubsystemHandler) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.proc != nil {
		h.proc.SendShutdown()
		h.proc.Stop()
	}
}

// Running returns true if the subsystem process is running.
func (h *SubsystemHandler) Running() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.proc != nil && h.proc.Running()
}

// Handle sends a command to the subsystem via RPC and returns the response.
func (h *SubsystemHandler) Handle(ctx context.Context, command string) (*plugin.Response, error) {
	h.mu.RLock()
	proc := h.proc
	h.mu.RUnlock()

	if proc == nil || !proc.Running() {
		return nil, ErrSubsystemNotRunning
	}

	// Send command via RPC execute-command
	conn := proc.Conn()
	if conn == nil {
		return nil, ErrSubsystemConnectionClosed
	}
	out, err := conn.SendExecuteCommand(ctx, "", command, nil, "")
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: err.Error()}, err
	}

	if out.Status == plugin.StatusError {
		return &plugin.Response{Status: plugin.StatusError, Error: string(out.Data)}, nil
	}
	return &plugin.Response{Status: out.Status, Data: plugin.RawJSON(out.Data)}, nil
}

// SubsystemManager manages multiple subsystem handlers.
type SubsystemManager struct {
	handlers map[string]*SubsystemHandler

	// acceptor is the TLS connect-back listener every forked subsystem shares.
	// The owner (hub Orchestrator) creates it with plugin.NewHubAcceptor and
	// installs it via SetAcceptor; the manager only hands it to handlers.
	acceptor *pluginipc.PluginAcceptor

	// frozen holds an immutable snapshot for lock-free Get/FindHandler after startup.
	// nil before Freeze() is called.
	frozen atomic.Pointer[frozenSubsystems]

	mu sync.RWMutex
}

// NewSubsystemManager creates a new subsystem manager.
func NewSubsystemManager() *SubsystemManager {
	return &SubsystemManager{
		handlers: make(map[string]*SubsystemHandler),
	}
}

// SetAcceptor installs the TLS acceptor every forked subsystem connects back
// to, and back-fills handlers registered before it existed (the orchestrator
// registers subsystems at construction but can only bind a listener at Start).
// MUST be called before any handler is started.
//
// The manager does NOT own the acceptor's lifecycle: the caller that created it
// MUST Stop it.
func (m *SubsystemManager) SetAcceptor(a *pluginipc.PluginAcceptor) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.acceptor = a
	for _, handler := range m.handlers {
		handler.SetAcceptor(a)
	}
}

// NewHandler builds a handler already wired to the manager's acceptor, for
// callers that must start a replacement before publishing it (hub reload).
// Use this rather than the bare NewSubsystemHandler, which leaves the handler
// with no acceptor and therefore unable to start.
func (m *SubsystemManager) NewHandler(config SubsystemConfig) *SubsystemHandler {
	m.mu.RLock()
	acceptor := m.acceptor
	m.mu.RUnlock()

	handler := NewSubsystemHandler(config)
	handler.SetAcceptor(acceptor)
	return handler
}

// Register adds a subsystem configuration.
func (m *SubsystemManager) Register(config SubsystemConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	handler := NewSubsystemHandler(config)
	handler.SetAcceptor(m.acceptor)
	m.handlers[config.Name] = handler
	m.publishFrozenLocked(false)
}

// Replace swaps in an already-constructed handler and stops the previous one
// after publishing the replacement. Used by reload paths that pre-start a
// replacement before disrupting the old subsystem.
func (m *SubsystemManager) Replace(name string, handler *SubsystemHandler) {
	m.mu.Lock()
	old := m.handlers[name]
	m.handlers[name] = handler
	m.publishFrozenLocked(false)
	m.mu.Unlock()

	if old != nil {
		old.Stop()
	}
}

// StartAll starts all registered subsystems.
func (m *SubsystemManager) StartAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, handler := range m.handlers {
		if err := handler.Start(ctx); err != nil {
			// Stop already started handlers
			for _, h := range m.handlers {
				if h.Running() {
					h.Stop()
				}
			}
			return fmt.Errorf("start subsystem %s: %w", name, err)
		}
	}
	return nil
}

// StopAll stops all subsystems.
func (m *SubsystemManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, handler := range m.handlers {
		handler.Stop()
	}
}

// Freeze creates an immutable snapshot of the handler map.
// After Freeze(), Get and FindHandler use atomic.Load instead of RLock.
// MUST be called after all Register calls complete (after startup barrier).
// Safe to call multiple times (each call overwrites the previous snapshot).
func (m *SubsystemManager) Freeze() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.publishFrozenLocked(true)
}

func (m *SubsystemManager) publishFrozenLocked(force bool) {
	if !force && m.frozen.Load() == nil {
		return
	}
	snap := &frozenSubsystems{
		handlers: make(map[string]*SubsystemHandler, len(m.handlers)),
	}
	maps.Copy(snap.handlers, m.handlers)

	m.frozen.Store(snap)
}

// Get returns a subsystem handler by name.
// After Freeze(), uses lock-free atomic.Load on the frozen snapshot.
func (m *SubsystemManager) Get(name string) *SubsystemHandler {
	if snap := m.frozen.Load(); snap != nil {
		return snap.handlers[name]
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.handlers[name]
}

// Unregister stops and removes a subsystem by name.
// No-op if the name is not registered.
// If frozen, publishes a new snapshot reflecting the removal.
func (m *SubsystemManager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	handler, ok := m.handlers[name]
	if !ok {
		return
	}
	handler.Stop()
	delete(m.handlers, name)

	m.publishFrozenLocked(false)
}

// Names returns the names of all registered subsystems.
func (m *SubsystemManager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.handlers))
	for name := range m.handlers {
		names = append(names, name)
	}
	return names
}

// FindHandler returns the handler for a given command, or nil if not found.
// After Freeze(), uses lock-free atomic.Load on the frozen snapshot.
func (m *SubsystemManager) FindHandler(command string) *SubsystemHandler {
	if snap := m.frozen.Load(); snap != nil {
		return findHandlerByCommand(snap.handlers, command)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return findHandlerByCommand(m.handlers, command)
}

// findHandlerByCommand performs command prefix match on a handler map.
func findHandlerByCommand(handlers map[string]*SubsystemHandler, command string) *SubsystemHandler {
	lowerCmd := strings.ToLower(command)
	for _, handler := range handlers {
		for _, cmd := range handler.Commands() {
			if strings.HasPrefix(lowerCmd, strings.ToLower(cmd)) {
				return handler
			}
		}
	}
	return nil
}

// allCommands returns all commands from all subsystems.
func (m *SubsystemManager) allCommands() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect per-handler command slices first (thread-safe via handler.Commands()).
	perHandler := make([][]string, 0, len(m.handlers))
	total := 0
	for _, handler := range m.handlers {
		cmds := handler.Commands()
		perHandler = append(perHandler, cmds)
		total += len(cmds)
	}

	commands := make([]string, 0, total)
	for _, cmds := range perHandler {
		commands = append(commands, cmds...)
	}
	return commands
}

// AllSchemas returns all schemas from all subsystems.
func (m *SubsystemManager) AllSchemas() []*Schema {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var schemas []*Schema
	for _, handler := range m.handlers {
		if s := handler.Schema(); s != nil {
			priority := s.Priority
			if priority == 0 {
				priority = 1000 // Default priority
			}
			schemas = append(schemas, &Schema{
				Module:    s.Module,
				Namespace: s.Namespace,
				Yang:      s.Yang,
				Handlers:  s.Handlers,
				Plugin:    handler.Name(),
				Priority:  priority,
			})
		}
	}
	return schemas
}

// RegisterSchemas registers all subsystem schemas with the given registry.
func (m *SubsystemManager) RegisterSchemas(registry *SchemaRegistry) error {
	schemas := m.AllSchemas()
	for _, s := range schemas {
		if err := registry.Register(s); err != nil {
			return err
		}
	}
	return nil
}
