// Design: plan/learned/631-host-0-inventory.md — non-Linux stub for FD limit adjustment

//go:build !linux

package cmd

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func RegisterSetFD() {
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
