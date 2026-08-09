// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- doctor-check registration

package transport

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := diagnostic.DoctorCheck{
		Name:         "ospf-raw-socket",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        735,
		Component:    "ospf",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-ospf-raw-socket"},
		Check:        checkOSPFRawSocket,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "ospf/transport: doctor check registration: %v\n", err)
		os.Exit(2)
	}
}
