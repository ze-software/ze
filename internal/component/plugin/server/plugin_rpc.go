// Design: docs/architecture/api/process-protocol.md — plugin process management
// Overview: register.go — RPC registration hub

package server

import (
	"errors"
	"fmt"

	plugin "github.com/ze-software/ze/internal/component/plugin"
)

var (
	errMissingCommandName  = errors.New("missing command name")
	errMissingPartialInput = errors.New("missing partial input")
)

func init() {
	RegisterRPCs(
		RPCRegistration{WireMethod: "ze-plugin:help", Handler: handlePluginHelp},
		RPCRegistration{WireMethod: "ze-plugin:command-list", Handler: handlePluginCommandList},
		RPCRegistration{WireMethod: "ze-plugin:command-help", Handler: handlePluginCommandHelp},
		RPCRegistration{WireMethod: "ze-plugin:command-complete", Handler: handlePluginCommandComplete},
	)
}

// handlePluginHelp returns list of plugin subcommands.
func handlePluginHelp(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"subcommands": []string{"session", "command"},
		},
	}, nil
}

// handlePluginCommandList returns plugin-registered commands (not builtins).
func handlePluginCommandList(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	var commands []map[string]any

	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Registry().All() {
			commands = append(commands, map[string]any{
				"name":        cmd.Name,
				"description": cmd.Description,
			})
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"commands": commands,
		},
	}, nil
}

// handlePluginCommandHelp returns details for a plugin-registered command.
func handlePluginCommandHelp(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: plugin command help \"<name>\"",
		}, errMissingCommandName
	}

	name := args[0]

	if ctx.Dispatcher() != nil {
		if cmd := ctx.Dispatcher().Registry().Lookup(name); cmd != nil {
			return &plugin.Response{
				Status: plugin.StatusDone,
				Data: plugin.Map{
					"command":     cmd.Name,
					"description": cmd.Description,
					"args":        cmd.Args,
					"source":      cmd.Process.Name(),
				},
			}, nil
		}
	}

	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "unknown plugin command: " + name,
	}, fmt.Errorf("unknown plugin command: %s", name)
}

// handlePluginCommandComplete returns completions for plugin commands.
func handlePluginCommandComplete(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: plugin command complete \"<partial>\"",
		}, errMissingPartialInput
	}

	partial := args[0]
	var completions []Completion

	if ctx.Dispatcher() != nil {
		completions = ctx.Dispatcher().Registry().Complete(partial)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"completions": completions,
		},
	}, nil
}
