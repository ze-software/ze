// Related: report.go -- the payload these cases read
//
// VALIDATES: a declared level below the ceiling is reported as a promotion
// candidate and never refused, and the report names the next level's unmet
// criteria (AC-12, AC-13).
// PREVENTS: an under-claim treated as an error, and a report silent on why a
// feature sits where it does.

package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

func TestReportListsUnderClaimAsCandidate(t *testing.T) {
	tree := fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| Level | supported |", "| Level | experimental |", 1)
	})
	entries, err := Report(tree, "widget")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries %v", entries)
	}
	if !entries[0].Promotion {
		t.Fatalf("not a promotion candidate: %+v", entries[0])
	}
	if len(entries[0].Refusals) != 0 {
		t.Fatalf("an under-claim was refused: %v", entries[0].Refusals)
	}
	if entries[0].Status != "Experimental" {
		t.Fatalf("status %q", entries[0].Status)
	}
}

func TestReportNamesNextLevelGaps(t *testing.T) {
	tree := fixtureTree(t, func(text string) string {
		return strings.Replace(text, "| Level | supported |", "| Level | experimental |", 1)
	})
	if err := os.Remove(filepath.Join(tree, runRecordRel("widget"))); err != nil {
		t.Fatal(err)
	}
	entries, err := Report(tree, "")
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Next != "supported" {
		t.Fatalf("next %q", entries[0].Next)
	}
	if !strings.Contains(strings.Join(entries[0].NextGaps, "\n"), "exists, not run") {
		t.Fatalf("gaps %v", entries[0].NextGaps)
	}
}

func TestJudgeRefusesAnEmptyPopulation(t *testing.T) {
	tree := t.TempDir()
	for rel, content := range rfc.FixtureFiles() {
		writeFile(t, tree, rel, content)
	}
	if err := os.Mkdir(filepath.Join(tree, declarationDir), 0o750); err != nil {
		t.Fatal(err)
	}
	report, err := Judge(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Refused) == 0 {
		t.Fatal("an empty declaration directory passed")
	}
}
