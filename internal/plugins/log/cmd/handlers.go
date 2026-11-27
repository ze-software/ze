// Design: docs/architecture/api/commands.md — log show and set handlers

package cmd

import (
	"context"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The CLI keyword the handlers accept, which is also the JSON key they answer
// with. One spelling for each so a rename cannot reach only one of the two.
const (
	fieldCount       = "count"
	fieldLevel       = "level"
	fieldComponent   = "component"
	fieldEntries     = "entries"
	fieldOperationID = "operation-id"
	pipeOpPlugin     = "plugin"
	pipeOpOperation  = "operation"
)

// inspectLogPluginFunc routes plugin log inspection through Server.InspectPlugin.
// It is a package variable so unit tests in this package can exercise external
// and fallback formatting deterministically; never written outside tests.
var inspectLogPluginFunc = func(
	s *pluginserver.Server,
	ctx context.Context,
	name string,
	input rpc.RuntimeInspectInput,
) (*pluginserver.PluginInspectResult, *opererr.Error) {
	return s.InspectPlugin(ctx, name, input)
}

// RPCs returns the RPC registrations for log commands.
// The caller is responsible for passing these to pluginserver.RegisterRPCs.
func RPCs() []pluginserver.RPCRegistration {
	return []pluginserver.RPCRegistration{
		{WireMethod: "ze-log:bgp-log-levels", Handler: handleLogLevels},
		{WireMethod: "ze-log:bgp-log-set", Handler: handleLogSet},
		{WireMethod: "ze-log:bgp-log-recent", Handler: handleLogRecent},
	}
}

func handleLogLevels(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	levels := slogutil.ListLevels()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"levels":   levels,
			fieldCount: len(levels),
		},
	}, nil
}

func handleLogSet(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	args := validated.Tokens()
	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "usage: request log level <subsystem> <level>",
		}, nil
	}

	subsystem := args[0]
	levelStr := args[1]

	return setLevel(subsystem, levelStr), nil
}

func handleLogRecent(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	pluginName, hasPlugin, refusal := extractLogPluginPipeTarget(validated)
	if refusal != nil {
		return refusal, nil
	}
	opID, _, opRefusal := extractLogOperationPipeTarget(validated)
	if opRefusal != nil {
		return opRefusal, nil
	}

	args := validated.Tokens()
	level, component, limit := "", "", 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case fieldLevel:
			if i+1 >= len(args) {
				return &plugin.Response{
					Status: plugin.StatusError,
					Error:  "log recent: \"level\" requires a value",
				}, nil
			}
			i++
			level = args[i]
		case fieldComponent:
			if i+1 >= len(args) {
				return &plugin.Response{
					Status: plugin.StatusError,
					Error:  "log recent: \"component\" requires a value",
				}, nil
			}
			i++
			component = args[i]
		case fieldCount:
			if i+1 >= len(args) {
				return &plugin.Response{
					Status: plugin.StatusError,
					Error:  "log recent: \"count\" requires a value",
				}, nil
			}
			i++
			n, _ := strconv.Atoi(args[i])
			if n < 1 {
				return &plugin.Response{
					Status: plugin.StatusError,
					Error:  "log recent: count " + strconv.Quote(args[i]) + ": not a positive number",
				}, nil
			}
			limit = n
		default:
			return &plugin.Response{
				Status: plugin.StatusError,
				Error:  "log recent: unknown option " + strconv.Quote(args[i]),
			}, nil
		}
	}

	if hasPlugin {
		return inspectPluginLogRecent(ctx, pluginName, level, component, limit, opID)
	}

	ring := slogutil.GlobalLogRing()
	entries, err := ring.SnapshotWithOp(limit, level, component, opID)
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: "log recent: " + err.Error()}, nil //nolint:nilerr // the operator's error travels in the Response, the transport succeeded
	}
	out := make([]map[string]any, 0, len(entries))
	for i := range entries {
		attributes, err := entries[i].Attributes()
		if err != nil {
			return nil, err
		}
		row := map[string]any{
			"timestamp":    entries[i].Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			fieldLevel:     entries[i].Level,
			fieldComponent: entries[i].Component,
			"message":      entries[i].Message,
			"attributes":   attributes,
		}
		if entries[i].OpID != "" {
			row[fieldOperationID] = entries[i].OpID
		}
		out = append(out, row)
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldEntries: out, fieldCount: len(out)},
	}, nil
}

func extractLogSinglePipeTarget(
	validated command.ValidatedArgs,
	pipeName, duplicateMsg, duplicateWhy, requiredMsg, requiredWhy, extraMsg, extraWhy, example string,
) (value string, present bool, refusal *plugin.Response) {
	for _, op := range validated.PipeOperations() {
		if !strings.EqualFold(op.Name, pipeName) {
			continue
		}
		if present {
			return "", true, plugin.OperatorErrorResponse(opererr.New(duplicateMsg, duplicateWhy, example))
		}
		present = true
		if len(op.Args) == 0 {
			return "", true, plugin.OperatorErrorResponse(opererr.New(requiredMsg, requiredWhy, example))
		}
		if len(op.Args) > 1 {
			return "", true, plugin.OperatorErrorResponse(opererr.New(extraMsg, extraWhy, example))
		}
		value = strings.TrimSpace(op.Args[0])
	}
	if !present {
		return "", false, nil
	}
	if value == "" {
		return "", true, plugin.OperatorErrorResponse(opererr.New(requiredMsg, requiredWhy, example))
	}
	return value, true, nil
}

func extractLogPluginPipeTarget(validated command.ValidatedArgs) (name string, present bool, refusal *plugin.Response) {
	return extractLogSinglePipeTarget(
		validated,
		pipeOpPlugin,
		"duplicate | plugin pipe operation",
		"only one | plugin selector may be given on a command line",
		"| plugin requires a plugin name",
		"an endpoint name must follow | plugin",
		"| plugin takes exactly one plugin name",
		"only one endpoint name may follow | plugin",
		"ze show log recent | plugin <name>",
	)
}

func extractLogOperationPipeTarget(validated command.ValidatedArgs) (opID string, present bool, refusal *plugin.Response) {
	return extractLogSinglePipeTarget(
		validated,
		pipeOpOperation,
		"duplicate | operation pipe operation",
		"only one | operation filter may be given on a command line",
		"| operation requires an operation ID",
		"an operation ID must follow | operation",
		"| operation takes exactly one operation ID",
		"only one operation ID may follow | operation",
		"ze show log recent | operation <id>",
	)
}

func inspectPluginLogRecent(
	ctx *pluginserver.CommandContext,
	pluginName, level, component string,
	limit int,
	opID string,
) (*plugin.Response, error) {
	if ctx == nil {
		return logPluginServerUnavailableResponse(), nil
	}
	if ctx.Server == nil {
		return logPluginServerUnavailableResponse(), nil
	}

	cappedLimit := min(limit, rpc.RuntimeInspectMaxLogEntries)
	if opID != "" {
		cappedLimit = rpc.RuntimeInspectMaxLogEntries
	}

	res, opErr := inspectLogPluginFunc(ctx.Server, ctx.Context(), pluginName, rpc.RuntimeInspectInput{
		Kind:         rpc.RuntimeInspectLogRecent,
		LogLevel:     level,
		LogComponent: component,
		LogCount:     cappedLimit,
	})
	if opErr != nil {
		return plugin.OperatorErrorResponse(opErr), nil
	}

	if res.Fallback {
		data := map[string]any{
			fieldEntries:      []map[string]any{},
			fieldCount:        0,
			"plugin":          res.Plugin,
			"process-mode":    res.ProcessMode,
			"pid":             res.PID,
			"fallback":        true,
			"fallback-reason": res.FallbackReason,
			"rpc-error":       res.RPCError,
		}
		if res.Host != nil {
			data["running"] = res.Host.Running
			data["stage"] = res.Host.Stage
			stderrTail := res.Host.StderrTail
			if stderrTail == nil {
				stderrTail = []string{}
			}
			data["stderr-tail"] = stderrTail
			if res.Host.Proc != nil {
				data["host"] = res.Host.Proc
			} else if res.Host.ProcError != "" {
				data["procfs-error"] = res.Host.ProcError
			}
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(data)}, nil
	}

	out := make([]map[string]any, 0, len(res.Output.LogEntries))
	for i := range res.Output.LogEntries {
		e := &res.Output.LogEntries[i]
		entryOpID, _ := e.Attributes[fieldOperationID].(string)
		if opID != "" {
			if entryOpID != opID {
				continue
			}
		}
		row := map[string]any{
			"timestamp":    e.Timestamp,
			fieldLevel:     e.Level,
			fieldComponent: e.Component,
			"message":      e.Message,
			"attributes":   e.Attributes,
		}
		if entryOpID != "" {
			row[fieldOperationID] = entryOpID
		}
		out = append(out, row)
		if limit > 0 {
			if len(out) >= limit {
				break
			}
		}
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldEntries:   out,
			fieldCount:     len(out),
			"plugin":       res.Plugin,
			"process-mode": res.ProcessMode,
			"pid":          res.PID,
		},
	}, nil
}

func logPluginServerUnavailableResponse() *plugin.Response {
	return plugin.OperatorErrorResponse(opererr.New(
		"plugin inspection is unavailable",
		"the daemon plugin server is not running in this context",
		"ze show system subsystem list",
	))
}

func setLevel(subsystem, levelStr string) *plugin.Response {
	if err := slogutil.SetLevel(subsystem, levelStr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  err.Error(),
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"subsystem": subsystem,
			fieldLevel:  levelStr,
		},
	}
}
