// Design: docs/features/ai-first.md -- doctor check registration for the config component
// Related: doctor_redistribute.go -- checkRedistributeRules, the check registered here
//
// The config component is not a plugin, so it registers through the component
// path (diagnostic.RegisterDoctorCheck) rather than a plugin Registration's
// DoctorChecks field. The one check it owns judges the `redistribute` root
// against the registries its own loader reads, so it lives beside that loader.

package config

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it -- a
	// duplicate name, a phase that does not exist, a code without the doctor
	// prefix -- and none of them can be reached from a config or a peer, so it
	// stops the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(redistributeDoctorCheck); err != nil {
		panic("BUG: config doctor check registration refused: " + err.Error())
	}
}
