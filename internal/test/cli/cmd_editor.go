// Design: docs/architecture/testing/ci-format.md — test runner CLI

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	editortesting "github.com/ze-software/ze/internal/component/cli/testing"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/test/editorcase"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/sessionpath"
	"github.com/ze-software/ze/internal/test/trace"
)

// CmdEditor is the harness command `le test editor`, registered by
// internal/le/test/editor. It answers the process exit code.

// EditorSuiteDir is the directory under test/ the editor runner walks, and
// EditorTestSuffix the suffix of the files it discovers there, recursively. The
// runner reads both, so a reader outside it (internal/le/feature) asks here
// which files this runner runs.
const (
	EditorSuiteDir   = "editor"
	EditorTestSuffix = ".et"
)

func CmdEditor(args []string) int {
	if err := cmdEditorMain(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}

func cmdEditorMain(args []string) error {
	fs := flag.NewFlagSet("editor", flag.ExitOnError)
	all := fs.Bool("a", false, "run all tests")
	fs.BoolVar(all, "all", false, "run all tests")
	pattern := fs.String("p", "", "run only tests matching pattern")
	fs.StringVar(pattern, "pattern", "", "run only tests matching pattern")
	start := fs.String("start", "", "start at test id/name and run through the end")
	dir := fs.String("dir", "", "test directory (default: test/editor)")
	draft := fs.Bool("draft", false, "discover from test/draft/editor instead of test/editor (tests under development)")
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	quiet := fs.Bool("q", false, "minimal output")
	fs.BoolVar(quiet, "quiet", false, "minimal output")
	listOnly := fs.Bool("l", false, "list tests without running")
	fs.BoolVar(listOnly, "list", false, "list tests without running")

	fs.Usage = func() {
		_, _ = os.Stderr.WriteString(`Usage: le test editor [options] [test-ids...]

Run editor functional tests (.et files).

Options:
`)
		fs.PrintDefaults()
		_, _ = os.Stderr.WriteString(`
Examples:
  le test editor --all                    # Run all tests in test/editor/
  le test editor --dir test/editor/navigation --all
  le test editor -p commit                # Run tests matching "commit"
  le test editor --start 42               # Resume at id 42 and run through the end
  le test editor 1 2                      # Run specific tests by id
  le test editor -v                       # Verbose output
  le test editor -l                       # List available tests with N/TOTAL and one-based id
`)
	}

	if len(args) > 0 && isHelpArg(args[0]) {
		fs.Usage()
		return nil
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	baseDir, err := FindBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	testDir := runner.SuiteDir(baseDir, EditorSuiteDir, *draft)
	testArgs := fs.Args()
	if *dir != "" {
		if filepath.IsAbs(*dir) {
			testDir = *dir
		} else {
			testDir = filepath.Join(baseDir, *dir)
		}
	} else if len(testArgs) > 0 {
		candidate := testArgs[0]
		candidatePath := candidate
		if !filepath.IsAbs(candidatePath) {
			candidatePath = filepath.Join(baseDir, candidatePath)
		}
		if info, statErr := os.Stat(candidatePath); statErr == nil && info.IsDir() {
			testDir = candidatePath
			testArgs = testArgs[1:]
		}
	}

	tests := runner.NewEditorTests()
	if err := discoverEditorTests(tests, testDir, baseDir); err != nil {
		return err
	}

	if tests.Count() == 0 {
		return fmt.Errorf("no .et files found in %s", testDir)
	}

	if !*all && *pattern == "" && *start == "" && len(testArgs) == 0 {
		*all = true
	}
	selected, err := tests.Select(runner.Selection{
		All:     *all,
		Start:   *start,
		Pattern: *pattern,
		Args:    testArgs,
	})
	if err != nil {
		return err
	}
	if selected == 0 && !*listOnly {
		fs.Usage()
		return nil
	}

	if *listOnly {
		tests.List()
		return nil
	}

	return runEditorTests(tests, baseDir, *verbose, *quiet)
}

func discoverEditorTests(tests *runner.EditorTests, testDir, baseDir string) error {
	runner.ResetNickCounter()

	return filepath.WalkDir(testDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, EditorTestSuffix) {
			return nil
		}

		rel, _ := filepath.Rel(baseDir, path)
		if rel == "" {
			rel = path
		}
		nick := runner.GenerateNick(rel)
		test := tests.Add(rel, nick, path)
		content, readErr := os.ReadFile(path) //nolint:gosec // WalkDir supplies this selected test directory's .et fixture paths; reading their contents is the editor harness's purpose.
		if readErr != nil {
			setEditorParseError(test, readErr)
			return nil
		}
		test.Content = string(content)
		options, parseErr := editortesting.ContentOptions(test.Content, runner.ParallelTimeoutHeadroom)
		if parseErr != nil {
			setEditorParseError(test, parseErr)
			return nil
		}
		for _, option := range options {
			if _, err := runner.ParseResourceOption(&test.Admission, option.Declaration); err != nil {
				setEditorParseError(test, err)
				return nil
			}
		}
		return nil
	})
}

// setEditorParseError keeps malformed files selectable and reports them as
// failures, even when a previously parsed capability would skip execution.
func setEditorParseError(test *runner.EditorTest, err error) {
	err = fmt.Errorf("parse %s: %w", test.Path, err)
	test.Admission.ParseFailed = true
	test.Admission.Error = err
	test.ErrMsg = err.Error()
	test.SetError(err)
}

func runEditorTests(tests *runner.EditorTests, baseDir string, verbose, quiet bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the editor runner executable: %w", err)
	}
	scratch := sessionpath.EnsureScratchRoot(baseDir)
	colors := runner.NewColors()

	pr := runner.NewParallelRunner[*runner.EditorTest](colors)
	pr.SetLabel("editor")
	pr.SetQuiet(quiet)
	pr.SetVerbose(verbose)
	pr.SetBaseDir(baseDir)

	for _, test := range tests.Selected() {
		record := pr.AddTestWithNick(test.Name, test.Nick, test, func(ctx context.Context, t *runner.EditorTest) (bool, error) {
			// The runner keeps its default concurrency, which is above one,
			// so each authored timeout gets the parallel headroom.
			testResult := editorcase.Run(ctx, executable, t.Content, scratch, runner.ParallelTimeoutHeadroom)

			t.ErrMsg = testResult.Error
			t.TempDir = testResult.TempDir
			t.Steps = testResult.Steps

			if !testResult.Passed {
				t.SetError(fmt.Errorf("%s", testResult.Error))
				return false, t.GetError()
			}
			return true, nil
		})
		record.NeedsLinux = test.Admission.NeedsLinux
		record.RequiredCaps = test.Admission.RequiredCaps
		record.ExclusiveGroup = test.Admission.ExclusiveGroup
		record.SkipReason = test.Admission.SkipReason
		record.ParseFailed = test.Admission.ParseFailed
		record.Error = test.Admission.Error
	}

	pr.SetOnFail(func(t *runner.EditorTest, err error) {
		// Admission can fail before the editor callback fills its result.
		if err != nil {
			t.SetError(err)
			t.ErrMsg = err.Error()
		}
		fmt.Fprintf(os.Stdout, "✗ %s\n", t.Name)   //nolint:errcheck // terminal output
		fmt.Fprintf(os.Stdout, "  %s\n", t.ErrMsg) //nolint:errcheck // terminal output
		if t.TempDir != "" {
			fmt.Fprintf(os.Stdout, "  temp dir: %s\n", t.TempDir) //nolint:errcheck // terminal output
		}
		// Under -v the whole step trace shows what ran; otherwise only the
		// failed step, so a normal run stays concise.
		if verbose {
			trace.PrintTrace(os.Stdout, t.Name, t.Steps, colors.Enabled())
			return
		}
		trace.PrintFailedSteps(os.Stdout, t.Name, t.Steps, colors.Enabled())
	})

	success := pr.Run(ctx)

	if verbose {
		for _, t := range tests.Selected() {
			if t.GetError() == nil && len(t.Steps) > 0 {
				trace.PrintTrace(os.Stdout, t.Name, t.Steps, colors.Enabled())
			}
		}
	}

	if !success {
		return ErrTestsFailed
	}

	return nil
}
