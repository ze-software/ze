// Design: docs/architecture/testing/ci-format.md — test runner CLI

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	editortesting "github.com/ze-software/ze/internal/component/cli/testing"
	_ "github.com/ze-software/ze/internal/component/plugin/all"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/trace"
)

func cmdEditor(args []string) int {
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
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	quiet := fs.Bool("q", false, "minimal output")
	fs.BoolVar(quiet, "quiet", false, "minimal output")
	listOnly := fs.Bool("l", false, "list tests without running")
	fs.BoolVar(listOnly, "list", false, "list tests without running")

	fs.Usage = func() {
		_, _ = os.Stderr.WriteString(`Usage: ze-test editor [options] [test-ids...]

Run editor functional tests (.et files).

Options:
`)
		fs.PrintDefaults()
		_, _ = os.Stderr.WriteString(`
Examples:
  ze-test editor --all                    # Run all tests in test/editor/
  ze-test editor --dir test/editor/navigation --all
  ze-test editor -p commit                # Run tests matching "commit"
  ze-test editor --start 42               # Resume at id 42 and run through the end
  ze-test editor 1 2                      # Run specific tests by id
  ze-test editor -v                       # Verbose output
  ze-test editor -l                       # List available tests with N/TOTAL and one-based id
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

	testDir := filepath.Join(baseDir, "test", "editor")
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
		if d.IsDir() || !strings.HasSuffix(path, ".et") {
			return nil
		}

		rel, _ := filepath.Rel(baseDir, path)
		if rel == "" {
			rel = path
		}
		nick := runner.GenerateNick(rel)
		tests.Add(rel, nick, path)
		return nil
	})
}

func runEditorTests(tests *runner.EditorTests, baseDir string, verbose, quiet bool) error {
	colors := runner.NewColors()

	pr := runner.NewParallelRunner[*runner.EditorTest](colors)
	pr.SetLabel("editor")
	pr.SetQuiet(quiet)
	pr.SetVerbose(verbose)
	pr.SetBaseDir(baseDir)

	for _, test := range tests.Selected() {
		pr.AddTestWithNick(test.Name, test.Nick, test, func(_ context.Context, t *runner.EditorTest) (bool, error) {
			testResult := editortesting.RunETFile(t.Path)

			t.ErrMsg = testResult.Error
			t.TempDir = testResult.TempDir
			t.Steps = testResult.Steps

			if !testResult.Passed {
				t.SetError(fmt.Errorf("%s", testResult.Error))
				return false, t.GetError()
			}
			return true, nil
		})
	}

	pr.SetOnFail(func(t *runner.EditorTest, _ error) {
		fmt.Fprintf(os.Stdout, "✗ %s\n", t.Name)   //nolint:errcheck // terminal output
		fmt.Fprintf(os.Stdout, "  %s\n", t.ErrMsg) //nolint:errcheck // terminal output
		if t.TempDir != "" {
			fmt.Fprintf(os.Stdout, "  temp dir: %s\n", t.TempDir) //nolint:errcheck // terminal output
		}
		if len(t.Steps) > 0 {
			trace.PrintTrace(os.Stdout, t.Name, t.Steps, colors.Enabled())
		}
	})

	success := pr.Run(context.Background())

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
