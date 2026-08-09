// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- non-Linux stub for show system memory (OS view)
// Overview: memory_map_linux.go -- Linux implementation
//
//go:build !linux

package show

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-memory-map", Handler: handleShowSystemMemoryMap},
	)
}

func handleShowSystemMemoryMap(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: "not available on this platform"}, nil
}
