// Design: docs/architecture/diagnostics/active-probes.md -- doctor check registration
// Related: doctor.go -- the check this file registers

package probe

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := diagnostic.DoctorCheck{
		Name:         doctorCheckName,
		Phase:        diagnostic.DoctorPhasePreConfig,
		Order:        760,
		Component:    "probe",
		Dependencies: []string{"capabilities"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeICMPProbeUnavailable, codeICMPProbeUnprivileged},
		Check:        checkICMPProbeSocket,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "probe: doctor check registration: %v\n", err)
		os.Exit(2)
	}
}
