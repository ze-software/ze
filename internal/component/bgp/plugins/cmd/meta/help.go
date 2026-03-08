// Design: docs/architecture/api/commands.md — BGP command discovery handlers
// Overview: doc.go — bgp-cmd-meta plugin overview

package bgpcmdmeta

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:help", CLICommand: "bgp help", Handler: handleBgpHelp, Help: "List bgp subcommands", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:command-list", CLICommand: "bgp command list", Handler: handleBgpCommandList, Help: "List bgp commands", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:command-help", CLICommand: "bgp command help", Handler: handleBgpCommandHelp, Help: "Show command details", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:command-complete", CLICommand: "bgp command complete", Handler: handleBgpCommandComplete, Help: "Complete command/args", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:event-list", CLICommand: "bgp event list", Handler: handleBgpEventList, Help: "List available BGP event types", ReadOnly: true},
	)
}

// BGP event types.
var bgpEventTypes = []string{
	"update", "open", "notification", "keepalive",
	"refresh", "state", "negotiated",
}

// handleBgpHelp returns list of bgp subcommands.
func handleBgpHelp(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	var commands []string

	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				commands = append(commands, cmd.Name+" - "+cmd.Help)
			}
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleBgpCommandList returns commands in bgp namespace.
func handleBgpCommandList(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []pluginserver.Completion

	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				c := pluginserver.Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				}
				if verbose {
					c.Source = sourceBuiltin
				}
				commands = append(commands, c)
			}
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleBgpCommandHelp returns detailed help for a bgp command.
func handleBgpCommandHelp(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: bgp command help \"<name>\"")
	}

	name := args[0]

	if ctx.Dispatcher() != nil {
		if cmd := ctx.Dispatcher().Lookup(name); cmd != nil {
			if strings.HasPrefix(cmd.Name, "bgp ") {
				return &plugin.Response{
					Status: plugin.StatusDone,
					Data: map[string]any{
						"command":     cmd.Name,
						"description": cmd.Help,
						"source":      sourceBuiltin,
					},
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("unknown bgp command: %s", name)
}

// handleBgpCommandComplete returns completions for bgp commands.
func handleBgpCommandComplete(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: bgp command complete \"<partial>\"")
	}

	partial := args[0]
	var completions []pluginserver.Completion

	if ctx.Dispatcher() != nil {
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "bgp ") &&
				strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, pluginserver.Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				})
			}
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}

// handleBgpEventList returns available BGP event types.
func handleBgpEventList(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"events": bgpEventTypes,
		},
	}, nil
}
