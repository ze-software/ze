package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// ErrUnknownCommand is returned when a command is not recognized.
var ErrUnknownCommand = errors.New("unknown command")

// ErrEmptyCommand is returned when the command is empty.
var ErrEmptyCommand = errors.New("empty command")

// builtinRegistry holds handlers registered via init() for self-registration.
// This eliminates the need for a central RegisterDefaultHandlers() function.
var builtinRegistry = &handlerRegistry{handlers: make(map[string]*handlerDef)}

// handlerDef describes a builtin command handler.
type handlerDef struct {
	Name    string
	Handler Handler
	Help    string
}

// handlerRegistry collects builtin handlers via init().
type handlerRegistry struct {
	handlers map[string]*handlerDef
}

// RegisterBuiltin registers a builtin handler (called from init()).
// Panics if a handler with the same name is already registered (catches bugs early).
func RegisterBuiltin(name string, handler Handler, help string) {
	if _, exists := builtinRegistry.handlers[name]; exists {
		panic("duplicate handler registration: " + name)
	}
	builtinRegistry.handlers[name] = &handlerDef{
		Name:    name,
		Handler: handler,
		Help:    help,
	}
}

// BuiltinCount returns the number of registered builtin handlers.
func BuiltinCount() int {
	return len(builtinRegistry.handlers)
}

// LoadBuiltins registers all builtin handlers with the dispatcher.
// Called by NewServer to load handlers from the init() registry.
func LoadBuiltins(d *Dispatcher) {
	for _, h := range builtinRegistry.handlers {
		d.Register(h.Name, h.Handler, h.Help)
	}
}

// RegisterDefaultHandlers registers all builtin handlers with the dispatcher.
// This is an alias for LoadBuiltins() for backward compatibility with tests.
func RegisterDefaultHandlers(d *Dispatcher) {
	LoadBuiltins(d)
}

// Handler processes a command and returns a response.
type Handler func(ctx *CommandContext, args []string) (*Response, error)

// CommandContext provides access to reactor and session state.
type CommandContext struct {
	Reactor       ReactorInterface
	Encoder       *JSONEncoder
	CommitManager *CommitManager
	Dispatcher    *Dispatcher          // For accessing registry/commands
	Process       *Process             // The API process (for session state)
	Subscriptions *SubscriptionManager // Event subscription tracking
	Peer          string               // Peer selector: "*" for all, or specific IP. Empty = "*"
	Serial        string               // Command serial from #N prefix (empty = no ack)
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
	Name    string
	Handler Handler
	Help    string
}

// Dispatcher routes commands to handlers.
type Dispatcher struct {
	commands   map[string]*Command
	sortedKeys []string          // sorted keys for longest-match lookup (longest first)
	registry   *CommandRegistry  // Plugin commands
	pending    *PendingRequests  // In-flight plugin requests
	subsystems *SubsystemManager // Forked subsystem processes
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

// Dispatch parses and executes a command.
// Supports bgp peer prefix: "bgp peer <addr> <command>" or "bgp peer * <command>".
// If no peer prefix, defaults to all peers ("*").
// Priority: 1) builtin commands, 2) forked subsystems, 3) plugin registry.
func (d *Dispatcher) Dispatch(ctx *CommandContext, input string) (*Response, error) {
	tokens := tokenize(input)
	if len(tokens) == 0 {
		return nil, ErrEmptyCommand
	}

	// Check for "bgp peer <selector>" prefix
	// Format: bgp peer <addr|*> <command>
	peerSelector := "*"
	if len(tokens) >= 4 && strings.EqualFold(tokens[0], "bgp") && strings.EqualFold(tokens[1], "peer") {
		// Check if third token looks like IP/glob (contains dots, colons, or is "*")
		if looksLikeIPOrGlob(tokens[2]) {
			peerSelector = tokens[2]
			if ctx != nil {
				ctx.Peer = peerSelector
			}
			// Rebuild input: "bgp peer <command>" (without the selector)
			input = "bgp peer " + strings.Join(tokens[3:], " ")
		}
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

	// If no builtin match, try forked subsystems
	if matchedCmd == nil {
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
		return &Response{Status: statusDone}, nil
	}

	return matchedCmd.Handler(ctx, args)
}

// dispatchSubsystem routes a command to a forked subsystem process.
func (d *Dispatcher) dispatchSubsystem(_ *CommandContext, handler *SubsystemHandler, input string) (*Response, error) {
	// TODO: Pass context from CommandContext when reactor provides it
	return handler.Handle(context.Background(), input)
}

// dispatchPlugin routes a command to a plugin process.
func (d *Dispatcher) dispatchPlugin(_ *CommandContext, input, peerSelector string) (*Response, error) {
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

// routeToProcess sends a command request to a plugin process.
func (d *Dispatcher) routeToProcess(cmd *RegisteredCommand, args []string, peerSelector string) (*Response, error) {
	proc := cmd.Process
	if proc == nil || !proc.Running() {
		return nil, errors.New("plugin process not running")
	}

	// Create response channel
	respCh := make(chan *Response, 1)

	// Add pending request
	serial := d.pending.Add(&PendingRequest{
		Command:  cmd.Name,
		Process:  proc,
		Timeout:  cmd.Timeout,
		RespChan: respCh,
	})

	if serial == "" {
		// Limit exceeded - error already sent to respCh
		select {
		case resp := <-respCh:
			return resp, nil
		default:
			return &Response{Status: statusError, Data: "too many pending requests"}, nil
		}
	}

	// Build request JSON
	request := map[string]any{
		"serial":  serial,
		"type":    "request",
		"command": cmd.Name,
		"args":    args,
		"peer":    peerSelector,
	}
	reqJSON, _ := json.Marshal(request)

	// Send to process
	if writeErr := proc.WriteEvent(string(reqJSON)); writeErr != nil {
		d.pending.Complete(serial, &Response{Status: statusError, Data: "failed to send request"})
	}

	// Collect responses (may be streaming with multiple partials)
	var partials []any
	for {
		resp := <-respCh
		if resp == nil {
			// Channel closed unexpectedly
			if len(partials) > 0 {
				return &Response{Status: statusDone, Data: partials}, nil
			}
			return &Response{Status: statusError, Data: "no response received"}, nil
		}

		if !resp.Partial {
			// Final response - combine with any partials
			if len(partials) > 0 {
				// Had streaming data, append final data if present
				if resp.Data != nil {
					partials = append(partials, resp.Data)
				}
				return &Response{Status: resp.Status, Data: partials}, nil
			}
			return resp, nil
		}

		// Partial response - collect and continue
		if resp.Data != nil {
			partials = append(partials, resp.Data)
		}
	}
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
