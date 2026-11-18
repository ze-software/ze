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

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/grammar"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// The canonical closed set of first tokens (verbs) a plugin command may begin
// with lives in command.Verbs (internal/component/command), the single source of
// truth shared with the grammar gate (ai/rules/cli.md, evidence.md).
// validateCommandName below checks against it; adding a verb is a deliberate edit
// to command.Verbs, not a second list here.
//
// Scope: this registration gate applies ONLY to plugin-registered commands
// (Register and RegisterDeprecated). Core builtins are registered through a
// separate path (AddBuiltin) and are covered instead by the static grammar gate
// (internal/le/cli/grammar/register.go) walking the YANG command tree.
//
// validVerbList returns the sorted, comma-separated list of valid command verbs,
// derived from the canonical command.Verbs registry (internal/component/command)
// so there is no second verb list to drift (ai/rules/evidence.md).
func validVerbList() string {
	return textbuf.Join(command.VerbList(), ", ")
}

// validateCommandName checks that a command name is well-formed for dispatch.
// A name is one or more single-space-separated tokens; each token is ASCII
// lowercase letters, digits, and interior hyphens. The first token must be a
// known verb (commandVerbs). Rejecting uppercase, Unicode, repeated whitespace,
// empty tokens, and unknown verbs prevents command shadowing and ambiguous
// dispatch keys leaking into every command surface.
func validateCommandName(name string) error {
	if name == "" {
		return errCommandNameCannotBeEmpty
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("command name %q has leading or trailing whitespace", name)
	}
	tokens := strings.Split(name, " ")
	for _, tok := range tokens {
		if tok == "" {
			return fmt.Errorf("command name %q contains repeated whitespace", name)
		}
		if err := validateCommandToken(name, tok); err != nil {
			return err
		}
	}
	if !command.IsVerb(tokens[0]) {
		return fmt.Errorf("command name %q has unknown verb %q (valid verbs: %s)", name, tokens[0], validVerbList())
	}
	// The remaining grammar rules (e.g. mutation tokens like add/remove, R7) come
	// from the shared checker so plugin commands obey the same grammar as the gate.
	if findings := grammar.CheckName(name); len(findings) > 0 {
		f := findings[0]
		return fmt.Errorf("command name %q violates %s: %s", name, f.Rule, f.Message)
	}
	return nil
}

// validateCommandToken checks a single non-empty command token: ASCII lowercase
// letters, digits, and hyphens only, with no leading or trailing hyphen. name is
// included for error context.
func validateCommandToken(name, tok string) error {
	if tok[0] == '-' || tok[len(tok)-1] == '-' {
		return fmt.Errorf("command name %q has a token %q with a leading or trailing hyphen", name, tok)
	}
	for _, r := range tok {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			// allowed
		default:
			return fmt.Errorf("command name %q contains invalid character %q (only lowercase ASCII letters, digits, hyphens within tokens, and single spaces allowed)", name, string(r))
		}
	}
	return nil
}

// Default timeouts for plugin commands.
const (
	DefaultCommandTimeout = 30 * time.Second
	CompletionTimeout     = 500 * time.Millisecond
)

// summaryKey is the JSON key a command's one-line summary is published under,
// and retiredSummaryKey is the key it was published under until 2026-09-03.
// Both are spelled here because a refusal has to name them, and the Completion
// tag below is the declaration they repeat.
const (
	summaryKey        = "short-help"
	retiredSummaryKey = "help"
)

// Completion represents a single completion suggestion.
// Used for both command and argument completion.
type Completion struct {
	Value string `json:"value"` // The completion text
	// ShortHelp is the one-line SUMMARY of the command. Every surface that
	// shows the command on one line reads it.
	ShortHelp string `json:"short-help,omitempty"`
	Source    string `json:"source,omitempty"` // "builtin" or process name (verbose mode)
	Hidden    bool   `json:"hidden,omitempty"` // Hidden from completion tree (works when typed in full)
	// Description is the explanation the command's own help page prints. Empty
	// means the command declares none, and the key is then absent. It is NEVER
	// read as a summary, and no one-line surface reads it at all.
	Description string `json:"description,omitempty"`
	// Completable says the daemon completes the command's argument from its
	// runtime state, through `system command complete args <cmd>`. Only a
	// `system command list` row carries it; absent means it does not.
	Completable bool `json:"completable,omitempty"`
}

// CommandDef describes a command to register.
// Passed from process to registry during registration.
type CommandDef struct {
	Name        string        // Command name (e.g., "myapp status")
	ShortHelp   string        // One-line summary, shown wherever the command appears on one line
	Description string        // Long explanation, printed by this command's own help page. Empty = the plugin declared none
	Args        string        // Usage hint (e.g., "<component>")
	Completable bool          // Process handles arg completion
	Hidden      bool          // Hidden from completion and help (works when typed in full)
	Timeout     time.Duration // Per-command timeout (0 = default 30s)
}

// RegisterResult holds the result of a single command registration.
type RegisterResult struct {
	Name  string // Command that was registered
	OK    bool   // True if registration succeeded
	Error string // Error message if failed
	// Owner names who holds the name when the refusal is a conflict: another
	// plugin's process name, or "builtin". Empty for any other refusal.
	Owner string
}

// RegisteredCommand represents a plugin command in the registry.
type RegisteredCommand struct {
	Name      string
	LowerName string // Pre-lowercased at registration for dispatch matching (zero alloc per lookup)
	ShortHelp string // One-line summary (CommandDecl.ShortHelp)
	// Description is the explanation this command's own help page prints
	// (CommandDecl.Description). Empty means the plugin declared none, and the
	// help page then prints the summary alone. It is NEVER read as a summary,
	// and no one-line surface reads it at all.
	Description  string
	Args         string           // Usage hint (e.g., "<component>")
	Completable  bool             // Process handles arg completion
	Hidden       bool             // Hidden from completion and help (works when typed in full)
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
	// commandTree is immutable schema metadata, bound before process commands
	// register. SDK declarations can hide a command but cannot unhide its path.
	commandTree *command.Node

	// deprecatedWarned tracks old command names that have already logged a
	// deprecation warning this session (log once per name).
	deprecatedWarned sync.Map

	// frozen holds an immutable snapshot for lock-free Lookup after startup.
	// nil before Freeze() is called.
	frozen atomic.Pointer[frozenCommands]
}

// newCommandRegistry creates a new command registry.
func newCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands:   make(map[string]*RegisteredCommand),
		builtins:   make(map[string]bool),
		deprecated: make(map[string]*deprecatedAlias),
	}
}

// setCommandTree binds the shared schema before plugin startup. The caller
// MUST finish this before Register accepts any process declarations.
func (r *CommandRegistry) setCommandTree(tree *command.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commandTree = tree
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
// A loader binding schema visibility MUST call setCommandTree before this.
//
// If frozen, publishes a new snapshot reflecting the addition, exactly as
// Unregister does for a removal. Freeze happens once startup's phases are over
// (signalStartupComplete, startup.go), and registration does NOT stop there: a
// plugin auto-loaded by a config reload and a plugin restarted after a broken
// rollback both declare their commands afterwards. Without the republish, Lookup
// keeps answering from a snapshot those commands are not in, so the plugin runs
// with every command it declared invisible.
func (r *CommandRegistry) Register(proc *process.Process, defs []CommandDef) []RegisterResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.republishFrozen()

	results := make([]RegisterResult, len(defs))
	now := time.Now()

	for i, def := range defs {
		key := strings.ToLower(def.Name)
		results[i].Name = def.Name

		if refusal, owner, refused := commandNameRefusal(def.Name, r.builtins, r.commands); refused {
			results[i].OK = false
			results[i].Error = refusal
			results[i].Owner = owner
			continue
		}

		// Apply default timeout
		timeout := def.Timeout
		if timeout == 0 {
			timeout = DefaultCommandTimeout
		}

		hidden := def.Hidden
		if command.HiddenAtPath(r.commandTree, strings.Fields(def.Name)) {
			hidden = true
		}
		// Register
		r.commands[key] = &RegisteredCommand{
			Name:         def.Name,
			LowerName:    key,
			ShortHelp:    def.ShortHelp,
			Description:  def.Description,
			Args:         def.Args,
			Completable:  def.Completable,
			Hidden:       hidden,
			Timeout:      timeout,
			Process:      proc,
			RegisteredAt: now,
		}
		results[i].OK = true
	}

	return results
}

// commandNameRefusal answers why Register refuses the command name, given the
// builtin names no plugin may take and the commands registered so far, both
// keyed by lower-case name, and who holds the name when the refusal is a
// conflict (another plugin's process name, or CommandOwnerBuiltin). refused is
// false when Register accepts the name.
//
// It is the one rule that decides which declared names reach the dispatcher:
// Register applies it at Stage 5, and the Stage 1 check command-nodes-served
// counts only a name it accepts (commandServed), so the check never calls a
// node served by a name the dispatcher would never route.
func commandNameRefusal(name string, builtins map[string]bool, commands map[string]*RegisteredCommand) (refusal, owner string, refused bool) {
	if err := validateCommandName(name); err != nil {
		return err.Error(), "", true
	}
	key := strings.ToLower(name)
	var text textbuf.Buffer
	if builtins[key] {
		return text.Str("conflicts with builtin: ").Str(name).String(), CommandOwnerBuiltin, true
	}
	if existing, held := commands[key]; held {
		holder := existing.Process.Config().Name
		return text.Str("already registered by process: ").Str(holder).String(), holder, true
	}
	return "", "", false
}

// commandRouting is what decides, beside one plugin's own declaration, which
// plugin a command line reaches: the builtin names no plugin may take, and the
// commands other plugins registered, both keyed by lower-case name. The daemon
// reads it from its registry (CommandRegistry.routingBeside); a checker that
// runs one program in isolation holds no daemon, and judges with none of either
// (programAlone).
type commandRouting struct {
	builtins map[string]bool
	commands map[string]*RegisteredCommand
}

// routingBeside answers what the registry holds now beside the plugin named
// name: every builtin name, and the commands of every other plugin. At Stage 1
// that is the commands of the plugins that reached Stage 5 before this one, in
// an earlier tier or before this plugin was started again; a plugin of the same
// tier that has not reached Stage 5 registered nothing yet, so a command it
// declares is not seen here. The plugin's own commands are left out: a plugin
// started again registers them again at its own Stage 5. Safe for concurrent
// use: the answer is a copy.
func (r *CommandRegistry) routingBeside(name string) commandRouting {
	r.mu.RLock()
	defer r.mu.RUnlock()
	routing := commandRouting{
		builtins: maps.Clone(r.builtins),
		commands: make(map[string]*RegisteredCommand, len(r.commands)),
	}
	for key, cmd := range r.commands {
		if cmd.Process.Config().Name == name {
			continue
		}
		routing.commands[key] = cmd
	}
	return routing
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

// unregisterAll removes all commands and deprecated aliases owned by the process.
// Called when a process dies.
// If frozen, publishes a new snapshot reflecting the removal.
func (r *CommandRegistry) unregisterAll(proc *process.Process) {
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

// registerDeprecated adds a deprecated alias that maps oldName to the
// canonical command registered under newName. When the old name is looked
// up, the canonical RegisteredCommand is returned and a deprecation
// warning is logged once per session.
//
// The alias name is validated with the same parser as a real command, and the
// alias is rejected if it conflicts with a builtin, an already-registered
// command, or an existing alias, or if the canonical command is not registered.
// This makes an unreachable or shadowing alias impossible to register.
//
// Requiring the canonical to be already registered is safe because a plugin's
// deprecated aliases (CommandDeprecatedNames) reference that same plugin's
// commands, which startup registers immediately before the aliases.
//
// If frozen, publishes a new snapshot reflecting the addition, for the reason
// Register states: an alias declared after Freeze is otherwise absent from the
// snapshot Lookup reads.
func (r *CommandRegistry) registerDeprecated(proc *process.Process, oldName, newName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.republishFrozen()

	if err := validateCommandName(oldName); err != nil {
		return fmt.Errorf("deprecated alias %q: %w", oldName, err)
	}

	oldKey := strings.ToLower(oldName)
	newKey := strings.ToLower(newName)

	if r.builtins[oldKey] {
		return &commandOwnedError{
			message: fmt.Errorf("deprecated alias %q conflicts with builtin command", oldName).Error(),
			owner:   CommandOwnerBuiltin,
		}
	}
	if existing, ok := r.commands[oldKey]; ok {
		owner := existing.Process.Config().Name
		return &commandOwnedError{
			message: fmt.Errorf("deprecated alias %q conflicts with command registered by process %s", oldName, owner).Error(),
			owner:   owner,
		}
	}
	if alias, ok := r.deprecated[oldKey]; ok {
		owner := ""
		if alias.Process != nil {
			owner = alias.Process.Config().Name
		}
		return &commandOwnedError{
			message: fmt.Errorf("deprecated alias %q already registered", oldName).Error(),
			owner:   owner,
		}
	}
	if _, ok := r.commands[newKey]; !ok {
		return fmt.Errorf("deprecated alias %q points to unregistered command %q", oldName, newName)
	}

	r.deprecated[oldKey] = &deprecatedAlias{
		OldName:      oldName,
		NewLowerName: newKey,
		Process:      proc,
	}
	return nil
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

// hasCommandPath reports whether lowerName is registered, under its own name or
// under a deprecated alias that still resolves. lowerName must already be
// lowercased by the caller, which registration keys always are.
//
// It answers the dispatch guard in matchBuiltinTokens, and it logs no
// deprecation warning: the guard asks whether a path exists, and asking is not
// invoking.
func (r *CommandRegistry) hasCommandPath(lowerName string) bool {
	if snap := r.frozen.Load(); snap != nil {
		if snap.commands[lowerName] != nil {
			return true
		}
		alias := snap.deprecated[lowerName]
		return alias != nil && snap.commands[alias.NewLowerName] != nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.commands[lowerName] != nil {
		return true
	}
	alias := r.deprecated[lowerName]
	return alias != nil && r.commands[alias.NewLowerName] != nil
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

// warnDeprecated logs a deprecation warning and raises its doctor event once
// per session per old name (command_events.go deprecatedUsed).
func (r *CommandRegistry) warnDeprecated(oldName, newName string) {
	if _, loaded := r.deprecatedWarned.LoadOrStore(strings.ToLower(oldName), true); !loaded {
		deprecatedUsed(oldName, newName)
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

// CommandCountsByProcess returns how many registered commands each process owns,
// keyed by process name.
//
// This registry is the answer, not a list kept on the Process. Process carried a
// registeredCommands mirror until 2026-08-18. Its writers were the text-protocol
// register and unregister handlers. The YANG RPC migration deleted those handlers
// and registered here instead, so the mirror stayed empty and every reader of it
// saw zero. Counting the entries dispatch resolves against cannot drift from what
// dispatch does.
//
// A command with no owning process is skipped. Nothing registers one today, and a
// count attributed to no plugin would be a number with no row to sit in.
func (r *CommandRegistry) CommandCountsByProcess() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[string]int)
	for _, cmd := range r.commands {
		if cmd.Process == nil {
			continue
		}
		counts[cmd.Process.Name()]++
	}
	return counts
}

// VisibleCommandEntries returns completion-tree entries for every registered
// command that command.ListedOnDaemonSurfaces lists: the surface predicate, not
// this registry, decides, and it lists no hidden command, so a hidden one never
// surfaces in tab-completion or help (it still dispatches when typed in full via
// Lookup). Every daemon surface that lists plugin commands (the SSH tree, the
// web admin tree, the web terminal completer) reads this one answer.
// Used to inject plugin-registered commands into the operational command tree
// (command.MergeCommandPaths) so interactive tab-completion offers them, matching
// the shell-completion path that already reads Complete().
//
// Each entry carries both help texts, under the names both packages spell them
// with: ShortHelp for the summary and Description for the explanation.
// MergeCommandPaths fills each field on its own, so a command that declared a
// summary and no explanation fills the summary alone.
func (r *CommandRegistry) VisibleCommandEntries() []command.CommandEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]command.CommandEntry, 0, len(r.commands))
	for _, cmd := range r.commands {
		listed, err := command.ListedOnDaemonSurfaces(command.CommandRegistration{
			Path: cmd.Name, Kind: command.KindPlugin, Hidden: cmd.Hidden,
		})
		if err != nil {
			// A plugin registration always has a kind and every daemon surface
			// answers it alike, so only a defect in the predicate lands here.
			panic("BUG: VisibleCommandEntries: " + err.Error())
		}
		if !listed {
			continue
		}
		entries = append(entries, command.CommandEntry{
			Name:        cmd.Name,
			ShortHelp:   cmd.ShortHelp,
			Description: cmd.Description,
		})
	}
	return entries
}

// Complete returns commands matching the partial input.
// Used for CLI command completion.
func (r *CommandRegistry) Complete(partial string) []Completion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	partial = strings.ToLower(partial)
	var completions []Completion

	for key, cmd := range r.commands {
		if cmd.Hidden {
			continue
		}
		if strings.HasPrefix(key, partial) {
			completions = append(completions, Completion{
				Value:     cmd.Name,
				ShortHelp: cmd.ShortHelp,
				Source:    cmd.Process.Config().Name,
			})
		}
	}

	return completions
}

// lookupDeprecatedPrefix finds the longest deprecated alias that is a prefix
// of lowerInput (already lowercased by the caller). Returns the canonical
// RegisteredCommand and the matched prefix length, or (nil, 0) if no
// deprecated alias matches. Logs a deprecation warning on first match.
func (r *CommandRegistry) lookupDeprecatedPrefix(lowerInput string) (*RegisteredCommand, int) {
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
