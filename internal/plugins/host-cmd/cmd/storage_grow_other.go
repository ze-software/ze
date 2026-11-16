// Design: docs/architecture/host/inventory.md -- request system storage grow off Linux
// Related: storage_grow_linux.go -- the handler this file stands in for

//go:build !linux

package cmd

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

func registerStorageGrow() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-host:request-system-storage-grow",
			Handler:    handleStorageGrow,
		},
	)
}

// handleStorageGrow refuses `request system storage grow`: an appliance runs
// Linux, so a system that does not is never one.
func handleStorageGrow(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return plugin.OperatorErrorResponse(opererr.New(
		"cannot grow the persistent storage: this system is not a Ze appliance",
		"only an appliance has a persistent partition that Ze manages, and an appliance runs Linux",
		"show host storage")), nil
}
