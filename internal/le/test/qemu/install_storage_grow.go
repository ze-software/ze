// Design: docs/architecture/testing/qemu-integration.md -- the appliance storage growth proof
// Overview: actions.go -- the table that registers this action

package testqemu

import (
	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// runApplianceStorageHere is the `./le test qemu appliance-storage` action:
// an appliance image copied to a larger disk boots, `ze doctor` reports the
// storage smaller than its disk, `request system storage grow` grows it, and
// the grown state survives a reboot.
//
// It is a wiring stub: it builds nothing, boots nothing and fails with exit
// code 1. The device-surface phase of
// plan/post-release/features/spec-appliance-perm-grows-at-install.md replaces
// it with the proof.
func runApplianceStorageHere() (any, int) {
	leaction.ReportError(opererr.New(
		"the appliance storage proof is not written yet, so it proved nothing",
		"the action is registered and its guest steps are not: no image was built and no guest was booted, and this run is a failure, not a pass",
		"./le test qemu installer"))
	return nil, 1
}
