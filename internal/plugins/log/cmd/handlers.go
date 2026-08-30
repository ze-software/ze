// Design: docs/architecture/api/commands.md — log show and set handlers

package cmd

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The CLI keyword the handlers accept, which is also the JSON key they answer
// with. One spelling for each so a rename cannot reach only one of the two.
const (
	fieldCount = "count"
	fieldLevel = "level"
)

// RPCs returns the RPC registrations for log commands.
// The caller is responsible for passing these to pluginserver.RegisterRPCs.
func RPCs() []pluginserver.RPCRegistration {
	return []pluginserver.RPCRegistration{
		{WireMethod: "ze-bgp:log-levels", Handler: handleLogLevels},
		{WireMethod: "ze-bgp:log-set", Handler: handleLogSet},
		{WireMethod: "ze-bgp:log-recent", Handler: handleLogRecent},
	}
}

func handleLogLevels(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	levels := slogutil.ListLevels()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"levels":   levels,
			fieldCount: len(levels),
		},
	}, nil
}

func handleLogSet(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
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

func handleLogRecent(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	ring := slogutil.GlobalLogRing()
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
		case "component":
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
	entries := ring.Snapshot(limit, level, component)
	out := make([]map[string]any, 0, len(entries))
	for i := range entries {
		out = append(out, map[string]any{
			"timestamp": entries[i].Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			fieldLevel:  entries[i].Level,
			"component": entries[i].Component,
			"message":   entries[i].Message,
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"entries": out, fieldCount: len(out)},
	}, nil
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
