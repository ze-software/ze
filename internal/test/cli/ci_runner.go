// Design: docs/architecture/testing/ci-format.md -- shared .ci test runner logic

package cli

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

type CIRunnerConfig struct {
	Name            string
	TestSubdir      string
	Description     string
	Detail          string
	DefaultParallel int
}

func RunCISubcommand(cfg CIRunnerConfig, args []string) int {
	if err := runCISubcommandInner(cfg, args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func runCISubcommandInner(cfg CIRunnerConfig, args []string) error {
	fs := flag.NewFlagSet(cfg.Name, flag.ContinueOnError)
	all := fs.Bool("a", false, "run all tests")
	fs.BoolVar(all, "all", false, "run all tests")
	listOnly := fs.Bool("l", false, "list available tests")
	fs.BoolVar(listOnly, "list", false, "list available tests")
	start := fs.String("start", "", "start at test id/name and run through the end")
	pattern := fs.String("pattern", "", "run tests whose id, name, or path contains pattern")
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	quiet := fs.Bool("q", false, "minimal output")
	fs.BoolVar(quiet, "quiet", false, "minimal output")
	parallel := fs.Int("p", cfg.DefaultParallel, "max concurrent tests (0 = all)")
	fs.IntVar(parallel, "parallel", cfg.DefaultParallel, "max concurrent tests (0 = all)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze-test %s [options] [test-ids...]\n\n%s\n\nOptions:\n", cfg.Name, cfg.Detail) //nolint:errcheck // terminal output
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n  ze-test %s -a              # Run all %s tests\n  ze-test %s -l              # List available tests with N/TOTAL and id\n  ze-test %s 0 1 2           # Run specific tests by id\n  ze-test %s --start 42      # Resume at id 42 and run through the end\n  ze-test %s --pattern bgp   # Run tests whose id, name, or path contains bgp\n  ze-test %s -a -v           # Verbose output\n", cfg.Name, cfg.Description, cfg.Name, cfg.Name, cfg.Name, cfg.Name, cfg.Name) //nolint:errcheck // terminal output
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

	runner.ResetNickCounter()
	tests := runner.NewEncodingTests(baseDir)
	testDir := filepath.Join(baseDir, "test", cfg.TestSubdir)

	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	if tests.Count() == 0 {
		return fmt.Errorf("no .ci files found in %s", testDir)
	}

	tests.Sort()

	if *listOnly {
		tests.List()
		return nil
	}

	selected, err := tests.Select(runner.Selection{
		All:     *all,
		Start:   *start,
		Pattern: *pattern,
		Args:    fs.Args(),
	})
	if err != nil {
		return err
	}
	if selected == 0 {
		fs.Usage()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
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

	ConfigureCIRunnerOutput(r, cfg.Name)

	opts := &runner.RunOptions{
		Timeout:  15 * time.Second,
		Verbose:  *verbose,
		Quiet:    *quiet,
		Parallel: *parallel,
	}

	success := r.Run(ctx, opts)

	if !success {
		return ErrTestsFailed
	}

	return nil
}

func ConfigureCIRunnerOutput(r *runner.Runner, suite string) {
	r.Display().SetLabel(suite)
	r.Report().SetLabel(suite)
	r.Display().Header()
}
