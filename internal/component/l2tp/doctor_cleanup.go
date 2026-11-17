// Design: docs/guide/monitoring.md -- runtime PPP cleanup diagnostics
// Related: ppp/cleanup_health.go -- the shared transport-neutral report producer

package l2tp

import (
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

const pppCleanupCheckName = "ppp-resource-cleanup"
const pppCleanupHealthName = "ppp-cleanup"

func init() {
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: pppCleanupCheckName, Phase: diagnostic.DoctorPhaseRuntime,
		Order: 720, Component: SubsystemName, Dependencies: []string{"ppp-session-resources"},
		Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes:     []string{diagnostic.CodeDoctorPPPCleanup}, Check: checkPPPCleanup,
	}); err != nil {
		panic("BUG: PPP cleanup doctor registration failed: " + err.Error())
	}
	health.Register(pppCleanupHealthName, report.HealthProbeDegraded(diagnostic.CodeDoctorPPPCleanup))
}

// checkPPPCleanup reads the daemon's retained cleanup failures. Its result does
// not depend on saved configuration: removal can remain unconfirmed after an
// access is disabled. Safe for concurrent use and makes no backend changes.
func checkPPPCleanup(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	var findings []diagnostic.Diagnostic
	for _, issue := range report.Warnings() {
		if issue.Source != ppp.CleanupReportSource {
			continue
		}
		if issue.Code != diagnostic.CodeDoctorPPPCleanup {
			continue
		}
		message := issue.Message
		if cause, found := issue.Detail["error"].(string); found {
			message += ": " + cause
		}
		findings = append(findings, diagnostic.Diagnostic{
			Code: diagnostic.CodeDoctorPPPCleanup, Severity: diagnostic.SeverityWarning,
			Message: message, Fix: ppp.CleanupFailureFix,
		})
	}
	if len(findings) != 0 {
		return findings
	}
	// Evicted state is unknown, never evidence that removal succeeded.
	status, reason := report.HealthProbeDegraded(diagnostic.CodeDoctorPPPCleanup)()
	if status != health.StatusHealthy {
		return []diagnostic.Diagnostic{{
			Code: diagnostic.CodeDoctorPPPCleanup, Severity: diagnostic.SeverityWarning,
			Message: reason, Fix: ppp.CleanupFailureFix,
		}}
	}
	return findings
}
