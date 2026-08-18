// Design: docs/architecture/anomaly/anomaly-3-observe.md -- the show anomaly observe surface
//
// Related: store.go holds the ring this reads, register.go publishes it on
// activeStore.
//
// The plugin runs in-process (a plugin is a goroutine), so the handler reads the
// live ring from the process-global pointer rather than making an RPC call back
// into the plugin.

package observe

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:anomaly-observe",
			Handler:    handleShowAnomalyObserve,
		},
	)
}

// handleShowAnomalyObserve returns the incident lifecycle list newest-first, with
// finalized incidents and their end time included: that history is what `show
// anomaly detect` cannot show, since the detector's report ring records
// confirmations only.
//
// No store means the plugin is not running, which is reported as enabled false and
// an empty list rather than as an error: absence of a plugin is not a failure of
// the command.
func handleShowAnomalyObserve(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	s := activeStore.Load()
	if s == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"enabled": false, "active-count": 0, "incidents": []incident{}},
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"enabled":      true,
			"active-count": s.activeCount(),
			"incidents":    s.list(),
		},
	}, nil
}
