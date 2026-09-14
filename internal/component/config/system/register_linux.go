//go:build linux

// Design: docs/features/ai-first.md -- the Linux-only registrations of the system component
// Related: register.go -- the cross-platform table this init() runs beside
// Related: doctor_conntrack_linux.go -- checkConntrackProcfs, the check registered here
//
// The conntrack check probes /proc/sys/net/netfilter with unix.Access, so it
// exists on Linux only and registers from here rather than from the table in
// doctor.go; off Linux nothing registers, which is what the doctor runner's
// stub for those platforms reported.

package system

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it and stops
	// the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(conntrackDoctorCheck); err != nil {
		panic("BUG: system conntrack doctor check registration refused: " + err.Error())
	}
}
