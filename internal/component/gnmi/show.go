// Design: docs/architecture/api/architecture.md -- gNMI show command handler

package gnmi

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:gnmi",
			Handler:    handleShowGNMI,
		},
	)
}

func handleShowGNMI(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	srv := LookupServer()
	if srv == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   ServerStatus{Enabled: false},
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   srv.Status(),
	}, nil
}
