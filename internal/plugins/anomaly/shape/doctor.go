// Design: plan/spec-anomaly-2-shape.md -- armed-mode firewall readiness check.
//
// In armed mode the responder installs live nft rules via the firewall component.
// If no firewall is configured there is no backend to apply them to, so `ze doctor`
// warns rather than letting arm attempts silently fail to reach the kernel.

package shape

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

func checkFirewall(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	as := tree.GetContainer(configRoot)
	if as == nil {
		return nil
	}
	// Only armed mode enforces; shadow logs and installs nothing.
	if mode, ok := as.Get("mode"); !ok || mode != ModeArmed {
		return nil
	}
	if tree.GetContainer("firewall") != nil {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-anomaly-shape-armed-no-firewall",
		Severity: "warning",
		Message:  "anomaly-shape is in armed mode but no firewall is configured; armed per-source rate-limits cannot be applied to the kernel",
	}}
}
