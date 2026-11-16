// Design: docs/architecture/doctor-and-health-checks.md -- transient PPP admission outcomes
// Related: ../reactor_kernel.go, ../pppoe/server.go -- transport factory refusals

package ppp

import (
	"context"
	"log/slog"
	"maps"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
)

// AdmissionStage identifies the boundary that refused the new PPP lifetime.
// Transport factories and the protocol driver share this one admission class.
type AdmissionStage uint8

const (
	AdmissionUnspecified AdmissionStage = iota
	AdmissionSessionFactory
	AdmissionOwnership
)

func (stage AdmissionStage) String() string {
	switch stage {
	case AdmissionSessionFactory:
		return "session factory"
	case AdmissionOwnership:
		return "session ownership"
	case AdmissionUnspecified:
		panic("BUG: PPP admission stage was not specified")
	default:
		panic("BUG: invalid PPP admission stage")
	}
}

const (
	codePPPAdmission    = "doctor-ppp-session-admission"
	admissionFailureFix = "ze show errors"
	admissionFailureWhy = "The new PPP lifetime was refused, so it cannot carry subscriber traffic. Inspect the recorded host, provider or ownership error and repair that cause before retrying the call."
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: codePPPAdmission, Title: "PPP session admission was refused",
		Description: admissionFailureWhy + " The retained event includes the original refusal and any joined rollback failure. Admission refusal alone does not mean cleanup failed.",
		Examples:    []string{admissionFailureFix, "ze show diagnostic " + codePPPAdmission},
	}); err != nil {
		panic("BUG: PPP admission diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codePPPAdmission}); err != nil {
		panic("BUG: PPP admission event registration failed: " + err.Error())
	}
}

// ReportAdmissionFailure records the actual factory or ownership refusal on the
// bounded report bus. It releases no resource and never replaces the refusing
// boundary's normal teardown. Optional detail is cloned; stage, subject and
// error are always the producer's values. Nil cause is not a failure.
func ReportAdmissionFailure(logger *slog.Logger, subject string, stage AdmissionStage, cause error, detail ...map[string]any) {
	if cause == nil {
		return
	}
	operation := stage.String()
	failure := opererr.Wrap(cause, "PPP "+operation+" refused for "+subject, admissionFailureWhy, admissionFailureFix)
	logger.LogAttrs(context.Background(), slog.LevelWarn, failure.Problem(),
		failure.LogAttrs(slog.String("code", codePPPAdmission), slog.String("subject", subject))...)
	var retained map[string]any
	if len(detail) != 0 {
		retained = maps.Clone(detail[0])
	}
	if retained == nil {
		retained = make(map[string]any)
	}
	retained["stage"], retained["subject"], retained["error"] = operation, subject, cause.Error()
	report.RaiseErrorFix("ppp", codePPPAdmission, diagnosticSubject(subject),
		report.FitMessage(failure.Problem()+". "+failure.Why()+": "+cause.Error()), failure.Fix(), retained)
}
