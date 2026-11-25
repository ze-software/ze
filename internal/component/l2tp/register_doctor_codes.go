// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/l2tp.

package l2tp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorL2TPUnavailable    = "doctor-l2tp-unavailable"
	codeDoctorL2TPUnknown        = "doctor-l2tp-unknown"
	codeDoctorL2TPPPPUnavailable = "doctor-l2tp-ppp-unavailable"
	codeDoctorL2TPPPPUnknown     = "doctor-l2tp-ppp-unknown"
	exampleDoctorJSON            = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorL2TPUnavailable,
		Title: "L2TP control netlink socket unavailable",
		Description: "The configuration uses the l2tp subsystem and the kernel does not provide the l2tp generic netlink family (CONFIG_L2TP). " +
			"The probe asks the generic netlink controller for the family, which loads l2tp_netlink on a modular kernel and changes nothing. This is an ERROR: the daemon refuses to start, and a Docker host that runs " +
			"Ze's labs is refused on it too. Remedy: run a kernel built with CONFIG_L2TP, such as Ze's own runtime kernel.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-l2tp-unavailable"},
		RelatedCodes: []string{codeDoctorL2TPUnknown},
	},
	{
		Code:  codeDoctorL2TPUnknown,
		Title: "L2TP control netlink socket support could not be determined",
		Description: "The configuration uses the l2tp subsystem and the probe for the l2tp generic netlink family (CONFIG_L2TP) gave no verdict, " +
			"usually because the process lacks a privilege the probe needs. This is a WARNING for the " +
			"daemon and a refusal for a Docker host that runs Ze's labs. The message carries the reason.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-l2tp-unknown"},
		RelatedCodes: []string{codeDoctorL2TPUnavailable},
	},
	{
		Code:  codeDoctorL2TPPPPUnavailable,
		Title: "L2TP PPP session socket unavailable",
		Description: "The configuration uses the l2tp subsystem and the kernel does not provide the PPPoL2TP socket (CONFIG_PPPOL2TP). " +
			"The probe opens and closes an AF_PPPOX PX_PROTO_OL2TP socket, which loads l2tp_ppp on a modular kernel and changes nothing. This is an ERROR: the daemon refuses to start, and a Docker host that runs " +
			"Ze's labs is refused on it too. Remedy: run a kernel built with CONFIG_PPPOL2TP, such as Ze's own runtime kernel.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-l2tp-ppp-unavailable"},
		RelatedCodes: []string{codeDoctorL2TPPPPUnknown},
	},
	{
		Code:  codeDoctorL2TPPPPUnknown,
		Title: "L2TP PPP session socket support could not be determined",
		Description: "The configuration uses the l2tp subsystem and the probe for the PPPoL2TP socket (CONFIG_PPPOL2TP) gave no verdict, " +
			"usually because the process lacks a privilege the probe needs. This is a WARNING for the " +
			"daemon and a refusal for a Docker host that runs Ze's labs. The message carries the reason.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-l2tp-ppp-unknown"},
		RelatedCodes: []string{codeDoctorL2TPPPPUnavailable},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: l2tp: doctor code registration refused: " + err.Error())
		}
	}
}
