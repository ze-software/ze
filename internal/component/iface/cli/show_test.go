package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ifacepkg "github.com/ze-software/ze/internal/component/iface"
)

// TestFormatInterfaceDetail verifies the detail formatter renders the os-name
// line and the permanent-MAC line as distinct from the operational MAC. This
// covers the render path the QEMU functional test cannot exercise on loopback,
// which has no permanent address.
//
// VALIDATES: spec-iface-resolve-1-model AC-2 -- os-name + permanent MAC visible.
func TestFormatInterfaceDetail(t *testing.T) {
	out := formatInterfaceDetail(&ifacepkg.InterfaceInfo{
		Name:         "eth0",
		OsName:       "eth0",
		Index:        3,
		State:        "up",
		MTU:          1500,
		MAC:          "02:00:00:00:00:01",
		PermanentMAC: "aa:bb:cc:dd:ee:ff",
	})
	assert.Contains(t, out, "OS Name:    eth0")
	assert.Contains(t, out, "MAC:        02:00:00:00:00:01")
	assert.Contains(t, out, "Perm MAC:   aa:bb:cc:dd:ee:ff")
}

// TestFormatInterfaceDetailNoPermMAC verifies a virtual interface with no
// permanent address omits the Perm MAC line (no empty field shown).
func TestFormatInterfaceDetailNoPermMAC(t *testing.T) {
	out := formatInterfaceDetail(&ifacepkg.InterfaceInfo{
		Name: "veth0", OsName: "veth0", Index: 9, State: "down", MTU: 1500,
	})
	assert.Contains(t, out, "OS Name:    veth0")
	assert.NotContains(t, out, "Perm MAC:")
}
