package plugin

import (
	"fmt"
)

// pluginRPCs returns RPC registrations for handlers defined in this file.
// Part of the ze-plugin module — aggregated by PluginLifecycleRPCs().
func pluginRPCs() []RPCRegistration {
	return []RPCRegistration{
		{"ze-plugin:help", "plugin help", handlePluginHelp, "List plugin subcommands"},
		{"ze-plugin:command-list", "plugin command list", handlePluginCommandList, "List plugin commands"},
		{"ze-plugin:command-help", "plugin command help", handlePluginCommandHelp, "Show command details"},
		{"ze-plugin:command-complete", "plugin command complete", handlePluginCommandComplete, "Complete command/args"},
	}
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
