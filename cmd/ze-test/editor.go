package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	editortesting "codeberg.org/thomas-mangin/ze/internal/config/editor/testing"
	"codeberg.org/thomas-mangin/ze/internal/test/runner"
)

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
	testDir := filepath.Join(baseDir, "test/editor")
	if fs.NArg() > 0 {
		arg := fs.Arg(0)
		if filepath.IsAbs(arg) {
			testDir = arg
		} else {
			testDir = filepath.Join(baseDir, arg)
		}
	}

	// Find all .et files
	tests, err := findETFiles(testDir)
	if err != nil {
		return fmt.Errorf("finding tests: %w", err)
	}

	if len(tests) == 0 {
		return fmt.Errorf("no .et files found in %s", testDir)
	}

	// Filter by pattern
	if *pattern != "" {
		var filtered []string
		for _, t := range tests {
			if strings.Contains(t, *pattern) {
				filtered = append(filtered, t)
			}
		}
		tests = filtered
		if len(tests) == 0 {
			return fmt.Errorf("no tests matching pattern %q", *pattern)
		}
	}

	// List mode
	if *listOnly {
		fmt.Printf("Found %d tests:\n", len(tests)) //nolint:errcheck // terminal output
		for _, t := range tests {
			rel, _ := filepath.Rel(baseDir, t)
			if rel == "" {
				rel = t
			}
			fmt.Printf("  %s\n", rel) //nolint:errcheck // terminal output
		}
		return nil
	}

	// Run tests in parallel with progress display
	return runEditorTests(tests, baseDir, *verbose, *quiet)
}

// editorTestResult holds failure details for verbose output.
type editorTestResult struct {
	rel     string
	errMsg  string
	tempDir string
}

// runEditorTests executes tests in parallel with real-time progress display.
func runEditorTests(testPaths []string, baseDir string, verbose, quiet bool) error {
	colors := runner.NewColors()

	// Create parallel runner with generic type for direct test access
	pr := runner.NewParallelRunner[*editorTestResult](colors)
	pr.SetQuiet(quiet)
	pr.SetVerbose(verbose)

	// Add tests to runner
	for _, path := range testPaths {
		testPath := path // Capture for closure
		rel, _ := filepath.Rel(baseDir, testPath)
		if rel == "" {
			rel = testPath
		}

		// Create test result placeholder
		result := &editorTestResult{rel: rel}

		pr.AddTest(rel, result, func(_ context.Context, res *editorTestResult) (bool, error) {
			testResult := editortesting.RunETFile(testPath)

			// Update result for verbose output
			res.errMsg = testResult.Error
			res.tempDir = testResult.TempDir

			if !testResult.Passed {
				return false, fmt.Errorf("%s", testResult.Error)
			}
			return true, nil
		})
	}

	// Set failure callback for verbose output
	pr.SetOnFail(func(res *editorTestResult, _ error) {
		fmt.Fprintf(os.Stdout, "✗ %s\n", res.rel)    //nolint:errcheck // terminal output
		fmt.Fprintf(os.Stdout, "  %s\n", res.errMsg) //nolint:errcheck // terminal output
		if res.tempDir != "" {
			fmt.Fprintf(os.Stdout, "  temp dir: %s\n", res.tempDir) //nolint:errcheck // terminal output
		}
	})

	if !pr.Run(context.Background()) {
		return fmt.Errorf("tests failed")
	}

	return nil
}

func findETFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".et") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}
