// Design: docs/architecture/doctor-and-health-checks.md -- recent events: an L2TP hidden AVP left hidden
// Related: hidden.go -- hiddenLeft.log writes the line and calls reportHiddenLeft
// Related: diagnostic_control.go -- the other L2TP event code, registered the same way
package l2tp

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	codeL2TPHiddenAVPUnrevealed = "doctor-l2tp-hidden-avp-unrevealed"
	hiddenAVPWhy                = "Ze could not reveal a hidden AVP the peer sent, so a mandatory one refuses the message and an optional one is ignored."
	// hiddenEventWindow bounds the events one peer raises. The TunnelID=0
	// SCCRQ path is rate-limited before the line is written, but a message on
	// an open tunnel is not, so a peer that hides every AVP under a secret Ze
	// lacks would otherwise raise one event per message and overwrite every
	// other owner's events in the bounded error ring.
	hiddenEventWindow = time.Minute
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code: codeL2TPHiddenAVPUnrevealed, Title: "L2TP hidden AVP could not be revealed",
		Description: hiddenAVPWhy + " The usual cause is a shared secret that is missing or differs from the peer's; a peer that hides a Random Vector, or hides an AVP with no Random Vector before it, violates RFC 2661 Section 4.3. The event names the peer and the fix for its reason.",
		Examples:    []string{"ze show errors", "ze show diagnostic " + codeL2TPHiddenAVPUnrevealed},
	}); err != nil {
		panic("BUG: L2TP hidden-AVP diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeL2TPHiddenAVPUnrevealed}); err != nil {
		panic("BUG: L2TP hidden-AVP event registration failed: " + err.Error())
	}
}

// reportHiddenLeft raises the error event of a message whose AVPs were left
// hidden, under the peer that sent it, with the fix its reason names
// (hiddenRevealFix). The caller MUST pass the nonnil reason it logged. At most
// one event per peer per hiddenEventWindow: the error ring is the ledger.
// Safe for concurrent use.
func reportHiddenLeft(peer netip.AddrPort, reason error, fix string) {
	// A numeric AddrPort has a bounded textual form, below the bus subject
	// limit. Only the reason's text needs fitting.
	destination := peer.String()
	report.RaiseErrorFixOnce(SubsystemName, codeL2TPHiddenAVPUnrevealed, destination,
		report.FitMessage("L2TP peer "+destination+": "+hiddenAVPWhy+" "+reason.Error()), fix, hiddenEventWindow)
}
