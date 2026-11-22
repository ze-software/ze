// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/fib/kernel.

package fibkernel

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorMPLSTransitMTUUnenforced     = "doctor-mpls-transit-mtu-unenforced"
	codeDoctorMPLSTransitMTUUnknown        = "doctor-mpls-transit-mtu-unknown"
	codeDoctorMPLSTTLPropagateOff          = "doctor-mpls-ttl-propagate-off"
	codeDoctorMPLSTTLPropagateUnknown      = "doctor-mpls-ttl-propagate-unknown"
	codeDoctorMPLSUnavailable              = "doctor-mpls-unavailable"
	codeDoctorMPLSUnknown                  = "doctor-mpls-unknown"
	codeDoctorSRv6EncapUnavailable         = "doctor-srv6-encap-unavailable"
	codeDoctorSRv6EncapUnknown             = "doctor-srv6-encap-unknown"
	codeDoctorSRv6ForwardFragUnavailable   = "doctor-srv6-forward-frag-unavailable"
	codeDoctorSRv6ForwardFragUnknown       = "doctor-srv6-forward-frag-unknown"
	codeDoctorSRv6ForwardTTLNotDecremented = "doctor-srv6-forward-ttl-not-decremented"
	codeDoctorSRv6ForwardTTLUnknown        = "doctor-srv6-forward-ttl-unknown"
	exampleDoctorJSON                      = "ze doctor --json"
)

// codeKernelNexthop names a kernel without nexthop objects.
// internal/core/diagnostic/codes.go declares it, so `ze show diagnostic
// doctor-kernel-nexthop` answers.
const codeKernelNexthop = "doctor-kernel-nexthop"

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
	{
		Code:         codeDoctorSRv6EncapUnavailable,
		Title:        "SRv6 seg6 kernel encapsulation unavailable",
		Description:  "BGP runs on the kernel FIB, and the kernel refuses seg6 lwtunnel encapsulation routes (RTA_ENCAP_TYPE LWTUNNEL_ENCAP_SEG6) with EOPNOTSUPP because it was built without CONFIG_IPV6_SEG6_LWTUNNEL. Plain IP routes still install and forward, but any BGP route carrying an SRv6 Service SID (RFC 9252) cannot be programmed into the kernel FIB and fails when installation is attempted. This is a WARNING and it does not refuse a start, because a BGP router on the kernel FIB may carry no SRv6 Service SID routes. Remedy: run ze's appliance kernel, or a Linux kernel built with CONFIG_IPV6_SEG6_LWTUNNEL=y, and run `ze doctor` to confirm.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-srv6-encap-unavailable"},
		RelatedCodes: []string{codeDoctorSRv6EncapUnknown, codeDoctorSRv6ForwardTTLNotDecremented},
	},
	{
		Code:         codeDoctorSRv6EncapUnknown,
		Title:        "SRv6 seg6 kernel encapsulation support unknown",
		Description:  "ze could not determine whether the kernel supports seg6 lwtunnel encapsulation routes (CONFIG_IPV6_SEG6_LWTUNNEL). The behavioral probe installs a test route and forwards one packet inside a throwaway network namespace using /dev/net/tun, which needs CAP_NET_ADMIN, CAP_SYS_ADMIN, and access to /dev/net/tun; an unprivileged `ze doctor` run or a container without /dev/net/tun cannot run it. This is a WARNING and never refuses a start. Remedy: run `ze doctor` as root on a host with /dev/net/tun available.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-srv6-encap-unknown"},
		RelatedCodes: []string{codeDoctorSRv6EncapUnavailable, codeDoctorSRv6ForwardTTLUnknown},
	},
	{
		Code:         codeDoctorSRv6ForwardTTLNotDecremented,
		Title:        "SRv6 headend does not decrement inner TTL or Hop Limit",
		Description:  "BGP runs on the kernel FIB, and the kernel supports seg6 lwtunnel encapsulation but forwards packets through seg6_input without passing ip_forward or ip6_forward (Linux limitation LL-SEG6-HEADEND-NO-TTL-DECREMENT, docs/architecture/linux-limitations.md): it is a stock kernel without CONFIG_IPV6_SEG6_FORWARD_TTL, which ze's own runtime kernel adds with gokrazy/kernel/patches/0014-seg6-forward-ttl.patch. SRv6 Service SID routes still install and encapsulate traffic, but forwarded packets leave with their inner IPv4 TTL or IPv6 Hop Limit unchanged (violating RFC 8986 Section 5.1 S05), packets arriving with TTL or Hop Limit 1 are encapsulated instead of answered with ICMP Time Exceeded, and the headend skips IPv4/IPv6 forward PMTU and FORWARD hooks. This is a WARNING and it does not refuse a start. Remedy: run ze's appliance kernel, or a Linux kernel built with CONFIG_IPV6_SEG6_LWTUNNEL=y and gokrazy/kernel/patches/0014-seg6-forward-ttl.patch, and run `ze doctor` to confirm.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-srv6-forward-ttl-not-decremented"},
		RelatedCodes: []string{codeDoctorSRv6ForwardTTLUnknown, codeDoctorSRv6EncapUnavailable},
	},
	{
		Code:         codeDoctorSRv6ForwardTTLUnknown,
		Title:        "SRv6 headend inner TTL decrement support unknown",
		Description:  "ze could not determine whether the kernel decrements the inner IPv4 TTL or IPv6 Hop Limit on forwarded SRv6 seg6 encapsulation routes (CONFIG_IPV6_SEG6_FORWARD_TTL, gokrazy/kernel/patches/0014-seg6-forward-ttl.patch). The behavioral probe forwards one packet through a seg6 encap route inside a throwaway network namespace using /dev/net/tun, which needs CAP_NET_ADMIN, CAP_SYS_ADMIN, and access to /dev/net/tun; an unprivileged `ze doctor` run or a container without /dev/net/tun cannot complete it. This is a WARNING and never refuses a start. Remedy: run `ze doctor` as root on a host with /dev/net/tun available.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-srv6-forward-ttl-unknown"},
		RelatedCodes: []string{codeDoctorSRv6ForwardTTLNotDecremented, codeDoctorSRv6EncapUnknown},
	},
	{
		Code:         codeDoctorSRv6ForwardFragUnavailable,
		Title:        "SRv6 headend drops oversized DF-clear IPv4 packets",
		Description:  "BGP runs on the kernel FIB, and the kernel does not fragment forwarded DF-clear IPv4 packets whose length exceeds the egress MTU minus seg6 headroom before encapsulation (Linux limitation LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG, docs/architecture/linux-limitations.md): it is a stock or unpatched kernel without CONFIG_IPV6_SEG6_FORWARD_FRAG, which ze's own runtime kernel adds with gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch. SRv6 Service SID routes still install and forward packets that fit within the effective MTU, but oversized DF-clear IPv4 packets are encapsulated past the egress MTU and silently dropped in ip6_fragment instead of being fragmented at the inner IPv4 layer before encapsulation. This is a WARNING and never refuses a start. Remedy: run ze's appliance kernel, or a Linux kernel built with CONFIG_IPV6_SEG6_LWTUNNEL=y and gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch, and confirm with `ze doctor`.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-srv6-forward-frag-unavailable"},
		RelatedCodes: []string{codeDoctorSRv6ForwardFragUnknown, codeDoctorSRv6ForwardTTLNotDecremented, codeDoctorSRv6EncapUnavailable},
	},
	{
		Code:         codeDoctorSRv6ForwardFragUnknown,
		Title:        "SRv6 headend DF-clear IPv4 fragmentation support unknown",
		Description:  "ze could not determine whether the kernel fragments oversized forwarded DF-clear IPv4 packets before SRv6 seg6 encapsulation (CONFIG_IPV6_SEG6_FORWARD_FRAG, gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch). The behavioral probe forwards packets through a seg6 encap route inside a throwaway network namespace using /dev/net/tun, which needs CAP_NET_ADMIN, CAP_SYS_ADMIN, and access to /dev/net/tun; an unprivileged `ze doctor` run or a container without /dev/net/tun cannot complete it. This is a WARNING and never refuses a start. Remedy: run `ze doctor` as root on a host with /dev/net/tun available.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-srv6-forward-frag-unknown"},
		RelatedCodes: []string{codeDoctorSRv6ForwardFragUnavailable, codeDoctorSRv6ForwardTTLUnknown, codeDoctorSRv6EncapUnknown},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: fibkernel: doctor code registration refused: " + err.Error())
		}
	}
}
