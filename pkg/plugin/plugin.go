// Package plugin provides a high-level SDK for creating ZeBGP plugins.
//
// A plugin extends ZeBGP's configuration schema by declaring YANG modules,
// handling verify/apply requests for configuration changes, and optionally
// providing additional commands.
//
// Basic usage:
//
//	p := plugin.New("my-plugin")
//	p.SetSchema(myYangSchema, "my-prefix")
//	p.OnVerify("my-prefix", myVerifyHandler)
//	p.OnApply("my-prefix", myApplyHandler)
//	p.OnCommand("status", myStatusCommand)
//	p.Run()
package plugin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Plugin name length limits.
const (
	MinNameLength = 1
	MaxNameLength = 64
)

// Plugin represents a ZeBGP plugin that follows the 5-stage protocol.
type Plugin struct {
	name     string
	schema   string
	handlers []string

	verifyHandlers  map[string]VerifyHandler
	applyHandlers   map[string]ApplyHandler
	commandHandlers map[string]CommandHandler

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
	Path   string // Full path including predicates
	Data   string // JSON data
}

// ApplyContext provides context for apply handlers.
type ApplyContext struct {
	Action string // create, modify, delete
	Path   string // Full path including predicates
}

// CommandContext provides context for command handlers.
type CommandContext struct {
	Command string   // Command name
	Args    []string // Command arguments
}

// New creates a new plugin with the given name.
// Returns nil if name is empty or exceeds 64 characters.
func New(name string) *Plugin {
	if len(name) < MinNameLength || len(name) > MaxNameLength {
		return nil
	}

	return &Plugin{
		name:            name,
		verifyHandlers:  make(map[string]VerifyHandler),
		applyHandlers:   make(map[string]ApplyHandler),
		commandHandlers: make(map[string]CommandHandler),
		input:           bufio.NewScanner(os.Stdin),
		output:          os.Stdout,
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return p.name
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

	// Stage 2: Config (wait for config done, handle verify during config)
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

	// Declare schema
	if p.schema != "" {
		// Escape newlines in schema for single-line transmission
		escaped := strings.ReplaceAll(p.schema, "\n", "\\n")
		_, _ = fmt.Fprintf(p.output, "declare schema %s\n", escaped)
	}

	// Declare handlers
	for _, h := range p.handlers {
		_, _ = fmt.Fprintf(p.output, "declare handler %s\n", h)
	}

	// Declare commands
	for name := range p.commandHandlers {
		_, _ = fmt.Fprintf(p.output, "declare cmd %s\n", name)
	}

	_, _ = fmt.Fprintln(p.output, "declare done")
}

// runConfigPhase handles the config phase, processing verify requests.
func (p *Plugin) runConfigPhase() error {
	for p.input.Scan() {
		line := p.input.Text()

		if line == "config done" {
			return nil
		}

		// Handle verify requests during config phase
		if strings.HasPrefix(line, "config verify") {
			if err := p.handleVerifyLine(line); err != nil {
				// Verification failure - config rejected
				return fmt.Errorf("verify failed: %w", err)
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

		// Check for shutdown
		if strings.Contains(line, `"shutdown"`) {
			return nil
		}

		// Parse #serial command
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
	// Check registered commands
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

// handleVerifyLine parses and handles a verify request line.
func (p *Plugin) handleVerifyLine(line string) error {
	// Parse: config verify action <action> path "<path>" data '<json>'
	ctx, err := parseVerifyLine(line)
	if err != nil {
		return err
	}
	return p.triggerVerify(ctx)
}

// parseVerifyLine parses a config verify line.
func parseVerifyLine(line string) (*VerifyContext, error) {
	// Remove "config verify " prefix
	rest := strings.TrimPrefix(line, "config verify ")

	ctx := &VerifyContext{}

	// Parse action
	if !strings.HasPrefix(rest, "action ") {
		return nil, fmt.Errorf("expected 'action'")
	}
	rest = strings.TrimPrefix(rest, "action ")
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("missing action value")
	}
	ctx.Action = parts[0]
	rest = parts[1]

	// Parse path
	if !strings.HasPrefix(rest, "path ") {
		return nil, fmt.Errorf("expected 'path'")
	}
	rest = strings.TrimPrefix(rest, "path ")
	path, rest, err := parseQuoted(rest)
	if err != nil {
		return nil, fmt.Errorf("parse path: %w", err)
	}
	ctx.Path = path

	// Parse data
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "data ") {
		return nil, fmt.Errorf("expected 'data'")
	}
	rest = strings.TrimPrefix(rest, "data ")
	data, _, err := parseSingleQuoted(rest)
	if err != nil {
		return nil, fmt.Errorf("parse data: %w", err)
	}
	ctx.Data = data

	return ctx, nil
}

// parseQuoted parses a double-quoted string.
func parseQuoted(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '"' {
		return "", "", fmt.Errorf("expected double quote")
	}

	var result strings.Builder
	i := 1
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			result.WriteByte(s[i])
			i++
			continue
		}
		if s[i] == '"' {
			return result.String(), s[i+1:], nil
		}
		result.WriteByte(s[i])
		i++
	}
	return "", "", fmt.Errorf("unclosed quote")
}

// parseSingleQuoted parses a single-quoted string (for JSON data).
func parseSingleQuoted(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '\'' {
		return "", "", fmt.Errorf("expected single quote")
	}

	var result strings.Builder
	i := 1
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			result.WriteByte(s[i])
			i++
			continue
		}
		if s[i] == '\'' {
			return result.String(), s[i+1:], nil
		}
		result.WriteByte(s[i])
		i++
	}
	return "", "", fmt.Errorf("unclosed quote")
}

// triggerVerify calls the appropriate verify handler.
func (p *Plugin) triggerVerify(ctx *VerifyContext) error {
	// Find handler by longest prefix match
	handler := p.findVerifyHandler(ctx.Path)
	if handler == nil {
		return fmt.Errorf("no handler for path: %s", ctx.Path)
	}
	return handler(ctx)
}

// triggerApply calls the appropriate apply handler.
func (p *Plugin) triggerApply(ctx *ApplyContext) error {
	handler := p.findApplyHandler(ctx.Path)
	if handler == nil {
		return fmt.Errorf("no handler for path: %s", ctx.Path)
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
	// Strip predicates for matching
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
