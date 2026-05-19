// Design: plan/spec-diag-capture-interface.md -- platform stub (non-Linux)

//go:build !linux

package show

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:capture-interface",
			Handler:    handleCaptureInterface,
		},
	)
}

func handleCaptureInterface(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusError,
		Data:   "not available on this platform",
	}, nil
}
