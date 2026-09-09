// Design: docs/architecture/testing/verify-freshness-scope.md -- a discharge is re-derived, never trusted
// Related: internal/le/commit/discharge_test.go -- the other kinds and their derivations.
package commit

import (
	"strconv"
	"strings"
	"testing"
)

// rfcTagRequirement is the requirement id the fixture's tag names.
//
// The tag text is ASSEMBLED from this constant at run time, so no line of THIS
// file matches rfcTagPattern. A literal tag here would make the file its own
// tag carrier, and the owner-approval gate would then refuse every later edit
// to the tests that prove that gate.
const rfcTagRequirement = "rfc9999-1 positive"

// taggedUnitText renders a test file whose one function carries an RFC
// requirement tag INSIDE its body.
//
// The call in the body is the parameter because the gate reads a unit whose
// TEXT moved. A comment-only edit changes no tagged unit, so it would prove
// nothing here.
func taggedUnitText(call string) string {
	return "package pkg\n\nfunc TestThing(t *testing.T) {\n" +
		"\t// RFC requirement: " + rfcTagRequirement + "\n\t" + call + "\n}\n"
}

// TestNotApplicableDischargeReadsTaggedUnitsAtTheCommitParent runs the
// owner-approval gate's own reading over the named commit, in both polarities.
//
// The refusing polarity is the one with no other tell: a commit that DOES move
// a tagged unit owes the owner approval the row records, so accepting it would
// discharge a real obligation on the operator's word alone.
//
// Each debt row is recorded BEFORE the commit that covers it, which is the
// order Create uses: recordDebt writes the shard and the same commit carries
// it, so dischargeCommits finds the shard among the commit's own files.
func TestNotApplicableDischargeReadsTaggedUnitsAtTheCommitParent(t *testing.T) {
	const tagged = "pkg/thing_test.go"

	root := newDischargeRepository(t)
	commitFixture(t, root, "the tagged test lands",
		map[string]string{tagged: taggedUnitText("check(1)")}, nil)

	shard, line := debtRowFor(t, root, "a prose edit only", gateRFCName)
	plain := commitFixture(t, root, "a prose edit only",
		map[string]string{"docs/note.md": "# note\n"}, nil)

	result, code := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", plain)
	if code != 0 {
		t.Fatalf("a commit changing no tagged unit exited %d: %#v", code, result)
	}
	if row := debtRowNow(t, root, shard, line); row.Status != statusDischarged {
		t.Fatalf("the row is %q after a derived discharge, want %q", row.Status, statusDischarged)
	}

	// The refusing polarity: the same file, its tagged function edited.
	shard, line = debtRowFor(t, root, "the tagged test changes", gateRFCName)
	changed := commitFixture(t, root, "the tagged test changes",
		map[string]string{tagged: taggedUnitText("check(2)")}, nil)

	result, code = discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", changed)
	if code == 0 {
		t.Fatalf("a commit changing a tagged unit was discharged: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Refused, " "), "TestThing") {
		t.Fatalf("the refusal %v does not name the unit it found", result.Refused)
	}
	if row := debtRowNow(t, root, shard, line); row.Status != statusOpen {
		t.Fatalf("the row is %q after a refusal, want open", row.Status)
	}
}

// TestARowBindsNoCommitThatOnlyClearedTheSamePair pins that a commit binds a
// row by OPENING one, in both polarities.
//
// A debt-clear rewrites a row in place, so its diff ADDS a line carrying the
// gate and the reason of the row it cleared. That line parses as a debt row,
// which is everything the binding read once asked for, and the commit that
// wrote it opened no obligation at all. The fixture builds the shape the live
// ledger has never held: a cleared row and an open row under one gate and one
// reason, in one shard.
func TestARowBindsNoCommitThatOnlyClearedTheSamePair(t *testing.T) {
	const reason = "the reason the pair is shared"
	const subject = "the shared subject"

	root := newDischargeRepository(t)
	openRowFor(t, root, gateRFCName, reason, subject)
	commitFixture(t, root, subject, map[string]string{"docs/note.md": "# note\n"}, nil)

	cleared, err := clearDebtRows(root, map[string]bool{gateRFCName: true})
	if err != nil {
		t.Fatalf("clear the first row: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("the debt-clear flipped %d row(s), want 1", cleared)
	}
	clearing := commitFixture(t, root, subject, nil, nil)

	// The gate is owed again. openDebtRowAt never extends a cleared row, so a
	// second row of the same pair opens beside the first.
	shard, line := openRowFor(t, root, gateRFCName, reason, subject)
	opening := commitFixture(t, root, subject, nil, nil)

	result, code := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", clearing)
	if code == 0 {
		t.Fatalf("a commit that only cleared the sibling row bound this one: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Refused, " "), clearing) {
		t.Fatalf("the refusal %v does not name the commit it could not bind", result.Refused)
	}
	if row := debtRowNow(t, root, shard, line); row.Status != statusOpen {
		t.Fatalf("the row is %q after a refusal, want open", row.Status)
	}

	result, code = discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", opening)
	if code != 0 {
		t.Fatalf("the commit that opened the row exited %d: %#v", code, result)
	}
	if row := debtRowNow(t, root, shard, line); row.Status != statusDischarged {
		t.Fatalf("the row is %q after a derived discharge, want %q", row.Status, statusDischarged)
	}
}

// openRowFor records one owed gate and answers the OPEN row the ledger holds
// for that gate and reason.
//
// debtRowFor answers the FIRST row of the pair, which is the CLEARED one once a
// debt-clear has run over the shard, and a cleared row discharges nothing.
func openRowFor(t *testing.T, root, gate, reason, subject string) (string, int) {
	t.Helper()
	owed := []Debt{{Gate: gate, Reason: reason}}
	if _, err := recordDebt(root, "aaaaaaaa", subject, owed); err != nil {
		t.Fatalf("record the debt row: %v", err)
	}
	rows, err := ListDebt(root)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	for index := range rows {
		if rows[index].Status != statusOpen {
			continue
		}
		if rows[index].Gate == gate && rows[index].Reason == reason {
			return rows[index].Shard, rows[index].Line
		}
	}
	t.Fatalf("the ledger holds no open row for gate %q under reason %q", gate, reason)
	return "", 0
}
