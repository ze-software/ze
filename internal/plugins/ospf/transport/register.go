// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- doctor-check registration

package transport

import "github.com/ze-software/ze/internal/core/diagnostic"

func init() {
	check := diagnostic.DoctorCheck{
		Name:         "ospf-raw-socket",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        735,
		Component:    "ospf",
		Dependencies: []string{"external-binary"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeDoctorOSPFRawSocket},
		Check:        checkOSPFRawSocket,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		panic("BUG: ospf/transport: doctor check registration: " + err.Error())
	}
}
