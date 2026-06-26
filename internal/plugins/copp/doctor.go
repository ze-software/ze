// Design: plan/spec-cp-survival-2-copp-port179.md -- doctor check

package copp

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

func checkCoppInputChain(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}

	cppTree := tree.GetContainer("control-plane-protection")
	if cppTree == nil {
		return nil
	}
	bgpTree := cppTree.GetContainer("bgp")
	if bgpTree == nil {
		return nil
	}

	if _, ok := bgpTree.Get("rate"); !ok {
		return nil
	}

	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-copp-missing",
		Severity: "warning",
		Message:  "control-plane-protection bgp is configured but the copp input chain may not be active yet",
	}}
}
