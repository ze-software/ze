package rfc

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

const (
	auditHeadUnjudgedRID = "RFC9999-2-3"
	auditHeadExtraCode   = "internal/sample/extra.go"
	auditHeadExtraKey    = auditHeadExtraCode + "::MakeWidget"
)

// auditHeadTree records native verdicts, commits coherent source, then uses
// the ordinary reseal to prepare records against dirty sibling-file edits.
// The saved verdict bytes are the expected committed-source bindings; no test
// manufactures fingerprints or keys.
func auditHeadTree(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	root := resealTwoStemTree(t)
	summary := auditHeadRead(t, root, selftestSummaryRel)
	carrier := auditHeadRead(t, root, resealCarrierFirst)
	writeFixtureFiles(t, root, map[string]string{
		selftestSummaryRel: summary +
			"- [ ] [" + selftestRIDDrop + "] [MUST] A speaker MUST drop the gadget (§2)\n" +
			"- [ ] [" + auditHeadUnjudgedRID + "] [MUST] A speaker MUST keep the widget (§2)\n",
		resealCarrierFirst: carrier + "\nfunc TestSibling() {\n" +
			"// RFC requirement: " + selftestRIDDrop + " positive\n" +
			"// RFC requirement: " + selftestRIDDrop + " negative\n}\n",
		selftestProducerPath:                    selftestProducerSource,
		auditHeadExtraCode:                      "package sample\n\nfunc MakeWidget() int { return 1 }\n",
		"internal/le/test/functional/suites.go": "package functional\nvar Gating = []string{\"plugin\"}\n",
	})
	from := stampPending(t, root, selftestStem, map[string]any{selftestRIDSend: map[string]any{
		"verdict": VerdictWeak, "note": "the fixture judges the row weak",
		fingerprintCode: map[string]any{stampProducerKey: ""},
	}})
	if _, err := auditStamp(root, selftestStem, from, stampModeRejudge); err != nil {
		t.Fatal(err)
	}
	from = stampPending(t, root, selftestStem, map[string]any{selftestRIDDrop: map[string]any{
		"verdict": VerdictWeak, "note": "the sibling is weak",
	}})
	if _, err := auditStamp(root, selftestStem, from, stampModeNew); err != nil {
		t.Fatal(err)
	}
	stems := []string{selftestStem, selftestStemSecond}
	if _, err := resealStems(root, stems); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, root, []string{"init", "-q"})
	commitFixture(t, root, "coherent audited source")
	expected := map[string][]byte{}
	for stem, ids := range map[string][]string{
		selftestStem: {selftestRIDSend, selftestRIDDrop}, selftestStemSecond: {resealRIDSecond},
	} {
		for _, rid := range ids {
			expected[rid] = readAuditRecord(t, root, stem, rid)
		}
	}
	for _, rel := range []string{resealCarrierFirst, resealCarrierSecond} {
		writeFixtureFiles(t, root, map[string]string{rel: "// Private unfinished sibling edit.\n" + auditHeadRead(t, root, rel)})
	}
	if _, err := resealStems(root, stems); err != nil {
		t.Fatal(err)
	}
	return root, expected
}

func auditHeadRead(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func auditHeadAction(t *testing.T, root string, args []string) (any, int) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	return Answer(args)
}

// The real action must bind only its exact ids to committed inputs, despite
// dirty summaries, tag populations, workflows and suite declarations.
func TestResealHeadActionUsesCommittedPopulationAndExactIDs(t *testing.T) {
	root, expected := auditHeadTree(t)
	var compact bytes.Buffer
	if err := json.Compact(&compact, readAuditRecord(t, root, selftestStem, selftestRIDDrop)); err != nil {
		t.Fatal(err)
	}
	writeFixtureFiles(t, root, map[string]string{
		auditRecordRel(selftestStem, selftestRIDDrop): compact.String(),
		selftestSummaryRel:                      "another session's unfinished summary\n",
		selftestWorkflowRel:                     "another session's unfinished workflow\n",
		"internal/le/test/functional/suites.go": "another session's unfinished Go declaration\n",
		resealCarrierFirst: strings.ReplaceAll(auditHeadRead(t, root, resealCarrierFirst),
			"// RFC requirement: "+selftestRIDSend+" negative", "// A tag removed only in the working tree."),
	})
	sibling := readAuditRecord(t, root, selftestStem, selftestRIDDrop)
	other := readAuditRecord(t, root, selftestStemSecond, resealRIDSecond)
	answer, code := auditHeadAction(t, root, []string{
		"reseal", keyStem, selftestStem, keyID, selftestRIDSend, keySource, auditSourceHead,
	})
	if code != 0 {
		t.Fatalf("committed reseal answered %d: %+v", code, answer)
	}
	report, held := answer.(ResealReport)
	if !held {
		t.Fatalf("answer is %T, want ResealReport", answer)
	}
	if report.Revision == "" {
		t.Fatal("committed reseal did not report its source revision")
	}
	if !slices.Equal(report.Resealed, []string{selftestStem + " " + selftestRIDSend}) {
		t.Fatalf("resealed %v, want only the selected id", report.Resealed)
	}
	if !bytes.Equal(readAuditRecord(t, root, selftestStem, selftestRIDSend), expected[selftestRIDSend]) {
		t.Fatal("committed reseal did not restore the exact original source bindings and judgment")
	}
	if !bytes.Equal(readAuditRecord(t, root, selftestStem, selftestRIDDrop), sibling) {
		t.Fatal("an unselected sibling verdict was re-rendered")
	}
	if !bytes.Equal(readAuditRecord(t, root, selftestStemSecond, resealRIDSecond), other) {
		t.Fatal("an unselected stem was written")
	}
	answer, code = auditHeadAction(t, root, []string{
		"reseal", keyStem, selftestStem, keyID, selftestRIDSend, keySource, auditSourceHead,
	})
	if code != 0 {
		t.Fatalf("fresh committed reseal answered %d", code)
	}
	report, held = answer.(ResealReport)
	if !held {
		t.Fatalf("fresh committed reseal answer is %T, want ResealReport", answer)
	}
	if !slices.Equal(report.Fresh, []string{selftestStem + " " + selftestRIDSend}) {
		t.Fatalf("committed freshness reported %v", report.Fresh)
	}
	if len(report.Resealed) != 0 {
		t.Fatal("a fresh committed verdict was rewritten")
	}
}

func TestResealHeadRefusesSelectionErrorsBeforeWriting(t *testing.T) {
	for _, rid := range []string{"RFC9999-2-999", resealRIDSecond, auditHeadUnjudgedRID} {
		t.Run(rid, func(t *testing.T) {
			root, _ := auditHeadTree(t)
			before := readStampAudit(t, root)
			if _, err := resealSelection(root, []string{selftestStem}, []string{selftestRIDSend, rid}, auditSourceHead); err == nil {
				t.Fatal("an unknown, out-of-scope or unjudged id was accepted")
			}
			if !bytes.Equal(before, readStampAudit(t, root)) {
				t.Fatal("a selection error wrote another selected verdict")
			}
		})
	}
}

// A committed change to what was judged must remain stale, including a tag
// deletion. File-only rebinding cannot authorize a changed unit or producer.
func TestResealHeadRefusesChangedJudgments(t *testing.T) {
	cases := []struct {
		name   string
		rel    string
		change func(string) string
	}{
		{"unit", resealCarrierFirst, func(body string) string {
			return strings.Replace(body, "func TestWidget() {", "func TestWidget() {\nprintln(1)", 1)
		}},
		{"producer", selftestProducerPath, func(body string) string { return strings.Replace(body, "return count", "return count + 1", 1) }},
		{"requirement", selftestSummaryRel, func(body string) string { return strings.Replace(body, "MUST send", "MUST resend", 1) }},
		{"tag deletion", resealCarrierFirst, func(body string) string {
			return strings.Replace(body, "// RFC requirement: "+selftestRIDSend+" negative", "// The negative tag was removed.", 1)
		}},
		{"missing producer", selftestProducerPath, func(string) string { return fixtureRemoved }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := auditHeadTree(t)
			body := auditHeadRead(t, root, tc.rel)
			changed := tc.change(body)
			if changed == body {
				t.Fatal("the committed negative control did not change source")
			}
			writeFixtureFiles(t, root, map[string]string{tc.rel: changed})
			commitFixture(t, root, "change the audited subject")
			before := readStampAudit(t, root)
			report, err := resealSelection(root, []string{selftestStem}, []string{selftestRIDSend}, auditSourceHead)
			if err == nil {
				if len(report.Refused) == 0 {
					t.Fatalf("changed judgment was accepted: %+v", report)
				}
			}
			if !bytes.Equal(before, readStampAudit(t, root)) {
				t.Fatal("a changed judgment was mechanically rewritten")
			}
		})
	}
}

// Rejudgment reaches the existing action with from still naming the pending
// file. Requirements, tag keys, units and newly cited code all use HEAD even
// when those working files describe a different unfinished implementation.
func TestAuditStampHeadActionUsesCommittedSources(t *testing.T) {
	root, expected := auditHeadTree(t)
	var original map[string]any
	if err := json.Unmarshal(expected[selftestRIDSend], &original); err != nil {
		t.Fatal(err)
	}
	sibling := readAuditRecord(t, root, selftestStem, selftestRIDDrop)
	writeFixtureFiles(t, root, map[string]string{
		selftestSummaryRel:   "an unfinished summary with no readable Meta table\n",
		resealCarrierFirst:   "package sample\nfunc TestWidget() { println(99) }\n",
		selftestProducerPath: strings.Replace(selftestProducerSource, "return count", "return 0", 1),
		auditHeadExtraCode:   "package sample\nfunc MakeWidget() int { return 99 }\n",
	})
	from := stampPending(t, root, selftestStem, map[string]any{selftestRIDSend: map[string]any{
		"verdict": VerdictWeak, "note": "independently rejudged committed widget assertions",
		fingerprintCode: map[string]any{stampProducerKey: "", auditHeadExtraKey: ""},
	}})
	answer, code := auditHeadAction(t, root, []string{
		"audit-stamp", keyStem, selftestStem, keyFrom, from, keyMode, stampModeWordRejudge, keySource, auditSourceHead,
	})
	if code != 0 {
		t.Fatalf("committed rejudgment answered %d: %+v", code, answer)
	}
	report, held := answer.(AuditStampReport)
	if !held {
		t.Fatalf("answer is %T, want AuditStampReport", answer)
	}
	if report.Revision == "" {
		t.Fatal("committed stamp did not report its source revision")
	}
	if !slices.Equal(report.Stamped, []string{selftestRIDSend}) {
		t.Fatalf("stamped %v, want the pending id alone", report.Stamped)
	}
	audit, err := loadAudit(root, selftestStem)
	if err != nil {
		t.Fatal(err)
	}
	verdict, held := audit.Verdict(selftestRIDSend)
	if !held {
		t.Fatal("the selected verdict disappeared")
	}
	for _, field := range []string{fingerprintTests, fingerprintUnits} {
		if !maps.Equal(recordedMap(verdict, field), recordedMap(original, field)) {
			t.Fatalf("%s was bound to working source rather than the original committed source", field)
		}
	}
	if verdict["requirement_sha"] != original["requirement_sha"] {
		t.Fatal("the requirement was bound to working source")
	}
	codeBindings := recordedMap(verdict, fingerprintCode)
	if codeBindings[stampProducerKey] != recordedMap(original, fingerprintCode)[stampProducerKey] {
		t.Fatal("the existing producer was bound to working source")
	}
	if codeBindings[auditHeadExtraKey] == "" {
		t.Fatal("a newly cited committed producer was not fingerprinted")
	}
	if !bytes.Equal(readAuditRecord(t, root, selftestStem, selftestRIDDrop), sibling) {
		t.Fatal("rejudgment wrote an id outside its pending file")
	}
	snapshot, err := resealInputs(root, resealScope{stems: map[string]bool{selftestStem: true}, head: true})
	if err != nil {
		t.Fatal(err)
	}
	states := auditFreshnessFrom(auditFreshnessInput{
		Requirements: snapshot.collected.Requirements, Tags: snapshot.collected.Tags,
		Enrolled: snapshot.collected.Enrolled, Audits: snapshot.audits,
	}, snapshot.reader)
	if states[selftestRIDSend].State != FreshState {
		t.Fatalf("native committed-source freshness answered %+v", states[selftestRIDSend])
	}
}

// A rejudgment replaces the old producer citations. Reading their deleted
// source first would prevent the native writer from publishing a valid new
// judgment, although its replacement producer is already committed.
func TestAuditStampHeadActionReplacesDeletedProducer(t *testing.T) {
	root, _ := auditHeadTree(t)
	writeFixtureFiles(t, root, map[string]string{selftestProducerPath: fixtureRemoved})
	commitFixture(t, root, "replace the old producing boundary")
	snapshot, err := committedAuditSource(root, map[string]bool{selftestStem: true}, []string{auditHeadExtraCode})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := unitSHAs([]string{auditHeadExtraKey}, snapshot.reader, newScopeIndex(), "committed replacement")
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFiles(t, root, map[string]string{
		auditHeadExtraCode: "package sample\nfunc MakeWidget() int { return 99 }\n",
	})
	dirty, err := unitSHAs([]string{auditHeadExtraKey}, newSourceReader(root), newScopeIndex(), "dirty replacement")
	if err != nil {
		t.Fatal(err)
	}
	if maps.Equal(expected, dirty) {
		t.Fatal("the working replacement does not distinguish its committed source")
	}
	from := stampPending(t, root, selftestStem, map[string]any{selftestRIDSend: map[string]any{
		"verdict": VerdictWeak, "note": "independently rejudged the committed replacement boundary",
		fingerprintCode: map[string]any{auditHeadExtraKey: ""},
	}})
	answer, code := auditHeadAction(t, root, []string{
		"audit-stamp", keyStem, selftestStem, keyFrom, from, keyMode, stampModeWordRejudge, keySource, auditSourceHead,
	})
	if code != 0 {
		t.Fatalf("a committed replacement was refused because its old producer is gone: %d %+v", code, answer)
	}
	audit, err := loadAudit(root, selftestStem)
	if err != nil {
		t.Fatal(err)
	}
	verdict, held := audit.Verdict(selftestRIDSend)
	if !held {
		t.Fatal("the replacement judgment was not written")
	}
	if !maps.Equal(recordedMap(verdict, fingerprintCode), expected) {
		t.Fatal("the replacement judgment retained obsolete citations or fingerprinted dirty source")
	}
}

func TestResealHeadActionIgnoresUnselectedDeletedProducer(t *testing.T) {
	root, _ := auditHeadTree(t)
	writeFixtureFiles(t, root, map[string]string{selftestProducerPath: fixtureRemoved})
	commitFixture(t, root, "remove a producer cited only by an unselected verdict")
	expected := readAuditRecord(t, root, selftestStem, selftestRIDDrop)
	writeFixtureFiles(t, root, map[string]string{
		resealCarrierFirst: "// Another unfinished sibling edit.\n" + auditHeadRead(t, root, resealCarrierFirst),
	})
	if _, err := resealSelection(root, []string{selftestStem}, []string{selftestRIDDrop}, ""); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(readAuditRecord(t, root, selftestStem, selftestRIDDrop), expected) {
		t.Fatal("the selected verdict was not shifted away from committed source")
	}
	sibling := readAuditRecord(t, root, selftestStem, selftestRIDSend)
	answer, code := auditHeadAction(t, root, []string{
		"reseal", keyStem, selftestStem, keyID, selftestRIDDrop, keySource, auditSourceHead,
	})
	if code != 0 {
		t.Fatalf("an unselected obsolete producer blocked the selected reseal: %d %+v", code, answer)
	}
	report, held := answer.(ResealReport)
	if !held {
		t.Fatalf("selected committed reseal answer is %T, want ResealReport", answer)
	}
	if !slices.Equal(report.Resealed, []string{selftestStem + " " + selftestRIDDrop}) {
		t.Fatalf("the selected SHIFTED verdict was not resealed: %+v", report)
	}
	if !bytes.Equal(readAuditRecord(t, root, selftestStem, selftestRIDDrop), expected) {
		t.Fatal("the selected verdict did not regain its committed bindings")
	}
	if !bytes.Equal(readAuditRecord(t, root, selftestStem, selftestRIDSend), sibling) {
		t.Fatal("an unselected verdict with an obsolete producer was written")
	}
}

func TestAuditStampHeadRefusesPartialWithoutWriting(t *testing.T) {
	root := partialStampTree(t)
	stampPartial(t, root)
	writeFixtureFiles(t, root, map[string]string{
		"internal/le/test/functional/suites.go": "package functional\nvar Gating = []string{\"plugin\"}\n",
	})
	gitFixture(t, root, []string{"init", "-q"})
	commitFixture(t, root, "partial judgment with native observations")
	before := readStampAudit(t, root)
	from := stampPending(t, root, selftestStem, map[string]any{selftestRIDSend: partialPending()})
	_, err := auditStampSource(root, selftestStem, from, stampModeRejudge, auditSourceHead)
	if err == nil {
		t.Fatal("committed stamping accepted partial checks that read working source")
	}
	if !strings.Contains(err.Error(), "source head cannot stamp a partial") {
		t.Fatalf("unexpected refusal: %v", err)
	}
	if !bytes.Equal(before, readStampAudit(t, root)) {
		t.Fatal("a refused committed partial judgment wrote its audit")
	}
	// The existing working-tree route still performs all partial checks.
	if _, err := auditStamp(root, selftestStem, from, stampModeRejudge); err != nil {
		t.Fatalf("default partial rejudgment changed: %v", err)
	}
}

func TestAuditHeadRefusesUnreadableAndMalformedCommittedInputs(t *testing.T) {
	t.Run("no commit", func(t *testing.T) {
		root := resealTwoStemTree(t)
		before := readStampAudit(t, root)
		if _, err := resealSelection(root, []string{selftestStem}, nil, auditSourceHead); err == nil {
			t.Fatal("committed reseal accepted a checkout with no commit")
		}
		if !bytes.Equal(before, readStampAudit(t, root)) {
			t.Fatal("an unreadable commit wrote a verdict")
		}
	})
	for name, mutation := range map[string]map[string]string{
		"suite declaration":         {"internal/le/test/functional/suites.go": "package functional\nvar Gating = missing\n"},
		"missing suite declaration": {"internal/le/test/functional/suites.go": fixtureRemoved},
		"malformed tag":             {resealCarrierFirst: "package sample\nfunc TestWidget() {\n// RFC requirement: " + selftestRIDSend + " sideways\n}\n"},
		"unrun tag":                 {"test/not-scheduled/widget.ci": "# RFC requirement: " + selftestRIDSend + " positive\n"},
		"malformed summary":         {selftestSummaryRel: "not a summary\n"},
	} {
		t.Run(name, func(t *testing.T) {
			root, _ := auditHeadTree(t)
			writeFixtureFiles(t, root, mutation)
			commitFixture(t, root, "unreadable committed source")
			before := readStampAudit(t, root)
			if _, err := resealSelection(root, []string{selftestStem}, []string{selftestRIDSend}, auditSourceHead); err == nil {
				t.Fatal("committed reseal accepted malformed or unreadable population inputs")
			}
			if !bytes.Equal(before, readStampAudit(t, root)) {
				t.Fatal("malformed committed source wrote a verdict")
			}
		})
	}
}

func TestAuditSourceSelectorsRefuseUnknownRevisions(t *testing.T) {
	root, _ := auditHeadTree(t)
	before := readStampAudit(t, root)
	from := stampPending(t, root, selftestStem, map[string]any{selftestRIDSend: map[string]any{
		"verdict": VerdictWeak, "note": "independently rejudged assertions",
	}})
	for _, args := range [][]string{
		{"reseal", keyStem, selftestStem, keySource, "working"},
		{"audit-stamp", keyStem, selftestStem, keyFrom, from, keyMode, stampModeWordRejudge, keySource, "working"},
	} {
		if _, code := auditHeadAction(t, root, args); code != 2 {
			t.Fatalf("unknown source answered %d", code)
		}
	}
	if !bytes.Equal(before, readStampAudit(t, root)) {
		t.Fatal("an unknown source wrote a verdict")
	}
}

func TestCommittedAuditSnapshotDoesNotFollowLaterHEAD(t *testing.T) {
	root, _ := auditHeadTree(t)
	snapshot, err := committedAuditSource(root, map[string]bool{selftestStem: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := snapshot.reader.text(resealCarrierFirst)
	writeFixtureFiles(t, root, map[string]string{resealCarrierFirst: "// A later committed edit.\n" + before})
	commitFixture(t, root, "advance HEAD after the snapshot")
	if snapshot.reader.text(resealCarrierFirst) != before {
		t.Fatal("a retained committed reader followed the new HEAD")
	}
	current, known := gitOutput(root, "rev-parse", "HEAD")
	if !known {
		t.Fatal("the fixture cannot read its new HEAD")
	}
	if strings.TrimSpace(string(current)) == snapshot.revision {
		t.Fatal("negative control did not advance HEAD")
	}
	if snapshot.reader.read("internal/sample/working-only.go") != nil {
		t.Fatal("a committed reader accepted a path outside its snapshot")
	}
}
