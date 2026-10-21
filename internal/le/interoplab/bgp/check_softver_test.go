// Design: docs/architecture/testing/interop.md — frr-software-version checks ze's WARN for a refused OPEN
// Related: check_softver.go — openRefusedWarnLine, requireZeWarnsOpenRefused

package bgp

import "testing"

// TestOpenRefusedWarnLine pins what the frr-software-version scenario accepts
// as ze's WARN for FRR refusing the draft-form OPEN (AC-3 of
// spec-bgp-open-session-rfc-defects).
//
// VALIDATES: a `notification received` WARN for the FRR peer with code 2/0 is
// accepted only when its state is OPENSENT or OPENCONFIRM; the same line with
// state ESTABLISHED, with no state, at INFO, for another peer, or with another
// code is refused.
// PREVENTS: the scenario passing on a NOTIFICATION that reached a session which
// came up, which proves nothing about a refused OPEN.
//
// Method: the accepted line is the shape Session.logNotificationReceived
// writes (as test/plugin/open-refused-notification-warn.ci matches it); each
// refused line changes one attribute of it.
func TestOpenRefusedWarnLine(t *testing.T) {
	const peerAttr = "peer=fd00:1e::3 "
	line := func(level, peer, state, codes string) string {
		return "time=2026-10-21T04:00:00Z level=" + level + ` msg="notification received"` +
			" subsystem=bgp.reactor.session peer=" + peer + state + " " + codes
	}
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"OpenSent", line("WARN", "fd00:1e::3", " state=OPENSENT", openRefusalCodes), true},
		{"OpenConfirm", line("WARN", "fd00:1e::3", " state=OPENCONFIRM", openRefusalCodes), true},
		{"Established", line("WARN", "fd00:1e::3", " state=ESTABLISHED", openRefusalCodes), false},
		{"no state", line("WARN", "fd00:1e::3", "", openRefusalCodes), false},
		{"INFO level", line("INFO", "fd00:1e::3", " state=OPENCONFIRM", openRefusalCodes), false},
		{"another peer", line("WARN", "fd00:1e::4", " state=OPENCONFIRM", openRefusalCodes), false},
		{"another code", line("WARN", "fd00:1e::3", " state=OPENCONFIRM",
			`code=Cease code-number=6 subcode="Administrative Shutdown" subcode-number=2`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := openRefusedWarnLine(tc.line, peerAttr); got != tc.want {
				t.Errorf("openRefusedWarnLine(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}
