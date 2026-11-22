// Design: docs/guide/monitoring.md -- exact native cleanup recovery
// Related: cleanup_health.go -- generic failures never imply recovery
// Related: native_release_budget.go -- retired exact-owner retry

package ppp

import (
	"crypto/sha256"
	"fmt"

	"github.com/ze-software/ze/internal/core/report"
)

// Native session warnings use the factory's validated, stamped birth tag,
// cached on the admitted worker. Interface names/indices can be reused and
// cannot authorize clearing another owner's cleanup failure.
func nativeSessionCleanupSubject(birthTag string) string {
	return fmt.Sprintf("native/%x/session", sha256.Sum256([]byte(birthTag)))
}

func (session *pppSession) reportSessionCleanupFailure(owner SessionHandle, err error) {
	if err == nil {
		return
	}
	if session.nativeBirthTag == "" {
		ReportCleanupFailure(owner.Identity(), CleanupSession, err)
		return
	}
	identity := session.nativeIdentity
	message := fmt.Sprintf("PPP session cleanup failed for interface %s (index %d); removal is unconfirmed",
		identity.InterfaceName, identity.NativeIndex)
	report.RaiseWarningFix(CleanupReportSource, CodeDoctorPPPCleanup,
		nativeSessionCleanupSubject(session.nativeBirthTag), report.FitMessage(message), CleanupFailureFix,
		map[string]any{
			reportFieldInterface: identity.InterfaceName, reportFieldNativeIndex: identity.NativeIndex,
			reportFieldOperation: CleanupSession.String(), "native-birth": session.nativeBirthTag,
			reportFieldError: err.Error(),
		})
}

// Caller MUST have completed this exact retired owner's Close with nil AND
// observed NativeReleasePending=false. Only this native session warning clears;
// generic session, IPv6, transport and network failures remain independent.
func (session *pppSession) clearNativeSessionCleanupFailure() {
	if session.nativeBirthTag == "" {
		return
	}
	report.ClearWarning(CleanupReportSource, CodeDoctorPPPCleanup,
		nativeSessionCleanupSubject(session.nativeBirthTag))
}

// Child warnings share the existing report bus, scoped to the SAME cached
// factory birth and identity. No current interface/owner lookup authorizes clear.
func nativeIPv6ChildCleanupSubject(birth string) string {
	return fmt.Sprintf("native/%x/IPv6", sha256.Sum256([]byte(birth)))
}
func (s *pppSession) reportIPv6ChildCleanupFailure(cause error) {
	if cause == nil {
		return
	}
	if s.driver == nil {
		ReportCleanupFailure(s.dataplane.Identity(), CleanupIPv6, cause)
		return
	}
	identity := s.nativeIdentity
	if s.nativeBirthTag == "" {
		ReportCleanupFailure(identity, CleanupIPv6, cause)
		return
	}
	message := fmt.Sprintf("PPP IPv6 cleanup failed for interface %s (index %d); removal is unconfirmed", identity.InterfaceName, identity.NativeIndex)
	report.RaiseWarningFix(CleanupReportSource, CodeDoctorPPPCleanup,
		nativeIPv6ChildCleanupSubject(s.nativeBirthTag), report.FitMessage(message), CleanupFailureFix, map[string]any{
			reportFieldInterface: identity.InterfaceName, reportFieldNativeIndex: identity.NativeIndex,
			reportFieldOperation: CleanupIPv6.String(), "native-birth": s.nativeBirthTag, reportFieldError: cause.Error(),
		})
}
func (s *pppSession) clearIPv6ChildCleanupFailure() {
	if s.nativeBirthTag == "" {
		return
	} // historical generic warnings never gain a recovery claim
	report.ClearWarning(CleanupReportSource, CodeDoctorPPPCleanup, nativeIPv6ChildCleanupSubject(s.nativeBirthTag))
}
