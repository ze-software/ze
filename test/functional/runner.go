package functional

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

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
		Timeout:  15 * time.Second,
		Parallel: 0, // 0 = all tests in parallel
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
	display   *Display
	report    *Report
	colors    *Colors
}

// NewRunner creates a test runner.
func NewRunner(tests *EncodingTests, baseDir string) (*Runner, error) {
	tmpDir, err := os.MkdirTemp("", "zebgp-functional-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	colors := NewColors()
	return &Runner{
		tests:     tests,
		baseDir:   baseDir,
		tmpDir:    tmpDir,
		zebgpPath: filepath.Join(tmpDir, "zebgp"),
		peerPath:  filepath.Join(tmpDir, "zebgp-peer"),
		colors:    colors,
		display:   NewDisplay(tests.Tests, colors),
		report:    NewReport(colors),
	}, nil
}

// Cleanup removes temporary files.
func (r *Runner) Cleanup() {
	if r.tmpDir != "" {
		_ = os.RemoveAll(r.tmpDir)
	}
}

// Build compiles the test binaries.
func (r *Runner) Build(ctx context.Context) error {
	r.display.BuildStatus(true, nil)

	// Build zebgp
	cmd := exec.CommandContext(ctx, "go", "build", "-o", r.zebgpPath, "./cmd/zebgp") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.BuildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build zebgp: %w", err)
	}

	// Build zebgp-peer
	cmd = exec.CommandContext(ctx, "go", "build", "-o", r.peerPath, "./test/cmd/zebgp-peer") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.BuildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build zebgp-peer: %w", err)
	}

	r.display.BuildStatus(false, nil)
	return nil
}

// Run executes selected tests.
func (r *Runner) Run(ctx context.Context, opts *RunOptions) bool {
	r.display.SetQuiet(opts.Quiet)
	r.display.SetTimeout(opts.Timeout)

	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Println("No tests selected")
		return true
	}

	// Set parallel for batch display
	parallel := opts.Parallel
	if parallel <= 0 {
		parallel = len(selected)
	}
	r.display.SetParallel(parallel, len(selected))

	r.display.Start()

	type result struct {
		record  *Record
		success bool
	}

	results := make(chan result, len(selected))
	done := make(chan struct{})
	var wg sync.WaitGroup

	// Semaphore for concurrency limit
	sem := make(chan struct{}, parallel)

	for _, rec := range selected {
		wg.Add(1)
		go func(rec *Record) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			success := r.runTest(ctx, rec, opts)
			results <- result{record: rec, success: success}
		}(rec)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Periodic status update
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				r.display.Status()
			}
		}
	}()

	allSuccess := true
	for res := range results {
		if res.success {
			res.record.State = StateSuccess
		} else {
			if res.record.State != StateTimeout {
				res.record.State = StateFail
			}
			allSuccess = false
		}
		r.display.Status()
	}

	close(done)
	r.display.Newline()
	r.display.FinalStatus() // For non-TTY mode

	// Print failure reports
	if !allSuccess && !opts.Quiet {
		r.report.PrintAllFailures(r.tests.Tests)
	}

	return allSuccess
}

// RunWithCount runs each test count times for stress testing.
// Returns StressResult with stats, iteration timings, and overall success.
func (r *Runner) RunWithCount(ctx context.Context, opts *RunOptions, count int) *StressResult {
	stats := NewStressStats(r.tests.Tests)
	result := &StressResult{
		Stats:              stats,
		IterationDurations: make([]time.Duration, 0, count),
		AllPassed:          true,
	}

	totalStart := time.Now()

	// Create stress-mode options (suppress per-iteration failure reports)
	stressOpts := *opts
	stressOpts.Quiet = true // Suppress verbose output per iteration

	for i := 1; i <= count; i++ {
		// Check for cancellation before each iteration
		select {
		case <-ctx.Done():
			result.TotalDuration = time.Since(totalStart)
			result.AllPassed = false
			return result
		default:
		}

		iterStart := time.Now()

		if !opts.Quiet {
			fmt.Printf("\n%s Iteration %d/%d\n", r.colors.Cyan("==>"), i, count)
		}

		// Reset test states for this iteration
		for _, rec := range r.tests.Selected() {
			rec.State = StateNone
			rec.Error = nil
			rec.Duration = 0
		}

		// Run iteration (with quiet mode to suppress failure reports)
		success := r.Run(ctx, &stressOpts)

		iterDuration := time.Since(iterStart)
		result.IterationDurations = append(result.IterationDurations, iterDuration)

		if !opts.Quiet {
			fmt.Printf("%s Iteration %d: %s\n", r.colors.Cyan("==>"), i, formatDurationShort(iterDuration))
		}

		if !success {
			result.AllPassed = false
		}

		// Collect stats from this iteration (only terminal states)
		for _, rec := range r.tests.Selected() {
			if s, ok := stats[rec.Nick]; ok {
				// Only record terminal states
				if rec.State == StateSuccess || rec.State == StateFail || rec.State == StateTimeout {
					s.Add(rec.State, rec.Duration)
				}
			}
		}
	}

	result.TotalDuration = time.Since(totalStart)
	return result
}

// formatDurationShort formats a duration concisely.
func formatDurationShort(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// runTest executes a single test.
func (r *Runner) runTest(ctx context.Context, rec *Record, opts *RunOptions) bool {
	rec.State = StateStarting
	rec.StartTime = time.Now()

	// Determine timeout - per-test override or global default
	timeout := opts.Timeout
	if timeoutStr, ok := rec.Extra["timeout"]; ok {
		if d, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = d
		}
	}

	// Create test context with timeout
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Write expects to temp file
	expectFile, err := r.writeExpectFile(rec)
	if err != nil {
		rec.Error = fmt.Errorf("write expect file: %w", err)
		return false
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
	peerEnv := append(os.Environ(),
		fmt.Sprintf("zebgp_tcp_port=%d", rec.Port),
	)
	peerCmd := exec.CommandContext(testCtx, r.peerPath, peerArgs...) //nolint:gosec // test runner, paths from temp dir
	peerCmd.Env = peerEnv

	peerStdout := &strings.Builder{}
	peerStderr := &strings.Builder{}
	peerCmd.Stdout = peerStdout
	peerCmd.Stderr = peerStderr

	if err := peerCmd.Start(); err != nil {
		rec.Error = fmt.Errorf("start peer: %w", err)
		return false
	}

	// Wait for peer to start
	time.Sleep(100 * time.Millisecond)

	rec.State = StateRunning

	// Start zebgp (client)
	configPath, _ := rec.Conf["config"].(string)
	// Add zebgp binary directory to PATH so child processes (like "zebgp api persist") can find it
	zebgpDir := filepath.Dir(r.zebgpPath)
	existingPath := os.Getenv("PATH")
	clientEnv := append(os.Environ(),
		fmt.Sprintf("zebgp_tcp_port=%d", rec.Port),
		// NOTE: zebgp_tcp_bind removed - listeners now derived from peer LocalAddress
		fmt.Sprintf("zebgp_api_socketpath=%s", filepath.Join(os.TempDir(), fmt.Sprintf("zebgp-test-%d.sock", rec.Port))),
		fmt.Sprintf("PATH=%s:%s", zebgpDir, existingPath),
	)

	clientCmd := exec.CommandContext(testCtx, r.zebgpPath, "server", configPath) //nolint:gosec // test runner, paths from temp dir
	clientCmd.Env = clientEnv

	clientStdout := &strings.Builder{}
	clientStderr := &strings.Builder{}
	clientCmd.Stdout = clientStdout
	clientCmd.Stderr = clientStderr

	if err := clientCmd.Start(); err != nil {
		_ = peerCmd.Process.Kill()
		rec.Error = fmt.Errorf("start client: %w", err)
		return false
	}

	// Kill processes on context cancellation (exec.CommandContext + Start() doesn't auto-kill)
	go func() {
		<-testCtx.Done()
		if peerCmd.Process != nil {
			_ = peerCmd.Process.Kill()
		}
		if clientCmd.Process != nil {
			_ = clientCmd.Process.Kill()
		}
	}()

	// Wait for peer to finish
	peerDone := make(chan error, 1)
	go func() {
		peerDone <- peerCmd.Wait()
	}()

	// Wait for peer to finish (context kill goroutine handles timeout)
	err = <-peerDone

	// Gracefully stop client - send SIGTERM first to allow cleanup
	if clientCmd.Process != nil {
		_ = clientCmd.Process.Signal(syscall.SIGTERM)
		// Wait briefly for graceful shutdown, then force kill
		done := make(chan struct{})
		go func() {
			_ = clientCmd.Wait()
			close(done)
		}()
		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(2 * time.Second):
			// Force kill if still running
			_ = clientCmd.Process.Kill()
			<-done
		}
	} else {
		_ = clientCmd.Wait()
	}

	rec.PeerOutput = peerStdout.String() + peerStderr.String()
	rec.ClientOutput = clientStdout.String() + clientStderr.String()
	rec.Duration = time.Since(rec.StartTime)

	// Parse received messages from peer output
	rec.ReceivedRaw = extractReceivedMessages(rec.PeerOutput)

	// Save outputs if requested
	if opts.SaveDir != "" {
		out := &testOutput{
			peerStdout:   peerStdout.String(),
			peerStderr:   peerStderr.String(),
			clientStdout: clientStdout.String(),
			clientStderr: clientStderr.String(),
		}
		if err := r.saveTestOutput(rec, out, opts.SaveDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: save %s: %v\n", rec.Nick, err)
		}
	}

	// Check if we timed out
	if testCtx.Err() != nil {
		rec.State = StateTimeout
		rec.FailureType = stateTimeout
		return false
	}

	// Check for success
	if err == nil && strings.Contains(rec.PeerOutput, "successful") {
		return true
	}

	// Determine failure type
	switch {
	case strings.Contains(rec.PeerOutput, "mismatch"):
		rec.FailureType = "mismatch"
		rec.LastExpectedIdx, rec.LastReceivedIdx = extractMismatchIndices(rec.PeerOutput)
	case strings.Contains(rec.PeerOutput, "connection refused"):
		rec.FailureType = "connection_refused"
	default:
		rec.FailureType = stateUnknown
	}

	if err != nil {
		rec.Error = err
	}
	return false
}

// writeExpectFile writes expected messages to a temp file.
func (r *Runner) writeExpectFile(rec *Record) (string, error) {
	f, err := os.CreateTemp("", "zebgp-functional-*.expect")
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

// extractReceivedMessages parses peer output for received raw messages.
func extractReceivedMessages(output string) []string {
	var messages []string

	// Look for "msg  recv" lines followed by hex
	// Format: "msg  recv    FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:0029:02:..."
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "msg  recv") || strings.Contains(line, "msg recv") {
			// Extract hex after the prefix
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				hex := parts[len(parts)-1]
				// Clean up hex (remove colons)
				hex = strings.ReplaceAll(hex, ":", "")
				if len(hex) >= 38 { // Minimum BGP message
					messages = append(messages, hex)
				}
			}
		}
	}

	return messages
}

// extractMismatchIndices tries to find which message mismatched.
func extractMismatchIndices(output string) (expected, received int) {
	// Default to first message
	expected = 1
	received = 0

	// Try to parse "message N mismatch" patterns
	re := regexp.MustCompile(`message\s+(\d+)`)
	if matches := re.FindStringSubmatch(output); len(matches) > 1 {
		if n, err := fmt.Sscanf(matches[1], "%d", &expected); err == nil && n > 0 {
			received = expected - 1
		}
	}

	return
}

// testOutput holds captured output for saving.
type testOutput struct {
	peerStdout   string
	peerStderr   string
	clientStdout string
	clientStderr string
}

// sanitizeFilename removes/replaces characters unsafe for filenames.
func sanitizeFilename(name string) string {
	// Replace path separators and other unsafe chars with underscore
	result := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '_'
		default:
			return r
		}
	}, name)
	// Truncate if too long
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// saveTestOutput saves test outputs to files when SaveDir is set.
func (r *Runner) saveTestOutput(rec *Record, out *testOutput, saveDir string) error {
	if saveDir == "" {
		return nil
	}

	// Create test directory (nick-name for easy identification)
	dirName := fmt.Sprintf("%s-%s", rec.Nick, sanitizeFilename(rec.Name))
	testDir := filepath.Join(saveDir, dirName)
	if err := os.MkdirAll(testDir, 0o700); err != nil {
		return fmt.Errorf("create save dir: %w", err)
	}

	// Write output files
	files := map[string]string{
		"peer-stdout.log":   out.peerStdout,
		"peer-stderr.log":   out.peerStderr,
		"client-stdout.log": out.clientStdout,
		"client-stderr.log": out.clientStderr,
	}

	for name, content := range files {
		path := filepath.Join(testDir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	// Write expected.txt (from .ci file)
	var expected strings.Builder
	for _, exp := range rec.Expects {
		expected.WriteString(exp)
		expected.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(testDir, "expected.txt"), []byte(expected.String()), 0o600); err != nil {
		return fmt.Errorf("write expected.txt: %w", err)
	}

	// Write received.txt (from peer output)
	var received strings.Builder
	for _, raw := range rec.ReceivedRaw {
		received.WriteString(raw)
		received.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(testDir, "received.txt"), []byte(received.String()), 0o600); err != nil {
		return fmt.Errorf("write received.txt: %w", err)
	}

	return nil
}
