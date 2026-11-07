// Design: docs/guide/configuration.md -- System Host Name: appliance-only kernel effect
// Related: doctor_hostname.go -- the check registered here

//go:build ze_appliance

package system

import "github.com/ze-software/ze/internal/core/diagnostic"

// Only the appliance build sets the kernel host name from system host
// (cmd/ze/hub/main_system_appliance.go), so only it reports a mismatch.
func init() {
	if err := diagnostic.RegisterDoctorCheck(hostnameDoctorCheck); err != nil {
		panic("BUG: system host name doctor check registration refused: " + err.Error())
	}
}
