// Design: plan/learned/929-isis-3-l2-transport.md -- raw-socket readiness doctor check
// Related: doctor_linux.go -- the AF_PACKET raw-socket probe this check runs
// Related: doctor_other.go -- non-Linux probe stub
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
//
// IS-IS's raw L2 transport opens an AF_PACKET/SOCK_RAW socket, which needs
// CAP_NET_RAW. This doctor check warns, before the engine starts, when IS-IS is
// configured but that socket cannot be opened, so the failure surfaces via
// `ze doctor` rather than only as a degraded engine at runtime (ai/rules/
// doctor-checks.md). The transport owns the raw-socket dependency, so it owns
// the check, its diagnostic code, and the unit test (plugin-self-containment).

package transport

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// rawSocketProbe is a test seam over the platform raw-socket probe.
var rawSocketProbe = rawSocketAvailable

// checkISISRawSocket warns when IS-IS is configured but a raw AF_PACKET socket
// cannot be opened (needs CAP_NET_RAW). It is a no-op when IS-IS is not
// configured, so boxes that do not run IS-IS get no spurious warning.
func checkISISRawSocket(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer("isis") == nil {
		return nil
	}
	if rawSocketProbe() {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-isis-raw-socket",
		Severity: diagnostic.SeverityWarning,
		Message:  "cannot open raw AF_PACKET socket (IS-IS needs CAP_NET_RAW)",
	}}
}
