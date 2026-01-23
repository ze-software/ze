package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/test/syslog"
	"codeberg.org/thomas-mangin/ze/internal/tmpfs"
)

var logger = slogutil.Logger("runner")

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
	tests    *EncodingTests
	baseDir  string
	tmpDir   string
	zePath   string
	peerPath string
	display  *Display
	report   *Report
	colors   *Colors
}

// NewRunner creates a test runner.
func NewRunner(tests *EncodingTests, baseDir string) (*Runner, error) {
	tmpDir, err := os.MkdirTemp("", "ze-functional-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	colors := NewColors()
	return &Runner{
		tests:    tests,
		baseDir:  baseDir,
		tmpDir:   tmpDir,
		zePath:   filepath.Join(tmpDir, "ze"),
		peerPath: filepath.Join(tmpDir, "ze-peer"),
		colors:   colors,
		display:  NewDisplay(tests.Tests, colors),
		report:   NewReport(colors),
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

	// Build ze
	cmd := exec.CommandContext(ctx, "go", "build", "-o", r.zePath, "./cmd/ze") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.BuildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build ze: %w", err)
	}

	// Build ze-peer
	cmd = exec.CommandContext(ctx, "go", "build", "-o", r.peerPath, "./cmd/ze-peer") //nolint:gosec // paths from internal runner
	cmd.Dir = r.baseDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		r.display.BuildStatus(false, fmt.Errorf("%w: %s", err, output))
		return fmt.Errorf("build ze-peer: %w", err)
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

	// Set up Tmpfs temp directory if there are Tmpfs files (needed by both paths)
	var tmpfsCleanup func()
	if len(rec.TmpfsFiles) > 0 {
		v := tmpfs.New()
		for path, content := range rec.TmpfsFiles {
			v.AddFile(path, content)
		}
		tmpfsTempDir, cleanup, err := v.WriteToTemp()
		if err != nil {
			rec.Error = fmt.Errorf("write Tmpfs files: %w", err)
			return false
		}
		tmpfsCleanup = cleanup
		rec.TmpfsTempDir = tmpfsTempDir
	}
	if tmpfsCleanup != nil {
		defer tmpfsCleanup()
	}

	// Use new orchestration if RunCommands present
	if len(rec.RunCommands) > 0 {
		return r.runOrchestrated(ctx, rec, opts)
	}

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
		fmt.Sprintf("ze_bgp_tcp_port=%d", rec.Port),
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

	// Start test-syslog server if syslog patterns are expected
	var syslogSrv *syslog.Server
	if len(rec.ExpectSyslog) > 0 {
		syslogSrv = syslog.New(0)
		if err := syslogSrv.Start(testCtx); err != nil {
			_ = peerCmd.Process.Kill()
			rec.Error = fmt.Errorf("start syslog server: %w", err)
			return false
		}
		rec.SyslogPort = syslogSrv.Port()
		defer func() { _ = syslogSrv.Close() }()
	}

	// Start ze (client)
	configPath, _ := rec.Conf["config"].(string)

	// If config is in Tmpfs, use the Tmpfs temp directory path
	if rec.TmpfsTempDir != "" && configPath != "" {
		configBase := filepath.Base(configPath)
		if _, ok := rec.TmpfsFiles[configBase]; ok {
			configPath = filepath.Join(rec.TmpfsTempDir, configBase)
		}
	}

	// Add ze binary directory to PATH so child processes (like "ze bgp persist") can find it
	zeDir := filepath.Dir(r.zePath)
	existingPath := os.Getenv("PATH")
	clientEnv := append(os.Environ(),
		fmt.Sprintf("ze_bgp_tcp_port=%d", rec.Port),
		// NOTE: ze_bgp_tcp_bind removed - listeners now derived from peer LocalAddress
		fmt.Sprintf("ze_bgp_api_socketpath=%s", filepath.Join(os.TempDir(), fmt.Sprintf("ze-test-%d.sock", rec.Port))),
		fmt.Sprintf("PATH=%s:%s", zeDir, existingPath),
		"SLOG_LEVEL=DEBUG", // Enable debug logging for tracing
	)

	// Add test-specific environment variables
	clientEnv = append(clientEnv, rec.EnvVars...)

	// Add syslog destination if syslog server is running
	if syslogSrv != nil {
		clientEnv = append(clientEnv,
			"ze.log.bgp.backend=syslog",
			fmt.Sprintf("ze.log.bgp.destination=127.0.0.1:%d", syslogSrv.Port()),
		)
	}

	clientCmd := exec.CommandContext(testCtx, r.zePath, "bgp", "server", configPath) //nolint:gosec // test runner, paths from temp dir
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
		// Validate JSON expectations if raw check passed
		if jsonErr := r.validateJSON(rec); jsonErr != nil {
			rec.Error = jsonErr
			rec.FailureType = FailTypeJSONMismatch
			return false
		}

		// Validate logging expectations
		if logErr := r.validateLogging(rec, clientStderr.String(), syslogSrv); logErr != nil {
			rec.Error = logErr
			rec.FailureType = FailTypeLoggingMismatch
			return false
		}

		return true
	}

	// Determine failure type
	switch {
	case strings.Contains(rec.PeerOutput, FailTypeMismatch):
		rec.FailureType = FailTypeMismatch
		rec.LastExpectedIdx, rec.LastReceivedIdx = extractMismatchIndices(rec.PeerOutput)
	case strings.Contains(rec.PeerOutput, "connection refused"):
		rec.FailureType = FailTypeConnectionRefuse
	default:
		rec.FailureType = stateUnknown
	}

	if err != nil {
		rec.Error = err
	}
	return false
}

// runOrchestrated executes a test using the new stdin/cmd orchestration format.
func (r *Runner) runOrchestrated(ctx context.Context, rec *Record, opts *RunOptions) bool {
	// Sort RunCommands by seq
	cmds := make([]RunCommand, len(rec.RunCommands))
	copy(cmds, rec.RunCommands)
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Seq < cmds[j].Seq
	})

	// Determine timeout from foreground command or default
	timeout := opts.Timeout
	for _, cmd := range cmds {
		if cmd.Mode == "foreground" && cmd.Timeout != "" {
			if d, err := time.ParseDuration(cmd.Timeout); err == nil {
				timeout = d
			}
		}
	}

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rec.State = StateRunning

	// Track background processes for cleanup
	var bgProcs []*exec.Cmd
	defer func() {
		for _, p := range bgProcs {
			if p.Process != nil {
				_ = p.Process.Kill()
			}
		}
	}()

	var peerStdout, peerStderr strings.Builder
	var clientStdout, clientStderr strings.Builder

	// Execute commands in order
	for _, cmd := range cmds {
		// Expand $PORT in exec string
		execStr := strings.ReplaceAll(cmd.Exec, "$PORT", fmt.Sprintf("%d", rec.Port))

		// Parse command and args
		cmdParts := strings.Fields(execStr)
		if len(cmdParts) == 0 {
			rec.Error = fmt.Errorf("empty exec command")
			return false
		}

		// Resolve binary path
		binName := cmdParts[0]
		var binPath string
		switch binName {
		case "ze-peer":
			binPath = r.peerPath
		case "ze":
			binPath = r.zePath
		default:
			binPath = binName // Use as-is for other commands
		}

		args := cmdParts[1:]

		// Handle stdin block content
		var stdinContent []byte
		if cmd.Stdin != "" {
			var ok bool
			stdinContent, ok = rec.StdinBlocks[cmd.Stdin]
			if !ok {
				rec.Error = fmt.Errorf("stdin block %q not found", cmd.Stdin)
				return false
			}
		}

		// ze-peer reads from file argument, not stdin.
		// Write stdin content to temp file and pass as argument.
		if binName == "ze-peer" && stdinContent != nil {
			tmpFile, err := os.CreateTemp("", "ze-peer-expect-*.msg")
			if err != nil {
				rec.Error = fmt.Errorf("create temp file for peer: %w", err)
				return false
			}
			logger.Debug("writing peer expect file", "path", tmpFile.Name(), "size", len(stdinContent), "content", string(stdinContent))
			defer func(name string) { _ = os.Remove(name) }(tmpFile.Name())
			if _, err := tmpFile.Write(stdinContent); err != nil {
				_ = tmpFile.Close()
				rec.Error = fmt.Errorf("write peer expect file: %w", err)
				return false
			}
			if err := tmpFile.Close(); err != nil {
				rec.Error = fmt.Errorf("close peer expect file: %w", err)
				return false
			}
			args = append(args, tmpFile.Name())
			stdinContent = nil // Don't pipe to stdin
		}

		// ze bgp server reads config from file, not stdin.
		// If args contain "-", replace with temp file.
		// Write to TmpfsTempDir if available so plugin paths (like ./plugin.run) resolve correctly.
		if binName == "ze" && stdinContent != nil {
			for i, arg := range args {
				if arg == "-" {
					var tmpFile *os.File
					var err error
					if rec.TmpfsTempDir != "" {
						// Write config to tmpfs dir so relative plugin paths work
						configPath := filepath.Join(rec.TmpfsTempDir, "ze-bgp.conf")
						tmpFile, err = os.Create(configPath) //nolint:gosec // test runner, path from temp dir
					} else {
						tmpFile, err = os.CreateTemp("", "ze-config-*.conf")
					}
					if err != nil {
						rec.Error = fmt.Errorf("create temp config file: %w", err)
						return false
					}
					if rec.TmpfsTempDir == "" {
						// Only cleanup if not in tmpfs dir (tmpfs cleanup handles it)
						defer func(name string) { _ = os.Remove(name) }(tmpFile.Name())
					}
					if _, err := tmpFile.Write(stdinContent); err != nil {
						_ = tmpFile.Close()
						rec.Error = fmt.Errorf("write config file: %w", err)
						return false
					}
					if err := tmpFile.Close(); err != nil {
						rec.Error = fmt.Errorf("close config file: %w", err)
						return false
					}
					args[i] = tmpFile.Name()
					stdinContent = nil // Don't pipe to stdin
					break
				}
			}
		}

		logger.Debug("executing command", "mode", cmd.Mode, "binary", binPath, "args", args)

		// Create command
		proc := exec.CommandContext(testCtx, binPath, args...) //nolint:gosec // test runner

		// Set up environment
		// Add ze binary directory to PATH so child processes can find "ze bgp plugin ..." commands
		zeDir := filepath.Dir(r.zePath)
		existingPath := os.Getenv("PATH")
		proc.Env = append(os.Environ(),
			fmt.Sprintf("ze_bgp_tcp_port=%d", rec.Port),
			fmt.Sprintf("ze_bgp_api_socketpath=%s", filepath.Join(os.TempDir(), fmt.Sprintf("ze-test-%d.sock", rec.Port))),
			fmt.Sprintf("PYTHONPATH=%s", filepath.Join(r.baseDir, "test/scripts")),
			fmt.Sprintf("PATH=%s:%s", zeDir, existingPath),
		)

		// Set working directory to tmpfs temp dir if available (for finding tmpfs files)
		if rec.TmpfsTempDir != "" {
			proc.Dir = rec.TmpfsTempDir
		}

		// Set up stdin if specified (for ze and other commands)
		if stdinContent != nil {
			proc.Stdin = strings.NewReader(string(stdinContent))
		}

		// Capture output
		if strings.Contains(execStr, "ze-peer") {
			proc.Stdout = &peerStdout
			proc.Stderr = &peerStderr
		} else {
			proc.Stdout = &clientStdout
			proc.Stderr = &clientStderr
		}

		// Start the process
		if err := proc.Start(); err != nil {
			rec.Error = fmt.Errorf("start %s: %w", binName, err)
			return false
		}

		if cmd.Mode == "background" {
			bgProcs = append(bgProcs, proc)
			// Give background process time to start
			time.Sleep(100 * time.Millisecond)
		} else {
			// Foreground (daemon): start but don't wait - we wait for peer instead
			bgProcs = append(bgProcs, proc) // Track for cleanup
		}
	}

	// Wait for peer (first background process, typically ze-peer) to finish.
	// The peer validates messages and exits when done (success/fail/timeout).
	// Daemons (foreground) run until killed.
	var peerProc *exec.Cmd
	for _, p := range bgProcs {
		// Find the peer process (the one with ze-peer args)
		if strings.Contains(p.Path, "ze-peer") || strings.Contains(p.String(), "ze-peer") {
			peerProc = p
			break
		}
	}

	var err error
	if peerProc != nil {
		// Wait for peer to finish
		peerDone := make(chan error, 1)
		go func() {
			peerDone <- peerProc.Wait()
		}()

		// Kill processes on context cancellation
		go func() {
			<-testCtx.Done()
			for _, p := range bgProcs {
				if p.Process != nil {
					_ = p.Process.Kill()
				}
			}
		}()

		err = <-peerDone
	}

	// Gracefully stop remaining processes (daemons)
	for _, p := range bgProcs {
		if p != peerProc && p.Process != nil {
			_ = p.Process.Signal(syscall.SIGTERM)
			done := make(chan struct{})
			go func(proc *exec.Cmd) {
				_ = proc.Wait()
				close(done)
			}(p)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				_ = p.Process.Kill()
				<-done
			}
		}
	}

	rec.PeerOutput = peerStdout.String() + peerStderr.String()
	rec.ClientOutput = clientStdout.String() + clientStderr.String()
	rec.Duration = time.Since(rec.StartTime)
	logger.Debug("collected output", "peerOutput", rec.PeerOutput, "clientOutput", rec.ClientOutput)

	// Parse received messages
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

	// Check for timeout
	if testCtx.Err() != nil {
		rec.State = StateTimeout
		rec.FailureType = stateTimeout
		return false
	}

	// Check for success
	if err == nil && strings.Contains(rec.PeerOutput, "successful") {
		// Validate JSON expectations
		if jsonErr := r.validateJSON(rec); jsonErr != nil {
			rec.Error = jsonErr
			rec.FailureType = FailTypeJSONMismatch
			return false
		}
		return true
	}

	// Determine failure type
	switch {
	case strings.Contains(rec.PeerOutput, FailTypeMismatch):
		rec.FailureType = FailTypeMismatch
		rec.LastExpectedIdx, rec.LastReceivedIdx = extractMismatchIndices(rec.PeerOutput)
	case strings.Contains(rec.PeerOutput, "connection refused"):
		rec.FailureType = FailTypeConnectionRefuse
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
	f, err := os.CreateTemp("", "ze-functional-*.expect")
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

// validateJSON validates JSON expectations against decoded messages.
// Returns nil if all validations pass or no JSON expectations exist.
// Skips tests with ExaBGP envelope format JSON (contains "exabgp" key).
// Matches by NLRI content, not position (ZeBGP may send routes in different order).
func (r *Runner) validateJSON(rec *Record) error {
	// Build cache of decoded received messages
	type decodedMsg struct {
		envelope map[string]any
		actual   map[string]any
		family   string
		nlris    []string // for content matching
		action   string   // "add" or "del"
		used     bool     // track if already matched
	}
	decoded := make([]*decodedMsg, 0, len(rec.ReceivedRaw))

	for _, rawHex := range rec.ReceivedRaw {
		envelope, err := r.decodeToEnvelope(rawHex)
		if err != nil {
			continue // Skip unparseable messages
		}
		family := extractFamily(envelope)
		actual, _ := transformEnvelopeToPlugin(envelope)
		nlris := extractNLRIs(actual)
		action := extractAction(actual)
		decoded = append(decoded, &decodedMsg{envelope, actual, family, nlris, action, false})
	}

	// Find messages with JSON expectations
	for _, msg := range rec.Messages {
		if msg.JSON == "" {
			continue // No JSON expectation
		}

		// Check if JSON is in ExaBGP envelope format (contains "exabgp" key)
		if strings.Contains(msg.JSON, `"exabgp"`) {
			continue // Skip ExaBGP envelope format (not plugin format)
		}

		// Parse expected JSON to extract NLRIs and action for matching
		var expectedMap map[string]any
		if err := json.Unmarshal([]byte(msg.JSON), &expectedMap); err != nil {
			return fmt.Errorf("message %d: invalid expected JSON: %w", msg.Index, err)
		}
		expectedNLRIs := extractNLRIs(expectedMap)
		expectedAction := extractAction(expectedMap)

		if len(expectedNLRIs) == 0 {
			continue // No NLRI to match (e.g., EOR)
		}

		// Find received message with matching NLRI and action (not already used)
		found := false
		for _, dm := range decoded {
			if dm.used {
				continue // Already matched to another expected
			}
			if dm.family != "" && !isSupportedFamily(dm.family) {
				continue // Skip unsupported families
			}
			if nlrisMatch(expectedNLRIs, dm.nlris) && dm.action == expectedAction {
				// Compare full JSON
				if err := comparePluginJSON(dm.actual, msg.JSON); err != nil {
					return fmt.Errorf("message %d: %w", msg.Index, err)
				}
				dm.used = true
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("message %d: no received message with NLRI %v action %s", msg.Index, expectedNLRIs, expectedAction)
		}
	}

	return nil
}

// extractNLRIs extracts NLRI identifiers from plugin format JSON for content matching.
// For unicast: extracts prefix strings.
// For FlowSpec: extracts the "string" field from the nlri object (human-readable rule).
func extractNLRIs(m map[string]any) []string {
	var nlris []string
	families := []string{
		"ipv4/unicast", "ipv6/unicast", "ipv4 unicast", "ipv6 unicast",
		"ipv4/flowspec", "ipv6/flowspec", "ipv4 flowspec", "ipv6 flowspec",
	}
	for _, fam := range families {
		if arr, ok := m[fam].([]any); ok {
			for _, item := range arr {
				if entry, ok := item.(map[string]any); ok {
					nlris = append(nlris, extractNLRIFromEntry(entry)...)
				}
			}
		}
		// Also handle []map[string]any from transformAnnounce/transformFlowspecAnnounce
		if arr, ok := m[fam].([]map[string]any); ok {
			for _, entry := range arr {
				nlris = append(nlris, extractNLRIFromEntry(entry)...)
			}
		}
	}
	return nlris
}

// extractAction extracts the action (add/del) from plugin format JSON.
func extractAction(m map[string]any) string {
	families := []string{
		"ipv4/unicast", "ipv6/unicast", "ipv4 unicast", "ipv6 unicast",
		"ipv4/flowspec", "ipv6/flowspec", "ipv4 flowspec", "ipv6 flowspec",
	}
	for _, fam := range families {
		if arr, ok := m[fam].([]any); ok {
			for _, item := range arr {
				if entry, ok := item.(map[string]any); ok {
					if action, ok := entry["action"].(string); ok {
						return action
					}
				}
			}
		}
		if arr, ok := m[fam].([]map[string]any); ok {
			for _, entry := range arr {
				if action, ok := entry["action"].(string); ok {
					return action
				}
			}
		}
	}
	return ""
}

// extractNLRIFromEntry extracts NLRI identifiers from an entry map.
// For unicast: entry["nlri"] is []string of prefixes.
// For FlowSpec: entry["nlri"] is map with "string" field containing human-readable rule.
func extractNLRIFromEntry(entry map[string]any) []string {
	var nlris []string
	// Handle []any (from JSON unmarshal) - unicast format
	if nlriArr, ok := entry["nlri"].([]any); ok {
		for _, n := range nlriArr {
			if s, ok := n.(string); ok {
				nlris = append(nlris, s)
			}
		}
	}
	// Handle []string (from transformAnnounce) - unicast format
	if nlriArr, ok := entry["nlri"].([]string); ok {
		nlris = append(nlris, nlriArr...)
	}
	// Handle map[string]any (from transformFlowspecAnnounce/Withdraw) - FlowSpec format
	// Use the "string" field as the NLRI identifier for matching
	if nlriMap, ok := entry["nlri"].(map[string]any); ok {
		if s, ok := nlriMap["string"].(string); ok {
			nlris = append(nlris, s)
		}
	}
	return nlris
}

// nlrisMatch returns true if expected and actual NLRI lists have the same prefixes.
func nlrisMatch(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	// Sort both for comparison
	e := make([]string, len(expected))
	a := make([]string, len(actual))
	copy(e, expected)
	copy(a, actual)
	sort.Strings(e)
	sort.Strings(a)
	for i := range e {
		if e[i] != a[i] {
			return false
		}
	}
	return true
}

// validateLogging validates logging expectations against stderr and syslog output.
// Returns nil if all validations pass or no logging expectations exist.
func (r *Runner) validateLogging(rec *Record, stderr string, syslogSrv *syslog.Server) error {
	// Check expected stderr patterns
	for _, pattern := range rec.ExpectStderr {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid expect:stderr pattern %q: %w", pattern, err)
		}
		if !re.MatchString(stderr) {
			return fmt.Errorf("expect:stderr pattern not found: %s", pattern)
		}
	}

	// Check rejected stderr patterns
	for _, pattern := range rec.RejectStderr {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid reject:stderr pattern %q: %w", pattern, err)
		}
		if re.MatchString(stderr) {
			return fmt.Errorf("reject:stderr pattern found: %s", pattern)
		}
	}

	// Check expected syslog patterns
	if syslogSrv != nil {
		for _, pattern := range rec.ExpectSyslog {
			if !syslogSrv.Match(pattern) {
				return fmt.Errorf("expect:syslog pattern not found: %s", pattern)
			}
		}
	}

	return nil
}

// decodeToEnvelope decodes a hex message using ze bgp decode and returns the envelope.
func (r *Runner) decodeToEnvelope(hexMsg string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.zePath, "bgp", "decode", "--update", hexMsg) //nolint:gosec // test runner
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ze bgp decode: %w: %s", err, string(output))
	}

	var envelope map[string]any
	if err := json.Unmarshal(output, &envelope); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return envelope, nil
}
