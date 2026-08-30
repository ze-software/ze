// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- RFC 4552 IPsec readiness doctor check.
// Related: ipsec_install.go -- the installer whose kernel dependency this check probes.
// Related: register.go -- registerOSPFIPsecDoctor wires this into the doctor runner.
// RFC: rfc/short/rfc4552.md -- OSPFv3 IPsec (needs CAP_NET_ADMIN + kernel XFRM).
//
// The check is a no-op unless an IPv6-family interface configures IPsec; when it does and
// the kernel XFRM dataplane cannot be reached, it warns (spec-ospf-ext-16 AC-12 / R-7) so
// the operator is not misled into thinking an unprotected adjacency is protected.

package ospf

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// codeOSPFv3IPsec is registered centrally in internal/core/diagnostic/codes.go alongside
// doctor-ospfv3-raw-socket (spec-ospf-ext-16 Current Behavior).
const codeOSPFv3IPsec = "doctor-ospfv3-ipsec"

// xfrmProbe reports whether the kernel XFRM dataplane is usable (CAP_NET_ADMIN + kernel
// IPsec). Tests override it; the platform default lives in doctor_ipsec_{linux,other}.go.
var xfrmProbe = xfrmAvailable

// ospfV6IPsecConfigured reports whether any IPv6-family interface configures RFC 4552 IPsec.
func ospfV6IPsecConfigured(cfg ospfConfig) bool {
	if cfg.V6 == nil {
		return false
	}
	for i := range cfg.V6.Interfaces {
		if cfg.V6.Interfaces[i].IPsec != nil {
			return true
		}
	}
	return false
}

// ospfIPsecDiagnostics returns the IPsec-readiness diagnostics for a resolved config given
// whether the kernel XFRM dataplane is available. It is pure so it can be unit-tested
// without a live kernel.
func ospfIPsecDiagnostics(cfg ospfConfig, xfrmOK bool) []diagnostic.Diagnostic {
	if !cfg.present || !ospfV6IPsecConfigured(cfg) || xfrmOK {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeOSPFv3IPsec,
		Severity: diagnostic.SeverityWarning,
		Message:  "an OSPFv3 interface configures RFC 4552 IPsec but the kernel XFRM dataplane is unavailable (needs CAP_NET_ADMIN and kernel IPsec); the interface would form an UNPROTECTED adjacency",
	}}
}

// checkOSPFv3IPsec is the registered doctor check. It resolves the OSPF config from the
// tree the way the engine does and probes the kernel XFRM dataplane.
func checkOSPFv3IPsec(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ospfTree := tree.GetContainer(Namespace)
	if ospfTree == nil {
		return nil
	}
	data, err := json.Marshal(map[string]any{Namespace: ospfTree.ToMap()})
	if err != nil {
		return nil
	}
	cfg, err := parseOSPFConfig([]configSection{{Root: Namespace, Data: string(data)}}, systemRouterIDSource{})
	if err != nil {
		return nil // a structural error is the per-leaf YANG validator's job to report.
	}
	if !ospfV6IPsecConfigured(cfg) {
		return nil
	}
	return ospfIPsecDiagnostics(cfg, xfrmProbe())
}
