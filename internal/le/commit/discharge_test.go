// Design: docs/architecture/testing/verify-freshness-scope.md -- a discharge is re-derived, never trusted
package commit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The commit session the fixtures discharge under. It names the record file
// and nothing else, exactly as a debt shard's session names its shard.
const dischargeSession = "cccccccc"

// The two gate names the discharge kinds answer for. Both are declared
// unrunnable, which is why no verification can ever clear a row naming one.
const (
	gateReviewName = "independent critical review"
	gateRFCName    = "owner approval for an RFC-tagged test change"
)

// A gate a verification DOES re-run. A row naming one clears by running it, so
// no discharge answers it whatever evidence the operator holds.
const gateRunnableName = "discovery-index freshness"

const reviewGateArtifact = "| Artifact | `tmp/review/fixture.md` (3 files pinned by SHA-256, verdict clean) |"

// TestDebtDischargeIsReachableFromTheCommitVerbTable drives the verb from the
// command table down to its own parser.
//
// The listing alone would pass against a row nothing dispatches, so the second
// half calls Answer and reads the refusal it prints: a kind outside the closed
// set is a message only parseDischarge writes, and an unknown verb prints
// another. Answer is safe to call here because the refusal happens before any
// ledger is read and before anything is written.
func TestDebtDischargeIsReachableFromTheCommitVerbTable(t *testing.T) {
	if !strings.Contains(Subs(), "debt-discharge") {
		t.Fatalf("le commit lists %q, want debt-discharge among the verbs", Subs())
	}
	listed := false
	for _, row := range listCommands().Verbs {
		if row.Verb == "debt-discharge" {
			listed = row.Writes
		}
	}
	if !listed {
		t.Fatal("the command table does not list debt-discharge as a verb that writes")
	}

	complaint, code := answerStderr(t, "debt-discharge", "kind", "bogus")
	if code != 2 {
		t.Fatalf("debt-discharge kind bogus exited %d, want 2", code)
	}
	for _, kind := range dischargeKinds {
		if !strings.Contains(complaint, kind) {
			t.Errorf("the refusal %q does not list kind %q", complaint, kind)
		}
	}
	if strings.Contains(complaint, "has no verb") {
		t.Fatalf("the refusal %q is the unknown-verb message: nothing dispatches debt-discharge", complaint)
	}
}

// TestDischargeRefusesAnUnknownKindAndWritesNothing pins the closed grammar in
// each of the three shapes AC-1 names, and pins that a refusal writes no file.
func TestDischargeRefusesAnUnknownKindAndWritesNothing(t *testing.T) {
	root := newDischargeRepository(t)
	for _, refusal := range []struct {
		why   string
		args  []string
		names string
	}{
		{"an unknown keyword", []string{"reason", "because"}, "reason"},
		{"a missing value", []string{"shard", "aaaaaaaa.md", "kind"}, "kind"},
		{"a kind outside the set", []string{"kind", "bogus", "shard", "aaaaaaaa.md"}, kindNotApplicable},
		{"no line at all", []string{"kind", kindOwner, "shard", "a.md", "owner", "yes"}, "line"},
		{"a line below one", []string{"kind", kindOwner, "shard", "a.md", "line", "0", "owner", "yes"}, "line"},
		{"a repeated line", []string{"kind", kindOwner, "shard", "a.md", "line", "3", "line", "3", "owner", "yes"}, "more than once"},
		{"a shard holding a separator", []string{"kind", kindOwner, "shard", "../etc/passwd.md", "line", "1", "owner", "y"}, "base name"},
		{"an empty authorisation", []string{"kind", kindOwner, "shard", "a.md", "line", "1", "owner", "  "}, "attests nothing"},
		{"a derived kind with no commit", []string{"kind", kindClosed, "shard", "a.md", "line", "1"}, "commit"},
		{"an artifact leaving the checkout", []string{"kind", kindReviewed, "shard", "a.md", "line", "1",
			"commit", "HEAD", "artifact", "../../etc/passwd"}, "leaves the checkout"},
		{"an absolute artifact", []string{"kind", kindReviewed, "shard", "a.md", "line", "1",
			"commit", "HEAD", "artifact", "/etc/passwd"}, "absolute"},
		{"a commit git would read as an option", []string{"kind", kindClosed, "shard", "a.md",
			"line", "1", "commit", "--help"}, "option"},
		// A keyword the kind never reads would be stored, printed as evidence,
		// and read as the input a verdict rests on. Each kind takes its own.
		{"kind owner carrying a commit", []string{"kind", kindOwner, "shard", "a.md", "line", "1",
			"owner", "yes", "commit", "HEAD"}, "reads no commit"},
		{"kind owner carrying an artifact", []string{"kind", kindOwner, "shard", "a.md", "line", "1",
			"owner", "yes", "artifact", "tmp/review/a.md"}, "reads no artifact"},
		{"kind not-applicable carrying an authorisation", []string{"kind", kindNotApplicable,
			"shard", "a.md", "line", "1", "commit", "HEAD", "owner", "yes"}, "reads no owner"},
		{"kind closed carrying an artifact", []string{"kind", kindClosed, "shard", "a.md",
			"line", "1", "commit", "HEAD", "artifact", "tmp/review/a.md"}, "reads no artifact"},
	} {
		_, err := parseDischarge(refusal.args)
		if err == nil {
			t.Errorf("%s was accepted", refusal.why)
			continue
		}
		if !strings.Contains(err.Error(), refusal.names) {
			t.Errorf("%s answered %q, want the message to name %q", refusal.why, err, refusal.names)
		}
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(debtDir), dischargeDirName)); err == nil {
		t.Fatal("a refused discharge created the record directory")
	}
}

// TestNotApplicableDischargeDerivesTheClosureStem runs the review gate's own
// producer over the commit the operator names, in both polarities.
//
// The refusing polarity is the one with no other tell: a commit that DOES close
// a spec owed the review the row records, so accepting it would discharge a
// real obligation on the operator's word alone.
func TestNotApplicableDischargeDerivesTheClosureStem(t *testing.T) {
	root := newDischargeRepository(t)
	journalCommit := commitFixture(t, root, "a journal row and no spec",
		map[string]string{"plan/journal/a-class.md": "# a class\n"}, nil)
	shard, line := debtRowFor(t, root, "a journal row and no spec", gateReviewName)

	result, code := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", journalCommit)
	if code != 0 {
		t.Fatalf("a commit closing no spec exited %d: %#v", code, result)
	}
	if row := debtRowNow(t, root, shard, line); row.Status != statusDischarged {
		t.Fatalf("the row is %q after a derived discharge, want %q", row.Status, statusDischarged)
	}

	// The other polarity, in its own checkout so the accepted record cannot
	// carry it: this commit removes a spec, so a review WAS owed.
	other := newDischargeRepository(t)
	spec := "plan/immediate/spec-a-thing.md"
	commitFixture(t, other, "the spec lands", map[string]string{spec: specText("in-progress", "")}, nil)
	closure := commitFixture(t, other, "the spec closes", nil, []string{spec})
	shard, line = debtRowFor(t, other, "the spec closes", gateReviewName)

	result, code = discharge(t, other, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", closure)
	if code == 0 {
		t.Fatalf("a commit that closes a spec was discharged as not-applicable: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Refused, " "), "a-thing") {
		t.Fatalf("the refusal %v does not name the stem the derivation found", result.Refused)
	}
	if row := debtRowNow(t, other, shard, line); row.Status != statusOpen {
		t.Fatalf("the row is %q after a refusal, want open", row.Status)
	}
}

// TestNotApplicableDischargeReadsRFCTagCarriersAtTheCommit runs the
// owner-approval gate's own reading over the named commit, in both polarities.
func TestNotApplicableDischargeReadsRFCTagCarriersAtTheCommit(t *testing.T) {
	const tagged = "pkg/thing_test.go"
	// The tag sits INSIDE the unit and the second commit moves the unit's
	// behavior, which is what the gate reads: a comment-only edit changes no
	// tagged unit, so it would prove nothing here.
	const body = "package pkg\n\nfunc TestThing(t *testing.T) {\n" +
		"\t// RFC requirement: rfc9999-1 positive\n\tcheck(1)\n}\n"

	root := newDischargeRepository(t)
	commitFixture(t, root, "the tagged test lands", map[string]string{tagged: body}, nil)
	plain := commitFixture(t, root, "a prose edit only",
		map[string]string{"docs/note.md": "# note\n"}, nil)
	shard, line := debtRowFor(t, root, "a prose edit only", gateRFCName)

	result, code := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", plain)
	if code != 0 {
		t.Fatalf("a commit changing no tagged unit exited %d: %#v", code, result)
	}

	// The refusing polarity: the same file, its tagged function edited.
	changed := commitFixture(t, root, "the tagged test changes", map[string]string{
		tagged: "package pkg\n\nfunc TestThing(t *testing.T) {\n" +
			"\t// RFC requirement: rfc9999-1 positive\n\tcheck(2)\n}\n",
	}, nil)
	shard, line = debtRowFor(t, root, "the tagged test changes", gateRFCName)
	result, code = discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", changed)
	if code == 0 {
		t.Fatalf("a commit changing a tagged unit was discharged: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Refused, " "), "TestThing") {
		t.Fatalf("the refusal %v does not name the unit it found", result.Refused)
	}
}

// TestReviewedDischargeJudgesTheArtifactAgainstTheCommitBytes pins that the
// hashes are compared against the COMMIT, not the working tree.
//
// The middle case is the one a working-tree comparison passes and this one must
// not: the artifact pins the bytes the file holds NOW, which is not what the
// reviewer read at the commit.
func TestReviewedDischargeJudgesTheArtifactAgainstTheCommitBytes(t *testing.T) {
	const code = "pkg/thing.go"
	root := newDischargeRepository(t)
	reviewed := commitFixture(t, root, "the reviewed change",
		map[string]string{code: "package pkg\n\nfunc Thing() {}\n"}, nil)
	shard, line := debtRowFor(t, root, "the reviewed change", gateReviewName)

	artifact := "tmp/review/fixture-clean.md"
	writeCommitFixture(t, root, artifact, reviewArtifact("clean",
		map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}))
	result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindReviewed, "commit", reviewed, "artifact", artifact)
	if exit != 0 {
		t.Fatalf("a clean artifact over the commit's own bytes exited %d: %#v", exit, result)
	}

	// The working tree moves on. An artifact pinned to the NEW bytes covers the
	// path and still says nothing about what the commit carried.
	writeCommitFixture(t, root, code, "package pkg\n\nfunc Thing() { println(1) }\n")
	stale := "tmp/review/fixture-stale.md"
	writeCommitFixture(t, root, stale, reviewArtifact("clean",
		map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}))
	result, exit = discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindReviewed, "commit", reviewed, "artifact", stale)
	if exit == 0 {
		t.Fatalf("an artifact pinned to the working tree discharged the commit: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Refused, " "), code) {
		t.Fatalf("the refusal %v does not name the path whose bytes disagree", result.Refused)
	}

	// A verdict that is not clean, and a code path the artifact never covered.
	for _, bad := range []struct {
		why      string
		verdict  string
		coverage map[string]string
		names    string
	}{
		{"a blocker verdict", "blocker",
			map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}, "not clean"},
		{"an uncovered path", "clean", map[string]string{}, "not covered"},
	} {
		path := "tmp/review/fixture-" + bad.verdict + strconv.Itoa(len(bad.coverage)) + ".md"
		writeCommitFixture(t, root, path, reviewArtifact(bad.verdict, bad.coverage))
		result, exit = discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
			"kind", kindReviewed, "commit", reviewed, "artifact", path)
		if exit == 0 {
			t.Errorf("%s discharged the row: %#v", bad.why, result)
			continue
		}
		if !strings.Contains(strings.Join(result.Refused, " "), bad.names) {
			t.Errorf("%s answered %v, want the refusal to name %q", bad.why, result.Refused, bad.names)
		}
	}
}

// TestReviewedDischargeFallsBackToTheCommittedReviewGate pins the durable half:
// tmp/review/ is untracked and is emptied, so a review recorded months ago has
// no artifact left, and the Review Gate the closure committed is what remains.
func TestReviewedDischargeFallsBackToTheCommittedReviewGate(t *testing.T) {
	root := newDischargeRepository(t)
	spec := "plan/immediate/spec-reviewed-thing.md"
	commitFixture(t, root, "the spec lands",
		map[string]string{spec: specText("in-progress", reviewGateSection("`review check`", "clean"))}, nil)
	closure := commitFixture(t, root, "the reviewed spec closes", nil, []string{spec})
	shard, line := debtRowFor(t, root, "the reviewed spec closes", gateReviewName)

	for _, route := range []struct {
		why  string
		args []string
	}{
		{"no artifact keyword at all", nil},
		{"an artifact path that no longer exists", []string{"artifact", "tmp/review/gone.md"}},
	} {
		args := append([]string{"shard", shard, "line", strconv.Itoa(line),
			"kind", kindReviewed, "commit", closure}, route.args...)
		result, exit := discharge(t, root, args...)
		if exit != 0 {
			t.Errorf("%s exited %d, want the committed Review Gate read instead: %#v", route.why, exit, result)
		}
		removeDischargeRecords(t, root)
	}

	// A commit that removes no spec has no gate to fall back to, and must say so
	// rather than discharge on the operator's word.
	plain := commitFixture(t, root, "an ordinary commit",
		map[string]string{"docs/note.md": "# note\n"}, nil)
	shard, line = debtRowFor(t, root, "an ordinary commit", gateReviewName)
	result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindReviewed, "commit", plain)
	if exit == 0 {
		t.Fatalf("a commit removing no spec discharged through the fallback: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Refused, " "), "removes no spec") {
		t.Fatalf("the refusal %v does not say there is nothing to read", result.Refused)
	}
}

// TestClosedDischargeRequiresARecordedReviewGate reads the gate out of the
// closure commit's parent, over both era spellings of the verdict row and over
// every shape AC-7b refuses.
//
// The two spellings are why the predicate reads the section's ROWS: the label
// is authored prose and tracks neither the recorder nor the date, so a verifier
// matching one spelling reads the other as absent.
func TestClosedDischargeRequiresARecordedReviewGate(t *testing.T) {
	for _, gate := range []struct {
		why     string
		section string
	}{
		{"the older spelling", reviewGateSection("`review_gate.py check`", "clean")},
		{"the newer spelling", reviewGateSection("`review check`", "OK -- review_gate: OK")},
	} {
		root := newDischargeRepository(t)
		spec := "plan/immediate/spec-closed-thing.md"
		commitFixture(t, root, "the spec lands",
			map[string]string{spec: specText("in-progress", gate.section)}, nil)
		closure := commitFixture(t, root, "the spec closes", nil, []string{spec})
		shard, line := debtRowFor(t, root, "the spec closes", gateReviewName)
		result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
			"kind", kindClosed, "commit", closure)
		if exit != 0 {
			t.Errorf("%s exited %d: %#v", gate.why, exit, result)
		}
	}

	for _, refused := range []struct {
		why     string
		section string
		names   string
	}{
		{"an absent gate section", "", "no ## Review Gate"},
		{"the unfilled template", "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
			"| Artifact | [path printed by `review_gate.py record`] |\n" +
			"| `review_gate.py check` | [clean / not run] |\n| Rounds | [N] |\n", "not run"},
		{"a gate recording no review", "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
			"| Artifact | not recorded |\n| `review check` | not run |\n| Rounds | 0 |\n", "not recorded"},
		{"a gate with no rounds count", "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
			reviewGateArtifact + "\n| `review check` | clean |\n", "rounds count"},
	} {
		root := newDischargeRepository(t)
		spec := "plan/immediate/spec-closed-thing.md"
		commitFixture(t, root, "the spec lands",
			map[string]string{spec: specText("in-progress", refused.section)}, nil)
		closure := commitFixture(t, root, "the spec closes", nil, []string{spec})
		shard, line := debtRowFor(t, root, "the spec closes", gateReviewName)
		result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
			"kind", kindClosed, "commit", closure)
		if exit == 0 {
			t.Errorf("%s discharged the row: %#v", refused.why, result)
			continue
		}
		if !strings.Contains(strings.Join(result.Refused, " "), refused.names) {
			t.Errorf("%s answered %v, want the refusal to quote %q", refused.why, result.Refused, refused.names)
		}
	}
}

// TestSkeletonClosureDischargesAndAnImplementedOneDoesNot pins the branch that
// must never be entered from the absence of a gate.
//
// Both halves remove a spec carrying NO Review Gate. Only the Status separates
// them, which is the whole point: an absent gate on a skeleton means there was
// nothing to review, and on an implemented spec it means the review is missing.
func TestSkeletonClosureDischargesAndAnImplementedOneDoesNot(t *testing.T) {
	for _, closure := range []struct {
		status     string
		discharges bool
	}{
		{"skeleton", true},
		{"design", true},
		{"in-progress", false},
		{"verification", false},
		{"ready", false},
	} {
		root := newDischargeRepository(t)
		spec := "plan/immediate/spec-status-thing.md"
		commitFixture(t, root, "the spec lands",
			map[string]string{spec: specText(closure.status, "")}, nil)
		sha := commitFixture(t, root, "the spec closes", nil, []string{spec})
		shard, line := debtRowFor(t, root, "the spec closes", gateReviewName)
		result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
			"kind", kindClosed, "commit", sha)
		if closure.discharges && exit != 0 {
			t.Errorf("a %s spec with no gate exited %d, want it discharged: %#v", closure.status, exit, result)
		}
		if !closure.discharges && exit == 0 {
			t.Errorf("a %s spec with no gate was discharged: %#v", closure.status, result)
		}
	}
}

// TestOwnerDischargeRecordsTheAuthorisationVerbatim pins the attestation route:
// the sentence is stored exactly as it was given, pipes and newlines included,
// and reads back through the ledger unchanged.
func TestOwnerDischargeRecordsTheAuthorisationVerbatim(t *testing.T) {
	const authorisation = "Thomas, 2026-09-08: I ordered this commit | and I reviewed it\nmyself."
	root := newDischargeRepository(t)
	shard, line := debtRowFor(t, root, "the commit the owner ordered", gateReviewName)

	result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindOwner, "owner", authorisation)
	if exit != 0 {
		t.Fatalf("an owner discharge exited %d: %#v", exit, result)
	}
	if result.Record != dischargePath(dischargeSession) {
		t.Fatalf("the record landed at %q, want %q", result.Record, dischargePath(dischargeSession))
	}
	record := readFixtureFile(t, root, result.Record)
	rows := 0
	for line := range strings.SplitSeq(record, "\n") {
		if _, _, isRecord := parseDischargeRow(line); isRecord {
			rows++
		}
	}
	if rows != 1 {
		t.Fatalf("the record holds %d rows, want the one row discharged:\n%s", rows, record)
	}
	if !strings.Contains(record, debtRowDigest(debtRowNow(t, root, shard, line).Raw)) {
		t.Fatalf("the record carries no digest of the row it answers:\n%s", record)
	}

	row := debtRowNow(t, root, shard, line)
	if row.Status != statusDischarged || row.DischargeKind != kindOwner {
		t.Fatalf("the row reads %#v, want a discharged owner row", row)
	}
	if !strings.Contains(row.DischargeEvidence, authorisation) {
		t.Fatalf("the evidence is %q, want the authorisation verbatim", row.DischargeEvidence)
	}
}

// TestDischargedRowsReportDischargedFromOneProducer pins that every consumer
// reads ListDebt and holds no rule of its own.
func TestDischargedRowsReportDischargedFromOneProducer(t *testing.T) {
	root := newDischargeRepository(t)
	shard, line := debtRowFor(t, root, "the discharged commit", gateReviewName)
	if _, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindOwner, "owner", "the owner's sentence"); exit != 0 {
		t.Fatalf("the discharge exited %d", exit)
	}

	open, err := openDebt(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 0 {
		t.Fatalf("openDebt answers %#v, want a discharged row counted as not open", open)
	}
	cleared, err := clearDebtRows(root, map[string]bool{gateReviewName: true})
	if err != nil {
		t.Fatal(err)
	}
	if cleared != 0 {
		t.Fatalf("clearDebtRows rewrote %d discharged row(s), want none: the row is not open", cleared)
	}
	if body := readFixtureFile(t, root, debtPath("aaaaaaaa")); strings.Contains(body, "| "+statusCleared+" |") {
		t.Fatalf("a discharged row was rewritten on disk:\n%s", body)
	}
}

// TestATamperedDischargeRecordLeavesTheRowOpen is the fail-closed guard, driven
// from ListDebt so every consumer inherits it.
//
// Both halves are the shape a hand edit takes: a record whose digest no longer
// matches the row it names, and a record naming a row the ledger does not hold.
func TestATamperedDischargeRecordLeavesTheRowOpen(t *testing.T) {
	root := newDischargeRepository(t)
	shard, line := debtRowFor(t, root, "the discharged commit", gateReviewName)
	if _, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindOwner, "owner", "the owner's sentence"); exit != 0 {
		t.Fatalf("the discharge exited %d", exit)
	}
	if debtRowNow(t, root, shard, line).Status != statusDischarged {
		t.Fatal("the row did not discharge, so the tamper below proves nothing")
	}

	record := dischargePath(dischargeSession)
	original := readFixtureFile(t, root, record)
	for _, tamper := range []struct {
		why   string
		row   string
		names string
	}{
		{"a digest that matches no row",
			strings.Replace(original, debtRowDigest(debtRowNow(t, root, shard, line).Raw),
				strings.Repeat("a", 64), 1), "digest"},
		{"a line the ledger does not hold",
			strings.Replace(original, "| "+strconv.Itoa(line)+" |", "| 9999 |", 1), "no row there"},
	} {
		writeCommitFixture(t, root, record, tamper.row)
		ledger, err := readDebt(root)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for index := range ledger.Rows {
			if ledger.Rows[index].Shard == shard && ledger.Rows[index].Line == line {
				found = ledger.Rows[index].Status == statusOpen
			}
		}
		if !found {
			t.Errorf("%s left the row discharged, want it open", tamper.why)
		}
		if !strings.Contains(strings.Join(ledger.Invalid, " "), tamper.names) {
			t.Errorf("%s was not reported: %v", tamper.why, ledger.Invalid)
		}
	}
}

// TestDischargeVerdictIsNeverReadFromTheRecord invalidates the evidence AFTER
// the record is written and reads the ledger again.
//
// Nothing in the record changes between the two reads. If a verdict had been
// stored, the row would still read discharged, which is exactly the failure a
// committed record makes cheap: a reader trusting a cell whoever last edited
// the file wrote.
func TestDischargeVerdictIsNeverReadFromTheRecord(t *testing.T) {
	const code = "pkg/thing.go"
	root := newDischargeRepository(t)
	reviewed := commitFixture(t, root, "the reviewed change",
		map[string]string{code: "package pkg\n\nfunc Thing() {}\n"}, nil)
	shard, line := debtRowFor(t, root, "the reviewed change", gateReviewName)

	artifact := "tmp/review/fixture-clean.md"
	writeCommitFixture(t, root, artifact, reviewArtifact("clean",
		map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}))
	if _, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindReviewed, "commit", reviewed, "artifact", artifact); exit != 0 {
		t.Fatalf("the discharge exited %d", exit)
	}
	before := readFixtureFile(t, root, dischargePath(dischargeSession))

	// The evidence stops holding: the artifact's verdict is no longer clean.
	writeCommitFixture(t, root, artifact, reviewArtifact("blocker",
		map[string]string{code: reviewHash(filepath.Join(root, filepath.FromSlash(code)))}))
	if row := debtRowNow(t, root, shard, line); row.Status != statusOpen {
		t.Fatalf("the row is %q after its evidence stopped deriving, want open", row.Status)
	}
	if after := readFixtureFile(t, root, dischargePath(dischargeSession)); after != before {
		t.Fatal("the record changed: the row returns to open by DERIVATION, not by an edit")
	}
}

// TestDebtStatusSplitsDischargedByKind pins the answer that makes an attested
// discharge visible beside a derived one, and pins its JSON keys kebab-case.
func TestDebtStatusSplitsDischargedByKind(t *testing.T) {
	root := newDischargeRepository(t)
	plain := commitFixture(t, root, "a prose edit only",
		map[string]string{"docs/note.md": "# note\n"}, nil)
	derivedShard, derivedLine := debtRowFor(t, root, "a prose edit only", gateReviewName)
	attestedShard, attestedLine := debtRowFor(t, root, "the commit the owner ordered", gateReviewName)

	if _, exit := discharge(t, root, "shard", derivedShard, "line", strconv.Itoa(derivedLine),
		"kind", kindNotApplicable, "commit", plain); exit != 0 {
		t.Fatalf("the derived discharge exited %d", exit)
	}
	if _, exit := discharge(t, root, "shard", attestedShard, "line", strconv.Itoa(attestedLine),
		"kind", kindOwner, "owner", "the owner's sentence"); exit != 0 {
		t.Fatalf("the attested discharge exited %d", exit)
	}

	ledger, err := readDebt(root)
	if err != nil {
		t.Fatal(err)
	}
	status := summarizeDebt(ledger)
	if status.Discharged != 2 || status.Open != 0 {
		t.Fatalf("debt-status answers %#v, want 2 discharged and 0 open", status)
	}
	if status.ByKind[kindOwner] != 1 || status.ByKind[kindNotApplicable] != 1 {
		t.Fatalf("the discharged split is %#v, want one of each kind", status.ByKind)
	}
	if text := status.Text(); !strings.Contains(text, "2 discharged") ||
		!strings.Contains(text, kindOwner+" 1") {
		t.Fatalf("debt-status prints %q, want the count and the split", text)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"discharged"`, `"discharged-by-kind"`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("the JSON %s carries no %s key", encoded, key)
		}
	}
}

// TestDischargedDebtRowsCarryTheirKindAndEvidence pins the debt-list half of
// AC-17: a row answers what discharged it, in kebab-case JSON.
func TestDischargedDebtRowsCarryTheirKindAndEvidence(t *testing.T) {
	root := newDischargeRepository(t)
	shard, line := debtRowFor(t, root, "the commit the owner ordered", gateReviewName)
	if _, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindOwner, "owner", "the owner's sentence"); exit != 0 {
		t.Fatalf("the discharge exited %d", exit)
	}
	encoded, err := json.Marshal(debtRowNow(t, root, shard, line))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"discharge-kind":"owner"`, `"discharge-evidence"`, `"status":"discharged"`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("the row JSON %s carries no %s", encoded, key)
		}
	}
}

// newDischargeRepository is a Ze checkout with one commit behind it, which is
// what every derivation below reads.
func newDischargeRepository(t *testing.T) string {
	t.Helper()
	return newCommitRepository(t)
}

// commitFixture writes and removes paths, commits them under the named subject,
// and answers the SHA the discharge names.
//
// The ledger directory is staged with the change, because that is what the
// commit script does: `recordDebt` writes the shard and the same commit carries
// it, so a commit a debt row covers holds the shard in its own file list. The
// fixtures record their rows BEFORE the commit that owes them, for that reason.
func commitFixture(t *testing.T, root, subject string, write map[string]string, remove []string) string {
	t.Helper()
	for path, content := range write {
		writeCommitFixture(t, root, path, content)
		runCommitGit(t, root, "add", "--", path)
	}
	for _, path := range remove {
		runCommitGit(t, root, "rm", "-q", "--", path)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(debtDir))); err == nil {
		runCommitGit(t, root, "add", "--", debtDir)
	}
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", subject)
	return strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
}

// debtRowFor writes one debt row and answers the shard and line the ledger
// gave it, which is the pair an operator types.
func debtRowFor(t *testing.T, root, subject, gate string) (string, int) {
	t.Helper()
	return debtRowRecord(t, root, gate, "the reason "+subject, subject)
}

// debtRowRecord records one owed gate under a fixed reason and answers the row
// the ledger holds for that pair.
//
// Calling it again with the same gate and reason EXTENDS that row rather than
// appending one, which is how a session's second commit reaches a row that
// already exists, and how a fixture builds a row covering several commits. The
// row is found by its gate and reason rather than its subject: an extended
// row's subject cell carries "(+N more)" and no longer equals what was written.
func debtRowRecord(t *testing.T, root, gate, reason, subject string) (string, int) {
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
		if rows[index].Gate == gate && rows[index].Reason == reason {
			return rows[index].Shard, rows[index].Line
		}
	}
	t.Fatalf("the ledger holds no row for gate %q under reason %q", gate, reason)
	return "", 0
}

// discharge drives the verb's own parser and writer, which is the path the
// command takes once Answer has resolved the checkout and the session.
func discharge(t *testing.T, root string, args ...string) (dischargeResult, int) {
	t.Helper()
	request, err := parseDischarge(args)
	if err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return dischargeDebt(root, dischargeSession, request)
}

// debtRowNow answers one row as the ledger reports it, overlay included.
func debtRowNow(t *testing.T, root, shard string, line int) Debt {
	t.Helper()
	rows, err := ListDebt(root)
	if err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	for index := range rows {
		if rows[index].Shard == shard && rows[index].Line == line {
			return rows[index]
		}
	}
	t.Fatalf("the ledger holds no row at %s:%d", shard, line)
	return Debt{}
}

func readFixtureFile(t *testing.T, root, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// removeDischargeRecords empties the record directory between two halves of one
// test, so the second half derives rather than reading the first half's row.
func removeDischargeRecords(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(debtDir), dischargeDirName)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
}

// specText is a spec file carrying a metadata Status and, when one is given, a
// Review Gate section.
func specText(status, gate string) string {
	text := "# Spec: a fixture\n\n| Field | Value |\n|-------|-------|\n| Status | " + status +
		" |\n| Scope | tooling |\n\n## Task\n\nA fixture.\n\n"
	if gate == "" {
		return text
	}
	return text + gate
}

// reviewGateSection renders a recorded Review Gate under one spelling of its
// verdict row. The predicate reads the rows, so the label is a parameter here.
func reviewGateSection(label, verdict string) string {
	return "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
		reviewGateArtifact + "\n| " + label + " | " + verdict + " |\n| Rounds | 2 |\n" +
		"| Reviewer lenses used | logic + wiring |\n"
}

// reviewArtifact renders a ze-review artifact pinning each path to a hash.
func reviewArtifact(verdict string, hashes map[string]string) string {
	var text strings.Builder
	text.WriteString("<!-- ze-review spec=fixture verdict=" + verdict + " -->\n\n")
	for path, hash := range hashes {
		text.WriteString("  " + hash + "  " + path + "\n")
	}
	return text.String()
}

// answerStderr runs one Answer and answers what it printed on stderr, so a
// refusal can be told from another refusal with the same exit code.
func answerStderr(t *testing.T, args ...string) (string, int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stderr.log")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = file

	_, code := Answer(args)

	os.Stderr = saved
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	printed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(printed), code
}

// TestADischargeRefusesACommitTheRowDoesNotName pins the pairing guard.
//
// The debt row carries no SHA, so the operator supplies one, and a derivation
// over the WRONG commit answers truthfully about a commit nobody asked about.
// The accepting polarity is the same row against its own commit, so the guard
// is shown to refuse rather than to refuse everything.
func TestADischargeRefusesACommitTheRowDoesNotName(t *testing.T) {
	root := newDischargeRepository(t)
	mine := commitFixture(t, root, "the row's own commit",
		map[string]string{"docs/mine.md": "# mine\n"}, nil)
	theirs := commitFixture(t, root, "a commit from another line of work",
		map[string]string{"docs/theirs.md": "# theirs\n"}, nil)
	shard, line := debtRowFor(t, root, "the row's own commit", gateReviewName)

	result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", theirs)
	if exit == 0 {
		t.Fatalf("a commit the row does not name discharged it: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Refused, " "), "another line of work") {
		t.Fatalf("the refusal %v does not print the commit's subject beside the row's", result.Refused)
	}

	if _, exit = discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindNotApplicable, "commit", mine); exit != 0 {
		t.Fatalf("the row's own commit was refused too, so the guard refuses everything")
	}
}

// TestADischargeAnswersOnlyAGateNoVerificationRuns pins the check every kind
// owes the ROW, before its own evidence is read.
//
// A discharge exists for a gate no command can run. Where one can, the fact a
// kind derives says nothing about what that gate would report, so accepting it
// opens the push gate on the wrong evidence: one owner sentence would answer a
// row whose freshness gate a verification re-runs in a minute. Every kind is
// driven here, because three of the four never read the row's gate at all.
func TestADischargeAnswersOnlyAGateNoVerificationRuns(t *testing.T) {
	root := newDischargeRepository(t)
	plain := commitFixture(t, root, "a prose edit only",
		map[string]string{"docs/note.md": "# note\n"}, nil)
	runnable, runnableLine := debtRowFor(t, root, "a prose edit only", gateRunnableName)
	undeclared, undeclaredLine := debtRowFor(t, root, "a prose edit only", "a gate nobody declared")
	unrunnable, unrunnableLine := debtRowFor(t, root, "a prose edit only", gateReviewName)

	kinds := [][]string{
		{"kind", kindOwner, "owner", "Thomas ordered this commit"},
		{"kind", kindNotApplicable, "commit", plain},
		{"kind", kindClosed, "commit", plain},
		{"kind", kindReviewed, "commit", plain},
	}
	for _, evidence := range kinds {
		result, exit := discharge(t, root, append([]string{"shard", runnable,
			"line", strconv.Itoa(runnableLine)}, evidence...)...)
		if exit == 0 {
			t.Errorf("%v discharged a row whose gate a verification re-runs: %#v", evidence, result)
			continue
		}
		if !strings.Contains(strings.Join(result.Refused, " "), "debt-clear") {
			t.Errorf("%v answered %v, want the refusal to route the row to debt-clear",
				evidence, result.Refused)
		}
	}
	for _, evidence := range kinds {
		result, exit := discharge(t, root, append([]string{"shard", undeclared,
			"line", strconv.Itoa(undeclaredLine)}, evidence...)...)
		if exit == 0 {
			t.Errorf("%v discharged a row naming a gate debtGates does not declare: %#v",
				evidence, result)
			continue
		}
		if !strings.Contains(strings.Join(result.Refused, " "), "does not declare") {
			t.Errorf("%v answered %v, want the refusal to name the undeclared gate",
				evidence, result.Refused)
		}
	}

	// The accepting polarity, on the same ledger and the same evidence: a gate
	// no verification runs is what a discharge is for.
	if _, exit := discharge(t, root, "shard", unrunnable, "line", strconv.Itoa(unrunnableLine),
		"kind", kindOwner, "owner", "Thomas ordered this commit"); exit != 0 {
		t.Fatalf("an owner discharge over an unrunnable gate exited %d, so the check refuses everything", exit)
	}
}

// TestADischargeAnswersOnlyAnOpenRow pins that a CLEARED row keeps its status.
//
// A cleared row's gate ran green, which is stronger evidence than any discharge
// carries, and the two statuses answer different questions for a reader. A pass
// that overwrote one with the other would report an attested row where the
// ledger holds a verified one (R-3, AC-17).
func TestADischargeAnswersOnlyAnOpenRow(t *testing.T) {
	root := newDischargeRepository(t)
	shard, line := debtRowFor(t, root, "the cleared commit", gateReviewName)
	open, openLine := debtRowFor(t, root, "the open commit", gateReviewName)
	cleared, err := clearDebtRows(root, map[string]bool{gateReviewName: true})
	if err != nil {
		t.Fatal(err)
	}
	if cleared != 2 {
		t.Fatalf("the fixture cleared %d row(s), want both rows cleared before the discharge", cleared)
	}

	result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
		"kind", kindOwner, "owner", "the owner's sentence")
	if exit == 0 {
		t.Fatalf("a cleared row was discharged: %#v", result)
	}
	if !strings.Contains(strings.Join(result.Refused, " "), statusCleared) {
		t.Fatalf("the refusal %v does not say the row is already cleared", result.Refused)
	}

	// The overlay half: a record written by hand for a cleared row must not
	// reclassify it, and must be reported rather than applied in silence.
	row := debtRowNow(t, root, shard, line)
	writeCommitFixture(t, root, dischargePath(dischargeSession),
		"| Date | Shard | Line | Row digest | Kind | Commits | Artifact | Authorisation |\n"+
			"|------|-------|------|------------|------|---------|----------|---------------|\n"+
			"| 2026-09-08 | "+shard+" | "+strconv.Itoa(line)+" | "+debtRowDigest(row.Raw)+
			" | "+kindOwner+" |  |  | the owner's sentence |\n")
	ledger, err := readDebt(root)
	if err != nil {
		t.Fatal(err)
	}
	for index := range ledger.Rows {
		if ledger.Rows[index].Line == line && ledger.Rows[index].Status != statusCleared {
			t.Fatalf("the cleared row reads %q, want it left cleared", ledger.Rows[index].Status)
		}
	}
	if !strings.Contains(strings.Join(ledger.Invalid, " "), statusCleared) {
		t.Fatalf("the record over a cleared row was not reported: %v", ledger.Invalid)
	}

	// The accepting polarity: the same evidence over a row that is open.
	writeCommitFixture(t, root, debtPath("aaaaaaaa"),
		strings.Replace(readFixtureFile(t, root, debtPath("aaaaaaaa")),
			"the open commit | "+gateReviewName+" | the reason the open commit | "+statusCleared,
			"the open commit | "+gateReviewName+" | the reason the open commit | "+statusOpen, 1))
	removeDischargeRecords(t, root)
	if row := debtRowNow(t, root, open, openLine); row.Status != statusOpen {
		t.Fatalf("the fixture row is %q, want it open before the accepting half", row.Status)
	}
	if _, exit := discharge(t, root, "shard", open, "line", strconv.Itoa(openLine),
		"kind", kindOwner, "owner", "the owner's sentence"); exit != 0 {
		t.Fatalf("an open row was refused too, so the check refuses everything")
	}
}

// TestARowCoveringSeveralCommitsAnswersForEachOfThem pins that the evidence
// covers the whole row.
//
// One row covers every commit its session made under the same gate and reason,
// and its subject cell keeps the first commit's subject alone. A discharge
// derived from that one commit says nothing about the others, so the ledger
// would report a row answered while the obligation stands for the rest of it.
func TestARowCoveringSeveralCommitsAnswersForEachOfThem(t *testing.T) {
	root := newDischargeRepository(t)
	const reason = "no spec closes in either commit"
	shard, line := debtRowRecord(t, root, gateReviewName, reason, "the first commit")
	first := commitFixture(t, root, "the first commit",
		map[string]string{"docs/one.md": "# one\n"}, nil)
	debtRowRecord(t, root, gateReviewName, reason, "the second commit")
	second := commitFixture(t, root, "the second commit",
		map[string]string{"docs/two.md": "# two\n"}, nil)
	foreign := commitFixture(t, root, "another line of work",
		map[string]string{"docs/three.md": "# three\n"}, nil)
	at := []string{"shard", shard, "line", strconv.Itoa(line), "kind", kindNotApplicable}

	for _, refusal := range []struct {
		why   string
		args  []string
		names string
	}{
		{"one commit for a row covering two", []string{"commit", first}, "covers 2"},
		{"the same commit named twice", []string{"commit", first, "commit", first}, "named twice"},
		{"a commit from another line of work",
			[]string{"commit", first, "commit", foreign}, "not a commit this row covers"},
	} {
		result, exit := discharge(t, root, append(append([]string{}, at...), refusal.args...)...)
		if exit == 0 {
			t.Errorf("%s discharged the row: %#v", refusal.why, result)
			continue
		}
		if !strings.Contains(strings.Join(result.Refused, " "), refusal.names) {
			t.Errorf("%s answered %v, want the refusal to name %q", refusal.why, result.Refused, refusal.names)
		}
	}

	result, exit := discharge(t, root, append(append([]string{}, at...),
		"commit", first, "commit", second)...)
	if exit != 0 {
		t.Fatalf("both commits of the row were refused: %#v", result)
	}
	row := debtRowNow(t, root, shard, line)
	if row.Status != statusDischarged {
		t.Fatalf("the row is %q once every commit it covers is answered, want discharged", row.Status)
	}
	for _, sha := range []string{first, second} {
		if !strings.Contains(row.DischargeEvidence, sha) {
			t.Errorf("the evidence %q does not name commit %s", row.DischargeEvidence, sha)
		}
	}
}

// TestClosedAndReviewedAnswerTheReviewGateOnly pins the second half of a kind
// reading the row's gate.
//
// Both kinds assert that a REVIEW ran. An owner's approval of an RFC-tagged
// test change is an act no reviewer performs, so a review that ran is true
// evidence about the wrong obligation.
func TestClosedAndReviewedAnswerTheReviewGateOnly(t *testing.T) {
	root := newDischargeRepository(t)
	spec := "plan/immediate/spec-a-thing.md"
	commitFixture(t, root, "the spec lands",
		map[string]string{spec: specText("in-progress", reviewGateSection("`review check`", "clean"))}, nil)
	closure := commitFixture(t, root, "the spec closes", nil, []string{spec})
	rfcShard, rfcLine := debtRowFor(t, root, "the spec closes", gateRFCName)
	reviewShard, reviewLine := debtRowFor(t, root, "the spec closes", gateReviewName)

	for _, kind := range []string{kindClosed, kindReviewed} {
		result, exit := discharge(t, root, "shard", rfcShard, "line", strconv.Itoa(rfcLine),
			"kind", kind, "commit", closure)
		if exit == 0 {
			t.Errorf("kind %s discharged an owner-approval row: %#v", kind, result)
		} else if !strings.Contains(strings.Join(result.Refused, " "), "does not answer gate") {
			t.Errorf("kind %s answered %v, want the refusal to name the gate", kind, result.Refused)
		}

		result, exit = discharge(t, root, "shard", reviewShard, "line", strconv.Itoa(reviewLine),
			"kind", kind, "commit", closure)
		if exit != 0 {
			t.Errorf("kind %s was refused over the review gate too: %#v", kind, result)
		}
		removeDischargeRecords(t, root)
	}
}

// TestAReviewGateArtifactMustNameAFile pins what "a filled artifact reference"
// is, for the two kinds that read a committed Review Gate.
//
// R-8 asks the predicate for a FILLED reference. A cell test that only refuses
// the empty string reads `n/a` and `-` as evidence, and those are exactly what
// an author writes where no review produced anything.
func TestAReviewGateArtifactMustNameAFile(t *testing.T) {
	for _, gate := range []struct {
		why        string
		cell       string
		discharges bool
	}{
		{"a review artifact under tmp/review", "`tmp/review/fixture.md` (3 files, verdict clean)", true},
		{"a dash", "-", false},
		{"an n/a", "n/a", false},
		{"a sentence naming no file", "the reviewer read every hunk", false},
	} {
		root := newDischargeRepository(t)
		spec := "plan/immediate/spec-artifact-thing.md"
		section := "## Review Gate\n\n| Field | Value |\n|-------|-------|\n" +
			"| Artifact | " + gate.cell + " |\n| `review check` | clean |\n| Rounds | 2 |\n"
		commitFixture(t, root, "the spec lands",
			map[string]string{spec: specText("in-progress", section)}, nil)
		closure := commitFixture(t, root, "the spec closes", nil, []string{spec})
		shard, line := debtRowFor(t, root, "the spec closes", gateReviewName)

		result, exit := discharge(t, root, "shard", shard, "line", strconv.Itoa(line),
			"kind", kindClosed, "commit", closure)
		if gate.discharges && exit != 0 {
			t.Errorf("%s exited %d, want the gate read as recorded: %#v", gate.why, exit, result)
		}
		if !gate.discharges {
			if exit == 0 {
				t.Errorf("%s discharged the row: %#v", gate.why, result)
				continue
			}
			if !strings.Contains(strings.Join(result.Refused, " "), "names no file") {
				t.Errorf("%s answered %v, want the refusal to say the artifact names no file",
					gate.why, result.Refused)
			}
		}
	}
}
