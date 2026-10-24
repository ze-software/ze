// Design: docs/architecture/traffic/traffic-usage.md -- traffic-usage doctor check (kernel eBPF/TCX + CAP_BPF)

package trafficusage

import (
	"errors"
	"syscall"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// doctorChecks registers the readiness check for traffic-usage. The plugin owns
// the traffic-usage config block, so it owns the eBPF availability check (see
// ai/rules/repo-maintenance.md).
func doctorChecks() []registry.DoctorCheckDef {
	return []registry.DoctorCheckDef{{
		Name:         "traffic-usage-ebpf",
		Phase:        rpc.DoctorPhasePostConfig,
		Order:        720,
		Dependencies: []string{dependencyInterface},
		Platforms:    []string{"any"},
		Codes:        []string{"doctor-traffic-usage-ebpf", "doctor-traffic-usage-ebpf-unknown"},
		Check:        checkTrafficUsageBPF,
	}}
}

// checkTrafficUsageBPF warns when traffic-usage is enabled but this host cannot
// attach the eBPF TCX programs, and says Unknown when the probe could not ask
// the kernel. The probe runs in the doctor's own process, so it measures this
// kernel and this process's privileges, not the daemon's.
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
	if !enabled {
		return nil
	}
	return trafficUsageDiagnostic(enabled, probeTCX())
}

// tcxReadiness is what the readiness probe established about eBPF TCX on this
// host. The zero value is no verdict, never a pass.
type tcxReadiness uint8

const (
	// tcxReadinessUnspecified is the zero value: no probe answered.
	tcxReadinessUnspecified tcxReadiness = iota
	// tcxReady means the kernel accepted a TCX attach request.
	tcxReady
	// tcxAbsent means the kernel or the build cannot attach TCX programs.
	tcxAbsent
	// tcxUnknown means the probe could not ask the kernel, so neither answer
	// was established.
	tcxUnknown
)

// String names the readiness for a test failure.
func (r tcxReadiness) String() string {
	switch r {
	case tcxReady:
		return "ready"
	case tcxAbsent:
		return "absent"
	case tcxUnknown:
		return "unknown"
	case tcxReadinessUnspecified:
		return "unspecified"
	}
	return "invalid"
}

// tcxReadinessResult is one probe's verdict and the evidence behind it.
type tcxReadinessResult struct {
	state  tcxReadiness
	reason error
}

// errNoTCX is the kernel's answer that it has no TCX attach point.
var errNoTCX = errors.New("the kernel has no eBPF TCX attach point (Linux 6.6 or later required)")

// classifyTCXProbe turns the probe's attach error into a verdict. The probe
// attaches to an interface index no device holds, so the kernel's ENODEV means
// it parsed a TCX request and looked for the device: TCX is there. A refused
// load or attach (EPERM, EACCES) means this process was not allowed to ask,
// which is Unknown; reporting absence there would send the operator after a
// kernel that is fine.
func classifyTCXProbe(err error) tcxReadinessResult {
	if err == nil {
		return tcxReadinessResult{state: tcxReady, reason: errors.New("the probe attach succeeded")}
	}
	if errors.Is(err, syscall.ENODEV) {
		return tcxReadinessResult{state: tcxReady, reason: err}
	}
	if errors.Is(err, errNoTCX) {
		return tcxReadinessResult{state: tcxAbsent, reason: err}
	}
	return tcxReadinessResult{state: tcxUnknown, reason: err}
}

// trafficUsageDiagnostic builds the diagnostic for an (enabled, readiness)
// pair: nothing when accounting is off or the host is ready, a warning when the
// host cannot attach, and an Unknown warning naming the measuring command when
// the probe could not tell.
func trafficUsageDiagnostic(enabled bool, readiness tcxReadinessResult) []rpc.DoctorCheckDiagnostic {
	if !enabled {
		return nil
	}
	var tb textbuf.Buffer
	switch readiness.state {
	case tcxReady:
		return nil
	case tcxAbsent:
		tb.Str("traffic-usage is enabled but eBPF TCX accounting is unavailable: ").Err(readiness.reason).
			Str("; byte accounting will not run. Use a Linux 6.6 or later kernel built with CONFIG_NET_XGRESS")
		return []rpc.DoctorCheckDiagnostic{{
			Code:     "doctor-traffic-usage-ebpf",
			Severity: "warning",
			Message:  tb.String(),
		}}
	case tcxUnknown:
		tb.Str("traffic-usage is enabled but eBPF TCX readiness could not be measured: ").Err(readiness.reason)
	case tcxReadinessUnspecified:
		tb.Str("traffic-usage is enabled but the eBPF TCX readiness probe returned no verdict")
	}
	tb.Str("; run `sudo ze doctor` (CAP_BPF and CAP_NET_ADMIN) to measure it")
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-traffic-usage-ebpf-unknown",
		Severity: "warning",
		Message:  tb.String(),
	}}
}
