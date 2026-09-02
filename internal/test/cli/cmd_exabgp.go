// Design: docs/architecture/testing/ci-format.md — predecessor encoding test runner

package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"

	"github.com/ze-software/ze/internal/test/runner"
)

const exabgpSuiteEncoding = "encoding"

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
	if suiteName != exabgpSuiteEncoding {
		return errors.New("only predecessor encoding tests are available")
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

	suite, err := discoverExaBGPSuite(baseDir)
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
	return name == exabgpSuiteEncoding
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

func discoverExaBGPSuite(baseDir string) (*exabgpSuite, error) {
	root := filepath.Join(baseDir, "test", predecessorTestDir)
	pattern := filepath.Join(root, "encoding", "*.ci")
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

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if after, ok := strings.CutPrefix(line, "option=file:"); ok {
			configPath := filepath.Join(root, "etc", after)
			test.configs = append(test.configs, configPath)
			rec.Files = append(rec.Files, configPath)
			continue
		}
		if after, ok := strings.CutPrefix(line, "option=tcp_connections:"); ok {
			count, err := strconv.Atoi(after)
			if err != nil || count <= 0 {
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

	testCtx, cancel := context.WithTimeout(ctx, cli.timeout)
	defer cancel()

	server, portCh, err := startExaBGPServer(testCtx, test, 0, cli.saveDir)
	if err != nil {
		rec.State = runner.StateFail
		rec.Duration = time.Since(rec.StartTime)
		rec.Error = err
		return false, exabgpRunDetail{}
	}

	var detail exabgpRunDetail
	port, err := waitExaBGPPort(testCtx, portCh, server)
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

	client, err := startExaBGPClient(testCtx, test, port, cli.zeBinary)
	if err != nil {
		stopExaProcess(server)
		detail.serverStdout = server.stdout.String()
		detail.serverStderr = server.stderr.String()
		rec.State = runner.StateFail
		rec.Duration = time.Since(rec.StartTime)
		rec.Error = err
		return false, detail
	}

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
	cmd := exec.CommandContext(context.Background(), zeBinary, "start", config) //nolint:gosec // zeBinary is the ze under test, named on this runner's own command line
	cmd.Env = exaBGPClientEnv(test, port, zeBinary)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startExaBGPServer(ctx context.Context, test *exabgpTestEntry, port int, saveDir string) (*exaProcess, <-chan int, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	portCh := make(chan int, 1)
	proc, err := startExaProcess(ctx, "server", binary, exaBGPServerArgs(test, port, saveDir), exaBGPServerEnv(test, port), portCh)
	return proc, portCh, err
}

func startExaBGPClient(ctx context.Context, test *exabgpTestEntry, port int, zeBinary string) (*exaProcess, error) {
	config, err := exaBGPClientConfig(ctx, test, zeBinary)
	if err != nil {
		return nil, err
	}
	return startExaProcess(ctx, "client", zeBinary, []string{"start", config}, exaBGPClientEnv(test, port, zeBinary), nil)
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

func exaBGPClientEnv(test *exabgpTestEntry, port int, zeBinary string) []string {
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
		// Appended AFTER os.Environ() so the verified binary wins over any
		// inherited ZE_BIN value.
		tb.Str("ZE_BIN=").Str(zeBinary).String(),
	)
	return env
}

func exaBGPClientConfig(ctx context.Context, test *exabgpTestEntry, zeBinary string) (string, error) {
	var config textbuf.Buffer
	for _, source := range test.configs {
		command := exec.CommandContext(ctx, zeBinary, "exabgp", "migrate", source) //nolint:gosec // zeBinary is the ze under test, named on this runner's own command line
		output, err := command.Output()
		if err != nil {
			return "", fmt.Errorf("migrate %s: %w", source, err)
		}
		config.Write(output)
		config.Byte('\n')
	}
	file, err := os.CreateTemp("", "ze-exabgp-native-*.conf")
	if err != nil {
		return "", err
	}
	path := file.Name()
	nativeConfig := strings.ReplaceAll(config.String(), "local {", "local {\n\t\t\t\t\taccept false;")
	if _, err := file.WriteString(nativeConfig); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

type exaProcess struct {
	name   string
	cmd    *exec.Cmd
	stdout lockedBuffer
	stderr lockedBuffer
	done   chan struct{}
	mu     sync.Mutex
	err    error
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Append(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, _ = b.b.WriteString(s)
	_ = b.b.WriteByte('\n')
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func startExaProcess(ctx context.Context, name, program string, args, env []string, portCh chan<- int) (*exaProcess, error) {
	cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // program and args target repository-owned compatibility fixtures.
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	proc := &exaProcess{name: name, cmd: cmd, done: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go copyExaOutput(stdout, &proc.stdout, portCh)
	go copyExaOutput(stderr, &proc.stderr, nil)
	go func() {
		err := cmd.Wait()
		proc.mu.Lock()
		proc.err = err
		proc.mu.Unlock()
		close(proc.done)
		if portCh != nil {
			close(portCh)
		}
	}()
	return proc, nil
}

func copyExaOutput(r io.Reader, dst *lockedBuffer, portCh chan<- int) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		dst.Append(line)
		if portCh == nil || !strings.HasPrefix(line, "PORT ") {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "PORT ")))
		if err == nil && port > 0 {
			select {
			case portCh <- port:
			default:
			}
		}
	}
}

func (p *exaProcess) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *exaProcess) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func stopExaProcess(p *exaProcess) {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	if !p.Running() {
		return
	}
	pid := p.cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	select {
	case <-p.done:
		return
	case <-time.After(500 * time.Millisecond):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	<-p.done
}
