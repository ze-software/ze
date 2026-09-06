//go:build linux

// Design: docs/architecture/vpp-host-tuning.md -- linux-only VPP registrations.
// Registers the hugepage readiness doctor check (doctor_linux.go) and the CPU
// isolation check (doctor_cpu_linux.go) with the diagnostic doctor registry.
// Linux-tagged because both checks read procfs/sysfs; on other platforms
// nothing registers.

package vpp

import "github.com/ze-software/ze/internal/core/diagnostic"

func init() {
	_ = diagnostic.RegisterDoctorCheck(vppHugepagesDoctorCheck())
	_ = diagnostic.RegisterDoctorCheck(vppCPUIsolationDoctorCheck())
}
