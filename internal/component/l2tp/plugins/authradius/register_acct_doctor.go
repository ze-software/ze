// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting answers a session-down without waiting for its server
// Related: acct_health.go -- the runtime doctor check that answers these codes
// Related: acct.go -- sendAcctPacket and the counter reads, which log under them

package l2tpauthradius

import "github.com/ze-software/ze/internal/core/diagnostic"

// codeRADIUSAccountingUnacknowledged is the doctor code of an accounting
// record no server acknowledged. This provider is its only emitter, so the
// code and its explanation are declared here and registered from init(),
// never in the core's built-in table (ai/rules/principles.md).
const codeRADIUSAccountingUnacknowledged = "doctor-radius-accounting-unacknowledged"

// codeRADIUSAccountingCounters is the doctor code of an accounting record sent
// without its traffic counters. Declared here for the same reason.
const codeRADIUSAccountingCounters = "doctor-radius-accounting-counters"

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
		Examples: []string{exampleShowDoctor, "ze " + acctHealthFix},
	}); err != nil {
		panic("BUG: RADIUS accounting diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:  codeRADIUSAccountingCounters,
		Title: "RADIUS accounting record sent without traffic counters",
		Description: "A subscriber session's Interim-Update or Accounting-Stop was sent without its " +
			"octet and packet counters, so the operator's billing system has an incomplete record " +
			"for that session. Ze reads the counters only from the PPP unit it created for the " +
			"session, and omits all six counter attributes rather than report a zero it never " +
			"read: a bill can be incomplete, never wrong. The causes are: PPP published no counter " +
			"baseline when the session's network started; the assignment named no network, or that " +
			"network could not be captured; a live read of the PPP unit failed; a reading fell below " +
			"the baseline; or PPP's final measurement did not arrive within 10 seconds of the " +
			"session-down, or arrived unavailable. The finding names the session, its " +
			"Acct-Session-Id and the cause; the log line under this code carries the same. A later " +
			"record of the same session that carries counters clears it, because the counters are " +
			"cumulative. A missing baseline, a reading below it and an uncaptured network last for " +
			"the session, and a Stop without counters stays until provider shutdown. Read the cause " +
			"with `ze show warnings`, then inspect the named session with `ze show l2tp session id " +
			"<id>`.",
		Examples: []string{exampleShowDoctor, "ze show warnings", "ze " + acctCountersFix},
	}); err != nil {
		panic("BUG: RADIUS accounting counters diagnostic registration failed: " + err.Error())
	}
}
