// Design: docs/architecture/doctor-and-health-checks.md -- every registered check, which the doctor-coverage gate's source read is held to
// Related: doctor_registry.go -- the registry this file lists

package diagnostic

import (
	"slices"
	"strings"
)

// DoctorChecks returns every registered check, whatever its phase, sorted by
// name. It reads the registry rather than walking a list of phases, so a
// phase added to the one phase declaration (pkg/plugin/rpc DoctorCheckPhase)
// is listed with no edit here. The doctor-coverage gate
// (internal/le/arch/doctorcoverage) reads the covered codes from the source,
// and its test holds that read to the codes each check listed here declares.
// Each entry is a copy: a caller cannot change what the registry holds.
//
// Safe for concurrent use.
func DoctorChecks() []DoctorCheck {
	doctorCheckRegistry.Lock()
	defer doctorCheckRegistry.Unlock()
	checks := make([]DoctorCheck, 0, len(doctorCheckRegistry.entries))
	for i := range doctorCheckRegistry.entries {
		checks = append(checks, cloneDoctorCheckReg(doctorCheckRegistry.entries[i]))
	}
	slices.SortFunc(checks, func(a, b DoctorCheck) int { return strings.Compare(a.Name, b.Name) })
	return checks
}
