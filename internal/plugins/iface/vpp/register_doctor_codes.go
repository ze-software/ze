// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/iface/vpp.

package ifacevpp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorVPPLCPNetns  = "doctor-vpp-lcp-netns"
	codeDoctorVPPLCPPlugin = "doctor-vpp-lcp-plugin"
	codeDoctorVPPWireguard = "doctor-vpp-wireguard"
	exampleDoctorJSON      = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorVPPLCPNetns,
		Title:       "LCP netns will not carry the TAPs ze needs",
		Description: "vpp.lcp.netns carries a value, so VPP resolves it as a network namespace NAME and opens /var/run/netns/<name> for the Linux Control Plane TAPs. ze also passes the same leaf as VPP's global default netns. Three outcomes follow, and this code reports whichever applies. The TAPs land outside the namespace ze runs in, so BGP cannot bind on an LCP-shadowed interface: ze has no netns-aware listener, and netns-aware BGP binding is specced but not implemented. ze's own markers host and root are not exempt, because to VPP they are ordinary names: netns=host asks for a namespace literally called host. And when the namespace is absent from this host, LCP pair creation fails at apply with a raw VPP error; a probe that cannot answer is reported as a probe failure and never as absence. Remedy: leave vpp.lcp.netns empty, which clears VPP's global default and leaves the TAPs in VPP's own namespace where ze runs, or run ze in the namespace the leaf names so BGP binds where the TAPs are.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-vpp-lcp-netns"},
	},
	{
		Code:        codeDoctorVPPLCPPlugin,
		Title:       "VPP linux_cp plugin not loaded",
		Description: "vpp.lcp is enabled, so ze writes a startup.conf that enables linux_cp_plugin.so, but the running VPP does not report that plugin as loaded. The linux_cp API is therefore unavailable and the config apply fails at the binapi layer with a raw VPP error that names the failing message rather than the missing plugin. Remedy: run a VPP build that ships linux_cp_plugin.so (and linux_nl_plugin.so), or disable vpp.lcp. When the probe gives no usable answer -- vppctl missing, VPP socket absent, VPP wedged, or a zero exit whose output carries no plugin listing -- this is reported as a WARNING instead, because none of those is evidence about which plugins VPP loaded.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-vpp-lcp-plugin"},
	},
	{
		Code:        codeVPPUnreachable,
		Title:       "VPP socket unreachable",
		Description: "The VPP API socket could not be reached. VPP may not be running.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeVPPVersion,
		Title:       "VPP version check failed",
		Description: "The VPP version could not be determined or is not compatible with the expected API.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:        codeDoctorVPPWireguard,
		Title:       "VPP wireguard plugin not enabled",
		Description: "A wireguard interface is configured under the vpp backend, but the VPP wireguard plugin is not enabled (vpp.plugins.wireguard) or not loaded in the running VPP. VPP cannot program wireguard tunnels without it, so the interface will fail at apply.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-vpp-wireguard"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ifacevpp: doctor code registration refused: " + err.Error())
		}
	}
}
