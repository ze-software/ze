// Design: docs/architecture/testing/ci-format.md — managed config test runner CLI

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/test/runner"
)

func managedCmd() int {
	if err := managedMain(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err) //nolint:errcheck // terminal output
		return 1
	}
	return 0
}

func managedMain() error {
	fs := flag.NewFlagSet("managed", flag.ExitOnError)
	all := fs.Bool("a", false, "run all tests")
	fs.BoolVar(all, "all", false, "run all tests")
	listOnly := fs.Bool("l", false, "list available tests")
	fs.BoolVar(listOnly, "list", false, "list available tests")
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	quiet := fs.Bool("q", false, "minimal output")
	fs.BoolVar(quiet, "quiet", false, "minimal output")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test managed [options] [test-nicks...]

Run managed config functional tests (.ci files in test/managed/).
Tests fleet management: hub config, per-client auth, managed boot, config change.

Options:
`) //nolint:errcheck // terminal output
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  ze-test managed -a              # Run all managed tests
  ze-test managed -l              # List available tests
  ze-test managed 0 1 2           # Run specific tests by nick
  ze-test managed -a -v           # Verbose output
`) //nolint:errcheck // terminal output
	}

	if len(os.Args) > 1 && isHelpArg(os.Args[1]) {
		fs.Usage()
		return nil
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	baseDir, err := findBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	runner.ResetNickCounter()
	tests := runner.NewEncodingTests(baseDir)
	testDir := filepath.Join(baseDir, "test", "managed")

	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	if tests.Count() == 0 {
		return fmt.Errorf("no .ci files found in %s", testDir)
	}

	if *listOnly {
		tests.List()
		return nil
	}

	switch {
	case *all:
		tests.EnableAll()
	case fs.NArg() > 0:
		for _, arg := range fs.Args() {
			if !tests.EnableByNick(arg) {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	r, err := runner.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer r.Cleanup()

	if err := r.Build(ctx); err != nil {
		return err
	}

	r.Display().SetLabel("managed")
	r.Report().SetLabel("managed")

	opts := &runner.RunOptions{
		Timeout: 15 * time.Second,
		Verbose: *verbose,
		Quiet:   *quiet,
	}

	success := r.Run(ctx, opts)
	r.Display().Summary()
	r.Display().TimingDetail("managed", r.Timings())
	r.Display().DebugHints()

	if !success {
		return ErrTestsFailed
	}

	return nil
}
