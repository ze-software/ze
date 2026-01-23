package plugin

import (
	"fmt"
	"strings"
)

// BGP event types.
var bgpEventTypes = []string{
	"update", "open", "notification", "keepalive",
	"refresh", "state", "negotiated",
}

func init() {
	// BGP introspection
	RegisterBuiltin("bgp help", handleBgpHelp, "List bgp subcommands")
	RegisterBuiltin("bgp command list", handleBgpCommandList, "List bgp commands")
	RegisterBuiltin("bgp command help", handleBgpCommandHelp, "Show command details")
	RegisterBuiltin("bgp command complete", handleBgpCommandComplete, "Complete command/args")
	RegisterBuiltin("bgp event list", handleBgpEventList, "List available BGP event types")

	// BGP plugin configuration
	RegisterBuiltin("bgp plugin encoding", handleBgpPluginEncoding, "Set event encoding (json|text)")
	RegisterBuiltin("bgp plugin format", handleBgpPluginFormat, "Set wire format (hex|base64|parsed|full)")
	RegisterBuiltin("bgp plugin ack", handleBgpPluginAck, "Set ACK timing (sync|async)")
}

// handleBgpHelp returns list of bgp subcommands.
func handleBgpHelp(ctx *CommandContext, _ []string) (*Response, error) {
	var commands []string

	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				commands = append(commands, cmd.Name+" - "+cmd.Help)
			}
		}
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleBgpCommandList returns commands in bgp namespace.
func handleBgpCommandList(ctx *CommandContext, args []string) (*Response, error) {
	verbose := len(args) > 0 && args[0] == "verbose"

	var commands []Completion

	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				c := Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				}
				if verbose {
					c.Source = "builtin"
				}
				commands = append(commands, c)
			}
		}
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleBgpCommandHelp returns detailed help for a bgp command.
func handleBgpCommandHelp(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: bgp command help \"<name>\"")
	}

	name := args[0]

	if ctx.Dispatcher != nil {
		if cmd := ctx.Dispatcher.Lookup(name); cmd != nil {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				return &Response{
					Status: statusDone,
					Data: map[string]any{
						"command":     cmd.Name,
						"description": cmd.Help,
						"source":      "builtin",
					},
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("unknown bgp command: %s", name)
}

// handleBgpCommandComplete returns completions for bgp commands.
func handleBgpCommandComplete(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: bgp command complete \"<partial>\"")
	}

	partial := args[0]
	var completions []Completion

	if ctx.Dispatcher != nil {
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher.Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") &&
				strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				})
			}
		}
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}

// handleBgpEventList returns available BGP event types.
func handleBgpEventList(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"events": bgpEventTypes,
		},
	}, nil
}

// handleBgpPluginEncoding sets event encoding for this process.
// Syntax: bgp plugin encoding <json|text>.
func handleBgpPluginEncoding(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing encoding: bgp plugin encoding <json|text>")
	}

	enc := strings.ToLower(args[0])
	switch enc {
	case EncodingJSON, EncodingText:
		if ctx.Process != nil {
			ctx.Process.SetEncoding(enc)
		}
	default:
		return nil, fmt.Errorf("invalid encoding: %s (valid: json, text)", args[0])
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"encoding": enc,
		},
	}, nil
}

// handleBgpPluginFormat sets wire format for this process.
// Syntax: bgp plugin format <hex|base64|parsed|full>.
func handleBgpPluginFormat(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing format: bgp plugin format <hex|base64|parsed|full>")
	}

	format := strings.ToLower(args[0])
	switch format {
	case FormatHex, FormatBase64, FormatParsed, FormatFull:
		if ctx.Process != nil {
			ctx.Process.SetFormat(format)
		}
	default:
		return nil, fmt.Errorf("invalid format: %s (valid: hex, base64, parsed, full)", args[0])
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"format": format,
		},
	}, nil
}

// handleBgpPluginAck sets ACK timing for this process.
// Syntax: bgp plugin ack <sync|async>.
func handleBgpPluginAck(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing mode: bgp plugin ack <sync|async>")
	}

	mode := strings.ToLower(args[0])
	switch mode {
	case "sync":
		if ctx.Process != nil {
			ctx.Process.SetSync(true)
		}
	case "async":
		if ctx.Process != nil {
			ctx.Process.SetSync(false)
		}
	default:
		return nil, fmt.Errorf("invalid mode: %s (valid: sync, async)", args[0])
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"ack": mode,
		},
	}, nil
}
