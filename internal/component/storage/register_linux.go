//go:build linux

// Design: docs/architecture/storage/smart-health.md -- the Linux-only registrations of this component
// Related: show.go -- the RPC registration this init() runs beside
// Related: doctor_linux.go -- checkSmartEnabled, the check registered here
//
// The check walks /sys/class/block and runs the SMART ioctls, so it exists on
// Linux only and registers from here; off Linux nothing registers, which is
// what the doctor runner's stub for those platforms reported.

package storage

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it and stops
	// the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(smartDoctorCheck); err != nil {
		panic("BUG: storage doctor check registration refused: " + err.Error())
	}
}
