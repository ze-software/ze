// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- show ddos flowspec surface
//
// The flowspec responder runs in-process (plugins are goroutines), so the show
// handler reads its live announcement state directly from the process-global
// responder published in register.go. Mirrors the ddos-observe show surface.

package flowspec

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod: "ze-show:ddos-flowspec",
		Handler:    handleShowDdosFlowspec,
	})
}

// handleShowDdosFlowspec reports whether an upstream FlowSpec rule is currently
// announced, the target vector it covers, and whether the leak-probe is running.
func handleShowDdosFlowspec(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	r := activeResponder.Load()
	if r == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"enabled": false, "active": false},
		}, nil
	}
	active, target, probing := r.status()
	data := plugin.Map{"enabled": true, "active": active}
	if active {
		data["target"] = target
		data["probing"] = probing
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}
