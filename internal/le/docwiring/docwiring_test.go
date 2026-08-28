// VALIDATES: spec-le-is-a-ze-binary AC-11 -- the changed-file router selects
// the same gates the script selects, and every failure it reports names the
// files it is about.
// PREVENTS: a router that reads fewer files than it claims, which passes a
// change nobody checked.

package docwiring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestExportedSymbolsReadsEachDeclarationForm(t *testing.T) {
	const content = `package example

func Exported() {}
func unexported() {}
type Kind struct{}
const One = 1
var Two = 2

const (
	Three = 3
	four  = 4
)
`
	symbols := exportedSymbols("internal/example/example.go", content)

	want := map[string]string{
		"Exported": "func",
		"Kind":     "type",
		"One":      "const",
		"Two":      "var",
		"Three":    "const",
	}
	got := make(map[string]string, len(symbols))
	for _, sym := range symbols {
		got[sym.Name] = sym.Kind
	}
	if len(got) != len(want) {
		t.Fatalf("read %d exported symbols, want %d: %+v", len(got), len(want), symbols)
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("%s is a %q, want %q", name, got[name], kind)
		}
	}
}

// tree writes a fixture checkout and answers its root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	return root
}

func TestExportedSymbolsIgnoresACommentAndAnUnexportedBlockMember(t *testing.T) {
	const content = `package example

// Exported is not a declaration here.
const (
	Kept   = 1
	hidden = 2
)
`
	symbols := exportedSymbols("internal/example/example.go", content)
	if len(symbols) != 1 || symbols[0].Name != "Kept" {
		t.Errorf("read %+v, want only Kept", symbols)
	}
}

func TestParseSleepBaselineSumsTheDeltaLedger(t *testing.T) {
	for _, tc := range []struct {
		text   string
		want   int
		active bool
	}{
		{"125\n", 125, true},
		{"# a comment\n130\n-3\n-2\n", 125, true},
		{"130\n+5\n", 135, true},
		{"# nothing but prose\n", 0, false},
		{"", 0, false},
	} {
		got, active := parseSleepBaseline(tc.text)
		if got != tc.want || active != tc.active {
			t.Errorf("parseSleepBaseline(%q) = (%d, %v), want (%d, %v)", tc.text, got, active, tc.want, tc.active)
		}
	}
}

func TestTheRatchetRefusesATreeItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the case cannot be built")
	}
	root := tree(t, map[string]string{
		"test/.ci-sleep-baseline": "2\n",
		"test/ui/a.ci":            "time.sleep(1)  # why\n",
		"test/ui/b.ci":            "time.sleep(1)  # why\n",
	})
	if err := os.Chmod(filepath.Join(root, "test", "ui", "b.ci"), 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	report, code := Run(root, Options{Changed: []string{"test/ui/a.ci"}})
	if code == 0 {
		t.Fatalf("the ratchet passed over a tree it could not read: %+v", report)
	}

	// This assertion tests the RATCHET, not the complete run. Two other checks
	// read the same .ci files. An exit-code-only test CAN pass for their failures
	// while the ratchet counts only readable files.
	var ratchet *CheckResult
	for i, check := range report.Checks {
		if check.Name == checkSleepRatchetName {
			ratchet = &report.Checks[i]
		}
	}
	if ratchet == nil {
		t.Fatal("the ratchet did not run at all")
	}
	if !ratchet.Failed {
		t.Errorf("the ratchet answered %+v over a tree it could not read", *ratchet)
	}
	// An unreadable file contributes no sleep, and fewer sleeps is what PASSING
	// looks like. Naming the file is what makes the refusal actionable.
	if !strings.Contains(ratchet.Message, "b.ci") {
		t.Errorf("the refusal does not name the file: %q", ratchet.Message)
	}
	if len(report.Groups) == 0 {
		t.Error("the failure declared no group, so nobody is charged for it")
	}
}

func TestTheRatchetCountsExactlyAtItsCeiling(t *testing.T) {
	// The ceiling is a bound, so the case AT it passes and the case one above
	// it fails. The draft incubator is excluded from both counts.
	root := tree(t, map[string]string{
		"test/.ci-sleep-baseline": "2\n",
		"test/ui/a.ci":            "time.sleep(1)  # why\ntime.sleep(2)  # why\n",
		"test/draft/hidden.ci":    "time.sleep(1)\ntime.sleep(1)\ntime.sleep(1)\n",
	})

	if _, code := Run(root, Options{Changed: []string{"test/ui/a.ci"}}); code != 0 {
		t.Error("a tree at the ceiling failed the ratchet, so the draft incubator was counted")
	}

	if err := os.WriteFile(filepath.Join(root, "test", "ui", "b.ci"),
		[]byte("time.sleep(3)  # why\n"), 0o600); err != nil {
		t.Fatalf("writing one sleep past the ceiling: %v", err)
	}
	report, code := Run(root, Options{Changed: []string{"test/ui/a.ci"}})
	if code == 0 {
		t.Errorf("a tree one sleep past the ceiling passed:\n%s", report.Text())
	}
}

func TestASleepIsJustifiedByACommentAboveOrBesideIt(t *testing.T) {
	lines := []string{
		"# the reason",
		"time.sleep(1)",
		"",
		"time.sleep(2)  # a trailing reason",
		"",
		"time.sleep(3)",
	}
	for _, tc := range []struct {
		idx  int
		want bool
	}{{1, true}, {3, true}, {5, false}} {
		if got := sleepIsJustified(lines, tc.idx); got != tc.want {
			t.Errorf("sleepIsJustified(line %d) = %v, want %v", tc.idx+1, got, tc.want)
		}
	}
}

func TestAFailingCheckNamesTheFilesItIsAbout(t *testing.T) {
	shard := "plan/known-failures/one.md"
	root := tree(t, map[string]string{
		"test/.ci-sleep-baseline": "9\n",
		"test/ui/blind.ci":        "time.sleep(1)\n",
		shard:                     "It fails under load.\n",
	})

	report, code := Run(root, Options{Changed: []string{"test/ui/blind.ci", shard}})
	if code == 0 {
		t.Fatalf("both violations passed:\n%s", report.Text())
	}
	related := make(map[string]bool)
	for _, group := range report.Groups {
		if group.Kind != pathBearingKind {
			t.Errorf("a group that names files carries the kind %q", group.Kind)
		}
		for _, path := range group.Related {
			related[path] = true
		}
	}
	for _, want := range []string{"test/ui/blind.ci", shard} {
		if !related[want] {
			t.Errorf("no group names %s, so its red is charged to whoever is committing", want)
		}
	}
}

func TestAGroupSplitsWhenItNamesMorePathsThanOneLineCarries(t *testing.T) {
	// The verify log is read with a token limit, so a group carries at most
	// RelatedPerGroup paths and the rest go to sibling groups. Nothing is
	// dropped: a truncated list would hide the file that makes the red this
	// session's.
	var many []string
	for i := range RelatedPerGroup + 1 {
		many = append(many, "a"+strconv.Itoa(i))
	}
	g := &checker{}
	g.declareFailureGroup("check", many, "summary", "rerun")

	if len(g.report.Groups) != 2 {
		t.Fatalf("%d paths became %d group(s), want two", len(many), len(g.report.Groups))
	}
	if got := len(g.report.Groups[0].Related) + len(g.report.Groups[1].Related); got != len(many) {
		t.Errorf("the groups carry %d paths of %d", got, len(many))
	}
	if g.report.Groups[1].GroupID != "files:check#2" {
		t.Errorf("the sibling group is %q", g.report.Groups[1].GroupID)
	}

	// One path fewer stays in a single group, which is what makes the split a
	// bound rather than a habit.
	g = &checker{}
	g.declareFailureGroup("check", many[:RelatedPerGroup], "summary", "rerun")
	if len(g.report.Groups) != 1 {
		t.Errorf("%d paths became %d groups, want one", RelatedPerGroup, len(g.report.Groups))
	}
}

func TestACheckThatDeclaresNothingIsStillCharged(t *testing.T) {
	// A failure that declared no group would publish a count that agrees with
	// itself, and the reader would then drop the red.
	g := &checker{}
	code := g.runCheck("silent", "rerun", func() CheckResult {
		return CheckResult{Failed: true, Message: "it failed"}
	})

	if code == 0 {
		t.Error("a failing check answered 0")
	}
	if len(g.report.Groups) != 1 {
		t.Fatalf("a silent failure declared %d group(s), want one", len(g.report.Groups))
	}
	if g.report.Groups[0].Kind != unattributableKind {
		t.Errorf("the group's kind is %q, want the one the commit gate charges", g.report.Groups[0].Kind)
	}
}

func TestAnUnknownTargetIsRefusedRatherThanRun(t *testing.T) {
	g := &checker{root: t.TempDir()}
	result := g.runAction("not-an-action")

	if !result.Failed || result.Code != 2 ||
		!strings.Contains(result.Message, "no native callback") {
		t.Errorf("an undeclared target answered %+v", result)
	}
}

func TestANativeCallbackCannotPassWithoutAResult(t *testing.T) {
	g := &checker{root: t.TempDir()}
	result := g.runGoAction("silent", call{
		answer: func(string) (any, int) { return nil, 0 },
	})
	if !result.Failed || result.Code != 2 ||
		!strings.Contains(result.Message, "returned no result") {
		t.Errorf("a callback with no result answered %+v", result)
	}
}

func TestTheDesignRefCheckerIsDeclaredAsAFork(t *testing.T) {
	root := nativeGitTree(t, map[string]string{
		"go.mod":               "module fixture\n",
		"internal/x/source.go": "// Design: docs/missing.md\npackage x\n",
	})
	g := &checker{root: root}
	g.run()
	var design CheckResult
	for _, result := range g.report.Checks {
		if result.Name == checkDesignRefsName {
			design = result
			break
		}
	}
	if design.Name == "" {
		t.Fatal("the complete run omitted the design-reference checker")
	}
	if !design.Failed {
		t.Fatalf("the complete run passed a broken Design reference: %+v", design)
	}
	if !strings.Contains(design.Output, "internal/x/source.go:1: broken Design reference: docs/missing.md") {
		t.Errorf("design checker output = %q", design.Output)
	}
}

func TestAGoTargetRunsInThisProcessAndAMakeTargetDoesNot(t *testing.T) {
	root := t.TempDir()
	called := false
	g := &checker{root: root}
	native := g.runGoAction("native", call{answer: func(gotRoot string) (any, int) {
		called = gotRoot == root
		return docVerifyPage{text: "native output\n"}, 0
	}})
	if !called {
		t.Error("the Go target did not receive the router's repository root")
	}
	if native.Failed || native.Code != 0 {
		t.Errorf("the Go target answered %+v", native)
	}
	if native.Output != "native output\n" {
		t.Errorf("the Go target output = %q", native.Output)
	}
	if native.Message != "native PASSED" {
		t.Errorf("the Go target message = %q", native.Message)
	}

	missing := g.runAction("missing/action")
	if !missing.Failed || missing.Code != 2 {
		t.Errorf("an undeclared action answered %+v", missing)
	}
	if !strings.Contains(missing.Message, "no native callback for action missing/action") {
		t.Errorf("undeclared action message = %q", missing.Message)
	}
}

func TestTheAdvisoryFiresOnlyWhenNoTestChanged(t *testing.T) {
	changed := []string{"internal/component/cli/thing.go"}
	if functionalTestAdvisory(changed) == "" {
		t.Error("a user-facing change with no test change drew no advisory")
	}
	if got := functionalTestAdvisory(append(changed, "test/ui/thing.ci")); got != "" {
		t.Errorf("a change carrying a test drew an advisory: %q", got)
	}
	if got := functionalTestAdvisory([]string{"internal/component/bgp/reactor/peer.go"}); got != "" {
		t.Errorf("an area outside the table drew an advisory: %q", got)
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(Report{
		Changed: []string{"a.go"},
		Actions: []string{"wiring"},
		Checks:  []CheckResult{{Name: "ci-sleep-ratchet", Message: "ok"}},
		Groups:  []Group{{GroupID: "files:wiring", Kind: "files", Related: []string{"a.go"}}},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, want := range []string{`"changed"`, `"actions"`, `"checks"`, `"failure-groups"`, `"declared-groups"`, `"group-id"`, `"dry-run"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the payload has no %s key: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is snake_case: %s", raw)
	}
}

func TestTheGrammarTakesEveryValueBehindAKeyword(t *testing.T) {
	opts, code, ok := parseOptions([]string{"changed-file", "dry-run", "changed-file", "b.go"})
	if !ok || code != 0 {
		t.Fatalf("a legal command line was refused with %d", code)
	}
	// The first `changed-file` consumes exactly one word, so a file NAMED like a
	// keyword is still a legal value.
	if len(opts.Changed) != 2 || opts.Changed[0] != "dry-run" || opts.Changed[1] != "b.go" {
		t.Errorf("the changed files are %v", opts.Changed)
	}
	if opts.DryRun {
		t.Error("a value read as a flag")
	}

	if _, code, ok := parseOptions([]string{"changed-file"}); ok || code == 0 {
		t.Error("a keyword with nothing after it was accepted")
	}
	if _, code, ok := parseOptions([]string{"make", "false"}); ok || code == 0 {
		t.Error("the removed process-runner option was accepted")
	}
	if _, code, ok := parseOptions([]string{"a.go"}); ok || code == 0 {
		t.Error("a bare value was accepted")
	}
}

// VALIDATES: changes to the native orphan owner select the templ output gate.
// PREVENTS: routing changes to a deleted script while native checker edits skip
// their own gate.
func TestTheNativeTemplCheckerSelectsTheOutputGate(t *testing.T) {
	if !isTemplSource(templCheckerSource) {
		t.Errorf("%s does not select ze-templ-output-check", templCheckerSource)
	}
	if isTemplSource("internal/le/docwiring/native_test.go") {
		t.Error("an unrelated test file selects the templ output gate")
	}
}

func TestAChangedFileTheRouterCannotReadIsRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the case cannot be built")
	}
	// A changed file the router cannot read selects NO gate, so the change is
	// judged by nobody and the run still reads green. It is the one failure a
	// changed-file router can have, and it is why an unreadable file is told
	// apart from an absent one.
	root := tree(t, map[string]string{
		"internal/component/config/yang/thing.yang": "// ze:command\n",
	})
	unreadable := filepath.Join(root, "internal", "component", "config", "yang", "thing.yang")
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if _, err := selectedActions(root, []string{"internal/component/config/yang/thing.yang"}); err == nil {
		t.Error("the router selected gates for a tree it could not read")
	}

	// An ABSENT file is the other half of this question. It is not an error
	// because a caller uses this state for a path that the change deleted.
	if _, err := selectedActions(root, []string{"internal/component/config/yang/gone.yang"}); err != nil {
		t.Errorf("a deleted file was refused: %v", err)
	}
}

func TestTheRatchetRefusesABaselineItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the case cannot be built")
	}
	// A baseline that is present and unreadable turns the ratchet OFF, and any
	// number of new sleeps then passes. An ABSENT baseline is the other fact:
	// the ratchet is not active for that tree at all.
	root := tree(t, map[string]string{
		"test/.ci-sleep-baseline": "0\n",
		"test/ui/a.ci":            "time.sleep(1)  # why\n",
	})
	baseline := filepath.Join(root, "test", ".ci-sleep-baseline")
	if err := os.Chmod(baseline, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if _, code := Run(root, Options{Changed: []string{"test/ui/a.ci"}}); code == 0 {
		t.Error("the ratchet turned itself off over a baseline it could not read")
	}

	if err := os.Remove(baseline); err != nil {
		t.Fatalf("removing the baseline: %v", err)
	}
	if _, code := Run(root, Options{Changed: []string{"test/ui/a.ci"}}); code != 0 {
		t.Error("a tree that commits no baseline was refused")
	}
}

// TestEveryDelegatedTargetIsAnsweredHereOrDeclaredAFork keeps the selected
// target table total. The cutover answers every former fork as a native call.
func TestEveryDelegatedTargetIsAnsweredHereOrDeclaredAFork(t *testing.T) {
	for _, target := range actionOrderList() {
		if target == wiringTarget {
			continue
		}
		one, found := goActions[target]
		if !found {
			t.Errorf("%s has no native callback", target)
			continue
		}
		if one.answer == nil {
			t.Errorf("%s has a nil native callback", target)
		}
	}
	if got, want := len(goActions), len(actionOrderList())-1; got != want {
		t.Fatalf("native target table has %d rows, want %d", got, want)
	}
}
