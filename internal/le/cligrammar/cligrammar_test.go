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
	"strconv"
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
		"demos/terminal/one/demo.tape":   "Type \"ze show interface\"\n",
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

	result, err := Check(tree, DefaultFloor, leProbeRoots())
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

	if _, err := Check(tree, DefaultFloor, leProbeRoots()); err == nil {
		t.Fatal("a tree holding one file per population passed the default floor")
	}

	for _, floor := range []Floor{
		{YANGFiles: 2, Roots: 0, DemoScripts: 0},
		{YANGFiles: 0, Roots: 2, DemoScripts: 0},
		{YANGFiles: 0, Roots: 0, DemoScripts: 2},
		{GoFiles: 2},
		{FlagSets: 1},
	} {
		if _, err := Check(tree, floor, leProbeRoots()); err == nil {
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

	_, err := Check(tree, Floor{}, leProbeRoots())
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
	files["demos/terminal/one/demo.tape"] = "Type \"ze show interface\"\nType \"ze mystery-verb\"\n"
	tree := writeTree(t, files)

	result, err := Check(tree, Floor{}, leProbeRoots())
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
		t.Fatalf("the call-site feeder drew %+v, want the one dead launch form", result.DemoLaunch)
	}
	if result.DemoLaunch[0].File != "demos/terminal/one/demo.tape" {
		t.Errorf("the row names %q, want the tree-relative path", result.DemoLaunch[0].File)
	}
}

// VALIDATES: a comment in a demo definition is prose rather than a call site.
// PREVENTS: false positives that would make the feeder unusable and get it
// switched off.
func TestProseInADemoDefinitionIsNotACallSite(t *testing.T) {
	files := cleanFixture(t)
	files["demos/terminal/one/demo.tape"] = "# ze narrated-verb\nType \"ze show interface\"\n"
	tree := writeTree(t, files)

	result, err := Check(tree, Floor{}, leProbeRoots())
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}
	if len(result.DemoLaunch) != 0 {
		t.Errorf("prose was read as a call site: %+v", result.DemoLaunch)
	}
}

func TestAFlagOnlyZeInvocationStopsAtTheShellBoundary(t *testing.T) {
	got := launchTokens(strings.Fields(`ze --version | ze pipe count`))
	if strings.Join(got, ",") != "pipe" {
		t.Fatalf("launch tokens = %v, want only the second ze command", got)
	}
}

// VALIDATES: AC-7 -- the payload is data a JSON encoder takes, with the native
// report's own keys.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestResultIsStructuredData(t *testing.T) {
	raw, err := json.Marshal(Result{
		Findings:   []grammar.Finding{{Command: "show x", Rule: "R1", Message: "m"}},
		FlagInYANG: []FlagHit{{File: "a.yang", Line: 2, Text: "--x"}},
		DemoLaunch: []DemoLaunchHit{{File: "b.tape", Line: 3, Token: "t"}},
		Exempt:     map[string]int{"bridge": 3},
		FlagFindings: []FlagRegisterHit{{
			File: "c.go", Line: 4,
			Command: "config dump", Rule: "F3", Flag: "--json", Message: "m",
		}},
		FlagDebt: []FlagDebt{{Entry: "F3 config dump --json", Reason: "r", Tracked: 1, Present: 1}},
	})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, key := range []string{
		`"findings"`, `"flag-in-yang"`, `"demo-launch"`, `"exempt-by-category"`,
		`"commands-checked"`, `"demo-scripts-checked"`, `"roots-checked"`,
		`"root-namespace-exempt"`, `"tree-namespace-exempt"`,
		`"pending-namespace-split"`, `"valid"`,
		`"flag-findings"`, `"flag-debt"`, `"flag-sets-read"`, `"flag"`, `"rule"`,
		`"flag-sets-in-scope"`, `"flag-sets-out-of-scope"`,
		`"flag-names-unresolved"`, `"flag-set-names-unresolved"`,
		`"client-literals-served-locally"`, `"go-files-read"`,
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
		FlagFindings: []FlagRegisterHit{{
			File: "c.go", Line: 4,
			Command: "config dump", Rule: "F3", Flag: "--json", Message: "second spelling",
		}},
		FlagDebt: []FlagDebt{
			{Entry: "F4 config set", Reason: "not declared yet", Tracked: 4, Present: 4},
			{Entry: "F1 --plugins", Reason: "fix in flight", Tracked: 1, Present: 0},
		},
	}.Text()
	for _, want := range []string{
		"## Grammar violations (1)", "## --flag in YANG (1)",
		"## Dead launch form in demo scripts (1)",
		"Root namespace-exempt (indivisible compounds): 1",
		"Tree namespace-exempt (indivisible compounds): 2",
		"Pending namespace-split (R9 debt, tracked for rename migration): 3",
		"## Flag in the wrong register (1)", "[F3] config dump  (c.go:4)",
		"## Tracked flag-register debt (2)",
		"F4 config set  -- 4 flags", "F1 --plugins  -- FIXED, delete this entry",
		"cli-grammar: FAILED (1 grammar, 1 flag-in-yang, 1 demo-launch, 1 flag-register)",
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

// leProbeRoots is a le command population for the feeders this file does not
// exercise. The real surface reaches Check from the action, where every le
// package is linked; a test binary linking one package would fail the floor
// for a reason that says nothing about the grammar.
func leProbeRoots() []string {
	roots := make([]string, 0, 64)
	for index := range 64 {
		roots = append(roots, "probe"+strconv.Itoa(index))
	}
	return roots
}

// flagFixture is a tree carrying one violation of each flag-register shape:
// a root spelled as a flag, a client command string with a flag in it, a flag
// that repeats a pipe operator, and a flag no registry declares.
func flagFixture(t *testing.T) map[string]string {
	t.Helper()
	files := cleanFixture(t)
	files["cmd/ze/roots.go"] = "package main\n\n" +
		"func wire() {\n" +
		"\tregistry.MustRegisterRootHandler(\"env\", nil, meta)\n" +
		"\tregistry.MustRegisterRootHandler(\"fixture\", nil, meta)\n" +
		"\tregistry.MustRegisterRootHandler(\"--everything\", nil, meta)\n" +
		"}\n"
	files["internal/fixture/client.go"] = "package fixture\n\n" +
		"const send = \"request interface migrate --from a\"\n"
	files["internal/fixture/tool.go"] = "package fixture\n\n" +
		"func register() {\n" +
		"\tregistry.MustRegisterLocalData(\"show fixture thing\", nil, meta, nil)\n" +
		"}\n\n" +
		"func run(args []string) int {\n" +
		"\tfs := flag.NewFlagSet(\"ze fixture thing\", flag.ContinueOnError)\n" +
		"\tjson := fs.Bool(\"json\", false, \"output as JSON\")\n" +
		"\tdepth := fs.Int(\"depth\", 0, \"how deep to walk\")\n" +
		"\treturn use(json, depth, fs.Parse(args))\n" +
		"}\n"
	return files
}

// rulesFound answers how many findings each rule drew.
func rulesFound(result Result) map[string]int {
	counts := map[string]int{}
	for _, hit := range result.FlagFindings {
		counts[hit.Rule]++
	}
	return counts
}

// VALIDATES: feeder 7 draws a row for each of the four shapes it detects.
// PREVENTS: a check that cannot fail. A gate over an offline surface with no
// violation in it reads identically to a gate that judged nothing.
func TestTheFlagFeederDrawsARowForEachShape(t *testing.T) {
	tree := writeTree(t, flagFixture(t))

	result, err := Check(tree, Floor{}, leProbeRoots())
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}
	if result.Valid {
		t.Fatalf("a tree holding one violation of each flag rule passed:\n%s", result.Text())
	}

	counts := rulesFound(result)
	for _, rule := range []string{
		grammar.RuleFlagIsACommand, grammar.RuleFlagToTheDaemon,
		grammar.RuleFlagIsAPipe, grammar.RuleFlagUndeclared,
	} {
		if counts[rule] == 0 {
			t.Errorf("rule %s drew no row over a tree that violates it:\n%s", rule, result.Text())
		}
	}
	if result.FlagSetsRead != 1 || result.FlagSetsInScope != 1 {
		t.Errorf("the scan read %d flag sets, %d in scope, want the fixture's one",
			result.FlagSetsRead, result.FlagSetsInScope)
	}
}

// VALIDATES: --version, -V, --help and -h pass, and so does a root with no
// hyphen at all.
// PREVENTS: the one exception ai/rules/cli.md names being gated away. A person
// meeting `ze` types one of the four before any help exists to tell them
// otherwise.
func TestTheFourUniversalFlagsPassTheGate(t *testing.T) {
	files := cleanFixture(t)
	files["cmd/ze/roots.go"] = "package main\n\n" +
		"func wire() {\n" +
		"\tregistry.MustRegisterRootHandler(\"--version\", nil, meta)\n" +
		"\tregistry.MustRegisterRootHandler(\"-V\", nil, meta)\n" +
		"\tregistry.MustRegisterRootHandler(\"--help\", nil, meta)\n" +
		"\tregistry.MustRegisterRootHandler(\"-h\", nil, meta)\n" +
		"\tregistry.MustRegisterRootHandler(\"env\", nil, meta)\n" +
		"}\n"
	tree := writeTree(t, files)

	result, err := Check(tree, Floor{}, leProbeRoots())
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}
	if count := rulesFound(result)[grammar.RuleFlagIsACommand]; count != 0 {
		t.Errorf("the four universal flags drew %d findings:\n%s", count, result.Text())
	}
}

// VALIDATES: a flag set outside the ze command surface is counted rather than
// judged, and a flag name no static scan can read is counted rather than
// dropped.
// PREVENTS: silence being read as coverage. A flag the scan never resolved is
// one no feeder judged, and only the count says so.
func TestWhatTheFlagScanCouldNotJudgeIsCounted(t *testing.T) {
	files := cleanFixture(t)
	files["internal/other/tool.go"] = "package other\n\n" +
		"func run(args []string) int {\n" +
		"\tfs := flag.NewFlagSet(\"ze-perf run\", flag.ContinueOnError)\n" +
		"\treturn use(fs.Bool(\"all\", false, \"every case\"), fs.Parse(args))\n" +
		"}\n\n" +
		"func named(args []string, name string) int {\n" +
		"\tfs := flag.NewFlagSet(name, flag.ContinueOnError)\n" +
		"\treturn use(fs.Parse(args))\n" +
		"}\n"
	tree := writeTree(t, files)

	result, err := Check(tree, Floor{}, leProbeRoots())
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}
	if result.FlagSetsOutOfScope != 1 {
		t.Errorf("the scan put %d flag sets outside the ze surface, want the ze-perf one", result.FlagSetsOutOfScope)
	}
	if result.FlagSetNamesUnresolved != 1 {
		t.Errorf("the scan reported %d unreadable flag-set names, want the one built from a variable",
			result.FlagSetNamesUnresolved)
	}
	if count := rulesFound(result)[grammar.RuleFlagUndeclared]; count != 0 {
		t.Errorf("a flag set outside the ze command surface was judged: %d finding(s)", count)
	}
}

// VALIDATES: this checkout's flag-register debt is tracked, counted and
// printed, and every entry states a reason.
// PREVENTS: the debt list becoming an allowlist. An entry with no reason
// forgives a violation nobody can act on, and an entry nobody prints forgives
// it silently.
func TestTheTrackedFlagDebtIsPrintedAndReasoned(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}
	result, err := Check(tree, DefaultFloor, leProbeRoots())
	if err != nil {
		t.Fatalf("the gate could not read this checkout: %v", err)
	}
	if len(result.FlagDebt) == 0 {
		t.Fatal("the debt ledger is empty; this checkout still carries flag-register debt")
	}
	page := result.Text()
	for _, entry := range result.FlagDebt {
		if strings.TrimSpace(entry.Reason) == "" {
			t.Errorf("debt entry %q states no reason", entry.Entry)
		}
		if entry.Tracked < 1 {
			t.Errorf("debt entry %q forgives nothing", entry.Entry)
		}
		if !strings.Contains(page, entry.Entry) {
			t.Errorf("debt entry %q is forgiven but never printed", entry.Entry)
		}
	}
}

// VALIDATES: every `// ze point:` binding on the flag checker names a rule
// point that exists on disk.
// PREVENTS: a dangling binding. `./le rules gate-map-report` reads only
// internal/le/hookruntime, so a binding on a gate's own checker is read by
// nothing else: without this test, a renamed or deleted point would leave the
// comment claiming an enforcement nobody can find.
func TestEveryPointBindingOnTheFlagCheckerResolves(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}
	source, err := os.ReadFile(filepath.Join(tree, "internal", "component", "command", "grammar", "flags.go"))
	if err != nil {
		t.Fatalf("read the flag checker: %v", err)
	}

	found := 0
	for line := range strings.SplitSeq(string(source), "\n") {
		_, ref, isBinding := strings.Cut(strings.TrimSpace(line), "// ze point:")
		if !isBinding {
			continue
		}
		ref = strings.TrimSpace(ref)
		if ref == "" {
			t.Error("a `// ze point:` line names no point")
			continue
		}
		found++
		point := filepath.Join(tree, "ai", "rules", "points", filepath.FromSlash(ref)+".md")
		if _, statErr := os.Stat(point); statErr != nil {
			t.Errorf("binding %s names no point on disk: %v", ref, statErr)
		}
	}
	if found < 4 {
		t.Errorf("the checker carries %d bindings, want one per flag rule", found)
	}
}
