// Design: docs/architecture/ddos/cp-survival-5-detect-5-characterization.md -- flow-source readiness.
// Related: register.go -- DoctorChecks registration (ddos-detect-flow-source)
// Related: characterize.go -- the trafficusage / flow-export queries this guards
//
// Characterization needs an on-box flow source: traffic-usage (track-ip) for the
// fast victim target and/or flow-export (conntrack) for the classifier. With
// neither configured the detector still runs but degrades to generic-flood with
// no target, so `ze doctor` warns rather than leaving the gap silent.
package detect

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// checkFlowSource warns when ddos-detect is enabled with characterization on but
// neither traffic-usage nor flow-export is configured. ddos-detect owns the
// characterization behavior, so it owns this readiness check (repo-maintenance.md).
func checkFlowSource(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	dd := tree.GetContainerPath(configRoot)
	if dd == nil {
		return nil
	}
	if enabled, ok := dd.Get("enabled"); !ok || enabled != "true" {
		return nil
	}
	// characterize-enable defaults true; only a value present is authoritative.
	if ce, ok := dd.Get("characterize-enable"); ok && ce == "false" {
		return nil
	}
	if tree.GetContainerPath("traffic/usage") != nil || tree.GetContainer("flow-export") != nil {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-ddos-detect-no-flow-source",
		Severity: "warning",
		Message:  "ddos-detect characterization is enabled but neither traffic-usage (track-ip) nor flow-export (conntrack) is configured; mitigation falls back to generic-flood with no target",
	}}
}
