// Design: docs/architecture/api/process-protocol.md — plugin process management
// Overview: register.go — RPC registration hub

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errMissingCommand              = errors.New("missing command")
	errDispatcherNotAvailable      = errors.New("dispatcher not available")
	errRebootFunctionNotConfigured = errors.New("reboot function not configured")
	errShutdownNotConfigured       = errors.New("shutdown not available: no reactor and no shutdown function configured")
)

func init() {
	RegisterRPCs(
		RPCRegistration{WireMethod: "ze-system:help", Handler: handleSystemHelp},
		RPCRegistration{WireMethod: "ze-system:version-software", Handler: handleSystemVersionSoftware},
		RPCRegistration{WireMethod: "ze-system:version-api", Handler: handleSystemVersionAPI},
		RPCRegistration{WireMethod: "ze-system:daemon-shutdown", Handler: handleDaemonShutdown},
		RPCRegistration{WireMethod: "ze-system:daemon-reboot", Handler: handleDaemonReboot},
		RPCRegistration{WireMethod: "ze-system:daemon-quit", Handler: handleDaemonQuit},
		RPCRegistration{WireMethod: "ze-system:daemon-status", Handler: handleDaemonStatus},
		RPCRegistration{WireMethod: "ze-system:daemon-reload", Handler: handleDaemonReload},
		RPCRegistration{WireMethod: "ze-system:subsystem-list", Handler: handleSystemSubsystemList},
		RPCRegistration{WireMethod: "ze-system:command-list", Handler: handleSystemCommandList},
		RPCRegistration{WireMethod: "ze-system:command-help", Handler: handleSystemCommandHelp},
		RPCRegistration{WireMethod: "ze-system:command-complete", Handler: handleSystemCommandComplete},
		RPCRegistration{WireMethod: "ze-system:dispatch", Handler: handleSystemDispatch},
	)
}

// handleSystemDispatch dispatches a text command through the standard command dispatcher.
// This enables API socket clients to invoke any command reachable through the text
// dispatcher, including plugin-registered commands (e.g., "request bgp watchdog announce dnsr").
// Args are joined into a single command string for the dispatcher.
func handleSystemDispatch(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: system dispatch \"<command>\"",
		}, errMissingCommand
	}

	d := ctx.Dispatcher()
	if d == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "dispatcher not available",
		}, errDispatcherNotAvailable
	}

	command := textbuf.Join(args, " ")
	return d.Dispatch(ctx, command)
}

// handleSystemHelp returns list of available commands.
func handleSystemHelp(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	var commands []string

	// Use dispatcher if available
	if ctx.Dispatcher() != nil {
		var tb textbuf.Buffer
		for _, cmd := range ctx.Dispatcher().Commands() {
			commands = append(commands, tb.Reset().Str(cmd.Name).Str(" - ").Str(cmd.Help).String())
		}
		// Add plugin commands (skip hidden)
		for _, cmd := range ctx.Dispatcher().Registry().All() {
			if cmd.Hidden {
				continue
			}
			tb.Reset().Str(cmd.Name)
			if cmd.Args != "" {
				tb.Byte(' ').Str(cmd.Args)
			}
			tb.Str(" - ").Str(cmd.Description)
			commands = append(commands, tb.String())
		}
	}

	// Fallback if no dispatcher
	if len(commands) == 0 {
		commands = []string{
			"request shutdown - Gracefully shutdown",
			"request reboot - Gracefully shutdown then reboot the system",
			"request halt - Dump goroutine stacks and terminate",
			"show status - Show process status",
			"show bgp peer list - List peers (brief)",
			"show bgp summary - BGP summary table",
			"system help - Show available commands",
			"system version software - Show ze version",
			"system version api - Show IPC protocol version",
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"commands": commands,
		},
	}, nil
}

// handleSystemVersionSoftware returns ze version information.
func handleSystemVersionSoftware(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"version":    version,
			"build-date": buildDate,
		},
	}, nil
}

// handleSystemVersionAPI returns IPC protocol version.
func handleSystemVersionAPI(_ *CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"version": APIVersion,
		},
	}, nil
}

// handleDaemonShutdown stops the daemon. A BGP daemon stops the reactor (which
// runs its graceful-shutdown sequence); a reactorless daemon (e.g. OSPF-only)
// has no reactor, so it falls back to the daemon-provided shutdownFunc that
// triggers the same signal-based teardown as SIGTERM. Without the fallback a
// reactorless daemon could not be stopped by command and hung until timeout.
func handleDaemonShutdown(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	// Coordinator.FullReactor returns the coordinator ITSELF as a no-op fallback
	// when no BGP reactor is registered, so ctx.Reactor() is non-nil even for an
	// OSPF-only daemon and its Stop() does nothing. Only a real reactor (not the
	// *plugin.Coordinator fallback) is stopped directly; otherwise fall back to
	// the daemon shutdownFunc (signal-based teardown). Without this a reactorless
	// daemon hangs until timeout on `request shutdown`.
	r := ctx.Reactor()
	if _, isFallback := r.(*plugin.Coordinator); r != nil && !isFallback {
		r.Stop()
		return shutdownInitiated(), nil
	}
	if ctx.Server != nil && ctx.Server.shutdownFunc != nil {
		ctx.Server.shutdownFunc()
		return shutdownInitiated(), nil
	}
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "shutdown not available: no reactor and no shutdown function configured",
	}, errShutdownNotConfigured
}

func shutdownInitiated() *plugin.Response {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"message": "shutdown initiated",
		},
	}
}

// handleDaemonReboot signals a system reboot after graceful shutdown.
// The reboot function is wired by the daemon at startup via SetRebootFunc.
// The response is sent to the caller before the shutdown sequence begins.
func handleDaemonReboot(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	_, errResp, err := RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}
	if ctx.Server == nil || ctx.Server.rebootFunc == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "reboot not available",
		}, errRebootFunctionNotConfigured
	}
	ctx.Server.rebootFunc()
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"message": "reboot initiated",
		},
	}, nil
}

// SetShutdownFunc sets a reactor-independent daemon-shutdown callback. The
// daemon wires it (ungated by BGP) to the same signal-based teardown SIGTERM
// triggers, so `request shutdown` stops a reactorless daemon (OSPF-only, etc.).
func (s *Server) SetShutdownFunc(fn func()) {
	s.shutdownFunc = fn
}

// SetRebootFunc sets the function called for "daemon reboot" commands.
// Called by the daemon to wire graceful shutdown + OS reboot.
func (s *Server) SetRebootFunc(fn func()) {
	s.rebootFunc = fn
}

// handleDaemonQuit dumps all goroutine stacks then shuts down. Like
// handleDaemonShutdown it must not rely on a BGP reactor: on a reactorless
// daemon ctx.Reactor() is the no-op Coordinator fallback, so it stops via the
// daemon shutdownFunc instead (otherwise quit dumps stacks but never exits).
func handleDaemonQuit(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	buf := make([]byte, 1<<20) // 1MB
	n := runtime.Stack(buf, true)
	slog.Warn("goroutine dump (quit)", "stacks", string(buf[:n]))

	r := ctx.Reactor()
	if _, isFallback := r.(*plugin.Coordinator); r != nil && !isFallback {
		r.Stop()
		return quitInitiated(), nil
	}
	if ctx.Server != nil && ctx.Server.shutdownFunc != nil {
		ctx.Server.shutdownFunc()
		return quitInitiated(), nil
	}
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "quit not available: no reactor and no shutdown function configured",
	}, errShutdownNotConfigured
}

func quitInitiated() *plugin.Response {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"message": "quit initiated (goroutines dumped)",
		},
	}
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
		Data: plugin.Map{
			"uptime":     stats.Uptime.Truncate(time.Second).String(),
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
	if ctx.Server != nil && ctx.Server.hasFullReloadFunc() {
		if err := ctx.Server.reloadFull(ctx.Context()); err != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  func() string { var tb textbuf.Buffer; return tb.Str("reload failed: ").Err(err).String() }(),
			}, err
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				"message": "configuration reloaded",
			},
		}, nil
	}

	// Use coordinator path when available: reloads config from disk, verifies with
	// all plugins that registered WantsConfigRoots, then applies to each.
	if ctx.Server != nil && ctx.Server.HasConfigLoader() {
		if err := ctx.Server.ReloadFromDisk(ctx.Server.Context()); err != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  func() string { var tb textbuf.Buffer; return tb.Str("reload failed: ").Err(err).String() }(),
			}, err
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				"message": "configuration reloaded",
			},
		}, nil
	}

	// Fallback: direct reactor reload (BGP peer reconciliation only).
	if err := ctx.Reactor().Reload(); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  func() string { var tb textbuf.Buffer; return tb.Str("reload failed: ").Err(err).String() }(),
		}, err
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"message": "configuration reloaded",
		},
	}, nil
}

// handleSystemSubsystemList returns available subsystems with their state.
func handleSystemSubsystemList(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"subsystems": []any{}, "count": 0},
		}, nil
	}
	pm := ctx.Server.ProcessManager()
	if pm == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"subsystems": []any{}, "count": 0},
		}, nil
	}
	procs := pm.AllProcesses()
	out := make([]map[string]any, 0, len(procs))
	for _, p := range procs {
		out = append(out, map[string]any{
			"name":          p.Name(),
			"stage":         p.Stage().String(),
			"running":       p.Running(),
			"command-count": len(p.RegisteredCommands()),
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"subsystems": out, "count": len(out)},
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
				Value:  cmd.Name,
				Help:   cmd.Description,
				Hidden: cmd.Hidden,
			}
			if verbose {
				c.Source = cmd.Process.Name()
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

// handleSystemCommandHelp returns detailed help for a specific command.
func handleSystemCommandHelp(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: system command help \"<name>\"",
		}, errMissingCommandName
	}

	return lookupCommandHelp(ctx, args[0], "command")
}

// lookupCommandHelp looks up a command by name in builtins then plugins.
// The kind parameter is used in error messages (e.g., "command", "bgp rib command").
func lookupCommandHelp(ctx *CommandContext, name, kind string) (*plugin.Response, error) {
	if ctx.Dispatcher() != nil {
		if cmd := ctx.Dispatcher().Lookup(name); cmd != nil {
			return &plugin.Response{
				Status: plugin.StatusDone,
				Data: plugin.Map{
					"command":     cmd.Name,
					"description": cmd.Help,
					"source":      sourceBuiltin,
				},
			}, nil
		}

		if cmd := ctx.Dispatcher().Registry().Lookup(name); cmd != nil {
			return &plugin.Response{
				Status: plugin.StatusDone,
				Data: plugin.Map{
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
		Error:  func() string { var tb textbuf.Buffer; return tb.Str("unknown ").Str(kind).Str(": ").Str(name).String() }(),
	}, fmt.Errorf("unknown %s: %s", kind, name)
}

// handleSystemCommandComplete returns completions for partial input.
// Usage:
//
//	system command complete "<partial>"                   - command completion
//	system command complete args "<cmd>" "<partial>"      - arg completion
//	system command complete args "<cmd>" <done...> "<partial>"
func handleSystemCommandComplete(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: system command complete \"<partial>\"",
		}, errMissingPartialInput
	}

	// Arg completion: the keyword must come before the free-form command string.
	if args[0] == "args" {
		if len(args) < 3 {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  "usage: system command complete args \"<cmd>\" [<completed>...] \"<partial>\"",
			}, errMissingPartialInput
		}
		cmdName := args[1]
		partialArg := args[len(args)-1]
		var completedArgs []string
		if len(args) > 3 {
			completedArgs = args[2 : len(args)-1]
		}
		return handleArgComplete(ctx, cmdName, completedArgs, partialArg)
	}

	partial := args[0]

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
		Data: plugin.Map{
			"completions": completions,
		},
	}, nil
}

// handleArgComplete handles argument completion for a specific command.
func handleArgComplete(ctx *CommandContext, cmdName string, completedArgs []string, partial string) (*plugin.Response, error) {
	emptyResult := &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"completions": []Completion{}},
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
	case rpcOut != nil && rpcOut.Status == plugin.StatusError:
		ctx.Dispatcher().Pending().Complete(serial, &plugin.Response{Status: plugin.StatusError, Error: string(rpcOut.Data)})
	case rpcOut != nil:
		ctx.Dispatcher().Pending().Complete(serial, &plugin.Response{Status: rpcOut.Status, Data: plugin.RawJSON(rpcOut.Data)})
	case rpcOut == nil: // no output and no error — complete with empty result
		ctx.Dispatcher().Pending().Complete(serial, emptyResult)
	}

	// Wait for response
	resp := <-respCh
	return resp, nil
}
