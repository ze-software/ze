// Design: docs/architecture/diagnostics/production-diagnostics.md -- show system watchdog handler
// Related: show.go -- ze-cmd:show-system-watchdog registration
// Related: triage.go -- watchdog summary counts in show system triage

package show

import (
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/watchdog"
)

// handleShowSystemWatchdog returns the live daemon liveness watchdog snapshot
// alongside active/recent hung command handlers under leaf locks only.
func handleShowSystemWatchdog(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	st := watchdog.Status()

	hung := []pluginserver.HungHandlerRecord{}
	if ctx != nil {
		if ctx.Server != nil {
			if records := ctx.Server.HungHandlers(); records != nil {
				hung = records
			}
		}
	}

	probes := st.Probes
	if probes == nil {
		probes = []watchdog.ProbeState{}
	}
	stalls := st.RecentStalls
	if stalls == nil {
		stalls = []watchdog.StallRecord{}
	}

	data := plugin.Map{
		"running":            st.Running,
		"interval-ms":        st.IntervalMS,
		"stall-threshold-ms": st.StallThresholdMS,
		"ticks":              st.Ticks,
		"stall-count":        st.StallCount,
		"probes":             probes,
		"hung-handlers":      hung,
		"recent-stalls":      stalls,
	}
	if !st.LastTickAt.IsZero() {
		data["last-tick-at"] = st.LastTickAt.UTC().Format(time.RFC3339)
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}
