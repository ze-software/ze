// Design: plan/spec-diag-core.md — non-Linux stub for kernel log

//go:build !linux

package cmd

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func RegisterShowKernelLog() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-kernel-log", Handler: handleShowSystemKernelLog},
	)
}

func handleShowSystemKernelLog(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: "not available on this platform"}, nil
}
