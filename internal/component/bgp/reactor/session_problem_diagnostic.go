// Design: docs/architecture/behavior/fsm-open-sent.md -- connection refusal and strict-BFD expiry history.
// Related: docs/architecture/doctor-and-health-checks.md -- recent-events reads the bounded report bus.
// Related: peer_stats.go -- unselected connections do not own the incumbent's last error.

package reactor

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codeOpenConnectionRefused   = "doctor-bgp-open-connection-refused"
	openConnectionRefusalFix    = "ze show bgp peer list"
	openIdentifierRefusalAction = "configure a nonzero BGP Identifier on the peer; for iBGP it must differ from this speaker's identifier"
	codeBFDWaitHoldExpired      = "doctor-bgp-bfd-wait-hold-expired"
	bfdWaitHoldExpiryFix        = "ze show bfd sessions"
	codeDoctorBGPBoRRBeforeEOR  = "doctor-bgp-borr-before-eor"
)

// init declares connection history through the existing recent-events check.
// These events describe one refused or expired connection, never the current
// state of another connection selected for the same peer address.
func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeOpenConnectionRefused,
		Title:       "BGP connection refused during OPEN validation",
		Description: "Ze refused a connection because the peer AS does not match its prepared configuration or the BGP Identifier is unacceptable. The event records that connection only; another selected session can remain healthy. Check the event's advertised and local values, correct the AS configuration or the peer's BGP Identifier, and check the current peer state with ze show bgp peer list.",
		Examples:    []string{openConnectionRefusalFix, "ze show diagnostic " + codeOpenConnectionRefused},
	}); err != nil {
		panic("BUG: bgp reactor: OPEN refusal diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeOpenConnectionRefused}); err != nil {
		panic("BUG: bgp reactor: OPEN refusal event registration failed: " + err.Error())
	}
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeBFDWaitHoldExpired,
		Title:       "BGP hold timer expired during a strict-mode BFD wait",
		Description: "The BGP hold timer expired while a connection waited for BFD or its hold-down interval. Ze ends this connection; the event does not establish that another selected session is down. Check BFD state with ze show bfd sessions, then correct BFD reachability or the configured hold-down interval if it cannot finish before the BGP hold timer expires.",
		Examples:    []string{bfdWaitHoldExpiryFix, "ze show diagnostic " + codeBFDWaitHoldExpired},
	}); err != nil {
		panic("BUG: bgp reactor: BFD wait expiry diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeBFDWaitHoldExpired}); err != nil {
		panic("BUG: bgp reactor: BFD wait expiry event registration failed: " + err.Error())
	}
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeDoctorBGPBoRRBeforeEOR,
		Title:       "BGP BoRR marker ignored before End-of-RIB from Graceful Restart peer",
		Description: "A neighbor that advertised the Graceful Restart Capability sent a Beginning-of-Route-Refresh (BoRR) marker for an address family before sending its End-of-RIB (EoR) marker for that family. Per RFC 7313 Section 4, Ze ignored the BoRR and kept the session Established.",
		Examples:    []string{"ze show bgp peer list", "ze show diagnostic " + codeDoctorBGPBoRRBeforeEOR},
	}); err != nil {
		panic("BUG: bgp reactor: BoRR-before-EoR diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPBoRRBeforeEOR}); err != nil {
		panic("BUG: bgp reactor: BoRR-before-EoR event registration failed: " + err.Error())
	}
}

// reportOpenConnectionRefusal retains the cause independently of notification
// delivery and selected-session statistics. The caller MUST still enforce its
// refusal. This method changes no session or peer state and takes no peer lock.
func (s *Session) reportOpenConnectionRefusal(reason, action string, detail map[string]any) {
	report.RaiseErrorFix(reportSourceBGP, codeOpenConnectionRefused, s.settings.Address.String(),
		report.FitMessage("BGP OPEN refused: "+reason+". Ze closes this connection; "+action+"."),
		openConnectionRefusalFix, detail)
}

// reportBFDWaitHoldExpiry retains the wait that Event10 terminates before its
// timers and sub-state are cleared. The caller MUST own openMu. This method
// changes no state and acquires no session or peer lock.
func (s *Session) reportBFDWaitHoldExpiry(wait string) {
	report.RaiseErrorFix(reportSourceBGP, codeBFDWaitHoldExpired, s.settings.Address.String(),
		"BGP hold timer expired while "+wait+"; Ze ends this connection. Check BFD reachability and the configured hold-down interval.",
		bfdWaitHoldExpiryFix, map[string]any{"wait": wait, "hold-time": s.timers.HoldTime(),
			"hold-down": s.timers.BfdHoldDown(), "bfd-sub-state": s.fsm.BfdSubState().String()})
}
