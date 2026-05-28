// Design: docs/architecture/api/architecture.md -- gNMI show command handler

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/gnmi"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func handleShowGNMI(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	srv := gnmi.LookupServer()
	if srv == nil {
		return &plugin.Response{
			Status: plugin.StatusDone,
			Data:   gnmi.ServerStatus{Enabled: false},
		}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   srv.Status(),
	}, nil
}
