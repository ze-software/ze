package rfc

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// stampProducerKey is the producer an unimplemented verdict cites in the fixture.
const stampProducerKey = selftestProducerPath + "::SendWidget"

// stampPendingRel is where the fixture's pending verdicts live, outside rfc/audit/.
const stampPendingRel = "scratch/pending/rfc9999.json"

// stampNow is the date the fixture stamps on.
var stampNow = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

// stampTree lays a two-row enrolled summary, a carrier tagging the first row in
// both polarities, and a producer. An audit file is written only when recorded
// verdicts are given, in the writer's own canonical form so a byte comparison
// means something.
func stampTree(t *testing.T, recorded map[string]any) string {
	t.Helper()

	files := map[string]string{
		selftestWorkflowRel: selftestWorkflow,
		selftestSummaryRel: "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" +
			"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n" +
			"- [ ] [" + selftestRIDDrop + "] [MUST] A speaker MUST drop the gadget (§2)\n",
		"rfc/full/rfc9999.txt": checkFixtureSource,
		"feature-gates.txt":    "ze_widget  internal/widget\n",
		selftestCIPath:         fixtureWidgetCI,
		selftestProducerPath:   selftestProducerSource,
	}
	if recorded != nil {
		files[auditRel+"/rfc9999.json"] = pyDump(map[string]any{
			"rfc": "rfc9999", "audited": "2026-09-01", "requirements": recorded,
		}) + "\n"
	}
	root := t.TempDir()
	writeFixtureFiles(t, root, files)
	return root
}

// stampPending writes a pending file naming the given stem and verdicts, and
// answers its path.
func stampPending(t *testing.T, root, stem string, verdicts map[string]any) string {
	t.Helper()

	writeFixtureFiles(t, root, map[string]string{
		stampPendingRel: pyDump(map[string]any{"rfc": stem, "requirements": verdicts}),
	})
	return filepath.Join(root, filepath.FromSlash(stampPendingRel))
}

// readStampAudit answers the audit file's bytes, and nil when it is absent.
func readStampAudit(t *testing.T, root string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(auditRel), "rfc9999.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read the audit file: %v", err)
	}
	return raw
}

// stampFreshness answers the state `./le rfc check` derives for every verdict
// of the tree, through the same load and the same derivation it uses.
func stampFreshness(t *testing.T, root string) map[string]Freshness {
	t.Helper()

	collected, err := Collect(root)
	if err != nil {
		t.Fatalf("collect the fixture: %v", err)
	}
	audits, err := loadAudits(root, collected.Enrolled)
	if err != nil {
		t.Fatalf("the stamped audit file does not load: %v", err)
	}
	return auditFreshness(auditFreshnessInput{
		Tree: root, Requirements: collected.Requirements, Tags: collected.Tags,
		Enrolled: collected.Enrolled, Audits: audits,
	})
}

// VALIDATES: verdicts an author has just judged, carrying only the verdict, the
// note and the producer keys they cite, are stamped with the requirement's
// fingerprint, one tests and units entry per tag, and a fingerprint for each
// cited producer, into an audit file the command creates, and `./le rfc check`
// then judges them fresh.
// METHOD: stamp an enforced verdict over a row tagged twice in one .ci file and
// an unimplemented verdict over an untagged row citing one producer, into a tree
// with no audit file, then load it through the validating reader and derive
// freshness the way the gate does.
// PREVENTS: fingerprints computed by hand, which the audit skill forbids, and a
// stamp that writes something the gate then reads as stale.
func TestAuditStampFillsNewVerdictsAndTheyReadFresh(t *testing.T) {
	root := stampTree(t, nil)
	from := stampPending(t, root, "rfc9999", map[string]any{
		selftestRIDSend: map[string]any{"verdict": VerdictEnforced, "note": "both polarities assert the widget"},
		selftestRIDDrop: map[string]any{
			"verdict": VerdictUnimplemented, "note": "SendWidget never drops a gadget",
			fingerprintCode: map[string]any{stampProducerKey: ""},
		},
	})

	report, err := auditStamp(root, "rfc9999", from, stampNow)
	if err != nil {
		t.Fatalf("stamp refused: %v", err)
	}
	if want := []string{selftestRIDSend, selftestRIDDrop}; !slices.Equal(report.Stamped, want) {
		t.Fatalf("stamped %v, want %v", report.Stamped, want)
	}

	audit, err := loadAudit(root, "rfc9999")
	if err != nil {
		t.Fatalf("the stamped file does not load: %v", err)
	}
	if audit.Document["rfc"] != "rfc9999" || audit.Document["audited"] != "2026-09-26" {
		t.Errorf("the created file is headed rfc=%v audited=%v, want rfc9999 and 2026-09-26",
			audit.Document["rfc"], audit.Document["audited"])
	}
	send, _ := audit.Record(selftestRIDSend)
	wantKeys := []string{selftestCIPath, selftestCIPath + "#2"}
	if got := sortedKeysOf(send.Tests); !slices.Equal(got, wantKeys) {
		t.Errorf("tests keys are %v, want %v", got, wantKeys)
	}
	if got := sortedKeysOf(send.Units); !slices.Equal(got, wantKeys) {
		t.Errorf("units keys are %v, want %v", got, wantKeys)
	}
	drop, _ := audit.Record(selftestRIDDrop)
	if len(drop.Tests) != 0 || len(drop.Units) != 0 {
		t.Errorf("an untagged row was given tests %v and units %v", drop.Tests, drop.Units)
	}
	if drop.Code[stampProducerKey] == "" {
		t.Errorf("the cited producer was not fingerprinted: %v", drop.Code)
	}

	states := stampFreshness(t, root)
	for _, rid := range []string{selftestRIDSend, selftestRIDDrop} {
		if states[rid].State != FreshState {
			t.Errorf("%s reads %q after the stamp, want %q", rid, states[rid].State, FreshState)
		}
	}
}

// VALIDATES: a merge into an existing audit file leaves every recorded verdict
// byte-identical, even a stale one, and the stale one stays stale. The file
// takes the pending file's audited date.
// METHOD: plant a recorded verdict whose requirement_sha no longer matches its
// row, stamp a not-applicable verdict for the other row, and compare the
// recorded verdict's rendering and its freshness.
// PREVENTS: the stamp laundering a stale verdict into a fresh one, which is a
// re-judgement nobody made.
func TestAuditStampLeavesARecordedVerdictUntouched(t *testing.T) {
	stale := map[string]any{
		"verdict": VerdictWeak, "note": "judged against an older text",
		"requirement_sha": "0000000000000000",
	}
	root := stampTree(t, map[string]any{selftestRIDSend: stale})
	before := pyDump(stale)
	writeFixtureFiles(t, root, map[string]string{stampPendingRel: pyDump(map[string]any{
		"rfc": "rfc9999", "audited": "2026-09-20",
		"requirements": map[string]any{selftestRIDDrop: map[string]any{
			"verdict": VerdictNotApplicable, "note": "binds the document's authors",
			"no_code_path": "no code runs for an obligation on authors",
		}},
	})})
	from := filepath.Join(root, filepath.FromSlash(stampPendingRel))

	report, err := auditStamp(root, "rfc9999", from, stampNow)
	if err != nil {
		t.Fatalf("stamp refused: %v", err)
	}
	if want := []string{selftestRIDDrop}; !slices.Equal(report.Stamped, want) {
		t.Fatalf("stamped %v, want %v", report.Stamped, want)
	}
	audit, err := loadAudit(root, "rfc9999")
	if err != nil {
		t.Fatalf("the merged file does not load: %v", err)
	}
	if audit.Document["audited"] != "2026-09-20" {
		t.Errorf("the merged file is dated %v, want the pending file's 2026-09-20", audit.Document["audited"])
	}
	after, _ := audit.Verdict(selftestRIDSend)
	if got := pyDump(after); got != before {
		t.Errorf("the recorded verdict changed:\nbefore %s\nafter  %s", before, got)
	}
	states := stampFreshness(t, root)
	if state := states[selftestRIDSend].State; state != StaleRequirementState {
		t.Errorf("the stale verdict reads %q, want %q: the stamp laundered it", state, StaleRequirementState)
	}
	if state := states[selftestRIDDrop].State; state != FreshState {
		t.Errorf("the stamped verdict reads %q, want %q", state, FreshState)
	}
}

// VALIDATES: a pending verdict for a row that already has one, for no row, with
// a word outside the closed vocabulary, claiming enforced over an untagged row,
// claiming not-applicable over a tagged row, or carrying a computed field, is
// refused by id and nothing is written. So is a pending file for another stem,
// and a stem that is not enrolled.
// METHOD: one pending file per defect over a tree holding one recorded verdict,
// each beside a valid entry where the row allows one, and the audit file's
// bytes compared before and after.
// PREVENTS: a re-judgement disguised as a stamp, a stamp that makes an
// unreadable claim look fingerprinted, and a partial write that merges the
// valid half of a refused file.
func TestAuditStampRefusesAndWritesNothing(t *testing.T) {
	recorded := map[string]any{
		"verdict": VerdictWeak, "note": "judged against an older text",
		"requirement_sha": "0000000000000000",
	}
	valid := map[string]any{
		"verdict": VerdictNotApplicable, "note": "binds the document's authors",
		"no_code_path": "no code runs for an obligation on authors",
	}
	cases := []struct {
		name     string
		stem     string
		verdicts map[string]any
		want     string
	}{
		{"row already judged", "rfc9999", map[string]any{
			selftestRIDSend: map[string]any{"verdict": VerdictEnforced, "note": "re-judged"},
			selftestRIDDrop: valid,
		}, selftestRIDSend},
		{"unknown id", "rfc9999", map[string]any{
			"RFC9999-9-9":   map[string]any{"verdict": VerdictWeak, "note": "no such row"},
			selftestRIDDrop: valid,
		}, "RFC9999-9-9"},
		{"verdict outside the enum", "rfc9999", map[string]any{
			selftestRIDDrop: map[string]any{"verdict": "implemented", "note": "drift"},
		}, selftestRIDDrop},
		{"enforced with no tags", "rfc9999", map[string]any{
			selftestRIDDrop: map[string]any{"verdict": VerdictEnforced, "note": "nothing tags it"},
		}, selftestRIDDrop},
		{"computed field written by hand", "rfc9999", map[string]any{
			selftestRIDDrop: map[string]any{
				"verdict": VerdictWeak, "note": "hand sha", "requirement_sha": "0123456789abcdef",
			},
		}, selftestRIDDrop},
		{"code sha written by hand", "rfc9999", map[string]any{
			selftestRIDDrop: map[string]any{
				"verdict": VerdictUnimplemented, "note": "hand sha",
				fingerprintCode: map[string]any{stampProducerKey: "0123456789abcdef"},
			},
		}, selftestRIDDrop},
		{"pending file for another stem", "rfc8888", map[string]any{selftestRIDDrop: valid}, "rfc8888"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := stampTree(t, map[string]any{selftestRIDSend: recorded})
			before := readStampAudit(t, root)
			from := stampPending(t, root, tc.stem, tc.verdicts)

			_, err := auditStamp(root, "rfc9999", from, stampNow)
			if err == nil {
				t.Fatal("the stamp accepted the pending file")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not name %s: %v", tc.want, err)
			}
			if got := readStampAudit(t, root); !bytes.Equal(got, before) {
				t.Errorf("a refused stamp wrote the file:\n%s", got)
			}
		})
	}

	t.Run("not-applicable over a tagged row", func(t *testing.T) {
		root := stampTree(t, nil)
		from := stampPending(t, root, "rfc9999", map[string]any{selftestRIDSend: valid})
		if _, err := auditStamp(root, "rfc9999", from, stampNow); err == nil ||
			!strings.Contains(err.Error(), selftestRIDSend) {
			t.Errorf("the refusal does not name %s: %v", selftestRIDSend, err)
		}
		if got := readStampAudit(t, root); got != nil {
			t.Errorf("a refused stamp created the file:\n%s", got)
		}
	})

	t.Run("stem not enrolled", func(t *testing.T) {
		root := stampTree(t, nil)
		from := stampPending(t, root, "rfc8888", map[string]any{selftestRIDDrop: valid})
		if _, err := auditStamp(root, "rfc8888", from, stampNow); err == nil {
			t.Error("a stem with no summary was accepted")
		}
	})
}
