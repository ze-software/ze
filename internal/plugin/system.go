package plugin

import (
	"context"
	"fmt"
	"strings"
)

// systemRPCs returns all RPCs for the ze-system module.
func systemRPCs() []RPCRegistration {
	return []RPCRegistration{
		{"ze-system:help", "system help", handleSystemHelp, "Show available commands"},
		{"ze-system:version-software", "system version software", handleSystemVersionSoftware, "Show ze version"},
		{"ze-system:version-api", "system version api", handleSystemVersionAPI, "Show IPC protocol version"},
		{"ze-system:shutdown", "system shutdown", handleSystemShutdown, "Graceful application shutdown"},
		{"ze-system:subsystem-list", "system subsystem list", handleSystemSubsystemList, "List available subsystems"},
		{"ze-system:command-list", "system command list", handleSystemCommandList, "List all commands"},
		{"ze-system:command-help", "system command help", handleSystemCommandHelp, "Show command details"},
		{"ze-system:command-complete", "system command complete", handleSystemCommandComplete, "Complete command/args"},
	}
}

// handleSystemHelp returns list of available commands.
func handleSystemHelp(ctx *CommandContext, _ []string) (*Response, error) {
	var commands []string

	// Use dispatcher if available
	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Commands() {
			commands = append(commands, cmd.Name+" - "+cmd.Help)
		}
		// Add plugin commands
		for _, cmd := range ctx.Dispatcher.Registry().All() {
			line := cmd.Name
			if cmd.Args != "" {
				line += " " + cmd.Args
			}
			line += " - " + cmd.Description
			commands = append(commands, line)
		}
	}

	// Fallback if no dispatcher
	if len(commands) == 0 {
		commands = []string{
			"bgp daemon shutdown - Gracefully shutdown the daemon",
			"bgp daemon status - Show daemon status",
			"bgp peer <selector> list - List peer(s) (brief)",
			"bgp peer <selector> show - Show peer(s) details",
			"system help - Show available commands",
			"system version software - Show ze version",
			"system version api - Show IPC protocol version",
		}
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleSystemVersionSoftware returns ze version information.
func handleSystemVersionSoftware(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"version": Version,
		},
	}, nil
}

// handleSystemVersionAPI returns IPC protocol version.
func handleSystemVersionAPI(_ *CommandContext, _ []string) (*Response, error) {
	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"version": APIVersion,
		},
	}, nil
}

// handleSystemShutdown triggers graceful application shutdown.
func handleSystemShutdown(ctx *CommandContext, _ []string) (*Response, error) {
	ctx.Reactor.Stop()
	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"message": "shutdown initiated",
		},
	}, nil
}

// handleSystemSubsystemList returns available subsystems.
func handleSystemSubsystemList(_ *CommandContext, _ []string) (*Response, error) {
	// For now, bgp is always available
	// Future: query reactor for enabled subsystems
	subsystems := []string{"bgp"}
	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"subsystems": subsystems,
		},
	}, nil
}

// handleSystemCommandList returns all commands (builtin + plugin).
func handleSystemCommandList(ctx *CommandContext, args []string) (*Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []Completion

	// Add builtin commands
	if ctx.Dispatcher != nil {
		for _, cmd := range ctx.Dispatcher.Commands() {
			c := Completion{
				Value: cmd.Name,
				Help:  cmd.Help,
			}
			if verbose {
				c.Source = sourceBuiltin
			}
			commands = append(commands, c)
		}

		// Add plugin commands
		for _, cmd := range ctx.Dispatcher.Registry().All() {
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

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleSystemCommandHelp returns detailed help for a specific command.
func handleSystemCommandHelp(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: system command help \"<name>\"",
		}, fmt.Errorf("missing command name")
	}

	name := args[0]

	// Check builtins first
	if ctx.Dispatcher != nil {
		if cmd := ctx.Dispatcher.Lookup(name); cmd != nil {
			return &Response{
				Status: statusDone,
				Data: map[string]any{
					"command":     cmd.Name,
					"description": cmd.Help,
					"source":      sourceBuiltin,
				},
			}, nil
		}

		// Check plugin commands
		if cmd := ctx.Dispatcher.Registry().Lookup(name); cmd != nil {
			return &Response{
				Status: statusDone,
				Data: map[string]any{
					"command":     cmd.Name,
					"description": cmd.Description,
					"args":        cmd.Args,
					"source":      cmd.Process.Name(),
					"timeout":     cmd.Timeout.String(),
				},
			}, nil
		}
	}

	return &Response{
		Status: statusError,
		Data:   fmt.Sprintf("unknown command: %s", name),
	}, fmt.Errorf("unknown command: %s", name)
}

// handleSystemCommandComplete returns completions for partial input.
// Usage:
//
//	system command complete "<partial>"           - command completion
//	system command complete "<cmd>" args "<partial>" - arg completion
func handleSystemCommandComplete(ctx *CommandContext, args []string) (*Response, error) {
	if len(args) < 1 {
		return &Response{
			Status: statusError,
			Data:   "usage: system command complete \"<partial>\"",
		}, fmt.Errorf("missing partial input")
	}

	partial := args[0]

	// Check for "args" subcommand for argument completion
	// Format: system command complete "<cmd>" args [<completed>...] "<partial>"
	if len(args) >= 3 && args[1] == "args" {
		cmdName := args[0]
		// Last arg is the partial, everything between "args" and last is completed args
		partialArg := args[len(args)-1]
		var completedArgs []string
		if len(args) > 3 {
			completedArgs = args[2 : len(args)-1]
		}
		return handleArgComplete(ctx, cmdName, completedArgs, partialArg)
	}

	var completions []Completion

	if ctx.Dispatcher != nil {
		// Complete builtins
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher.Commands() {
			if strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				})
			}
		}

		// Complete plugin commands
		completions = append(completions, ctx.Dispatcher.Registry().Complete(partial)...)
	}

	return &Response{
		Status: statusDone,
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}

// handleArgComplete handles argument completion for a specific command.
func handleArgComplete(ctx *CommandContext, cmdName string, completedArgs []string, partial string) (*Response, error) {
	emptyResult := &Response{
		Status: statusDone,
		Data:   map[string]any{"completions": []Completion{}},
	}

	if ctx.Dispatcher == nil {
		return emptyResult, nil
	}

	// Check if it's a plugin command with completable flag
	cmd := ctx.Dispatcher.Registry().Lookup(cmdName)
	if cmd == nil || !cmd.Completable {
		return emptyResult, nil
	}

	// Route completion request to process
	proc := cmd.Process
	if proc == nil || !proc.Running() {
		return emptyResult, nil
	}

	// Create response channel
	respCh := make(chan *Response, 1)

	// Add pending request with completion timeout
	serial := ctx.Dispatcher.Pending().Add(&PendingRequest{
		Command:  cmd.Name,
		Process:  proc,
		Timeout:  CompletionTimeout,
		RespChan: respCh,
	})

	if serial == "" {
		return emptyResult, nil
	}

	// Send completion request via RPC
	connB := proc.ConnB()
	if connB == nil {
		ctx.Dispatcher.Pending().Complete(serial, emptyResult)
		return <-respCh, nil
	}
	rpcCtx, cancel := context.WithTimeout(context.Background(), CompletionTimeout)
	defer cancel()
	rpcOut, rpcErr := connB.SendExecuteCommand(rpcCtx, serial, cmd.Name, completedArgs, partial)
	if rpcErr != nil {
		ctx.Dispatcher.Pending().Complete(serial, emptyResult)
	} else if rpcOut != nil {
		ctx.Dispatcher.Pending().Complete(serial, &Response{Status: rpcOut.Status, Data: rpcOut.Data})
	}

	// Wait for response
	resp := <-respCh
	return resp, nil
}
