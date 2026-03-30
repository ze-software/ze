// Design: docs/architecture/api/commands.md — command discovery handlers
// Overview: doc.go — cmd-meta plugin overview

package meta

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:help", Handler: handleBgpHelp},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:command-list", Handler: handleBgpCommandList},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:command-help", Handler: handleBgpCommandHelp},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:command-complete", Handler: handleBgpCommandComplete},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:event-list", Handler: handleBgpEventList},
		pluginserver.RPCRegistration{WireMethod: "ze-event:monitor", Handler: handleEventMonitor},
	)
}

// bgpEventTypes returns the current list of valid BGP event types,
// excluding non-event entries like "sent" (a direction flag).
func bgpEventTypes() []string {
	names := plugin.ValidEventNames(plugin.NamespaceBGP)
	if names == "" {
		return nil
	}
	types := strings.Split(names, ", ")
	// Filter out "sent" which is a direction flag, not an event type.
	result := types[:0]
	for _, t := range types {
		if t != plugin.DirectionSent {
			result = append(result, t)
		}
	}
	return result
}

// handleBgpHelp returns list of all available commands.
func handleBgpHelp(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	var commands []string

	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			commands = append(commands, cmd.Name+" - "+cmd.Help)
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleBgpCommandList returns all registered commands.
func handleBgpCommandList(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []pluginserver.Completion

	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
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

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleBgpCommandHelp returns detailed help for a command.
func handleBgpCommandHelp(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: command help \"<name>\"")
	}

	name := args[0]

	if ctx.Dispatcher() != nil {
		if cmd := ctx.Dispatcher().Lookup(name); cmd != nil {
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

	return nil, fmt.Errorf("unknown command: %s", name)
}

// handleBgpCommandComplete returns completions for commands.
func handleBgpCommandComplete(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: command complete \"<partial>\"")
	}

	partial := args[0]
	var completions []pluginserver.Completion

	if ctx.Dispatcher() != nil {
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
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
			"events": bgpEventTypes(),
		},
	}, nil
}

// handleEventMonitor is the RPC handler for non-streaming callers.
// Streaming is handled by StreamEventMonitor via the SSH/TUI path.
// This handler validates arguments and returns the parsed configuration.
func handleEventMonitor(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	opts, err := pluginserver.ParseEventMonitorArgs(args)
	if err != nil {
		return nil, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"status":    "monitor-configured",
			"include":   opts.IncludeTypes,
			"exclude":   opts.ExcludeTypes,
			"peer":      opts.Peer,
			"direction": opts.Direction,
		},
	}, nil
}
