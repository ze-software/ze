// Design: docs/architecture/testing/ci-format.md -- ze-test vpp subcommand

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

func cmdVpp(args []string) int {
	if err := zeTestVppMain(args); err != nil {
		if !errors.Is(err, ErrTestsFailed) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1
	}
	return 0
}

func zeTestVppMain(args []string) error {
	cli, ok := zeTestParseVPPCLI(args)
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

	baseDir, err := FindBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	runner.ResetNickCounter()

	tests := runner.NewEncodingTests(baseDir)
	testDir := runner.SuiteDir(baseDir, "vpp", cli.draft)
	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	tests.Sort()

	if cli.list {
		tests.List()
		return nil
	}

	selected, err := tests.Select(runner.Selection{
		All:     cli.all,
		Start:   cli.start,
		Pattern: cli.pattern,
		Args:    cli.testArgs,
	})
	if err != nil {
		return err
	}
	if selected == 0 {
		zeTestPrintVPPUsage()
		return nil
	}

	r, err := runner.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer r.Cleanup()

	r.Display().SetLabel("vpp")
	r.Report().SetLabel("vpp")
	r.Display().Header()

	portReservation, shifted, err := runner.ReservePorts(cli.port, tests.Count())
	if err != nil {
		return fmt.Errorf("allocate ports: %w", err)
	}
	defer portReservation.Release()
	pr := portReservation.PortRange
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

	if !success {
		return ErrTestsFailed
	}
	return nil
}

type vppCLIFlags struct {
	all      bool
	list     bool
	start    string
	pattern  string
	timeout  time.Duration
	parallel int
	verbose  bool
	quiet    bool
	saveDir  string
	port     int
	// draft discovers from test/draft/vpp instead of test/vpp (test/draft/README.md).
	draft    bool
	testArgs []string
}

func zeTestParseVPPCLI(args []string) (*vppCLIFlags, bool) {
	fs := flag.NewFlagSet("vpp", flag.ExitOnError)
	cli := &vppCLIFlags{}
	fs.BoolVar(&cli.all, "a", false, "run all tests")
	fs.BoolVar(&cli.all, "all", false, "run all tests")
	fs.BoolVar(&cli.list, "l", false, "list available tests")
	fs.BoolVar(&cli.list, "list", false, "list available tests")
	fs.StringVar(&cli.start, "start", "", "start at test id/name and run through the end")
	fs.StringVar(&cli.pattern, "pattern", "", "run tests whose id, name, or path contains pattern")
	fs.DurationVar(&cli.timeout, "t", 30*time.Second, "timeout per test")
	fs.DurationVar(&cli.timeout, "timeout", 30*time.Second, "timeout per test")
	fs.IntVar(&cli.parallel, "p", 1, "max concurrent tests (default 1)")
	fs.IntVar(&cli.parallel, "parallel", 1, "max concurrent tests")
	fs.BoolVar(&cli.verbose, "v", false, "verbose output")
	fs.BoolVar(&cli.verbose, "verbose", false, "verbose output")
	fs.BoolVar(&cli.quiet, "q", false, "minimal output")
	fs.BoolVar(&cli.quiet, "quiet", false, "minimal output")
	fs.StringVar(&cli.saveDir, "s", "", "save logs to directory")
	fs.StringVar(&cli.saveDir, "save", "", "save logs to directory")
	fs.IntVar(&cli.port, "port", 21790, "base port reservation (unused by stub, but runner needs one)")
	fs.BoolVar(&cli.draft, "draft", false, "discover from test/draft/vpp instead of test/vpp (tests under development)")

	if err := fs.Parse(args); err != nil {
		return nil, false
	}
	cli.testArgs = fs.Args()

	if len(cli.testArgs) > 0 && isHelpArg(cli.testArgs[0]) {
		zeTestPrintVPPUsage()
		return nil, false
	}

	return cli, true
}

func zeTestPrintVPPUsage() {
	_, _ = os.Stderr.WriteString(`Usage: ze-test vpp [options] [tests...]

Run VPP stub-backed functional tests from test/vpp/.

Modes:
  -l, --list          List available tests with N/TOTAL and one-based id
  -a, --all           Run all tests
  --start ID          Start at test id/name and run through the end
  --pattern TEXT      Run tests whose id, name, or path contains TEXT
Options:
  -t, --timeout N     Timeout per test (default: 30s)
  -p, --parallel N    Max concurrent tests (default: 1)
  -v, --verbose       Show output for each test
  -q, --quiet         Minimal output
  -s, --save DIR      Save logs to directory

Examples:
  ze-test vpp -l
  ze-test vpp -a
  ze-test vpp 1 2         # by numeric id
  ze-test vpp --start 4
`)
}
