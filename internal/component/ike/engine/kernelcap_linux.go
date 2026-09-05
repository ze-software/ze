//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Related: register.go -- the plugin registration this init() runs beside
// Related: internal/component/kernelcap -- the enrolment and the shared probe
//
// Every Child SA ze installs goes through XFRM. A host whose kernel holds no
// XFRM dataplane negotiates a tunnel that carries nothing, and no other IPsec
// surface says so: they all report what the engine believed at install time.
//
// The enrolment is Linux-only because XFRM is. Off Linux nothing registers, so
// a developer's `ze` on a laptop is not refused for a dataplane this build
// never programs.

package engine

import (
	"github.com/ze-software/ze/internal/component/kernelcap"
)

func init() {
	kernelcap.MustRegister(kernelcap.Capability{
		Subsystem:   "ipsec",
		Component:   "ike",
		Kernel:      "CONFIG_XFRM_USER",
		ConfigLeaf:  "vpn ipsec",
		CodeAbsent:  diagnosticIPsecXFRMUnavailable,
		CodeUnknown: diagnosticIPsecXFRMUnknown,
		Order:       734,
		InUse:       kernelcap.IPsecInUse,
		Probe:       kernelcap.XFRM,
	})
}
