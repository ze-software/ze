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
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/test/runner"
)

// errTestsFailed is returned when tests fail (not an error, but indicates exit code 1).
var errTestsFailed = errors.New("tests failed")

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
	case "encode", "plugin":
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
		testDir = filepath.Join(baseDir, "test/decode")
	case "parse":
		testDir = filepath.Join(baseDir, "test/parse")
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

	// Build ze
	zePath, err := buildZe(ctx, baseDir)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(zePath) }()

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
	colors := runner.NewColors()
	runner.ResetNickCounter()

	// Discover tests first (needed for --server/--client modes)
	tests := runner.NewEncodingTests(baseDir)
	testDir := filepath.Join(baseDir, "test/encode")
	if cli.command == "plugin" {
		testDir = filepath.Join(baseDir, "test/plugin")
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

	// Check ulimit (only for normal test runs)
	limitCheck, err := runner.CheckUlimit(cli.parallel)
	if err != nil {
		return fmt.Errorf("ulimit check: %w", err)
	}
	if limitCheck.Raised && !cli.quiet {
		fmt.Printf("%s raised to %d\n", colors.Yellow("ulimit:"), limitCheck.RaisedTo)
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
	pr, shifted, err := runner.AllocatePorts(cli.port, tests.Count())
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

	// Print summary
	display := runner.NewDisplay(tests.Tests, colors)
	display.SetQuiet(cli.quiet)

	var success bool
	if cli.count > 1 {
		// Stress test mode
		result := r.RunWithCount(ctx, opts, cli.count)
		success = result.AllPassed
		display.StressSummary(result, cli.count)
	} else {
		// Normal mode
		success = r.Run(ctx, opts)
		display.Summary()
	}

	if !success {
		return errTestsFailed
	}

	return nil
}

// runServerOnly runs only the ze-peer (server) for manual debugging.
func runServerOnly(ctx context.Context, cli *runCLIFlags, tests *runner.EncodingTests, baseDir string) error {
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

	// Build ze-peer
	tmpDir, err := os.MkdirTemp("", "ze-functional-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	peerPath := filepath.Join(tmpDir, "ze-peer")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", peerPath, "./cmd/ze-peer") //nolint:gosec // test runner, paths from temp dir
	cmd.Dir = baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build ze-peer: %w: %s", err, output)
	}

	// Write expects to temp file
	expectFile := filepath.Join(tmpDir, "expects.msg")
	f, err := os.Create(expectFile) //nolint:gosec // test runner, path from temp dir
	if err != nil {
		return fmt.Errorf("create expect file: %w", err)
	}
	for _, opt := range rec.Options {
		_, _ = fmt.Fprintln(f, opt)
	}
	for _, exp := range rec.Expects {
		_, _ = fmt.Fprintln(f, exp)
	}
	_ = f.Close()

	// Print info
	port := cli.port
	if rec.Port != 0 {
		port = rec.Port
	}
	fmt.Printf("Server mode for test: %s (%s)\n", rec.Nick, rec.Name)
	fmt.Printf("Config: %s\n", rec.ConfigFile)
	fmt.Printf("Port: %d\n", port)
	fmt.Printf("Waiting for client connection...\n")
	fmt.Printf("\nRun client in another terminal:\n")
	fmt.Printf("   ze-test bgp %s --client %s --port %d\n\n", cli.command, cli.server, port)

	// Build peer args
	peerArgs := []string{"--port", fmt.Sprintf("%d", port)}
	if asn, ok := rec.Extra["asn"]; ok {
		peerArgs = append(peerArgs, "--asn", asn)
	}
	if rec.Extra["bind"] == "ipv6" {
		peerArgs = append(peerArgs, "--ipv6")
	}
	peerArgs = append(peerArgs, expectFile)

	// Run peer (blocks until client connects and test completes or Ctrl+C)
	peerCmd := exec.CommandContext(ctx, peerPath, peerArgs...) //nolint:gosec // test runner, paths from temp dir
	peerCmd.Env = append(os.Environ(), fmt.Sprintf("ze_bgp_tcp_port=%d", port))
	peerCmd.Stdout = os.Stdout
	peerCmd.Stderr = os.Stderr

	return peerCmd.Run()
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
	defer func() { _ = os.Remove(zePath) }()

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

	// Build client env
	// Set tcp.attempts=1 so ze bgp exits after the session ends (instead of reconnecting)
	zeDir := filepath.Dir(zePath)
	existingPath := os.Getenv("PATH")
	clientEnv := append(os.Environ(),
		fmt.Sprintf("ze_bgp_tcp_port=%d", port),
		fmt.Sprintf("ze_bgp_api_socketpath=%s", filepath.Join(os.TempDir(), fmt.Sprintf("ze-debug-%d.sock", port))),
		fmt.Sprintf("PATH=%s:%s", zeDir, existingPath),
		"ze_bgp_tcp_attempts=1", // Exit after first session ends
	)

	// Run ze bgp (blocks until stopped)
	clientCmd := exec.CommandContext(ctx, zePath, "server", configPath) //nolint:gosec // test runner, paths from temp dir
	clientCmd.Env = clientEnv
	clientCmd.Stdout = os.Stdout
	clientCmd.Stderr = os.Stderr

	return clientCmd.Run()
}

// buildZe builds the ze binary and returns its path.
func buildZe(ctx context.Context, baseDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "ze-functional-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	zePath := filepath.Join(tmpDir, "ze")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", zePath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
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
	if command == "-h" || command == "--help" || command == "help" {
		printRunUsage()
		return nil
	}

	validCommands := map[string]bool{
		"encode": true,
		"plugin": true,
		"decode": true,
		"parse":  true,
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
	fs.BoolVar(&cli.shortList, "short-list", false, "list test codes only")
	fs.DurationVar(&cli.timeout, "t", 15*time.Second, "timeout per test")
	fs.DurationVar(&cli.timeout, "timeout", 15*time.Second, "timeout per test")
	fs.IntVar(&cli.parallel, "p", 0, "max concurrent tests (0 = all)")
	fs.IntVar(&cli.parallel, "parallel", 0, "max concurrent tests (0 = all)")
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
  plugin      Run plugin tests (dynamic routes via .run scripts)
  decode    Run decode tests (BGP message hex to JSON)
  parse     Run parse tests (config file validation)

Modes:
  -l, --list          List available tests
  --short-list        List test codes only (space separated)
  -a, --all           Run all tests

Options:
  -t, --timeout N     Timeout per test (default: 15s)
  -p, --parallel N    Max concurrent tests (0 = all, default: 0)
  -v, --verbose       Show output for each test
  -q, --quiet         Minimal output
  -s, --save DIR      Save logs to directory
  --port N            Base port to use (default: 1790)
  -c, --count N       Run each test N times (stress testing)

Debugging:
  --server NICK       Run server only for test
  --client NICK       Run client only for test

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
