package plugin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Protocol stage markers.
const (
	markerDeclareDone    = "declare done"
	markerConfigDone     = "config done"
	markerCapabilityDone = "capability done"
	markerRegistryDone   = "registry done"
	markerReady          = "ready"
)

// SubsystemConfig describes a forked subsystem process.
type SubsystemConfig struct {
	Name     string   // Subsystem name (cache, route, session)
	Binary   string   // Path to binary (default: ze-subsystem)
	Commands []string // Commands this subsystem handles (for pre-registration)
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
	if config.Binary == "" {
		config.Binary = "ze-subsystem"
	}
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

// parsePriorityLine parses a "declare priority <number>" line.
func (h *SubsystemHandler) parsePriorityLine(line string) {
	parts := strings.Fields(line)
	if len(parts) < 3 { // declare priority <number>
		return
	}

	n, err := strconv.Atoi(parts[2])
	if err != nil {
		return // Invalid priority, ignore
	}

	if h.schema == nil {
		h.schema = &PluginSchemaDecl{Priority: 1000}
	}
	h.schema.Priority = n
}

// parseSchemaLine parses a "declare schema <type> <value>" line.
func (h *SubsystemHandler) parseSchemaLine(line string) {
	parts := strings.Fields(line)
	if len(parts) < 4 { // declare schema <type> <value>
		return // Not enough parts, might be heredoc start
	}

	schemaType := parts[2]
	value := parts[3]

	if h.schema == nil {
		h.schema = &PluginSchemaDecl{}
	}

	switch schemaType {
	case "module":
		h.schema.Module = value
	case "namespace":
		h.schema.Namespace = value
	case "handler":
		h.schema.Handlers = append(h.schema.Handlers, value)
	case "yang":
		// Single-line YANG or heredoc start - heredoc handled separately
		if !strings.Contains(line, "<<") {
			idx := strings.Index(line, "declare schema yang ")
			if idx >= 0 {
				h.schema.Yang = strings.TrimSpace(line[idx+len("declare schema yang "):])
			}
		}
	}
}

// Start spawns the subsystem process and completes the 5-stage protocol.
func (h *SubsystemHandler) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create process config
	procConfig := PluginConfig{
		Name: "subsystem-" + h.config.Name,
		Run:  fmt.Sprintf("%s --mode=%s", h.config.Binary, h.config.Name),
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

// completeProtocol runs through the 5-stage startup protocol.
func (h *SubsystemHandler) completeProtocol(ctx context.Context) error {
	// Stage 1: Read declarations until "declare done"
	var heredocDelimiter string
	for {
		line, err := h.proc.ReadCommand(ctx)
		if err != nil {
			return fmt.Errorf("stage 1 read: %w", err)
		}

		// Handle heredoc continuation
		if heredocDelimiter != "" {
			if IsHeredocEnd(line, heredocDelimiter) {
				heredocDelimiter = ""
				continue
			}
			// Append to YANG content
			if h.schema == nil {
				h.schema = &PluginSchemaDecl{}
			}
			if h.schema.Yang != "" {
				h.schema.Yang += "\n"
			}
			h.schema.Yang += line
			continue
		}

		// Check for declare done
		if line == markerDeclareDone {
			break
		}

		// Parse "declare cmd <name>"
		if strings.HasPrefix(line, "declare cmd ") {
			cmdName := strings.TrimPrefix(line, "declare cmd ")
			h.commands = append(h.commands, cmdName)
			continue
		}

		// Parse "declare schema <type> <value>"
		if strings.HasPrefix(line, "declare schema ") {
			h.parseSchemaLine(line)
			// Check for heredoc start
			if delim, ok := StartHeredoc(line); ok {
				heredocDelimiter = delim
			}
			continue
		}

		// Parse "declare priority <number>"
		if strings.HasPrefix(line, "declare priority ") {
			h.parsePriorityLine(line)
		}
	}

	// Stage 2: Send config done (no config for now)
	if err := h.proc.WriteEvent(markerConfigDone); err != nil {
		return fmt.Errorf("stage 2 write: %w", err)
	}

	// Stage 3: Read capability done
	line, err := h.proc.ReadCommand(ctx)
	if err != nil {
		return fmt.Errorf("stage 3 read: %w", err)
	}
	if line != markerCapabilityDone {
		return fmt.Errorf("stage 3: expected '%s', got: %s", markerCapabilityDone, line)
	}

	// Stage 4: Send registry done
	if err := h.proc.WriteEvent(markerRegistryDone); err != nil {
		return fmt.Errorf("stage 4 write: %w", err)
	}

	// Stage 5: Read ready
	line, err = h.proc.ReadCommand(ctx)
	if err != nil {
		return fmt.Errorf("stage 5 read: %w", err)
	}
	if line != markerReady {
		return fmt.Errorf("stage 5: expected '%s', got: %s", markerReady, line)
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

// Handle sends a command to the subsystem and returns the response.
func (h *SubsystemHandler) Handle(ctx context.Context, command string) (*Response, error) {
	h.mu.RLock()
	proc := h.proc
	h.mu.RUnlock()

	if proc == nil || !proc.Running() {
		return nil, errors.New("subsystem not running")
	}

	// Send command via pipe and wait for response
	resp, err := proc.SendRequest(ctx, command)
	if err != nil {
		return &Response{Status: statusError, Data: err.Error()}, err
	}

	// Parse response: "ok {json}" or "error message"
	return parseSubsystemResponse(resp)
}

// parseSubsystemResponse parses "@serial ok data" or "@serial error msg" format.
// The serial has already been stripped by Process.SendRequest, so we get "ok data".
func parseSubsystemResponse(resp string) (*Response, error) {
	// Split into status and data
	parts := strings.SplitN(resp, " ", 2)
	if len(parts) == 0 {
		return &Response{Status: statusError, Data: "empty response"}, nil
	}

	status := parts[0]
	var data any
	if len(parts) > 1 {
		data = parts[1]
	}

	switch status {
	case "ok":
		return &Response{Status: statusDone, Data: data}, nil
	case "error":
		return &Response{Status: statusError, Data: data}, errors.New(fmt.Sprint(data))
	default:
		return &Response{Status: statusDone, Data: resp}, nil
	}
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
