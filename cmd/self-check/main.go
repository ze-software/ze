// Command self-check runs ZeBGP functional tests.
//
// It orchestrates tests by:
//  1. Starting zebgp-peer as the BGP test server
//  2. Starting zebgp with a test configuration
//  3. Verifying expected BGP messages are exchanged
//
// Test files are in ExaBGP's .ci format:
//
//	option:file:config.conf           # config file to use
//	option:asn:65000                  # peer ASN
//	1:raw:FFFF...:0017:02:...         # expected message
//
// Usage:
//
//	self-check --list
//	self-check 0
//	self-check --all
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[91m"
	colorGreen  = "\033[92m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorNormal = "\033[0m"
)

// Test states.
type State int

const (
	StateSkip State = iota
	StateNone
	StateStarting
	StateRunning
	StateFail
	StateSuccess
)

// Test represents a single test case.
type Test struct {
	Nick    string
	Name    string
	CIFile  string   // Path to .ci file
	Config  string   // Path to .conf file (from option:file:)
	Port    int      // Port for this test
	Options []string // Options for zebgp-peer (option:asn:, etc.)
	Expects []string // Expected messages (N:raw:...)
	Files   []string // All files related to this test
	State   State
}

// Colored returns the test nick with ANSI color based on state.
func (t *Test) Colored() string {
	switch t.State {
	case StateNone:
		return colorGray + t.Nick + colorReset
	case StateStarting:
		return colorGray + t.Nick + colorReset
	case StateRunning:
		return colorNormal + t.Nick + colorReset
	case StateFail:
		return colorRed + t.Nick + colorReset
	case StateSuccess:
		return colorGreen + "✓" + colorReset
	case StateSkip:
		return colorBlue + "✖" + colorReset
	}
	return t.Nick
}

// Tests holds all test cases.
type Tests struct {
	tests   []*Test
	byNick  map[string]*Test
	baseDir string
}

// NewTests creates a new test collection.
func NewTests(baseDir string) *Tests {
	return &Tests{
		byNick:  make(map[string]*Test),
		baseDir: baseDir,
	}
}

// Load discovers tests from the testdata directory.
func (ts *Tests) Load() error {
	testdataDir := filepath.Join(ts.baseDir, "testdata")
	pattern := filepath.Join(testdataDir, "*.ci")

	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	sort.Strings(files)

	nicks := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	port := 1790

	for i, ciFile := range files {
		name := strings.TrimSuffix(filepath.Base(ciFile), ".ci")
		nick := string(nicks[i%len(nicks)])

		test, err := ts.parseCIFile(ciFile, name, nick, port)
		if err != nil {
			continue
		}

		ts.tests = append(ts.tests, test)
		ts.byNick[nick] = test
		port++
	}

	return nil
}

// parseCIFile parses an ExaBGP-format .ci file.
func (ts *Tests) parseCIFile(ciFile, name, nick string, port int) (*Test, error) {
	f, err := os.Open(ciFile) //nolint:gosec // Test files from known directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	test := &Test{
		Nick:   nick,
		Name:   name,
		CIFile: ciFile,
		Port:   port,
		State:  StateSkip,
		Files:  []string{ciFile},
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "option:file:"):
			configName := strings.TrimPrefix(line, "option:file:")
			test.Config = filepath.Join(ts.baseDir, "testdata", configName)
			test.Files = append(test.Files, test.Config)

		case strings.HasPrefix(line, "option:"):
			// Pass through other options to zebgp-peer
			test.Options = append(test.Options, line)

		case strings.Contains(line, ":raw:"):
			// Expected message in raw hex format
			test.Expects = append(test.Expects, line)

		case strings.Contains(line, ":cmd:"):
			// Command to send - skip for now (ZeBGP handles via config)
			continue

		case strings.Contains(line, ":json:"):
			// JSON expected - skip for now
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if test.Config == "" {
		return nil, fmt.Errorf("no config file specified")
	}

	return test, nil
}

// Get returns a test by nick.
func (ts *Tests) Get(nick string) *Test {
	return ts.byNick[nick]
}

// EnableAll enables all tests.
func (ts *Tests) EnableAll() {
	for _, t := range ts.tests {
		t.State = StateNone
	}
}

// Enable enables a specific test by nick.
func (ts *Tests) Enable(nick string) bool {
	if t, ok := ts.byNick[nick]; ok {
		t.State = StateNone
		return true
	}
	return false
}

// Selected returns enabled tests.
func (ts *Tests) Selected() []*Test {
	var result []*Test
	for _, t := range ts.tests {
		if t.State != StateSkip {
			result = append(result, t)
		}
	}
	return result
}

// List prints available tests.
func (ts *Tests) List() {
	fmt.Println("\nAvailable tests:")
	fmt.Println()
	for _, t := range ts.tests {
		fmt.Printf("  %s  %s\n", t.Nick, t.Name)
	}
	fmt.Println()
}

// Display shows current test status.
func (ts *Tests) Display() {
	for _, t := range ts.tests {
		fmt.Print(" " + t.Colored())
	}
	fmt.Print(colorReset + "\r")
}

// RunResult holds the result of running a test.
type RunResult struct {
	Test    *Test
	Success bool
	Output  string
}

// Runner executes tests.
type Runner struct {
	tests     *Tests
	baseDir   string
	timeout   time.Duration
	zebgpPath string
	peerPath  string
	tmpDir    string
}

// NewRunner creates a new test runner.
func NewRunner(tests *Tests, baseDir string, timeout time.Duration) (*Runner, error) {
	// Build binaries to temp dir for reliable process control.
	tmpDir, err := os.MkdirTemp("", "zebgp-test-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	zebgpPath := filepath.Join(tmpDir, "zebgp")
	peerPath := filepath.Join(tmpDir, "zebgp-peer")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Build zebgp.
	cmd := exec.CommandContext(ctx, "go", "build", "-o", zebgpPath, "./cmd/zebgp") //nolint:gosec // Build path is constructed safely
	cmd.Dir = baseDir
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("build zebgp: %w\n%s", err, out)
	}

	// Build zebgp-peer.
	cmd = exec.CommandContext(ctx, "go", "build", "-o", peerPath, "./cmd/zebgp-peer") //nolint:gosec // Build path is constructed safely
	cmd.Dir = baseDir
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("build zebgp-peer: %w\n%s", err, out)
	}

	return &Runner{
		tests:     tests,
		baseDir:   baseDir,
		timeout:   timeout,
		zebgpPath: zebgpPath,
		peerPath:  peerPath,
		tmpDir:    tmpDir,
	}, nil
}

// Cleanup removes temporary files.
func (r *Runner) Cleanup() {
	if r.tmpDir != "" {
		_ = os.RemoveAll(r.tmpDir)
	}
}

// Run executes selected tests.
func (r *Runner) Run(ctx context.Context) bool {
	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Println("No tests selected")
		return true
	}

	results := make(chan *RunResult, len(selected))
	var wg sync.WaitGroup

	for _, test := range selected {
		wg.Add(1)
		go func(t *Test) {
			defer wg.Done()
			success, output := r.runTest(ctx, t)
			results <- &RunResult{Test: t, Success: success, Output: output}
		}(test)
	}

	// Collect results.
	go func() {
		wg.Wait()
		close(results)
	}()

	allSuccess := true
	for result := range results {
		if result.Success {
			result.Test.State = StateSuccess
		} else {
			result.Test.State = StateFail
			allSuccess = false
			if result.Output != "" {
				fmt.Printf("\n%s failed:\n%s\n", result.Test.Name, result.Output)
			}
		}
		r.tests.Display()
	}

	return allSuccess
}

func (r *Runner) runTest(ctx context.Context, test *Test) (bool, string) {
	test.State = StateStarting
	r.tests.Display()

	// Check config exists.
	if _, err := os.Stat(test.Config); os.IsNotExist(err) {
		return false, fmt.Sprintf("config not found: %s", test.Config)
	}

	testCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Write expects to temp file for zebgp-peer.
	expectFile, err := r.writeExpectFile(test)
	if err != nil {
		return false, fmt.Sprintf("failed to write expect file: %v", err)
	}
	defer func() { _ = os.Remove(expectFile) }()

	// Build zebgp-peer arguments.
	peerArgs := []string{"--port", fmt.Sprintf("%d", test.Port)}

	// Add options from .ci file.
	for _, opt := range test.Options {
		switch {
		case strings.HasPrefix(opt, "option:asn:"):
			peerArgs = append(peerArgs, "--asn", strings.TrimPrefix(opt, "option:asn:"))
		case opt == "option:bind:ipv6":
			peerArgs = append(peerArgs, "--ipv6")
		}
	}

	peerArgs = append(peerArgs, expectFile)

	// Start zebgp-peer (server).
	serverCmd := exec.CommandContext(testCtx, r.peerPath, peerArgs...) //nolint:gosec // Args from known test files
	serverCmd.Env = append(os.Environ(), fmt.Sprintf("exabgp_tcp_port=%d", test.Port))

	serverOut, _ := serverCmd.StdoutPipe()
	serverErr, _ := serverCmd.StderrPipe()

	if err := serverCmd.Start(); err != nil {
		return false, fmt.Sprintf("failed to start server: %v", err)
	}
	defer func() {
		_ = serverCmd.Process.Kill()
		_ = serverCmd.Wait()
	}()

	// Wait for server to be ready.
	time.Sleep(100 * time.Millisecond)

	test.State = StateRunning
	r.tests.Display()

	// Start zebgp (client).
	clientCmd := exec.CommandContext(testCtx, r.zebgpPath, "run", test.Config) //nolint:gosec // Paths from known base dir
	clientCmd.Env = append(os.Environ(),
		fmt.Sprintf("exabgp_tcp_port=%d", test.Port),
		"exabgp_tcp_bind=",
	)

	clientOut, _ := clientCmd.StdoutPipe()
	clientErr, _ := clientCmd.StderrPipe()

	if err := clientCmd.Start(); err != nil {
		return false, fmt.Sprintf("failed to start client: %v", err)
	}
	defer func() {
		_ = clientCmd.Process.Kill()
		_ = clientCmd.Wait()
	}()

	// Wait for test to complete.
	done := make(chan error, 1)
	go func() {
		done <- serverCmd.Wait()
	}()

	select {
	case err := <-done:
		// Server finished - check if successful.
		var output strings.Builder

		// Read outputs.
		serverOutBytes := readAll(serverOut)
		serverErrBytes := readAll(serverErr)
		clientOutBytes := readAll(clientOut)
		clientErrBytes := readAll(clientErr)

		if err != nil {
			output.WriteString(fmt.Sprintf("server error: %v\n", err))
		}
		if len(serverOutBytes) > 0 {
			output.WriteString(fmt.Sprintf("server stdout:\n%s\n", serverOutBytes))
		}
		if len(serverErrBytes) > 0 {
			output.WriteString(fmt.Sprintf("server stderr:\n%s\n", serverErrBytes))
		}
		if len(clientOutBytes) > 0 {
			output.WriteString(fmt.Sprintf("client stdout:\n%s\n", clientOutBytes))
		}
		if len(clientErrBytes) > 0 {
			output.WriteString(fmt.Sprintf("client stderr:\n%s\n", clientErrBytes))
		}

		// Check for "successful" in output.
		fullOutput := string(serverOutBytes) + string(serverErrBytes)
		if strings.Contains(fullOutput, "successful") {
			return true, ""
		}

		return false, output.String()

	case <-testCtx.Done():
		return false, "test timed out"
	}
}

// writeExpectFile writes the expected messages to a temp file for zebgp-peer.
func (r *Runner) writeExpectFile(test *Test) (string, error) {
	f, err := os.CreateTemp("", "zebgp-test-*.expect")
	if err != nil {
		return "", err
	}

	// Write options.
	for _, opt := range test.Options {
		if _, err := fmt.Fprintln(f, opt); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return "", err
		}
	}

	// Write expected messages.
	for _, exp := range test.Expects {
		if _, err := fmt.Fprintln(f, exp); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return "", err
		}
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}

	return f.Name(), nil
}

func readAll(r interface{ Read([]byte) (int, error) }) []byte {
	var result []byte
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return result
}

func main() {
	// Find base directory.
	baseDir, err := findBaseDir()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Parse flags.
	listFlag := flag.Bool("list", false, "list available tests")
	allFlag := flag.Bool("all", false, "run all tests")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "timeout per test")
	flag.Parse()

	// Load tests.
	tests := NewTests(baseDir)
	if err := tests.Load(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error loading tests: %v\n", err)
		os.Exit(1)
	}

	if *listFlag {
		tests.List()
		return
	}

	// Select tests.
	switch {
	case *allFlag:
		tests.EnableAll()
	case flag.NArg() > 0:
		for _, arg := range flag.Args() {
			if !tests.Enable(arg) {
				_, _ = fmt.Fprintf(os.Stderr, "Unknown test: %s\n", arg)
				os.Exit(1)
			}
		}
	default:
		tests.List()
		fmt.Println("Use --all to run all tests, or specify test nick(s)")
		return
	}

	// Setup signal handling.
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Build and run tests.
	fmt.Println("Building test binaries...")
	runner, err := NewRunner(tests, baseDir, *timeoutFlag)
	if err != nil {
		cancel()
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1) //nolint:gocritic // cancel() called above
	}

	success := runner.Run(ctx)
	runner.Cleanup()
	cancel()

	fmt.Println()
	if !success {
		fmt.Println("Some tests failed")
		os.Exit(1)
	}
	fmt.Println("All tests passed")
}

func findBaseDir() (string, error) {
	// Try to find go.mod.
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

	return os.Getwd()
}
