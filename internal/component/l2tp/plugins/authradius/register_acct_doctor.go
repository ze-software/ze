// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting answers a session-down without waiting for its server
// Related: acct_health.go -- the runtime doctor check that answers this code
// Related: acct.go -- sendAcctPacket, which logs an unacknowledged record under it

package l2tpauthradius

import "github.com/ze-software/ze/internal/core/diagnostic"

// codeRADIUSAccountingUnacknowledged is the doctor code of an accounting
// record no server acknowledged. This provider is its only emitter, so the
// code and its explanation are declared here and registered from init(),
// never in the core's built-in table (ai/rules/principles.md).
const codeRADIUSAccountingUnacknowledged = "doctor-radius-accounting-unacknowledged"

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:  codeRADIUSAccountingUnacknowledged,
		Title: "RADIUS accounting record not acknowledged",
		Description: "No configured RADIUS accounting server acknowledged a subscriber session's " +
			"Accounting-Start, Interim-Update or Accounting-Stop within the 10 second record bound, " +
			"retransmissions included, so the operator's billing system may lack that record. " +
			"The finding names the session and its Acct-Session-Id. A later acknowledged record of " +
			"the same session clears it; an unacknowledged Stop stays until provider shutdown. " +
			"Restore the accounting server's reachability and shared key.",
		Examples: []string{"ze show doctor", "ze " + acctHealthFix},
	}); err != nil {
		panic("BUG: RADIUS accounting diagnostic registration failed: " + err.Error())
	}
}
