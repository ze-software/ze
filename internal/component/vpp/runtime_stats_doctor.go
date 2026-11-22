// Design: docs/research/vpp-deployment-reference.md -- VPP runtime statistics diagnostics

package vpp

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

const runtimeStatsReportSource = "vpp-stats"
const runtimeStatsFix = "show warnings"

func init() {
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: "vpp-runtime-stats", Phase: diagnostic.DoctorPhaseRuntime, Order: 750,
		Component: componentVPP, Platforms: []string{diagnostic.DoctorPlatformAny},
		Dependencies: []string{"vpp-stats-segment"}, Codes: []string{diagnostic.CodeDoctorVPPStats},
		Check: checkRuntimeStats,
	}); err != nil {
		panic("BUG: VPP stats doctor registration failed: " + err.Error())
	}
	health.Register("vpp-stats", report.HealthProbeDegraded(diagnostic.CodeDoctorVPPStats))
}

func reportRuntimeStatsFailure(subject string, err error) {
	report.RaiseWarningFix(runtimeStatsReportSource, diagnostic.CodeDoctorVPPStats, subject,
		"VPP statistics are unavailable", runtimeStatsFix, map[string]any{reportDetailErrorKey: err.Error()})
}

func checkRuntimeStats(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	var findings []diagnostic.Diagnostic
	for _, issue := range report.Warnings() {
		if issue.Source != runtimeStatsReportSource {
			continue
		}
		if issue.Code != diagnostic.CodeDoctorVPPStats {
			continue
		}
		message := issue.Message
		if cause, ok := issue.Detail[reportDetailErrorKey].(string); ok {
			message += ": " + cause
		}
		findings = append(findings, diagnostic.Diagnostic{Code: diagnostic.CodeDoctorVPPStats,
			Severity: diagnostic.SeverityWarning, Message: message, Fix: runtimeStatsFix})
	}
	if len(findings) != 0 {
		return findings
	}
	status, reason := report.HealthProbeDegraded(diagnostic.CodeDoctorVPPStats)()
	if status != health.StatusHealthy {
		return []diagnostic.Diagnostic{{Code: diagnostic.CodeDoctorVPPStats,
			Severity: diagnostic.SeverityWarning, Message: reason, Fix: runtimeStatsFix}}
	}
	return findings
}
