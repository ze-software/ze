//go:build linux

// Design: docs/architecture/iface/logical-name-resolution.md -- the Linux-only registrations of this component
// Related: register.go -- the component registration this init() runs beside
// Related: doctor_linux.go -- checkEthernetInterfaces, the check registered here
//
// The ethernet check reads sysfs, so it exists on Linux only and registers
// from here rather than from register.go; off Linux nothing registers, which
// is what the doctor runner's stub for those platforms reported.

package iface

import "github.com/ze-software/ze/internal/core/diagnostic"

func init() {
	// A refusal is a programmer error in the declaration beside it and stops
	// the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(ethernetDoctorCheck); err != nil {
		panic("BUG: iface doctor check registration refused: " + err.Error())
	}
}
