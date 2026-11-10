// Design: docs/architecture/doctor-and-health-checks.md -- doctor check registration for the gNMI transport
// Related: doctor.go -- checkGNMITLSMaterial, the check registered here
//
// gNMI is not a plugin, so it registers through the component path
// (diagnostic.RegisterDoctorCheck). The package is linked only through the
// ze_gnmi blank import in internal/component/plugin/all, so a build without
// gNMI drops the check with the transport.

package gnmi

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it -- a
	// duplicate name, a phase that does not exist, a code without the doctor
	// prefix -- and none of them can be reached from a config or a peer, so it
	// stops the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(gnmiTLSDoctorCheck); err != nil {
		panic("BUG: gnmi doctor check registration refused: " + err.Error())
	}
}
