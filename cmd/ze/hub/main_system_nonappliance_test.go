// Design: docs/guide/configuration.md -- System Host Name: a distro keeps its own host name
// Related: main_system_hostname.go -- reconcileKernelHostname

//go:build !ze_appliance

package hub

import "testing"

// TestHostnameApplierNonApplianceNeverSets drives the applier of a build
// without ze_appliance with system host set and differing from the kernel.
// VALIDATES: AC-10 -- the distro's host name is preserved: sethostname is never
// called, and the kernel name is not even read.
// PREVENTS: Ze taking over a host name systemd-hostnamed owns.
func TestHostnameApplierNonApplianceNeverSets(t *testing.T) {
	log, _ := captureLog()
	kernel := &fakeKernel{name: "distro-box"}

	reconcileKernelHostname("cpe-lab", true, kernel.ops(), log)

	if len(kernel.calls) != 0 || kernel.gets != 0 || kernel.name != "distro-box" {
		t.Fatalf("non-appliance build touched the kernel: gets %d, calls %v, name %q",
			kernel.gets, kernel.calls, kernel.name)
	}
}
