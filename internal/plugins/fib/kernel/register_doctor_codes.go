// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/fib/kernel.

package fibkernel

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorMPLSTransitMTUUnenforced = "doctor-mpls-transit-mtu-unenforced"
	codeDoctorMPLSTransitMTUUnknown    = "doctor-mpls-transit-mtu-unknown"
	codeDoctorMPLSTTLPropagateOff      = "doctor-mpls-ttl-propagate-off"
	codeDoctorMPLSTTLPropagateUnknown  = "doctor-mpls-ttl-propagate-unknown"
	codeDoctorMPLSUnavailable          = "doctor-mpls-unavailable"
	codeDoctorMPLSUnknown              = "doctor-mpls-unknown"
	exampleDoctorJSON                  = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeKernelNexthop,
		Title:       "Kernel nexthop objects unavailable",
		Description: "The kernel does not support nexthop objects (requires Linux 5.3+). ECMP will fall back to multipath routes.",
		Examples:    []string{exampleDoctorJSON},
	},
	{
		Code:         codeDoctorMPLSTransitMTUUnenforced,
		Title:        "MPLS transit path MTU not enforced",
		Description:  "RSVP-TE runs on the kernel FIB, and the kernel refuses a path MTU (RTA_METRICS/RTAX_MTU) on an AF_MPLS route (Linux limitation LL-MPLS-FORWARD-NO-PMTU, docs/architecture/linux-limitations.md): it is an upstream kernel without CONFIG_MPLS_IP_MTU, which ze's own runtime kernel adds with gokrazy/kernel/patches/0002-mpls-ip-mtu.patch. ze still installs every transit swap and pop, WITHOUT the downstream path MTU the ADSPEC carried, so a transit LSP comes up and forwards, but an oversized labeled packet is dropped by the kernel's MPLS forwarding instead of being fragmented or answered with ICMP Fragmentation Needed or Packet Too Big. Push routes keep their MTU. This is a WARNING and it does not refuse a start: a stock kernel forwards correctly within the device MTU. Remedy: run ze's appliance kernel, or a kernel built with that patch, when the LSP path MTU must be enforced at this transit router.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-mpls-transit-mtu-unenforced"},
		RelatedCodes: []string{codeDoctorMPLSTransitMTUUnknown, codeDoctorMPLSUnavailable},
	},
	{
		Code:         codeDoctorMPLSTransitMTUUnknown,
		Title:        "MPLS transit path MTU support unknown",
		Description:  "ze could not tell whether the kernel accepts a path MTU on an AF_MPLS route. The probe sends two non-creating route requests for label 16 and needs CAP_NET_ADMIN and a non-empty MPLS label space (/proc/sys/net/mpls/platform_labels); an unprivileged `ze doctor`, or one run before the daemon enabled the label space, cannot ask. This is a WARNING and never refuses a start. The daemon asks again when it installs its first transit route that carries a path MTU, and installs that route without the MTU unless the kernel answered that it accepts one. Remedy: run `ze doctor` as root after the daemon has started.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-mpls-transit-mtu-unknown"},
		RelatedCodes: []string{codeDoctorMPLSTransitMTUUnenforced},
	},
	{
		Code:         codeDoctorMPLSTTLPropagateOff,
		Title:        "Pushed MPLS labels do not carry the IP TTL",
		Description:  "The configuration makes the kernel FIB push MPLS labels, and the kernel holds net.mpls.ip_ttl_propagate 0. ze writes no TTL into the labels it pushes, so the kernel chooses it: the packet's IP TTL while the key is 1, and net.mpls.default_ttl while it is 0. RFC 3032 Section 2.4.3 requires a label pushed onto an IP packet to carry the packet's IP TTL. ze refuses 0 on every write it makes, so another program (sysctl -w, a boot script) wrote it. This is an ERROR: `ze doctor` reports the box not ready. Remedy: run `sysctl -w net.mpls.ip_ttl_propagate=1`, and remove what wrote 0.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-mpls-ttl-propagate-off"},
		RelatedCodes: []string{codeDoctorMPLSTTLPropagateUnknown},
	},
	{
		Code:         codeDoctorMPLSTTLPropagateUnknown,
		Title:        "TTL of pushed MPLS labels unknown",
		Description:  "The configuration makes the kernel FIB push MPLS labels, and ze could not read net.mpls.ip_ttl_propagate, or read a value other than 0 or 1, so it cannot tell whether the labels it pushes carry the IP TTL that RFC 3032 Section 2.4.3 requires. A read that fails is reported, never taken as a pass. An absent key means the kernel holds no MPLS table, which doctor-mpls-unavailable also reports. This is a WARNING. Remedy: load mpls_router when the key is absent, run `ze doctor` as root when the read was refused, or run `sysctl -w net.mpls.ip_ttl_propagate=1` for an unexpected value.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-mpls-ttl-propagate-unknown"},
		RelatedCodes: []string{codeDoctorMPLSTTLPropagateOff, codeDoctorMPLSUnavailable},
	},
	{
		Code:         codeDoctorMPLSUnavailable,
		Title:        "MPLS kernel support unavailable",
		Description:  "The kernel holds no AF_MPLS forwarding table: /proc/sys/net/mpls/platform_labels does not exist, and the configuration asks the kernel FIB to forward MPLS. BGP labeled routes cannot be installed, and neither LDP nor RSVP-TE can program a label. This is an ERROR: `ze` refuses to start on it and `ze config validate` fails on it, because a daemon that advertises label forwarding it cannot perform blackholes the traffic it attracted. Remedy: load mpls_router, or run a kernel built with CONFIG_MPLS_ROUTING and CONFIG_MPLS_IPTUNNEL. ze's own appliance kernel builds both in, so it needs no module. A table that EXISTS with a label space of 0 is not reported at all: ze writes a label space before it programs its first label.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-mpls-unavailable"},
		RelatedCodes: []string{codeDoctorMPLSUnknown},
	},
	{
		Code:         codeDoctorMPLSUnknown,
		Title:        "MPLS kernel support unknown",
		Description:  "The MPLS capability probe (/proc/sys/net/mpls/platform_labels, under the root named by ze.test.doctor.procfs-root) could not be read, so ze cannot tell whether the kernel holds an AF_MPLS forwarding table. This is reported rather than passed over in silence: a check that cannot be evaluated is not a check that succeeded. It is a WARNING and it does not refuse a start, because refusing on a probe ze could not read would stop a router whose kernel is fine. Remedy: check the permissions on /proc/sys/net/mpls and the path above it. An ABSENT probe is not this code -- it means the kernel holds no AF_MPLS table, it reports doctor-mpls-unavailable, and that one does refuse the start.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-mpls-unknown"},
		RelatedCodes: []string{codeDoctorMPLSUnavailable},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: fibkernel: doctor code registration refused: " + err.Error())
		}
	}
}
