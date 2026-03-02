// Design: docs/architecture/api/commands.md — API command handlers
// Overview: register.go — handler aggregation
// Related: bgp.go — BGP introspection handlers (parallel pattern)

package handler

import (
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/plugin/server"
)

// RibMetaRPCs returns RPCs for the ze-rib module meta-commands.
// Data commands (show/clear in/out) are handled by the RIB plugin, not engine builtins.
// Only meta-commands that need Dispatcher access remain here.
func RibMetaRPCs() []pluginserver.RPCRegistration {
	return []pluginserver.RPCRegistration{
		{WireMethod: "ze-rib:help", CLICommand: "rib help", Handler: handleRibHelp, Help: "Show RIB subcommands"},
		{WireMethod: "ze-rib:command-list", CLICommand: "rib command list", Handler: handleRibCommandList, Help: "List RIB commands"},
		{WireMethod: "ze-rib:command-help", CLICommand: "rib command help", Handler: handleRibCommandHelp, Help: "Show RIB command details"},
		{WireMethod: "ze-rib:command-complete", CLICommand: "rib command complete", Handler: handleRibCommandComplete, Help: "Complete RIB command/args"},
		{WireMethod: "ze-rib:event-list", CLICommand: "rib event list", Handler: handleRibEventList, Help: "List RIB event types"},
	}
}

// handleRibHelp returns list of RIB subcommands.
func handleRibHelp(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	subcommands := []string{
		"command",
		"event",
	}

	// Add plugin-provided subcommands (e.g., "show", "clear", "adjacent" from RIB plugin)
	if ctx.Dispatcher() != nil {
		seen := make(map[string]bool)
		for _, sub := range subcommands {
			seen[sub] = true
		}
		for _, cmd := range ctx.Dispatcher().Registry().All() {
			if after, ok := strings.CutPrefix(cmd.Name, "rib "); ok {
				parts := strings.SplitN(after, " ", 2)
				if len(parts) > 0 && !seen[parts[0]] {
					subcommands = append(subcommands, parts[0])
					seen[parts[0]] = true
				}
			}
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"subcommands": subcommands,
		},
	}, nil
}

// handleRibCommandList returns all RIB commands (builtin + plugin).
func handleRibCommandList(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []pluginserver.Completion

	// Add builtin rib commands
	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "rib ") {
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

		// Add plugin-provided rib commands
		for _, cmd := range ctx.Dispatcher().Registry().All() {
			if strings.HasPrefix(cmd.Name, "rib ") {
				c := pluginserver.Completion{
					Value: cmd.Name,
					Help:  cmd.Description,
				}
				if verbose {
					c.Source = cmd.Process.Name()
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

// handleRibCommandHelp returns detailed help for a RIB command.
func handleRibCommandHelp(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: rib command help \"<name>\"",
		}, fmt.Errorf("missing command name")
	}

	name := args[0]
	// Ensure it's a rib command
	if !strings.HasPrefix(name, "rib ") {
		name = "rib " + name
	}

	return pluginserver.LookupCommandHelp(ctx, name, "rib command")
}

// handleRibCommandComplete returns completions for RIB commands.
func handleRibCommandComplete(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: rib command complete \"<partial>\"",
		}, fmt.Errorf("missing partial input")
	}

	partial := args[0]
	// Ensure we complete within rib namespace
	if !strings.HasPrefix(partial, "rib ") {
		partial = "rib " + partial
	}

	var completions []pluginserver.Completion

	if ctx.Dispatcher() != nil {
		// Complete builtin rib commands
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "rib ") &&
				strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, pluginserver.Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				})
			}
		}

		// Complete plugin rib commands
		for _, c := range ctx.Dispatcher().Registry().Complete(partial) {
			if strings.HasPrefix(c.Value, "rib ") {
				completions = append(completions, c)
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

// handleRibEventList returns available RIB event types.
func handleRibEventList(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	// RIB event types per ipc_protocol.md
	events := []string{
		"cache",  // msg-id cache operations (new, expire, evict)
		"route",  // route state changes
		"peer",   // peer RIB state changes
		"memory", // memory pressure events
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"events": events,
		},
	}, nil
}
