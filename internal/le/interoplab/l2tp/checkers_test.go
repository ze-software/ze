// Design: docs/labs/l2tp-interop.md -- what scenario 04 proves about the accounting records
// Related: checkers.go -- checkAccountingAttributes, the assertion under test
package l2tp

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// accountingLine builds one decoded RADIUS-RX line of the shape radiusmock
// writes, with a fresh Event-Timestamp so the drift bound is satisfied.
func accountingLine(status string, extra ...string) string {
	fields := []string{
		"RADIUS-RX Accounting-Request",
		"Acct-Status-Type=" + status,
		"Acct-Session-Id=sess-1",
		"NAS-Port-Id=lns1:12.34",
		"Event-Timestamp=" + strconv.FormatInt(time.Now().Unix(), 10),
		"Acct-Delay-Time=0",
	}
	return strings.Join(append(fields, extra...), " ")
}

// VALIDATES: checkAccountingAttributes accepts the records ze is expected to
// send in scenario 04, and refuses each way one can be wrong.
// PREVENTS: the scenario passing while an attribute is missing, while a Stop
// reports the wrong cause, or while Acct-Terminate-Cause rides on a record RFC
// 2866 Section 5.10 forbids it on. The absence rows are the ones that catch an
// append made in the wrong place.
func TestCheckAccountingAttributes(t *testing.T) {
	cases := []struct {
		name   string
		record string
		line   string
		cause  string
		refuse string
	}{
		{
			name:   "start carries the attributes and no cause",
			record: "Accounting-Start",
			line:   accountingLine("Start"),
		},
		{
			name:   "stop carries the cause the operator teardown names",
			record: "Accounting-Stop",
			line:   accountingLine("Stop", "Acct-Terminate-Cause=6"),
			cause:  terminateCauseAdminReset,
		},
		{
			name:   "start carrying a cause is refused",
			record: "Accounting-Start",
			line:   accountingLine("Start", "Acct-Terminate-Cause=6"),
			refuse: "Stop record alone",
		},
		{
			name:   "stop reporting another cause is refused",
			record: "Accounting-Stop",
			line:   accountingLine("Stop", "Acct-Terminate-Cause=1"),
			cause:  terminateCauseAdminReset,
			refuse: "reports Acct-Terminate-Cause 1",
		},
		{
			name:   "stop with no cause is refused",
			record: "Accounting-Stop",
			line:   accountingLine("Stop"),
			cause:  terminateCauseAdminReset,
			refuse: "Acct-Terminate-Cause missing",
		},
		{
			name:   "a calling station id is refused, because xl2tpd names no calling number",
			record: "Accounting-Start",
			line:   accountingLine("Start", "Calling-Station-Id=+441632960123"),
			refuse: "xl2tpd sends no Calling Number AVP",
		},
		{
			name:   "a missing Event-Timestamp is refused",
			record: "Accounting-Start",
			line:   "RADIUS-RX Accounting-Request Acct-Status-Type=Start Acct-Delay-Time=0",
			refuse: "Event-Timestamp missing",
		},
		{
			name:   "an Event-Timestamp far from the lab clock is refused",
			record: "Accounting-Start",
			line:   "RADIUS-RX Accounting-Request Acct-Status-Type=Start Event-Timestamp=0 Acct-Delay-Time=0",
			refuse: "from the lab clock",
		},
		{
			name:   "a missing Acct-Delay-Time is refused",
			record: "Accounting-Start",
			line:   "RADIUS-RX Accounting-Request Acct-Status-Type=Start Event-Timestamp=" + strconv.FormatInt(time.Now().Unix(), 10),
			refuse: "Acct-Delay-Time missing",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkAccountingAttributes(testCase.record, testCase.line, testCase.cause)
			if testCase.refuse == "" {
				if err != nil {
					t.Fatalf("checkAccountingAttributes refused a good record: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("checkAccountingAttributes accepted %s", testCase.line)
			}
			if !strings.Contains(err.Error(), testCase.refuse) {
				t.Fatalf("refusal %q does not name %q", err, testCase.refuse)
			}
		})
	}
}
