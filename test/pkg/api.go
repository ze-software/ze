package functional

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// APITests manages API test discovery and execution.
// API tests are like encoding tests but have .run scripts that send commands.
type APITests struct {
	*Tests
	baseDir string
	port    int
}

// NewAPITests creates a new API test manager.
func NewAPITests(baseDir string) *APITests {
	return &APITests{
		Tests:   NewTests(),
		baseDir: baseDir,
		port:    1890, // Start API tests at different port range
	}
}

// Discover finds all .ci files in the API test directory.
func (at *APITests) Discover(dir string) error {
	pattern := filepath.Join(dir, "*.ci")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	// Sort for deterministic nick assignment
	sort.Strings(files)

	for _, ciFile := range files {
		if err := at.parseAndAdd(ciFile); err != nil {
			// Log but continue with other tests
			continue
		}
	}

	return nil
}

// parseAndAdd parses a .ci file and adds it as a test.
func (at *APITests) parseAndAdd(ciFile string) error {
	name := strings.TrimSuffix(filepath.Base(ciFile), ".ci")
	r := at.Add(name)
	r.Port = at.port
	at.port++
	r.IsAPI = true
	r.Files = append(r.Files, ciFile)

	// Find associated files
	dir := filepath.Dir(ciFile)

	// Config file
	confFile := filepath.Join(dir, name+".conf")
	if _, err := os.Stat(confFile); err == nil {
		r.Conf["config"] = confFile
		r.Files = append(r.Files, confFile)
	}

	// Run script - validate it stays within test directory
	runFile := filepath.Join(dir, name+".run")
	if _, err := os.Stat(runFile); err == nil {
		absRun, err := filepath.Abs(runFile)
		if err != nil {
			return fmt.Errorf("invalid run script path: %w", err)
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("invalid test dir: %w", err)
		}
		if !strings.HasPrefix(absRun, absDir+string(filepath.Separator)) {
			return fmt.Errorf("run script outside test directory: %s", runFile)
		}
		r.Conf["run"] = runFile
		r.Files = append(r.Files, runFile)
	}

	// Parse CI file for expected messages
	f, err := os.Open(ciFile) //nolint:gosec // Test files from known directory
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// Use same parser as encoding tests
	et := &EncodingTests{Tests: NewTests(), baseDir: at.baseDir}
	if err := et.parseAndAdd(ciFile); err != nil {
		return err
	}

	// Copy parsed data
	if et.Count() > 0 {
		parsed := et.Registered()[0]
		r.Options = parsed.Options
		r.Expects = parsed.Expects
		r.Extra = parsed.Extra
		if config, ok := parsed.Conf["config"].(string); ok {
			r.Conf["config"] = config
		}
	}

	return nil
}

// APIRunner executes API tests.
type APIRunner struct {
	tests     *APITests
	baseDir   string
	tmpDir    string
	zebgpPath string
	peerPath  string
	timing    *TimingCache
}

// NewAPIRunner creates an API test runner.
func NewAPIRunner(tests *APITests, baseDir string) (*APIRunner, error) {
	// Create temp directory for binaries
	tmpDir, err := os.MkdirTemp("", "zebgp-api-test-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	return &APIRunner{
		tests:     tests,
		baseDir:   baseDir,
		tmpDir:    tmpDir,
		zebgpPath: filepath.Join(tmpDir, "zebgp"),
		peerPath:  filepath.Join(tmpDir, "zebgp-peer"),
		timing:    NewTimingCache(),
	}, nil
}

// Build compiles the test binaries.
func (r *APIRunner) Build(ctx context.Context) error {
	// Build zebgp
	buildCmd := NewExec()
	buildCmd.SetDir(r.baseDir)
	args := []string{"go", "build", "-o", r.zebgpPath, "./cmd/zebgp"}
	env := map[string]string{"CGO_ENABLED": "0"}

	if err := buildCmd.Run(ctx, args, env); err != nil {
		return fmt.Errorf("build zebgp: %w", err)
	}
	if err := buildCmd.Wait(); err != nil {
		return fmt.Errorf("build zebgp: %w\n%s", err, buildCmd.Stderr())
	}
	if buildCmd.ExitCode() != 0 {
		return fmt.Errorf("build zebgp failed: %s", buildCmd.Stderr())
	}

	// Build zebgp-peer
	buildCmd = NewExec()
	buildCmd.SetDir(r.baseDir)
	args = []string{"go", "build", "-o", r.peerPath, "./test/cmd/zebgp-peer"}
	if err := buildCmd.Run(ctx, args, env); err != nil {
		return fmt.Errorf("build zebgp-peer: %w", err)
	}
	if err := buildCmd.Wait(); err != nil {
		return fmt.Errorf("build zebgp-peer: %w\n%s", err, buildCmd.Stderr())
	}
	if buildCmd.ExitCode() != 0 {
		return fmt.Errorf("build zebgp-peer failed: %s", buildCmd.Stderr())
	}

	return nil
}

// Cleanup removes temporary files.
func (r *APIRunner) Cleanup() {
	if r.tmpDir != "" {
		_ = os.RemoveAll(r.tmpDir)
	}
}

// Run executes selected API tests.
func (r *APIRunner) Run(ctx context.Context, opts *RunOptions) bool {
	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Println("No API tests selected")
		return true
	}

	// Load timing cache
	_ = r.timing.Load()

	allSuccess := true
	for _, rec := range selected {
		success, output := r.runTest(ctx, rec, opts)
		if success {
			rec.State = StateSuccess
		} else {
			rec.State = StateFail
			allSuccess = false
			if output != "" && (opts.Verbose || !opts.Quiet) {
				fmt.Printf("\n%s %s failed:\n%s\n", rec.Nick, rec.Name, output)
			}
		}
		if !opts.Quiet {
			r.tests.Display()
		}
	}

	// Save timing cache
	_ = r.timing.Save()

	return allSuccess
}

// runTest executes a single API test.
func (r *APIRunner) runTest(ctx context.Context, rec *Record, opts *RunOptions) (bool, string) {
	rec.State = StateStarting
	start := time.Now()

	// Create test context with timeout
	testCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Write expects to temp file
	expectFile, err := r.writeExpectFile(rec)
	if err != nil {
		return false, fmt.Sprintf("write expect file: %v", err)
	}
	defer func() { _ = os.Remove(expectFile) }()

	// Build peer args
	peerArgs := []string{"--port", fmt.Sprintf("%d", rec.Port)}
	if asn, ok := rec.Extra["asn"]; ok {
		peerArgs = append(peerArgs, "--asn", asn)
	}
	peerArgs = append(peerArgs, expectFile)

	// Start peer (server)
	peerEnv := map[string]string{
		"zebgp_tcp_port": fmt.Sprintf("%d", rec.Port),
	}
	peer := NewExec()
	if err := peer.Run(testCtx, append([]string{r.peerPath}, peerArgs...), peerEnv); err != nil {
		return false, fmt.Sprintf("start peer: %v", err)
	}
	defer peer.Terminate()

	// Wait for peer to start
	time.Sleep(100 * time.Millisecond)

	rec.State = StateRunning
	rec.StartTime = time.Now()

	// Socket path for API
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("zebgp-api-test-%d.sock", rec.Port))
	defer func() { _ = os.Remove(socketPath) }()

	// Start zebgp (client)
	configPath, _ := rec.Conf["config"].(string)
	clientEnv := map[string]string{
		"zebgp_tcp_port":       fmt.Sprintf("%d", rec.Port),
		"zebgp_tcp_bind":       "",
		"zebgp_api_socketpath": socketPath,
	}

	client := NewExec()
	if err := client.Run(testCtx, []string{r.zebgpPath, "server", configPath}, clientEnv); err != nil {
		return false, fmt.Sprintf("start client: %v", err)
	}
	defer client.Terminate()

	// Wait for zebgp to start and create socket
	time.Sleep(200 * time.Millisecond)

	// Execute .run script if present
	if runScript, ok := rec.Conf["run"].(string); ok {
		runEnv := map[string]string{
			"zebgp_api_socketpath": socketPath,
			"ZEBGP_SOCKET":         socketPath,
		}
		runExec := NewExec()
		if err := runExec.Run(testCtx, []string{runScript}, runEnv); err != nil {
			return false, fmt.Sprintf("start run script: %v", err)
		}
		defer runExec.Terminate()

		// Wait for run script to complete
		select {
		case <-runExec.done:
			runExec.Collect()
			if runExec.ExitCode() != 0 {
				return false, fmt.Sprintf("run script failed: %s", runExec.Stderr())
			}
		case <-testCtx.Done():
			return false, "run script timed out"
		}
	}

	// Wait for peer to finish (success indicator)
	select {
	case <-peer.done:
		// Peer finished
		client.Terminate()

		peer.Collect()
		client.Collect()

		// Check for success
		output := peer.Stdout() + peer.Stderr()
		if strings.Contains(output, "successful") {
			r.timing.Update(rec.Name, time.Since(start))
			return true, ""
		}

		return false, fmt.Sprintf("peer: %s\nclient: %s", output, client.Stderr())

	case <-testCtx.Done():
		rec.State = StateTimeout
		return false, "test timed out"
	}
}

// writeExpectFile writes expected messages to a temp file.
func (r *APIRunner) writeExpectFile(rec *Record) (string, error) {
	f, err := os.CreateTemp("", "zebgp-api-test-*.expect")
	if err != nil {
		return "", err
	}

	// Write options
	for _, opt := range rec.Options {
		if _, err := fmt.Fprintln(f, opt); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return "", err
		}
	}

	// Write expects
	for _, exp := range rec.Expects {
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
