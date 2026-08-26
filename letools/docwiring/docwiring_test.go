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
	symbols := ExportedSymbols("internal/example/example.go", content)

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
	symbols := ExportedSymbols("internal/example/example.go", content)
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
		got, active := ParseSleepBaseline(tc.text)
		if got != tc.want || active != tc.active {
			t.Errorf("ParseSleepBaseline(%q) = (%d, %v), want (%d, %v)", tc.text, got, active, tc.want, tc.active)
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
	g := &gate{}
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
	g = &gate{}
	g.declareFailureGroup("check", many[:RelatedPerGroup], "summary", "rerun")
	if len(g.report.Groups) != 1 {
		t.Errorf("%d paths became %d groups, want one", RelatedPerGroup, len(g.report.Groups))
	}
}

func TestACheckThatDeclaresNothingIsStillCharged(t *testing.T) {
	// A failure that declared no group would publish a count that agrees with
	// itself, and the reader would then drop the red.
	g := &gate{}
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
	g := &gate{root: t.TempDir(), opts: Options{Make: "false"}}
	result := g.runMakeTarget("ze-not-a-target")

	if !result.Failed || !strings.Contains(result.Message, "unknown make target") {
		t.Errorf("an undeclared target answered %+v", result)
	}
}

func TestTheAdvisoryFiresOnlyWhenNoTestChanged(t *testing.T) {
	changed := []string{"internal/component/cli/thing.go"}
	if FunctionalTestAdvisory(changed) == "" {
		t.Error("a user-facing change with no test change drew no advisory")
	}
	if got := FunctionalTestAdvisory(append(changed, "test/ui/thing.ci")); got != "" {
		t.Errorf("a change carrying a test drew an advisory: %q", got)
	}
	if got := FunctionalTestAdvisory([]string{"internal/component/bgp/reactor/peer.go"}); got != "" {
		t.Errorf("an area outside the table drew an advisory: %q", got)
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	raw, err := json.Marshal(Report{
		Changed: []string{"a.go"},
		Targets: []string{"wiring"},
		Checks:  []CheckResult{{Name: "ci-sleep-ratchet", Message: "ok"}},
		Groups:  []Group{{GroupID: "files:wiring", Kind: "files", Related: []string{"a.go"}}},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, want := range []string{`"changed"`, `"targets"`, `"checks"`, `"failure-groups"`, `"declared-groups"`, `"group-id"`, `"dry-run"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the payload has no %s key: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "_") {
		t.Errorf("a JSON key is snake_case: %s", raw)
	}
}

func TestTheGrammarTakesEveryValueBehindAKeyword(t *testing.T) {
	opts, code, ok := parseOptions([]string{"changed-file", "dry-run", "changed-file", "b.go", "make", "gmake"})
	if !ok || code != 0 {
		t.Fatalf("a legal command line was refused with %d", code)
	}
	// The first `changed-file` consumes exactly one word, so a file NAMED like a
	// keyword is still a legal value.
	if len(opts.Changed) != 2 || opts.Changed[0] != "dry-run" || opts.Changed[1] != "b.go" {
		t.Errorf("the changed files are %v", opts.Changed)
	}
	if opts.Make != "gmake" {
		t.Errorf("the make executable is %q", opts.Make)
	}
	if opts.DryRun {
		t.Error("a value read as a flag")
	}

	if _, code, ok := parseOptions([]string{"changed-file"}); ok || code == 0 {
		t.Error("a keyword with nothing after it was accepted")
	}
	if _, code, ok := parseOptions([]string{"a.go"}); ok || code == 0 {
		t.Error("a bare value was accepted")
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

	if _, err := SelectedTargets(root, []string{"internal/component/config/yang/thing.yang"}); err == nil {
		t.Error("the router selected gates for a tree it could not read")
	}

	// An ABSENT file is the other half of this question. It is not an error
	// because a caller uses this state for a path that the change deleted.
	if _, err := SelectedTargets(root, []string{"internal/component/config/yang/gone.yang"}); err != nil {
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

// TestEveryDelegatedTargetIsAnsweredHereOrDeclaredAFork tests the router's
// claim. Each target is either work in this binary (delegate.go) or a published
// child process (forks.go). A target in neither set would use an undeclared
// `make` invocation. letools/parity would still count the eight claimed targets
// as converted work.
func TestEveryDelegatedTargetIsAnsweredHereOrDeclaredAFork(t *testing.T) {
	forked := map[string]bool{}
	for _, argv := range Forks() {
		if len(argv) == 2 && argv[0] == defaultMake {
			forked[argv[1]] = true
		}
	}

	for _, target := range TargetOrder() {
		if target == wiringTarget {
			continue
		}
		_, inGo := goTargets[target]
		switch {
		case inGo && forked[target]:
			t.Errorf("%s is answered in Go and declared as a fork", target)
		case !inGo && !forked[target]:
			t.Errorf("%s is neither answered in Go nor declared as a process this gate starts", target)
		}
	}

	if len(goTargets) == 0 {
		t.Fatal("no delegated target is answered in Go, so this case is watching nothing")
	}
}

// TestTheDesignRefCheckerIsDeclaredAsAFork pins the only script that this gate
// starts. The declaration keeps the census status at claimed. Without a Go port,
// removing it would falsely mark the gate as converted.
func TestTheDesignRefCheckerIsDeclaredAsAFork(t *testing.T) {
	var found []string
	for _, argv := range Forks() {
		if len(argv) > 1 && strings.HasSuffix(argv[1], ".py") {
			found = argv
		}
	}
	if len(found) == 0 {
		t.Fatalf("no script is declared, and checks.go still starts one: %v", Forks())
	}
	if found[1] != "scripts/dev/check_doc_links.py" {
		t.Errorf("the declared script is %q, and checkDesignRefs starts scripts/dev/check_doc_links.py", found[1])
	}
}

// TestAGoTargetRunsInThisProcessAndAMakeTargetDoesNot is the behavioral half.
// The make executable is a recorder here, so what it wrote says which of the
// two routes each target took.
func TestAGoTargetRunsInThisProcessAndAMakeTargetDoesNot(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "make-ran")
	recorder := filepath.Join(dir, "make")
	script := "#!/bin/sh\necho \"$@\" >> " + marker + "\n"
	if err := os.WriteFile(recorder, []byte(script), 0o700); err != nil { //nolint:gosec // an executable recorder must be executable
		t.Fatalf("write the recorder: %v", err)
	}

	g := &gate{root: dir, opts: Options{Make: recorder}}
	if result := g.runTarget("ze-command-list-json"); result.Failed {
		t.Errorf("the in-process command list failed: %s", result.Message)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("a target this binary holds started make, so the census counts work that has not moved")
	}

	if result := g.runTarget("ze-doc-verify"); result.Failed {
		t.Errorf("the recorder answered a failure: %s", result.Message)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("a target no Go package holds did not reach make, so nothing ran it")
	}
}
