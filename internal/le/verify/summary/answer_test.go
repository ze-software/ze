package verifysummary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestMigratedSummaryKeepsItsExitCodes pins every refusal of `le verify summary`
// to the code the hand-rolled parser answered before the area moved onto its
// action table: 2 for a malformed line, 1 for an append that failed, 0 for the
// bare listing.
//
// VALIDATES: AC-17 for this area, the golden captured from the parser it
// replaced (answer.go before Phase 3: len(args) != 7, a wrong verb and a wrong
// keyword each answered 2; AppendSummary's error answered 1).
// PREVENTS: a migration that turns a usage mistake into a failure or a pass.
func TestMigratedSummaryKeepsItsExitCodes(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "failures.log")
	missing := filepath.Join(dir, "no-such-stage.log")
	// A failure index under a regular file cannot be created, so the append
	// itself fails after the line parsed.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	unwritable := filepath.Join(blocker, "failures.log")
	cases := []struct {
		args []string
		want int
	}{
		{nil, 0},
		{[]string{"walk"}, 2},
		{[]string{"append"}, 2},
		{[]string{"append", "failures", index}, 2},
		{[]string{"append", "failures", index, "stage", "lint"}, 2},
		{[]string{"append", "failures", index, "stage", "lint", "log"}, 2},
		{[]string{"append", "failures", index, "stage", "lint", "log", missing, "extra"}, 2},
		{[]string{"append", "failures", unwritable, "stage", "lint", "log", missing}, 1},
	}
	for _, tc := range cases {
		if _, code := Answer(tc.args); code != tc.want {
			t.Errorf("`le verify summary %s` answered %d, want %d", strings.Join(tc.args, " "), code, tc.want)
		}
	}
}

// TestSummaryAppendsTheBlockTheLineNames proves a well-formed line reaches the
// engine with its three values, in any keyword order the table accepts.
func TestSummaryAppendsTheBlockTheLineNames(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "failures.log")
	stageLog := filepath.Join(dir, "lint.log")
	if err := os.WriteFile(stageLog, []byte("FAIL something broke\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, code := Answer([]string{"append", "stage", "lint", "log", stageLog, "failures", index}); code != 0 {
		t.Fatalf("a well-formed append answered %d, want 0", code)
	}
	written, err := os.ReadFile(index) //nolint:gosec // the path is this test's own temporary file
	if err != nil {
		t.Fatalf("the index was not written: %v", err)
	}
	if !strings.Contains(string(written), "lint") {
		t.Errorf("the index does not name the stage:\n%s", written)
	}
}

// TestSummaryDeclaresItsGrammar proves the listing comes from the table: one
// append action whose three keywords are required and typed.
func TestSummaryDeclaresItsGrammar(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 1 || list.Actions[0].Verb != appendVerb {
		t.Fatalf("verify summary declares %+v, want the one append action", list.Actions)
	}
	for _, parameter := range list.Actions[0].Parameters {
		if parameter.Requirement != leaction.Required {
			t.Errorf("%s is not required", parameter.Keyword)
		}
	}
}
