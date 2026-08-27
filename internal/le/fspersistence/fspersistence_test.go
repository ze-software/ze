// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the persistence guard is
// called as a function, answers structured rows, and keeps its exit codes apart.
// PREVENTS: a port whose detection silently narrowed. A guard over a clean tree
// and a guard that stopped looking print the same page, so the fixtures are the
// only thing that tells them apart.

package fspersistence

import (
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// writeTree writes a tree from a path-to-body map and answers its root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

// fixture writes a tree holding one violation of each kind the gate draws, plus
// the cases it must NOT report.
func fixture(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"internal/plugins/a/save.go": "package a\n\nimport \"os\"\n\nfunc save(p string, d []byte) error { return os.WriteFile(p, d, 0o600) }\n",
		"internal/component/b/open.go": "package b\n\nimport \"os\"\n\nfunc w(p string) (*os.File, error) { return os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o644) }\n" +
			"func r(p string) (*os.File, error) { return os.OpenFile(p, os.O_RDONLY, 0) }\n",
		"cmd/ze/read.go":                         "package main\n\nimport \"os\"\n\nfunc load(p string) ([]byte, error) { return os.ReadFile(p) }\n",
		"internal/component/b/b_test.go":         "package b\n\nimport \"os\"\n\nfunc TestX() { _ = os.WriteFile(\"x\", nil, 0o600) }\n",
		"internal/component/b/testing/helper.go": "package testing\n\nimport \"os\"\n\nfunc fixture(p string) error { return os.WriteFile(p, nil, 0o600) }\n",
		"internal/component/config/storage/s.go": "package storage\n\nimport \"os\"\n\nfunc put(p string, d []byte) error { return os.WriteFile(p, d, 0o600) }\n",
		"internal/component/config/provider.go":  "package config\n\nimport \"os\"\n\nfunc save(p string, d []byte) error { return os.WriteFile(p, d, 0o600) }\n",
	})
}

// VALIDATES: this checkout passes the gate and the walk actually read it.
// PREVENTS: a port whose walk roots resolve to nothing, which looks identical to
// a clean tree.
func TestTheRealCheckoutPasses(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}

	findings, err := Check(tree, scanFloor)
	if err != nil {
		t.Fatalf("the gate could not read this checkout: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("this checkout fails the persistence gate:\n%s", findings.Text())
	}
}

// VALIDATES: a tree too small to be the one asked about is an ERROR.
// PREVENTS: a run whose walk roots exist but hold nothing reporting OK, which is
// the same false green a missing root would give.
func TestATreeTooSmallToBeTheOneAskedAboutIsAnError(t *testing.T) {
	tree := fixture(t)
	if _, err := Check(tree, scanFloor); err == nil {
		t.Error("a seven-file tree passed the package floor")
	}
}

// VALIDATES: both polarities over one tree -- each violating shape is drawn and
// each exempt shape is left alone.
// PREVENTS: an allowlist or a pattern that quietly stopped applying, which no
// clean-tree run can show.
func TestTheFixtureDrawsOnlyTheViolations(t *testing.T) {
	findings, err := Check(fixture(t), 0)
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}

	got := make(map[string]string, len(findings))
	for _, finding := range findings {
		got[finding.File] = finding.Fn
	}
	want := map[string]string{
		"internal/plugins/a/save.go":   "WriteFile",
		"internal/component/b/open.go": "OpenFile",
	}
	if len(got) != len(want) {
		t.Fatalf("the gate drew %d files, want %d: %+v", len(got), len(want), got)
	}
	for file, fn := range want {
		if got[file] != fn {
			t.Errorf("%s was drawn as %q, want %q", file, got[file], fn)
		}
	}
}

// VALIDATES: a file the parser cannot read stops the run.
// PREVENTS: a raw write nobody parsed being counted as no raw write at all.
func TestAFileThatWillNotParseStopsTheRun(t *testing.T) {
	dir := fixture(t)
	if err := os.WriteFile(filepath.Join(dir, "internal", "plugins", "a", "bad.go"), []byte("package a\n\nfunc broken( {\n"), 0o600); err != nil {
		t.Fatalf("write the broken file: %v", err)
	}

	_, err := Check(dir, 0)
	if err == nil {
		t.Fatal("a file that will not parse was walked past")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("the error is %v, want one naming the parse", err)
	}
}

// VALIDATES: the selftest table passes over its own fixtures and answers 0.
// PREVENTS: a selftest that cannot pass, which would be discovered only by the
// gate that runs it.
func TestTheSelftestPassesOverItsOwnFixtures(t *testing.T) {
	report, err := Selftest()
	if err != nil {
		t.Fatalf("the selftest could not run: %v", err)
	}
	if failures := report.Failures(); len(failures) != 0 {
		t.Errorf("the selftest failed: %v", failures)
	}
	if code := report.Code(1); code != 0 {
		t.Errorf("a passing selftest answers %d, want 0", code)
	}
	if len(report.Results) != len(selftestCases) {
		t.Fatalf("the selftest answered %d rows for %d cases", len(report.Results), len(selftestCases))
	}
	for i, result := range report.Results {
		if result.Case != selftestCases[i].name {
			t.Errorf("row %d names %q, want %q", i, result.Case, selftestCases[i].name)
		}
	}
}

// VALIDATES: every selftest fixture is DISCRIMINATING -- each one draws exactly
// the count it declares, and the four zero-count fixtures are not vacuous.
// PREVENTS: a fixture whose expectation the guard would meet however it broke,
// which is a selftest row that proves nothing.
func TestEverySelftestFixtureDrawsWhatItDeclares(t *testing.T) {
	dir := t.TempDir()
	fset := token.NewFileSet()
	flagged, silent := 0, 0

	for _, testCase := range selftestCases {
		path := filepath.Join(dir, testCase.name, "k.go")
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(testCase.source), 0o600); err != nil {
			t.Fatalf("write %s: %v", testCase.name, err)
		}
		found, err := ScanFile(fset, path, testCase.name)
		if err != nil {
			t.Fatalf("scan %s: %v", testCase.name, err)
		}
		if len(found) != testCase.want {
			t.Errorf("fixture %s drew %d findings, want %d", testCase.name, len(found), testCase.want)
		}
		if testCase.want > 0 {
			flagged++
			continue
		}
		silent++
	}

	if flagged < 4 || silent < 4 {
		t.Errorf("the table holds %d must-flag and %d must-not-flag fixtures; both polarities are what make a silent gate meaningful", flagged, silent)
	}
}

// VALIDATES: AC-7 -- the payload is data a JSON encoder takes, with the
// script's own keys.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestFindingsAreStructuredRows(t *testing.T) {
	raw, err := json.Marshal(Findings{{File: "a.go", Line: 3, Pkg: "os", Fn: "WriteFile", Code: "os.WriteFile(p, d, 0o600)"}})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	if !strings.HasPrefix(string(raw), "[") {
		t.Errorf("the payload is not an array: %s", raw)
	}
	for _, key := range []string{`"file"`, `"line"`, `"pkg"`, `"fn"`, `"code"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}

// VALIDATES: the page says OK when there is nothing, and carries the site and
// the remedy when there is.
// PREVENTS: a failing gate whose page still reads OK, which is what a reader
// acts on.
func TestThePageCarriesItsVerdictAndItsRemedy(t *testing.T) {
	if clean := (Findings{}).Text(); clean != "direct-fs-persistence: OK\n" {
		t.Errorf("the clean page is %q", clean)
	}

	found := Findings{{File: "a.go", Line: 3, Pkg: "os", Fn: "WriteFile", Code: "os.WriteFile(p, d, 0o600)"}}.Text()
	if strings.Contains(found, "OK") {
		t.Errorf("a page holding a finding still says OK:\n%s", found)
	}
	if !strings.Contains(found, "a.go:3 (os.WriteFile)") {
		t.Errorf("the page does not carry its site:\n%s", found)
	}
	if !strings.Contains(found, "internal/core/statestore") {
		t.Errorf("the page does not carry the remedy:\n%s", found)
	}
}

// VALIDATES: the area dispatches its two actions and refuses the two mistakes.
// PREVENTS: a verb that drifts from its gate name, which would leave the Make
// target pointing at nothing after the swap.
func TestTheAreaDispatchesItsActions(t *testing.T) {
	if _, code := Answer([]string{"selftest"}); code != 0 {
		t.Errorf("the selftest action answers %d over its own fixtures, want 0", code)
	}
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"check", "value"}); code != 1 {
		t.Errorf("a value after an action answers %d, want 1", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want 2", len(verbs.Actions))
	}
	if verbs.Actions[0].Verb != "check" || verbs.Actions[1].Verb != "selftest" {
		t.Errorf("the verbs are %q and %q, want check and selftest", verbs.Actions[0].Verb, verbs.Actions[1].Verb)
	}
}
