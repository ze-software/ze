// Design: docs/guide/configuration.md -- System Host Name: appliance-only kernel effect
// Related: doctor_hostname.go -- checkKernelHostname, the check function registered here

//go:build ze_appliance

package system

import "github.com/ze-software/ze/internal/core/diagnostic"

// hostnameDoctorCheck is declared and registered only by the appliance build:
// on a distro the operating system owns the host name and a different one is
// expected.
var hostnameDoctorCheck = diagnostic.DoctorCheck{
	// Runs after the resolv.conf path check (2290, doctor.go).
	Name:         "system-host-name",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2295,
	Component:    doctorComponentSystem,
	Dependencies: []string{doctorDependencyConfigTree},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeSystemHostNotApplied},
	Check:        checkKernelHostname,
}

// Only the appliance build sets the kernel host name from system host
// (cmd/ze/hub/main_system_appliance.go), so only it reports a mismatch.
func init() {
	if err := diagnostic.RegisterDoctorCheck(hostnameDoctorCheck); err != nil {
		panic("BUG: system host name doctor check registration refused: " + err.Error())
	}
}
