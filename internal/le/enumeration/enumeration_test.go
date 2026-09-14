// Goal: prove the gate reports a literal that restates a live registry, stays
// silent on a mention and on the registry's own package, and refuses rather
// than passes when a registry or a walk answers nothing.
//
// Method: each corpus has a fixture tree under testdata/ holding one literal
// written from that registry's real keys, and the two floors are driven over
// the real checkout through the same function the command runs.

package enumeration

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/changed"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// fixtureFindings runs the literal walk over one fixture tree, with no file
// floor because a fixture holds a handful of files.
func fixtureFindings(t *testing.T, fixture string) Findings {
	t.Helper()
	found, err := Check(filepath.Join("testdata", fixture), []string{"."}, liveCorpora(t), 0)
	if err != nil {
		t.Fatalf("walk %s: %v", fixture, err)
	}
	return found
}

// wantOneCopy asserts exactly one literal finding, naming the corpus expected.
func wantOneCopy(t *testing.T, found Findings, corpus, symbol string) {
	t.Helper()
	literals := make(Findings, 0, len(found))
	for _, finding := range found {
		if finding.Kind == KindLiteral {
			literals = append(literals, finding)
		}
	}
	if len(literals) != 1 {
		t.Fatalf("want one literal finding, got %d:\n%s", len(literals), found.Text())
	}
	if literals[0].Corpus != corpus {
		t.Errorf("the finding names the corpus %q, want %q", literals[0].Corpus, corpus)
	}
	if literals[0].Symbol != symbol {
		t.Errorf("the finding names the symbol %q, want %q", literals[0].Symbol, symbol)
	}
	if literals[0].Line == 0 {
		t.Error("the finding names no line")
	}
}

// TestPluginNameCopyIsReported is AC-1.
func TestPluginNameCopyIsReported(t *testing.T) {
	wantOneCopy(t, fixtureFindings(t, "plugin-names"), "plugin names", "pluginTitles")
}

// TestFamilyNameCopyIsReported is AC-2.
func TestFamilyNameCopyIsReported(t *testing.T) {
	wantOneCopy(t, fixtureFindings(t, "family-names"), "family names", "label")
}

// TestVerbCopyIsReported is AC-3.
func TestVerbCopyIsReported(t *testing.T) {
	wantOneCopy(t, fixtureFindings(t, "verbs"), "CLI verbs", "verbMonitor")
}

// TestYANGEnumCopyIsReported is AC-4. The finding names the leaf the
// enumeration is declared at, so the reader can go and read the model.
func TestYANGEnumCopyIsReported(t *testing.T) {
	found := fixtureFindings(t, "yang-enums")
	wantOneCopy(t, found, "YANG enumerations", "speeds")
	if !strings.Contains(found[0].Detail, "console/device/speed") {
		t.Errorf("the finding says %q, want the declaring leaf named", found[0].Detail)
	}
	// The row names both sides and stops there. Which side derives from the
	// other is a design decision the gate cannot make, and a row that told the
	// reader to derive from the model would be prescribing one.
	if !strings.Contains(found[0].Detail, "must agree") {
		t.Errorf("the finding says %q, want it to say the two must agree", found[0].Detail)
	}
}

// TestDiagnosticCodeCopyIsReported is AC-5. It also proves the builtin codes
// are in the corpus: both keys in the fixture are builtins, which no init()
// registers.
func TestDiagnosticCodeCopyIsReported(t *testing.T) {
	wantOneCopy(t, fixtureFindings(t, "diagnostic-codes"), "diagnostic codes", "interesting")
}

// TestSingleKeyIsNotAFinding proves the two-key threshold: one key of two
// different registries in one literal is two mentions, not a copy.
func TestSingleKeyIsNotAFinding(t *testing.T) {
	if found := fixtureFindings(t, "single-key"); len(found) != 0 {
		t.Fatalf("want no finding, got:\n%s", found.Text())
	}
}

// TestOwningPackageIsNotAFinding proves the registry's own package may write
// its own key set out, because that is the declaration.
func TestOwningPackageIsNotAFinding(t *testing.T) {
	if found := fixtureFindings(t, "owner"); len(found) != 0 {
		t.Fatalf("want no finding, got:\n%s", found.Text())
	}
}

// TestRegistrationLiteralIsNotAFinding proves a literal that FEEDS a registry
// is the declaration rather than a copy of one. Both shapes are in the fixture:
// a Registration built into a local and registered from it, and a slice built
// where the registrar is called.
func TestRegistrationLiteralIsNotAFinding(t *testing.T) {
	if found := fixtureFindings(t, "declaration"); len(found) != 0 {
		t.Fatalf("want no finding, got:\n%s", found.Text())
	}
}

// TestCanonicalFamilyConstBlockIsReported holds the other side of that line: a
// const block of family names outside the family package READS the key space
// the registry holds, so it stays a finding after the declaration rule lands.
// It is the shape the 2026-09-11 sweep found at
// internal/component/bgp/message/family.go:35.
func TestCanonicalFamilyConstBlockIsReported(t *testing.T) {
	wantOneCopy(t, fixtureFindings(t, "family-consts"), "family names", "FamilyIPv4Unicast")
}

// TestOneLineNestedLiteralsAreOneFinding proves the enclosing-unit filter
// reports the innermost unit once. Two literals that open and close on one line
// share a line range, so a line-based filter called neither the inner one and
// reported the copy twice.
func TestOneLineNestedLiteralsAreOneFinding(t *testing.T) {
	wantOneCopy(t, fixtureFindings(t, "nested-one-line"), "plugin names", "profile")
}

// TestRegisteredConstBlockIsNotAFinding proves the declaration rule reads a
// const block too, and across the package: the codes are declared in one file
// and registered in another.
func TestRegisteredConstBlockIsNotAFinding(t *testing.T) {
	if found := fixtureFindings(t, "const-declaration"); len(found) != 0 {
		t.Fatalf("want no finding, got:\n%s", found.Text())
	}
}

// TestPartialEnumerationMatchIsNotAFinding is the other half of the closed-set
// rule: two of an enumeration's five values are two words the code and the
// model both use, not a transcription. The fixture holds both literals, and
// only the one covering every value is reported.
func TestPartialEnumerationMatchIsNotAFinding(t *testing.T) {
	found := fixtureFindings(t, "yang-enums")
	wantOneCopy(t, found, "YANG enumerations", "speeds")
	if len(found[0].Keys) != 5 {
		t.Fatalf("the finding names %d keys, want all 5 of the enumeration", len(found[0].Keys))
	}
}

// TestAnotherNamespaceIsNotAFinding is the flat-corpus mirror of the closed-set
// rule: a table of twenty service names holding two plugin names is a table of
// service names, and the share of it that is registry keys says so.
func TestAnotherNamespaceIsNotAFinding(t *testing.T) {
	if found := fixtureFindings(t, "other-namespace"); len(found) != 0 {
		t.Fatalf("want no finding, got:\n%s", found.Text())
	}
}

// TestMarkerWithReasonSuppresses is AC-8.
func TestMarkerWithReasonSuppresses(t *testing.T) {
	if found := fixtureFindings(t, "marker-reason"); len(found) != 0 {
		t.Fatalf("want no finding, got:\n%s", found.Text())
	}
}

// TestMarkerWithoutReasonDoesNotSuppress is AC-9: the literal is still
// reported, and the reasonless marker is reported beside it.
func TestMarkerWithoutReasonDoesNotSuppress(t *testing.T) {
	found := fixtureFindings(t, "marker-noreason")
	wantOneCopy(t, found, "family names", "shipped")
	said := false
	for _, finding := range found {
		if finding.Kind == KindMarker && strings.Contains(finding.Detail, "states no reason") {
			said = true
		}
	}
	if !said {
		t.Fatalf("the run does not say the marker states no reason:\n%s", found.Text())
	}
}

// TestDeadExemptionIsReported is AC-10: a marker that suppresses nothing is
// itself a finding, so an exemption cannot rot in place.
func TestDeadExemptionIsReported(t *testing.T) {
	found := fixtureFindings(t, "dead-marker")
	if len(found) != 1 {
		t.Fatalf("want one finding, got %d:\n%s", len(found), found.Text())
	}
	if found[0].Kind != KindMarker || !strings.Contains(found[0].Detail, "suppresses nothing") {
		t.Fatalf("want the dead-marker finding, got %+v", found[0])
	}
}

// TestCorpusFloorFailsClosed is AC-6, driven through the function the command
// runs: a registry that answers short refuses the run rather than passing every
// literal in the tree.
func TestCorpusFloorFailsClosed(t *testing.T) {
	short := []Corpus{{
		Name:     "plugin names",
		Producer: "registry.All()",
		Floor:    10_000,
		Groups:   []Group{{Name: "plugin names", Keys: []string{"as112"}}},
	}}
	found, code := walkTree(checkoutRoot(t), short, 0)
	if code != 2 {
		t.Fatalf("a short corpus answered code %d, want 2", code)
	}
	if len(found) != 0 {
		t.Fatalf("a short corpus answered %d findings, want none", len(found))
	}
}

// TestCorpusFloorNamesTheCorpus proves the refusal says which registry did not
// answer, because the reader has five to check.
func TestCorpusFloorNamesTheCorpus(t *testing.T) {
	err := checkFloors([]Corpus{{Name: "family names", Producer: "family.RegisteredFamilyNames()", Floor: 5}})
	if !errors.Is(err, ErrShortCorpus) {
		t.Fatalf("a short corpus answered %v, want ErrShortCorpus", err)
	}
	if !strings.Contains(err.Error(), "family names") {
		t.Errorf("the refusal says %q, want the corpus named", err.Error())
	}
}

// TestScanFloorFailsClosed is AC-7: a walk that read fewer files than the floor
// refuses rather than answering the clean-tree verdict.
func TestScanFloorFailsClosed(t *testing.T) {
	found, code := walkTree(checkoutRoot(t), liveCorpora(t), 1_000_000)
	if code != 2 {
		t.Fatalf("a short walk answered code %d, want 2", code)
	}
	if len(found) != 0 {
		t.Fatalf("a short walk answered %d findings, want none", len(found))
	}
}

// TestScanFloorNamesTheFloor proves the refusal carries both counts, so the
// reader can tell a moved repository from a shrunken one.
func TestScanFloorNamesTheFloor(t *testing.T) {
	_, err := Check(filepath.Join("testdata", "plugin-names"), []string{"."}, liveCorpora(t), 50)
	if !errors.Is(err, ErrShortScan) {
		t.Fatalf("a short walk answered %v, want ErrShortScan", err)
	}
	if !strings.Contains(err.Error(), "50") {
		t.Errorf("the refusal says %q, want the floor named", err.Error())
	}
}

// TestCorporaMeetTheirFloors is assumption A-2: the le binary's build tags
// expose the whole registry, so every corpus clears its floor in this process.
func TestCorporaMeetTheirFloors(t *testing.T) {
	corpora := liveCorpora(t)
	if len(corpora) != 5 {
		t.Fatalf("Corpora answered %d corpora, want 5", len(corpora))
	}
	if err := checkFloors(corpora); err != nil {
		t.Fatalf("a live corpus came back short: %v", err)
	}
	for _, corpus := range corpora {
		t.Logf("%s: %d keys in %d group(s), floor %d", corpus.Name, corpus.keyCount(), len(corpus.Groups), corpus.Floor)
	}
}

// TestHandCalledDoctorCheckIsReported is AC-13, over a fixture doctor package:
// a check the runner reaches by name is reported, and one the registry names is
// not.
func TestHandCalledDoctorCheckIsReported(t *testing.T) {
	found, err := handCalledDoctorChecks(filepath.Join("testdata", "doctor"))
	if err != nil {
		t.Fatalf("read the fixture doctor package: %v", err)
	}
	named := map[string]bool{}
	for _, finding := range found {
		if finding.Kind != KindDoctorCheck {
			t.Fatalf("want a doctor-check finding, got %+v", finding)
		}
		named[finding.Symbol] = true
		if !strings.Contains(finding.Detail, "runChecks") {
			t.Errorf("the finding says %q, want the runner named", finding.Detail)
		}
	}
	for _, want := range []string{"checkDisk", "checkClock"} {
		if !named[want] {
			t.Errorf("%s is hand-called and was not reported", want)
		}
	}
	if named["checkRegistered"] {
		t.Error("checkRegistered is reached through the registry and was reported anyway")
	}
}

// TestDoctorRunnerRefusesAnUnreadablePackage proves the second anchor fails
// closed: a tree with no doctor runner refuses rather than reporting that no
// check is hand-called, which is what a fixed tree reports.
func TestDoctorRunnerRefusesAnUnreadablePackage(t *testing.T) {
	_, err := handCalledDoctorChecks(filepath.Join("testdata", "plugin-names"))
	if !errors.Is(err, ErrNoDoctorRunner) {
		t.Fatalf("a tree with no doctor package answered %v, want ErrNoDoctorRunner", err)
	}
}

// TestHandCalledDoctorChecksOverTheCheckout proves the anchors still resolve in
// the tree the gate is run over. A gate whose anchor has been renamed reports
// nothing, which reads as a repaired component.
func TestHandCalledDoctorChecksOverTheCheckout(t *testing.T) {
	found, err := handCalledDoctorChecks(checkoutRoot(t))
	if err != nil {
		t.Fatalf("read the doctor package: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("no hand-called doctor check was found, and the component holds about forty")
	}
	t.Logf("%d hand-called doctor checks", len(found))
}

// changeSetRows is the two rows the scoping tests judge: one under
// internal/component/bgp, one far from it.
func changeSetRows() Findings {
	return Findings{
		{Kind: KindLiteral, File: "internal/component/bgp/message/family.go", Line: 35, Corpus: "family names"},
		{Kind: KindLiteral, File: "internal/core/portname/services_table.go", Line: 14, Corpus: "plugin names"},
	}
}

// noWorkingTree is the diff route made unavailable, for a test that must not
// reach it.
func noWorkingTree() ([]string, error) { return nil, errors.New("git is not reachable") }

// TestCheckJudgesOnlyTheChangeSet is the first route: the selector said what
// moved, so the findings are filtered by package and the diff is never asked.
func TestCheckJudgesOnlyTheChangeSet(t *testing.T) {
	report, err := inChangeSet(changeSetRows(), changed.ScopeReport{Packages: []string{"./internal/component/bgp/..."}}, noWorkingTree)
	if err != nil {
		t.Fatalf("scope the findings: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].File != "internal/component/bgp/message/family.go" {
		t.Fatalf("the change set kept %d row(s), want the one under internal/component/bgp", len(report.Findings))
	}
	if !strings.Contains(report.Scope, "package") {
		t.Errorf("the scope reads %q, want it to name the selector's package answer", report.Scope)
	}
}

// TestWidenedChangeSetFallsBackToTheWorkingTree is the second route. The
// selector widening means it cannot tell what moved, and reading that as
// "everything is new" would judge every copy already in the tree on every run,
// which is whole-tree blocking under a change-set name.
func TestWidenedChangeSetFallsBackToTheWorkingTree(t *testing.T) {
	diff := func() ([]string, error) {
		return []string{"internal/core/portname/services_table.go", "docs/guide/cli.md"}, nil
	}
	report, err := inChangeSet(changeSetRows(), changed.ScopeReport{Packages: []string{"./..."}, Widened: true, Reason: "no green commit"}, diff)
	if err != nil {
		t.Fatalf("scope the findings: %v", err)
	}
	if len(report.Findings) != 1 || report.Findings[0].File != "internal/core/portname/services_table.go" {
		t.Fatalf("the fallback kept %d row(s), want the one file the diff names", len(report.Findings))
	}
	for _, want := range []string{"working tree", "widened", "no green commit"} {
		if !strings.Contains(report.Scope, want) {
			t.Errorf("the scope reads %q, want it to say %q", report.Scope, want)
		}
	}
}

// TestBothRoutesUnavailableFailsClosed is the third path: the selector cannot
// say what moved and the working tree cannot be read either. Two silences are
// not an answer.
func TestBothRoutesUnavailableFailsClosed(t *testing.T) {
	_, err := inChangeSet(changeSetRows(), changed.ScopeReport{Packages: []string{"./..."}, Widened: true}, noWorkingTree)
	if !errors.Is(err, ErrNoChangeSet) {
		t.Fatalf("both routes failing answered %v, want ErrNoChangeSet", err)
	}
}

// TestEmptyChangeSetFailsClosed guards the defect this gate is named after: a
// scoped check that scopes to zero passes every literal in the tree and prints
// what a clean tree prints.
func TestEmptyChangeSetFailsClosed(t *testing.T) {
	if _, err := inChangeSet(changeSetRows(), changed.ScopeReport{}, noWorkingTree); !errors.Is(err, ErrNoChangeSet) {
		t.Fatalf("an empty change set answered %v, want ErrNoChangeSet", err)
	}
}

// TestEnumerationActionIsRegistered is the wiring row: `le enumeration check`
// typed in a shell reaches Answer through the registered action table.
func TestEnumerationActionIsRegistered(t *testing.T) {
	if !leroot.Owns(area) {
		t.Fatalf("le does not own %q, so nothing dispatches to it", area)
	}
	if leroot.LookupCommand(area) == nil {
		t.Fatalf("no handler is registered at %q", leroot.CommandPath(area))
	}
	shape, declared := command.ShapeForCommand(leroot.CommandPath(area))
	if !declared {
		t.Fatal("the command declared no answer shape")
	}
	// ShapeDoc, because every answer carries the findings AND the scope that
	// produced them, and a consumer reading an empty row set still needs to see
	// what was judged.
	if shape != command.ShapeDoc {
		t.Fatalf("the command declared the shape %v, want ShapeDoc", shape)
	}
	verbs := map[string]bool{}
	for _, action := range Actions().Actions {
		verbs[action.Verb] = true
	}
	for _, want := range []string{"check", "report"} {
		if !verbs[want] {
			t.Errorf("the action table holds no %q", want)
		}
	}
}

// liveCorpora reads the five registries, which a test never expects to fail.
func liveCorpora(t *testing.T) []Corpus {
	t.Helper()
	corpora, err := readCorpora()
	if err != nil {
		t.Fatalf("read the registries: %v", err)
	}
	return corpora
}

// checkoutRoot answers the repository root, which is the tree the command runs
// over.
func checkoutRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	return root
}

// publishScope writes the package answer a verify run publishes for this
// checkout and names it to the selector, exactly as publishChangeScope does
// (internal/le/verify/engine/scope.go).
func publishScope(t *testing.T, root string, packages []string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scope-packages.txt")
	if err := changed.WriteScopePackages(path, root, packages); err != nil {
		t.Fatalf("publish the scope answer: %v", err)
	}
	previous := env.Get(changed.ScopeFileKey)
	if err := env.Set(changed.ScopeFileKey, path); err != nil {
		t.Fatalf("name the scope answer: %v", err)
	}
	t.Cleanup(func() { _ = env.Set(changed.ScopeFileKey, previous) })
}

// checkReport runs the check action and asserts it answered a report rather
// than a refusal.
func checkReport(t *testing.T) (CheckReport, int) {
	t.Helper()
	answer, code := runCheck()
	if code == 2 {
		t.Fatalf("the check refused the run")
	}
	report, ok := answer.(CheckReport)
	if !ok {
		t.Fatalf("the check answered %T, want a CheckReport", answer)
	}
	return report, code
}

// TestAPublishedWidenedScopeStillFallsBackToTheWorkingTree is the route the
// gate really takes: inside `le verify worktree` the change set is selected once
// and PUBLISHED, and changed.WriteScopePackages carries the packages without the
// Widened flag. Believing the flag made the stage judge all 230 copies already
// in the tree and exit 1 on every session, which is whole-tree blocking under a
// change-set label, and a gate red for every session gets disarmed.
func TestAPublishedWidenedScopeStillFallsBackToTheWorkingTree(t *testing.T) {
	root := checkoutRoot(t)
	publishScope(t, root, []string{"./..."})

	report, _ := checkReport(t)
	if !strings.Contains(report.Scope, "working tree") {
		t.Fatalf("the check judged %q, want the working-tree fallback: a published `./...` is the selector saying it cannot tell what moved", report.Scope)
	}
}

// TestAPublishedNarrowScopeJudgesOnlyThatPackage is AC-14 and AC-15 through the
// action itself: a change set naming one package that holds no copy answers no
// row and exit 0, whatever the rest of the tree holds.
func TestAPublishedNarrowScopeJudgesOnlyThatPackage(t *testing.T) {
	root := checkoutRoot(t)
	// The gate's own package: it owns no registry and copies none, and a row
	// appearing in it would be this gate judging itself.
	publishScope(t, root, []string{"./internal/le/enumeration"})

	report, code := checkReport(t)
	if code != 0 {
		t.Fatalf("the check answered %d over a change set that introduces no copy, want 0:\n%s", code, report.Text())
	}
	if len(report.Findings) != 0 {
		t.Fatalf("the check reported %d row(s) outside its change set:\n%s", len(report.Findings), report.Text())
	}
	if !strings.Contains(report.Scope, "package") {
		t.Errorf("the answer reads %q, want it to name the scope that produced it", report.Scope)
	}
}

// TestReportAnswersTheWholeTreeAtExitZero is AC-16 and AC-20: a measurement is
// not a gate, so every row in the tree comes back at exit 0, carrying the scope
// that produced it.
func TestReportAnswersTheWholeTreeAtExitZero(t *testing.T) {
	answer, code := runReport()
	if code != 0 {
		t.Fatalf("report answered %d, want 0: a measurement is not a gate", code)
	}
	report, ok := answer.(CheckReport)
	if !ok {
		t.Fatalf("report answered %T, want a CheckReport", answer)
	}
	if len(report.Findings) == 0 {
		t.Fatal("report answered no row over a tree that holds copies, so it read nothing")
	}
	if !strings.Contains(report.Scope, "every Go file") {
		t.Errorf("report reads %q, want it to say it judged the whole tree", report.Scope)
	}
}

// TestATwoValueEnumerationIsNotAFinding holds closedGroupKeysMin closed. A
// closed corpus reports a literal that covers a whole enumeration, and the
// smallest enumerations are covered by accident: `true`/`false`, `in`/`out` and
// `ipv4`/`ipv6` are each an enumeration somewhere in the model, and every piece
// of code that talks about a boolean holds the first pair.
func TestATwoValueEnumerationIsNotAFinding(t *testing.T) {
	if found := fixtureFindings(t, "two-value-enum"); len(found) != 0 {
		t.Fatalf("want no finding, got:\n%s", found.Text())
	}
}
