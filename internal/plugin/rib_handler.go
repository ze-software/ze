package plugin

import (
	"fmt"
	"strings"
)

// ribRPCs returns all RPCs for the ze-rib module.
func ribRPCs() []RPCRegistration {
	return []RPCRegistration{
		{"ze-rib:help", "rib help", handleRibHelp, "Show RIB subcommands"},
		{"ze-rib:command-list", "rib command list", handleRibCommandList, "List RIB commands"},
		{"ze-rib:command-help", "rib command help", handleRibCommandHelp, "Show RIB command details"},
		{"ze-rib:command-complete", "rib command complete", handleRibCommandComplete, "Complete RIB command/args"},
		{"ze-rib:event-list", "rib event list", handleRibEventList, "List RIB event types"},
		{"ze-rib:show-in", "rib show in", handleRIBShowIn, "Show Adj-RIB-In"},
		{"ze-rib:clear-in", "rib clear in", handleRIBClearIn, "Clear Adj-RIB-In"},
		{"ze-rib:show-out", "rib show out", handleRIBShowOut, "Show Adj-RIB-Out"},
		{"ze-rib:clear-out", "rib clear out", handleRIBClearOut, "Clear Adj-RIB-Out"},
	}
}

// handleRibHelp returns list of RIB subcommands.
func handleRibHelp(ctx *CommandContext, _ []string) (*Response, error) {
	subcommands := []string{
		"clear",
		"command",
		"event",
		"show",
	}

	// Add plugin-provided subcommands (e.g., "adjacent" from RIB plugin)
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

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"subcommands": subcommands,
		},
	}, nil
}

// handleRibCommandList returns all RIB commands (builtin + plugin).
func handleRibCommandList(ctx *CommandContext, args []string) (*Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []Completion

	// Add builtin rib commands
	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "rib ") {
				c := Completion{
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
				c := Completion{
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

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleRibCommandHelp returns detailed help for a RIB command.
func handleRibCommandHelp(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: rib command help \"<name>\"",
		}, fmt.Errorf("missing command name")
	}

	name := args[0]
	// Ensure it's a rib command
	if !strings.HasPrefix(name, "rib ") {
		name = "rib " + name
	}

	return lookupCommandHelp(ctx, name, "rib command")
}

// handleRibCommandComplete returns completions for RIB commands.
func handleRibCommandComplete(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: rib command complete \"<partial>\"",
		}, fmt.Errorf("missing partial input")
	}

	partial := args[0]
	// Ensure we complete within rib namespace
	if !strings.HasPrefix(partial, "rib ") {
		partial = "rib " + partial
	}

	var completions []Completion

	if ctx.Dispatcher() != nil {
		// Complete builtin rib commands
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(cmd.Name, "rib ") &&
				strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, Completion{
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

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}

// handleRibEventList returns available RIB event types.
func handleRibEventList(_ *CommandContext, _ []string) (*Response, error) {
	// RIB event types per ipc_protocol.md
	events := []string{
		"cache",  // msg-id cache operations (new, expire, evict)
		"route",  // route state changes
		"peer",   // peer RIB state changes
		"memory", // memory pressure events
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"events": events,
		},
	}, nil
}

// handleRIBShowIn returns Adj-RIB-In contents.
func handleRIBShowIn(ctx *CommandContext, args []string) (*Response, error) {
	_, errResp, err := requireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	// Optional peer filter
	peerID := ""
	if len(args) > 0 {
		peerID = args[0]
	}

	routes := ctx.Reactor().RIBInRoutes(peerID)
	stats := ctx.Reactor().RIBStats()

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"routes":      routes,
			"route_count": len(routes),
			"peer_count":  stats.InPeerCount,
		},
	}, nil
}

// handleRIBClearIn clears all routes from Adj-RIB-In.
func handleRIBClearIn(ctx *CommandContext, _ []string) (*Response, error) {
	_, errResp, err := requireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	count := ctx.Reactor().ClearRIBIn()

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"routes_cleared": count,
		},
	}, nil
}

// handleRIBShowOut returns Adj-RIB-Out contents.
// Stub: Adj-RIB-Out is maintained by RIB plugins, not the engine.
func handleRIBShowOut(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: statusError,
		Data:   "not yet implemented: rib show out requires a RIB plugin",
	}, fmt.Errorf("not yet implemented")
}

// handleRIBClearOut clears all routes from Adj-RIB-Out.
// Stub: Adj-RIB-Out is maintained by RIB plugins, not the engine.
func handleRIBClearOut(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: statusError,
		Data:   "not yet implemented: rib clear out requires a RIB plugin",
	}, fmt.Errorf("not yet implemented")
}
