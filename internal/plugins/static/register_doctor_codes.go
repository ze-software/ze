// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/static.

package static

import "github.com/ze-software/ze/internal/core/diagnostic"

const exampleDoctorJSON = "ze doctor --json"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorCodeInterfaceNexthopNoBackend,
		Title:       "Static interface next-hop has no iface backend",
		Description: "A static route forwards over an interface-only next-hop (no gateway address) but the config declares no `interface { backend ... }` stanza. The next-hop interface name cannot be resolved to an ifindex at runtime, so the static section fails to load. Add an interface backend, or give the next-hop a gateway address.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-static-interface-nexthop-no-backend"},
	},
	{
		Code:        doctorCodeRouteSkipped,
		Title:       "Static route skipped (rest of section kept programmed)",
		Description: "The running static plugin could not program one or more routes (an absent device, an absent interface backend, or an unresolvable next-hop) and skipped them so the rest of the static section stayed programmed. A skipped prefix is unrouted until its dependency appears; it is re-attempted on the next config apply and clears once it programs. Run `static show` to see which prefixes are skipped and why, then fix the offending next-hop or create the missing device or interface backend.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-static-route-skipped"},
	},
	{
		Code:        doctorCodeVPPChannel,
		Title:       "Static named-table route cannot open VPP channel",
		Description: "The configuration selects `interface { backend vpp; }`, so named-table static routes must be programmed into VPP and never fall back to the Linux kernel. Opening a GoVPP API channel failed (VPP is not connected, no VPP connector is active in this process, or the binary was built without `ze_vpp`). Main-table static routes continue to reach the FIB through the Loc-RIB. Check VPP readiness with `ze show doctor` and `ze show errors`, then re-apply once VPP is connected.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-static-vpp-channel"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: static: doctor code registration refused: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: doctorCodeVPPChannel}); err != nil {
		panic("BUG: static: event code registration refused: " + err.Error())
	}
}
