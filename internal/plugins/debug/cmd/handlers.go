// Design: docs/architecture/diagnostics/debug-filtering.md -- live debug state query via RPC
// Related: internal/core/slogutil/ -- level and filter registries

package cmd

import (
	"sort"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// RPCs returns the RPC registrations for debug commands.
func RPCs() []pluginserver.RPCRegistration {
	return []pluginserver.RPCRegistration{
		{WireMethod: "ze-debug:debug-state", Handler: handleDebugState},
	}
}

func handleDebugState(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	levels := slogutil.ListLevels()

	names := make([]string, 0, len(levels))
	for name := range levels {
		names = append(names, name)
	}
	sort.Strings(names)

	subsystems := make([]map[string]any, 0, len(names))
	for _, name := range names {
		entry := map[string]any{
			"name":  name,
			"level": levels[name],
		}
		if state := slogutil.ActiveFilter(name); state != nil {
			if len(state.Flags) > 0 {
				entry["flags"] = state.Flags
			}
			if len(state.Scopes) > 0 {
				entry["scopes"] = state.Scopes
			}
		}
		subsystems = append(subsystems, entry)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"subsystems": subsystems,
			"count":      len(subsystems),
		},
	}, nil
}
