// Design: docs/architecture/testing/ci-format.md — predecessor encoding test runner

package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/test/runner"
)

const exabgpSuiteEncoding = "encoding"
const exabgpSuiteAPI = "api"

// exabgpSuites are the predecessor test populations, each a subdirectory of
// test/exabgp-compat holding .ci fixtures. encoding drives ze from a migrated
// config alone; api drives it through the ExaBGP bridge, which runs the script
// the config's process block names.
var exabgpSuites = []string{exabgpSuiteEncoding, exabgpSuiteAPI}

const predecessorPrefix = "exabgp-"
const predecessorTestDir = predecessorPrefix + "compat"

var (
	errExaBGPPortTimeout = errors.New("timed out waiting for mock BGP port")
	errExaBGPMissingPort = errors.New("mock BGP server did not report a port")
)

type exabgpCLI struct {
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
	server    string
	client    string
	port      int
	testArgs  []string
	zeBinary  string
}

type exabgpTestEntry struct {
	record         *runner.Record
	configs        []string
	ciFile         string
	tcpConnections int
	serial         bool
}

type exabgpSuite struct {
	baseDir string
	rootDir string
	tests   *runner.Tests
	byNick  map[string]*exabgpTestEntry
}

func cmdExabgp(args []string) int {
	if err := zeTestExabgpMain(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func zeTestExabgpMain(args []string) error {
	suiteName := exabgpSuiteEncoding
	if len(args) > 0 && args[0] == exabgpSuiteEncoding {
		args = args[1:]
	}
	if len(args) > 0 && args[0] != "" && args[0][0] != '-' && isKnownExaBGPSuite(args[0]) {
		suiteName = args[0]
		args = args[1:]
	}
	if len(args) > 0 && isHelpArg(args[0]) {
		printExaBGPUsage()
		return nil
	}

	cli, err := parseExaBGPCLI(args)
	if err != nil {
		return err
	}

	baseDir, err := FindBaseDir()
	if err != nil {
		return fmt.Errorf("find base dir: %w", err)
	}

	suite, err := discoverExaBGPSuite(baseDir, suiteName)
	if err != nil {
		return err
	}
	if suite.tests.Count() == 0 {
		return errors.New("no predecessor encoding tests found")
	}
	suite.tests.Sort()

	if cli.list {
		suite.tests.List()
		return nil
	}
	if cli.shortList {
		printShortExaBGPList(suite.tests)
		return nil
	}

	if cli.server != "" {
		test, ok := suite.byNick[cli.server]
		if !ok {
			var tb textbuf.Buffer
			return errors.New(tb.Str("no such ExaBGP test: ").Str(cli.server).String())
		}
		return runExaBGPServerForeground(test, cli.port, cli.saveDir)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Every path below spawns the compiled ExaBGP wrapper, which drives a ze
	// daemon. The wrapper receives the DUT explicitly: it must not guess whether
	// ZE_BIN is set, where this session's bin/ is, or which build tags a binary
	// carries. Resolve it HERE through the same resolver as every other suite
	// (TestSuiteRunnersResolveDUTThroughBuildZe), then pass it down.
	//
	// It matters more here than elsewhere: `ze exabgp migrate` exists only under
	// the ze_exabgp tag (cmd/ze/dispatch_exabgp.go, feature-gates.txt), which
	// runner.TestBuildTags supplies, so a wrapper left to guess does not degrade
	// -- it fails all 42 tests with "unknown command: exabgp" and names no cause.
	zeBinary, err := buildZe(ctx, baseDir)
	if err != nil {
		return err
	}
	cli.zeBinary = zeBinary

	if cli.client != "" {
		test, ok := suite.byNick[cli.client]
		if !ok {
			var tb textbuf.Buffer
			return errors.New(tb.Str("no such ExaBGP test: ").Str(cli.client).String())
		}
		if cli.port <= 0 {
			return errors.New("--client requires --port")
		}
		return runExaBGPClientForeground(test, cli.port, cli.zeBinary)
	}

	selected, err := suite.tests.Select(runner.Selection{
		All:     cli.all,
		Start:   cli.start,
		Pattern: cli.pattern,
		Args:    cli.testArgs,
	})
	if err != nil {
		return err
	}
	if selected == 0 {
		printExaBGPUsage()
		return nil
	}

	success := runExaBGPSelected(ctx, suite, cli)
	if !success {
		return ErrTestsFailed
	}
	return nil
}

func isKnownExaBGPSuite(name string) bool {
	return slices.Contains(exabgpSuites, name)
}

func parseExaBGPCLI(args []string) (exabgpCLI, error) {
	var cli exabgpCLI
	fs := flag.NewFlagSet("ze-test exabgp", flag.ExitOnError)
	fs.BoolVar(&cli.all, "a", false, "run all tests")
	fs.BoolVar(&cli.all, "all", false, "run all tests")
	fs.BoolVar(&cli.list, "l", false, "list available tests")
	fs.BoolVar(&cli.list, "list", false, "list available tests")
	fs.StringVar(&cli.start, "start", "", "start at test id/name and run through the end")
	fs.StringVar(&cli.pattern, "pattern", "", "run tests whose id, name, or path contains pattern")
	fs.BoolVar(&cli.shortList, "short-list", false, "list numeric test ids only")
	fs.DurationVar(&cli.timeout, "t", 180*time.Second, "timeout per test")
	fs.DurationVar(&cli.timeout, "timeout", 180*time.Second, "timeout per test")
	fs.IntVar(&cli.parallel, "p", runner.DefaultParallelConcurrent, "max concurrent tests (0 = all)")
	fs.IntVar(&cli.parallel, "parallel", runner.DefaultParallelConcurrent, "max concurrent tests (0 = all)")
	fs.BoolVar(&cli.verbose, "v", false, "verbose output")
	fs.BoolVar(&cli.verbose, "verbose", false, "verbose output")
	fs.BoolVar(&cli.quiet, "q", false, "minimal output")
	fs.BoolVar(&cli.quiet, "quiet", false, "minimal output")
	fs.StringVar(&cli.saveDir, "s", "", "save BGP mock logs under directory")
	fs.StringVar(&cli.saveDir, "save", "", "save BGP mock logs under directory")
	fs.StringVar(&cli.server, "server", "", "start the mock BGP server for one test id")
	fs.StringVar(&cli.client, "client", "", "start the ExaBGP wrapper client for one test id")
	fs.IntVar(&cli.port, "port", 0, "port for --server or --client")
	fs.Usage = printExaBGPUsage

	if err := fs.Parse(args); err != nil {
		return cli, err
	}
	cli.testArgs = fs.Args()
	return cli, nil
}

func printExaBGPUsage() {
	_, _ = os.Stderr.WriteString(`Usage: ze-test exabgp [encoding] [options] [test-ids...]

Run predecessor encoding tests using Ze's standard test selection and progress output.

Modes:
  -l, --list          List available tests with N/TOTAL and one-based id
  --short-list        List numeric test ids only (space separated)
  -a, --all           Run all tests
  --start ID          Start at test id/name and run through the end
  --pattern TEXT      Run tests whose id, name, or path contains TEXT

Options:
  -t, --timeout N     Timeout per test (default: 180s)
  -p, --parallel N    Max concurrent tests (0 = all, default: 20)
  -v, --verbose       Show process output for each test
  -q, --quiet         Minimal output
  -s, --save DIR      Save BGP mock logs under DIR
  --server ID         Start mock BGP server for one test id
  --client ID         Start ExaBGP wrapper client for one test id
  --port N            Port for --server or --client

Examples:
  ze-test exabgp --list
  ze-test exabgp --all
  ze-test exabgp --start 20
  ze-test exabgp 1 2 3
  ze-test exabgp --server 1 --port 17900
  ze-test exabgp --client 1 --port 17900
`)
}

func discoverExaBGPSuite(baseDir, suiteName string) (*exabgpSuite, error) {
	root := filepath.Join(baseDir, "test", predecessorTestDir)
	pattern := filepath.Join(root, suiteName, "*.ci")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		left := strings.TrimSuffix(filepath.Base(files[i]), filepath.Ext(files[i]))
		right := strings.TrimSuffix(filepath.Base(files[j]), filepath.Ext(files[j]))
		return left < right
	})

	runner.ResetNickCounter()
	tests := runner.NewTests()
	suite := &exabgpSuite{
		baseDir: baseDir,
		rootDir: root,
		tests:   tests,
		byNick:  make(map[string]*exabgpTestEntry, len(files)),
	}

	for _, ciFile := range files {
		name := strings.TrimSuffix(filepath.Base(ciFile), filepath.Ext(ciFile))
		rec := tests.Add(name)
		rec.CIFile = ciFile
		rec.Files = append(rec.Files, ciFile)

		compat, err := parseExaBGPCI(root, rec, ciFile)
		if err != nil {
			return nil, err
		}
		suite.byNick[rec.Nick] = compat
	}
	return suite, nil
}

func parseExaBGPCI(root string, rec *runner.Record, ciFile string) (*exabgpTestEntry, error) {
	file, err := os.Open(ciFile) //nolint:gosec // ciFile comes from the discovered test predecessor fixture set.
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	test := &exabgpTestEntry{
		record:         rec,
		ciFile:         ciFile,
		tcpConnections: 1,
	}

	signals := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// A `<prefix>:signal:<NAME>` step asks this runner to reload ze, and a
		// reload loads the NEXT config the fixture named. Counting them here is
		// what lets the pairing be checked before a daemon starts, rather than
		// leaving a fixture that names too few configs to fail 180 seconds later
		// on a timeout that names no cause.
		if parts := strings.Split(line, ":"); len(parts) >= 3 && parts[1] == "signal" {
			signals++
			continue
		}
		if after, ok := strings.CutPrefix(line, "option=file:"); ok {
			configPath := filepath.Join(root, "etc", after)
			test.configs = append(test.configs, configPath)
			rec.Files = append(rec.Files, configPath)
			continue
		}
		if after, ok := strings.CutPrefix(line, "option=tcp_connections:"); ok {
			// Zero is a real answer, not an absent one: api-peer-lifecycle
			// expects NO connection from ze, because its peer is created from
			// the API rather than declared in the config.
			count, err := strconv.Atoi(after)
			if err != nil || count < 0 {
				var tb textbuf.Buffer
				return nil, errors.New(tb.Str("invalid tcp_connections in ").Str(ciFile).String())
			}
			test.tcpConnections = count
		}
		if line == "option=serial" {
			test.serial = true
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(test.configs) == 0 {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("predecessor encoding test has no option=file: ").Str(ciFile).String())
	}
	if len(test.configs) != signals+1 {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str(ciFile).
			Str(" names ").Int(int64(len(test.configs))).Str(" configs and ").
			Int(int64(signals)).Str(" signal steps: the first config is the one ze starts on, ").
			Str("and each signal reloads the next one, so a case owes one config more than it has signals").String())
	}
	return test, nil
}

func printShortExaBGPList(tests *runner.Tests) {
	registered := tests.Registered()
	for i, rec := range registered {
		if i > 0 {
			fmt.Print(" ") //nolint:forbidigo // CLI output
		}
		fmt.Print(rec.Nick) //nolint:forbidigo // CLI output
	}
	fmt.Println() //nolint:forbidigo // CLI output
}

func runExaBGPSelected(ctx context.Context, suite *exabgpSuite, cli exabgpCLI) bool {
	selected := suite.tests.Selected()
	if len(selected) == 0 {
		return true
	}

	parallel := cli.parallel
	if parallel <= 0 || parallel > len(selected) {
		parallel = len(selected)
	}

	display := runner.NewDisplay(suite.tests, runner.NewColors())
	display.SetLabel("exabgp encoding")
	display.SetQuiet(cli.quiet)
	display.SetTimeout(cli.timeout)
	display.SetParallel(parallel, len(selected))
	display.Header()
	display.Start()

	statusDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-statusDone:
				return
			case <-ticker.C:
				display.Status()
			}
		}
	}()

	var parallelTests []*exabgpTestEntry
	var serialTests []*exabgpTestEntry
	allOK := true
	for _, rec := range selected {
		test := suite.byNick[rec.Nick]
		if test == nil {
			rec.State = runner.StateFail
			rec.Error = errors.New("missing ExaBGP test metadata")
			allOK = false
			continue
		}
		if test.serial {
			serialTests = append(serialTests, test)
			continue
		}
		parallelTests = append(parallelTests, test)
	}

	var okMu sync.Mutex
	runBatch := func(tests []*exabgpTestEntry, limit int) {
		if len(tests) == 0 {
			return
		}
		if limit <= 0 || limit > len(tests) {
			limit = len(tests)
		}
		sem := make(chan struct{}, limit)
		var wg sync.WaitGroup
		for _, test := range tests {
			wg.Add(1)
			go func(t *exabgpTestEntry) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				passed, detail := runOneExaBGPTest(ctx, t, cli)
				if !passed {
					okMu.Lock()
					allOK = false
					okMu.Unlock()
					if !cli.quiet {
						printExaBGPFailure(t, detail)
					}
				} else if cli.verbose && !cli.quiet {
					printExaBGPOutput(t, detail)
				}
				display.TestFinished(t.record.Nick, t.record.State, t.record.Duration)
			}(test)
		}
		wg.Wait()
	}

	runBatch(parallelTests, parallel)
	runBatch(serialTests, 1)

	close(statusDone)
	display.Newline()
	display.Summary()
	okMu.Lock()
	success := allOK
	okMu.Unlock()
	return success
}

type exabgpRunDetail struct {
	port         int
	serverStdout string
	serverStderr string
	clientStdout string
	clientStderr string
}

func runOneExaBGPTest(ctx context.Context, test *exabgpTestEntry, cli exabgpCLI) (bool, exabgpRunDetail) {
	rec := test.record
	rec.State = runner.StateRunning
	rec.StartTime = time.Now()

	// Make the addresses this fixture's config binds usable before either
	// process starts. A host missing one used to learn it only from the test's
	// own deadline: ze wrote `Ze running`, never reached its local-address, and
	// the runner reported `context deadline exceeded` 180 seconds later with no
	// cause and no fix in the output.
	if err := runner.EnsureConfigFileBindAddresses(test.configs); err != nil {
		rec.State = runner.StateFail
		rec.Duration = time.Since(rec.StartTime)
		rec.Error = err
		rec.FailureType = runner.FailTypeLoopbackMissing
		return false, exabgpRunDetail{}
	}

	testCtx, cancel := context.WithTimeout(ctx, cli.timeout)
	defer cancel()

	server, events, err := startExaBGPServer(testCtx, test, 0, cli.saveDir)
	if err != nil {
		rec.State = runner.StateFail
		rec.Duration = time.Since(rec.StartTime)
		rec.Error = err
		return false, exabgpRunDetail{}
	}

	var detail exabgpRunDetail
	port, err := waitExaBGPPort(testCtx, events.port, server)
	if err != nil {
		stopExaProcess(server)
		detail.serverStdout = server.stdout.String()
		detail.serverStderr = server.stderr.String()
		rec.State = runner.StateFail
		rec.Duration = time.Since(rec.StartTime)
		rec.Error = err
		return false, detail
	}
	detail.port = port

	client, configs, err := startExaBGPClient(testCtx, test, port, cli.zeBinary)
	if err != nil {
		stopExaProcess(server)
		detail.serverStdout = server.stdout.String()
		detail.serverStderr = server.stderr.String()
		rec.State = runner.StateFail
		rec.Duration = time.Since(rec.StartTime)
		rec.Error = err
		return false, detail
	}
	go deliverExaBGPReloads(events.signal, client, configs)

	serverDone := false
	clientFailed := false
	var serverErr error
	var clientErrEarly error
	serverDoneCh := server.done
	clientDoneCh := client.done
	for !serverDone {
		select {
		case <-testCtx.Done():
			stopExaProcess(client)
			stopExaProcess(server)
			detail = collectExaBGPDetail(server, client, port)
			rec.State = runner.StateTimeout
			rec.Duration = time.Since(rec.StartTime)
			rec.Error = testCtx.Err()
			return false, detail
		case <-serverDoneCh:
			serverDone = true
			serverDoneCh = nil
			serverErr = server.Err()
		case <-clientDoneCh:
			clientDoneCh = nil
			clientErr := client.Err()
			if clientErr != nil && !serverDone {
				clientFailed = true
				clientErrEarly = clientErr
				stopExaProcess(server)
				serverDone = true
				serverDoneCh = nil
				serverErr = server.Err()
			}
		}
	}

	if client.Running() {
		stopExaProcess(client)
	}
	detail = collectExaBGPDetail(server, client, port)

	rec.Duration = time.Since(rec.StartTime)
	serverOK := serverErr == nil && strings.Contains(detail.serverStdout, "successful")
	if serverOK && !clientFailed {
		rec.State = runner.StateSuccess
		rec.Error = nil
		return true, detail
	}
	rec.State = runner.StateFail
	switch {
	case clientFailed:
		rec.Error = clientErrEarly
		if rec.Error == nil {
			rec.Error = errors.New("ExaBGP wrapper exited before mock BGP server completed")
		}
	case serverErr != nil:
		rec.Error = serverErr
	default:
		rec.Error = errors.New("ExaBGP mock BGP server did not report success")
	}
	return false, detail
}

func waitExaBGPPort(ctx context.Context, portCh <-chan int, server *exaProcess) (int, error) {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case port, ok := <-portCh:
		if !ok || port <= 0 {
			return 0, errExaBGPMissingPort
		}
		return port, nil
	case <-server.done:
		return 0, errExaBGPMissingPort
	case <-timer.C:
		return 0, errExaBGPPortTimeout
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func collectExaBGPDetail(server, client *exaProcess, port int) exabgpRunDetail {
	detail := exabgpRunDetail{port: port}
	if server != nil {
		detail.serverStdout = server.stdout.String()
		detail.serverStderr = server.stderr.String()
	}
	if client != nil {
		detail.clientStdout = client.stdout.String()
		detail.clientStderr = client.stderr.String()
	}
	return detail
}

func printExaBGPFailure(test *exabgpTestEntry, detail exabgpRunDetail) {
	_, _ = fmt.Fprintln(os.Stdout)                                                      //nolint:errcheck // output
	_, _ = fmt.Fprintln(os.Stdout, "TEST FAILURE:", test.record.Nick, test.record.Name) //nolint:errcheck // output
	if test.record.Error != nil {
		_, _ = fmt.Fprintln(os.Stdout, "  error:", test.record.Error) //nolint:errcheck // output
	}
	printExaBGPOutput(test, detail)
	_, _ = fmt.Fprintln(os.Stdout, "  rerun: ze-test exabgp", test.record.Nick) //nolint:errcheck // output
}

func printExaBGPOutput(_ *exabgpTestEntry, detail exabgpRunDetail) {
	if detail.port > 0 {
		_, _ = fmt.Fprintln(os.Stdout, "  port:", detail.port) //nolint:errcheck // output
	}
	printNamedOutput("server stdout", detail.serverStdout)
	printNamedOutput("server stderr", detail.serverStderr)
	printNamedOutput("client stdout", detail.clientStdout)
	printNamedOutput("client stderr", detail.clientStderr)
}

func printNamedOutput(name, output string) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, "  "+name+":") //nolint:errcheck // output
	for line := range strings.SplitSeq(trimmed, "\n") {
		_, _ = fmt.Fprintln(os.Stdout, "    "+line) //nolint:errcheck // output
	}
}

func runExaBGPServerForeground(test *exabgpTestEntry, port int, saveDir string) error {
	if port < 0 {
		return errors.New("--port must be >= 0")
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(context.Background(), binary, exaBGPServerArgs(test, port, saveDir)...) //nolint:gosec // binary is os.Executable(), this test runner re-executing itself
	cmd.Env = exaBGPServerEnv(test, port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runExaBGPClientForeground(test *exabgpTestEntry, port int, zeBinary string) error {
	config, err := exaBGPClientConfig(context.Background(), test, zeBinary)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(context.Background(), zeBinary, "start", config.path) //nolint:gosec // zeBinary is the ze under test, named on this runner's own command line
	cmd.Env = exaBGPClientEnv(test, port, zeBinary, config.path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startExaBGPServer(ctx context.Context, test *exabgpTestEntry, port int, saveDir string) (*exaProcess, *exaEvents, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	// One signal slot for each config the fixture names, which is a slot more
	// than the signals it can ask for (parseExaBGPCI checks that pairing). The
	// send must never block: the sender is the goroutine draining the server's
	// stdout pipe, and a blocked drain stops reap from ever calling Wait, which
	// hangs stopExaProcess and with it the whole run.
	events := &exaEvents{port: make(chan int, 1), signal: make(chan string, len(test.configs))}
	proc, err := startExaProcess(ctx, "server", binary, exaBGPServerArgs(test, port, saveDir), exaBGPServerEnv(test, port), events)
	return proc, events, err
}

func startExaBGPClient(ctx context.Context, test *exabgpTestEntry, port int, zeBinary string) (*exaProcess, exabgpClientConfigs, error) {
	config, err := exaBGPClientConfig(ctx, test, zeBinary)
	if err != nil {
		return nil, exabgpClientConfigs{}, err
	}
	client, err := startExaProcess(ctx, "client", zeBinary, []string{"start", config.path}, exaBGPClientEnv(test, port, zeBinary, config.path), nil)
	if err != nil {
		return nil, exabgpClientConfigs{}, err
	}
	return client, config, nil
}

func exaBGPServerArgs(test *exabgpTestEntry, port int, saveDir string) []string {
	args := []string{"interop-bgp", "exabgp-server", "--port", strconv.Itoa(port), "--terse"}
	if saveDir != "" {
		args = append(args, "--save", saveDir)
	}
	return append(args, test.ciFile)
}

func exaBGPServerEnv(test *exabgpTestEntry, port int) []string {
	env := os.Environ()
	env = append(env,
		"exabgp_tcp_port="+strconv.Itoa(port),
		"EXABGP_TEST_CONFIG="+test.configs[0],
		"EXABGP_TEST_NAME="+test.record.Nick,
	)
	return env
}

func exaBGPClientEnv(test *exabgpTestEntry, port int, zeBinary, configPath string) []string {
	env := os.Environ()
	portText := strconv.Itoa(port)
	var tb textbuf.Buffer
	env = append(env,
		"ze_test_bgp_port="+portText,
		"exabgp_tcp_port="+portText,
		"exabgp_tcp_connections="+strconv.Itoa(test.tcpConnections),
		"exabgp_api_cli=false",
		"exabgp_debug_rotate=true",
		"exabgp_debug_configuration=true",
		"exabgp_api_socketname=exabgp-test-"+portText,
		"exabgp_api_version=4",
		// Each daemon gets its own store. Without it they share the one derived
		// from the binary's location and corrupt each other's zefs and CA.
		tb.Reset().Str("ze.config.dir=").Str(filepath.Dir(configPath)).String(),
		// The daemon FORKS the helper a config's process block names, and
		// conf-watchdog names `ze-test fixture ...`, which sits beside the ze
		// under test. It was reachable from nowhere until the migration started
		// emitting the bridge, because the process was being dropped and nothing
		// ever forked it.
		tb.Reset().Str("PATH=").Str(filepath.Dir(zeBinary)).Byte(os.PathListSeparator).Str(os.Getenv("PATH")).String(),
		// Appended AFTER os.Environ() so the verified binary wins over any
		// inherited ZE_BIN value.
		tb.Str("ZE_BIN=").Str(zeBinary).String(),
	)
	return env
}

// exabgpClientConfigs is what a case's `option=file:` configs become: the path
// ze starts on, and the native text of every config the fixture named after the
// first, in fixture order.
//
// A case that names several configs is testing a RELOAD. The later ones are
// held as TEXT rather than written beside the active path, because a reload
// writes one of them OVER that path and nothing else ever reads them from disk.
//
// Concatenating them into one file was the old approximation and it was
// silently wrong: a second `neighbor` block for the same address overrides the
// first, so api-reload.1.conf's two static routes became one and the withdrawal
// its fixture expects on reload could never happen (ai/rules/principles.md).
type exabgpClientConfigs struct {
	path    string
	reloads []string
}

func exaBGPClientConfig(ctx context.Context, test *exabgpTestEntry, zeBinary string) (exabgpClientConfigs, error) {
	native := make([]string, 0, len(test.configs))
	for _, source := range test.configs {
		migrated, err := migrateExaBGPConfig(ctx, zeBinary, source)
		if err != nil {
			return exabgpClientConfigs{}, err
		}
		native = append(native, strings.ReplaceAll(migrated, "local {", "local {\n\t\t\t\t\taccept false;"))
	}
	// A DIRECTORY per test, not just a file. ze derives its config directory
	// from its own binary unless ze.config.dir says otherwise
	// (internal/core/paths.DefaultConfigDir), so every concurrent daemon in this
	// suite shared one database.zefs and one certificate authority. That is what
	// made the suite answer between 9 and 13 passes for an unchanged tree:
	// api-rib and api-rr-rib pass alone and deliver nothing under load.
	//
	// The migrated config lives in it and exaBGPClientEnv points ze at it, so
	// each daemon reads and writes its own store.
	directory, err := os.MkdirTemp("", "ze-exabgp-native-*")
	if err != nil {
		return exabgpClientConfigs{}, err
	}
	// Seeded from the run's own config directory, not left empty. The store
	// holds the local username and the plugin CA, and a daemon handed an empty
	// one spends its startup minting them: conf-watchdog, which runs a plugin
	// over that CA, timed out waiting for a daemon busy doing it.
	if err := copyConfigDir(paths.ConfigDirFromBinary(zeBinary), directory); err != nil {
		return exabgpClientConfigs{}, err
	}
	// NOT ze.conf. That is the name the blob store adopts as its active config
	// (internal/core/resolve.DefaultConfig), so a file called that in the config
	// directory is migrated into the store at startup, and conf-watchdog timed
	// out waiting for a daemon busy doing it.
	path := filepath.Join(directory, "migrated.conf")
	if err := os.WriteFile(path, []byte(native[0]), 0o600); err != nil {
		return exabgpClientConfigs{}, err
	}
	return exabgpClientConfigs{path: path, reloads: native[1:]}, nil
}

// absoluteBridgeRun rewrites the ExaBGP bridge's run command to an absolute
// path rooted at the directory the ExaBGP config came from.
//
// An ExaBGP config names its process script relative to itself, as
// `run ./run/api-announce.run`, and the migrated config is written to a
// temporary file somewhere else entirely. The relative path would then resolve
// against the daemon's working directory, where the script does not exist, and
// the bridge would start nothing.
//
// Only a run line under the bridge is rewritten, and only when its value is
// relative. An absolute path is already unambiguous and is left alone.
func absoluteBridgeRun(migrated, configDir string) string {
	const runKeyword = "run "
	lines := strings.Split(migrated, "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		command, ok := strings.CutPrefix(trimmed, runKeyword)
		if !ok {
			continue
		}
		command = strings.Trim(command, `"`)
		// Only an EXPLICITLY relative command is rooted at the config. A bare
		// name is resolved through PATH, and conf-watchdog runs `ze-test fixture
		// ...`: absolutising that turned it into a path under the fixture
		// directory and the bridge forked a file that does not exist.
		if !strings.HasPrefix(command, "./") && !strings.HasPrefix(command, "../") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		var tb textbuf.Buffer
		lines[index] = tb.Str(indent).Str(runKeyword).Quoted(filepath.Join(configDir, command)).String()
	}
	return strings.Join(lines, "\n")
}

// copyConfigDir seeds a per-test config directory from the one the run shares,
// so a daemon starts from the state every other test starts from and cannot
// corrupt a peer's store while it runs.
//
// A missing or unnamed source is not an error: ze mints what it needs, and the
// only cost is startup time.
func copyConfigDir(source, destination string) error {
	if source == "" {
		return nil
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil //nolint:nilerr // an absent shared store is a state ze can start from
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name())) //nolint:gosec // the run's own config directory
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(destination, entry.Name()), data, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// migrateExaBGPConfig converts one ExaBGP config into ze's syntax, with the
// bridge's `run` line rooted at the config's own directory.
func migrateExaBGPConfig(ctx context.Context, zeBinary, source string) (string, error) {
	command := exec.CommandContext(ctx, zeBinary, "exabgp", "migrate", source) //nolint:gosec // zeBinary is the ze under test, named on this runner's own command line
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("migrate %s: %w", source, err)
	}
	return absoluteBridgeRun(string(output), filepath.Dir(source)), nil
}
