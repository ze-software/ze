// Package main provides the functional test runner with AI-friendly diagnostics.
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

	"codeberg.org/thomas-mangin/zebgp/test/functional"
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

	// Route to appropriate handler
	switch cli.command {
	case "encoding", "api":
		return runEncodingOrAPI(ctx, cli, baseDir)
	case "decoding":
		return runSimpleTests(ctx, cli, baseDir, newDecodingTestSuite)
	case "parsing":
		return runSimpleTests(ctx, cli, baseDir, newParsingTestSuite)
	default:
		return fmt.Errorf("unknown command: %s", cli.command)
	}
}

// testSuite abstracts decoding and parsing test suites.
type testSuite interface {
	Discover(dir string) error
	List()
	Count() int
	EnableAll()
	EnableByNick(nick string) bool
	GetNicks() []string
	GetNames() map[string]string
	SetActive(name string)
	Run(ctx context.Context, zebgpPath string, verbose, quiet bool) bool
}

// decodingTestSuite wraps DecodingTests to implement testSuite.
type decodingTestSuite struct {
	*functional.DecodingTests
	baseDir string
}

func newDecodingTestSuite(baseDir string) testSuite {
	return &decodingTestSuite{
		DecodingTests: functional.NewDecodingTests(baseDir),
		baseDir:       baseDir,
	}
}

func (d *decodingTestSuite) GetNicks() []string {
	registered := d.Registered()
	nicks := make([]string, 0, len(registered))
	for _, t := range registered {
		nicks = append(nicks, t.Nick)
	}
	return nicks
}

func (d *decodingTestSuite) GetNames() map[string]string {
	names := make(map[string]string)
	for _, t := range d.Registered() {
		names[t.Name] = t.Nick
	}
	return names
}

func (d *decodingTestSuite) SetActive(name string) {
	for _, t := range d.Registered() {
		if t.Name == name {
			t.Active = true
		}
	}
}

func (d *decodingTestSuite) Run(ctx context.Context, zebgpPath string, verbose, quiet bool) bool {
	runner := functional.NewDecodingRunner(d.DecodingTests, d.baseDir, zebgpPath)
	return runner.Run(ctx, verbose, quiet)
}

// parsingTestSuite wraps ParsingTests to implement testSuite.
type parsingTestSuite struct {
	*functional.ParsingTests
	baseDir string
}

func newParsingTestSuite(baseDir string) testSuite {
	return &parsingTestSuite{
		ParsingTests: functional.NewParsingTests(baseDir),
		baseDir:      baseDir,
	}
}

func (p *parsingTestSuite) GetNicks() []string {
	registered := p.Registered()
	nicks := make([]string, 0, len(registered))
	for _, t := range registered {
		nicks = append(nicks, t.Nick)
	}
	return nicks
}

func (p *parsingTestSuite) GetNames() map[string]string {
	names := make(map[string]string)
	for _, t := range p.Registered() {
		names[t.Name] = t.Nick
	}
	return names
}

func (p *parsingTestSuite) SetActive(name string) {
	for _, t := range p.Registered() {
		if t.Name == name {
			t.Active = true
		}
	}
}

func (p *parsingTestSuite) Run(ctx context.Context, zebgpPath string, verbose, quiet bool) bool {
	runner := functional.NewParsingRunner(p.ParsingTests, p.baseDir, zebgpPath)
	return runner.Run(ctx, verbose, quiet)
}

// runSimpleTests handles decoding and parsing tests using the testSuite interface.
func runSimpleTests(ctx context.Context, cli *cliFlags, baseDir string, newSuite func(string) testSuite) error {
	functional.ResetNickCounter()

	tests := newSuite(baseDir)

	// Determine test directory
	var testDir string
	switch cli.command {
	case "decoding":
		testDir = filepath.Join(baseDir, "test/data/decode")
	case "parsing":
		testDir = filepath.Join(baseDir, "test/data/parse")
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
		printUsage()
		return nil
	}

	// Build zebgp
	zebgpPath, err := buildZebgp(ctx, baseDir)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(zebgpPath) }()

	// Run tests
	success := tests.Run(ctx, zebgpPath, cli.verbose, cli.quiet)

	if !success {
		return errTestsFailed
	}

	return nil
}

// runEncodingOrAPI handles encoding and API tests (original behavior).
func runEncodingOrAPI(ctx context.Context, cli *cliFlags, baseDir string) error {
	// Initialize
	colors := functional.NewColors()
	functional.ResetNickCounter()

	// Check ulimit
	limitCheck, err := functional.CheckUlimit(cli.parallel)
	if err != nil {
		return fmt.Errorf("ulimit check: %w", err)
	}
	if limitCheck.Raised && !cli.quiet {
		fmt.Printf("%s raised to %d\n", colors.Yellow("ulimit:"), limitCheck.RaisedTo)
	}

	// Discover tests
	tests := functional.NewEncodingTests(baseDir)
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
	pr, shifted, err := functional.AllocatePorts(cli.port, tests.Count())
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
	runner, err := functional.NewRunner(tests, baseDir)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}
	defer runner.Cleanup()

	// Build
	if err := runner.Build(ctx); err != nil {
		return err
	}

	// Run options
	opts := &functional.RunOptions{
		Timeout:  cli.timeout,
		Parallel: cli.parallel,
		Verbose:  cli.verbose,
		Quiet:    cli.quiet,
		SaveDir:  cli.saveDir,
	}

	// Print summary
	display := functional.NewDisplay(tests.Tests, colors)
	display.SetQuiet(cli.quiet)

	var success bool
	if cli.count > 1 {
		// Stress test mode
		result := runner.RunWithCount(ctx, opts, cli.count)
		success = result.AllPassed
		display.StressSummary(result, cli.count)
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

// buildZebgp builds the zebgp binary and returns its path.
func buildZebgp(ctx context.Context, baseDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "zebgp-functional-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	zebgpPath := filepath.Join(tmpDir, "zebgp")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", zebgpPath, "./cmd/zebgp") //nolint:gosec // paths from internal runner
	cmd.Dir = baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", fmt.Errorf("build zebgp: %w: %s", err, output)
	}

	return zebgpPath, nil
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

	validCommands := map[string]bool{
		"encoding": true,
		"api":      true,
		"decoding": true,
		"parsing":  true,
	}

	if !validCommands[command] {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		return nil
	}

	cli := &cliFlags{command: command}

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

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: functional <command> [options] [tests...]

Commands:
  encoding    Run encoding tests (static routes)
  api         Run API tests (dynamic routes via .run scripts)
  decoding    Run decoding tests (BGP message hex to JSON)
  parsing     Run parsing tests (config file validation)

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
  functional encoding -l
  functional encoding -a
  functional encoding 0 1 2
  functional api -a -q
  functional decoding -a
  functional parsing -a
  functional encoding -c 10 0 1    # stress test: run tests 0,1 ten times
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
