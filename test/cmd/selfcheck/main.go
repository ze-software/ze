// Package main provides the selfcheck functional test runner with AI-friendly diagnostics.
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

	"github.com/exa-networks/zebgp/test/selfcheck"
)

// errTestsFailed is returned when tests fail (not an error, but indicates exit code 1).
var errTestsFailed = errors.New("tests failed")

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, errTestsFailed) {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func run() error {
	// Parse command line
	cli := parseCLI()
	if cli == nil {
		return nil // Help was printed
	}

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Find base directory
	baseDir, err := findBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	// Initialize
	colors := selfcheck.NewColors()
	selfcheck.ResetNickCounter()

	// Check ulimit
	limitCheck, err := selfcheck.CheckUlimit(cli.parallel)
	if err != nil {
		return fmt.Errorf("ulimit check: %w", err)
	}
	if limitCheck.Raised && !cli.quiet {
		fmt.Printf("%s raised to %d\n", colors.Yellow("ulimit:"), limitCheck.RaisedTo)
	}

	// Discover tests
	tests := selfcheck.NewEncodingTests(baseDir)
	testDir := filepath.Join(baseDir, "test/data/encode")
	if cli.command == "api" {
		testDir = filepath.Join(baseDir, "test/data/api")
	}

	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	// Handle list mode
	if cli.list {
		tests.List()
		return nil
	}

	if cli.shortList {
		for _, r := range tests.Registered() {
			fmt.Printf("%s ", r.Nick)
		}
		fmt.Println()
		return nil
	}

	// Allocate ports
	pr, shifted, err := selfcheck.AllocatePorts(cli.port, tests.Count())
	if err != nil {
		return fmt.Errorf("allocate ports: %w", err)
	}

	// Update test ports based on allocation
	basePort := pr.Start
	for _, r := range tests.Registered() {
		r.Port = basePort
		basePort++
	}

	if !cli.quiet {
		if shifted {
			fmt.Printf("%s %s (base %d in use, shifted)\n",
				colors.Yellow("ports:"), pr.String(), cli.port)
		} else {
			fmt.Printf("%s %s (%d tests)\n",
				colors.Cyan("ports:"), pr.String(), tests.Count())
		}
	}

	// Select tests
	switch {
	case cli.all:
		tests.EnableAll()
	case len(cli.testArgs) > 0:
		for _, arg := range cli.testArgs {
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
		printUsage()
		return nil
	}

	tests.Sort()

	// Create runner
	runner, err := selfcheck.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer runner.Cleanup()

	// Build
	if err := runner.Build(ctx); err != nil {
		return err
	}

	// Run options
	opts := &selfcheck.RunOptions{
		Timeout:  cli.timeout,
		Parallel: cli.parallel,
		Verbose:  cli.verbose,
		Quiet:    cli.quiet,
		SaveDir:  cli.saveDir,
	}

	// Print summary
	display := selfcheck.NewDisplay(tests.Tests, colors)
	display.SetQuiet(cli.quiet)

	var success bool
	if cli.count > 1 {
		// Stress test mode
		stats, ok := runner.RunWithCount(ctx, opts, cli.count)
		success = ok
		display.StressSummary(stats, cli.count)
	} else {
		// Normal mode
		success = runner.Run(ctx, opts)
		display.Summary()
	}

	if !success {
		return errTestsFailed
	}

	return nil
}

type cliFlags struct {
	command   string
	all       bool
	list      bool
	shortList bool
	timeout   time.Duration
	parallel  int
	verbose   bool
	quiet     bool
	saveDir   string
	port      int
	server    string
	client    string
	count     int
	testArgs  []string
}

func parseCLI() *cliFlags {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	command := os.Args[1]
	if command == "-h" || command == "--help" || command == "help" {
		printUsage()
		return nil
	}

	if command != "encoding" && command != "api" {
		fmt.Fprintf(os.Stderr, "Unknown command: %s (use 'encoding' or 'api')\n", command)
		printUsage()
		return nil
	}

	cli := &cliFlags{command: command}

	fs := flag.NewFlagSet(command, flag.ExitOnError)
	fs.BoolVar(&cli.all, "all", false, "run all tests")
	fs.BoolVar(&cli.list, "list", false, "list available tests")
	fs.BoolVar(&cli.list, "l", false, "list available tests (shorthand)")
	fs.BoolVar(&cli.shortList, "short-list", false, "list test codes only")
	fs.DurationVar(&cli.timeout, "timeout", 30*time.Second, "timeout per test")
	fs.IntVar(&cli.parallel, "parallel", 4, "max concurrent tests")
	fs.BoolVar(&cli.verbose, "verbose", false, "verbose output")
	fs.BoolVar(&cli.verbose, "v", false, "verbose output (shorthand)")
	fs.BoolVar(&cli.quiet, "quiet", false, "minimal output")
	fs.BoolVar(&cli.quiet, "q", false, "minimal output (shorthand)")
	fs.StringVar(&cli.saveDir, "save", "", "save logs to directory")
	fs.IntVar(&cli.port, "port", 1790, "base port to use")
	fs.StringVar(&cli.server, "server", "", "run server only for test")
	fs.StringVar(&cli.client, "client", "", "run client only for test")
	fs.IntVar(&cli.count, "count", 1, "run each test N times (stress testing)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return nil
	}

	cli.testArgs = fs.Args()

	return cli
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: selfcheck <command> [options] [tests...]

Commands:
  encoding    Run encoding tests (static routes)
  api         Run API tests (dynamic routes via .run scripts)

Modes:
  --list, -l          List available tests
  --short-list        List test codes only (space separated)
  --all               Run all tests

Options:
  --timeout N         Timeout per test (default: 30s)
  --parallel N        Max concurrent tests (default: 4)
  --verbose, -v       Show output for each test
  --quiet, -q         Minimal output
  --save DIR          Save logs to directory
  --port N            Base port to use (default: 1790)
  --count N           Run each test N times (stress testing)

Debugging:
  --server NICK       Run server only for test
  --client NICK       Run client only for test

Examples:
  selfcheck encoding --list
  selfcheck encoding --all
  selfcheck encoding 0 1 2
  selfcheck api --all --quiet
  selfcheck encoding --count 10 0 1    # stress test: run tests 0,1 ten times
`)
}

func findBaseDir() (string, error) {
	// Start from current directory, look for go.mod
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find go.mod in parent directories")
}
