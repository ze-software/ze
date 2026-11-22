// Design: docs/guide/monitoring.md -- retained PPPoE VPP cleanup diagnostics
package vpp

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

const reconciliationSource = "pppoe-vpp"
const reconciliationFix = "show warnings"

// reconciliationErrorKey is the issue detail key that carries the native cause.
const reconciliationErrorKey = "error"

func init() {
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: "pppoe-vpp-reconciliation", Phase: diagnostic.DoctorPhaseRuntime, Order: 755,
		Component: "l2tp", Platforms: []string{diagnostic.DoctorPlatformAny},
		Dependencies: []string{"vpp-startup-pppoe"}, Codes: []string{diagnostic.CodeDoctorPPPoEVPPReconcile}, Check: checkReconciliation,
	}); err != nil {
		panic("BUG: PPPoE VPP reconciliation doctor registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{
		Code: diagnostic.CodeDoctorPPPoEVPPSocketCleanup,
	}); err != nil {
		panic("BUG: PPPoE VPP socket cleanup event registration failed: " + err.Error())
	}
	health.Register("pppoe-vpp-reconciliation", report.HealthProbeDegraded(diagnostic.CodeDoctorPPPoEVPPReconcile))
}

func sessionReconciliationSubject(p *pendingMutation) string {
	return fmt.Sprintf("session/%x/%d", p.request.PeerMAC, p.request.SessionID)
}
func mapReconciliationSubject(key controlMapKey) string {
	var identity uint64
	if key.connector != nil {
		identity = key.connector.Identity()
	}
	return fmt.Sprintf("control/%d/%d/%d/%d", identity, key.generation, key.pair.dataplane, key.pair.control)
}
func reportSessionReconciliation(p *pendingMutation, err error) {
	if err == nil {
		err = errors.New("native session cleanup remains unconfirmed")
	}
	report.RaiseWarningFix(reconciliationSource, diagnostic.CodeDoctorPPPoEVPPReconcile, sessionReconciliationSubject(p),
		"PPPoE VPP session cleanup remains unconfirmed; its subscriber key stays reserved", reconciliationFix,
		map[string]any{"access": p.request.AccessName, "session-id": p.request.SessionID, "client-mac": fmt.Sprintf("%x", p.request.PeerMAC), reconciliationErrorKey: err.Error()})
}
func clearSessionReconciliation(p *pendingMutation) {
	report.ClearWarning(reconciliationSource, diagnostic.CodeDoctorPPPoEVPPReconcile, sessionReconciliationSubject(p))
}
func reportMapReconciliation(key controlMapKey, err error) {
	if err == nil {
		err = errors.New("native control mapping ownership remains unconfirmed")
	}
	report.RaiseWarningFix(reconciliationSource, diagnostic.CodeDoctorPPPoEVPPReconcile, mapReconciliationSubject(key),
		"PPPoE VPP control mapping cleanup remains unconfirmed; its native indices stay reserved", reconciliationFix,
		map[string]any{"data-index": key.pair.dataplane, "control-index": key.pair.control, reconciliationErrorKey: err.Error()})
}
func clearMapReconciliation(key controlMapKey) {
	report.ClearWarning(reconciliationSource, diagnostic.CodeDoctorPPPoEVPPReconcile, mapReconciliationSubject(key))
}

func checkReconciliation(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	var findings []diagnostic.Diagnostic
	for _, issue := range report.Warnings() {
		if issue.Source != reconciliationSource {
			continue
		}
		if issue.Code != diagnostic.CodeDoctorPPPoEVPPReconcile {
			continue
		}
		message := issue.Message
		if cause, ok := issue.Detail[reconciliationErrorKey].(string); ok {
			message += ": " + cause
		}
		findings = append(findings, diagnostic.Diagnostic{Code: diagnostic.CodeDoctorPPPoEVPPReconcile,
			Severity: diagnostic.SeverityWarning, Message: message, Fix: reconciliationFix})
	}
	if len(findings) != 0 {
		return findings
	}
	status, reason := report.HealthProbeDegraded(diagnostic.CodeDoctorPPPoEVPPReconcile)()
	if status != health.StatusHealthy {
		return []diagnostic.Diagnostic{{Code: diagnostic.CodeDoctorPPPoEVPPReconcile, Severity: diagnostic.SeverityWarning, Message: reason, Fix: reconciliationFix}}
	}
	return findings
}
