// Design: docs/architecture/api/commands.md — command discovery handlers

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	sourceBuiltin = "builtin"
	argVerbose    = "verbose"
	// keyDescription is the response payload key carrying a one-line summary.
	keyDescription = "description"
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

func bgpEventTypes() []string {
	names := events.ValidEventNames(bgpevents.Namespace)
	if names == "" {
		return nil
	}
	return strings.Split(names, ", ")
}

func handleBgpHelp(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	var commands []string

	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			commands = append(commands, cmd.Name+" - "+cmd.Help)
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"commands": commands,
		},
	}, nil
}

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
		Data: plugin.Map{
			"commands": commands,
		},
	}, nil
}

func handleBgpCommandHelp(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, errors.New("usage: command help \"<name>\"")
	}

	name := textbuf.Join(args, " ")

	dispatcher := ctx.Dispatcher()
	if dispatcher == nil {
		return nil, fmt.Errorf("unknown command: %s", name)
	}

	if cmd := dispatcher.Lookup(name); cmd != nil {
		return commandHelp(cmd.Name, cmd.Help, sourceBuiltin, ""), nil
	}

	// A plugin's command sits in the command registry rather than in the
	// dispatcher's builtin table, and it is the command a plugin's pipe alias
	// is declared on. Reading the builtins alone answers "unknown command" for
	// every command any plugin declares.
	if cmd := dispatcher.Registry().Lookup(name); cmd != nil {
		return commandHelp(cmd.Name, cmd.Description, cmd.Process.Name(), cmd.Args), nil
	}

	return nil, fmt.Errorf("unknown command: %s", name)
}

// commandHelp answers for one command: what it is, and the pipe names it
// answers to beside the built-in operators.
//
// A running daemon is the only place both lists exist. A pipe filter and a pipe
// alias are each registered at startup, an alias by an in-tree package or by a
// plugin's Stage 1 message, so a tool reading the compiled command tree in its
// own process can report neither.
func commandHelp(name, description, source, args string) *plugin.Response {
	data := map[string]any{
		"command":      name,
		keyDescription: description,
		"source":       source,
	}
	if args != "" {
		data["args"] = args
	}
	if filters := pipeFilterHelp(command.PipeFiltersForCommand(name)); len(filters) > 0 {
		data["pipe-filters"] = filters
	}
	if aliases := pipeAliasHelp(command.AliasesForCommand(name)); len(aliases) > 0 {
		data["pipe-aliases"] = aliases
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map(data),
	}
}

// pipeAliasHelp renders the pipe aliases a command answers to.
//
// The expansion is reported beside the description because an alias takes no
// argument and names no other alias, so the chain it stands for is fixed at
// registration and is the whole of what the name does.
func pipeAliasHelp(aliases []command.Alias) []map[string]any {
	if len(aliases) == 0 {
		return nil
	}
	items := make([]map[string]any, 0, len(aliases))
	for _, alias := range aliases {
		items = append(items, map[string]any{
			"name":         alias.Name,
			keyDescription: alias.Description,
			"expansion":    alias.Expansion,
		})
	}
	return items
}

func pipeFilterHelp(filters []command.PipeFilter) []map[string]any {
	if len(filters) == 0 {
		return nil
	}
	items := make([]map[string]any, 0, len(filters))
	for _, filter := range filters {
		items = append(items, map[string]any{
			"name":         filter.Name,
			keyDescription: filter.Description,
			"takes-arg":    filter.TakesArg,
		})
	}
	return items
}

func handleBgpCommandComplete(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return nil, errors.New("usage: command complete \"<partial>\"")
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
		Data: plugin.Map{
			"completions": completions,
		},
	}, nil
}

func handleBgpEventList(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"events": bgpEventTypes(),
		},
	}, nil
}

func handleEventMonitor(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	opts, err := pluginserver.ParseEventMonitorArgs(args)
	if err != nil {
		return nil, err
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"status":    "monitor-configured",
			"include":   opts.IncludeTypes,
			"exclude":   opts.ExcludeTypes,
			"peer":      opts.Peer,
			"direction": opts.Direction,
		},
	}, nil
}
