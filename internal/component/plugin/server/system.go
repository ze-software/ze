// Design: docs/architecture/api/process-protocol.md — plugin process management
// Overview: register.go — RPC registration hub

package server

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

func init() {
	RegisterRPCs(
		RPCRegistration{WireMethod: "ze-system:help", Handler: handleSystemHelp, Help: "Show available commands", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-system:version-software", Handler: handleSystemVersionSoftware, Help: "Show ze version", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-system:version-api", Handler: handleSystemVersionAPI, Help: "Show IPC protocol version", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-system:daemon-shutdown", Handler: handleDaemonShutdown, Help: "Gracefully shutdown the daemon"},
		RPCRegistration{WireMethod: "ze-system:daemon-quit", Handler: handleDaemonQuit, Help: "Goroutine dump + shutdown"},
		RPCRegistration{WireMethod: "ze-system:daemon-status", Handler: handleDaemonStatus, Help: "Show daemon status", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-system:daemon-reload", Handler: handleDaemonReload, Help: "Reload the configuration"},
		RPCRegistration{WireMethod: "ze-system:subsystem-list", Handler: handleSystemSubsystemList, Help: "List available subsystems", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-system:command-list", Handler: handleSystemCommandList, Help: "List all commands", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-system:command-help", Handler: handleSystemCommandHelp, Help: "Show command details", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-system:command-complete", Handler: handleSystemCommandComplete, Help: "Complete command/args", ReadOnly: true},
		RPCRegistration{WireMethod: "ze-system:dispatch", Handler: handleSystemDispatch, Help: "Dispatch a text command"},
	)
}

// handleSystemDispatch dispatches a text command through the standard command dispatcher.
// This enables API socket clients to invoke any command reachable through the text
// dispatcher, including plugin-registered commands (e.g., "watchdog announce dnsr").
// Args are joined into a single command string for the dispatcher.
func handleSystemDispatch(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: system dispatch \"<command>\"",
		}, fmt.Errorf("missing command")
	}

	d := ctx.Dispatcher()
	if d == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "dispatcher not available",
		}, fmt.Errorf("dispatcher not available")
	}

	command := strings.Join(args, " ")
	return d.Dispatch(ctx, command)
}

// handleSystemHelp returns list of available commands.
func handleSystemHelp(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	var commands []string

	// Use dispatcher if available
	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
			commands = append(commands, cmd.Name+" - "+cmd.Help)
		}
		// Add plugin commands
		for _, cmd := range ctx.Dispatcher().Registry().All() {
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
			"daemon shutdown - Gracefully shutdown the daemon",
			"daemon status - Show daemon status",
			"peer <selector> list - List peer(s) (brief)",
			"peer <selector> show - Show peer(s) details",
			"system help - Show available commands",
			"system version software - Show ze version",
			"system version api - Show IPC protocol version",
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleSystemVersionSoftware returns ze version information.
func handleSystemVersionSoftware(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"version":    version,
			"build-date": buildDate,
		},
	}, nil
}

// handleSystemVersionAPI returns IPC protocol version.
func handleSystemVersionAPI(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"version": APIVersion,
		},
	}, nil
}

// handleDaemonShutdown signals the reactor to stop.
func handleDaemonShutdown(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	ctx.Reactor().Stop()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"message": "shutdown initiated",
		},
	}, nil
}

// handleDaemonQuit dumps all goroutine stacks then shuts down.
func handleDaemonQuit(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	buf := make([]byte, 1<<20) // 1MB
	n := runtime.Stack(buf, true)
	slog.Warn("goroutine dump (quit)", "stacks", string(buf[:n]))
	ctx.Reactor().Stop()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"message": "quit initiated (goroutines dumped)",
		},
	}, nil
}

// handleDaemonStatus returns daemon status.
func handleDaemonStatus(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	stats := ctx.Reactor().Stats()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"uptime":     stats.Uptime.String(),
			"peer_count": stats.PeerCount,
			"start_time": stats.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		},
	}, nil
}

// handleDaemonReload reloads the configuration.
// Routes through the coordinator (verify→apply across all plugins) when a config loader
// is available. Falls back to direct Reactor.Reload() when no coordinator is configured
// (e.g., no Server, or no config loader set).
func handleDaemonReload(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	// Use coordinator path when available: reloads config from disk, verifies with
	// all plugins that registered WantsConfigRoots, then applies to each.
	if ctx.Server != nil && ctx.Server.HasConfigLoader() {
		if err := ctx.Server.ReloadFromDisk(ctx.Server.Context()); err != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("reload failed: %v", err),
			}, err
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: map[string]any{
				"message": "configuration reloaded",
			},
		}, nil
	}

	// Fallback: direct reactor reload (BGP peer reconciliation only).
	if err := ctx.Reactor().Reload(); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("reload failed: %v", err),
		}, err
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"message": "configuration reloaded",
		},
	}, nil
}

// handleSystemSubsystemList returns available subsystems.
func handleSystemSubsystemList(_ *CommandContext, _ []string) (*plugin.Response, error) {
	// For now, bgp is always available
	// Future: query reactor for enabled subsystems
	subsystems := []string{"bgp"}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"subsystems": subsystems,
		},
	}, nil
}

// handleSystemCommandList returns all commands (builtin + plugin).
func handleSystemCommandList(ctx *CommandContext, args []string) (*plugin.Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose

	var commands []Completion

	// Add builtin commands
	if ctx.Dispatcher() != nil {
		for _, cmd := range ctx.Dispatcher().Commands() {
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
		for _, cmd := range ctx.Dispatcher().Registry().All() {
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

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"commands": commands,
		},
	}, nil
}

// handleSystemCommandHelp returns detailed help for a specific command.
func handleSystemCommandHelp(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: system command help \"<name>\"",
		}, fmt.Errorf("missing command name")
	}

	return LookupCommandHelp(ctx, args[0], "command")
}

// LookupCommandHelp looks up a command by name in builtins then plugins.
// The kind parameter is used in error messages (e.g., "command", "rib command").
func LookupCommandHelp(ctx *CommandContext, name, kind string) (*plugin.Response, error) {
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

		if cmd := ctx.Dispatcher().Registry().Lookup(name); cmd != nil {
			return &plugin.Response{
				Status: plugin.StatusDone,
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

	return &plugin.Response{
		Status: plugin.StatusError,
		Data:   fmt.Sprintf("unknown %s: %s", kind, name),
	}, fmt.Errorf("unknown %s: %s", kind, name)
}

// handleSystemCommandComplete returns completions for partial input.
// Usage:
//
//	system command complete "<partial>"           - command completion
//	system command complete "<cmd>" args "<partial>" - arg completion
func handleSystemCommandComplete(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
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

	if ctx.Dispatcher() != nil {
		// Complete builtins
		lowerPartial := strings.ToLower(partial)
		for _, cmd := range ctx.Dispatcher().Commands() {
			if strings.HasPrefix(strings.ToLower(cmd.Name), lowerPartial) {
				completions = append(completions, Completion{
					Value: cmd.Name,
					Help:  cmd.Help,
				})
			}
		}

		// Complete plugin commands
		completions = append(completions, ctx.Dispatcher().Registry().Complete(partial)...)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"completions": completions,
		},
	}, nil
}

// handleArgComplete handles argument completion for a specific command.
func handleArgComplete(ctx *CommandContext, cmdName string, completedArgs []string, partial string) (*plugin.Response, error) {
	emptyResult := &plugin.Response{
		Status: plugin.StatusDone,
		Data:   map[string]any{"completions": []Completion{}},
	}

	if ctx.Dispatcher() == nil {
		return emptyResult, nil
	}

	// Check if it's a plugin command with completable flag
	cmd := ctx.Dispatcher().Registry().Lookup(cmdName)
	if cmd == nil || !cmd.Completable {
		return emptyResult, nil
	}

	// Route completion request to process
	proc := cmd.Process
	if proc == nil || !proc.Running() {
		return emptyResult, nil
	}

	// Create response channel
	respCh := make(chan *plugin.Response, 1)

	// Add pending request with completion timeout
	serial := ctx.Dispatcher().Pending().Add(&PendingRequest{
		Command:  cmd.Name,
		Process:  proc,
		Timeout:  CompletionTimeout,
		RespChan: respCh,
	})

	if serial == "" {
		return emptyResult, nil
	}

	// Send completion request via RPC
	conn := proc.Conn()
	if conn == nil {
		ctx.Dispatcher().Pending().Complete(serial, emptyResult)
		return <-respCh, nil
	}
	rpcCtx, cancel := context.WithTimeout(context.Background(), CompletionTimeout)
	defer cancel()
	rpcOut, rpcErr := conn.SendExecuteCommand(rpcCtx, serial, cmd.Name, completedArgs, partial)
	switch {
	case rpcErr != nil:
		ctx.Dispatcher().Pending().Complete(serial, emptyResult)
	case rpcOut != nil:
		ctx.Dispatcher().Pending().Complete(serial, &plugin.Response{Status: rpcOut.Status, Data: rpcOut.Data})
	case rpcOut == nil: // no output and no error — complete with empty result
		ctx.Dispatcher().Pending().Complete(serial, emptyResult)
	}

	// Wait for response
	resp := <-respCh
	return resp, nil
}
