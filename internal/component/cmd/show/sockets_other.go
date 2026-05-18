// Design: plan/spec-diag-core.md -- non-Linux stub for socket state
// Overview: sockets_linux.go -- Linux implementation
//
//go:build !linux

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-sockets", Handler: handleShowSystemSockets},
	)
}

func handleShowSystemSockets(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Data: "not available on this platform"}, nil
}
