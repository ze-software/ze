// Design: docs/architecture/testing/ci-format.md — test execution and process orchestration
// Overview: runner.go — Runner struct, Build, Run lifecycle
// Related: runner_validate.go — post-execution result validation
// Related: runner_output.go — output capture and saving

package runner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/test/syslog"
	"codeberg.org/thomas-mangin/ze/internal/test/tmpfs"
)

const modeForeground = "foreground"

// syncWriter is an io.Writer that captures output and supports waiting for patterns.
// Used to wait for ze-peer's "listening on" message before starting the client.
type syncWriter struct {
	mu      sync.Mutex
	buf     strings.Builder
	pattern string
	found   bool
}

// peerListeningPattern is the string ze-peer prints to stdout when ready.
const peerListeningPattern = "listening on"

// newSyncWriter creates a writer that waits for ze-peer's "listening on" output.
func newSyncWriter() *syncWriter {
	return &syncWriter{pattern: peerListeningPattern}
}

// maxOutputBytes caps captured output to prevent OOM from runaway processes.
const maxOutputBytes = 10 << 20 // 10 MB

// Write captures data and checks for the pattern.
// Output is capped at maxOutputBytes to prevent unbounded memory growth.
func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.buf.Len() < maxOutputBytes {
		remaining := maxOutputBytes - sw.buf.Len()
		if len(p) > remaining {
			p = p[:remaining]
		}
		sw.buf.Write(p) //nolint:errcheck // strings.Builder.Write never fails
	}
	if !sw.found && strings.Contains(sw.buf.String(), sw.pattern) {
		sw.found = true
	}
	return len(p), nil
}

// WaitFor waits until the pattern is found or context is canceled.
// Returns true if pattern was found, false on timeout/cancel.
func (sw *syncWriter) WaitFor(ctx context.Context) bool {
	// Poll with small intervals to support context cancellation.
	// Using sync.Cond with context is tricky; polling is simpler and reliable.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		sw.mu.Lock()
		found := sw.found
		sw.mu.Unlock()

		if found {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			// Continue polling
		}
	}
}

// String returns all captured output.
func (sw *syncWriter) String() string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.String()
}

// peerOutput tracks stdout/stderr for a single ze-peer background process.
// Multi-peer tests create one per ze-peer so each has independent WaitFor
// synchronization and output capture.
type peerOutput struct {
	stdout *syncWriter
	stderr *strings.Builder
	proc   *exec.Cmd
}

// waitReady polls for a readiness file, returning when found or timeout expires.
// Used to synchronize daemon.pid creation with foreground process initialization:
// the process writes the readiness file after registering signal handlers, and
// the test runner writes daemon.pid only after the readiness file appears.
func waitReady(ctx context.Context, path string, timeout time.Duration) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case <-waitCtx.Done():
			return
		case <-ticker.C:
		}
	}
}

// runTest executes a single test.
func (r *Runner) runTest(ctx context.Context, rec *Record, opts *RunOptions) bool {
	// option=skip-os matched the current GOOS at parse time: report SKIP
	// without touching any subprocess or port. The feature under test is
	// stubbed on this platform (see rules/os-specific-tests.md); running
	// it would produce a meaningless failure.
	if rec.SkipReason != "" {
		rec.State = StateSkip
		return true
	}

	rec.State = StateStarting
	rec.StartTime = time.Now()

	// Set up Tmpfs temp directory if there are Tmpfs files (needed by both paths)
	var tmpfsCleanup func()
	if len(rec.TmpfsFiles) > 0 {
		v := tmpfs.New()
		for path, content := range rec.TmpfsFiles {
			// Expand $PORT2 before $PORT in tmpfs content (scripts, configs)
			s := string(content)
			s = strings.ReplaceAll(s, "$PORT2", fmt.Sprintf("%d", rec.Port+1))
			s = strings.ReplaceAll(s, "$PORT", fmt.Sprintf("%d", rec.Port))
			v.AddFile(path, []byte(s))
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

	// Determine timeout: explicit .ci override > baseline-derived > global default.
	// Baseline-derived = min(global, max(5s, 5x avg)) — catches hangs faster.
	timeout := r.timings.SuggestedTimeout(r.display.label, rec.Name, opts.Timeout)
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

	// Build peer args (ze-test peer ...)
	peerArgs := []string{"peer", "--port", fmt.Sprintf("%d", rec.Port)}
	if asn, ok := rec.Extra["asn"]; ok {
		peerArgs = append(peerArgs, "--asn", asn)
	}
	if rec.Extra["bind"] == "ipv6" {
		peerArgs = append(peerArgs, "--ipv6")
	}
	peerArgs = append(peerArgs, expectFile)

	// Start peer (server)
	peerEnv := append(os.Environ(),
		fmt.Sprintf("ze_test_bgp_port=%d", rec.Port),
	)
	peerCmd := exec.CommandContext(testCtx, r.testPath, peerArgs...) //nolint:gosec // test runner, paths from temp dir
	peerCmd.Env = peerEnv

	// Use syncWriter to wait for "listening on" before starting client
	peerStdout := newSyncWriter()
	peerStderr := &strings.Builder{}
	peerCmd.Stdout = peerStdout
	peerCmd.Stderr = peerStderr

	if err := peerCmd.Start(); err != nil {
		rec.Error = fmt.Errorf("start peer: %w", err)
		return false
	}

	// Wait for peer to be ready (listening) instead of fixed sleep
	// Use a short timeout context to avoid hanging forever if peer fails to start
	waitCtx, waitCancel := context.WithTimeout(testCtx, 5*time.Second)
	if !peerStdout.WaitFor(waitCtx) {
		waitCancel()
		_ = peerCmd.Process.Kill()
		rec.Error = fmt.Errorf("peer did not start listening within 5s (stderr=%q, stdout=%q)", peerStderr.String(), peerStdout.String())
		return false
	}
	waitCancel()

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
		fmt.Sprintf("ze_test_bgp_port=%d", rec.Port),
		// NOTE: ze_bgp_tcp_bind removed - listeners now derived from peer LocalAddress
		fmt.Sprintf("PATH=%s:%s", zeDir, existingPath),
		"SLOG_LEVEL=DEBUG",            // Enable debug logging for tracing
		"ze_plugin_stage_timeout=10s", // Allow more time for plugin stage barriers under concurrent test load
	)

	// Add test-specific environment variables
	clientEnv = append(clientEnv, rec.EnvVars...)

	// Add syslog destination if syslog server is running.
	// These use the ze.log.backend / ze.log.destination convention from
	// internal/core/slogutil/slogutil.go. Older code used ze.log.bgp.*
	// which is not registered and was silently ignored.
	if syslogSrv != nil {
		clientEnv = append(clientEnv,
			"ze.log.backend=syslog",
			fmt.Sprintf("ze.log.destination=127.0.0.1:%d", syslogSrv.Port()),
		)
	}

	clientCmd := exec.CommandContext(testCtx, r.zePath, configPath) //nolint:gosec // test runner, paths from temp dir
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

	// Wait for peer to finish.
	// exec.CommandContext auto-kills on context cancellation (Go 1.20+).
	err = peerCmd.Wait()

	// Gracefully stop client - SIGTERM first, force kill after timeout
	terminateGracefully(clientCmd, 2*time.Second)

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
		if saveErr := r.saveTestOutput(rec, out, opts.SaveDir); saveErr != nil {
			logger().Warn("save test output failed", "nick", rec.Nick, "error", saveErr)
		}
	}

	// Observer sentinel takes precedence over every other outcome: if an
	// observer plugin called ze_api.runtime_fail(), the sentinel in stderr is
	// the authoritative failure reason regardless of whether ze itself
	// timed out, exited cleanly, or reported peer mismatches. Without this
	// check, a slow daemon shutdown after runtime_fail would be reported as a
	// generic "timeout" and lose the actual cause.
	if sentinelErr := checkObserverSentinel(clientStderr.String()); sentinelErr != nil {
		rec.Error = sentinelErr
		rec.FailureType = FailTypeLoggingMismatch
		return false
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

	// Determine timeout: explicit foreground cmd > baseline-derived > global default.
	timeout := r.timings.SuggestedTimeout(r.display.label, rec.Name, opts.Timeout)
	for _, cmd := range cmds {
		if cmd.Mode == modeForeground && cmd.Timeout != "" {
			if d, err := time.ParseDuration(cmd.Timeout); err == nil {
				timeout = d
			}
		}
	}

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rec.State = StateRunning

	// Start test-syslog server if syslog patterns are expected.
	// Mirrors the setup in the non-orchestrated path: bound ze process env
	// gets ze.log.backend=syslog and ze.log.destination=<host:port>.
	var syslogSrv *syslog.Server
	if len(rec.ExpectSyslog) > 0 {
		syslogSrv = syslog.New(0)
		if err := syslogSrv.Start(testCtx); err != nil {
			rec.Error = fmt.Errorf("start syslog server: %w", err)
			return false
		}
		rec.SyslogPort = syslogSrv.Port()
		defer func() { _ = syslogSrv.Close() }()
	}

	// Track background processes for cleanup
	var bgProcs []*exec.Cmd
	defer func() {
		for _, p := range bgProcs {
			if p.Process != nil {
				_ = p.Process.Kill()
			}
		}
	}()

	// Per-process output tracking for ze-peer instances.
	// Each ze-peer gets its own syncWriter/stderr so WaitFor works independently.
	var peerOutputs []peerOutput
	var clientStdout, clientStderr strings.Builder

	// Track temp files for cleanup after loop (avoid defer in loop)
	var tmpFilesToClean []string
	defer func() {
		for _, name := range tmpFilesToClean {
			os.Remove(name) //nolint:errcheck // best-effort temp file cleanup
		}
	}()

	// Check if any command uses ze-peer (which provides BGP-level synchronization).
	hasPeer := false
	for _, cmd := range cmds {
		if strings.Contains(cmd.Exec, "ze-peer") {
			hasPeer = true
			break
		}
	}

	// Ensure loopback aliases exist for any ze-peer --bind addresses.
	// On Linux this is a no-op (127.0.0.0/8 routes to lo automatically).
	// On macOS/FreeBSD this adds aliases via SIOCAIFADDR ioctl on lo0.
	for _, cmd := range cmds {
		if !strings.Contains(cmd.Exec, "ze-peer") {
			continue
		}
		parts := strings.Fields(cmd.Exec)
		for i, p := range parts {
			if p == "--bind" && i+1 < len(parts) {
				if ip := net.ParseIP(parts[i+1]); ip != nil {
					if err := ensureLoopbackAlias(ip); err != nil {
						logger().Warn("loopback alias setup failed", "ip", ip, "error", err)
					}
				}
			}
		}
	}

	// Execute commands in order
	for cmdIdx, cmd := range cmds {
		// Expand $PORT2 before $PORT to avoid partial match ("$PORT2" contains "$PORT")
		execStr := strings.ReplaceAll(cmd.Exec, "$PORT2", fmt.Sprintf("%d", rec.Port+1))
		execStr = strings.ReplaceAll(execStr, "$PORT", fmt.Sprintf("%d", rec.Port))

		// Parse command and args
		cmdParts := strings.Fields(execStr)
		if len(cmdParts) == 0 {
			rec.Error = fmt.Errorf("empty exec command")
			return false
		}

		// Resolve binary path
		binName := cmdParts[0]
		var binPath string
		var extraArgs []string
		switch binName {
		case binNameZePeer:
			// ze-peer is now ze-test peer
			binPath = r.testPath
			extraArgs = []string{"peer"}
		case "ze-test":
			// ze-test subcommands (peeringdb, rpki, syslog, etc.)
			binPath = r.testPath
		case "ze":
			binPath = r.zePath
		default:
			// Check if the binary was built as an extra binary in the temp dir.
			tmpBin := filepath.Join(r.tmpDir, binName)
			if _, err := os.Stat(tmpBin); err == nil {
				binPath = tmpBin
			} else {
				binPath = binName // Use as-is (PATH lookup)
			}
		}

		args := make([]string, 0, len(extraArgs)+len(cmdParts)-1)
		args = append(args, extraArgs...)
		args = append(args, cmdParts[1:]...)

		// Handle stdin block content
		var stdinContent []byte
		if cmd.Stdin != "" {
			var ok bool
			stdinContent, ok = rec.StdinBlocks[cmd.Stdin]
			if !ok {
				rec.Error = fmt.Errorf("stdin block %q not found", cmd.Stdin)
				return false
			}
			// Expand $PORT2 before $PORT in stdin content (config files, scripts)
			s := string(stdinContent)
			s = strings.ReplaceAll(s, "$PORT2", fmt.Sprintf("%d", rec.Port+1))
			s = strings.ReplaceAll(s, "$PORT", fmt.Sprintf("%d", rec.Port))
			stdinContent = []byte(s)
		}

		// ze-peer reads from file argument, not stdin.
		// Write stdin content to temp file and pass as argument.
		if binName == binNameZePeer && stdinContent != nil {
			tmpFile, err := os.CreateTemp("", "ze-peer-expect-*.msg")
			if err != nil {
				rec.Error = fmt.Errorf("create temp file for peer: %w", err)
				return false
			}
			logger().Debug("writing peer expect file", "path", tmpFile.Name(), "size", len(stdinContent), "content", string(stdinContent))
			tmpFilesToClean = append(tmpFilesToClean, tmpFile.Name())
			if _, err := tmpFile.Write(stdinContent); err != nil {
				tmpFile.Close() //nolint:errcheck,gosec // best-effort close on write error
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

		// ze reads config from file, not stdin.
		// If args contain "-", replace with temp file.
		// Write to TmpfsTempDir if available so plugin paths (like ./plugin.run) resolve correctly.
		if binName == "ze" && stdinContent != nil {
			for i, arg := range args {
				if arg != "-" {
					continue
				}
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
					tmpFilesToClean = append(tmpFilesToClean, tmpFile.Name())
				}
				if _, err := tmpFile.Write(stdinContent); err != nil {
					tmpFile.Close() //nolint:errcheck,gosec // best-effort close on write error
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

		logger().Debug("executing command", "mode", cmd.Mode, "binary", binPath, "args", args)

		// Create command
		proc := exec.CommandContext(testCtx, binPath, args...) //nolint:gosec // test runner

		// Set up environment
		// Add ze binary directory to PATH so child processes can find "ze plugin ..." commands
		zeDir := filepath.Dir(r.zePath)
		existingPath := os.Getenv("PATH")
		proc.Env = append(os.Environ(),
			fmt.Sprintf("PYTHONPATH=%s", filepath.Join(r.baseDir, "test", "scripts")),
			fmt.Sprintf("PATH=%s:%s", zeDir, existingPath),
			"ze_plugin_stage_timeout=10s", // Allow more time for plugin stage barriers under concurrent test load
		)
		// Only set ze_test_bgp_port for ze and ze-peer binaries. Other processes
		// (e.g., ze-chaos --in-process) manage their own port configuration and
		// the override breaks their mock network setup.
		if binName == "ze" || binName == binNameZePeer {
			proc.Env = append(proc.Env, fmt.Sprintf("ze_test_bgp_port=%d", rec.Port))
		}
		// Point ze at the test syslog server when one was started. Uses the
		// ze.log.backend / ze.log.destination convention from slogutil.go.
		// Only applied to ze: ze-peer and helper scripts do not need it.
		if binName == "ze" && syslogSrv != nil {
			proc.Env = append(proc.Env,
				"ze.log.backend=syslog",
				fmt.Sprintf("ze.log.destination=127.0.0.1:%d", syslogSrv.Port()),
			)
		}
		// Add test-specific environment variables (option=env:var=KEY:value=VALUE)
		proc.Env = append(proc.Env, rec.EnvVars...)
		// Tell foreground ze processes to write a readiness file after signal
		// handlers are registered. The test runner waits for this file before
		// writing daemon.pid, eliminating a startup race condition.
		if cmd.Mode == modeForeground && binName == "ze" && rec.TmpfsTempDir != "" {
			proc.Env = append(proc.Env,
				fmt.Sprintf("ZE_READY_FILE=%s", filepath.Join(rec.TmpfsTempDir, "daemon.ready")))
		}

		// Set working directory to tmpfs temp dir if available (for finding tmpfs files)
		if rec.TmpfsTempDir != "" {
			proc.Dir = rec.TmpfsTempDir
		}

		// Set up stdin if specified (for ze and other commands)
		if stdinContent != nil {
			proc.Stdin = strings.NewReader(string(stdinContent))
		}

		// Capture output: each ze-peer gets its own syncWriter/stderr
		// so WaitFor works independently per process.
		if strings.Contains(execStr, "ze-peer") {
			po := peerOutput{
				stdout: newSyncWriter(),
				stderr: &strings.Builder{},
			}
			peerOutputs = append(peerOutputs, po)
			proc.Stdout = po.stdout
			proc.Stderr = po.stderr
		} else {
			proc.Stdout = &clientStdout
			proc.Stderr = &clientStderr
		}

		// Start the process. Retry on ETXTBSY which occurs when a concurrent
		// fork+exec in another test goroutine holds a write-open fd to the
		// script between fork and execve (see https://go.dev/issue/22315).
		var startErr error
		for attempt := range 3 {
			if attempt > 0 {
				time.Sleep(10 * time.Millisecond)
				// Go 1.25+ marks Cmd as started even on failure (go.dev/issue/77075),
				// so Start cannot be retried on the same Cmd. Create a fresh one.
				old := proc
				proc = exec.CommandContext(testCtx, binPath, args...) //nolint:gosec // test runner
				proc.Env = old.Env
				proc.Dir = old.Dir
				proc.Stdin = old.Stdin
				proc.Stdout = old.Stdout
				proc.Stderr = old.Stderr
			}
			startErr = proc.Start()
			if startErr == nil || !errors.Is(startErr, syscall.ETXTBSY) {
				break
			}
		}
		if startErr != nil {
			rec.Error = fmt.Errorf("start %s: %w", binName, startErr)
			return false
		}

		switch {
		case cmd.Mode == "background":
			bgProcs = append(bgProcs, proc)
			// Wait for ze-peer to be ready (listening) instead of fixed sleep
			// Skip waiting for peer if this is an exit code test (peer may not start)
			if strings.Contains(execStr, "ze-peer") && rec.ExpectExitCode == nil {
				po := &peerOutputs[len(peerOutputs)-1]
				po.proc = proc
				waitCtx, waitCancel := context.WithTimeout(testCtx, 5*time.Second)
				if !po.stdout.WaitFor(waitCtx) {
					waitCancel()
					rec.Error = fmt.Errorf("peer did not start listening within 5s (stderr=%q, stdout=%q)", po.stderr.String(), po.stdout.String())
					return false
				}
				waitCancel()
			} else if !strings.Contains(execStr, "ze-peer") {
				// Non-peer background process: brief sleep for startup
				time.Sleep(100 * time.Millisecond)
			}
		case cmd.Mode == modeForeground && binName != "ze" && binName != binNameZePeer && cmdIdx < len(cmds)-1:
			// Foreground setup script (non-daemon, e.g., create-marker.sh) that
			// precedes other commands: wait for completion before starting the
			// next command. Without this, the setup script may not finish before
			// ze reads its output, causing races under concurrent test load.
			// Only applies when followed by more commands; if it's the last
			// command, fall through to normal exit code handling.
			if err := proc.Wait(); err != nil {
				rec.Error = fmt.Errorf("setup script %s: %w", binName, err)
				return false
			}
			continue // Already finished, don't track for cleanup
		default:
			// Foreground daemon (ze): start but don't wait - we wait for peer instead
			bgProcs = append(bgProcs, proc) // Track for cleanup

			// Write daemon PID to tmpfs dir so background scripts can send signals.
			if rec.TmpfsTempDir != "" && proc.Process != nil {
				// When no ze-peer provides BGP-level synchronization, wait for
				// the process readiness file before writing daemon.pid. This
				// prevents a race where signal.sh sends SIGHUP before the
				// process has registered signal handlers.
				if !hasPeer {
					readyPath := filepath.Join(rec.TmpfsTempDir, "daemon.ready")
					waitReady(testCtx, readyPath, 5*time.Second)
				}
				pidPath := filepath.Join(rec.TmpfsTempDir, "daemon.pid")
				_ = os.WriteFile(pidPath, fmt.Appendf(nil, "%d", proc.Process.Pid), 0o600)
			}
		}
	}

	// Execute HTTP waits (readiness polls) before assertion checks.
	if len(rec.HTTPWaits) > 0 {
		if waitErr := r.executeHTTPWaits(testCtx, rec); waitErr != nil {
			rec.Error = waitErr
			rec.FailureType = "http_check_failed"
			rec.Duration = time.Since(rec.StartTime)
			return false
		}
	}

	// Execute HTTP checks (after background processes have started).
	if len(rec.HTTPChecks) > 0 {
		if httpErr := r.executeHTTPChecks(testCtx, rec); httpErr != nil {
			rec.Error = httpErr
			rec.FailureType = "http_check_failed"
			rec.Duration = time.Since(rec.StartTime)
			return false
		}
		// If HTTP checks are the only assertions (no peer, no exit code), pass.
		if len(rec.Expects) == 0 && rec.ExpectExitCode == nil {
			rec.Duration = time.Since(rec.StartTime)
			return true
		}
	}

	// Build set of peer processes for exclusion from graceful stop and fgProc detection.
	peerProcs := make(map[*exec.Cmd]bool, len(peerOutputs))
	for i := range peerOutputs {
		if peerOutputs[i].proc != nil {
			peerProcs[peerOutputs[i].proc] = true
		}
	}

	// Find foreground (daemon) process -- the last non-peer background process.
	// Uses peerProcs map for reliable detection since ze-peer is executed as
	// "ze-test peer ..." and p.Path/p.String() won't contain "ze-peer".
	var fgProc *exec.Cmd
	for _, p := range bgProcs {
		if !peerProcs[p] {
			fgProc = p
		}
	}

	// Wait for the signaling process to finish.
	// exec.CommandContext auto-kills on context cancellation (Go 1.20+).
	var err error

	if rec.ExpectExitCode != nil && fgProc != nil {
		// Testing exit code: wait for foreground process
		err = fgProc.Wait()
	} else {
		// Wait for all peer processes (each validates its own messages).
		// Daemons run until killed below. Collect all errors so no peer
		// failure is silently lost.
		var peerErrs []error
		for i := range peerOutputs {
			if peerOutputs[i].proc != nil {
				if waitErr := peerOutputs[i].proc.Wait(); waitErr != nil {
					peerErrs = append(peerErrs, waitErr)
				}
			}
		}
		err = errors.Join(peerErrs...)
	}

	// Gracefully stop remaining processes (daemons)
	for _, p := range bgProcs {
		if !peerProcs[p] && p.Process != nil {
			terminateGracefully(p, 2*time.Second)
		}
	}

	// Combine per-peer outputs (concatenated in process start order).
	var allPeerStdout, allPeerStderr strings.Builder
	for i := range peerOutputs {
		allPeerStdout.WriteString(peerOutputs[i].stdout.String())
		allPeerStderr.WriteString(peerOutputs[i].stderr.String())
	}
	rec.PeerOutput = allPeerStdout.String() + allPeerStderr.String()
	rec.ClientOutput = clientStdout.String() + clientStderr.String()
	rec.Duration = time.Since(rec.StartTime)
	logger().Debug("collected output", "peerOutput", rec.PeerOutput, "clientOutput", rec.ClientOutput)

	// Parse received messages
	rec.ReceivedRaw = extractReceivedMessages(rec.PeerOutput)

	// Save outputs if requested
	if opts.SaveDir != "" {
		out := &testOutput{
			peerStdout:   allPeerStdout.String(),
			peerStderr:   allPeerStderr.String(),
			clientStdout: clientStdout.String(),
			clientStderr: clientStderr.String(),
		}
		if saveErr := r.saveTestOutput(rec, out, opts.SaveDir); saveErr != nil {
			logger().Warn("save test output failed", "nick", rec.Nick, "error", saveErr)
		}
	}

	// Observer sentinel takes precedence in the orchestrated path too --
	// see the non-orchestrated branch above for rationale. Keep both paths
	// in sync: a runtime_fail from a python observer must surface as the
	// authoritative failure even when the daemon subsequently times out.
	if sentinelErr := checkObserverSentinel(clientStderr.String()); sentinelErr != nil {
		rec.Error = sentinelErr
		rec.FailureType = FailTypeLoggingMismatch
		return false
	}

	// Check for timeout
	if testCtx.Err() != nil {
		rec.State = StateTimeout
		rec.FailureType = stateTimeout
		return false
	}

	// If testing exit code, validate it
	if rec.ExpectExitCode != nil {
		expectedCode := *rec.ExpectExitCode
		actualCode := 0
		var exitErr *exec.ExitError
		if err != nil && errors.As(err, &exitErr) {
			actualCode = exitErr.ExitCode()
		}

		if actualCode != expectedCode {
			rec.Error = fmt.Errorf("expected exit code %d, got %d", expectedCode, actualCode)
			rec.FailureType = "exit_code_mismatch"
			return false
		}

		// Check stderr match if specified
		if rec.ExpectStderrMatch != "" {
			if !strings.Contains(rec.ClientOutput, rec.ExpectStderrMatch) {
				rec.Error = fmt.Errorf("stderr does not contain %q", rec.ExpectStderrMatch)
				rec.FailureType = "stderr_mismatch"
				return false
			}
		}

		// Check stdout matches if specified
		for _, expected := range rec.ExpectStdoutMatch {
			if !strings.Contains(rec.ClientOutput, expected) {
				rec.Error = fmt.Errorf("stdout does not contain %q", expected)
				rec.FailureType = "stdout_mismatch"
				return false
			}
		}

		// Validate logging expectations (expect/reject stderr patterns, expect syslog)
		if logErr := r.validateLogging(rec, clientStderr.String(), syslogSrv); logErr != nil {
			rec.Error = logErr
			rec.FailureType = FailTypeLoggingMismatch
			return false
		}

		return true
	}

	// Check for success
	if err == nil && strings.Contains(rec.PeerOutput, "successful") {
		// Validate JSON expectations
		if jsonErr := r.validateJSON(rec); jsonErr != nil {
			rec.Error = jsonErr
			rec.FailureType = FailTypeJSONMismatch
			return false
		}
		// Validate logging expectations (expect/reject stderr patterns, expect syslog)
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

// terminateGracefully sends SIGTERM to a process and waits for it to exit.
// If it doesn't exit within timeout, it is forcefully killed.
// No goroutines are spawned — time.AfterFunc handles the deadline.
func terminateGracefully(cmd *exec.Cmd, timeout time.Duration) {
	if cmd.Process == nil {
		_ = cmd.Wait()
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	timer := time.AfterFunc(timeout, func() {
		_ = cmd.Process.Kill()
	})
	_ = cmd.Wait()
	timer.Stop()
}
