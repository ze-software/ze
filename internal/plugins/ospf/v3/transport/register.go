// Design: plan/learned/970-ospfv3-3-ipv6-transport.md -- doctor-check registration

package transport

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	check := diagnostic.DoctorCheck{
		Name:         "ospfv3-raw-socket",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        737, // just after the OSPFv2 raw-socket checks (735/736)
		Component:    "ospfv3",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{"doctor-ospfv3-raw-socket"},
		Check:        checkOSPFv3RawSocket,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "ospfv3/transport: doctor check registration: %v\n", err)
		os.Exit(2)
	}
}
