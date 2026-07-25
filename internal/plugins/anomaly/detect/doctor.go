// Design: plan/learned/1048-anomaly-1-detect.md -- feature-source readiness check.
//
// The detector consumes trafficfeature, which is fed by the observation feed from
// flow-export (conntrack) or traffic-usage (eBPF). With neither configured the
// feed carries no per-source flow data, so trafficfeature yields no features and
// the detector sits idle. `ze doctor` warns rather than leaving the gap silent.

package detect

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// checkFeatureSource warns when anomaly-detect is enabled but no flow source
// (traffic-usage or flow-export) feeds the observation feed the trafficfeature
// layer depends on.
func checkFeatureSource(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ad := tree.GetContainerPath(configRoot)
	if ad == nil {
		return nil
	}
	if enabled, ok := ad.Get("enabled"); !ok || enabled != "true" {
		return nil
	}
	if tree.GetContainerPath("traffic/usage") != nil || tree.GetContainer("flow-export") != nil {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-anomaly-detect-no-feature-source",
		Severity: "warning",
		Message:  "anomaly-detect is enabled but neither traffic-usage nor flow-export is configured; the trafficfeature layer receives no flow data, so the detector will observe no per-source features",
	}}
}
