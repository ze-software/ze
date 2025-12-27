// Command functional runs ZeBGP functional tests.
//
// This is modeled after ExaBGP's qa/bin/functional test runner,
// providing state machine-based test lifecycle, concurrent execution,
// timing tracking, and rich output.
//
// Usage:
//
//	functional encoding --list
//	functional encoding 0 1 2
//	functional encoding --all
//	functional api --all --verbose
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	functional "github.com/exa-networks/zebgp/test/pkg"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Find project root
	baseDir, err := findBaseDir()
	if err != nil {
		return err
	}

	// Parse CLI
	cli := functional.DefaultCLI()
	if err := cli.Parse(os.Args[1:]); err != nil {
		cli.PrintUsage()
		return err
	}

	// Handle based on command
	switch cli.Command {
	case "encoding":
		return runEncoding(baseDir, cli)
	case "api":
		return runAPI(baseDir, cli)
	default:
		cli.PrintUsage()
		return fmt.Errorf("unknown command: %s", cli.Command)
	}
}

//nolint:dupl // Encoding and API runners are intentionally separate
func runEncoding(baseDir string, cli *functional.CLI) error {
	encodeDir := filepath.Join(baseDir, "test", "data", "encode")

	// Create test manager
	functional.ResetNickCounter()
	tests := functional.NewEncodingTests(baseDir)
	if err := tests.Discover(encodeDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	// Handle modes
	if cli.List {
		tests.List()
		return nil
	}

	if cli.ShortList {
		for _, r := range tests.Registered() {
			fmt.Print(r.Nick + " ")
		}
		fmt.Println()
		return nil
	}

	if cli.Edit {
		return editTest(tests.Tests, cli.TestArgs)
	}

	// Select tests
	if err := selectTests(tests.Tests, cli); err != nil {
		return err
	}
	if len(tests.Selected()) == 0 {
		tests.List()
		fmt.Println("Use --all to run all tests, or specify test nick(s)")
		return nil
	}

	// Build binaries
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupSignalHandler(cancel)

	fmt.Println("Building test binaries...")
	runner, err := functional.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer runner.Cleanup()

	if err := runner.Build(ctx); err != nil {
		return fmt.Errorf("build: %w", err)
	}

	// Run tests
	opts := cli.ToRunOptions()
	success := runner.Run(ctx, opts)

	// Print summary
	passed, failed, timedOut, skipped := tests.Summary()
	functional.PrintSummary(passed, failed, timedOut, skipped, tests.FailedNicks())

	if !success {
		return fmt.Errorf("some tests failed")
	}
	return nil
}

//nolint:dupl // API and Encoding runners are intentionally separate
func runAPI(baseDir string, cli *functional.CLI) error {
	apiDir := filepath.Join(baseDir, "test", "data", "api")

	// Create test manager
	functional.ResetNickCounter()
	tests := functional.NewAPITests(baseDir)
	if err := tests.Discover(apiDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	// Handle modes
	if cli.List {
		tests.List()
		return nil
	}

	if cli.ShortList {
		for _, r := range tests.Registered() {
			fmt.Print(r.Nick + " ")
		}
		fmt.Println()
		return nil
	}

	if cli.Edit {
		return editTest(tests.Tests, cli.TestArgs)
	}

	// Select tests
	if err := selectTests(tests.Tests, cli); err != nil {
		return err
	}
	if len(tests.Selected()) == 0 {
		tests.List()
		fmt.Println("Use --all to run all tests, or specify test nick(s)")
		return nil
	}

	// Build binaries
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupSignalHandler(cancel)

	fmt.Println("Building test binaries...")
	runner, err := functional.NewAPIRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer runner.Cleanup()

	if err := runner.Build(ctx); err != nil {
		return fmt.Errorf("build: %w", err)
	}

	// Run tests
	opts := cli.ToRunOptions()
	success := runner.Run(ctx, opts)

	// Print summary
	passed, failed, timedOut, skipped := tests.Summary()
	functional.PrintSummary(passed, failed, timedOut, skipped, tests.FailedNicks())

	if !success {
		return fmt.Errorf("some tests failed")
	}
	return nil
}

// selectTests enables tests based on CLI flags.
func selectTests(tests *functional.Tests, cli *functional.CLI) error {
	if cli.All {
		tests.EnableAll()
		return nil
	}
	for _, nick := range cli.TestArgs {
		if !tests.EnableByNick(nick) {
			return fmt.Errorf("unknown test: %s", nick)
		}
	}
	return nil
}

// setupSignalHandler sets up SIGINT/SIGTERM handling.
func setupSignalHandler(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
}

func editTest(tests *functional.Tests, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("--edit requires exactly one test nick")
	}

	r := tests.GetByNick(args[0])
	if r == nil {
		return fmt.Errorf("unknown test: %s", args[0])
	}

	if len(r.Files) == 0 {
		return fmt.Errorf("no files for test: %s", args[0])
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, r.Files...) //nolint:gosec,noctx // Files from known test dir, interactive command
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func findBaseDir() (string, error) {
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

	return os.Getwd()
}
