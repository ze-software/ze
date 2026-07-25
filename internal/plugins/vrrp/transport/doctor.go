// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- raw-socket readiness doctor check
// Related: doctor_linux.go -- the AF_INET/SOCK_RAW proto-112 probe this check runs
// Related: doctor_other.go -- non-Linux probe stub
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
//
// VRRP's raw transport opens an AF_INET/SOCK_RAW (protocol 112) socket, which
// needs CAP_NET_RAW. This check warns, before the engine starts, when VRRP is
// configured but that socket cannot be opened, so the failure surfaces via
// `ze doctor` rather than only as a degraded engine at runtime. The transport
// owns the raw-socket dependency, so it owns the check, its diagnostic code, and
// the unit test (plugin-self-containment).

package transport

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// rawSocketProbe is a test seam over the platform raw-socket probe.
var rawSocketProbe = rawSocketAvailable

// checkVRRPRawSocket warns when VRRP is configured but a raw proto-112 socket
// cannot be opened (needs CAP_NET_RAW). It is a no-op when VRRP is not configured,
// so boxes that do not run VRRP get no spurious warning.
func checkVRRPRawSocket(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if !vrrpConfigured(tree) {
		return nil
	}
	if rawSocketProbe() {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-vrrp-raw-socket",
		Severity: diagnostic.SeverityWarning,
		Message:  "cannot open raw IP socket for protocol 112 (VRRP needs CAP_NET_RAW)",
	}}
}

// vrrpConfigured reports whether any vrrp container exists anywhere in the tree.
// VRRP is configured under an interface unit (interface ... unit ... ipv4/ipv6
// vrrp group), not as a top-level container, so the check walks every container
// path and matches a final "vrrp" segment. This is robust to the exact nesting
// spec-vrrp-5's YANG augment chooses.
func vrrpConfigured(tree *config.Tree) bool {
	for _, p := range config.CollectContainerPaths(tree) {
		segs := config.SplitPath(p)
		if len(segs) > 0 && segs[len(segs)-1] == "vrrp" {
			return true
		}
	}
	return false
}
