//go:build linux

// register_iface_mirror_linux.go names the fixtures that read a port mirror
// back from the kernel's tc state.
//
// Related: iface_mirror_fixture_linux.go -- the drivers, the devices they build and the tc read
// Related: test/reload/tx-iface-mirror-target-delete-refused.ci -- deleting a live mirror target
// Related: test/plugin/iface-mirror-vlan-target-selector.ci -- a VLAN-unit target on a selected parent

package fixture

func init() {
	Register("reload/tx-iface-mirror-target-delete-refused", ifaceMirrorTargetDeleteRefused)
	Register("plugin/iface-mirror-vlan-target-selector-setup", ifaceMirrorVLANTargetSetup)
	Register("plugin/iface-mirror-vlan-target-selector-driver", ifaceMirrorVLANTargetDriver)
}
