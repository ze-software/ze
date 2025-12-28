package functional

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// EncodingTests manages encoding test discovery and execution.
type EncodingTests struct {
	*Tests
	baseDir string
	port    int
}

// NewEncodingTests creates a new encoding test manager.
func NewEncodingTests(baseDir string) *EncodingTests {
	return &EncodingTests{
		Tests:   NewTests(),
		baseDir: baseDir,
		port:    1790,
	}
}

// Discover finds all .ci files in the given directory.
func (et *EncodingTests) Discover(dir string) error {
	pattern := filepath.Join(dir, "*.ci")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	// Sort for deterministic nick assignment
	sort.Strings(files)

	for _, ciFile := range files {
		if err := et.parseAndAdd(ciFile); err != nil {
			// Log but continue with other tests
			continue
		}
	}

	return nil
}

// parseAndAdd parses a .ci file and adds it as a test.
func (et *EncodingTests) parseAndAdd(ciFile string) error {
	f, err := os.Open(ciFile) //nolint:gosec // Test files from known directory
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	name := strings.TrimSuffix(filepath.Base(ciFile), ".ci")
	r := et.Add(name)
	r.Port = et.port
	et.port++
	r.Files = append(r.Files, ciFile)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "option:file:"):
			configName := strings.TrimPrefix(line, "option:file:")
			configPath := filepath.Join(filepath.Dir(ciFile), configName)
			// Validate config stays within test directory (prevent path traversal)
			absConfig, err := filepath.Abs(configPath)
			if err != nil {
				return fmt.Errorf("invalid config path: %w", err)
			}
			absTestDir, err := filepath.Abs(filepath.Dir(ciFile))
			if err != nil {
				return fmt.Errorf("invalid test dir: %w", err)
			}
			if !strings.HasPrefix(absConfig, absTestDir+string(filepath.Separator)) && absConfig != absTestDir {
				return fmt.Errorf("config file outside test directory: %s", configName)
			}
			r.Conf["config"] = configPath
			r.Files = append(r.Files, configPath)

		case strings.HasPrefix(line, "option:asn:"):
			r.Extra["asn"] = strings.TrimPrefix(line, "option:asn:")
			r.Options = append(r.Options, line)

		case strings.HasPrefix(line, "option:bind:"):
			r.Extra["bind"] = strings.TrimPrefix(line, "option:bind:")
			r.Options = append(r.Options, line)

		case strings.HasPrefix(line, "option:"):
			r.Options = append(r.Options, line)

		case strings.Contains(line, ":raw:"):
			r.Expects = append(r.Expects, line)

		case strings.Contains(line, ":notification:"):
			r.Expects = append(r.Expects, line)

		case strings.Contains(line, ":cmd:"):
			// Commands are handled by config, skip
			continue

		case strings.Contains(line, ":json:"):
			// JSON expectations, skip for now
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Verify config exists
	if configPath, ok := r.Conf["config"].(string); ok {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config not found: %s", configPath)
		}
	}

	return nil
}

// RunOptions configures test execution.
type RunOptions struct {
	Timeout    time.Duration
	Parallel   int
	Verbose    bool
	DebugNicks []string
	Quiet      bool
	SaveDir    string
}

// DefaultRunOptions returns sensible defaults.
func DefaultRunOptions() *RunOptions {
	return &RunOptions{
		Timeout:  30 * time.Second,
		Parallel: 4,
		Verbose:  false,
		Quiet:    false,
	}
}

// Runner executes encoding tests.
type Runner struct {
	tests     *EncodingTests
	baseDir   string
	tmpDir    string
	zebgpPath string
	peerPath  string
	timing    *TimingCache
}

// NewRunner creates a test runner.
func NewRunner(tests *EncodingTests, baseDir string) (*Runner, error) {
	// Create temp directory for binaries
	tmpDir, err := os.MkdirTemp("", "zebgp-test-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	return &Runner{
		tests:     tests,
		baseDir:   baseDir,
		tmpDir:    tmpDir,
		zebgpPath: filepath.Join(tmpDir, "zebgp"),
		peerPath:  filepath.Join(tmpDir, "zebgp-peer"),
		timing:    NewTimingCache(),
	}, nil
}

// Build compiles the test binaries.
func (r *Runner) Build(ctx context.Context) error {
	// Build zebgp
	e := NewExec()
	e.SetDir(r.baseDir)
	if err := e.Run(ctx, []string{"go", "build", "-o", r.zebgpPath, "./cmd/zebgp"}, map[string]string{"CGO_ENABLED": "0"}); err != nil {
		return fmt.Errorf("build zebgp: %w", err)
	}
	if err := e.Wait(); err != nil {
		return fmt.Errorf("build zebgp: %w\n%s", err, e.Stderr())
	}
	if e.ExitCode() != 0 {
		return fmt.Errorf("build zebgp failed: %s", e.Stderr())
	}

	// Build zebgp-peer
	e = NewExec()
	e.SetDir(r.baseDir)
	if err := e.Run(ctx, []string{"go", "build", "-o", r.peerPath, "./test/cmd/zebgp-peer"}, map[string]string{"CGO_ENABLED": "0"}); err != nil {
		return fmt.Errorf("build zebgp-peer: %w", err)
	}
	if err := e.Wait(); err != nil {
		return fmt.Errorf("build zebgp-peer: %w\n%s", err, e.Stderr())
	}
	if e.ExitCode() != 0 {
		return fmt.Errorf("build zebgp-peer failed: %s", e.Stderr())
	}

	return nil
}

// Cleanup removes temporary files.
func (r *Runner) Cleanup() {
	if r.tmpDir != "" {
		_ = os.RemoveAll(r.tmpDir)
	}
}

// Run executes selected tests.
func (r *Runner) Run(ctx context.Context, opts *RunOptions) bool {
	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Println("No tests selected")
		return true
	}

	// Load timing cache
	_ = r.timing.Load()

	type result struct {
		record  *Record
		success bool
		output  string
	}

	results := make(chan result, len(selected))
	var wg sync.WaitGroup

	// Semaphore for concurrency limit
	sem := make(chan struct{}, opts.Parallel)

	for _, rec := range selected {
		wg.Add(1)
		go func(rec *Record) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			success, output := r.runTest(ctx, rec, opts)
			results <- result{record: rec, success: success, output: output}
		}(rec)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	allSuccess := true
	for res := range results {
		if res.success {
			res.record.State = StateSuccess
		} else {
			res.record.State = StateFail
			allSuccess = false
			if res.output != "" && (opts.Verbose || !opts.Quiet) {
				fmt.Printf("\n%s %s failed:\n%s\n", res.record.Nick, res.record.Name, res.output)
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

// runTest executes a single test.
func (r *Runner) runTest(ctx context.Context, rec *Record, opts *RunOptions) (bool, string) {
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
	if rec.Extra["bind"] == "ipv6" {
		peerArgs = append(peerArgs, "--ipv6")
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

	// Start zebgp (client)
	configPath, _ := rec.Conf["config"].(string)
	clientEnv := map[string]string{
		"zebgp_tcp_port": fmt.Sprintf("%d", rec.Port),
		"zebgp_tcp_bind": "",
	}
	if rec.IsAPI {
		clientEnv["zebgp_api_socketpath"] = filepath.Join(os.TempDir(), fmt.Sprintf("zebgp-test-%d.sock", rec.Port))
	}

	client := NewExec()
	if err := client.Run(testCtx, []string{r.zebgpPath, "server", configPath}, clientEnv); err != nil {
		return false, fmt.Sprintf("start client: %v", err)
	}
	defer client.Terminate()

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
		// Collect output before terminating for debugging
		peer.Terminate()
		client.Terminate()
		peer.Collect()
		client.Collect()
		return false, fmt.Sprintf("test timed out\npeer: %s\nclient: %s",
			peer.Stdout()+peer.Stderr(), client.Stdout()+client.Stderr())
	}
}

// writeExpectFile writes expected messages to a temp file.
func (r *Runner) writeExpectFile(rec *Record) (string, error) {
	f, err := os.CreateTemp("", "zebgp-test-*.expect")
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
