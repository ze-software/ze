package api

import (
	"errors"
	"sort"
	"strings"
)

// ErrUnknownCommand is returned when a command is not recognized.
var ErrUnknownCommand = errors.New("unknown command")

// ErrEmptyCommand is returned when the command is empty.
var ErrEmptyCommand = errors.New("empty command")

// Handler processes a command and returns a response.
type Handler func(ctx *CommandContext, args []string) (*Response, error)

// CommandContext provides access to reactor and session state.
type CommandContext struct {
	Reactor ReactorInterface
	Encoder *JSONEncoder
	Peer    string // Peer selector: "*" for all, or specific IP. Empty = "*"
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
	commands map[string]*Command
	// sorted keys for longest-match lookup (longest first)
	sortedKeys []string
}

// NewDispatcher creates a new command dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		commands: make(map[string]*Command),
	}
}

// Register adds a command handler.
func (d *Dispatcher) Register(name string, handler Handler, help string) {
	// Store with lowercase key for case-insensitive matching
	key := strings.ToLower(name)
	d.commands[key] = &Command{
		Name:    name,
		Handler: handler,
		Help:    help,
	}
	d.updateSortedKeys()
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
// Supports neighbor prefix: "neighbor <addr> <command>" or "neighbor * <command>".
// If no neighbor prefix, defaults to all peers ("*").
func (d *Dispatcher) Dispatch(ctx *CommandContext, input string) (*Response, error) {
	tokens := tokenize(input)
	if len(tokens) == 0 {
		return nil, ErrEmptyCommand
	}

	// Check for neighbor/peer prefix (peer is alias for neighbor)
	// Only applies when second token looks like an IP address or glob pattern
	prefix := strings.ToLower(tokens[0])
	if (prefix == "neighbor" || prefix == "peer") && len(tokens) >= 3 {
		// Check if second token looks like IP/glob (contains dots or is "*")
		if looksLikeIPOrGlob(tokens[1]) {
			if ctx != nil {
				ctx.Peer = tokens[1]
			}
			// Rebuild input without neighbor/peer prefix
			input = strings.Join(tokens[2:], " ")
		}
	}

	// Build lowercase input for matching
	lowerInput := strings.ToLower(input)
	lowerInput = strings.TrimSpace(lowerInput)

	// Find longest matching command prefix
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

	if matchedCmd == nil {
		return nil, ErrUnknownCommand
	}

	// Extract remaining args
	remaining := strings.TrimSpace(input[matchedLen:])
	var args []string
	if remaining != "" {
		args = tokenize(remaining)
	}

	// Execute handler
	if matchedCmd.Handler == nil {
		return &Response{Status: "done"}, nil
	}

	return matchedCmd.Handler(ctx, args)
}

// tokenize splits a command string into tokens.
func tokenize(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	return strings.Fields(input)
}

// looksLikeIPOrGlob returns true if s looks like an IP address or glob pattern.
// Examples: "*", "192.168.1.1", "192.168.*.*", "2001:db8::1".
func looksLikeIPOrGlob(s string) bool {
	// Wildcard all
	if s == "*" {
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
