// Design: docs/architecture/diagnostics/procfs-diagnostics.md -- non-Linux stub for FD inspection
// Overview: fd_linux.go -- Linux implementation
//
//go:build !linux

package show

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// msgPlatformUnsupported is what every non-Linux stub in this package returns.
// It is declared here because the three stub files share one build constraint.
const msgPlatformUnsupported = "not available on this platform"

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:system-file-descriptors", Handler: handleShowSystemFD},
	)
}

func handleShowSystemFD(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{Status: plugin.StatusError, Error: msgPlatformUnsupported}, nil
}
