// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/mrt.

package mrt

import "github.com/ze-software/ze/internal/core/diagnostic"

// codeMRTGeoDump names an RFC 6397 GEO_PEER_TABLE build, encode, or write failure.
const codeMRTGeoDump = "doctor-mrt-geo-dump"

func registerMRTDoctorChecks() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeMRTGeoDump,
		Title:       "MRT GEO_PEER_TABLE dump failed",
		Description: "An RFC 6397 TABLE_DUMP_V2 GEO_PEER_TABLE record could not be built, encoded, or written during an MRT RIB dump.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-mrt-geo-dump"},
	}); err != nil {
		panic("BUG: mrt: doctor code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name:         "mrt-geo-dump",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        2250,
		Component:    "mrt",
		Dependencies: []string{"config-loaded"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeMRTGeoDump},
		Check: func(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			return nil
		},
	}); err != nil {
		panic("BUG: mrt: doctor check registration refused: " + err.Error())
	}
}
