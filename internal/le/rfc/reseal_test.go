package rfc

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The second stem's carrier and its one requirement, beside the fixture's own.
const (
	resealRIDSecond      = "RFC8888-2-1"
	resealCarrierFirst   = "internal/sample/widget_test.go"
	resealCarrierSecond  = "internal/sample/gadget_test.go"
	resealShiftingHeader = "// A header another session's edit inserted.\n"
)

// resealTwoStemTree lays two enrolled summaries, rfc9999 and rfc8888, each
// with one row tagged in both polarities by its own Go carrier, and a recorded
// weak verdict for each row. Both carriers are then shifted by one header
// line, so both verdicts sit in the 'shifted' state a reseal acts on.
func resealTwoStemTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFixtureFiles(t, root, map[string]string{
		FixtureAggregateRel: FixtureAggregateProfile,
		selftestWorkflowRel: selftestWorkflow,
		selftestSummaryRel: "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" +
			"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n",
		selftestSummaryRelSecond: "# RFC 8888\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" +
			"- [ ] [" + resealRIDSecond + "] [MUST] A speaker MUST send the widget (§2)\n",
		selftestSourceRel:      checkFixtureSource,
		"rfc/full/rfc8888.txt": checkFixtureSource,
		resealCarrierFirst: "package sample\n\nfunc TestWidget() {\n" +
			"\t// RFC requirement: " + selftestRIDSend + " positive\n" +
			"\t// RFC requirement: " + selftestRIDSend + " negative\n}\n",
		resealCarrierSecond: "package sample\n\nfunc TestGadget() {\n" +
			"\t// RFC requirement: " + resealRIDSecond + " positive\n" +
			"\t// RFC requirement: " + resealRIDSecond + " negative\n}\n",
	})
	for stem, rid := range map[string]string{selftestStem: selftestRIDSend, selftestStemSecond: resealRIDSecond} {
		from := stampPending(t, root, stem, map[string]any{rid: map[string]any{
			"verdict": VerdictWeak, "note": "the fixture judges the row weak",
		}})
		if _, err := auditStamp(root, stem, from, stampModeNew); err != nil {
			t.Fatalf("stamp %s: %v", stem, err)
		}
	}
	for _, carrier := range []string{resealCarrierFirst, resealCarrierSecond} {
		path := filepath.Join(root, filepath.FromSlash(carrier))
		body, err := os.ReadFile(path) // #nosec G304 -- a file inside the test's own temporary tree
		if err != nil {
			t.Fatal(err)
		}
		writeFixtureFiles(t, root, map[string]string{carrier: resealShiftingHeader + string(body)})
	}
	states := stampFreshness(t, root)
	for _, rid := range []string{selftestRIDSend, resealRIDSecond} {
		if states[rid].State != ShiftedState {
			t.Fatalf("the fixture did not shift %s: %s", rid, states[rid].State)
		}
	}
	return root
}

// readAuditRecord answers the bytes of one requirement's verdict file.
func readAuditRecord(t *testing.T, root, stem, rid string) []byte {
	t.Helper()

	body, err := os.ReadFile(auditRecordPath(root, stem, rid))
	if err != nil {
		t.Fatalf("read the verdict of %s: %v", rid, err)
	}
	return body
}

// TestResealWritesOnlyTheStemsItWasGiven proves the scope of `./le rfc reseal`.
//
// Goal: a reseal of stem A re-stamps A's shifted verdict and leaves stem B's
// verdict file byte-identical, although B's verdict is shifted too. Method:
// two stems, both shifted by an unrelated header line, a reseal naming only
// the first, then a byte comparison of the second stem's file and a freshness
// read of both. The reseal of the whole corpus this replaces re-stamped both.
func TestResealWritesOnlyTheStemsItWasGiven(t *testing.T) {
	root := resealTwoStemTree(t)
	firstBefore := readAuditRecord(t, root, selftestStem, selftestRIDSend)
	secondBefore := readAuditRecord(t, root, selftestStemSecond, resealRIDSecond)

	report, err := resealStems(root, []string{selftestStem})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.Resealed, []string{selftestStem + " " + selftestRIDSend}) {
		t.Errorf("the reseal named %v, want only %s %s", report.Resealed, selftestStem, selftestRIDSend)
	}
	if len(report.Refused) != 0 {
		t.Errorf("a stem nobody named reached the refusals: %v", report.Refused)
	}
	if !bytes.Equal(readAuditRecord(t, root, selftestStemSecond, resealRIDSecond), secondBefore) {
		t.Errorf("the reseal of %s rewrote %s's verdict file", selftestStem, selftestStemSecond)
	}
	if bytes.Equal(readAuditRecord(t, root, selftestStem, selftestRIDSend), firstBefore) {
		t.Errorf("the reseal of %s left its own shifted verdict as it was", selftestStem)
	}
	states := stampFreshness(t, root)
	if states[selftestRIDSend].State != FreshState {
		t.Errorf("%s is %s after its own reseal, want fresh", selftestRIDSend, states[selftestRIDSend].State)
	}
	if states[resealRIDSecond].State != ShiftedState {
		t.Errorf("%s is %s, want still shifted: nobody named its stem", resealRIDSecond, states[resealRIDSecond].State)
	}
}

// TestResealRefusesWithoutAStemAndForAStemNotEnrolled proves the reseal never
// falls back to the whole corpus: no stem and an unknown stem both refuse, and
// neither writes a verdict file.
func TestResealRefusesWithoutAStemAndForAStemNotEnrolled(t *testing.T) {
	root := resealTwoStemTree(t)
	before := readStampAudit(t, root)

	if _, err := resealStems(root, nil); err == nil || !strings.Contains(err.Error(), "requires stem") {
		t.Errorf("a reseal naming no stem answered %v, want the refusal naming the stem keyword", err)
	}
	if _, err := resealStems(root, []string{"rfc7777"}); err == nil || !strings.Contains(err.Error(), "rfc7777 is not an enrolled RFC") {
		t.Errorf("a reseal naming an unknown stem answered %v, want the refusal naming it", err)
	}
	if !bytes.Equal(readStampAudit(t, root), before) {
		t.Error("a refused reseal wrote a verdict file")
	}
}
