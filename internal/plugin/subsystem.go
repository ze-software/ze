package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

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
type SubsystemHandler struct {
	config   SubsystemConfig
	proc     *Process
	commands []string          // Commands declared during Stage 1
	schema   *PluginSchemaDecl // YANG schema declared during Stage 1
	mu       sync.RWMutex
}

// NewSubsystemHandler creates a handler backed by a forked process.
func NewSubsystemHandler(config SubsystemConfig) *SubsystemHandler {
	return &SubsystemHandler{
		config: config,
	}
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
func (h *SubsystemHandler) Schema() *PluginSchemaDecl {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.schema
}

// Start spawns the subsystem process and completes the 5-stage protocol.
func (h *SubsystemHandler) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Build command:
	// - If Binary contains spaces (full command), use as-is
	// - Otherwise, add --mode=<name>
	cmd := h.config.Binary
	if !strings.Contains(cmd, " ") {
		cmd = fmt.Sprintf("%s --mode=%s", cmd, h.config.Name)
	}

	// Append config path if provided
	if h.config.ConfigPath != "" {
		cmd = fmt.Sprintf("%s --config %s", cmd, h.config.ConfigPath)
	}

	// Create process config
	procConfig := PluginConfig{
		Name: "subsystem-" + h.config.Name,
		Run:  cmd,
	}

	h.proc = NewProcess(procConfig)
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

// completeProtocol runs through the 5-stage startup protocol via RPC.
func (h *SubsystemHandler) completeProtocol(ctx context.Context) error {
	connA := h.proc.engineConnA
	connB := h.proc.ConnB()
	if connB == nil {
		return fmt.Errorf("subsystem connection closed before protocol")
	}

	// Stage 1: Read declare-registration from plugin (Socket A)
	req, err := connA.ReadRequest(ctx)
	if err != nil {
		return fmt.Errorf("stage 1 read: %w", err)
	}
	if req.Method != "ze-plugin-engine:declare-registration" {
		_ = connA.SendError(ctx, req.ID, "expected declare-registration, got "+req.Method)
		return fmt.Errorf("stage 1: expected declare-registration, got %s", req.Method)
	}

	var regInput rpc.DeclareRegistrationInput
	if err := json.Unmarshal(req.Params, &regInput); err != nil {
		_ = connA.SendError(ctx, req.ID, "invalid registration: "+err.Error())
		return fmt.Errorf("stage 1 parse: %w", err)
	}

	// Extract commands from registration
	for _, cmd := range regInput.Commands {
		h.commands = append(h.commands, cmd.Name)
	}

	// Extract schema from registration
	if regInput.Schema != nil {
		if h.schema == nil {
			h.schema = &PluginSchemaDecl{}
		}
		h.schema.Yang = regInput.Schema.YANGText
		h.schema.Module = regInput.Schema.Module
		h.schema.Namespace = regInput.Schema.Namespace
		h.schema.Handlers = regInput.Schema.Handlers
	}

	// Send OK response
	if err := connA.SendResult(ctx, req.ID, nil); err != nil {
		return fmt.Errorf("stage 1 respond: %w", err)
	}

	// Stage 2: Send configure to plugin (Socket B)
	if err := connB.SendConfigure(ctx, nil); err != nil {
		return fmt.Errorf("stage 2 configure: %w", err)
	}

	// Stage 3: Read declare-capabilities from plugin (Socket A)
	req, err = connA.ReadRequest(ctx)
	if err != nil {
		return fmt.Errorf("stage 3 read: %w", err)
	}
	if req.Method != "ze-plugin-engine:declare-capabilities" {
		_ = connA.SendError(ctx, req.ID, "expected declare-capabilities, got "+req.Method)
		return fmt.Errorf("stage 3: expected declare-capabilities, got %s", req.Method)
	}
	if err := connA.SendResult(ctx, req.ID, nil); err != nil {
		return fmt.Errorf("stage 3 respond: %w", err)
	}

	// Stage 4: Send share-registry to plugin (Socket B)
	if err := connB.SendShareRegistry(ctx, nil); err != nil {
		return fmt.Errorf("stage 4 share-registry: %w", err)
	}

	// Stage 5: Read ready from plugin (Socket A)
	req, err = connA.ReadRequest(ctx)
	if err != nil {
		return fmt.Errorf("stage 5 read: %w", err)
	}
	if req.Method != "ze-plugin-engine:ready" {
		_ = connA.SendError(ctx, req.ID, "expected ready, got "+req.Method)
		return fmt.Errorf("stage 5: expected ready, got %s", req.Method)
	}
	if err := connA.SendResult(ctx, req.ID, nil); err != nil {
		return fmt.Errorf("stage 5 respond: %w", err)
	}

	return nil
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
func (h *SubsystemHandler) Handle(ctx context.Context, command string) (*Response, error) {
	h.mu.RLock()
	proc := h.proc
	h.mu.RUnlock()

	if proc == nil || !proc.Running() {
		return nil, errors.New("subsystem not running")
	}

	// Send command via RPC execute-command
	connB := proc.ConnB()
	if connB == nil {
		return nil, errors.New("subsystem connection closed")
	}
	out, err := connB.SendExecuteCommand(ctx, "", command, nil, "")
	if err != nil {
		return &Response{Status: statusError, Data: err.Error()}, err
	}

	return &Response{Status: out.Status, Data: out.Data}, nil
}

// SubsystemManager manages multiple subsystem handlers.
type SubsystemManager struct {
	handlers map[string]*SubsystemHandler
	mu       sync.RWMutex
}

// NewSubsystemManager creates a new subsystem manager.
func NewSubsystemManager() *SubsystemManager {
	return &SubsystemManager{
		handlers: make(map[string]*SubsystemHandler),
	}
}

// Register adds a subsystem configuration.
func (m *SubsystemManager) Register(config SubsystemConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[config.Name] = NewSubsystemHandler(config)
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

// Get returns a subsystem handler by name.
func (m *SubsystemManager) Get(name string) *SubsystemHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.handlers[name]
}

// FindHandler returns the handler for a given command, or nil if not found.
func (m *SubsystemManager) FindHandler(command string) *SubsystemHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lowerCmd := strings.ToLower(command)
	for _, handler := range m.handlers {
		for _, cmd := range handler.Commands() {
			if strings.HasPrefix(lowerCmd, strings.ToLower(cmd)) {
				return handler
			}
		}
	}
	return nil
}

// AllCommands returns all commands from all subsystems.
func (m *SubsystemManager) AllCommands() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Count total commands for preallocation
	total := 0
	for _, handler := range m.handlers {
		total += len(handler.commands)
	}

	commands := make([]string, 0, total)
	for _, handler := range m.handlers {
		commands = append(commands, handler.Commands()...)
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
