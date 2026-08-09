// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- non-Linux stub for kernel log

//go:build !linux

package cmd

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func RegisterShowKernelLog() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-kernel-log", Handler: handleShowSystemKernelLog},
	)
}

func handleShowSystemKernelLog(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: "not available on this platform"}, nil
}
