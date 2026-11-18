// Design: docs/architecture/traffic/cos-dynamic.md -- daemon-visible CoS failures
// Related: diagnostic_shared.go -- original-owner warning and policy event producers

package cos

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	codeCoSCleanupUnconfirmed = "doctor-cos-cleanup-unconfirmed"
	codeCoSPolicyRefused      = "doctor-cos-policy-refused"
	cleanupFix                = "ze request class-of-service cleanup"
	policyRefusalFix          = "ze show class-of-service"
	cleanupWhy                = "The original access maps are not confirmed restored. The original owner remains retained for another cleanup attempt."
	policyRefusalWhy          = "The requested subscriber policy was not installed. Inspect the profiles and correct the conflicting RADIUS assignment or profile before retrying; cleanup does not remove a healthy subscriber's policy."
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeCoSCleanupUnconfirmed,
		Title:       "Class-of-service cleanup is not confirmed",
		Description: cleanupWhy + " The warning records the original access and native failure. Run the cleanup command to retry failed original owners; a new session or a replacement interface is not recovery evidence.",
		Examples:    []string{cleanupFix, "ze show diagnostic " + codeCoSCleanupUnconfirmed},
	}); err != nil {
		panic("BUG: class-of-service cleanup diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeCoSCleanupUnconfirmed}); err != nil {
		panic("BUG: class-of-service cleanup event registration failed: " + err.Error())
	}
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeCoSPolicyRefused,
		Title:       "Class-of-service policy was refused",
		Description: policyRefusalWhy + " This is an error-history event, not evidence that a healthy or pending policy needs cleanup.",
		Examples:    []string{policyRefusalFix, "ze show diagnostic " + codeCoSPolicyRefused},
	}); err != nil {
		panic("BUG: class-of-service policy diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeCoSPolicyRefused}); err != nil {
		panic("BUG: class-of-service policy event registration failed: " + err.Error())
	}
}

// publishDaemonCoSIssue is bound only by ConfigureEventBus, which runs inside
// the daemon. A forked runner MUST instead bind its SDK report-issue publisher;
// this function is never a fallback for an unavailable transport. Validation
// precedes every write because the report bus otherwise rejects bad fields
// without returning an error. For retained cleanup subjects this callback
// acknowledges observation only: the owning access separately requires its
// actual ResourceWarningEpisode Publish/Clear. Ordinary writes to that protected
// subject are no-ops and cannot supply resource custody. Policy history remains
// ordinary. Safe for concurrent use.
func publishDaemonCoSIssue(in rpc.ReportIssueInput) error {
	if err := rpc.ValidateReportIssue(&in); err != nil {
		return err
	}
	switch in.Code {
	case codeCoSCleanupUnconfirmed:
		if in.Action == rpc.ReportClear {
			report.ClearWarning(Name, codeCoSCleanupUnconfirmed, in.Subject)
			return nil
		}
		if in.Severity != rpc.ReportWarning {
			return fmt.Errorf("class-of-service cleanup report requires warning severity, got %q", in.Severity)
		}
		report.RaiseWarningFix(Name, codeCoSCleanupUnconfirmed, in.Subject, in.Message, in.Fix)
	case codeCoSPolicyRefused:
		if in.Action != rpc.ReportRaise {
			return errors.New("class-of-service policy refusal is a history event and cannot be cleared")
		}
		if in.Severity != rpc.ReportError {
			return fmt.Errorf("class-of-service policy refusal requires error severity, got %q", in.Severity)
		}
		report.RaiseErrorFix(Name, codeCoSPolicyRefused, in.Subject, in.Message, in.Fix)
	default: // Open code set at the reporting boundary; this owner accepts its two codes only.
		return fmt.Errorf("class-of-service report code %q is not owned by this publisher", in.Code)
	}
	return nil
}
