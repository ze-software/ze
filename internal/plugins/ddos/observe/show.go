// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- show ddos surface
//
// The observe plugin runs in-process (plugins are goroutines), so the show
// handlers read the live incident ring directly from the process-global store
// published in register.go. Mirrors the anomaly/detect show surface.

package observe

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// fieldEnabled is the payload key that says whether ddos observation is
// running. Both show handlers return it, so a false reading always means the
// same thing to the operator.
const fieldEnabled = "enabled"

// fieldIncidents is the payload key that carries the incident ring, and the
// same key that carries the ring's SIZE in the status answer. Both show
// handlers write it on both their branches, so naming it once is what keeps a
// nil store answering under the key a live store answers under.
const fieldIncidents = "incidents"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:ddos-status",
			Handler:    handleShowDdos,
		},
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:ddos-incidents",
			Handler:    handleShowDdosIncidents,
		},
	)
}

// handleShowDdos returns a one-line status: whether observation is running, how
// many attacks are currently active, and how many incidents are held in the ring.
func handleShowDdos(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	s := activeStore.Load()
	if s == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldEnabled: false, "active-attacks": 0, fieldIncidents: 0},
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldEnabled:     true,
			"active-attacks": s.activeCount(),
			fieldIncidents:   s.count(),
		},
	}, nil
}

// handleShowDdosIncidents returns the incident ring newest-first (the JSON-tagged
// incident struct: id, interface, target vector, family, top-sources, peak
// pps/bps, start/end time, active flag).
func handleShowDdosIncidents(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	s := activeStore.Load()
	if s == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{fieldEnabled: false, fieldIncidents: []incident{}},
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldEnabled: true, fieldIncidents: s.list()},
	}, nil
}
