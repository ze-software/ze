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
)

// MPLS diagnostic codes. Both are declared in
// internal/core/diagnostic/codes.go, which `ze explain <code>` reads.
const (
	diagnosticMPLSUnavailable = "doctor-mpls-unavailable"
	diagnosticMPLSUnknown     = "doctor-mpls-unknown"
)

func init() {
	kernelcap.MustRegister(kernelcap.Capability{
		Subsystem:   "mpls",
		Component:   "fib-kernel",
		Kernel:      "CONFIG_MPLS_ROUTING",
		ConfigLeaf:  "a labeled BGP family, ldp, rsvp-te or an interface mpls block on the kernel FIB",
		CodeAbsent:  diagnosticMPLSUnavailable,
		CodeUnknown: diagnosticMPLSUnknown,
		Order:       735,
		InUse:       kernelcap.MPLSInUse,
		Probe:       kernelcap.MPLS,
	})
}
