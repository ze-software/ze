// Design: docs/functional-tests.md -- the selection contract of `le test bgp <suite>`.

package cli

import (
	"os"
	"path/filepath"
	"slices"
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
// MUTATION: (internal/test/runner/selection.go, `return strings.Contains(rec.Nick, pattern) || strings.Contains(rec.Name, pattern) || strings.Contains(rec.CIFile, pattern)`, `return true`): the plugin pattern line selects all three and goes red.
// MUTATION: (internal/test/runner/selection.go, `return strings.Contains(test.GetNick(), pattern) || strings.Contains(test.GetName(), pattern)`, `return true`): the parse pattern line selects all three and goes red.
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
