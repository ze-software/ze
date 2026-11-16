// Design: docs/architecture/host/inventory.md -- request system storage grow
// Related: storage_grow_other.go -- the refusal on a system that is not Linux

//go:build linux

package cmd

import (
	"errors"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

// errStorageGrowNotWired is what the handler wraps until the device-surface
// phase writes it: the partition-table and filesystem leaves it will call are
// written, the handler's reading of the mounted device is not.
var errStorageGrowNotWired = errors.New("the storage grow command does not call the growth leaves yet")

func registerStorageGrow() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-host:request-system-storage-grow",
			Handler:    handleStorageGrow,
		},
	)
}

// handleStorageGrow answers `request system storage grow`.
//
// It is a wiring stub: it reads no disk, writes nothing and refuses with
// errStorageGrowNotWired. The device-surface phase of
// plan/post-release/features/spec-appliance-perm-grows-at-install.md replaces
// it with the growth of the mounted persistent partition (gptdisk.Grow,
// gptdisk.ResizeInKernel, ext4fs.Resize) and its refusals.
func handleStorageGrow(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return plugin.OperatorErrorResponse(opererr.Wrap(errStorageGrowNotWired,
		"request system storage grow cannot grow the persistent storage in this version of Ze",
		"the command is registered and its work is not written yet, so no disk and no filesystem was changed",
		"show host storage")), nil
}
