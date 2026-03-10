// Design: docs/architecture/api/commands.md — BGP log show and set handlers
// Overview: doc.go — bgp-cmd-log plugin registration

package log

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:log-levels", CLICommand: "bgp log levels", Handler: handleLogLevels, Help: "Subsystem log levels", ReadOnly: true},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:log-set", CLICommand: "bgp log set", Handler: handleLogSet, Help: "Set subsystem log level at runtime"},
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
