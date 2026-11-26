// Design: docs/architecture/core-design.md -- a service whose configuration cannot be served is not started, and never stops ze
//
// Registration of the runtime doctor check that reports each service this
// daemon's start left down. Always on: the record it reads is always-on hub
// state, and a build without a service never records it.
//
// Related: service_not_started.go -- the record, the report and the check function

package hub

import "github.com/ze-software/ze/internal/core/diagnostic"

// servicesNotStartedOrder prints these rows before the environment values and
// the recent events of the runtime phase: a service that is down is the first
// thing an operator who cannot reach it needs to read.
const servicesNotStartedOrder = 2850

func init() {
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "hub-services-not-started",
		Phase:        diagnostic.DoctorPhaseRuntime,
		Order:        servicesNotStartedOrder,
		Component:    "hub",
		Dependencies: []string{"service-configuration"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeDoctorServiceNotStarted},
		Check:        checkServicesNotStarted,
	}); err != nil {
		panic("BUG: hub: services-not-started doctor registration refused: " + err.Error())
	}
}
