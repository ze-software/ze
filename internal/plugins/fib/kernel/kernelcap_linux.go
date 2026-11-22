//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: register.go -- the plugin registration this init() runs beside
// Related: labelspace_linux.go -- the label space this plugin writes before it programs a label
// Related: internal/component/kernelcap -- the enrolment and the shared probe
//
// This plugin is the one that programs MPLS labels and SRv6 seg6 encapsulations
// into the kernel, so it is the one that owns their kernel requirements. A VPP
// or P4 backend does its own MPLS and SRv6 and never reaches these
// registrations, and a config on another backend is not gated by them either:
// MPLSInUse and SRv6InUse check `fib { kernel { } }` first.
//
// The enrolments are Linux-only because AF_MPLS and seg6 LWTUNNEL are.

package fibkernel

import (
	"github.com/ze-software/ze/internal/component/kernelcap"
)

const srv6ConfigLeaf = "a BGP peer or group on the kernel FIB"

func init() {
	kernelcap.MustRegister(kernelcap.Capability{
		Subsystem:   "mpls",
		Component:   pluginName,
		Kernel:      "CONFIG_MPLS_ROUTING",
		ConfigLeaf:  "a labeled BGP family, ldp, rsvp-te or an interface mpls block on the kernel FIB",
		CodeAbsent:  codeDoctorMPLSUnavailable,
		CodeUnknown: codeDoctorMPLSUnknown,
		Order:       735,
		InUse:       kernelcap.MPLSInUse,
		Probe:       kernelcap.MPLS,
	})
	kernelcap.MustRegister(transitMTUCapability)
	kernelcap.MustRegister(srv6EncapCapability)
	kernelcap.MustRegister(srv6ForwardTTLCapability)
	kernelcap.MustRegister(srv6ForwardFragCapability)
}

// transitMTUCapability is the path MTU an RSVP-TE transit route carries. Only
// a kernel with gokrazy/kernel/patches/0002-mpls-ip-mtu.patch accepts it, so on
// a stock kernel addMPLSSwap installs the route without it and this capability
// reports the loss as a warning (owner decision, 2026-10-08). The backend asks
// the same question in its own namespace before its first such install
// (transitRouteMTU, kernelcap.MPLSIPMTUInThisNamespace).
var transitMTUCapability = kernelcap.Capability{
	Subsystem:   "mpls-transit-mtu",
	Component:   pluginName,
	Kernel:      "CONFIG_MPLS_IP_MTU",
	ConfigLeaf:  "rsvp-te on the kernel FIB",
	Degrades:    "RSVP-TE transit routes install without the path MTU, so an oversized labeled packet is dropped instead of fragmented or answered with ICMP",
	CodeAbsent:  codeDoctorMPLSTransitMTUUnenforced,
	CodeUnknown: codeDoctorMPLSTransitMTUUnknown,
	Order:       739,
	InUse:       kernelcap.MPLSTransitMTUInUse,
	Probe:       kernelcap.MPLSIPMTU,
}

// srv6EncapCapability is kernel seg6 LWTUNNEL encapsulation support
// (CONFIG_IPV6_SEG6_LWTUNNEL), needed to program RFC 9252 SRv6 Service SID
// routes into the Linux FIB. It degrades rather than refusing a start because
// SRv6InUse matches any BGP peer or group on the kernel FIB (IBGP always
// accepts Prefix-SID), so a router carrying no SRv6 routes must still boot.
var srv6EncapCapability = kernelcap.Capability{
	Subsystem:   "srv6-encap",
	Component:   pluginName,
	Kernel:      "CONFIG_IPV6_SEG6_LWTUNNEL",
	ConfigLeaf:  srv6ConfigLeaf,
	Degrades:    "BGP routes carrying an SRv6 Service SID cannot be installed into the kernel FIB (plain IP routes still install)",
	CodeAbsent:  codeDoctorSRv6EncapUnavailable,
	CodeUnknown: codeDoctorSRv6EncapUnknown,
	Order:       740,
	InUse:       kernelcap.SRv6InUse,
	Probe:       kernelcap.SRv6Encap,
}

// srv6ForwardTTLCapability is RFC 8986 Section 5.1 S05 inner TTL/Hop Limit
// decrement on forwarded seg6 L3 encapsulation routes. Only a kernel carrying
// gokrazy/kernel/patches/0014-seg6-forward-ttl.patch
// (CONFIG_IPV6_SEG6_FORWARD_TTL) performs the decrement; a stock kernel still
// encapsulates traffic with the inner TTL unchanged, so absence warns without
// refusing a start.
var srv6ForwardTTLCapability = kernelcap.Capability{
	Subsystem:   "srv6-forward-ttl",
	Component:   pluginName,
	Kernel:      "CONFIG_IPV6_SEG6_FORWARD_TTL",
	ConfigLeaf:  srv6ConfigLeaf,
	Degrades:    "forwarded packets entering an SRv6 seg6 headend leave with their inner IPv4 TTL or IPv6 Hop Limit unchanged and TTL 1 packets are encapsulated instead of answered with ICMP Time Exceeded (LL-SEG6-HEADEND-NO-TTL-DECREMENT)",
	CodeAbsent:  codeDoctorSRv6ForwardTTLNotDecremented,
	CodeUnknown: codeDoctorSRv6ForwardTTLUnknown,
	Order:       741,
	InUse:       kernelcap.SRv6InUse,
	Probe:       kernelcap.SRv6ForwardTTL,
}

// srv6ForwardFragCapability is pre-encapsulation IPv4 fragmentation of
// forwarded DF-clear IPv4 packets exceeding the effective seg6 MTU. Only a
// kernel carrying gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch
// (CONFIG_IPV6_SEG6_FORWARD_FRAG) performs this fragmentation; a stock kernel
// still encapsulates packets that fit within the effective MTU, so absence
// warns without refusing a start.
var srv6ForwardFragCapability = kernelcap.Capability{
	Subsystem:   "srv6-forward-frag",
	Component:   pluginName,
	Kernel:      "CONFIG_IPV6_SEG6_FORWARD_FRAG",
	ConfigLeaf:  srv6ConfigLeaf,
	Degrades:    "forwarded DF-clear IPv4 packets exceeding the egress MTU minus seg6 headroom are dropped after encapsulation instead of being fragmented before encapsulation (LL-SEG6-HEADEND-NO-DF-CLEAR-FRAG)",
	CodeAbsent:  codeDoctorSRv6ForwardFragUnavailable,
	CodeUnknown: codeDoctorSRv6ForwardFragUnknown,
	Order:       742,
	InUse:       kernelcap.SRv6InUse,
	Probe:       kernelcap.SRv6ForwardFrag,
}
