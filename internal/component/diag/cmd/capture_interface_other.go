// Design: plan/spec-diag-capture-interface.md -- platform stub (non-Linux)

//go:build !linux

package cmd

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func HandleCaptureInterface(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "not available on this platform",
	}, nil
}
