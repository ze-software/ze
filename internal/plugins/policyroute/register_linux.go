//go:build linux

// Design: docs/architecture/policyroute/policy-routing.md -- the Linux-only registrations of this plugin
// Related: register.go -- the plugin registration this init() runs beside
// Related: doctor_linux.go -- checkPolicyRouteNetlink, the check registered here
//
// The check opens a NETLINK_ROUTE handle, so it exists on Linux only and
// registers from here rather than from register.go; off Linux nothing
// registers, which is what the doctor runner's stub for those platforms
// reported.

package policyroute

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it and stops
	// the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(policyRouteDoctorCheck); err != nil {
		panic("BUG: policy-routes doctor check registration refused: " + err.Error())
	}
}
