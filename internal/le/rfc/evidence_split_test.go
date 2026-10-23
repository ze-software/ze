package rfc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// splitRetiredFixture answers a per-RFC discrimination file holding a record
// for each of two requirements, a per-RFC audit file judging both, and the
// records themselves.
func splitRetiredFixture(t *testing.T) (map[string]string, []DiscriminationRecord) {
	t.Helper()

	files, send := discriminationFixture(t)
	drop := send
	drop.RID = selftestRIDDrop
	raw, err := json.Marshal(retiredDiscriminationFile{RFC: selftestStem, Records: []DiscriminationRecord{send, drop}})
	if err != nil {
		t.Fatalf("render the retired file: %v", err)
	}
	files[discriminationRel+"/rfc9999.json"] = string(raw)
	files[auditRel+"/rfc9999.json"] = pyDump(map[string]any{
		"rfc": "rfc9999", "audited": "2026-09-01", "reaudit_note": "a re-stamp",
		"requirements": map[string]any{
			selftestRIDSend: map[string]any{"verdict": VerdictWeak, "note": "one count only", "requirement_sha": "0000000000000000"},
			selftestRIDDrop: map[string]any{"verdict": VerdictWrong, "note": "asserts another rule", "requirement_sha": "1111111111111111"},
		},
	}) + "\n"
	return files, []DiscriminationRecord{send, drop}
}

// VALIDATES: a per-RFC evidence file converts into one file per requirement,
// holding exactly the records and verdicts it held, and the loaders refuse the
// per-RFC file until it is gone rather than dropping its records in silence.
// METHOD: lay both retired files, convert, assert the loaders refuse while they
// remain and naming the conversion, remove them, then load and compare. A second
// conversion must write nothing.
func TestEvidenceSplitConvertsTheRetiredLayout(t *testing.T) {
	files, records := splitRetiredFixture(t)
	root := t.TempDir()
	writeFixtureFiles(t, root, files)

	report, err := splitEvidence(root, "")
	if err != nil {
		t.Fatalf("the conversion refused: %v", err)
	}
	if len(report.Files) != 2 || len(report.Files[0].Written) != 2 || len(report.Files[1].Written) != 2 {
		t.Fatalf("the conversion wrote %+v, want two files from each retired file", report.Files)
	}
	if _, err := loadDiscrimination(root); err == nil || !strings.Contains(err.Error(), "./le rfc evidence-split") {
		t.Fatalf("a retired discrimination file loaded, or was refused without the fix: %v", err)
	}
	if _, err := loadAudit(root, selftestStem); err == nil || !strings.Contains(err.Error(), "./le rfc evidence-split") {
		t.Fatalf("a retired audit file loaded, or was refused without the fix: %v", err)
	}

	again, err := splitEvidence(root, "")
	if err != nil {
		t.Fatalf("the second conversion refused: %v", err)
	}
	for _, file := range again.Files {
		if len(file.Written) != 0 || file.Unchanged != 2 {
			t.Errorf("a second conversion of %s wrote %v, want nothing", file.Retired, file.Written)
		}
	}

	for _, retired := range []string{discriminationRel + "/rfc9999.json", auditRel + "/rfc9999.json"} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(retired))); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := loadDiscrimination(root)
	if err != nil {
		t.Fatalf("the converted records do not load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d records, want 2", len(loaded))
	}
	for index, record := range loaded {
		if record.RID != records[index].RID || record.Source != discriminationRecordRel(selftestStem, record.RID) {
			t.Errorf("record %d proves %s from %s, want %s from its own file", index, record.RID, record.Source, records[index].RID)
		}
	}
	audit, err := loadAudit(root, selftestStem)
	if err != nil {
		t.Fatalf("the converted verdicts do not load: %v", err)
	}
	if want := []string{selftestRIDSend, selftestRIDDrop}; !slices.Equal(audit.Order, want) {
		t.Fatalf("loaded verdicts %v, want %v", audit.Order, want)
	}
	if drop, _ := audit.Record(selftestRIDDrop); drop.Verdict != VerdictWrong || drop.RequirementSHA != "1111111111111111" {
		t.Errorf("the converted drop verdict reads %+v", drop)
	}
}

// VALIDATES: `from head` converts what HEAD holds, so a file that also carries
// another session's uncommitted record converts without that record.
// METHOD: commit the retired files, add a record for a third requirement in the
// working tree only, convert from HEAD, and assert no file was written for it.
func TestEvidenceSplitFromHeadLeavesUncommittedRecordsOut(t *testing.T) {
	files, records := splitRetiredFixture(t)
	root := t.TempDir()
	writeFixtureFiles(t, root, files)
	gitFixture(t, root, []string{"init", "-q"})
	commitFixture(t, root, "retired layout")

	foreign := records[0]
	foreign.RID = "RFC9999-2-3"
	raw, err := json.Marshal(retiredDiscriminationFile{RFC: selftestStem, Records: append(records, foreign)})
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFiles(t, root, map[string]string{discriminationRel + "/rfc9999.json": string(raw)})

	if _, err := splitEvidence(root, splitSourceHead); err != nil {
		t.Fatalf("the conversion from HEAD refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(discriminationRecordRel(selftestStem, foreign.RID)))); !os.IsNotExist(err) {
		t.Fatalf("the uncommitted record was converted from HEAD: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(discriminationRecordRel(selftestStem, selftestRIDSend)))); err != nil {
		t.Fatalf("the committed record was not converted: %v", err)
	}
}

// VALIDATES: a record filed under another requirement's file is refused, since
// the file name is what a per-requirement read trusts.
// METHOD: write the send proof into the drop requirement's file and load.
func TestDiscriminationRecordUnderAnotherRequirementRefused(t *testing.T) {
	files, proof := discriminationFixture(t)
	body, err := renderDiscriminationRecords([]DiscriminationRecord{proof})
	if err != nil {
		t.Fatal(err)
	}
	files[discriminationRecordRel(selftestStem, selftestRIDDrop)] = body + "\n"
	root := t.TempDir()
	writeFixtureFiles(t, root, files)

	if _, err := loadDiscrimination(root); err == nil || !strings.Contains(err.Error(), "the filename says") {
		t.Fatalf("a record under another requirement's file loaded: %v", err)
	}
}
