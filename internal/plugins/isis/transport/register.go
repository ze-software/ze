// Design: docs/architecture/isis/isis-3-l2-transport.md -- doctor-check registration
// Related: doctor.go -- the check function registered here
//
// The transport registers its raw-socket readiness check via
// diagnostic.RegisterDoctorCheck from init(). The transport is a
// platform-specific raw-socket backend, not its own plugin Registration (the
// single IS-IS component Registration is owned by isis-4), so it uses the
// component-style registration path (ai/rules/repo-maintenance.md "Components that
// are not plugins"). This keeps the entire check self-contained under transport/.

package transport

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := diagnostic.DoctorCheck{
		Name:         "isis-raw-socket",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        730,
		Component:    "isis",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-isis-raw-socket"},
		Check:        checkISISRawSocket,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "isis/transport: doctor check registration: %v\n", err)
		os.Exit(2)
	}
}
