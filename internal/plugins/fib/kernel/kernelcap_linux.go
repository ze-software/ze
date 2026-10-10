//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: register.go -- the plugin registration this init() runs beside
// Related: labelspace_linux.go -- the label space this plugin writes before it programs a label
// Related: internal/component/kernelcap -- the enrolment and the shared probe
//
// This plugin is the one that programs MPLS labels into the kernel, so it is the
// one that owns the kernel requirement. A VPP or P4 backend does its own MPLS
// and never reaches this registration, and a config on another backend is not
// gated by it either: MPLSInUse checks `fib { kernel { } }` before it looks at
// any labeled family.
//
// The enrolment is Linux-only because the AF_MPLS table is.

package fibkernel

import (
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	kernelcap.MustRegister(kernelcap.Capability{
		Subsystem:   "mpls",
		Component:   pluginName,
		Kernel:      "CONFIG_MPLS_ROUTING",
		ConfigLeaf:  "a labeled BGP family, ldp, rsvp-te or an interface mpls block on the kernel FIB",
		CodeAbsent:  diagnostic.CodeDoctorMPLSUnavailable,
		CodeUnknown: diagnostic.CodeDoctorMPLSUnknown,
		Order:       735,
		InUse:       kernelcap.MPLSInUse,
		Probe:       kernelcap.MPLS,
	})
	kernelcap.MustRegister(transitMTUCapability)
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
	CodeAbsent:  diagnostic.CodeDoctorMPLSTransitMTUUnenforced,
	CodeUnknown: diagnostic.CodeDoctorMPLSTransitMTUUnknown,
	Order:       739,
	InUse:       kernelcap.MPLSTransitMTUInUse,
	Probe:       kernelcap.MPLSIPMTU,
}
