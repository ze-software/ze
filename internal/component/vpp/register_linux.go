//go:build linux

// Design: plan/spec-vpp-host-tuning.md -- linux-only VPP registrations.
// Registers the hugepage readiness doctor check (defined in doctor_linux.go)
// with the diagnostic doctor registry. Linux-tagged because the check reads
// procfs/sysfs; on other platforms nothing registers.

package vpp

import "codeberg.org/thomas-mangin/ze/internal/core/diagnostic"

func init() {
	_ = diagnostic.RegisterDoctorCheck(vppHugepagesDoctorCheck())
}
