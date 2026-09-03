// Design: docs/architecture/api/process-protocol.md — plugin process management
// Overview: register.go — RPC registration hub

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"runtime"
	"strings"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var (
	errMissingCommand              = errors.New("missing command")
	errDispatcherNotAvailable      = errors.New("dispatcher not available")
	errRebootFunctionNotConfigured = errors.New("reboot function not configured")
	errShutdownNotConfigured       = errors.New("shutdown not available: no reactor and no shutdown function configured")
)

// messageConfigReloaded is the answer every reload path gives once the new
// configuration is in force, whichever of the three routes applied it.
const messageConfigReloaded = "configuration reloaded"

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
			commands = append(commands, tb.Reset().Str(cmd.Name).Str(" - ").Str(cmd.Description).String())
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
			"show bgp - BGP summary table",
			"system help - Show available commands",
			"system version software - Show ze version",
			"system version api - Show IPC protocol version",
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldCommands: commands,
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

// handleDaemonShutdown accepts a daemon stop. Command transports defer the
// lifecycle action until after they have written the accepted response, so
// teardown cannot close the requesting process connection first. A BGP daemon
// stops its real reactor; a reactorless daemon uses the daemon-provided
// shutdownFunc that triggers the same signal-based teardown as SIGTERM.
func handleDaemonShutdown(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	// Coordinator.FullReactor returns the coordinator ITSELF as a no-op fallback
	// when no BGP reactor is registered, so ctx.Reactor() is non-nil even for an
	// OSPF-only daemon and its Stop() does nothing. Only a real reactor (not the
	// *plugin.Coordinator fallback) is stopped directly; otherwise fall back to
	// the daemon shutdownFunc (signal-based teardown). Without this a reactorless
	// daemon hangs until timeout on `request shutdown`.
	r := ctx.Reactor()
	if _, isFallback := r.(*plugin.Coordinator); r != nil && !isFallback {
		return shutdownInitiated(func() {
			r.Stop()
			if ctx.Server != nil {
				ctx.Server.signalShutdownRequested()
			}
		}), nil
	}
	if ctx.Server != nil && ctx.Server.shutdownFunc != nil {
		return shutdownInitiated(func() {
			ctx.Server.shutdownFunc()
			ctx.Server.signalShutdownRequested()
		}), nil
	}
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "shutdown not available: no reactor and no shutdown function configured",
	}, errShutdownNotConfigured
}

func shutdownInitiated(action func()) *plugin.Response {
	resp := &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldMessage: "shutdown initiated",
		},
	}
	resp.OnTransportComplete(action)
	return resp
}

// handleDaemonReboot accepts a system reboot after graceful shutdown. The
// reboot function and Server.Wait notification run only after the requesting
// command transport has written the accepted response.
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
	resp := &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldMessage: "reboot initiated",
		},
	}
	resp.OnTransportComplete(func() {
		ctx.Server.rebootFunc()
		ctx.Server.signalShutdownRequested()
	})
	return resp, nil
}

// SetShutdownFunc sets a reactor-independent daemon-shutdown callback. The
// daemon wires it (ungated by BGP) to the same signal-based teardown SIGTERM
// triggers, so `request shutdown` stops a reactorless daemon (OSPF-only, etc.).
func (s *Server) SetShutdownFunc(fn func()) {
	s.shutdownFunc = fn
}

// SetRebootFunc sets the lifecycle action accepted by "daemon reboot".
// Command transports run it after writing the accepted response.
func (s *Server) SetRebootFunc(fn func()) {
	s.rebootFunc = fn
}

// handleDaemonQuit dumps all goroutine stacks and accepts daemon shutdown. The
// lifecycle action follows the written accepted response, and it does not rely
// on a BGP reactor: a reactorless daemon uses shutdownFunc instead of the no-op
// Coordinator fallback.
func handleDaemonQuit(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	buf := make([]byte, 1<<20) // 1MB
	n := runtime.Stack(buf, true)
	slog.Warn("goroutine dump (quit)", "stacks", string(buf[:n]))

	r := ctx.Reactor()
	if _, isFallback := r.(*plugin.Coordinator); r != nil && !isFallback {
		return quitInitiated(func() {
			r.Stop()
			if ctx.Server != nil {
				ctx.Server.signalShutdownRequested()
			}
		}), nil
	}
	if ctx.Server != nil && ctx.Server.shutdownFunc != nil {
		return quitInitiated(func() {
			ctx.Server.shutdownFunc()
			ctx.Server.signalShutdownRequested()
		}), nil
	}
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "quit not available: no reactor and no shutdown function configured",
	}, errShutdownNotConfigured
}

func quitInitiated(action func()) *plugin.Response {
	resp := &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldMessage: "quit initiated (goroutines dumped)",
		},
	}
	resp.OnTransportComplete(action)
	return resp
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
	reloadCtx := contextWithReloadCaller(ctx.Context(), ctx.Process)
	if ctx.Server != nil && ctx.Server.hasFullReloadFunc() {
		if err := ctx.Server.reloadFull(reloadCtx); err != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  func() string { var tb textbuf.Buffer; return tb.Str("reload failed: ").Err(err).String() }(),
			}, err
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				fieldMessage: messageConfigReloaded,
			},
		}, nil
	}

	// Use coordinator path when available: reloads config from disk, verifies with
	// all plugins that registered WantsConfigRoots, then applies to each.
	if ctx.Server != nil && ctx.Server.HasConfigLoader() {
		if err := ctx.Server.ReloadFromDisk(reloadCtx); err != nil {
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  func() string { var tb textbuf.Buffer; return tb.Str("reload failed: ").Err(err).String() }(),
			}, err
		}
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data: plugin.Map{
				fieldMessage: messageConfigReloaded,
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
			fieldMessage: messageConfigReloaded,
		},
	}, nil
}

// handleSystemSubsystemList returns available subsystems with their state.
func handleSystemSubsystemList(ctx *CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldSubsystems: []any{}, fieldCount: 0},
		}, nil
	}
	pm := ctx.Server.ProcessManager()
	if pm == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldSubsystems: []any{}, fieldCount: 0},
		}, nil
	}
	// The dispatcher's registry is what command-count counts. A per-Process list
	// was the source until 2026-08-18 and nothing had fed it since the YANG RPC
	// migration, so every operator read 0.
	var counts map[string]int
	if d := ctx.Server.Dispatcher(); d != nil {
		counts = d.Registry().CommandCountsByProcess()
	}
	procs := pm.AllProcesses()
	out := make([]map[string]any, 0, len(procs))
	for _, p := range procs {
		out = append(out, map[string]any{
			"name":          p.Name(),
			"stage":         p.Stage().String(),
			"running":       p.Running(),
			"command-count": counts[p.Name()],
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldSubsystems: out, fieldCount: len(out)},
	}, nil
}

// handleSystemCommandList returns all commands (builtin + plugin).
//
// It answers with a row generator rather than a built collection, because this
// is the longest answer the daemon has: every builtin command and every command
// every plugin registered, each with its help text. A consumer that reads it
// through `| first 10` or `| match bgp` pays for the rows it keeps, and one that
// wants the whole list still receives the document it always received
// (rpc.CollapseRecords, pkg/plugin/rpc/collapse.go).
func handleSystemCommandList(ctx *CommandContext, args []string) (*plugin.Response, error) {
	verbose := len(args) > 0 && args[0] == argVerbose
	dispatcher := ctx.Dispatcher()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Records{
			Key:  fieldCommands,
			Rows: commandRows(dispatcher, verbose),
		},
	}, nil
}

// commandRows yields one row for each command the daemon can dispatch: the
// builtins first, then the commands the plugins registered.
//
// The dispatcher is read when the walk runs rather than when the handler
// returns, so the rows describe the registry at the moment the answer is
// written. Both getters copy under their own lock, so a plugin registering a
// command mid-walk changes the next answer and not this one.
func commandRows(dispatcher *Dispatcher, verbose bool) iter.Seq[rpc.RowRecord] {
	return func(yield func(rpc.RowRecord) bool) {
		if dispatcher == nil {
			return
		}
		// One row carries every row of the walk in turn. The encoder appends it
		// before the yield returns and keeps no reference to it (rpc.Row), so
		// the walk states each row through the same value instead of a value of
		// its own.
		var encoded rpc.RawRow

		for _, cmd := range dispatcher.Commands() {
			row := Completion{Value: cmd.Name, Description: cmd.Description, LongHelp: cmd.LongHelp}
			if verbose {
				row.Source = sourceBuiltin
			}
			if !yieldCompletion(yield, &encoded, row) {
				return
			}
		}
		for _, cmd := range dispatcher.Registry().All() {
			row := Completion{Value: cmd.Name, Description: cmd.Description, Hidden: cmd.Hidden, LongHelp: cmd.LongHelp}
			if verbose {
				row.Source = cmd.Process.Name()
			}
			if !yieldCompletion(yield, &encoded, row) {
				return
			}
		}
	}
}

// yieldCompletion encodes one row into encoded and hands it to the walk, and
// reports whether the walk wants another. A row that cannot be encoded is
// yielded as a rejected row. The answer then names the command it cannot
// report, instead of being one row shorter than the registry.
//
// The encoding happens HERE rather than inside the row's AppendTo, because a
// row that cannot be encoded is a rejected row and an appender has no way to
// say so. What the walk hands on is the appender over the bytes this produced
// (rpc.RawRow), so the encoder copies them into its own buffer and allocates
// nothing for the row.
func yieldCompletion(yield func(rpc.RowRecord) bool, encoded *rpc.RawRow, row Completion) bool {
	item, err := json.Marshal(row)
	if err != nil {
		fault, faultErr := json.Marshal(completionFault{Command: row.Value, Message: err.Error()})
		if faultErr != nil {
			return false
		}
		*encoded = fault
		return yield(rpc.RowRecord{Fault: encoded})
	}
	*encoded = item
	return yield(rpc.RowRecord{Item: encoded})
}

// completionFault is the rejected row that stands in for a command whose row
// cannot be encoded. Its fields are written in the order a reader meets
// them, which is the order the map it replaces happened to sort into.
type completionFault struct {
	Command string `json:"command"`
	Message string `json:"message"`
}

// handleSystemCommandHelp returns detailed help for a specific command.
func handleSystemCommandHelp(ctx *CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 1 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: system command help \"<name>\"",
		}, errMissingCommandName
	}

	return lookupCommandHelp(ctx, args[0], fieldCommand)
}

// lookupCommandHelp looks up a command by name in builtins then plugins.
// The kind parameter is used in error messages (e.g., "command", "bgp rib command").
func lookupCommandHelp(ctx *CommandContext, name, kind string) (*plugin.Response, error) {
	if ctx.Dispatcher() != nil {
		if cmd := ctx.Dispatcher().Lookup(name); cmd != nil {
			return &plugin.Response{
				Status: plugin.StatusDone,
				Data: plugin.Map{
					fieldCommand:     cmd.Name,
					fieldDescription: cmd.Description,
					fieldLongHelp:    cmd.LongHelp,
					fieldSource:      sourceBuiltin,
				},
			}, nil
		}

		if cmd := ctx.Dispatcher().Registry().Lookup(name); cmd != nil {
			return &plugin.Response{
				Status: plugin.StatusDone,
				Data: plugin.Map{
					fieldCommand:     cmd.Name,
					fieldDescription: cmd.Description,
					fieldLongHelp:    cmd.LongHelp,
					fieldArgs:        cmd.Args,
					fieldSource:      cmd.Process.Name(),
					"timeout":        cmd.Timeout.String(),
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
	if args[0] == fieldArgs {
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
					Value:       cmd.Name,
					Description: cmd.Description,
				})
			}
		}

		// Complete plugin commands
		completions = append(completions, ctx.Dispatcher().Registry().Complete(partial)...)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldCompletions: completions,
		},
	}, nil
}

// handleArgComplete handles argument completion for a specific command.
func handleArgComplete(ctx *CommandContext, cmdName string, completedArgs []string, partial string) (*plugin.Response, error) {
	emptyResult := &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldCompletions: []Completion{}},
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
	completion := emptyResult
	if conn != nil {
		rpcCtx, cancel := context.WithTimeout(context.Background(), CompletionTimeout)
		defer cancel()
		input := &rpc.ExecuteCommandInput{Serial: serial, Command: cmd.Name, Args: completedArgs, Peer: partial}
		rpcOut, rpcErr := conn.SendExecuteCommand(rpcCtx, input)
		switch {
		case rpcErr != nil: // The empty result is the answer to a failed call.
		case rpcOut != nil && rpcOut.Status == plugin.StatusError:
			completion = &plugin.Response{Status: plugin.StatusError, Error: string(rpcOut.Data)}
		case rpcOut != nil:
			completion = &plugin.Response{Status: rpcOut.Status, Data: plugin.RawJSON(rpcOut.Data)}
		}
	}

	if err := ctx.Dispatcher().Pending().Complete(serial, completion); err != nil {
		slog.Warn("plugin completion: answer not delivered",
			"command", cmd.Name, "serial", serial, "error", err)
	}

	// The pending request holds respCh, so the answer is already in it: the one
	// Complete just delivered, or the timeout error that removed the request
	// before it. A completion is best-effort, so a request that left nothing
	// behind answers empty rather than waiting for a write nobody will make.
	select {
	case resp := <-respCh:
		return resp, nil
	default:
		return emptyResult, nil
	}
}
