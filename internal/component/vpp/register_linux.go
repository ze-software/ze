//go:build linux

// Design: docs/architecture/vpp-host-tuning.md -- linux-only VPP registrations.
// Registers the hugepage readiness doctor check (doctor_linux.go), the CPU
// isolation check (doctor_cpu_linux.go) and the DPDK bind check
// (doctor_dpdk_linux.go) with the diagnostic doctor registry. Linux-tagged
// because all three read procfs/sysfs; on other platforms nothing registers.

package vpp

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// vppLinuxDoctorChecks are the checks this component owns on Linux. The
// registry-reach test reads the same table.
func vppLinuxDoctorChecks() []diagnostic.DoctorCheck {
	return []diagnostic.DoctorCheck{
		vppHugepagesDoctorCheck(),
		vppCPUIsolationDoctorCheck(),
		vppDPDKDoctorCheck(),
	}
}

func init() {
	// A refusal is a programmer error in the table beside it -- a duplicate
	// name, a phase that does not exist, a code without the doctor prefix --
	// and none of them can be reached from a config, so it stops the process
	// rather than leaving `ze doctor` quietly short of a check.
	for _, check := range vppLinuxDoctorChecks() {
		if err := diagnostic.RegisterDoctorCheck(check); err != nil {
			fmt.Fprintf(os.Stderr, "vpp: doctor check registration failed: %v\n", err)
			os.Exit(1)
		}
	}
}
