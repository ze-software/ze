// Design: docs/architecture/doctor-and-health-checks.md -- config evolution at start.
// Related: main_evolve.go -- warnEvolutionNotices raises the event this registers.

package hub

import "github.com/ze-software/ze/internal/core/diagnostic"

func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorConfigEvolved}); err != nil {
		panic("BUG: hub: config evolution event registration refused: " + err.Error())
	}
}
