package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	editortesting "codeberg.org/thomas-mangin/ze/internal/config/editor/testing"
)

func editorCmd() int {
	if err := editorMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
	listOnly := fs.Bool("l", false, "list tests without running")
	fs.BoolVar(listOnly, "list", false, "list tests without running")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test editor [options] [test-dir]

Run editor functional tests (.et files).

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze-test editor                           # Run all tests in test/editor/
  ze-test editor test/editor/navigation/   # Run navigation tests only
  ze-test editor -p commit                 # Run tests matching "commit"
  ze-test editor -v                        # Verbose output
  ze-test editor -l                        # List tests without running
`)
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
		fmt.Printf("Found %d tests:\n", len(tests))
		for _, t := range tests {
			rel, _ := filepath.Rel(baseDir, t)
			if rel == "" {
				rel = t
			}
			fmt.Printf("  %s\n", rel)
		}
		return nil
	}

	// Run tests
	passed := 0
	failed := 0

	for _, testPath := range tests {
		result := editortesting.RunETFile(testPath)
		rel, _ := filepath.Rel(baseDir, testPath)
		if rel == "" {
			rel = testPath
		}

		if result.Passed {
			passed++
			if *verbose {
				fmt.Printf("✓ %s (%v)\n", rel, result.Duration)
			}
		} else {
			failed++
			fmt.Printf("✗ %s\n", rel)
			if result.Error != "" {
				fmt.Printf("  %s\n", result.Error)
			}
		}
	}

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)

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
