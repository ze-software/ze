// Design: docs/architecture/api/commands.md — BGP log show and set handlers
// Overview: doc.go — bgp-cmd-log plugin registration

package log

import (
	"fmt"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:log-levels", Handler: handleLogLevels},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:log-set", Handler: handleLogSet},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:log-recent", Handler: handleLogRecent},
	)
}

// handleLogLevels returns a map of subsystem names to their current log levels.
func handleLogLevels(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	levels := slogutil.ListLevels()

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"levels": levels,
			"count":  len(levels),
		},
	}, nil
}

// handleLogSet changes the log level for a subsystem at runtime.
func handleLogSet(_ *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	if len(args) < 2 {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   "usage: bgp log set <subsystem> <level>",
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
		case "level":
			if i+1 >= len(args) {
				return &plugin.Response{
					Status: plugin.StatusError,
					Data:   "log recent: \"level\" requires a value",
				}, nil
			}
			i++
			level = args[i]
		case "component":
			if i+1 >= len(args) {
				return &plugin.Response{
					Status: plugin.StatusError,
					Data:   "log recent: \"component\" requires a value",
				}, nil
			}
			i++
			component = args[i]
		case "count":
			if i+1 >= len(args) {
				return &plugin.Response{
					Status: plugin.StatusError,
					Data:   "log recent: \"count\" requires a value",
				}, nil
			}
			i++
			n, _ := strconv.Atoi(args[i])
			if n < 1 {
				return &plugin.Response{
					Status: plugin.StatusError,
					Data:   fmt.Sprintf("log recent: count %q: not a positive number", args[i]),
				}, nil
			}
			limit = n
		default:
			return &plugin.Response{
				Status: plugin.StatusError,
				Data:   fmt.Sprintf("log recent: unknown option %q", args[i]),
			}, nil
		}
	}
	entries := ring.Snapshot(limit, level, component)
	out := make([]map[string]any, 0, len(entries))
	for i := range entries {
		out = append(out, map[string]any{
			"timestamp": entries[i].Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
			"level":     entries[i].Level,
			"component": entries[i].Component,
			"message":   entries[i].Message,
		})
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   map[string]any{"entries": out, "count": len(out)},
	}, nil
}

// setLevel validates and applies the level change, returning an appropriate response.
func setLevel(subsystem, levelStr string) *plugin.Response {
	if err := slogutil.SetLevel(subsystem, levelStr); err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   err.Error(),
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"subsystem": subsystem,
			"level":     levelStr,
		},
	}
}
