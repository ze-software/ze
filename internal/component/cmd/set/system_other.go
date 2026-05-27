// Design: (none -- new runtime FD limit adjustment)

//go:build !linux

package set

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-set:system-file-descriptors",
			Handler:    handleSetSystemFD,
		},
	)
}

func handleSetSystemFD(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "set system file-descriptors is only supported on Linux",
	}, nil
}
