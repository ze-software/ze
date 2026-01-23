package plugin

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseRegisterCommand parses a register command from tokens after "register".
func parseRegisterCommand(tokens []string) (*CommandDef, error) {
	if len(tokens) < 4 {
		return nil, fmt.Errorf("register command requires: command \"<name>\" description \"<help>\"")
	}

	// Expect: command "<name>" description "<help>"
	if strings.ToLower(tokens[0]) != "command" {
		return nil, fmt.Errorf("expected 'command', got %q", tokens[0])
	}

	name := tokens[1]
	if name == "" {
		return nil, fmt.Errorf("command name cannot be empty")
	}

	// Validate command name: must be lowercase, no quotes
	if name != strings.ToLower(name) {
		return nil, fmt.Errorf("command name must be lowercase: %q", name)
	}
	if strings.ContainsAny(name, `"'`) {
		return nil, fmt.Errorf("command name cannot contain quotes: %q", name)
	}

	if strings.ToLower(tokens[2]) != "description" {
		return nil, fmt.Errorf("expected 'description', got %q", tokens[2])
	}

	description := tokens[3]

	def := &CommandDef{
		Name:        name,
		Description: description,
		Timeout:     DefaultCommandTimeout,
	}

	// Parse optional args, completable, timeout
	i := 4
	for i < len(tokens) {
		switch strings.ToLower(tokens[i]) {
		case "args":
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("args requires a value")
			}
			def.Args = tokens[i+1]
			i += 2

		case "completable":
			def.Completable = true
			i++

		case "timeout":
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("timeout requires a value")
			}
			d, err := parseDuration(tokens[i+1])
			if err != nil {
				return nil, fmt.Errorf("invalid timeout: %w", err)
			}
			def.Timeout = d
			i += 2

		default:
			return nil, fmt.Errorf("unexpected token: %q", tokens[i])
		}
	}

	return def, nil
}

// parseUnregisterCommand parses an unregister command from tokens after "unregister".
func parseUnregisterCommand(tokens []string) (string, error) {
	if len(tokens) < 2 {
		return "", fmt.Errorf("unregister command requires: command \"<name>\"")
	}

	if strings.ToLower(tokens[0]) != "command" {
		return "", fmt.Errorf("expected 'command', got %q", tokens[0])
	}

	name := tokens[1]
	if name == "" {
		return "", fmt.Errorf("command name cannot be empty")
	}

	return name, nil
}

// parseDuration parses a duration string like "60s" or "500ms".
func parseDuration(s string) (time.Duration, error) {
	// Try standard Go duration format
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Try simple number + unit
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %q", s)
	}

	// Find where the number ends
	numEnd := 0
	for numEnd < len(s) && (s[numEnd] >= '0' && s[numEnd] <= '9') {
		numEnd++
	}
	if numEnd == 0 {
		return 0, fmt.Errorf("invalid duration: %q", s)
	}

	n, err := strconv.Atoi(s[:numEnd])
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %w", err)
	}

	unit := s[numEnd:]
	switch unit {
	case "s":
		return time.Duration(n) * time.Second, nil
	case "ms":
		return time.Duration(n) * time.Millisecond, nil
	case "m":
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %q", unit)
	}
}

// parsePluginResponse parses a plugin response line.
// Returns (serial, type, data, ok) where:
//   - serial: the alpha serial (e.g., "a", "bcd")
//   - type: "done", "error", or "partial"
//   - data: the remaining data after type keyword
//   - ok: true if this is a valid response line
//
// Formats:
//
//	@serial done [json-data]
//	@serial error "<message>"
//	@serial+ [json-data]  (partial/streaming)
func parsePluginResponse(line string) (serial, respType, data string, ok bool) {
	if len(line) < 2 || line[0] != '@' {
		return "", "", "", false
	}

	// Check for streaming marker (+)
	streaming := false
	serialEnd := 1
	for serialEnd < len(line) {
		if line[serialEnd] == ' ' {
			break
		}
		if line[serialEnd] == '+' {
			streaming = true
			break
		}
		serialEnd++
	}

	if serialEnd == 1 {
		return "", "", "", false // Just "@"
	}

	serial = line[1:serialEnd]

	// Find rest after serial (and optional +)
	restStart := serialEnd
	if streaming {
		restStart++ // Skip +
	}
	if restStart < len(line) && line[restStart] == ' ' {
		restStart++ // Skip space
	}

	rest := ""
	if restStart < len(line) {
		rest = line[restStart:]
	}

	if streaming {
		// @serial+ data
		return serial, "partial", rest, true
	}

	// Parse "done" or "error"
	if strings.HasPrefix(rest, "done") {
		data = strings.TrimPrefix(rest, "done")
		data = strings.TrimSpace(data)
		return serial, "done", data, true
	}

	if strings.HasPrefix(rest, "error") {
		data = strings.TrimPrefix(rest, "error")
		data = strings.TrimSpace(data)
		// Unquote if quoted
		if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
			data = data[1 : len(data)-1]
		}
		return serial, "error", data, true
	}

	return "", "", "", false
}

func init() {
	// Plugin lifecycle (moved from session namespace)
	RegisterBuiltin("plugin session ready", handlePluginSessionReady, "Signal plugin init complete")
	// Also register with bgp peer prefix for per-peer ready signals (e.g., after route replay)
	RegisterBuiltin("bgp peer plugin session ready", handlePluginSessionReady, "Signal peer-specific plugin init complete")
	RegisterBuiltin("plugin session ping", handlePluginSessionPing, "Health check (returns PID)")
	RegisterBuiltin("plugin session bye", handlePluginSessionBye, "Disconnect")

	// Plugin introspection
	RegisterBuiltin("plugin help", handlePluginHelp, "List plugin subcommands")
	RegisterBuiltin("plugin command list", handlePluginCommandList, "List plugin commands")
	RegisterBuiltin("plugin command help", handlePluginCommandHelp, "Show command details")
	RegisterBuiltin("plugin command complete", handlePluginCommandComplete, "Complete command/args")
}

// handlePluginHelp returns list of plugin subcommands.
func handlePluginHelp(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"subcommands": []string{"session", "command"},
		},
	}, nil
}

// handlePluginCommandList returns plugin-registered commands (not builtins).
func handlePluginCommandList(ctx *CommandContext, _ []string) (*Response, error) {
	var commands []map[string]any

	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Registry().All() {
			commands = append(commands, map[string]any{
				"name":        cmd.Name,
				"description": cmd.Description,
			})
		}
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handlePluginCommandHelp returns details for a plugin-registered command.
func handlePluginCommandHelp(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: plugin command help \"<name>\"",
		}, fmt.Errorf("missing command name")
	}

	name := args[0]

	if ctx.Dispatcher != nil {
		if cmd := ctx.Dispatcher.Registry().Lookup(name); cmd != nil {
			return &Response{
				Status: statusDone,
				Data: map[string]any{
					"command":     cmd.Name,
					"description": cmd.Description,
					"args":        cmd.Args,
					"source":      cmd.Process.Name(),
				},
			}, nil
		}
	}

	return &Response{
		Status: statusError,
		Data:   fmt.Sprintf("unknown plugin command: %s", name),
	}, fmt.Errorf("unknown plugin command: %s", name)
}

// handlePluginCommandComplete returns completions for plugin commands.
func handlePluginCommandComplete(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: plugin command complete \"<partial>\"",
		}, fmt.Errorf("missing partial input")
	}

	partial := args[0]
	var completions []Completion

	if ctx.Dispatcher != nil {
		completions = ctx.Dispatcher.Registry().Complete(partial)
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}
