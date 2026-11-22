// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/iface/netlink.

package ifacenetlink

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorWireGuardUnavailable     = "doctor-wireguard-unavailable"
	codeDoctorWireGuardUnknown         = "doctor-wireguard-unknown"
	codeDoctorXFRMInterfaceUnavailable = "doctor-xfrm-interface-unavailable"
	codeDoctorXFRMInterfaceUnknown     = "doctor-xfrm-interface-unknown"
	exampleDoctorJSON                  = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeIfaceMacvlan,
		Title:       "Kernel macvlan support unavailable",
		Description: "The kernel cannot create a bridge-mode macvlan device (CONFIG_MACVLAN). ze's owned-device mechanism -- plugin-requested macvlan devices on a parent interface -- needs it; without CONFIG_MACVLAN those devices fail at apply. Enable CONFIG_MACVLAN or load the macvlan module. If the probe instead reports a permission failure, it lacked CAP_NET_ADMIN.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-iface-macvlan"},
	},
	{
		Code:  codeDoctorWireGuardUnavailable,
		Title: "WireGuard unavailable",
		Description: "The configuration uses a wireguard interface and the kernel does not provide the wireguard generic netlink family (CONFIG_WIREGUARD). " +
			"The probe asks the generic netlink controller for the family, which loads wireguard on a modular kernel and changes nothing. This is an ERROR: the daemon refuses to start, and a Docker host that runs " +
			"Ze's labs is refused on it too. Remedy: run a kernel built with CONFIG_WIREGUARD, such as Ze's own runtime kernel.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-wireguard-unavailable"},
		RelatedCodes: []string{codeDoctorWireGuardUnknown},
	},
	{
		Code:  codeDoctorWireGuardUnknown,
		Title: "WireGuard support could not be determined",
		Description: "The configuration uses a wireguard interface and the probe for the wireguard generic netlink family (CONFIG_WIREGUARD) gave no verdict, " +
			"usually because the process lacks a privilege the probe needs. This is a WARNING for the " +
			"daemon and a refusal for a Docker host that runs Ze's labs. The message carries the reason.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-wireguard-unknown"},
		RelatedCodes: []string{codeDoctorWireGuardUnavailable},
	},
	{
		Code:  codeDoctorXFRMInterfaceUnavailable,
		Title: "XFRM interface unavailable",
		Description: "The configuration uses an xfrm interface and the kernel does not provide the xfrm link kind (CONFIG_XFRM_INTERFACE). " +
			"The probe asks for an xfrm link with interface id 0 inside a throwaway network namespace, which the kernel refuses after finding the link kind; it needs CAP_SYS_ADMIN to create that namespace. This is an ERROR: the daemon refuses to start, and a Docker host that runs " +
			"Ze's labs is refused on it too. Remedy: run a kernel built with CONFIG_XFRM_INTERFACE, such as Ze's own runtime kernel.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-xfrm-interface-unavailable"},
		RelatedCodes: []string{codeDoctorXFRMInterfaceUnknown},
	},
	{
		Code:  codeDoctorXFRMInterfaceUnknown,
		Title: "XFRM interface support could not be determined",
		Description: "The configuration uses an xfrm interface and the probe for the xfrm link kind (CONFIG_XFRM_INTERFACE) gave no verdict, " +
			"usually because the process lacks a privilege the probe needs. This is a WARNING for the " +
			"daemon and a refusal for a Docker host that runs Ze's labs. The message carries the reason.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-xfrm-interface-unknown"},
		RelatedCodes: []string{codeDoctorXFRMInterfaceUnavailable},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ifacenetlink: doctor code registration refused: " + err.Error())
		}
	}
}
