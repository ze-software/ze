// Design: docs/architecture/doctor-and-health-checks.md -- source authority admission.
// Related: source_session.go -- a failed capture refuses session publication.
package reactor

import (
	"math"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const codeSourceReceiptAdmission = "doctor-bgp-source-receipt-admission"
const sourceReceiptRestartFix = "ze request restart"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeSourceReceiptAdmission,
		Title:       "BGP source session authority could not be captured",
		Description: "Ze could not capture the negotiated facts or allocate an identity for a BGP source session. The session is closed before its UP is published. Inspect the recorded cause; exhausted identities require restarting Ze with ze request restart.",
		Examples:    []string{sourceReceiptRestartFix, "ze show diagnostic " + codeSourceReceiptAdmission},
	}); err != nil {
		panic("BUG: BGP source receipt diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeSourceReceiptAdmission}); err != nil {
		panic("BUG: BGP source receipt event registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: "bgp-source-receipt-issuer", Phase: diagnostic.DoctorPhaseRuntime, Order: 735,
		Component: reportSourceBGP, Dependencies: []string{"bgp-source-sessions"},
		Platforms: []string{diagnostic.DoctorPlatformAny}, Codes: []string{codeSourceReceiptAdmission},
		Check: func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			return sourceReceipts.sourceReceiptDiagnostics()
		},
	}); err != nil {
		panic("BUG: BGP source receipt doctor registration failed: " + err.Error())
	}
}

// sourceReceiptDiagnostics reads only the issuer's live authority clock. A
// Reactor restart cannot repair exhaustion because it does not reset that clock.
func (store *sourceReceiptStore) sourceReceiptDiagnostics() []diagnostic.Diagnostic {
	store.mu.Lock()
	exhausted := store.next == math.MaxUint64
	store.mu.Unlock()
	if !exhausted {
		return nil
	}
	return []diagnostic.Diagnostic{{Code: codeSourceReceiptAdmission, Severity: diagnostic.SeverityError,
		Message: "BGP source session identities are exhausted; no new source session can be admitted until Ze restarts.",
		Fix:     sourceReceiptRestartFix}}
}

// reportSourceReceiptAdmissionFailure records the source's exact refusal in
// the existing bounded event bus. The caller MUST close before publishing UP.
func reportSourceReceiptAdmissionFailure(peer *Peer, cause error) {
	message := report.FitMessage("BGP source session authority could not be captured; Ze closes this connection before announcing it: " + cause.Error())
	report.RaiseErrorFix(reportSourceBGP, codeSourceReceiptAdmission, peer.peerAddrLabel(), message,
		sourceReceiptRestartFix)
	peerLogger().Error(message, "peer", peer.peerAddrLabel(), "code", codeSourceReceiptAdmission,
		"fix", sourceReceiptRestartFix)
}
