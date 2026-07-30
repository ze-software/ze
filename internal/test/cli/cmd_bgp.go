// Design: docs/architecture/testing/ci-format.md — test runner CLI

package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/peer"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/internal/test/sessionpath"
)

var errPeerCheckFailed = errors.New("peer check failed")

const (
	cmdPlugin    = "plugin"
	cmdReload    = "reload"
	cmdChaosWeb  = "chaos-web"
	cmdChaosIntg = "chaos"
)

// bgpCIRunnerDirs is the set of "ze-test bgp <sub>" subcommands. Each name is
// also the test/<name> directory that subcommand walks (see zeTestRunEncodingOrAPI
// and zeTestRunSimpleTests), so it is the single source of truth for both argument
// validation (the `if !bgpCIRunnerDirs[command]` gate below) and the orphaned-suite
// guard (TestCIRootsRegistered): those dirs are covered by a big runner, not
// registerCIRoot.
var bgpCIRunnerDirs = map[string]bool{
	"encode":     true,
	cmdPlugin:    true,
	"decode":     true,
	"parse":      true,
	cmdReload:    true,
	cmdChaosIntg: true,
	cmdChaosWeb:  true,
}

func cmdBgp(args []string) int {
	if err := zeTestBgpMain(args); err != nil {
		if !errors.Is(err, ErrTestsFailed) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1
	}
	return 0
}

func zeTestBgpMain(args []string) error {
	cli := zeTestParseRunCLI(args)
	if cli == nil {
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

	switch cli.command {
	case "encode", cmdPlugin, cmdReload, cmdChaosWeb, cmdChaosIntg:
		return zeTestRunEncodingOrAPI(ctx, cli, baseDir)
	case "decode":
		return zeTestRunSimpleTests(ctx, cli, baseDir, zeTestNewDecodingTestSuite)
	case "parse":
		return zeTestRunSimpleTests(ctx, cli, baseDir, zeTestNewParsingTestSuite)
	default:
		return fmt.Errorf("unknown command: %s", cli.command)
	}
}

type zeTestSuite interface {
	Discover(dir string) error
	List()
	Select(runner.Selection) (int, error)
	getNicks() []string
	Run(ctx context.Context, zePath string, verbose, quiet bool) bool
}

type zeTestDecodeTestSuite struct {
	*runner.DecodingTests
	baseDir string
}

func zeTestNewDecodingTestSuite(baseDir string) zeTestSuite {
	return &zeTestDecodeTestSuite{
		DecodingTests: runner.NewDecodingTests(baseDir),
		baseDir:       baseDir,
	}
}

func (d *zeTestDecodeTestSuite) getNicks() []string {
	registered := d.Registered()
	nicks := make([]string, 0, len(registered))
	for _, t := range registered {
		nicks = append(nicks, t.Nick)
	}
	return nicks
}

func (d *zeTestDecodeTestSuite) Run(ctx context.Context, zePath string, verbose, quiet bool) bool {
	r := runner.NewDecodingRunner(d.DecodingTests, d.baseDir, zePath)
	return r.Run(ctx, verbose, quiet)
}

type zeTestParseTestSuite struct {
	*runner.ParsingTests
	baseDir string
}

func zeTestNewParsingTestSuite(baseDir string) zeTestSuite {
	return &zeTestParseTestSuite{
		ParsingTests: runner.NewParsingTests(baseDir),
		baseDir:      baseDir,
	}
}

func (p *zeTestParseTestSuite) getNicks() []string {
	registered := p.Registered()
	nicks := make([]string, 0, len(registered))
	for _, t := range registered {
		nicks = append(nicks, t.Nick)
	}
	return nicks
}

func (p *zeTestParseTestSuite) Run(ctx context.Context, zePath string, verbose, quiet bool) bool {
	r := runner.NewParsingRunner(p.ParsingTests, p.baseDir, zePath)
	return r.Run(ctx, verbose, quiet)
}

func zeTestRunSimpleTests(ctx context.Context, cli *zeTestRunCLIFlags, baseDir string, newSuite func(string) zeTestSuite) error {
	runner.ResetNickCounter()

	tests := newSuite(baseDir)

	var testDir string
	switch cli.command {
	case "decode":
		testDir = runner.SuiteDir(baseDir, "decode", cli.draft)
	case "parse":
		testDir = runner.SuiteDir(baseDir, "parse", cli.draft)
	}

	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	if cli.list {
		tests.List()
		return nil
	}

	if cli.shortList {
		for _, nick := range tests.getNicks() {
			fmt.Fprintf(os.Stdout, "%s ", nick) //nolint:errcheck // terminal output
		}
		fmt.Fprintln(os.Stdout) //nolint:errcheck // terminal output
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
		zeTestPrintRunUsage()
		return nil
	}

	if !cli.quiet {
		runner.PrintHeader(cli.command)
	}

	zePath, err := buildZe(ctx, baseDir)
	if err != nil {
		return err
	}

	success := tests.Run(ctx, zePath, cli.verbose, cli.quiet)

	if !success {
		return ErrTestsFailed
	}

	return nil
}

func zeTestRunEncodingOrAPI(ctx context.Context, cli *zeTestRunCLIFlags, baseDir string) error {
	runner.ResetNickCounter()

	tests := runner.NewEncodingTests(baseDir)
	suite := "encode"
	switch cli.command {
	case cmdPlugin:
		suite = "plugin"
	case cmdReload:
		suite = cmdReload
	case cmdChaosWeb:
		suite = "chaos-web"
	case cmdChaosIntg:
		suite = "chaos"
	}
	testDir := runner.SuiteDir(baseDir, suite, cli.draft)

	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	if cli.server != "" {
		return zeTestRunServerOnly(ctx, cli, tests, baseDir)
	}
	if cli.client != "" {
		return zeTestRunClientOnly(ctx, cli, tests, baseDir)
	}

	if cli.list {
		tests.List()
		return nil
	}

	if cli.shortList {
		for _, r := range tests.Registered() {
			fmt.Fprintf(os.Stdout, "%s ", r.Nick) //nolint:errcheck // terminal output
		}
		fmt.Fprintln(os.Stdout) //nolint:errcheck // terminal output
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
		zeTestPrintRunUsage()
		return nil
	}

	r, err := runner.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer r.Cleanup()

	// ze-chaos drives an in-process BGP reactor, so it must also set ze_bgp.
	// The ze_chaos tag alone does not add ZE_FEATURES. Without ze_bgp, the BGP
	// YANG modules are not linked.
	//
	// The binary still BUILDS. It then stops at startup with "resolve YANG
	// modules: no such module: ze-bgp-conf". Every chaos-web and
	// chaos-integration test then fails on a refused connection, and no client
	// output explains it.
	//
	// The Makefile `chaos` rule carries the same pair for the same reason.
	// These two must not drift apart.
	const chaosTags = "ze_chaos ze_bgp"
	switch cli.command {
	case cmdChaosWeb:
		r.SetExtraBinaries(map[string]runner.ExtraBinary{
			"ze-chaos": {Pkg: "./cmd/ze", Tags: chaosTags},
		})
	case cmdChaosIntg:
		r.SetExtraBinaries(map[string]runner.ExtraBinary{
			"ze-chaos": {Pkg: "./cmd/ze", Tags: chaosTags},
			"ze":       {Pkg: "./cmd/ze"},
		})
	}

	r.Display().SetLabel(cli.command)
	r.Report().SetLabel(cli.command)
	r.Display().Header()

	limitCheck, err := runner.CheckUlimit(cli.parallel)
	if err != nil {
		return fmt.Errorf("ulimit check: %w", err)
	}
	r.Display().UlimitInfo(limitCheck)

	portReservation, shifted, err := runner.ReservePorts(cli.port, tests.Count()*2)
	if err != nil {
		return fmt.Errorf("allocate ports: %w", err)
	}
	defer portReservation.Release()
	pr := portReservation.PortRange

	basePort := pr.Start
	for _, rr := range tests.Registered() {
		rr.Port = basePort
		basePort += 2
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

	var success bool
	if cli.count > 1 {
		result := r.RunWithCount(ctx, opts, cli.count)
		success = result.AllPassed
		r.Display().StressSummary(result, cli.count)
	} else {
		success = r.Run(ctx, opts)
	}

	if !success {
		return ErrTestsFailed
	}

	return nil
}

func zeTestRunServerOnly(ctx context.Context, cli *zeTestRunCLIFlags, tests *runner.EncodingTests, _ string) error {
	rec := tests.GetByNick(cli.server)
	if rec == nil {
		for _, r := range tests.Registered() {
			if r.Name == cli.server {
				rec = r
				break
			}
		}
	}
	if rec == nil {
		return fmt.Errorf("test not found: %s", cli.server)
	}

	expects := make([]string, 0, len(rec.Options)+len(rec.Expects))
	expects = append(expects, rec.Options...)
	expects = append(expects, rec.Expects...)

	port := cli.port
	if rec.Port != 0 {
		port = rec.Port
	}

	config := &peer.Config{
		Port:   port,
		Mode:   peer.ModeCheck,
		Expect: expects,
		Output: os.Stdout,
	}

	if asn, ok := rec.Extra["asn"]; ok {
		if v, err := strconv.Atoi(asn); err == nil {
			config.ASN = v
		}
	}
	if rec.Extra["bind"] == "ipv6" {
		config.IPv6 = true
	}

	fmt.Fprintf(os.Stdout, "Server mode for test: %s (%s)\n", rec.Nick, rec.Name)                        //nolint:errcheck // terminal output
	fmt.Fprintf(os.Stdout, "Config: %s\n", rec.ConfigFile)                                               //nolint:errcheck // terminal output
	fmt.Fprintf(os.Stdout, "Port: %d\n", port)                                                           //nolint:errcheck // terminal output
	fmt.Fprintf(os.Stdout, "Waiting for client connection...\n")                                         //nolint:errcheck // terminal output
	fmt.Fprintf(os.Stdout, "\nRun client in another terminal:\n")                                        //nolint:errcheck // terminal output
	fmt.Fprintf(os.Stdout, "   ze-test bgp %s --client %s --port %d\n\n", cli.command, cli.server, port) //nolint:errcheck // terminal output

	p, err := peer.New(config)
	if err != nil {
		return fmt.Errorf("create peer: %w", err)
	}
	result := p.Run(ctx)

	fmt.Fprintln(os.Stdout) //nolint:errcheck // terminal output

	if result.Error != nil {
		return result.Error
	}
	if !result.Success {
		return errPeerCheckFailed
	}

	fmt.Fprintln(os.Stdout, "successful") //nolint:errcheck // terminal output
	return nil
}

func zeTestRunClientOnly(ctx context.Context, cli *zeTestRunCLIFlags, tests *runner.EncodingTests, baseDir string) error {
	rec := tests.GetByNick(cli.client)
	if rec == nil {
		for _, r := range tests.Registered() {
			if r.Name == cli.client {
				rec = r
				break
			}
		}
	}
	if rec == nil {
		return fmt.Errorf("test not found: %s", cli.client)
	}

	configPath, ok := rec.Conf["config"].(string)
	if !ok || configPath == "" {
		return fmt.Errorf("test %s has no config file", cli.client)
	}

	zePath, err := buildZe(ctx, baseDir)
	if err != nil {
		return err
	}

	port := cli.port
	if rec.Port != 0 {
		port = rec.Port
	}
	fmt.Fprintf(os.Stdout, "Client mode for test: %s (%s)\n", rec.Nick, rec.Name)                        //nolint:errcheck // terminal output
	fmt.Fprintf(os.Stdout, "Config: %s\n", configPath)                                                   //nolint:errcheck // terminal output
	fmt.Fprintf(os.Stdout, "Port: %d\n", port)                                                           //nolint:errcheck // output
	fmt.Fprintf(os.Stdout, "Starting ze bgp client...\n")                                                //nolint:errcheck // output
	fmt.Fprintf(os.Stdout, "\nServer should be running. If not:\n")                                      //nolint:errcheck // output
	fmt.Fprintf(os.Stdout, "   ze-test bgp %s --server %s --port %d\n\n", cli.command, cli.client, port) //nolint:errcheck // output

	zeDir := filepath.Dir(zePath)
	existingPath := os.Getenv("PATH")
	clientEnv := append(os.Environ(),
		"ze_test_bgp_port="+strconv.Itoa(port),
		"PATH="+zeDir+":"+existingPath,
	)

	clientCmd := exec.CommandContext(ctx, zePath, "server", configPath) //nolint:gosec // test runner, paths from temp dir
	clientCmd.Cancel = func() error {
		if clientCmd.Process == nil {
			return nil
		}
		return clientCmd.Process.Signal(syscall.SIGTERM)
	}
	clientCmd.WaitDelay = 5 * time.Second
	clientCmd.Env = clientEnv
	clientCmd.Stdout = os.Stdout
	clientCmd.Stderr = os.Stderr

	return clientCmd.Run()
}

func buildZe(ctx context.Context, baseDir string) (string, error) {
	// "ze.bin" and "ze.test.no.build" are registered in internal/test/runner.
	// BinDir is <baseDir>/bin off-session and this session's private bin/ under
	// an AI session, so this build cannot overwrite a sibling session's ze while
	// that session is running tests against it (same reasoning as runner.NewRunner).
	zePath := filepath.Join(sessionpath.BinDir(baseDir), "ze")
	if v := env.Get("ze.bin"); v != "" {
		if !filepath.IsAbs(v) {
			v = filepath.Join(baseDir, v)
		}
		zePath = v
	}
	if env.IsEnabled("ze.test.no.build") {
		_, err := os.Stat(zePath)
		if err == nil {
			return zePath, nil
		}
		// Fall back to a pre-built binary in the shared bin/ (same reasoning as
		// runner.verifyPrebuilt: reading someone's build clobbers nothing).
		// An explicit ze.bin is exempt -- it names one binary, so a miss is fatal.
		if env.Get("ze.bin") == "" {
			if dir := sessionpath.FindPrebuiltDir(baseDir, "ze"); dir != "" {
				return filepath.Join(dir, "ze"), nil
			}
		}
		return "", fmt.Errorf("ZE_TEST_NO_BUILD set but %s is missing (cross-compile it first): %w", zePath, err)
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", runner.TestBuildTags(), "-o", zePath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build ze: %w: %s", err, output)
	}

	return zePath, nil
}

type zeTestRunCLIFlags struct {
	command   string
	all       bool
	list      bool
	start     string
	pattern   string
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
	// draft discovers from test/draft/<suite> instead of test/<suite>, so a test
	// under development runs without being visible to any suite or gate
	// (test/draft/README.md).
	draft    bool
	testArgs []string
}

func zeTestParseRunCLI(args []string) *zeTestRunCLIFlags {
	if len(args) < 1 {
		zeTestPrintRunUsage()
		return nil
	}

	command := args[0]
	if isHelpArg(command) {
		zeTestPrintRunUsage()
		return nil
	}

	if !bgpCIRunnerDirs[command] {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		zeTestPrintRunUsage()
		return nil
	}

	cli := &zeTestRunCLIFlags{command: command}

	fs := flag.NewFlagSet(command, flag.ExitOnError)
	fs.BoolVar(&cli.all, "a", false, "run all tests")
	fs.BoolVar(&cli.all, "all", false, "run all tests")
	fs.BoolVar(&cli.list, "l", false, "list available tests")
	fs.BoolVar(&cli.list, "list", false, "list available tests")
	fs.StringVar(&cli.start, "start", "", "start at test id/name and run through the end")
	fs.StringVar(&cli.pattern, "pattern", "", "run tests whose id, name, or path contains pattern")
	fs.BoolVar(&cli.shortList, "short-list", false, "list numeric test ids only")
	fs.DurationVar(&cli.timeout, "t", 15*time.Second, "timeout per test")
	fs.DurationVar(&cli.timeout, "timeout", 15*time.Second, "timeout per test")
	fs.IntVar(&cli.parallel, "p", runner.DefaultParallelConcurrent, "max concurrent tests (0 = all)")
	fs.IntVar(&cli.parallel, "parallel", runner.DefaultParallelConcurrent, "max concurrent tests (0 = all)")
	fs.BoolVar(&cli.verbose, "v", false, "verbose output")
	fs.BoolVar(&cli.verbose, "verbose", false, "verbose output")
	fs.BoolVar(&cli.quiet, "q", false, "minimal output")
	fs.BoolVar(&cli.quiet, "quiet", false, "minimal output")
	fs.StringVar(&cli.saveDir, "s", "", "save logs to directory")
	fs.StringVar(&cli.saveDir, "save", "", "save logs to directory")
	fs.IntVar(&cli.port, "port", 1790, "base port to use")
	fs.StringVar(&cli.server, "server", "", "run server only for test")
	fs.StringVar(&cli.client, "client", "", "run client only for test")
	fs.IntVar(&cli.count, "c", 1, "run each test N times")
	fs.IntVar(&cli.count, "count", 1, "run each test N times")
	fs.BoolVar(&cli.draft, "draft", false, "discover from test/draft/<suite> instead of test/<suite> (tests under development)")

	if err := fs.Parse(args[1:]); err != nil {
		return nil
	}

	cli.testArgs = fs.Args()

	return cli
}

func zeTestPrintRunUsage() {
	_, _ = os.Stderr.WriteString(`Usage: ze-test bgp <type> [options] [tests...]

Types:
  encode    Run encode tests (static routes)
  plugin    Run plugin tests (dynamic routes via .run scripts)
  decode    Run decode tests (BGP message hex to JSON)
  parse     Run parse tests (config file validation)
  reload    Run reload tests (SIGHUP config reload)
  chaos     Run chaos integration tests (Ze + chaos peers end-to-end)
  chaos-web Run chaos web dashboard tests (HTTP endpoint checks)

Modes:
  -l, --list          List available tests with N/TOTAL and one-based id
  --short-list        List numeric test ids only (space separated)
  -a, --all           Run all tests
  --start ID          Start at test id/name and run through the end
  --pattern TEXT      Run tests whose id, name, or path contains TEXT
Options:
  -t, --timeout N     Timeout per test (default: 15s)
  -p, --parallel N    Max concurrent tests (0 = all, default: 20)
  -v, --verbose       Show output for each test
  -q, --quiet         Minimal output
  -s, --save DIR      Save logs to directory
  --port N            Base port to use (default: 1790)
  -c, --count N       Run each test N times (stress testing)

Debugging:
  --server ID         Run server only for test
  --client ID         Run client only for test

Examples:
  ze-test bgp encode -l
  ze-test bgp encode -a
  ze-test bgp encode 1 2 3
  ze-test bgp encode --start 42
  ze-test bgp plugin -a -q
  ze-test bgp decode -a
  ze-test bgp parse -a
  ze-test bgp encode -c 10 1 2    # stress test: run tests 1,2 ten times
`)
}
