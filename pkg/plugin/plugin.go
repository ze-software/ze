// Package plugin provides a high-level SDK for creating ze plugins.
//
// A plugin extends ze's configuration schema by declaring YANG modules,
// handling verify/apply requests for configuration changes, and optionally
// providing additional commands.
//
// Plugins use a candidate/running configuration model:
//   - Commands modify the candidate configuration
//   - "commit" validates and applies changes atomically
//   - "rollback" discards pending changes
//   - "diff" shows pending changes
//
// Basic usage:
//
//	p := plugin.New("bgp")
//	p.SetSchema(myYangSchema, "bgp", "bgp.peer")
//	p.OnVerify("bgp.peer", myVerifyHandler)
//	p.OnApply("bgp.peer", myApplyHandler)
//	p.OnCommand("status", myStatusCommand)
//	p.Run()
package plugin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

// Plugin name length limits.
const (
	MinNameLength = 1
	MaxNameLength = 64
)

// Action constants for configuration changes.
const (
	ActionCreate = "create"
	ActionModify = "modify"
	ActionDelete = "delete"
)

// Plugin represents a ze plugin that follows the 5-stage protocol.
type Plugin struct {
	name      string
	namespace string // Primary namespace (e.g., "bgp")
	schema    string
	handlers  []string

	verifyHandlers  map[string]VerifyHandler
	applyHandlers   map[string]ApplyHandler
	commandHandlers map[string]CommandHandler

	// Candidate/running state management.
	mu        sync.Mutex                   // Protects candidate/running.
	candidate map[string]map[string]string // handler → key → JSON data
	running   map[string]map[string]string // handler → key → JSON data

	input  *bufio.Scanner
	output io.Writer
}

// VerifyHandler handles config verification requests.
type VerifyHandler func(ctx *VerifyContext) error

// ApplyHandler handles config application requests.
type ApplyHandler func(ctx *ApplyContext) error

// CommandHandler handles command requests.
type CommandHandler func(ctx *CommandContext) (any, error)

// VerifyContext provides context for verify handlers.
type VerifyContext struct {
	Action string // create, modify, delete
	Path   string // Handler path (e.g., "peer")
	Data   string // JSON data
}

// ApplyContext provides context for apply handlers.
type ApplyContext struct {
	Action string // create, modify, delete
	Path   string // Handler path (e.g., "peer")
	Data   string // JSON data
}

// CommandContext provides context for command handlers.
type CommandContext struct {
	Command string   // Command name
	Args    []string // Command arguments
}

// ConfigChange represents a change between candidate and running.
type ConfigChange struct {
	Action  string // create, modify, delete
	Handler string // Handler path
	Key     string // Item key
	OldData string // Previous data (for modify/delete)
	NewData string // New data (for create/modify)
}

// New creates a new plugin with the given name.
// The name is also used as the primary namespace.
// Returns nil if name is empty or exceeds 64 characters.
func New(name string) *Plugin {
	if len(name) < MinNameLength || len(name) > MaxNameLength {
		return nil
	}

	return &Plugin{
		name:            name,
		namespace:       name,
		verifyHandlers:  make(map[string]VerifyHandler),
		applyHandlers:   make(map[string]ApplyHandler),
		commandHandlers: make(map[string]CommandHandler),
		candidate:       make(map[string]map[string]string),
		running:         make(map[string]map[string]string),
		input:           bufio.NewScanner(os.Stdin),
		output:          os.Stdout,
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return p.name
}

// Namespace returns the plugin's primary namespace.
func (p *Plugin) Namespace() string {
	return p.namespace
}

// Schema returns the YANG schema.
func (p *Plugin) Schema() string {
	return p.schema
}

// Handlers returns the registered handler prefixes.
func (p *Plugin) Handlers() []string {
	return p.handlers
}

// SetSchema sets the YANG schema and handler prefixes.
// At least one handler prefix is required.
func (p *Plugin) SetSchema(schema string, handlers ...string) error {
	if schema == "" {
		return fmt.Errorf("schema cannot be empty")
	}
	if len(handlers) == 0 {
		return fmt.Errorf("at least one handler prefix is required")
	}

	p.schema = schema
	p.handlers = handlers
	return nil
}

// OnVerify registers a verify handler for the given path prefix.
// The handler is called when a config verify request matches the prefix.
func (p *Plugin) OnVerify(prefix string, handler VerifyHandler) {
	p.verifyHandlers[prefix] = handler
}

// OnApply registers an apply handler for the given path prefix.
// The handler is called when a config apply request matches the prefix.
func (p *Plugin) OnApply(prefix string, handler ApplyHandler) {
	p.applyHandlers[prefix] = handler
}

// OnCommand registers a command handler.
// The handler is called when a command with the given name is received.
func (p *Plugin) OnCommand(name string, handler CommandHandler) {
	p.commandHandlers[name] = handler
}

// SetInput sets the input source for the plugin (for testing).
func (p *Plugin) SetInput(r io.Reader) {
	p.input = bufio.NewScanner(r)
}

// SetOutput sets the output destination for the plugin (for testing).
func (p *Plugin) SetOutput(w io.Writer) {
	p.output = w
}

// Run starts the plugin's 5-stage protocol loop.
// This method blocks until shutdown is received.
func (p *Plugin) Run() error {
	// Stage 1: Declaration
	p.sendDeclarations()

	// Stage 2: Config (wait for config done, handle commands during config)
	if err := p.runConfigPhase(); err != nil {
		return err
	}

	// Stage 3: Capability
	_, _ = fmt.Fprintln(p.output, "capability done")

	// Stage 4: Registry (wait for registry done)
	if err := p.waitForRegistryDone(); err != nil {
		return err
	}

	// Stage 5: Ready
	_, _ = fmt.Fprintln(p.output, "ready")

	// Main command loop
	return p.runCommandLoop()
}

// sendDeclarations sends all declaration messages.
func (p *Plugin) sendDeclarations() {
	_, _ = fmt.Fprintln(p.output, "declare encoding text")

	// Declare schema.
	if p.schema != "" {
		escaped := strings.ReplaceAll(p.schema, "\n", "\\n")
		_, _ = fmt.Fprintf(p.output, "declare schema %s\n", escaped)
	}

	// Declare handlers.
	for _, h := range p.handlers {
		_, _ = fmt.Fprintf(p.output, "declare handler %s\n", h)
	}

	// Declare commands.
	for name := range p.commandHandlers {
		_, _ = fmt.Fprintf(p.output, "declare cmd %s\n", name)
	}

	_, _ = fmt.Fprintln(p.output, "declare done")
}

// runConfigPhase handles the config phase, processing commands.
func (p *Plugin) runConfigPhase() error {
	for p.input.Scan() {
		line := p.input.Text()

		if line == "config done" {
			return nil
		}

		// Handle namespace commands during config phase.
		if strings.HasPrefix(line, p.namespace+" ") {
			if err := p.handleNamespaceCommand(line); err != nil {
				return err
			}
		}
	}
	return p.input.Err()
}

// waitForRegistryDone waits for the registry done signal.
func (p *Plugin) waitForRegistryDone() error {
	for p.input.Scan() {
		if p.input.Text() == "registry done" {
			return nil
		}
	}
	return p.input.Err()
}

// runCommandLoop handles commands after ready.
func (p *Plugin) runCommandLoop() error {
	for p.input.Scan() {
		line := p.input.Text()

		// Check for shutdown.
		if strings.Contains(line, `"shutdown"`) {
			return nil
		}

		// Parse #serial command.
		if !strings.HasPrefix(line, "#") {
			continue
		}

		serial, command := parseSerialCommand(line)
		if serial == "" {
			continue
		}

		response := p.handleCommand(serial, command)
		_, _ = fmt.Fprintln(p.output, response)
	}
	return p.input.Err()
}

// parseSerialCommand extracts serial and command from "#serial command".
func parseSerialCommand(line string) (serial, command string) {
	if !strings.HasPrefix(line, "#") {
		return "", ""
	}

	idx := strings.Index(line, " ")
	if idx <= 1 {
		return "", ""
	}

	return line[1:idx], strings.TrimSpace(line[idx+1:])
}

// handleCommand dispatches a command to the appropriate handler.
func (p *Plugin) handleCommand(serial, command string) string {
	// Check for namespace commands (bgp peer create, bgp commit, etc.).
	if strings.HasPrefix(command, p.namespace+" ") {
		if err := p.handleNamespaceCommand(command); err != nil {
			return fmt.Sprintf("@%s error %v", serial, err)
		}
		return fmt.Sprintf("@%s ok", serial)
	}

	// Check registered commands.
	for name, handler := range p.commandHandlers {
		if strings.HasPrefix(command, name) {
			args := strings.Fields(strings.TrimPrefix(command, name))
			ctx := &CommandContext{
				Command: name,
				Args:    args,
			}

			result, err := handler(ctx)
			if err != nil {
				return fmt.Sprintf("@%s error %v", serial, err)
			}

			if result != nil {
				data, _ := json.Marshal(result)
				return fmt.Sprintf("@%s ok %s", serial, data)
			}
			return fmt.Sprintf("@%s ok", serial)
		}
	}

	return fmt.Sprintf("@%s error unknown command: %s", serial, command)
}

// handleNamespaceCommand handles commands in the plugin's namespace.
// Format: <namespace> <path> <action> {json}.
// Or: <namespace> commit|rollback|diff.
func (p *Plugin) handleNamespaceCommand(line string) error {
	// Remove namespace prefix.
	rest := strings.TrimPrefix(line, p.namespace+" ")
	rest = strings.TrimSpace(rest)

	// Check for built-in commands.
	switch rest {
	case "commit":
		return p.commit()
	case "rollback":
		p.rollback()
		return nil
	case "diff":
		p.outputDiff()
		return nil
	}

	// Parse: <path> <action> {json}
	return p.handleConfigCommand(rest)
}

// outputDiff outputs pending changes to the output writer as JSON.
func (p *Plugin) outputDiff() {
	changes := p.computeDiff()
	if len(changes) == 0 {
		_, _ = fmt.Fprintln(p.output, "[]")
		return
	}
	// Sort for deterministic output.
	sortChanges(changes)
	// Output as JSON array.
	data, _ := json.Marshal(changes)
	_, _ = fmt.Fprintln(p.output, string(data))
}

// handleConfigCommand parses and applies a config command to candidate.
// Format: <path> <action> {json}.
// Or: <action> {json} (for namespace-level config).
func (p *Plugin) handleConfigCommand(line string) error {
	// Find JSON start.
	jsonIdx := strings.Index(line, "{")
	if jsonIdx < 0 {
		return fmt.Errorf("expected JSON data starting with '{'")
	}

	// Parse path and action.
	prefix := strings.TrimSpace(line[:jsonIdx])
	parts := strings.Fields(prefix)

	var handler, action string
	data := line[jsonIdx:]

	// Validate JSON and parse for key extraction (single parse).
	// Config data must be a JSON object, not an array or primitive.
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return fmt.Errorf("invalid JSON (expected object): %w", err)
	}

	switch len(parts) {
	case 1:
		// Format: <action> {json} (namespace-level config).
		handler = p.namespace
		action = parts[0]
	case 2:
		// Format: <path> <action> {json}.
		handler = p.namespace + "." + parts[0]
		action = parts[1]
	default:
		return fmt.Errorf("expected '<action>' or '<path> <action>'")
	}

	// Extract key from parsed JSON for storage.
	key := extractKeyFromMap(obj, data)

	// Apply to candidate (protected by mutex).
	p.mu.Lock()
	defer p.mu.Unlock()

	switch action {
	case ActionCreate, ActionModify:
		if p.candidate[handler] == nil {
			p.candidate[handler] = make(map[string]string)
		}
		p.candidate[handler][key] = data

	case ActionDelete:
		if p.candidate[handler] != nil {
			delete(p.candidate[handler], key)
		}

	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	return nil
}

// extractKeyFromMap extracts a key from a parsed JSON object for indexing.
// Looks for common key fields in order: address, name, prefix, id, key.
// Falls back to raw JSON string if no key field found.
func extractKeyFromMap(obj map[string]any, rawData string) string {
	for _, field := range []string{"address", "name", "prefix", "id", "key"} {
		if v, ok := obj[field]; ok {
			return fmt.Sprintf("%v", v)
		}
	}
	return rawData
}

// extractKey extracts a key from JSON data for indexing.
// Looks for common key fields: address, name, prefix, id, key.
func extractKey(data string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return data // Use entire JSON as key if parse fails.
	}
	return extractKeyFromMap(obj, data)
}

// commit validates and applies all pending changes.
// Releases mutex during callbacks to prevent deadlock.
func (p *Plugin) commit() error {
	// Phase 1: Compute diff while holding lock.
	p.mu.Lock()
	changes := p.computeDiffLocked()
	if len(changes) == 0 {
		p.mu.Unlock()
		return nil // Nothing to commit.
	}
	// Sort changes for deterministic order.
	sortChanges(changes)
	// Copy candidate state for later.
	candidateCopy := p.cloneState(p.candidate)
	p.mu.Unlock()

	// Phase 2: Verify all changes (no lock held - callbacks may access state).
	for _, change := range changes {
		// Use OldData for delete so handler knows what's being deleted.
		data := change.NewData
		if change.Action == ActionDelete {
			data = change.OldData
		}
		if err := p.triggerVerify(&VerifyContext{
			Action: change.Action,
			Path:   change.Handler,
			Data:   data,
		}); err != nil {
			return fmt.Errorf("verify failed for %s: %w", change.Handler, err)
		}
	}

	// Phase 3: Apply all changes (no lock held).
	// Track applied changes for rollback on failure.
	var applied []ConfigChange
	for _, change := range changes {
		data := change.NewData
		if change.Action == ActionDelete {
			data = change.OldData
		}
		if err := p.triggerApply(&ApplyContext{
			Action: change.Action,
			Path:   change.Handler,
			Data:   data,
		}); err != nil {
			// Rollback already-applied changes.
			rollbackErrs := p.rollbackApplied(applied)
			if len(rollbackErrs) > 0 {
				return fmt.Errorf("apply failed for %s: %w (rollback errors: %v)", change.Handler, err, rollbackErrs)
			}
			return fmt.Errorf("apply failed for %s: %w", change.Handler, err)
		}
		applied = append(applied, change)
	}

	// Phase 4: Update running state.
	p.mu.Lock()
	p.running = candidateCopy
	p.mu.Unlock()
	return nil
}

// rollbackApplied attempts to undo already-applied changes.
// Called when apply fails mid-way through changes.
// Returns collected errors if any rollback operations fail.
func (p *Plugin) rollbackApplied(applied []ConfigChange) []error {
	var errs []error
	// Reverse order for rollback.
	for i := len(applied) - 1; i >= 0; i-- {
		change := applied[i]
		// Invert the action.
		var action, data string
		switch change.Action {
		case ActionCreate:
			action = ActionDelete
			data = change.NewData
		case ActionDelete:
			action = ActionCreate
			data = change.OldData
		case ActionModify:
			action = ActionModify
			data = change.OldData // Revert to old data.
		default:
			// Unknown action - skip rollback, record error.
			errs = append(errs, fmt.Errorf("rollback skipped for unknown action %q on %s", change.Action, change.Handler))
			continue
		}
		if err := p.triggerApply(&ApplyContext{
			Action: action,
			Path:   change.Handler,
			Data:   data,
		}); err != nil {
			errs = append(errs, fmt.Errorf("rollback %s %s: %w", action, change.Handler, err))
		}
	}
	return errs
}

// rollback discards candidate and reverts to running.
func (p *Plugin) rollback() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.candidate = p.cloneState(p.running)
}

// computeDiff compares candidate vs running and returns changes.
// Acquires lock internally.
func (p *Plugin) computeDiff() []ConfigChange {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.computeDiffLocked()
}

// computeDiffLocked compares candidate vs running. Must hold mu.
func (p *Plugin) computeDiffLocked() []ConfigChange {
	var changes []ConfigChange

	// Find creates and modifies (in candidate but not in running, or different).
	for handler, items := range p.candidate {
		runningItems := p.running[handler]
		for key, data := range items {
			if runningItems == nil {
				changes = append(changes, ConfigChange{
					Action:  ActionCreate,
					Handler: handler,
					Key:     key,
					NewData: data,
				})
			} else if oldData, exists := runningItems[key]; !exists {
				changes = append(changes, ConfigChange{
					Action:  ActionCreate,
					Handler: handler,
					Key:     key,
					NewData: data,
				})
			} else if oldData != data {
				changes = append(changes, ConfigChange{
					Action:  ActionModify,
					Handler: handler,
					Key:     key,
					OldData: oldData,
					NewData: data,
				})
			}
		}
	}

	// Find deletes (in running but not in candidate).
	for handler, items := range p.running {
		candidateItems := p.candidate[handler]
		for key, data := range items {
			_, exists := candidateItems[key]
			if candidateItems == nil || !exists {
				changes = append(changes, ConfigChange{
					Action:  ActionDelete,
					Handler: handler,
					Key:     key,
					OldData: data,
				})
			}
		}
	}

	return changes
}

// cloneState creates a deep copy of a state map.
func (p *Plugin) cloneState(state map[string]map[string]string) map[string]map[string]string {
	clone := make(map[string]map[string]string)
	for handler, items := range state {
		clone[handler] = make(map[string]string)
		for key, data := range items {
			clone[handler][key] = data
		}
	}
	return clone
}

// sortChanges sorts changes for deterministic processing order.
// Order: handler (alphabetic), then key (alphabetic), then action (delete < create < modify).
func sortChanges(changes []ConfigChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Handler != changes[j].Handler {
			return changes[i].Handler < changes[j].Handler
		}
		if changes[i].Key != changes[j].Key {
			return changes[i].Key < changes[j].Key
		}
		return actionRank(changes[i].Action) < actionRank(changes[j].Action)
	})
}

// actionRank returns sort order for actions. Unknown actions sort last.
func actionRank(action string) int {
	switch action {
	case ActionDelete:
		return 0
	case ActionCreate:
		return 1
	case ActionModify:
		return 2
	default:
		return 99 // Unknown actions sort last
	}
}

// triggerVerify calls the appropriate verify handler.
func (p *Plugin) triggerVerify(ctx *VerifyContext) error {
	handler := p.findVerifyHandler(ctx.Path)
	if handler == nil {
		// No handler registered - allow by default.
		return nil
	}
	return handler(ctx)
}

// triggerApply calls the appropriate apply handler.
func (p *Plugin) triggerApply(ctx *ApplyContext) error {
	handler := p.findApplyHandler(ctx.Path)
	if handler == nil {
		// No handler registered - allow by default.
		return nil
	}
	return handler(ctx)
}

// triggerCommand calls the appropriate command handler.
func (p *Plugin) triggerCommand(ctx *CommandContext) (any, error) {
	handler, ok := p.commandHandlers[ctx.Command]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", ctx.Command)
	}
	return handler(ctx)
}

// findVerifyHandler finds a verify handler by longest prefix match.
func (p *Plugin) findVerifyHandler(path string) VerifyHandler {
	cleanPath := stripPredicates(path)

	var best VerifyHandler
	var bestLen int

	for prefix, handler := range p.verifyHandlers {
		if strings.HasPrefix(cleanPath, prefix) && len(prefix) > bestLen {
			best = handler
			bestLen = len(prefix)
		}
	}
	return best
}

// findApplyHandler finds an apply handler by longest prefix match.
func (p *Plugin) findApplyHandler(path string) ApplyHandler {
	cleanPath := stripPredicates(path)

	var best ApplyHandler
	var bestLen int

	for prefix, handler := range p.applyHandlers {
		if strings.HasPrefix(cleanPath, prefix) && len(prefix) > bestLen {
			best = handler
			bestLen = len(prefix)
		}
	}
	return best
}

// stripPredicates removes YANG predicates like [key=value] from a path.
func stripPredicates(path string) string {
	var result strings.Builder
	depth := 0
	for _, c := range path {
		if c == '[' {
			depth++
			continue
		}
		if c == ']' {
			depth--
			continue
		}
		if depth == 0 {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// Candidate returns the current candidate configuration (for testing).
func (p *Plugin) Candidate() map[string]map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cloneState(p.candidate)
}

// Running returns the current running configuration (for testing).
func (p *Plugin) Running() map[string]map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cloneState(p.running)
}

// SetRunning sets the running configuration (for testing).
// This is thread-safe and should be used instead of direct field access.
func (p *Plugin) SetRunning(state map[string]map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = p.cloneState(state)
}

// SetCandidate sets the candidate configuration (for testing).
// This is thread-safe and should be used instead of direct field access.
func (p *Plugin) SetCandidate(state map[string]map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.candidate = p.cloneState(state)
}
