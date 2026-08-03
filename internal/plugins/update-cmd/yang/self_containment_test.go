package yang

import (
	"strings"
	"testing"
)

// TestUpdateShowCmdSchemaOwnsSystemUpdate is the owner half of the
// self-containment invariant: the central show schema must NOT declare
// show system update commands, and this package MUST.
// See ai/rules/plugins.md.
func TestUpdateShowCmdSchemaOwnsSystemUpdate(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:system-update"`,
		`ze:command "ze-show:system-update-history"`,
		"container update",
		"container history",
	} {
		if !strings.Contains(ZeUpdateShowCmdYANG, want) {
			t.Errorf("ze-update-show-cmd.yang must declare %q so removing the update component removes the surface", want)
		}
	}
}

// TestUpdateFirmwareCmdSchemaOwnsFirmware is the owner half of the
// self-containment invariant: the central update schema must NOT declare
// firmware commands, and this package MUST.
// See ai/rules/plugins.md.
func TestUpdateFirmwareCmdSchemaOwnsFirmware(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-update:system-firmware-check"`,
		`ze:command "ze-update:system-firmware-download"`,
		`ze:command "ze-update:system-firmware-apply"`,
		`ze:command "ze-update:system-firmware-restart"`,
		`ze:command "ze-update:system-firmware-rollback"`,
		"container firmware",
		"container check",
		"container download",
		"container apply",
		"container restart",
		"container rollback",
	} {
		if !strings.Contains(ZeUpdateFirmwareCmdYANG, want) {
			t.Errorf("ze-update-firmware-cmd.yang must declare %q so removing the update component removes the surface", want)
		}
	}
}
