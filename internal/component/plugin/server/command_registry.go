// Design: docs/architecture/api/process-protocol.md — plugin process management

package server

import (
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
)

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
	mu       sync.RWMutex
	commands map[string]*RegisteredCommand // lowercase name → registration
	builtins map[string]bool               // lowercase builtin names (cannot be shadowed)
}

// NewCommandRegistry creates a new command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]*RegisteredCommand),
		builtins: make(map[string]bool),
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
func (r *CommandRegistry) Unregister(proc *process.Process, names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, name := range names {
		key := strings.ToLower(name)
		if cmd, ok := r.commands[key]; ok && cmd.Process == proc {
			delete(r.commands, key)
		}
	}
}

// UnregisterAll removes all commands owned by the process.
// Called when a process dies.
func (r *CommandRegistry) UnregisterAll(proc *process.Process) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, cmd := range r.commands {
		if cmd.Process == proc {
			delete(r.commands, key)
		}
	}
}

// Lookup finds a command by exact name (case-insensitive).
func (r *CommandRegistry) Lookup(name string) *RegisteredCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.commands[strings.ToLower(name)]
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
