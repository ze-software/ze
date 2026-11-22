// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/static.

package static

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorCodeInterfaceNexthopNoBackend,
		Title:       "Static interface next-hop has no iface backend",
		Description: "A static route forwards over an interface-only next-hop (no gateway address) but the config declares no `interface { backend ... }` stanza. The next-hop interface name cannot be resolved to an ifindex at runtime, so the static section fails to load. Add an interface backend, or give the next-hop a gateway address.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-static-interface-nexthop-no-backend"},
	},
	{
		Code:        doctorCodeRouteSkipped,
		Title:       "Static route skipped (rest of section kept programmed)",
		Description: "The running static plugin could not program one or more routes (an absent device, an absent interface backend, or an unresolvable next-hop) and skipped them so the rest of the static section stayed programmed. A skipped prefix is unrouted until its dependency appears; it is re-attempted on the next config apply and clears once it programs. Run `static show` to see which prefixes are skipped and why, then fix the offending next-hop or create the missing device or interface backend.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-static-route-skipped"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: static: doctor code registration refused: " + err.Error())
		}
	}
}
