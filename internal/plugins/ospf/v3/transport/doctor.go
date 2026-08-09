// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- raw IPv6 socket readiness doctor check
// RFC: rfc/short/rfc5340.md (§2.9 raw IPv6 proto 89 needs CAP_NET_RAW)

package transport

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// rawSocketProbe is the platform raw-IPv6-socket open probe; tests override it.
var rawSocketProbe = rawSocketAvailable

// checkOSPFv3RawSocket warns when OSPFv3 is configured but a raw IPv6 proto-89
// socket cannot be opened (missing CAP_NET_RAW). It is a no-op when OSPFv3 is not
// configured, so the check costs nothing on non-OSPFv3 nodes.
func checkOSPFv3RawSocket(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer("ospfv3") == nil {
		return nil
	}
	if rawSocketProbe() {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     "doctor-ospfv3-raw-socket",
		Severity: diagnostic.SeverityWarning,
		Message:  "cannot open raw IPv6 socket for protocol 89 (OSPFv3 needs CAP_NET_RAW)",
	}}
}
