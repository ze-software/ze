// Design: docs/architecture/api/process-protocol.md — plugin process management

package server

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/authz"
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
)

// ErrUnknownCommand is returned when a command is not recognized.
var ErrUnknownCommand = errors.New("unknown command")

// ErrEmptyCommand is returned when the command is empty.
var ErrEmptyCommand = errors.New("empty command")

// ErrUnauthorized is returned when a command is denied by authorization.
var ErrUnauthorized = errors.New("unauthorized")

// ErrPluginProcessNotRunning is returned when a plugin command targets a non-running process.
var ErrPluginProcessNotRunning = errors.New("plugin process not running")

// ErrPluginConnectionClosed is returned when the plugin's connection is no longer available.
var ErrPluginConnectionClosed = errors.New("plugin connection closed")

// AllBuiltinRPCs returns all RPCs registered via init() + RegisterRPCs().
// Includes server, handler, and editor RPCs (when their packages are imported).
func AllBuiltinRPCs() []RPCRegistration {
	return registeredRPCs
}

// BuiltinCount returns the number of registered builtin handlers.
func BuiltinCount() int {
	return len(AllBuiltinRPCs())
}

// LoadBuiltins registers all builtin handlers with the dispatcher.
// The wireToPath map provides the dispatch key for each handler, derived from
// the YANG command tree (WireMethod -> CLI path). Handlers without a YANG
// entry (e.g., editor-internal "run"/"edit") are skipped.
func LoadBuiltins(d *Dispatcher, wireToPath map[string]string) {
	for _, reg := range AllBuiltinRPCs() {
		name := wireToPath[reg.WireMethod]
		if name == "" {
			continue // No YANG tree entry (editor-internal)
		}
		d.RegisterWithOptions(name, reg.Handler, reg.Help, RegisterOptions{
			ReadOnly:         reg.ReadOnly,
			RequiresSelector: reg.RequiresSelector,
			PluginProxy:      reg.PluginCommand != "",
		})
	}
}

// RegisterDefaultHandlers registers all builtin handlers with the dispatcher.
func RegisterDefaultHandlers(d *Dispatcher, wireToPath map[string]string) {
	LoadBuiltins(d, wireToPath)
}

// Handler processes a command and returns a response.
type Handler func(ctx *CommandContext, args []string) (*plugin.Response, error)

// Authorizer checks whether a user is allowed to execute a command.
type Authorizer interface {
	Authorize(username, command string, isReadOnly bool) authz.Action
}

// CommandContext provides access to reactor and session state.
// Dependencies are accessed through Server; per-request state is stored directly.
type CommandContext struct {
	Server   *Server          // Gateway to all server state (reactor, dispatcher, etc.)
	Process  *process.Process // The API process (for session state)
	Peer     string           // Peer selector: "*" for all, or specific IP. Empty = "*"
	Username string           // Authenticated username (empty = no auth, full access)
}

// Reactor returns the reactor lifecycle interface via Server. Nil-safe: returns nil if Server is nil.
func (c *CommandContext) Reactor() plugin.ReactorLifecycle {
	if c.Server == nil {
		return nil
	}
	return c.Server.Reactor()
}

// Dispatcher returns the command dispatcher via Server. Nil-safe: returns nil if Server is nil.
func (c *CommandContext) Dispatcher() *Dispatcher {
	if c.Server == nil {
		return nil
	}
	return c.Server.Dispatcher()
}

// CommitManager returns the commit manager via Server. Nil-safe: returns nil if Server is nil.
func (c *CommandContext) CommitManager() any {
	if c.Server == nil {
		return nil
	}
	return c.Server.CommitManager()
}

// Subscriptions returns the subscription manager via Server. Nil-safe: returns nil if Server is nil.
func (c *CommandContext) Subscriptions() *SubscriptionManager {
	if c.Server == nil {
		return nil
	}
	return c.Server.Subscriptions()
}

// RequireReactor returns the reactor or an error response if not available.
func RequireReactor(ctx *CommandContext) (plugin.ReactorLifecycle, *plugin.Response, error) {
	r := ctx.Reactor()
	if r == nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Data:   "reactor not available",
		}, fmt.Errorf("reactor not available")
	}
	return r, nil, nil
}

// PeerSelector returns the effective neighbor selector.
// Returns "*" if no neighbor was specified.
func (c *CommandContext) PeerSelector() string {
	if c.Peer == "" {
		return "*"
	}
	return c.Peer
}

// Command represents a registered command with metadata.
type Command struct {
	Name             string
	Handler          Handler
	Help             string
	ReadOnly         bool // True if command only reads state (safe for "ze show")
	RequiresSelector bool // True if command requires explicit peer selector (not default "*")
}

// RegisterOptions holds optional settings for command registration.
type RegisterOptions struct {
	ReadOnly         bool // True if command only reads state
	RequiresSelector bool // True if "bgp peer <command>" must have an explicit peer selector
	PluginProxy      bool // True if this builtin proxies to a plugin command (allows plugin to register same name)
}

// Dispatcher routes commands to handlers.
type Dispatcher struct {
	commands   map[string]*Command
	sortedKeys []string          // sorted keys for longest-match lookup (longest first)
	registry   *CommandRegistry  // Plugin commands
	pending    *PendingRequests  // In-flight plugin requests
	subsystems *SubsystemManager // Forked subsystem processes
	authorizer Authorizer        // Authorization checker (nil = allow all)
}

// NewDispatcher creates a new command dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		commands:   make(map[string]*Command),
		registry:   NewCommandRegistry(),
		pending:    NewPendingRequests(),
		subsystems: NewSubsystemManager(),
	}
}

// HasCommandPrefix returns true if the input matches any registered command prefix
// (builtin or plugin). Used by dispatch routing to distinguish top-level commands
// from peer subcommands that need "peer <selector> " prepended.
func (d *Dispatcher) HasCommandPrefix(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	// Check builtin commands.
	for _, key := range d.sortedKeys {
		if strings.HasPrefix(lower, key) && (len(lower) == len(key) || lower[len(key)] == ' ') {
			return true
		}
	}
	// Check plugin registry commands.
	if d.registry != nil {
		for _, cmd := range d.registry.All() {
			key := strings.ToLower(cmd.Name)
			if strings.HasPrefix(lower, key) && (len(lower) == len(key) || lower[len(key)] == ' ') {
				return true
			}
		}
	}
	return false
}

// SetAuthorizer sets the authorization checker for the dispatcher.
// When set, all commands are checked against the authorizer before execution.
func (d *Dispatcher) SetAuthorizer(a Authorizer) {
	d.authorizer = a
}

// Subsystems returns the subsystem manager.
func (d *Dispatcher) Subsystems() *SubsystemManager {
	return d.subsystems
}

// SetSubsystems sets the subsystem manager.
func (d *Dispatcher) SetSubsystems(sm *SubsystemManager) {
	d.subsystems = sm
}

// Registry returns the plugin command registry.
func (d *Dispatcher) Registry() *CommandRegistry {
	return d.registry
}

// Pending returns the pending requests tracker.
func (d *Dispatcher) Pending() *PendingRequests {
	return d.pending
}

// Register adds a builtin command handler.
// Also marks the command as builtin in the registry to prevent shadowing.
func (d *Dispatcher) Register(name string, handler Handler, help string) {
	// Store with lowercase key for case-insensitive matching
	key := strings.ToLower(name)
	d.commands[key] = &Command{
		Name:    name,
		Handler: handler,
		Help:    help,
	}
	d.updateSortedKeys()

	// Mark as builtin to prevent plugin shadowing
	d.registry.AddBuiltin(name)
}

// RegisterWithOptions adds a builtin command handler with additional options.
func (d *Dispatcher) RegisterWithOptions(name string, handler Handler, help string, opts RegisterOptions) {
	key := strings.ToLower(name)
	d.commands[key] = &Command{
		Name:             name,
		Handler:          handler,
		Help:             help,
		ReadOnly:         opts.ReadOnly,
		RequiresSelector: opts.RequiresSelector,
	}
	d.updateSortedKeys()

	// Plugin proxy handlers must not block the plugin from registering
	// the same command name in the CommandRegistry. ForwardToPlugin needs
	// the plugin's own registration to route the command to the process.
	if !opts.PluginProxy {
		d.registry.AddBuiltin(name)
	}
}

// updateSortedKeys rebuilds the sorted key list for longest-match lookup.
func (d *Dispatcher) updateSortedKeys() {
	d.sortedKeys = make([]string, 0, len(d.commands))
	for k := range d.commands {
		d.sortedKeys = append(d.sortedKeys, k)
	}
	// Sort by length descending (longest first)
	sort.Slice(d.sortedKeys, func(i, j int) bool {
		return len(d.sortedKeys[i]) > len(d.sortedKeys[j])
	})
}

// Lookup finds a command by exact name.
func (d *Dispatcher) Lookup(name string) *Command {
	return d.commands[strings.ToLower(name)]
}

// Commands returns all registered commands.
func (d *Dispatcher) Commands() []*Command {
	result := make([]*Command, 0, len(d.commands))
	for _, cmd := range d.commands {
		result = append(result, cmd)
	}
	return result
}

// IsAuthorized checks if the user is allowed to execute the command.
// Exported for use by streaming handlers (e.g., monitor) that bypass the normal dispatch path.
func (d *Dispatcher) IsAuthorized(ctx *CommandContext, input string, readOnly bool) bool {
	return d.isAuthorized(ctx, input, readOnly)
}

// isAuthorized checks if the user is allowed to execute the command.
func (d *Dispatcher) isAuthorized(ctx *CommandContext, input string, readOnly bool) bool {
	if d.authorizer == nil {
		return true
	}
	username := ""
	if ctx != nil {
		username = ctx.Username
	}
	return d.authorizer.Authorize(username, input, readOnly) != authz.Deny
}

// Dispatch parses and executes a command.
// Supports peer selector prefix: "peer <addr|name|*> <command>".
// If no peer prefix, defaults to all peers ("*").
// Priority: 1) builtin commands, 2) forked subsystems, 3) plugin registry.
func (d *Dispatcher) Dispatch(ctx *CommandContext, input string) (*plugin.Response, error) {
	tokens := tokenize(input)
	if len(tokens) == 0 {
		return nil, ErrEmptyCommand
	}

	// Extract peer selector from "peer <selector>" at any position in the command.
	// Format: ... peer <addr|name|*> <rest>
	// Selector can be an IP address, glob pattern, peer name, or "*" for all.
	// The selector is stripped from the input so it matches the registered YANG path.
	peerSelector, hasExplicitSelector, selectorIdx := extractPeerSelector(ctx, tokens)
	if hasExplicitSelector {
		if ctx != nil {
			ctx.Peer = peerSelector
		}
		input = rebuildWithoutSelector(tokens, selectorIdx)
	}

	// Build lowercase input for matching
	lowerInput := strings.ToLower(input)
	lowerInput = strings.TrimSpace(lowerInput)

	// Find longest matching builtin command prefix
	var matchedCmd *Command
	var matchedLen int

	for _, key := range d.sortedKeys {
		if strings.HasPrefix(lowerInput, key) {
			// Check it's a word boundary (end of input or followed by space)
			if len(lowerInput) == len(key) || lowerInput[len(key)] == ' ' {
				matchedCmd = d.commands[key]
				matchedLen = len(key)
				break // sortedKeys is longest-first, so first match is best
			}
		}
	}

	// Enforce peer selector for commands that require it
	if matchedCmd != nil && matchedCmd.RequiresSelector && !hasExplicitSelector {
		return nil, fmt.Errorf("%s requires a peer selector: peer <address> %s",
			matchedCmd.Name,
			strings.TrimPrefix(strings.ToLower(matchedCmd.Name), "peer "))
	}

	// Authorization check — after command resolution, before execution
	if matchedCmd != nil && !d.isAuthorized(ctx, input, matchedCmd.ReadOnly) {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("authorization denied for %q", input),
		}, ErrUnauthorized
	}

	// If no builtin match, try forked subsystems and plugin registry.
	// Authorization applies to these paths too — treat as non-ReadOnly (write).
	if matchedCmd == nil {
		if !d.isAuthorized(ctx, input, false) {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("authorization denied for %q", input),
			}, ErrUnauthorized
		}
		if d.subsystems != nil {
			if handler := d.subsystems.FindHandler(input); handler != nil {
				return d.dispatchSubsystem(ctx, handler, input)
			}
		}
		// No subsystem match, try plugin registry
		return d.dispatchPlugin(ctx, input, peerSelector)
	}

	// Extract remaining args
	remaining := strings.TrimSpace(input[matchedLen:])
	var args []string
	if remaining != "" {
		args = tokenize(remaining)
	}

	// Execute handler
	if matchedCmd.Handler == nil {
		return &plugin.Response{Status: plugin.StatusDone}, nil
	}

	return matchedCmd.Handler(ctx, args)
}

// dispatchSubsystem routes a command to a forked subsystem process.
func (d *Dispatcher) dispatchSubsystem(_ *CommandContext, handler *SubsystemHandler, input string) (*plugin.Response, error) {
	// TODO: Pass context from CommandContext when reactor provides it
	return handler.Handle(context.Background(), input)
}

// ForwardToPlugin routes a command to a plugin process by exact name lookup.
// Used by proxy handlers that bridge CLI builtins to plugin commands.
// Returns ErrUnknownCommand if the command is not registered (plugin may not be running).
func (d *Dispatcher) ForwardToPlugin(command string, args []string, peerSelector string) (*plugin.Response, error) {
	cmd := d.registry.Lookup(command)
	if cmd == nil {
		return nil, fmt.Errorf("plugin command %q not registered (plugin may not be running): %w", command, ErrUnknownCommand)
	}
	return d.routeToProcess(cmd, args, peerSelector)
}

// dispatchPlugin routes a command to a plugin process.
func (d *Dispatcher) dispatchPlugin(_ *CommandContext, input, peerSelector string) (*plugin.Response, error) {
	lowerInput := strings.ToLower(strings.TrimSpace(input))

	// Find longest matching plugin command
	var matchedPlugin *RegisteredCommand
	var matchedLen int

	for _, cmd := range d.registry.All() {
		key := strings.ToLower(cmd.Name)
		if strings.HasPrefix(lowerInput, key) {
			// Check it's a word boundary
			if len(lowerInput) == len(key) || lowerInput[len(key)] == ' ' {
				if len(key) > matchedLen {
					matchedPlugin = cmd
					matchedLen = len(key)
				}
			}
		}
	}

	if matchedPlugin == nil {
		all := d.registry.All()
		names := make([]string, len(all))
		for i, c := range all {
			names[i] = c.Name
		}
		logger().Debug("dispatchPlugin: no match", "input", lowerInput, "registry_count", len(all), "registered", strings.Join(names, ", "))
		return nil, ErrUnknownCommand
	}

	// Extract remaining args
	remaining := strings.TrimSpace(input[matchedLen:])
	var args []string
	if remaining != "" {
		args = tokenize(remaining)
	}

	// Route to process
	return d.routeToProcess(matchedPlugin, args, peerSelector)
}

// routeToProcess sends a command request to a plugin process via synchronous RPC.
func (d *Dispatcher) routeToProcess(cmd *RegisteredCommand, args []string, peerSelector string) (*plugin.Response, error) {
	proc := cmd.Process
	if proc == nil || !proc.Running() {
		return nil, ErrPluginProcessNotRunning
	}

	conn := proc.Conn()
	if conn == nil {
		return nil, ErrPluginConnectionClosed
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
	defer cancel()

	rpcOut, err := conn.SendExecuteCommand(ctx, "", cmd.Name, args, peerSelector)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Data: "failed to send request: " + err.Error()}, nil
	}
	if rpcOut != nil {
		return &plugin.Response{Status: rpcOut.Status, Data: rpcOut.Data}, nil
	}
	return &plugin.Response{Status: plugin.StatusDone}, nil
}

// tokenize splits a command string into tokens.
// Handles quoted strings: "hello world" → single token "hello world".
// Supports backslash escaping: \" for literal quote, \\ for literal backslash.
// Quotes are stripped from the result.
func tokenize(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var tokens []string
	var current strings.Builder
	inQuote := false
	escape := false

	for _, r := range input {
		if escape {
			current.WriteRune(r)
			escape = false
			continue
		}

		if r == '\\' {
			escape = true
			continue
		}

		if r == '"' {
			inQuote = !inQuote
			continue
		}

		if (r == ' ' || r == '\t') && !inQuote {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// looksLikeIPOrGlob returns true if s looks like an IP address or glob pattern.
// Examples: "*", "192.168.1.1", "192.168.*.*", "2001:db8::1", "10.0.0.1,10.0.0.2".
func looksLikeIPOrGlob(s string) bool {
	// Wildcard all
	if s == "*" {
		return true
	}
	// Contains comma (multi-IP: ip,ip,ip)
	if strings.Contains(s, ",") {
		return true
	}
	// Contains dots (IPv4 or IPv4 glob)
	if strings.Contains(s, ".") {
		return true
	}
	// Contains colons (IPv6)
	if strings.Contains(s, ":") {
		return true
	}
	return false
}

// isKnownPeerName checks whether s matches the name of any configured peer.
// Uses the reactor's peer list for an exact match. Returns false if the
// reactor is unavailable (e.g., during shell completion without a running daemon).
func isKnownPeerName(ctx *CommandContext, s string) bool {
	if ctx == nil || ctx.Reactor() == nil {
		return false
	}
	peers := ctx.Reactor().Peers()
	for i := range peers {
		if peers[i].Name == s {
			return true
		}
	}
	return false
}

// looksLikeASNSelector returns true if s looks like an ASN selector: "as" prefix
// (case-insensitive) followed by a valid 32-bit unsigned integer
// (e.g., "as65001", "AS65001", "As4294967295").
func looksLikeASNSelector(s string) bool {
	if len(s) < 3 || (s[0] != 'a' && s[0] != 'A') || (s[1] != 's' && s[1] != 'S') {
		return false
	}
	_, err := strconv.ParseUint(s[2:], 10, 32)
	return err == nil
}

// extractPeerSelector scans tokens for "peer <selector>" at any position.
// Returns the selector, whether one was found, and the index of the selector token.
// The selector is a token that looks like an IP, glob, ASN selector, or known peer name.
// If no selector is found, returns ("*", false, -1).
func extractPeerSelector(ctx *CommandContext, tokens []string) (string, bool, int) {
	for i, tok := range tokens {
		if !strings.EqualFold(tok, "peer") {
			continue
		}
		if i+1 >= len(tokens) {
			continue
		}
		candidate := tokens[i+1]
		if looksLikeIPOrGlob(candidate) || looksLikeASNSelector(candidate) || isKnownPeerName(ctx, candidate) {
			return candidate, true, i + 1
		}
	}
	return "*", false, -1
}

// rebuildWithoutSelector rebuilds a token list, removing the token at selectorIdx.
// Used after extractPeerSelector found a selector to produce a YANG-matching path.
func rebuildWithoutSelector(tokens []string, selectorIdx int) string {
	out := make([]string, 0, len(tokens)-1)
	for i, tok := range tokens {
		if i == selectorIdx {
			continue
		}
		out = append(out, tok)
	}
	return strings.Join(out, " ")
}
