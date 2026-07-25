// Design: plan/learned/730-diag-capture-interface.md -- platform stub (non-Linux)

//go:build !linux

package cmd

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func HandleCaptureInterface(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusError,
		Error:  "not available on this platform",
	}, nil
}
