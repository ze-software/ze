// Design: docs/architecture/testing/ci-format.md -- ze-test vpp subcommand
// Related: bgp.go -- shared test-runner infrastructure and CLI parser
//
// ze-test vpp runs the GoVPP-stub-backed functional tests under test/vpp/.
// The .ci files embed a Python driver that spawns test/scripts/vpp-stub.py
// and ze, drives the scenario, and asserts against the JSONL request log.
// Shape matches `ze-test bgp plugin` so ze and ze-test build once and the
// runner lifecycle (discover, build, run, summary) is shared.

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

func vppCmd() int {
	if err := vppMain(); err != nil {
		if !errors.Is(err, errTestsFailed) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1
	}
	return 0
}

func vppMain() error {
	cli, ok := parseVPPCLI()
	if !ok {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	baseDir, err := findBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	runner.ResetNickCounter()

	tests := runner.NewEncodingTests(baseDir)
	testDir := filepath.Join(baseDir, "test", "vpp")
	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	if cli.list {
		tests.List()
		return nil
	}

	switch {
	case cli.all:
		tests.EnableAll()
	case len(cli.testArgs) > 0:
		for _, arg := range cli.testArgs {
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
		printVPPUsage()
		return nil
	}

	tests.Sort()

	r, err := runner.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer r.Cleanup()

	r.Display().SetLabel("vpp")
	r.Report().SetLabel("vpp")
	r.Display().Header()

	// VPP stub tests do not bind to network ports; the plugin tests'
	// per-test port reservation is unnecessary here, but the runner expects
	// at least some allocation. Reserve one port per test at a high base.
	pr, shifted, err := runner.AllocatePorts(cli.port, tests.Count())
	if err != nil {
		return fmt.Errorf("allocate ports: %w", err)
	}
	basePort := pr.Start
	for _, rr := range tests.Registered() {
		rr.Port = basePort
		basePort++
	}
	r.Display().PortInfo(pr, shifted)

	if err := r.Build(ctx); err != nil {
		return err
	}

	opts := &runner.RunOptions{
		Timeout:  cli.timeout,
		Parallel: cli.parallel,
		Verbose:  cli.verbose,
		Quiet:    cli.quiet,
		SaveDir:  cli.saveDir,
	}

	success := r.Run(ctx, opts)
	r.Display().Summary()
	r.Display().TimingDetail("vpp", r.Timings())
	r.Display().DebugHints()

	if !success {
		return errTestsFailed
	}
	return nil
}

type vppCLIFlags struct {
	all      bool
	list     bool
	timeout  time.Duration
	parallel int
	verbose  bool
	quiet    bool
	saveDir  string
	port     int
	testArgs []string
}

func parseVPPCLI() (*vppCLIFlags, bool) {
	if len(os.Args) < 1 {
		printVPPUsage()
		return nil, false
	}

	fs := flag.NewFlagSet("vpp", flag.ExitOnError)
	cli := &vppCLIFlags{}
	fs.BoolVar(&cli.all, "a", false, "run all tests")
	fs.BoolVar(&cli.all, "all", false, "run all tests")
	fs.BoolVar(&cli.list, "l", false, "list available tests")
	fs.BoolVar(&cli.list, "list", false, "list available tests")
	fs.DurationVar(&cli.timeout, "t", 30*time.Second, "timeout per test")
	fs.DurationVar(&cli.timeout, "timeout", 30*time.Second, "timeout per test")
	fs.IntVar(&cli.parallel, "p", 1, "max concurrent tests (default 1 -- stub binds a Unix socket per test)")
	fs.IntVar(&cli.parallel, "parallel", 1, "max concurrent tests")
	fs.BoolVar(&cli.verbose, "v", false, "verbose output")
	fs.BoolVar(&cli.verbose, "verbose", false, "verbose output")
	fs.BoolVar(&cli.quiet, "q", false, "minimal output")
	fs.BoolVar(&cli.quiet, "quiet", false, "minimal output")
	fs.StringVar(&cli.saveDir, "s", "", "save logs to directory")
	fs.StringVar(&cli.saveDir, "save", "", "save logs to directory")
	fs.IntVar(&cli.port, "port", 21790, "base port reservation (unused by stub, but runner needs one)")

	// os.Args[0] is this subcommand's name after main.go shifted args.
	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, false
	}
	cli.testArgs = fs.Args()

	if len(cli.testArgs) > 0 && isHelpArg(cli.testArgs[0]) {
		printVPPUsage()
		return nil, false
	}

	return cli, true
}

func printVPPUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze-test vpp [options] [tests...]

Run VPP stub-backed functional tests from test/vpp/.

Modes:
  -l, --list          List available tests
  -a, --all           Run all tests

Options:
  -t, --timeout N     Timeout per test (default: 30s)
  -p, --parallel N    Max concurrent tests (default: 1)
  -v, --verbose       Show output for each test
  -q, --quiet         Minimal output
  -s, --save DIR      Save logs to directory

Examples:
  ze-test vpp -l
  ze-test vpp -a
  ze-test vpp 001-boot
  ze-test vpp 0 1         # by nick
`)
}
