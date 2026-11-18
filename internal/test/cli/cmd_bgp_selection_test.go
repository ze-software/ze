// Design: docs/functional-tests.md -- the selection contract of `le test bgp <suite>`.

package cli

import (
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
// to see the names, and each name as its own argument.
// PREVENTS: a refusal that leaves the reader to guess the matching rule.
//
// MUTATION: (internal/test/runner/selection.go errNoPatternMatch, `return opererr.New(problem, patternRule, patternForms)`, `return fmt.Errorf("%s", problem)`): the refusal keeps its first line and loses the rule and the forms, and both suites go red.
func TestBgpUnmatchedPatternNamesTheFormsThatWork(t *testing.T) {
	base := checkoutFixture(t)
	pattern := "lg-covering-lookup|rib-pipe-filter"
	for _, suite := range []string{"plugin", "parse"} {
		_, err := selectedNames(t, base, []string{suite, "--pattern", pattern})
		if err == nil {
			t.Fatalf("%s: an alternation selected tests, want a refusal", suite)
		}
		for _, needle := range []string{`no tests matching pattern "` + pattern + `"`, "not a regex or a glob", "--list", "own argument"} {
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
