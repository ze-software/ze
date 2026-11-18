// Design: docs/architecture/testing/ci-format.md -- shared .ci test runner logic

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/test/runner"
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
	listOnly := fs.Bool("l", false, listFlagUsage)
	fs.BoolVar(listOnly, "list", false, listFlagUsage)
	start := fs.String("start", "", "start at test id/name and run through the end")
	pattern := fs.String("pattern", "", "run tests whose id or name contains pattern (as typed, not a regex or a glob)")
	verbose := fs.Bool("v", false, "verbose output")
	fs.BoolVar(verbose, "verbose", false, "verbose output")
	quiet := fs.Bool("q", false, "minimal output")
	fs.BoolVar(quiet, "quiet", false, "minimal output")
	parallel := fs.Int("p", cfg.DefaultParallel, "max concurrent tests (0 = all)")
	fs.IntVar(parallel, "parallel", cfg.DefaultParallel, "max concurrent tests (0 = all)")
	saveDir := fs.String("save", "", "save captured test output to directory")
	fs.StringVar(saveDir, "s", "", "save captured test output to directory")
	draft := fs.Bool("draft", false, "discover from test/draft/<suite> instead of test/<suite> (tests under development)")

	fs.Usage = func() {
		var b textbuf.Buffer
		b.Str("Usage: le test ").Str(cfg.Name).Str(" [options] [test-ids...]\n\n").Str(cfg.Detail).Str("\n\nOptions:\n")
		_ = b.StdErr()
		fs.PrintDefaults()
		b.Reset().Str("\nExamples:\n  le test ").Str(cfg.Name).Str(" -a              # Run all ").Str(cfg.Description).
			Str(" tests\n  le test ").Str(cfg.Name).Str(" -l              # List every test with N/TOTAL and one-based id\n  le test ").
			Str(cfg.Name).Str(" -l --pattern bgp # List only the tests --pattern bgp runs\n  le test ").
			Str(cfg.Name).Str(" 1 2 3           # Run specific tests by id\n  le test ").Str(cfg.Name).
			Str(" --start 42      # Resume at id 42 and run through the end\n  le test ").Str(cfg.Name).
			Str(" --pattern bgp   # Run tests whose id or name contains bgp (as typed, not a regex or a glob)\n  le test ").Str(cfg.Name).Str(" -a -v           # Verbose output\n")
		_ = b.StdErr()
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
	testDir := runner.SuiteDir(baseDir, cfg.TestSubdir, *draft)

	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	if tests.Count() == 0 {
		return fmt.Errorf("no .ci files found in %s", testDir)
	}

	tests.Sort()

	sel := runner.Selection{
		All:     *all,
		Start:   *start,
		Pattern: *pattern,
		Args:    fs.Args(),
	}
	if *listOnly {
		return tests.List(sel)
	}

	selected, err := tests.Select(sel)
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

	configureCIRunnerOutput(r, cfg.Name)

	opts := &runner.RunOptions{
		Timeout:  15 * time.Second,
		Verbose:  *verbose,
		Quiet:    *quiet,
		Parallel: *parallel,
		SaveDir:  *saveDir,
	}

	success := r.Run(ctx, opts)

	if !success {
		return ErrTestsFailed
	}

	return nil
}

func configureCIRunnerOutput(r *runner.Runner, suite string) {
	r.Display().SetLabel(suite)
	r.Report().SetLabel(suite)
	r.Display().Header()
}
