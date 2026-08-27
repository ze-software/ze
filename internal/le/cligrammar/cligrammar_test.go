// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the grammar gate is
// called as a function, answers structured data, and keeps its three exit codes
// apart.
// PREVENTS: the fail-open the script carried. Three of its five feeders walk the
// working directory, and it skipped a file it could not open, ignored a scanner
// error and ignored a Go file it could not parse, so a tree it never read
// printed `cli-grammar: OK`.

package cligrammar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/grammar"
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

// cleanFixture is a tree holding one of every population the gate reads, and no
// violation in any of them.
func cleanFixture(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"internal/a/yang/ze-a-conf.yang": "module ze-a-conf {\n  container traffic {\n    leaf x { type string; }\n  }\n}\n",
		"cmd/ze/roots.go":                "package main\n\nfunc wire() { registry.MustRegisterRootHandler(\"env\", nil, meta) }\n",
		"demos/terminal/one/run.sh":      "#!/bin/sh\nze show interface\n",
	}
}

// VALIDATES: the gate passes over this checkout and read every population.
// PREVENTS: a port whose walk roots resolve to nothing, which would look
// identical to a clean tree.
func TestTheRealCheckoutPassesAndWasRead(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}

	result, err := Check(tree, DefaultFloor)
	if err != nil {
		t.Fatalf("the gate could not read this checkout: %v", err)
	}
	if !result.Valid {
		t.Errorf("this checkout fails the grammar gate:\n%s", result.Text())
	}
	if result.Checked == 0 || result.RootsChecked == 0 || result.DemoScripts == 0 {
		t.Errorf("the gate checked %d commands, %d roots and %d demo scripts; a zero is a population it never read",
			result.Checked, result.RootsChecked, result.DemoScripts)
	}
}

// VALIDATES: a tree holding none of the gate's populations is an ERROR.
// PREVENTS: the script's fail-open. Run outside a checkout it read no .yang
// file, resolved no root and opened no demo script, and still printed OK.
func TestATreeTooSmallToBeTheOneAskedAboutIsAnError(t *testing.T) {
	tree := writeTree(t, cleanFixture(t))

	if _, err := Check(tree, DefaultFloor); err == nil {
		t.Fatal("a tree holding one file per population passed the default floor")
	}

	for _, floor := range []Floor{
		{YANGFiles: 2, Roots: 0, DemoScripts: 0},
		{YANGFiles: 0, Roots: 2, DemoScripts: 0},
		{YANGFiles: 0, Roots: 0, DemoScripts: 2},
	} {
		if _, err := Check(tree, floor); err == nil {
			t.Errorf("a tree under the floor %+v passed", floor)
		}
	}
}

// VALIDATES: a Go file the parser cannot read stops the run.
// PREVENTS: a root registration nobody resolved being counted as no
// registration at all, which is how a bad name would reach the shipped CLI with
// the gate green.
func TestAFileThatWillNotParseStopsTheRun(t *testing.T) {
	files := cleanFixture(t)
	files["cmd/ze/bad.go"] = "package main\n\nfunc broken( {\n"
	tree := writeTree(t, files)

	_, err := Check(tree, Floor{})
	if err == nil {
		t.Fatal("a file that will not parse was walked past")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("the error is %v, want one naming the parse", err)
	}
}

// VALIDATES: each of the three row sets is drawn over a tree that carries one.
// PREVENTS: a feeder that stopped firing, which no clean-tree run can show.
func TestEachFeederDrawsItsRowOverAFixture(t *testing.T) {
	files := cleanFixture(t)
	files["internal/a/yang/ze-a-cmd.yang"] = "module ze-a-cmd {\n  leaf detail { description \"--detail is banned here\"; }\n}\n"
	files["demos/terminal/one/run.sh"] = "#!/bin/sh\nze show interface\nze mystery-verb\n"
	tree := writeTree(t, files)

	result, err := Check(tree, Floor{})
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}
	if result.Valid {
		t.Fatalf("a tree holding a flag in YANG and a dead launch form passed:\n%s", result.Text())
	}
	if len(result.FlagInYANG) != 1 {
		t.Errorf("the R3 feeder drew %d rows, want the one --detail line: %+v", len(result.FlagInYANG), result.FlagInYANG)
	}
	if len(result.DemoLaunch) != 1 || result.DemoLaunch[0].Token != "mystery-verb" {
		t.Errorf("the call-site feeder drew %+v, want the one dead launch form", result.DemoLaunch)
	}
	if result.DemoLaunch[0].File != "demos/terminal/one/run.sh" {
		t.Errorf("the row names %q, want the tree-relative path", result.DemoLaunch[0].File)
	}
}

// VALIDATES: a heredoc body and a comment are prose rather than call sites.
// PREVENTS: the false positives the heredoc rule exists to remove coming back,
// which would make the feeder unusable and get it switched off.
func TestProseInADemoScriptIsNotACallSite(t *testing.T) {
	files := cleanFixture(t)
	files["demos/terminal/one/run.sh"] = "#!/bin/sh\n# ze narrated-verb\ncat <<'EOF'\nze prose-verb\nEOF\nze show interface\n"
	tree := writeTree(t, files)

	result, err := Check(tree, Floor{})
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}
	if len(result.DemoLaunch) != 0 {
		t.Errorf("prose was read as a call site: %+v", result.DemoLaunch)
	}
}

// VALIDATES: AC-7 -- the payload is data a JSON encoder takes, with the
// script's own keys.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestResultIsStructuredData(t *testing.T) {
	raw, err := json.Marshal(Result{
		Findings:   []grammar.Finding{{Command: "show x", Rule: "R1", Message: "m"}},
		FlagInYANG: []FlagHit{{File: "a.yang", Line: 2, Text: "--x"}},
		DemoLaunch: []DemoLaunchHit{{File: "b.sh", Line: 3, Token: "t"}},
		Exempt:     map[string]int{"bridge": 3},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, key := range []string{
		`"findings"`, `"flag-in-yang"`, `"demo-launch"`, `"exempt-by-category"`,
		`"commands-checked"`, `"demo-scripts-checked"`, `"roots-checked"`,
		`"root-namespace-exempt"`, `"tree-namespace-exempt"`,
		`"pending-namespace-split"`, `"valid"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}

// VALIDATES: the page carries a section per row set and the right verdict.
// PREVENTS: a failing gate whose page still reads OK, which is what a reader
// acts on.
func TestThePageCarriesEachSectionAndItsVerdict(t *testing.T) {
	clean := Result{Checked: 5, RootsChecked: 2, Exempt: map[string]int{"bridge": 1}, Valid: true}.Text()
	if !strings.Contains(clean, "cli-grammar: OK") {
		t.Errorf("the clean page does not carry its verdict:\n%s", clean)
	}
	if !strings.Contains(clean, "Exempt (bridge): 1") {
		t.Errorf("the clean page does not carry its exemptions:\n%s", clean)
	}

	failed := Result{
		Findings:     []grammar.Finding{{Command: "show x", Rule: "R1", Message: "m"}},
		FlagInYANG:   []FlagHit{{File: "a.yang", Line: 2, Text: "--x"}},
		DemoLaunch:   []DemoLaunchHit{{File: "b.sh", Line: 3, Token: "t"}},
		Exempt:       map[string]int{},
		RootExempt:   1,
		TreeExempt:   2,
		PendingSplit: 3,
	}.Text()
	for _, want := range []string{
		"## Grammar violations (1)", "## --flag in YANG (1)",
		"## Dead launch form in demo scripts (1)",
		"Root namespace-exempt (indivisible compounds): 1",
		"Tree namespace-exempt (indivisible compounds): 2",
		"Pending namespace-split (R9 debt, tracked for rename migration): 3",
		"cli-grammar: FAILED (1 grammar, 1 flag-in-yang, 1 demo-launch)",
	} {
		if !strings.Contains(failed, want) {
			t.Errorf("the failing page has no %q:\n%s", want, failed)
		}
	}
}

// VALIDATES: the command takes no argument.
// PREVENTS: a value read as a tree path, which would let a caller point a gate
// at something other than the checkout it is run in.
func TestTheCommandRefusesAnArgument(t *testing.T) {
	if _, code := Answer([]string{"internal"}); code != 1 {
		t.Errorf("a value after the command answers %d, want 1", code)
	}
}
