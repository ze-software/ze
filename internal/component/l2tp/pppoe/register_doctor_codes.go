// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/l2tp/pppoe.

package pppoe

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorPPPoEUnavailable = "doctor-pppoe-unavailable"
	codeDoctorPPPoEUnknown     = "doctor-pppoe-unknown"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorPPPoEUnavailable,
		Title: "PPPoE session socket unavailable",
		Description: "The configuration uses the pppoe subsystem and the kernel does not provide the PPPoE socket (CONFIG_PPPOE). " +
			"The probe opens and closes an AF_PPPOX PX_PROTO_OE socket, which loads pppoe on a modular kernel and changes nothing. This is an ERROR: the daemon refuses to start, and a Docker host that runs " +
			"Ze's labs is refused on it too. Remedy: run a kernel built with CONFIG_PPPOE, such as Ze's own runtime kernel.",
		Examples:     []string{"ze doctor --json", "ze show diagnostic doctor-pppoe-unavailable"},
		RelatedCodes: []string{codeDoctorPPPoEUnknown},
	},
	{
		Code:  codeDoctorPPPoEUnknown,
		Title: "PPPoE session socket support could not be determined",
		Description: "The configuration uses the pppoe subsystem and the probe for the PPPoE socket (CONFIG_PPPOE) gave no verdict, " +
			"usually because the process lacks a privilege the probe needs. This is a WARNING for the " +
			"daemon and a refusal for a Docker host that runs Ze's labs. The message carries the reason.",
		Examples:     []string{"ze doctor --json", "ze show diagnostic doctor-pppoe-unknown"},
		RelatedCodes: []string{codeDoctorPPPoEUnavailable},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: pppoe: doctor code registration refused: " + err.Error())
		}
	}
}
