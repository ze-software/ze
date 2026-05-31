// Design: docs/architecture/api/process-protocol.md — plugin process management

package server

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
)

var errCommandNameCannotBeEmpty = errors.New("command name cannot be empty")

// frozenCommands holds an immutable snapshot of the CommandRegistry's command
// map. Created by Freeze() after startup, used by Lookup() on the hot path
// to avoid RLock.
type frozenCommands struct {
	commands   map[string]*RegisteredCommand
	deprecated map[string]*deprecatedAlias
}

// deprecatedAlias maps an old command name to the canonical (new) name.
type deprecatedAlias struct {
	OldName      string
	NewLowerName string
	Process      *process.Process
}

// validateCommandName checks that a command name contains only safe characters.
// Prevents command shadowing via prefix matching with special characters.
func validateCommandName(name string) error {
	if name == "" {
		return errCommandNameCannotBeEmpty
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' && r != '-' {
			return fmt.Errorf("command name %q contains invalid character %q (only letters, digits, spaces, hyphens allowed)", name, r)
		}
	}
	return nil
}

// Default timeouts for plugin commands.
const (
	DefaultCommandTimeout = 30 * time.Second
	CompletionTimeout     = 500 * time.Millisecond
)

// Completion represents a single completion suggestion.
// Used for both command and argument completion.
type Completion struct {
	Value  string `json:"value"`            // The completion text
	Help   string `json:"help,omitempty"`   // Optional description
	Source string `json:"source,omitempty"` // "builtin" or process name (verbose mode)
}

// CommandDef describes a command to register.
// Passed from process to registry during registration.
type CommandDef struct {
	Name        string        // Command name (e.g., "myapp status")
	Description string        // Help text
	Args        string        // Usage hint (e.g., "<component>")
	Completable bool          // Process handles arg completion
	Timeout     time.Duration // Per-command timeout (0 = default 30s)
}

// RegisterResult holds the result of a single command registration.
type RegisterResult struct {
	Name  string // Command that was registered
	OK    bool   // True if registration succeeded
	Error string // Error message if failed
}

// RegisteredCommand represents a plugin command in the registry.
type RegisteredCommand struct {
	Name         string
	LowerName    string // Pre-lowercased at registration for dispatch matching (zero alloc per lookup)
	Description  string
	Args         string           // Usage hint (e.g., "<component>")
	Completable  bool             // Process handles arg completion
	Timeout      time.Duration    // Per-command timeout
	Process      *process.Process // Owning process
	RegisteredAt time.Time
}

// CommandRegistry manages plugin commands.
// Thread-safe for concurrent registration and lookup.
type CommandRegistry struct {
	mu         sync.RWMutex
	commands   map[string]*RegisteredCommand // lowercase name → registration
	builtins   map[string]bool               // lowercase builtin names (cannot be shadowed)
	deprecated map[string]*deprecatedAlias   // old lowercase name → alias

	// deprecatedWarned tracks old command names that have already logged a
	// deprecation warning this session (log once per name).
	deprecatedWarned sync.Map

	// frozen holds an immutable snapshot for lock-free Lookup after startup.
	// nil before Freeze() is called.
	frozen atomic.Pointer[frozenCommands]
}

// NewCommandRegistry creates a new command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands:   make(map[string]*RegisteredCommand),
		builtins:   make(map[string]bool),
		deprecated: make(map[string]*deprecatedAlias),
	}
}

// AddBuiltin marks a command name as builtin (cannot be shadowed).
// Called during dispatcher initialization.
func (r *CommandRegistry) AddBuiltin(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtins[strings.ToLower(name)] = true
}

// Register adds commands for a process.
// Returns results for each command (success or failure reason).
func (r *CommandRegistry) Register(proc *process.Process, defs []CommandDef) []RegisterResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	results := make([]RegisterResult, len(defs))
	now := time.Now()

	for i, def := range defs {
		key := strings.ToLower(def.Name)
		results[i].Name = def.Name

		// Validate command name format
		if err := validateCommandName(def.Name); err != nil {
			results[i].OK = false
			results[i].Error = err.Error()
			continue
		}

		// Check builtin conflict
		if r.builtins[key] {
			results[i].OK = false
			results[i].Error = "conflicts with builtin: " + def.Name
			continue
		}

		// Check existing registration
		if existing, ok := r.commands[key]; ok {
			results[i].OK = false
			results[i].Error = "already registered by process: " + existing.Process.Config().Name
			continue
		}

		// Apply default timeout
		timeout := def.Timeout
		if timeout == 0 {
			timeout = DefaultCommandTimeout
		}

		// Register
		r.commands[key] = &RegisteredCommand{
			Name:         def.Name,
			LowerName:    key,
			Description:  def.Description,
			Args:         def.Args,
			Completable:  def.Completable,
			Timeout:      timeout,
			Process:      proc,
			RegisteredAt: now,
		}
		results[i].OK = true
	}

	return results
}

// Unregister removes commands owned by the process.
// Only the owning process can unregister a command.
// Unknown commands are silently ignored.
// If frozen, publishes a new snapshot reflecting the removal.
func (r *CommandRegistry) Unregister(proc *process.Process, names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, name := range names {
		key := strings.ToLower(name)
		if cmd, ok := r.commands[key]; ok && cmd.Process == proc {
			delete(r.commands, key)
		}
	}

	r.republishFrozen()
}

// UnregisterAll removes all commands and deprecated aliases owned by the process.
// Called when a process dies.
// If frozen, publishes a new snapshot reflecting the removal.
func (r *CommandRegistry) UnregisterAll(proc *process.Process) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, cmd := range r.commands {
		if cmd.Process == proc {
			delete(r.commands, key)
		}
	}

	for key, alias := range r.deprecated {
		if alias.Process == proc {
			delete(r.deprecated, key)
		}
	}

	r.republishFrozen()
}

// RegisterDeprecated adds a deprecated alias that maps oldName to the
// canonical command registered under newName. When the old name is looked
// up, the canonical RegisteredCommand is returned and a deprecation
// warning is logged once per session.
func (r *CommandRegistry) RegisterDeprecated(proc *process.Process, oldName, newName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldKey := strings.ToLower(oldName)
	newKey := strings.ToLower(newName)
	r.deprecated[oldKey] = &deprecatedAlias{
		OldName:      oldName,
		NewLowerName: newKey,
		Process:      proc,
	}
}

// republishFrozen rebuilds and stores a new frozen snapshot from the current
// mutable map. Must be called under r.mu.Lock. No-op if Freeze was never called.
func (r *CommandRegistry) republishFrozen() {
	if r.frozen.Load() == nil {
		return
	}
	snap := &frozenCommands{
		commands:   make(map[string]*RegisteredCommand, len(r.commands)),
		deprecated: make(map[string]*deprecatedAlias, len(r.deprecated)),
	}
	maps.Copy(snap.commands, r.commands)
	maps.Copy(snap.deprecated, r.deprecated)
	r.frozen.Store(snap)
}

// Freeze creates an immutable snapshot of the commands and deprecated maps.
// After Freeze(), Lookup uses atomic.Load instead of RLock.
// MUST be called after all Register calls complete (after startup barrier).
// Safe to call multiple times (each call overwrites the previous snapshot).
func (r *CommandRegistry) Freeze() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := &frozenCommands{
		commands:   make(map[string]*RegisteredCommand, len(r.commands)),
		deprecated: make(map[string]*deprecatedAlias, len(r.deprecated)),
	}
	maps.Copy(snap.commands, r.commands)
	maps.Copy(snap.deprecated, r.deprecated)

	r.frozen.Store(snap)
}

// Lookup finds a command by exact name (case-insensitive).
// If no primary match is found, checks deprecated aliases and returns the
// canonical command (logging a deprecation warning once per session).
// After Freeze(), uses lock-free atomic.Load on the frozen snapshot.
func (r *CommandRegistry) Lookup(name string) *RegisteredCommand {
	key := strings.ToLower(name)
	if snap := r.frozen.Load(); snap != nil {
		if cmd := snap.commands[key]; cmd != nil {
			return cmd
		}
		return r.resolveDeprecatedFrozen(snap, key)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if cmd := r.commands[key]; cmd != nil {
		return cmd
	}
	return r.resolveDeprecatedLocked(key)
}

// resolveDeprecatedFrozen checks the frozen deprecated map and returns the
// canonical command. Logs a warning once per session per old name.
func (r *CommandRegistry) resolveDeprecatedFrozen(snap *frozenCommands, oldKey string) *RegisteredCommand {
	alias := snap.deprecated[oldKey]
	if alias == nil {
		return nil
	}
	cmd := snap.commands[alias.NewLowerName]
	if cmd == nil {
		return nil
	}
	r.warnDeprecated(alias.OldName, cmd.Name)
	return cmd
}

// resolveDeprecatedLocked checks the mutable deprecated map. Caller holds RLock.
func (r *CommandRegistry) resolveDeprecatedLocked(oldKey string) *RegisteredCommand {
	alias := r.deprecated[oldKey]
	if alias == nil {
		return nil
	}
	cmd := r.commands[alias.NewLowerName]
	if cmd == nil {
		return nil
	}
	r.warnDeprecated(alias.OldName, cmd.Name)
	return cmd
}

// warnDeprecated logs a deprecation warning once per session per old name.
func (r *CommandRegistry) warnDeprecated(oldName, newName string) {
	if _, loaded := r.deprecatedWarned.LoadOrStore(strings.ToLower(oldName), true); !loaded {
		logger().Warn("deprecated command used", "old", oldName, "new", newName,
			"hint", "use '"+newName+"' instead; old form will be removed in a future release")
	}
}

// All returns all registered commands.
func (r *CommandRegistry) All() []*RegisteredCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredCommand, 0, len(r.commands))
	for _, cmd := range r.commands {
		result = append(result, cmd)
	}
	return result
}

// Complete returns commands matching the partial input.
// Used for CLI command completion.
func (r *CommandRegistry) Complete(partial string) []Completion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	partial = strings.ToLower(partial)
	var completions []Completion

	for key, cmd := range r.commands {
		if strings.HasPrefix(key, partial) {
			completions = append(completions, Completion{
				Value:  cmd.Name,
				Help:   cmd.Description,
				Source: cmd.Process.Config().Name,
			})
		}
	}

	return completions
}

// IsBuiltin returns true if the command name is a builtin.
func (r *CommandRegistry) IsBuiltin(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.builtins[strings.ToLower(name)]
}

// LookupDeprecatedPrefix finds the longest deprecated alias that is a prefix
// of lowerInput (already lowercased by the caller). Returns the canonical
// RegisteredCommand and the matched prefix length, or (nil, 0) if no
// deprecated alias matches. Logs a deprecation warning on first match.
func (r *CommandRegistry) LookupDeprecatedPrefix(lowerInput string) (*RegisteredCommand, int) {
	if snap := r.frozen.Load(); snap != nil {
		return r.lookupDeprecatedPrefixInMaps(snap.deprecated, snap.commands, lowerInput)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lookupDeprecatedPrefixInMaps(r.deprecated, r.commands, lowerInput)
}

func (r *CommandRegistry) lookupDeprecatedPrefixInMaps(
	deprecated map[string]*deprecatedAlias,
	commands map[string]*RegisteredCommand,
	lowerInput string,
) (*RegisteredCommand, int) {
	var bestAlias *deprecatedAlias
	var bestLen int
	for oldKey, alias := range deprecated {
		if len(oldKey) <= bestLen {
			continue
		}
		if strings.HasPrefix(lowerInput, oldKey) &&
			(len(lowerInput) == len(oldKey) || lowerInput[len(oldKey)] == ' ') {
			bestAlias = alias
			bestLen = len(oldKey)
		}
	}
	if bestAlias == nil {
		return nil, 0
	}
	cmd := commands[bestAlias.NewLowerName]
	if cmd == nil {
		return nil, 0
	}
	r.warnDeprecated(bestAlias.OldName, cmd.Name)
	return cmd, bestLen
}
