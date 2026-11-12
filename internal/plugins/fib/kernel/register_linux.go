//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the Linux-only registrations of this plugin
// Related: register.go -- the plugin registration this init() runs beside
// Related: doctor_linux.go -- checkKernelNexthop, a check registered here
// Related: doctor_mplsttl_linux.go -- checkMPLSTTLPropagate, a check registered here
//
// Both checks read procfs, so they exist on Linux only and register
// from here rather than from register.go; off Linux nothing registers, which
// is what the doctor runner's stub for those platforms reported.

package fibkernel

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it and stops
	// the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(kernelNexthopDoctorCheck); err != nil {
		panic("BUG: fib-kernel doctor check registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(mplsTTLPropagateDoctorCheck); err != nil {
		panic("BUG: fib-kernel doctor check registration refused: " + err.Error())
	}
}
