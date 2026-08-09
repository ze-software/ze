// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- raw-socket (proto 46) readiness check
// Related: register.go -- DoctorChecks registration (rsvp-te-rawsock)
// Related: transport_linux.go -- the raw socket this check probes
//
// RSVP-TE's transport opens an AF_INET SOCK_RAW socket on IP protocol 46, which
// needs CAP_NET_RAW. This doctor check warns, before the engine starts, when
// rsvp-te is configured but that socket cannot be opened, so the failure is
// surfaced by `ze doctor` rather than only as a degraded engine at runtime.
package rsvpte

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rsvpRawSocketProbe is a test seam over the platform raw-socket probe.
var rsvpRawSocketProbe = rsvpRawSocketAvailable

// checkRSVPTERawSocket warns when rsvp-te is configured but a raw IP socket for
// protocol 46 cannot be opened (needs CAP_NET_RAW). rsvp-te owns the rsvp-te
// config block, so it owns this readiness check (ai/rules/repo-maintenance.md).
func checkRSVPTERawSocket(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer("rsvp-te") == nil {
		return nil
	}
	if rsvpRawSocketProbe() {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-rsvpte-rawsock-unavailable",
		Severity: "warning",
		Message:  "cannot open raw IP socket for protocol 46 (RSVP-TE needs CAP_NET_RAW)",
	}}
}
