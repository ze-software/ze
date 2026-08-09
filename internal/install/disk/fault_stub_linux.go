// Design: docs/architecture/appliance/installer-initrd.md -- R-6 fault injection compiled out of shipping initrd

//go:build linux && ze_installer && !ze_installer_fault

package disk

// maybeInjectFault is a no-op in the shipping installer initrd. The fault
// hook (fault_linux.go) is compiled in only under the ze_installer_fault build
// tag, used by the QEMU fault-injection evidence harness; the production build
// carries no fault-injection code path at all.
func maybeInjectFault(_ InstallConfig) {}
