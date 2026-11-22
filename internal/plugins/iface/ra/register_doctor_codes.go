// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/iface/ra.

package ifacera

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorCodeRAForwarding,
		Title:       "Router Advertisements sent while IPv6 forwarding is off",
		Description: "A unit sends Router Advertisements (interface ... unit ... ipv6 router-advertisement enabled true) while net.ipv6.conf.<device>.forwarding is 0. Hosts on that link autoconfigure and install a default route through Ze, and the kernel then drops the off-link traffic they send, so the link looks configured and carries nothing. Set ipv6 forwarding true on the advertising unit, or set the sysctl through a profile. Ze reports this state and never changes it, because a kernel change outside declared config would hide the real setting. Advertising with forwarding off is deliberate in one case: router-lifetime 0, which says Ze is not a default router and advertises prefixes only.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-iface-ra-forwarding"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ifacera: doctor code registration refused: " + err.Error())
		}
	}
}
