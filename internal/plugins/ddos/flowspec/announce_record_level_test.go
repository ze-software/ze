// Design: docs/guide/ddos-mitigation.md -- the record of the live announcement
// Related: responder.go (announce, withdrawEarlierRecord)

package flowspec

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

// rewriteFailedFact is the part of the line announce logs when the record write
// fails over a record that already names the announced rule.
const rewriteFailedFact = "could not be recorded again"

// writeFailedFact is the part of the line announce logs when the record write
// fails and no stored record names the announced rule.
const writeFailedFact = "the announced FlowSpec rule could not be recorded, so"

// TestAnnounceOverARecordOfTheSameRuleLogsAFailedWriteAtInfo proves that a
// failed record write is logged at INFO, with its error, when the record an
// earlier failed withdrawal left already names the announced rule.
//
// VALIDATES: the store then holds what a successful write would have left, a
// record of the live rule, so the next start withdraws it and ze doctor reports
// it: nothing is owed to the operator, and a line that reports nothing wrong is
// INFO (ai/rules/user-facing-errors.md).
// METHOD: the dispatcher holds a record of the victim's rule and refuses every
// write; the responder announces that same rule.
// PREVENTS: a WARN with no fix, for a state that needs none.
//
// MUTATION: with the recordsNext arm in announce logging at WARN, the WARN
// count is 1 and the INFO count 0.
func TestAnnounceOverARecordOfTheSameRuleLogsAFailedWriteAtInfo(t *testing.T) {
	logs := captureExitLog(t)
	earlier := buildMatch(ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17})
	disp := &fakeDispatcher{recorded: &earlier}
	r := newResponder(enforcingConfig(), recordRefusingDispatcher{disp})

	announceOnce(t, r, exitTestVictim)

	if warns := logs.atLevel(t, "WARN"); len(warns) != 0 {
		t.Errorf("WARN lines for a failed write over a record of the same rule = %+v, want none", warns)
	}
	var rewrite int
	for _, info := range logs.atLevel(t, "INFO") {
		if strings.Contains(info.Msg, rewriteFailedFact) {
			rewrite++
		}
	}
	if rewrite != 1 {
		t.Errorf("INFO lines saying %q = %d, want 1 (log: %+v)", rewriteFailedFact, rewrite, logs.lines(t))
	}
}

// TestAnnounceWithNoRecordOfTheRuleWarnsOfAFailedWrite proves that a failed
// record write stays a WARN, with the doctor code, when no stored record names
// the announced rule.
//
// VALIDATES: the other half of the pair above: the plugin then cannot confirm
// that any record names the live rule, so a crash or daemon stop may leave it at
// the peers with nothing to withdraw it, and the operator is told.
// METHOD: the dispatcher holds no record and refuses every write; the
// responder announces.
// PREVENTS: the INFO of the same-rule case applied to every failed write.
//
// MUTATION: with the !recordsNext branch in announce logging at INFO, the WARN
// count is 0.
func TestAnnounceWithNoRecordOfTheRuleWarnsOfAFailedWrite(t *testing.T) {
	logs := captureExitLog(t)
	disp := &fakeDispatcher{}
	r := newResponder(enforcingConfig(), recordRefusingDispatcher{disp})

	announceOnce(t, r, exitTestVictim)

	warns := logs.atLevel(t, "WARN")
	if len(warns) != 1 {
		t.Fatalf("WARN lines for a failed write with no record = %d, want 1 (log: %+v)", len(warns), logs.lines(t))
	}
	if !strings.Contains(warns[0].Msg, writeFailedFact) {
		t.Errorf("WARN %q does not say %q", warns[0].Msg, writeFailedFact)
	}
	if warns[0].Code != orphanTestCode {
		t.Errorf("WARN code = %q, want %q", warns[0].Code, orphanTestCode)
	}
}
