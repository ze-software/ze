// Design: plan/learned/977-traffic-usage.md -- traffic-usage doctor check (kernel eBPF/TCX + CAP_BPF)

package trafficusage

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// doctorChecks registers the readiness check for traffic-usage. The plugin owns
// the traffic-usage config block, so it owns the eBPF availability check (see
// ai/rules/doctor-checks.md).
func doctorChecks() []registry.DoctorCheckDef {
	return []registry.DoctorCheckDef{{
		Name:         "traffic-usage-ebpf",
		Phase:        rpc.DoctorPhasePostConfig,
		Order:        720,
		Dependencies: []string{"interface"},
		Platforms:    []string{"any"},
		Codes:        []string{"doctor-traffic-usage-ebpf"},
		Check:        checkTrafficUsageBPF,
	}}
}

// checkTrafficUsageBPF warns when traffic-usage is enabled but the kernel cannot
// load/attach the eBPF programs (missing CAP_BPF, no TCX, or a non-Linux build).
func checkTrafficUsageBPF(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	tu := tree.GetContainerPath(configRoot)
	if tu == nil {
		return nil
	}
	enabled := false
	if v, ok := tu.Get("enabled"); ok && v == "true" {
		enabled = true
	}
	return trafficUsageDiagnostic(enabled, ebpfSupported())
}

// trafficUsageDiagnostic builds the diagnostic for a (enabled, availability)
// pair: a warning only when accounting is requested but unavailable.
func trafficUsageDiagnostic(enabled bool, availErr error) []rpc.DoctorCheckDiagnostic {
	if !enabled || availErr == nil {
		return nil
	}
	var tb textbuf.Buffer
	tb.Str("traffic-usage is enabled but eBPF TCX accounting is unavailable: ").Err(availErr)
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-traffic-usage-ebpf",
		Severity: "warning",
		Message:  tb.String(),
	}}
}
