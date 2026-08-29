// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the boundary guard is
// called as a function, answers structured rows, and keeps its exit codes apart.
// PREVENTS: the fail-open the script carried. It skipped any scan root it could
// not stat, so a tree holding the generator and none of the plugin directories
// printed `plugin-process-boundary: OK` having read no file.

package pluginboundary

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/plugin/imports"
)

// fixtureTree writes the four package fixtures into a temporary tree and
// answers its root.
func fixtureTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := WriteFixture(dir); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	return dir
}

// VALIDATES: this checkout passes the gate and the walk actually read it.
// PREVENTS: a port whose scan roots resolve to nothing, which looks identical to
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
		t.Errorf("this checkout fails the boundary gate:\n%s", findings.Text())
	}
}

// VALIDATES: the scan roots come from the composition-root generator, by CALL.
// PREVENTS: a second hardcoded list. A namespace added to the generator must be
// scanned without editing this package, which is what the script's regex over
// the generator's source text bought and what the call buys without the parse.
func TestTheRootsAreTheGeneratorsOwnList(t *testing.T) {
	if got, want := Roots(), pluginimports.PluginSearchRoots(); !slices.Equal(got, want) {
		t.Errorf("the gate scans %v, the generator declares %v", got, want)
	}
	if len(Roots()) == 0 {
		t.Fatal("the derivation answered no root at all, so the gate would scan nothing")
	}
}

// VALIDATES: a tree carrying none of the plugin roots is an ERROR.
// PREVENTS: the script's fail-open, demonstrated on the built script over a
// tree holding only the generator: OK, exit 0, no file read.
func TestATreeWithNoPluginRootIsAnError(t *testing.T) {
	if _, err := Check(t.TempDir(), scanFloor); err == nil {
		t.Fatal("a tree holding no plugin root passed the floor")
	}
	if _, err := Check(fixtureTree(t), scanFloor); err == nil {
		t.Error("a five-file fixture tree passed the package floor")
	}
}

// VALIDATES: both polarities over one tree -- the two unguarded packages are
// drawn and the guarded and blank-import ones are left alone.
// PREVENTS: alias resolution regressing, which the real tree cannot show: it
// holds no dangerous call made through a renamed import.
func TestTheFixtureDrawsOnlyTheUnguardedPackages(t *testing.T) {
	dir := fixtureTree(t)
	findings, err := Check(dir, 0)
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}

	drawn := map[string]bool{}
	for _, finding := range findings {
		drawn[filepath.ToSlash(filepath.Dir(finding.File))] = true
	}
	for _, fixture := range packageFixtures {
		pkg := filepath.ToSlash(filepath.Join("internal", "plugins", fixture.name))
		if drawn[pkg] != fixture.flagged {
			t.Errorf("package %s was drawn=%v, want %v: %s", fixture.name, drawn[pkg], fixture.flagged, fixture.why)
		}
	}
	if len(drawn) != 2 {
		t.Errorf("the gate drew %d packages, want the two unguarded ones: %+v", len(drawn), findings)
	}
}

// VALIDATES: a call named only in a comment is not a call site.
// PREVENTS: the prose false positives the comment rule exists to remove, which
// would make the gate unusable and get it switched off.
func TestACallNamedOnlyInProseIsNotDrawn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "internal", "plugins", "prose", "register.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	body := "package prose\n\nimport \"" + ifacePkg + "\"\n\n// iface.GetBackend() is what this replaces\nfunc run() { _ = iface.Name }\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}

	findings, err := Check(dir, 0)
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("a call named in a comment was drawn: %+v", findings)
	}
}

// VALIDATES: a file whose imports will not parse stops the run.
// PREVENTS: a dangerous call nobody resolved being counted as no dangerous call
// at all.
func TestAFileThatWillNotParseStopsTheRun(t *testing.T) {
	dir := fixtureTree(t)
	path := filepath.Join(dir, "internal", "plugins", "plain", "bad.go")
	if err := os.WriteFile(path, []byte("package plain\n\nimport ( \"os\"\n"), 0o600); err != nil {
		t.Fatalf("write the broken file: %v", err)
	}

	_, err := Check(dir, 0)
	if err == nil {
		t.Fatal("a file whose imports will not parse was walked past")
	}
	if !strings.Contains(err.Error(), "parse imports") {
		t.Errorf("the error is %v, want one naming the import parse", err)
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
	if want := len(packageFixtures) + len(rootCases); len(report.Results) != want {
		t.Fatalf("the selftest answered %d rows for %d cases", len(report.Results), want)
	}
}

// VALIDATES: every root case FAILS on a derivation that breaks the property it
// declares.
// PREVENTS: a case whose check can never fail, which is a selftest row that
// proves nothing and still counts as a pass.
func TestEveryRootCaseFailsOnABrokenDerivation(t *testing.T) {
	broken := map[string][]string{
		"roots-derived":               {"internal/component/x/plugins"},
		"roots-deduplicated":          {"internal/plugins", "internal/plugins", "internal/component/x/plugins"},
		"roots-expand-nested-domains": {"internal/plugins"},
	}
	for _, testCase := range rootCases {
		roots, ok := broken[testCase.name]
		if !ok {
			t.Fatalf("case %s has no broken derivation to test against", testCase.name)
		}
		if detail := testCase.check(roots); detail == "" {
			t.Errorf("case %s passed over %v, which breaks it", testCase.name, roots)
		}
		if detail := testCase.check(Roots()); detail != "" {
			t.Errorf("case %s fails over the real derivation: %s", testCase.name, detail)
		}
	}
}

// VALIDATES: AC-7 -- both payloads are data a JSON encoder takes, with the
// script's own keys.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestBothAnswersAreStructuredRows(t *testing.T) {
	raw, err := json.Marshal(Findings{{File: "a.go", Line: 3, Code: "iface.GetBackend()"}})
	if err != nil {
		t.Fatalf("the findings do not encode: %v", err)
	}
	for _, key := range []string{`"file"`, `"line"`, `"code"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}

	rootsRaw, err := json.Marshal(RootList(Roots()))
	if err != nil {
		t.Fatalf("the roots do not encode: %v", err)
	}
	if !strings.HasPrefix(string(rootsRaw), `["`) {
		t.Errorf("the roots payload is not an array of strings: %s", rootsRaw)
	}
}

// VALIDATES: the page says OK when there is nothing, and carries the site and
// the remedy when there is.
// PREVENTS: a failing gate whose page still reads OK, which is what a reader
// acts on.
func TestThePageCarriesItsVerdictAndItsRemedy(t *testing.T) {
	if clean := (Findings{}).Text(); clean != "plugin-process-boundary: OK\n" {
		t.Errorf("the clean page is %q", clean)
	}

	found := Findings{{File: "a.go", Line: 3, Code: "iface.GetBackend()"}}.Text()
	if strings.Contains(found, "OK") {
		t.Errorf("a page holding a finding still says OK:\n%s", found)
	}
	if !strings.Contains(found, "a.go:3: iface.GetBackend()") {
		t.Errorf("the page does not carry its site:\n%s", found)
	}
	if !strings.Contains(found, "p.IsInternal()") {
		t.Errorf("the page does not carry the remedy:\n%s", found)
	}
}

// VALIDATES: the area dispatches its three actions and refuses the two
// mistakes.
// PREVENTS: a verb that drifts from its registered native action and becomes
// unreachable.
func TestTheAreaDispatchesItsActions(t *testing.T) {
	payload, code := Answer([]string{"roots"})
	if code != 0 {
		t.Errorf("the roots action answers %d, want 0", code)
	}
	if list, ok := payload.(RootList); !ok || len(list) == 0 {
		t.Errorf("the roots action answered %T with %v", payload, payload)
	}
	if _, code := Answer([]string{"selftest"}); code != 0 {
		t.Errorf("the selftest action answers %d over its own fixtures, want 0", code)
	}
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"check", "value"}); code != 2 {
		t.Errorf("a value after an action answers %d, want 2", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 3 {
		t.Fatalf("the area holds %d actions, want 3", len(verbs.Actions))
	}
	for i, want := range []string{"check", "selftest", "roots"} {
		if verbs.Actions[i].Verb != want {
			t.Errorf("action %d is %q, want %q", i, verbs.Actions[i].Verb, want)
		}
	}
}
