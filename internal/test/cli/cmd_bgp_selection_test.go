// Design: docs/functional-tests.md -- the selection contract of `le test bgp <suite>`.

package cli

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
)

// selectionFixture writes three .ci files into test/<suite> under a fresh
// base directory and answers that directory. The names share the `zq-`
// prefix so no path component of the temporary directory can match a pattern.
func selectionFixture(t *testing.T, suites ...string) string {
	t.Helper()
	base := t.TempDir()
	for _, suite := range suites {
		dir := filepath.Join(base, "test", suite)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"zq-alpha-one", "zq-beta-one", "zq-alpha-two"} {
			if err := os.WriteFile(filepath.Join(dir, name+".ci"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return base
}

// TestBgpSelectionPicksExactlyTheDocumentedTests drives the documented
// selector forms of `le test bgp <suite>` from argv to the selected set, for
// both suite drivers: the record suites (encode, plugin, reload) and the
// generic ones (parse, decode).
//
// VALIDATES: zeTestParseRunCLI reads the line, the flags' own selection()
// mapping hands it to Select over a discovered suite directory, and the
// selected tests are exactly the documented ones: `--pattern TEXT` every
// test whose name contains TEXT, a positional name that test alone. A pattern
// no test matches is refused with an error naming it, never answered with an
// empty run.
// PREVENTS: a selector the parser accepts and the run ignores, which runs the
// whole suite or nothing while the command line reads as a selection.
//
// MUTATION: (cmd_bgp.go, `Pattern: cli.pattern,`, `Pattern: "",`): the pattern lines select nothing and go red.
// MUTATION: (internal/test/runner/selection.go patternMatches, `return strings.Contains(nick, pattern) || strings.Contains(name, pattern)`, `return true`): the plugin and the parse pattern lines select all three and go red.
func TestBgpSelectionPicksExactlyTheDocumentedTests(t *testing.T) {
	base := selectionFixture(t, "plugin", "parse")
	for _, tc := range []struct {
		line []string
		want []string
	}{
		{line: []string{"plugin", "--pattern", "zq-alpha"}, want: []string{"zq-alpha-one", "zq-alpha-two"}},
		{line: []string{"plugin", "zq-beta-one"}, want: []string{"zq-beta-one"}},
		{line: []string{"parse", "--pattern", "zq-alpha"}, want: []string{"zq-alpha-one", "zq-alpha-two"}},
		{line: []string{"parse", "zq-beta-one"}, want: []string{"zq-beta-one"}},
	} {
		t.Run(strings.Join(tc.line, " "), func(t *testing.T) {
			got, err := selectedNames(t, base, tc.line)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("selected %v, want exactly %v", got, tc.want)
			}
		})
	}

	for _, suite := range []string{"plugin", "parse"} {
		_, err := selectedNames(t, base, []string{suite, "--pattern", "zq-gamma"})
		if err == nil || !strings.Contains(err.Error(), `no tests matching pattern "zq-gamma"`) {
			t.Errorf("%s: unmatched pattern answered %v, want the refusal naming it", suite, err)
		}
	}
}

// selectedNames parses line as `le test bgp` does, discovers its suite under
// base with that suite's driver, selects, and answers the selected names in
// suite order.
func selectedNames(t *testing.T, base string, line []string) ([]string, error) {
	t.Helper()
	flags, err := zeTestParseRunCLI(line)
	if err != nil {
		return nil, err
	}
	dir := runner.SuiteDir(base, flags.command, false)
	runner.ResetNickCounter()
	if flags.command == cmdParse {
		tests := runner.NewParsingTests(base)
		if err := tests.Discover(dir); err != nil {
			t.Fatal(err)
		}
		if _, err := tests.Select(flags.selection()); err != nil {
			return nil, err
		}
		var names []string
		for _, test := range tests.Selected() {
			names = append(names, test.GetName())
		}
		return names, nil
	}
	tests := runner.NewEncodingTests(base)
	if err := tests.Discover(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := tests.Select(flags.selection()); err != nil {
		return nil, err
	}
	var names []string
	for _, rec := range tests.Selected() {
		names = append(names, rec.Name)
	}
	return names, nil
}

// TestBgpUnmatchedPatternFailsTheEntry types an unmatched `--pattern` at
// CmdBgp, the `le test bgp` entry, over the checkout's own test/parse.
//
// VALIDATES: the command exits non-zero, before any build, rather than taking
// the empty-selection path that prints usage and exits 0.
// PREVENTS: a mistyped pattern reading as a green run of nothing.
//
// MUTATION: (cmd_bgp.go, `Pattern: cli.pattern,`, `Pattern: "",`): the selection is empty, the usage path exits 0, and the test goes red.
func TestBgpUnmatchedPatternFailsTheEntry(t *testing.T) {
	if code := CmdBgp([]string{"parse", "--pattern", "zq-no-test-is-named-this"}); code == 0 {
		t.Fatal("an unmatched --pattern exited 0: a selection that matched nothing read as success")
	}
}

// checkoutFixture reproduces the checkout the defect was met in: a base
// directory whose own name holds "lg-", as `.claude/worktrees/lg-search-matches-pages`
// did, carrying a plugin and a parse suite of two looking-glass tests and one
// other. The names of the tests calling it hold none of the text they select
// on, because t.TempDir puts the test name in the path.
func checkoutFixture(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "lg-search-matches-pages")
	for _, suite := range []string{"plugin", "parse"} {
		dir := filepath.Join(base, "test", suite)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"bmp-lg-ingest", "lg-covering-lookup", "rib-pipe-filter"} {
			if err := os.WriteFile(filepath.Join(dir, name+".ci"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return base
}

// TestBgpPatternSelectsOnlyTheTestsItNames types the line that ran the whole
// plugin suite, `--pattern lg-`, in a checkout whose own path holds "lg-".
//
// VALIDATES: the pattern selects exactly the tests whose name contains it,
// for both suite drivers, and the test that only its path matched is not
// selected.
// PREVENTS: a substring of the checkout location selecting every test of the
// suite, an hour-long full run typed as a selection of a few.
//
// MUTATION: (internal/test/runner/selection.go, `if pattern == "" || patternMatches(rec.Nick, rec.Name, pattern) {`, `if pattern == "" || patternMatches(rec.Nick, rec.Name, pattern) || strings.Contains(rec.CIFile, pattern) {`): the plugin row selects all three and goes red.
func TestBgpPatternSelectsOnlyTheTestsItNames(t *testing.T) {
	base := checkoutFixture(t)
	want := []string{"bmp-lg-ingest", "lg-covering-lookup"}
	for _, suite := range []string{"plugin", "parse"} {
		got, err := selectedNames(t, base, []string{suite, "--pattern", "lg-"})
		if err != nil {
			t.Fatalf("%s: %v", suite, err)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s --pattern lg-: selected %v, want exactly %v", suite, got, want)
		}
	}
}

// TestBgpPatternDoesNotMatchTheFilePath types patterns that occur only in the
// path of every .ci file of the suite: a piece of the checkout location, the
// suite directory, and the file extension.
//
// VALIDATES: each is refused as matching no test, because a pattern is
// matched against the test's id and name, never its path.
// PREVENTS: a pattern that every test's path shares widening to the whole
// suite while the command line reads as a selection.
//
// MUTATION: (internal/test/runner/selection.go, `if pattern == "" || patternMatches(rec.Nick, rec.Name, pattern) {`, `if pattern == "" || patternMatches(rec.Nick, rec.Name, pattern) || strings.Contains(rec.CIFile, pattern) {`): every row selects all three and goes red.
func TestBgpPatternDoesNotMatchTheFilePath(t *testing.T) {
	base := checkoutFixture(t)
	for _, pattern := range []string{"search-matches", "test/plugin", ".ci"} {
		got, err := selectedNames(t, base, []string{"plugin", "--pattern", pattern})
		if err == nil || !strings.Contains(err.Error(), "no tests matching pattern "+strconv.Quote(pattern)) {
			t.Errorf("plugin --pattern %s: selected %v with error %v, want the refusal naming the pattern", pattern, got, err)
		}
	}
}

// TestBgpUnmatchedPatternNamesTheFormsThatWork types a regex alternation, the
// line that answered `no tests matching pattern` and nothing more.
//
// VALIDATES: the refusal names the pattern, says that it is matched as typed
// and is not a regex or a glob, and names the two forms that work: `--list`
// with no `--pattern` to see the names, and each name as its own argument.
// `--list` lists what the selectors on its line select, so advice to add
// `--list` to the refused line would be refused again.
// PREVENTS: a refusal that leaves the reader to guess the matching rule, or
// whose advice, typed as written, fails the same way.
//
// MUTATION: (internal/test/runner/selection.go errNoPatternMatch, `return opererr.New(problem, patternRule, patternForms)`, `return fmt.Errorf("%s", problem)`): the refusal keeps its first line and loses the rule and the forms, and both suites go red.
// MUTATION: (internal/test/runner/selection.go patternForms, `"run --list with no --pattern to see every test name,"`, `"add --list to see every test name,"`): the advice keeps the pattern on the line and both suites go red.
func TestBgpUnmatchedPatternNamesTheFormsThatWork(t *testing.T) {
	base := checkoutFixture(t)
	pattern := "lg-covering-lookup|rib-pipe-filter"
	for _, suite := range []string{"plugin", "parse"} {
		_, err := selectedNames(t, base, []string{suite, "--pattern", pattern})
		if err == nil {
			t.Fatalf("%s: an alternation selected tests, want a refusal", suite)
		}
		for _, needle := range []string{`no tests matching pattern "` + pattern + `"`, "not a regex or a glob", "--list with no --pattern", "own argument"} {
			if !strings.Contains(err.Error(), needle) {
				t.Errorf("%s: refusal %q lacks %q", suite, err.Error(), needle)
			}
		}
	}
}

// TestBgpPositionalNamesSelectExactlyThoseTests types the form the refusal
// names, each test name as its own argument, in the same checkout.
//
// VALIDATES: one name selects that test alone, and two names select exactly
// those two, for both suite drivers.
// PREVENTS: the form the refusal recommends selecting more or less than it
// names.
//
// MUTATION: (internal/test/runner/selection.go, indexRecordSelector, `if rec.Nick == selector || rec.Name == selector || rec.CIFile == selector {`, `if rec.Nick == selector || rec.CIFile == selector {`): the plugin rows answer `test "lg-covering-lookup" not found` and go red.
func TestBgpPositionalNamesSelectExactlyThoseTests(t *testing.T) {
	base := checkoutFixture(t)
	for _, suite := range []string{"plugin", "parse"} {
		for _, names := range [][]string{
			{"lg-covering-lookup"},
			{"lg-covering-lookup", "rib-pipe-filter"},
		} {
			got, err := selectedNames(t, base, append([]string{suite}, names...))
			if err != nil {
				t.Fatalf("%s %v: %v", suite, names, err)
			}
			if !slices.Equal(got, names) {
				t.Errorf("%s %v: selected %v, want exactly those", suite, names, got)
			}
		}
	}
}

// listSuiteCI is the suite directory the shared `le test <suite>` runner
// lists in the --list tests below. It names no real suite, so the fixture
// holds every test that runner discovers.
const listSuiteCI = "zq-list"

// listEntry is one runner entry that takes --list: a label for the failure
// message, and the call that types argv at the entry and answers its exit
// code.
type listEntry struct {
	name string
	run  func(args []string) int
}

// listEntries answers the entries the --list tests type at: both `le test
// bgp` drivers (the record suites through plugin, the generic ones through
// parse), the shared `le test <suite>` runner, and `le test vpp`. Each reads
// its suite from test/<suite> under the base directory FindBaseDir answers.
func listEntries() []listEntry {
	ci := CIRunnerConfig{Name: listSuiteCI, TestSubdir: listSuiteCI, Description: "list", DefaultParallel: 1}
	return []listEntry{
		{name: "bgp plugin", run: func(args []string) int { return CmdBgp(append([]string{"plugin"}, args...)) }},
		{name: "bgp parse", run: func(args []string) int { return CmdBgp(append([]string{"parse"}, args...)) }},
		{name: listSuiteCI, run: func(args []string) int { return RunCISubcommand(ci, args) }},
		{name: "vpp", run: CmdVpp},
	}
}

// enterListFixture writes the selection fixture for every suite of
// listEntries, with a go.mod at its root, and changes into it, so each
// entry's FindBaseDir answers the fixture rather than the checkout. Every
// suite then holds, in suite order, zq-alpha-one (id 1), zq-alpha-two (id 2)
// and zq-beta-one (id 3).
func enterListFixture(t *testing.T) {
	t.Helper()
	base := selectionFixture(t, "plugin", "parse", "vpp", listSuiteCI)
	if err := os.WriteFile(filepath.Join(base, "go.mod"), []byte("module zqlist\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(base)
}

// listRun is what one entry answered to a line: its exit code, its two
// streams, and the `N/TOTAL ID NAME` lines of its stdout in order, each cut
// to those three fields, joined by one space. The parse suite's list adds a
// note on what the test expects after the name, which is not compared here.
type listRun struct {
	code   int
	stdout string
	stderr string
	lines  []string
}

// runList types args at entry and answers what it printed. The list lines
// are the stdout lines that open with the two-space indent the list writes;
// the header and the blank lines around it are not.
func runList(t *testing.T, entry listEntry, args []string) listRun {
	t.Helper()
	var run listRun
	run.stderr = captureStderr(t, func() {
		run.stdout = captureStdout(t, func() { run.code = entry.run(args) })
	})
	for line := range strings.Lines(run.stdout) {
		if !strings.HasPrefix(line, "  ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			t.Errorf("%s %v: list line %q lacks N/TOTAL, ID and NAME", entry.name, args, line)
			continue
		}
		run.lines = append(run.lines, strings.Join(fields[:3], " "))
	}
	return run
}

// captureStdout runs fn with os.Stdout replaced by a pipe and returns what fn
// wrote there: an entry prints its list on stdout and answers only an exit
// code.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w
	type read struct {
		out string
		err error
	}
	done := make(chan read, 1)
	go func() {
		out, err := io.ReadAll(r)
		done <- read{out: string(out), err: err}
	}()
	fn()
	os.Stdout = saved
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	got := <-done
	if got.err != nil {
		t.Fatalf("read captured stdout: %v", got.err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close pipe read end: %v", err)
	}
	return got.out
}

// checkListed asserts that run succeeded and listed exactly want under a
// header that opens with lead: `Available` over every test of the suite,
// `Selected` over what the selectors select. The noun after it is the
// suite's own (`tests`, `parsing tests`).
func checkListed(t *testing.T, label string, run listRun, lead string, want []string) {
	t.Helper()
	if run.code != 0 {
		t.Errorf("%s: exit %d, stderr %q, want a list", label, run.code, run.stderr)
		return
	}
	if !strings.Contains(run.stdout, "\n"+lead+" ") {
		t.Errorf("%s: stdout %q lacks a header opening with %q", label, run.stdout, lead)
	}
	if !slices.Equal(run.lines, want) {
		t.Errorf("%s: listed %q, want exactly %q", label, run.lines, want)
	}
}

// TestListAloneListsEveryTest types `--list` with no selector at each runner
// entry: the line the --pattern refusal tells the operator to type to see
// every test name.
//
// VALIDATES: every test of the suite is listed, in suite order, as
// `N/TOTAL ID NAME` under a header opening with `Available`.
// PREVENTS: listing what the selectors select turning a bare --list, whose
// line selects nothing, into an empty list.
//
// MUTATION: (internal/test/runner/selection.go Tests.Listed, `if !sel.requestsRun() {`, `if false {`): Select activates nothing for a bare line, the record suites list nothing and go red.
// MUTATION: (internal/test/runner/selection.go TestSet.Listed, `if !sel.requestsRun() {`, `if false {`): the parse row lists nothing and goes red.
func TestListAloneListsEveryTest(t *testing.T) {
	enterListFixture(t)
	want := []string{"1/3 1 zq-alpha-one", "2/3 2 zq-alpha-two", "3/3 3 zq-beta-one"}
	for _, entry := range listEntries() {
		checkListed(t, entry.name+" --list", runList(t, entry, []string{"--list"}), "Available", want)
	}
}

// TestListWithPatternListsOnlyTheMatches types `--list --pattern one`, the
// question "which tests would this pattern run", at each runner entry.
//
// VALIDATES: exactly the tests --pattern selects are listed, in the suite
// order a run takes them, under a header opening with `Selected`. N/TOTAL
// counts the listed tests, and ID stays the suite id the operator types to
// run one.
// PREVENTS: --list ignoring --pattern and printing the whole suite, which
// answers "which tests would this pattern run" with "all of them".
//
// MUTATION: (cmd_bgp.go zeTestRunEncodingOrAPI, `return tests.List(cli.selection())`, `return tests.List(runner.Selection{})`): the plugin row lists all three and goes red.
// MUTATION: (cmd_bgp.go zeTestRunSimpleTests, `return tests.List(cli.selection())`, `return tests.List(runner.Selection{})`): the parse row lists all three and goes red.
// MUTATION: (ci_runner.go runCISubcommandInner, `return tests.List(sel)`, `return tests.List(runner.Selection{})`): the zq-list row lists all three and goes red.
// MUTATION: (cmd_vpp.go zeTestVppMain, `return tests.List(cli.selection())`, `return tests.List(runner.Selection{})`): the vpp row lists all three and goes red.
func TestListWithPatternListsOnlyTheMatches(t *testing.T) {
	enterListFixture(t)
	want := []string{"1/2 1 zq-alpha-one", "2/2 3 zq-beta-one"}
	for _, entry := range listEntries() {
		run := runList(t, entry, []string{"--list", "--pattern", "one"})
		checkListed(t, entry.name+" --list --pattern one", run, "Selected", want)
	}
}

// TestListWithAnUnmatchedSelectorIsRefused types `--list` with a pattern no
// test matches, and with a test name no test carries, at each runner entry.
//
// VALIDATES: the line exits non-zero, lists no test, and names the selector
// on stderr: the --pattern refusal of a run, and the not-found refusal of a
// positional name.
// PREVENTS: an unmatched selector answered with an empty list, which reads as
// a suite with no such test, or with the whole suite.
//
// MUTATION: (internal/test/runner/selection.go Tests.Listed, `if _, err := ts.Select(sel); err != nil {`, `if _, err := ts.Select(sel); err != nil && false {`): the record suites list nothing and exit 0, and go red.
func TestListWithAnUnmatchedSelectorIsRefused(t *testing.T) {
	enterListFixture(t)
	for _, tc := range []struct {
		args   []string
		reason string
	}{
		{args: []string{"--list", "--pattern", "zq-gamma"}, reason: `no tests matching pattern "zq-gamma"`},
		{args: []string{"--list", "zq-gamma"}, reason: `test "zq-gamma" not found`},
	} {
		for _, entry := range listEntries() {
			label := entry.name + " " + strings.Join(tc.args, " ")
			run := runList(t, entry, tc.args)
			if run.code == 0 {
				t.Errorf("%s: exit 0 with stdout %q, want the refusal", label, run.stdout)
			}
			if len(run.lines) != 0 {
				t.Errorf("%s: listed %q, want no test listed", label, run.lines)
			}
			if !strings.Contains(run.stderr, tc.reason) {
				t.Errorf("%s: stderr %q lacks %q", label, run.stderr, tc.reason)
			}
		}
	}
}

// TestListWithTestNamesListsThoseTests types `--list` with two test names as
// arguments, in the reverse of their suite order, at each runner entry.
//
// VALIDATES: exactly the named tests are listed, in the suite order a run
// takes them, which is not the order they were typed in.
// PREVENTS: --list ignoring the positional names and printing the whole
// suite.
//
// MUTATION: (cmd_vpp.go zeTestVppMain, `return tests.List(cli.selection())`, `return tests.List(runner.Selection{})`): the vpp row lists all three and goes red.
func TestListWithTestNamesListsThoseTests(t *testing.T) {
	enterListFixture(t)
	want := []string{"1/2 1 zq-alpha-one", "2/2 3 zq-beta-one"}
	for _, entry := range listEntries() {
		run := runList(t, entry, []string{"--list", "zq-beta-one", "zq-alpha-one"})
		checkListed(t, entry.name+" --list zq-beta-one zq-alpha-one", run, "Selected", want)
	}
}

// TestListWithStartListsFromThatTest types `--list --start zq-alpha-two` at
// each runner entry.
//
// VALIDATES: the named test and every later one in suite order are listed,
// as the same --start runs them.
// PREVENTS: --list ignoring --start and printing the whole suite.
//
// MUTATION: (ci_runner.go runCISubcommandInner, `return tests.List(sel)`, `return tests.List(runner.Selection{})`): the zq-list row lists all three and goes red.
func TestListWithStartListsFromThatTest(t *testing.T) {
	enterListFixture(t)
	want := []string{"1/2 2 zq-alpha-two", "2/2 3 zq-beta-one"}
	for _, entry := range listEntries() {
		run := runList(t, entry, []string{"--list", "--start", "zq-alpha-two"})
		checkListed(t, entry.name+" --list --start zq-alpha-two", run, "Selected", want)
	}
}

// TestShortListWithPatternListsOnlyTheMatches types `--short-list --pattern
// one` at both `le test bgp` drivers, the two entries that take --short-list
// here.
//
// VALIDATES: the ids printed are exactly those of the tests --pattern
// selects, in suite order.
// PREVENTS: --short-list ignoring the selectors on its line, the same defect
// as --list in its other form.
//
// MUTATION: (cmd_bgp.go zeTestRunEncodingOrAPI, `records, err := tests.Listed(cli.selection())`, `records, err := tests.Listed(runner.Selection{})`): the plugin row prints all three ids and goes red.
func TestShortListWithPatternListsOnlyTheMatches(t *testing.T) {
	enterListFixture(t)
	for _, entry := range listEntries()[:2] {
		run := runList(t, entry, []string{"--short-list", "--pattern", "one"})
		if run.code != 0 {
			t.Errorf("%s --short-list --pattern one: exit %d, stderr %q", entry.name, run.code, run.stderr)
			continue
		}
		if got := strings.Join(strings.Fields(run.stdout), " "); got != "1 3" {
			t.Errorf("%s --short-list --pattern one: printed %q, want exactly %q", entry.name, got, "1 3")
		}
	}
}
