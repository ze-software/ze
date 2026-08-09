// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- show ddos local surface
//
// The local responder runs in-process (plugins are goroutines), so the show
// handler reads its live mitigation state directly from the process-global
// responder published in register.go. Mirrors the ddos-observe show surface.

package local

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod: "ze-show:ddos-local",
		Handler:    handleShowDdosLocal,
	})
}

// handleShowDdosLocal reports whether an on-host nft drop is currently installed
// and, if so, the target vector (prefix / proto / port) it covers.
func handleShowDdosLocal(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	r := activeResponder.Load()
	if r == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   plugin.Map{"enabled": false, "active": false},
		}, nil
	}
	active, target := r.status()
	data := plugin.Map{"enabled": true, "active": active}
	if active {
		data["target"] = target
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}
