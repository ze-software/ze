//go:build linux

// Design: docs/architecture/core-design.md -- the Linux-only registrations of the nft backend
// Related: register.go -- the backend registration this init() runs beside
// Related: doctor_linux.go -- checkFirewallNftables, the check registered here
//
// The check reads the kernel's module list, so it exists on Linux only and
// registers from here rather than from register.go; off Linux nothing
// registers, which is what the doctor runner's stub for those platforms
// reported.

package firewallnft

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it and stops
	// the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(nftDoctorCheck); err != nil {
		panic("BUG: firewallnft doctor check registration refused: " + err.Error())
	}
}
