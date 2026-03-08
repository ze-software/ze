// Design: docs/architecture/testing/ci-format.md — UI test runner CLI

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/test/runner"
)

// ErrTestsFailed is returned when one or more tests fail.
var ErrTestsFailed = errors.New("tests failed")

func uiCmd() int {
	if err := uiMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}

func uiMain() error {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	all := fs.Bool("a", false, "run all tests")
	fs.BoolVar(all, "all", false, "run all tests")
	listOnly := fs.Bool("l", false, "list available tests")
	fs.BoolVar(listOnly, "list", false, "list available tests")
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	quiet := fs.Bool("q", false, "minimal output")
	fs.BoolVar(quiet, "quiet", false, "minimal output")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test ui [options] [test-nicks...]

Run UI functional tests (.ci files in test/ui/).
Tests config completion, editor CLI, and other UI-facing features.

Options:
`) //nolint:errcheck // terminal output
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze-test ui -a              # Run all UI tests
  ze-test ui -l              # List available tests
  ze-test ui 0 1 2           # Run specific tests by nick
  ze-test ui -a -v           # Verbose output
`) //nolint:errcheck // terminal output
	}

	// Handle help before parsing
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

	// Discover tests
	runner.ResetNickCounter()
	tests := runner.NewEncodingTests(baseDir)
	testDir := filepath.Join(baseDir, "test", "ui")

	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	if tests.Count() == 0 {
		return fmt.Errorf("no .ci files found in %s", testDir)
	}

	// List mode
	if *listOnly {
		tests.List()
		return nil
	}

	// Select tests
	switch {
	case *all:
		tests.EnableAll()
	case fs.NArg() > 0:
		for _, arg := range fs.Args() {
			if !tests.EnableByNick(arg) {
				// Try by name
				for _, r := range tests.Registered() {
					if r.Name == arg {
						r.Activate()
						break
					}
				}
			}
		}
	default:
		fs.Usage()
		return nil
	}

	tests.Sort()

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create runner and execute
	r, err := runner.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer r.Cleanup()

	// Build ze binary (needed for foreground command execution)
	if err := r.Build(ctx); err != nil {
		return err
	}

	opts := &runner.RunOptions{
		Timeout: 15 * time.Second,
		Verbose: *verbose,
		Quiet:   *quiet,
	}

	if !r.Run(ctx, opts) {
		return ErrTestsFailed
	}

	return nil
}
