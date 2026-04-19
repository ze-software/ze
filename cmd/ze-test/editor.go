// Design: docs/architecture/testing/ci-format.md — test runner CLI

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	editortesting "codeberg.org/thomas-mangin/ze/internal/component/cli/testing"
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"
	"codeberg.org/thomas-mangin/ze/internal/test/runner"
)

var _ = register("editor", "Run editor functional tests (.et files)", editorCmd)

func editorCmd() int {
	if err := editorMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}

func editorMain() error {
	fs := flag.NewFlagSet("editor", flag.ExitOnError)
	pattern := fs.String("p", "", "run only tests matching pattern")
	fs.StringVar(pattern, "pattern", "", "run only tests matching pattern")
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	quiet := fs.Bool("q", false, "minimal output")
	fs.BoolVar(quiet, "quiet", false, "minimal output")
	listOnly := fs.Bool("l", false, "list tests without running")
	fs.BoolVar(listOnly, "list", false, "list tests without running")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test editor [options] [test-dir]

Run editor functional tests (.et files).

Options:
`) //nolint:errcheck // terminal output
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze-test editor                           # Run all tests in test/editor/
  ze-test editor test/editor/navigation/   # Run navigation tests only
  ze-test editor -p commit                 # Run tests matching "commit"
  ze-test editor -v                        # Verbose output
  ze-test editor -l                        # List tests without running
`) //nolint:errcheck // terminal output
	}

	// Handle help before parsing (os.Args shifted by main: ["editor", ...])
	if len(os.Args) > 1 && isHelpArg(os.Args[1]) {
		fs.Usage()
		return nil
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Find base directory
	baseDir, err := findBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	// Determine test directory
	testDir := filepath.Join(baseDir, "test", "editor")
	if fs.NArg() > 0 {
		arg := fs.Arg(0)
		if filepath.IsAbs(arg) {
			testDir = arg
		} else {
			testDir = filepath.Join(baseDir, arg)
		}
	}

	// Discover and create editor tests
	tests := runner.NewEditorTests()
	if err := discoverEditorTests(tests, testDir, baseDir); err != nil {
		return err
	}

	if tests.Count() == 0 {
		return fmt.Errorf("no .et files found in %s", testDir)
	}

	// Filter by pattern
	if *pattern != "" {
		matched := false
		for _, t := range tests.Registered() {
			if strings.Contains(t.Path, *pattern) || strings.Contains(t.Name, *pattern) {
				t.SetActive(true)
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("no tests matching pattern %q", *pattern)
		}
	} else {
		tests.EnableAll()
	}

	// List mode
	if *listOnly {
		fmt.Fprintf(os.Stdout, "Found %d tests:\n", tests.Count()) //nolint:errcheck // terminal output
		for _, t := range tests.Registered() {
			fmt.Fprintf(os.Stdout, "  %s  %s\n", t.Nick, t.Name) //nolint:errcheck // terminal output
		}
		return nil
	}

	// Run tests in parallel with progress display
	return runEditorTests(tests, baseDir, *verbose, *quiet)
}

// discoverEditorTests finds all .et files and adds them to the test set.
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

// runEditorTests executes tests in parallel with real-time progress display.
func runEditorTests(tests *runner.EditorTests, baseDir string, verbose, quiet bool) error {
	colors := runner.NewColors()

	// Create parallel runner with generic type for direct test access
	pr := runner.NewParallelRunner[*runner.EditorTest](colors)
	pr.SetLabel("editor")
	pr.SetQuiet(quiet)
	pr.SetVerbose(verbose)
	pr.SetBaseDir(baseDir)

	// Add selected tests to runner
	for _, test := range tests.Selected() {
		pr.AddTest(test.Name, test, func(_ context.Context, t *runner.EditorTest) (bool, error) {
			testResult := editortesting.RunETFile(t.Path)

			// Update test with results
			t.ErrMsg = testResult.Error
			t.TempDir = testResult.TempDir

			if !testResult.Passed {
				t.SetError(fmt.Errorf("%s", testResult.Error))
				return false, t.GetError()
			}
			return true, nil
		})
	}

	// Set failure callback for verbose output
	pr.SetOnFail(func(t *runner.EditorTest, _ error) {
		fmt.Fprintf(os.Stdout, "✗ %s\n", t.Name)   //nolint:errcheck // terminal output
		fmt.Fprintf(os.Stdout, "  %s\n", t.ErrMsg) //nolint:errcheck // terminal output
		if t.TempDir != "" {
			fmt.Fprintf(os.Stdout, "  temp dir: %s\n", t.TempDir) //nolint:errcheck // terminal output
		}
	})

	if !pr.Run(context.Background()) {
		return fmt.Errorf("tests failed")
	}

	return nil
}
