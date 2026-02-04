package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

// runEditorTests executes tests in parallel with real-time progress display.
func runEditorTests(testPaths []string, baseDir string, verbose, quiet bool) error {
	// Create Tests container and populate with Records for display
	tests := runner.NewTests()
	colors := runner.NewColors()
	testMap := make(map[string]*runner.Record) // Map relative path -> Record

	for _, path := range testPaths {
		rel, _ := filepath.Rel(baseDir, path)
		if rel == "" {
			rel = path
		}
		rec := tests.Add(rel)
		rec.Active = true
		testMap[path] = rec
	}

	// Create display
	display := runner.NewDisplay(tests, colors)
	display.SetQuiet(quiet)
	display.SetTimeout(30 * time.Second)
	display.SetParallel(10, len(testPaths))
	display.Start()

	// Channel for collecting results
	type result struct {
		path    string
		rel     string
		passed  bool
		errMsg  string
		tempDir string
	}
	results := make(chan result, len(testPaths))
	done := make(chan struct{})

	// Run tests in parallel with semaphore
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Max 10 concurrent tests

	for _, testPath := range testPaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			rec := testMap[path]
			rel, _ := filepath.Rel(baseDir, path)
			if rel == "" {
				rel = path
			}

			rec.State = runner.StateRunning
			rec.StartTime = time.Now()

			testResult := editortesting.RunETFile(path)
			rec.Duration = time.Since(rec.StartTime)

			if testResult.Passed {
				rec.State = runner.StateSuccess
			} else {
				rec.State = runner.StateFail
				rec.Error = fmt.Errorf("%s", testResult.Error)
			}

			results <- result{
				path:    path,
				rel:     rel,
				passed:  testResult.Passed,
				errMsg:  testResult.Error,
				tempDir: testResult.TempDir,
			}
		}(testPath)
	}

	// Close results when all done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Periodic status update
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				display.Status()
			}
		}
	}()

	// Collect results and track failures for verbose output
	var failures []result
	for res := range results {
		if !res.passed {
			failures = append(failures, res)
		}
		display.Status()
	}

	close(done)
	display.Newline()
	display.Summary()

	// Print failure details if verbose
	if verbose && len(failures) > 0 {
		fmt.Fprintln(os.Stdout) //nolint:errcheck // terminal output
		for _, f := range failures {
			fmt.Fprintf(os.Stdout, "✗ %s\n", f.rel)    //nolint:errcheck // terminal output
			fmt.Fprintf(os.Stdout, "  %s\n", f.errMsg) //nolint:errcheck // terminal output
			if f.tempDir != "" {
				fmt.Fprintf(os.Stdout, "  temp dir: %s\n", f.tempDir) //nolint:errcheck // terminal output
			}
		}
	}

	_, failed, _, _ := tests.Summary()
	if failed > 0 {
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
