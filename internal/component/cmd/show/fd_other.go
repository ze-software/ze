// Design: plan/learned/727-diag-core.md -- non-Linux stub for FD inspection
// Overview: fd_linux.go -- Linux implementation
//
//go:build !linux

package show

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-file-descriptors", Handler: handleShowSystemFD},
	)
}

func handleShowSystemFD(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: "not available on this platform"}, nil
}
