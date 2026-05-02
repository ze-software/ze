// Design: docs/architecture/testing/ci-format.md — test runner CLI
// Related: vpp.go -- shared test-runner infrastructure and CLI parser

package main

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

	"codeberg.org/thomas-mangin/ze/internal/test/peer"
	"codeberg.org/thomas-mangin/ze/internal/test/runner"
)

// errTestsFailed is returned when tests fail (not an error, but indicates exit code 1).
var errTestsFailed = errors.New("tests failed")

// Command name constants for test suites.
const (
	cmdPlugin   = "plugin"
	cmdChaosWeb = "chaos-web"
)

var _ = register("bgp", "Run BGP functional tests (encoding, plugin, decoding, parsing)", bgpCmd)

func bgpCmd() int {
	if err := bgpMain(); err != nil {
		if !errors.Is(err, errTestsFailed) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return 1
	}
	return 0
}

func bgpMain() error {
	// Parse command line
	cli := parseRunCLI()
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

	// Route to appropriate handler
	switch cli.command {
	case "encode", cmdPlugin, "reload", cmdChaosWeb:
		return runEncodingOrAPI(ctx, cli, baseDir)
	case "decode":
		return runSimpleTests(ctx, cli, baseDir, newDecodingTestSuite)
	case "parse":
		return runSimpleTests(ctx, cli, baseDir, newParsingTestSuite)
	default:
		return fmt.Errorf("unknown command: %s", cli.command)
	}
}

// testSuite abstracts decode and parse test suites.
type testSuite interface {
	Discover(dir string) error
	List()
	Count() int
	EnableAll()
	EnableByNick(nick string) bool
	GetNicks() []string
	GetNames() map[string]string
	SetActive(name string)
	Run(ctx context.Context, zePath string, verbose, quiet bool) bool
}

// decodeTestSuite wraps DecodingTests to implement testSuite.
type decodeTestSuite struct {
	*runner.DecodingTests
	baseDir string
}

func newDecodingTestSuite(baseDir string) testSuite {
	return &decodeTestSuite{
		DecodingTests: runner.NewDecodingTests(baseDir),
		baseDir:       baseDir,
	}
}

func (d *decodeTestSuite) GetNicks() []string {
	registered := d.Registered()
	nicks := make([]string, 0, len(registered))
	for _, t := range registered {
		nicks = append(nicks, t.Nick)
	}
	return nicks
}

func (d *decodeTestSuite) GetNames() map[string]string {
	names := make(map[string]string)
	for _, t := range d.Registered() {
		names[t.Name] = t.Nick
	}
	return names
}

func (d *decodeTestSuite) SetActive(name string) {
	for _, t := range d.Registered() {
		if t.Name == name {
			t.Active = true
		}
	}
}

func (d *decodeTestSuite) Run(ctx context.Context, zePath string, verbose, quiet bool) bool {
	runner := runner.NewDecodingRunner(d.DecodingTests, d.baseDir, zePath)
	return runner.Run(ctx, verbose, quiet)
}

// parseTestSuite wraps ParsingTests to implement testSuite.
type parseTestSuite struct {
	*runner.ParsingTests
	baseDir string
}

func newParsingTestSuite(baseDir string) testSuite {
	return &parseTestSuite{
		ParsingTests: runner.NewParsingTests(baseDir),
		baseDir:      baseDir,
	}
}

func (p *parseTestSuite) GetNicks() []string {
	registered := p.Registered()
	nicks := make([]string, 0, len(registered))
	for _, t := range registered {
		nicks = append(nicks, t.Nick)
	}
	return nicks
}

func (p *parseTestSuite) GetNames() map[string]string {
	names := make(map[string]string)
	for _, t := range p.Registered() {
		names[t.Name] = t.Nick
	}
	return names
}

func (p *parseTestSuite) SetActive(name string) {
	for _, t := range p.Registered() {
		if t.Name == name {
			t.Active = true
		}
	}
}

func (p *parseTestSuite) Run(ctx context.Context, zePath string, verbose, quiet bool) bool {
	runner := runner.NewParsingRunner(p.ParsingTests, p.baseDir, zePath)
	return runner.Run(ctx, verbose, quiet)
}

// runSimpleTests handles decode and parse tests using the testSuite interface.
func runSimpleTests(ctx context.Context, cli *runCLIFlags, baseDir string, newSuite func(string) testSuite) error {
	runner.ResetNickCounter()

	tests := newSuite(baseDir)

	// Determine test directory
	var testDir string
	switch cli.command {
	case "decode":
		testDir = filepath.Join(baseDir, "test", "decode")
	case "parse":
		testDir = filepath.Join(baseDir, "test", "parse")
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
		for _, nick := range tests.GetNicks() {
			fmt.Printf("%s ", nick)
		}
		fmt.Println()
		return nil
	}

	// Select tests
	switch {
	case cli.all:
		tests.EnableAll()
	case len(cli.testArgs) > 0:
		names := tests.GetNames()
		for _, arg := range cli.testArgs {
			if !tests.EnableByNick(arg) {
				// Try by name
				if _, ok := names[arg]; ok {
					tests.SetActive(arg)
				}
			}
		}
	default:
		printRunUsage()
		return nil
	}

	// Section header
	if !cli.quiet {
		runner.PrintHeader(cli.command)
	}

	// Build ze
	zePath, err := buildZe(ctx, baseDir)
	if err != nil {
		return err
	}

	// Run tests
	success := tests.Run(ctx, zePath, cli.verbose, cli.quiet)

	if !success {
		return errTestsFailed
	}

	return nil
}

// runEncodingOrAPI handles encode and API tests (original behavior).
func runEncodingOrAPI(ctx context.Context, cli *runCLIFlags, baseDir string) error {
	// Initialize
	runner.ResetNickCounter()

	// Discover tests first (needed for --server/--client modes)
	tests := runner.NewEncodingTests(baseDir)
	testDir := filepath.Join(baseDir, "test", "encode")
	switch cli.command {
	case cmdPlugin:
		testDir = filepath.Join(baseDir, "test", "plugin")
	case "reload":
		testDir = filepath.Join(baseDir, "test", "reload")
	case cmdChaosWeb:
		testDir = filepath.Join(baseDir, "test", "chaos-web")
	}

	if err := tests.Discover(testDir); err != nil {
		return fmt.Errorf("discover tests: %w", err)
	}

	// Handle --server or --client debug modes
	if cli.server != "" {
		return runServerOnly(ctx, cli, tests, baseDir)
	}
	if cli.client != "" {
		return runClientOnly(ctx, cli, tests, baseDir)
	}

	// Handle list mode (before any output)
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
		printRunUsage()
		return nil
	}

	tests.Sort()

	// Create runner
	r, err := runner.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer r.Cleanup()

	// Extra binaries needed by specific test suites.
	if cli.command == cmdChaosWeb {
		r.SetExtraBinaries(map[string]string{
			"ze-chaos": "./cmd/ze-chaos",
		})
	}

	// Section header first, then all info within it
	r.Display().SetLabel(cli.command)
	r.Report().SetLabel(cli.command)
	r.Display().Header()

	// Check ulimit
	limitCheck, err := runner.CheckUlimit(cli.parallel)
	if err != nil {
		return fmt.Errorf("ulimit check: %w", err)
	}
	r.Display().UlimitInfo(limitCheck)

	// Allocate ports
	// Reserve 2 ports per test: $PORT for the main process and $PORT2 ($PORT+1)
	// for tests that need a second port (e.g., RPKI cache mock).
	portReservation, shifted, err := runner.ReservePorts(cli.port, tests.Count()*2)
	if err != nil {
		return fmt.Errorf("allocate ports: %w", err)
	}
	defer portReservation.Release()
	pr := portReservation.PortRange

	// Update test ports based on allocation, spacing by 2 to avoid overlap.
	basePort := pr.Start
	for _, rr := range tests.Registered() {
		rr.Port = basePort
		basePort += 2
	}

	r.Display().PortInfo(pr, shifted)

	// Build
	if err := r.Build(ctx); err != nil {
		return err
	}

	// Run options
	opts := &runner.RunOptions{
		Timeout:  cli.timeout,
		Parallel: cli.parallel,
		Verbose:  cli.verbose,
		Quiet:    cli.quiet,
		SaveDir:  cli.saveDir,
	}

	// Run and print summary via runner's display (which tracks start time)
	var success bool
	if cli.count > 1 {
		// Stress test mode
		result := r.RunWithCount(ctx, opts, cli.count)
		success = result.AllPassed
		r.Display().StressSummary(result, cli.count)
	} else {
		// Normal mode
		success = r.Run(ctx, opts)
		r.Display().Summary()
		r.Display().TimingDetail(cli.command, r.Timings())
		r.Display().DebugHints()
	}

	if !success {
		return errTestsFailed
	}

	return nil
}

// runServerOnly runs only the peer (server) for manual debugging.
// Uses the peer library directly instead of subprocess for faster startup.
func runServerOnly(ctx context.Context, cli *runCLIFlags, tests *runner.EncodingTests, _ string) error {
	rec := tests.GetByNick(cli.server)
	if rec == nil {
		// Try by name
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

	// Build expect rules from test record
	expects := make([]string, 0, len(rec.Options)+len(rec.Expects))
	expects = append(expects, rec.Options...)
	expects = append(expects, rec.Expects...)

	// Determine port
	port := cli.port
	if rec.Port != 0 {
		port = rec.Port
	}

	// Build peer config
	config := &peer.Config{
		Port:   port,
		Mode:   peer.ModeCheck,
		Expect: expects,
		Output: os.Stdout,
	}

	// Apply extra options from test record
	if asn, ok := rec.Extra["asn"]; ok {
		if v, err := strconv.Atoi(asn); err == nil {
			config.ASN = v
		}
	}
	if rec.Extra["bind"] == "ipv6" {
		config.IPv6 = true
	}

	// Print info
	fmt.Printf("Server mode for test: %s (%s)\n", rec.Nick, rec.Name)
	fmt.Printf("Config: %s\n", rec.ConfigFile)
	fmt.Printf("Port: %d\n", port)
	fmt.Printf("Waiting for client connection...\n")
	fmt.Printf("\nRun client in another terminal:\n")
	fmt.Printf("   ze-test bgp %s --client %s --port %d\n\n", cli.command, cli.server, port)

	// Run peer directly (no subprocess needed)
	p, err := peer.New(config)
	if err != nil {
		return fmt.Errorf("create peer: %w", err)
	}
	result := p.Run(ctx)

	fmt.Println()

	if result.Error != nil {
		return result.Error
	}
	if !result.Success {
		return fmt.Errorf("peer check failed")
	}

	fmt.Println("successful")
	return nil
}

// runClientOnly runs only ze bgp (client) for manual debugging.
func runClientOnly(ctx context.Context, cli *runCLIFlags, tests *runner.EncodingTests, baseDir string) error {
	rec := tests.GetByNick(cli.client)
	if rec == nil {
		// Try by name
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

	// Build ze
	zePath, err := buildZe(ctx, baseDir)
	if err != nil {
		return err
	}

	// Print info
	port := cli.port
	if rec.Port != 0 {
		port = rec.Port
	}
	fmt.Printf("Client mode for test: %s (%s)\n", rec.Nick, rec.Name)
	fmt.Printf("Config: %s\n", configPath)
	fmt.Printf("Port: %d\n", port)
	fmt.Printf("Starting ze bgp client...\n")
	fmt.Printf("\nServer should be running. If not:\n")
	fmt.Printf("   ze-test bgp %s --server %s --port %d\n\n", cli.command, cli.client, port)

	// Build client env. Press Ctrl+C when the peer has finished validating;
	// the client sends SIGTERM to ze so it shuts down gracefully.
	zeDir := filepath.Dir(zePath)
	existingPath := os.Getenv("PATH")
	clientEnv := append(os.Environ(),
		fmt.Sprintf("ze_test_bgp_port=%d", port),
		fmt.Sprintf("PATH=%s:%s", zeDir, existingPath),
	)

	// Run ze bgp (blocks until stopped).
	// Override CommandContext's default Kill with SIGTERM so ze shuts down
	// cleanly when the user Ctrl+Cs (canceling ctx).
	//
	// exec doc: Cancel is invoked only after Start succeeds, so Process is
	// guaranteed non-nil at call time. Guard defensively in case that changes.
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

// buildZe builds the ze binary into bin/ and returns its path.
// Uses the project's bin/ directory so DefaultConfigDir() resolves correctly
// (binary in bin/ → config in etc/ze via GNU prefix conventions).
func buildZe(ctx context.Context, baseDir string) (string, error) {
	zePath := filepath.Join(baseDir, "bin", "ze")
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", runner.TestBuildTags(), "-o", zePath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build ze: %w: %s", err, output)
	}

	return zePath, nil
}

type runCLIFlags struct {
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

func parseRunCLI() *runCLIFlags {
	if len(os.Args) < 2 {
		printRunUsage()
		return nil
	}

	command := os.Args[1]
	if isHelpArg(command) {
		printRunUsage()
		return nil
	}

	validCommands := map[string]bool{
		"encode":    true,
		cmdPlugin:   true,
		"decode":    true,
		"parse":     true,
		"reload":    true,
		cmdChaosWeb: true,
	}

	if !validCommands[command] {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printRunUsage()
		return nil
	}

	cli := &runCLIFlags{command: command}

	fs := flag.NewFlagSet(command, flag.ExitOnError)
	fs.BoolVar(&cli.all, "a", false, "run all tests")
	fs.BoolVar(&cli.all, "all", false, "run all tests")
	fs.BoolVar(&cli.list, "l", false, "list available tests")
	fs.BoolVar(&cli.list, "list", false, "list available tests")
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

	if err := fs.Parse(os.Args[2:]); err != nil {
		return nil
	}

	cli.testArgs = fs.Args()

	return cli
}

func printRunUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze-test bgp <type> [options] [tests...]

Types:
  encode    Run encode tests (static routes)
  plugin    Run plugin tests (dynamic routes via .run scripts)
  decode    Run decode tests (BGP message hex to JSON)
  parse     Run parse tests (config file validation)
  reload    Run reload tests (SIGHUP config reload)
  chaos-web Run chaos web dashboard tests (HTTP endpoint checks)

Modes:
  -l, --list          List available tests
  --short-list        List numeric test ids only (space separated)
  -a, --all           Run all tests

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
  ze-test bgp encode 0 1 2
  ze-test bgp plugin -a -q
  ze-test bgp decode -a
  ze-test bgp parse -a
  ze-test bgp encode -c 10 0 1    # stress test: run tests 0,1 ten times
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
