// Design: docs/architecture/testing/ci-format.md -- test execution and process orchestration
// Overview: runner.go -- Runner struct, Build, Run lifecycle
// Related: runner_exec_util.go -- syncWriter, daemon arg helpers, terminateGracefully
// Related: runner_validate.go -- post-execution result validation
// Related: runner_output.go -- output capture and saving

package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/sessionpath"
	"github.com/ze-software/ze/internal/test/syslog"
	"github.com/ze-software/ze/internal/test/tmpfs"
	"github.com/ze-software/ze/internal/test/trace"
)

// stepKindExpect is the trace.StepResult kind for an assertion step, and
// zeTestVerbPeer is the `ze-test peer` verb that drives the BGP peer.
const (
	stepKindExpect = "expect"
	zeTestVerbPeer = "peer"
)

// runTest executes a single test.
func (r *Runner) runTest(ctx context.Context, rec *Record, opts *RunOptions) bool {
	// A .ci file that failed to parse at discovery (see EncodingTests.Discover)
	// has no runnable commands. Report the parse error as a hard failure without
	// attempting execution, so one bad file fails the suite loudly rather than
	// aborting discovery of every other test.
	//
	// Checked BEFORE SkipReason on purpose: a malformed file's skip marker was
	// parsed from the same broken file, so it is not trustworthy evidence that
	// the file need not run. See the fuller argument at the real entry point,
	// parallel.go's per-test goroutine, which short-circuits before this function
	// is ever reached. This ordering governs only direct callers.
	if rec.ParseFailed {
		rec.State = StateFail
		return false
	}

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

	// Lease this test's port pair NOW, and hold the lease until the test is
	// done. Every consumer of rec.Port below this line reads the leased value:
	// the tmpfs $PORT/$PORT2 expansion, the ze-peer --port argument, the
	// ze_test_bgp_port child env, the option=env expansion, the exec strings in
	// runOrchestrated, and the http= URLs in runner_validate.go.
	//
	// The port a test binds used to be decided at DISCOVERY time from a counter
	// shared by every suite, minutes before the test ran, and nothing rechecked
	// it. That is what made "bind: address already in use" a suite-wide flake:
	// see LeaseTestPorts.
	portLease, err := LeaseTestPorts(rec.Port)
	if err != nil {
		rec.State = StateFail
		rec.Error = fmt.Errorf("lease ports for test: %w", err)
		return false
	}
	defer portLease.Release()
	rec.Port = portLease.Start

	// Every test runs its children in a directory of its own, whether or not it
	// declares tmpfs files. A child given no directory inherits the runner's,
	// which is the repository root `./le` runs from, and a ze daemon started
	// there writes database.zefs, daemon.log, its rendered config, its host keys
	// and its rollback/ and crash/ trees into the checkout. 1503 of 1789 .ci
	// files declare no tmpfs block, so that was the common case rather than the
	// rare one. See plan/journal/test-artifacts-land-in-the-repository-root.md.
	//
	// TmpfsTempDir keeps its narrower meaning and is still set only when the
	// record declares files. Consumers read it as that question: whether to arm
	// the daemon readiness file, where a sendfile lives, which directory a
	// file= check is rooted at. A directory that is always present would answer
	// a different question and change all three.
	workDir, err := os.MkdirTemp(sessionpath.DefaultScratchRoot(), workDirPrefix+"*")
	if err != nil {
		rec.Error = fmt.Errorf("create test work directory: %w", err)
		return false
	}
	defer func() { _ = os.RemoveAll(workDir) }()
	rec.WorkDir = workDir

	// Set up Tmpfs files in that same directory when the record declares any
	// (needed by both paths).
	if len(rec.TmpfsFiles) > 0 || len(rec.EngineSteps) > 0 {
		v := tmpfs.New()
		for path, content := range rec.TmpfsFiles {
			// Expand $PORT2 before $PORT in tmpfs content (scripts, configs)
			s := string(content)
			s = strings.ReplaceAll(s, "$PORT2", strconv.Itoa(rec.Port+1))
			s = strings.ReplaceAll(s, "$PORT", strconv.Itoa(rec.Port))
			v.AddFile(path, []byte(s))
		}
		if len(rec.EngineSteps) > 0 {
			// Contract with the .ci-declared external executor plugin:
			// run "ze-test engine-steps ./engine-steps.json" (engine_steps.go).
			stepsJSON, stepsErr := marshalEngineSteps(r.engineStepsForRun(rec.EngineSteps))
			if stepsErr != nil {
				rec.Error = fmt.Errorf("marshal engine steps: %w", stepsErr)
				return false
			}
			v.AddFile(EngineStepsFileName, stepsJSON)
		}
		if err := v.WriteTo(workDir); err != nil {
			rec.Error = fmt.Errorf("write Tmpfs files: %w", err)
			return false
		}
		rec.TmpfsTempDir = workDir
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
	// testBudget is the AUTHORED budget, before parallel headroom. The plugin
	// stall watchdog derives its window from it and applies the same headroom
	// itself, so handing it the already-scaled value would square it (the same
	// split runOrchestrated makes for the await fence).
	testBudget := timeout
	timeout = r.withParallelHeadroom(timeout)

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
	peerArgs := []string{zeTestVerbPeer, "--port", strconv.Itoa(rec.Port)}
	if asn, ok := rec.Extra["asn"]; ok {
		peerArgs = append(peerArgs, "--asn", asn)
	}
	if rec.Extra["bind"] == "ipv6" {
		peerArgs = append(peerArgs, "--ipv6")
	}
	peerArgs = append(peerArgs, expectFile)

	// Start peer (server)
	peerEnv := childEnv(
		textbuf.StrInt("ze_test_bgp_port=", int64(rec.Port)),
	)
	peerCmd := exec.CommandContext(testCtx, r.testPath, peerArgs...) //nolint:gosec // test runner, paths from temp dir
	peerCmd.Env = peerEnv
	// Every child runs in this test's own directory. See Record.WorkDir.
	peerCmd.Dir = rec.WorkDir

	// Use syncWriter to wait for "listening on" before starting client
	peerStdout := newSyncWriter()
	peerStderr := &strings.Builder{}
	peerCmd.Stdout = peerStdout
	peerCmd.Stderr = peerStderr

	if err := peerCmd.Start(); err != nil {
		rec.Error = fmt.Errorf("start peer: %w", err)
		return false
	}

	// Wait for peer to be ready (listening) instead of fixed sleep. Use a short
	// timeout context to avoid hanging forever if peer fails to start; scale it by
	// the parallel headroom (identity for serial runs) so a ze-peer slow to bind
	// under CPU oversubscription is not read as a spurious "never listening" -- the
	// same non-orchestrated-path deadline named in the "fixed startup deadlines"
	// entry archived in plan/known-failures/RESOLVED.md.
	peerBindTimeout := r.withParallelHeadroom(5 * time.Second)
	waitCtx, waitCancel := context.WithTimeout(testCtx, peerBindTimeout)
	if !peerStdout.waitFor(waitCtx) {
		waitCancel()
		_ = peerCmd.Process.Kill()
		rec.Error = fmt.Errorf("peer did not start listening within %s (stderr=%q, stdout=%q)", peerBindTimeout, peerStderr.String(), peerStdout.String())
		return false
	}
	waitCancel()

	rec.State = StateRunning

	// Start test-syslog server if syslog patterns are expected or rejected.
	// Both expect=syslog and reject=syslog need the capture server: a reject is
	// only meaningful when ze's logs are actually routed to and recorded by it.
	var syslogSrv *syslog.Server
	if len(rec.ExpectSyslog) > 0 || len(rec.RejectSyslog) > 0 {
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
	// parseOption builds the config path by joining the .ci file's directory,
	// and discovery walks a relative tree, so the result carries whatever
	// spelling the walk used. The child runs in this test's own directory now
	// rather than in the repository root, so anchor the path before handing it
	// over.
	if configPath != "" && !filepath.IsAbs(configPath) {
		configPath = filepath.Join(r.baseDir, configPath)
	}

	// Put the bare-name shim dir on PATH so child processes (like "ze bgp
	// persist") resolve THIS run's ze, not a stale or wrong-architecture one
	// left in bin/ (see Runner.setupBinShims).
	clientEnv := childEnv(
		textbuf.StrInt("ze_test_bgp_port=", int64(rec.Port)),
		// NOTE: ze_bgp_tcp_bind removed - listeners now derived from peer LocalAddress
		r.childPathEnv(),
		"ze.storage.blob=false",
		"SLOG_LEVEL=DEBUG", // Enable debug logging for tracing
		// Plugin startup stall watchdog. Derived from this test's own budget, not
		// a constant: see pluginStageStall (plugin_stage_stall.go).
		r.pluginStageStallEnv(testBudget),
	)

	// Add test-specific environment variables ($PORT/$PORT2 expand like exec
	// strings so per-test ports work in env knobs too).
	for _, kv := range rec.EnvVars {
		kv = strings.ReplaceAll(kv, "$PORT2", strconv.Itoa(rec.Port+1))
		kv = strings.ReplaceAll(kv, "$PORT", strconv.Itoa(rec.Port))
		clientEnv = append(clientEnv, kv)
	}

	// Add syslog destination if syslog server is running.
	// These use the ze.log.backend / ze.log.destination convention from
	// internal/core/slogutil/slogutil.go. Older code used ze.log.bgp.*
	// which is not registered and was silently ignored.
	if syslogSrv != nil {
		clientEnv = append(clientEnv,
			"ze.log.backend=syslog",
			textbuf.StrInt("ze.log.destination=127.0.0.1:", int64(syslogSrv.Port())),
		)
	}

	// Config-file daemon launch: keyword-first grammar places the config path
	// behind the `start` verb (spec-fixit-config-file-positional-grammar).
	// configPath here is always a real file (option=file:), never the stdin
	// sentinel, so it is safe to route through `ze start <config>`.
	clientArgs := []string{configPath}
	if configPath != "" {
		clientArgs = []string{zeVerbStart, configPath}
	}
	clientCmd := exec.CommandContext(testCtx, r.zePath, clientArgs...) //nolint:gosec // test runner, paths from temp dir
	clientCmd.Env = clientEnv
	clientCmd.Dir = rec.WorkDir

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
	terminateGracefully(clientCmd)

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
	if sentinelErr := observerSentinelInSyslog(syslogSrv); sentinelErr != nil {
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

	stepN := 0
	recStep := func(assert string, passed bool, detail string) {
		stepN++
		rec.StepTrace = append(rec.StepTrace, trace.StepResult{
			Step: stepN, Kind: stepKindExpect, Assert: assert,
			Passed: passed, Detail: detail,
		})
	}

	// Check for success
	if err == nil && strings.Contains(rec.PeerOutput, "successful") {
		recStep("peer-exchange", true, "")
		// Validate JSON expectations if raw check passed
		if jsonErr := r.validateJSON(rec); jsonErr != nil {
			rec.Error = jsonErr
			rec.FailureType = FailTypeJSONMismatch
			recStep("json-match", false, jsonErr.Error())
			return false
		}
		recStep("json-match", true, "")

		// Validate logging expectations
		if logErr := r.validateLogging(rec, clientStderr.String(), syslogSrv); logErr != nil {
			rec.Error = logErr
			rec.FailureType = FailTypeLoggingMismatch
			recStep("logging", false, logErr.Error())
			return false
		}
		if len(rec.ExpectStderr) > 0 || len(rec.RejectStderr) > 0 ||
			len(rec.ExpectSyslog) > 0 || len(rec.RejectSyslog) > 0 {
			recStep("logging", true, "")
		}
		if fileErr := r.validateFileChecks(rec); fileErr != nil {
			rec.Error = fileErr
			rec.FailureType = fileCheckFailed
			recStep("file-check", false, fileErr.Error())
			return false
		}
		if len(rec.FileChecks) > 0 {
			recStep("file-check", true, "")
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

	// Reclassify "unknown" as "near_timeout" when the test consumed most of
	// its budget. This distinguishes CPU-starvation near-misses from genuine
	// unknown failures so failure groups are actionable under load.
	if isNearTimeout(float64(rec.Duration)/float64(timeout), rec.FailureType) {
		rec.FailureType = FailTypeNearTimeout
	}

	if err != nil {
		rec.Error = err
	}
	recStep("peer-exchange", false, rec.FailureType)
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

	// Determine timeout, most specific wins: per-command timeout= on a foreground
	// cmd, then the record-level `option=timeout:value=`, then baseline-derived,
	// then the global default.
	//
	// The record-level option used to be read only on the non-orchestrated path
	// (see the sibling block in Run, which returns to this function before
	// reaching it). Any test declaring `option=timeout:` alongside a `cmd=`
	// directive therefore had its declaration silently ignored and ran on the
	// global default instead -- a stated timeout that did nothing.
	// testBudget is the AUTHORED budget, before parallel headroom. The
	// await=stderr fence derives its own default from it and then applies the
	// same headroom itself, so passing the already-scaled value would square it.
	testBudget := resolveOrchestratedTimeout(
		r.timings.SuggestedTimeout(r.display.label, rec.Name, opts.Timeout),
		rec.Extra["timeout"],
		cmds,
	)
	timeout := r.withParallelHeadroom(testBudget)

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rec.State = StateRunning

	// Fix B: opt-in per-test network-namespace isolation. When ZE_TEST_NETNS is
	// set (Linux only) this locks the goroutine's OS thread into a fresh netns
	// before spawning any child. ze, ze-peer, and compiled fixture helpers
	// fork-inherit it (A-5), so they share one throwaway namespace, reach each
	// other over 127.0.0.1, and
	// the nft firewall the daemon programs stays in that netns -- the host
	// firewall is untouched. The whole orchestration below (syslog socket, HTTP
	// checks) runs on this locked thread so it too reaches the daemon inside the
	// netns. All of it is gated on netnsMode, so the default path is unchanged.
	netnsMode := netnsModeActive()
	var netnsHostInode uint64
	netnsUID, netnsGID, netnsHasUID := 0, 0, false
	if netnsMode {
		netnsUID, netnsGID, netnsHasUID = netnsChildIDs()
		if !netnsHasUID {
			// ze must run as a normal user in the netns: its readiness file is
			// written after dropPrivileges, so a root ze never writes it and the
			// handshake times out (A-4). Fail loudly on a misconfigured netns run
			// rather than silently run ze as root.
			rec.Error = errNetnsNeedsUID
			rec.FailureType = stateUnknown
			return false
		}
		restore, hostInode, nsErr := enterTestNetns(testNetnsName(rec.Nick, rec.Port))
		if nsErr != nil {
			// AC-4: fail loudly rather than fall through and program the host
			// firewall. A netlink suite that cannot get its own namespace is a
			// setup error (missing CAP_SYS_ADMIN / not under sudo), not a pass.
			rec.Error = nsErr
			rec.FailureType = stateUnknown
			return false
		}
		defer restore()
		netnsHostInode = hostInode
		// Provision any interfaces the test declared (option=netns-link) inside the
		// fresh namespace, on this locked thread, before spawning ze. A policy
		// next-hop route needs a connected interface to resolve its gateway; without
		// this the daemon's RouteAdd fails "network is unreachable" and the test
		// asserts against a daemon that never reached its target state.
		if nlErr := provisionNetnsLinks(rec.NetnsLinks); nlErr != nil {
			rec.Error = nlErr
			rec.FailureType = stateUnknown
			return false
		}
		// The ze daemon is dropped to a normal user; make it own the directory it
		// runs in so it can chdir in, read config, and write daemon.ready (see
		// chownTree). The work directory rather than the tmpfs one: every child
		// runs there now, and a test that declares no tmpfs files still has a
		// daemon that must write into it.
		if netnsHasUID && rec.WorkDir != "" {
			if chErr := chownTree(rec.WorkDir, netnsUID, netnsGID); chErr != nil {
				rec.Error = fmt.Errorf("prepare work dir for netns child: %w", chErr)
				rec.FailureType = stateUnknown
				return false
			}
		}
	}

	// Start test-syslog server if syslog patterns are expected or rejected.
	// Mirrors the setup in the non-orchestrated path: bound ze process env
	// gets ze.log.backend=syslog and ze.log.destination=<host:port>.
	var syslogSrv *syslog.Server
	if len(rec.ExpectSyslog) > 0 || len(rec.RejectSyslog) > 0 {
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

	// Name -> tracked background process, populated when a cmd=background line
	// carries name=NAME. A later cmd=stop directive looks the process up here to
	// terminate it mid-test. A stopped process is removed from both bgProcs and
	// this map after it is reaped, so teardown never touches an already-dead
	// process and fgProc selection skips it (see the modeStop case below).
	namedBg := make(map[string]*exec.Cmd)

	// Per-process output tracking for ze-peer instances.
	// Each ze-peer gets its own syncWriter/stderr so WaitFor works independently.
	var peerOutputs []peerOutput
	// Every non-peer client process (the ze daemon plus each cmd=background
	// helper, mock server, or observer) shares these two accumulators, and
	// os/exec runs one copy goroutine per stream per process.
	// They MUST be concurrency-safe: a plain strings.Builder loses whole lines
	// when two processes write at once, which silently turns an
	// expect=stderr:pattern= into a spurious failure under load.
	var clientStdout, clientStderr lockedBuilder

	// await=stderr fence: when set, the daemon's relayed stderr is teed through
	// this syncWriter so the runner can block until it carries the needle before
	// teardown (see await_stderr.go). nil (the common case) leaves stderr
	// handling byte-for-byte unchanged.
	var awaitStderrSW *syncWriter
	if rec.AwaitStderr != "" {
		awaitStderrSW = newSyncWriterPattern(rec.AwaitStderr)
	}

	// Exit error of the last awaited quick-exit ze command (see awaitQuickZe),
	// fed into the expect=exit check when there is no daemon fgProc to wait on.
	var lastQuickZeErr error
	quickZeRan := false

	// Track temp files for cleanup after loop (avoid defer in loop)
	var tmpFilesToClean []string
	defer func() {
		for _, name := range tmpFilesToClean {
			os.RemoveAll(name) //nolint:errcheck,gosec // best-effort temp path cleanup
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

	// Make every address this fixture binds usable before anything starts: the
	// ones ze-peer takes from --bind, and the ones ze takes from the config the
	// .ci embeds (connection > local > ip).
	//
	// This FAILS the test. It used to log a warning and carry on, so a host
	// missing the address paid for it later as a bind failure or a whole-test
	// timeout, with nothing on the way naming the cause or the fix.
	if err := ensureBindAddresses(rec); err != nil {
		rec.Error = err
		rec.FailureType = FailTypeLoopbackMissing
		return false
	}

	// Execute commands in order
	for cmdIdx, cmd := range cmds {
		// A cmd=stop step terminates a named background process mid-test. It runs
		// no binary, so intercept it before the exec/arg parsing below (an empty
		// Exec would otherwise trip errEmptyExecCommand). An unknown name fails
		// the test (AC-2); the process is reaped before step N+1 runs (AC-1).
		if cmd.Mode == modeStop {
			stopped, newBgProcs, stopErr := stopNamedBackground(cmd, bgProcs, namedBg)
			if stopErr != nil {
				rec.Error = stopErr
				rec.FailureType = "stop_target_not_found"
				return false
			}
			bgProcs = newBgProcs
			// If the stopped process is also tracked as a ze-peer, drop its proc
			// handle so the end-of-test peer Wait loop does not Wait() an already
			// reaped process (which returns "Wait was already called" and would
			// surface as a spurious peer failure). Its captured output is kept.
			for i := range peerOutputs {
				if peerOutputs[i].proc == stopped {
					peerOutputs[i].proc = nil
				}
			}
			continue
		}

		// Expand $PORT2 before $PORT to avoid partial match ("$PORT2" contains "$PORT")
		execStr := strings.ReplaceAll(cmd.Exec, "$PORT2", strconv.Itoa(rec.Port+1))
		execStr = strings.ReplaceAll(execStr, "$PORT", strconv.Itoa(rec.Port))

		// Parse command and args
		cmdParts := strings.Fields(execStr)
		if len(cmdParts) == 0 {
			rec.Error = errEmptyExecCommand
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
			extraArgs = []string{zeTestVerbPeer}
		case binNameZeTest:
			// ze-test subcommands (peeringdb, rpki, syslog, etc.)
			binPath = r.testPath
		case binNameZe:
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
			s = strings.ReplaceAll(s, "$PORT2", strconv.Itoa(rec.Port+1))
			s = strings.ReplaceAll(s, "$PORT", strconv.Itoa(rec.Port))
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
		if binName == binNameZe && stdinContent != nil {
			// A bare `ze -` daemon launch (the `-` IS the daemon config arg) now runs
			// as `ze start <file>` after spec-fixit-config-file-positional-grammar:
			// keyword-first grammar places the config path behind the `start` verb.
			// A `-` that is a subcommand value (e.g. `ze config validate -`, where
			// zeDaemonConfigArgIndex returns a different index or -1) keeps its
			// in-place file substitution and is NOT prefixed with `start`.
			daemonCfgIdx := zeDaemonConfigArgIndex(args)
			for i, arg := range args {
				if arg != "-" {
					continue
				}
				var tmpFile *os.File
				var err error
				if rec.TmpfsTempDir != "" {
					// Write config to tmpfs dir so relative plugin paths work. The
					// first ze daemon's config keeps the fixed name ze-bgp.conf so
					// tests that rewrite the daemon config (action=rewrite:dest=
					// ze-bgp.conf), restart a second `ze -` against the same file, or
					// assert on rollback/ze-bgp-*.conf versions all address the file
					// the daemon actually reads. A SECOND concurrent ze daemon in the
					// same test uses a DISTINCT stdin block (e.g. an IKE responder +
					// initiator pair), so it gets a per-block file and does not clobber
					// the first daemon's config -- without which a two-daemon test can
					// never form a distinct pair (both load whichever config was
					// written last). Reusing the same block (a restart) reuses its file.
					configName := zeConfigFileName(rec, cmd.Stdin)
					configPath := filepath.Join(rec.TmpfsTempDir, configName)
					tmpFile, err = os.Create(configPath) //nolint:gosec // test runner, path from temp dir
				} else {
					configDir, mkdirErr := os.MkdirTemp(sessionpath.DefaultScratchRoot(), "ze-config-*")
					if mkdirErr != nil {
						rec.Error = fmt.Errorf("create temp config dir: %w", mkdirErr)
						return false
					}
					tmpFilesToClean = append(tmpFilesToClean, configDir)
					// Fix B: this branch handles a tmpfs-less ze (e.g. `ze config
					// validate -`). MkdirTemp is 0700-root, so a credential-dropped
					// ze cannot read its own config -- chown the dir to the target
					// user, mirroring the chownTree done for TmpfsTempDir.
					if netnsMode && netnsHasUID {
						if chErr := chownTree(configDir, netnsUID, netnsGID); chErr != nil {
							rec.Error = fmt.Errorf("chown temp config dir for netns child: %w", chErr)
							return false
						}
					}
					tmpFile, err = os.Create(filepath.Join(configDir, zeDefaultConfigName)) //nolint:gosec // test runner, path from temp dir
				}
				if err != nil {
					rec.Error = fmt.Errorf("create temp config file: %w", err)
					return false
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
				// Fix B: a credential-dropped ze reads this config, but it is
				// created root-owned, so relying on world-read (umask 022) is
				// fragile under a hardened umask (027/077 -> 0640/0600 -> EACCES).
				// Own the file to the target uid so the read never depends on umask.
				if netnsMode && netnsHasUID {
					if chErr := os.Chown(tmpFile.Name(), netnsUID, netnsGID); chErr != nil {
						rec.Error = fmt.Errorf("chown config file for netns child: %w", chErr)
						return false
					}
				}
				if i == daemonCfgIdx {
					// Bare daemon config launch: insert the `start` verb before the
					// config path so `ze -` runs as `ze start <file>`.
					newArgs := make([]string, 0, len(args)+1)
					newArgs = append(newArgs, args[:i]...)
					newArgs = append(newArgs, zeVerbStart, tmpFile.Name())
					newArgs = append(newArgs, args[i+1:]...)
					args = newArgs
				} else {
					args[i] = tmpFile.Name()
				}
				stdinContent = nil // Don't pipe to stdin
				break
			}
		}

		logger().Debug("executing command", "mode", cmd.Mode, "binary", binPath, "args", args)

		// Create command
		proc := exec.CommandContext(testCtx, binPath, args...) //nolint:gosec // test runner

		// Set up environment
		// Put the bare-name shim dir on PATH so child processes resolve THIS
		// run's ze for "ze plugin ..." commands, not a stale or
		// wrong-architecture one left in bin/ (see Runner.setupBinShims).
		proc.Env = childEnv(
			r.childPathEnv(),
			// Repo root for shell-script tests that must run repo-anchored
			// tools (e.g. `ze appliance build` resolves gokrazy/modcache from
			// CWD). Deriving the root from the ze binary's location is wrong
			// here: the runner builds ze into a temp dir, not <repo>/bin.
			zeRepoRootEnv(r.baseDir),
			// Plugin startup stall watchdog. Derived from this test's own budget,
			// not a constant: see pluginStageStall (plugin_stage_stall.go).
			r.pluginStageStallEnv(testBudget),
			r.testBudgetEnv(testBudget),
			// Contention factor for a deadline the child enforces itself (an
			// in-binary readiness wait). See parallelFactorEnv.
			r.parallelFactorEnv(),
			// Cap doctor reachability probes so they fail fast against
			// deliberately-unreachable fixtures instead of waiting out their full
			// multi-second timeout (which dominates wall-clock and flakes under
			// load); the destination is still reported unreachable. See checks_reach.go.
			"ze.test.doctor.probe-timeout=250ms",
		)
		if binName == binNameZe && zeDaemonShouldForceFileStorage(args) {
			// Functional daemon configs are per-test files. Keep them out of the
			// developer's shared zefs active pointer so tests cannot load stale state.
			proc.Env = append(proc.Env, "ze.storage.blob=false")
		}
		// Only set ze_test_bgp_port for ze and ze-peer binaries. Other processes
		// (e.g., ze-chaos --in-process) manage their own port configuration and
		// the override breaks their mock network setup.
		if binName == binNameZe || binName == binNameZePeer {
			proc.Env = append(proc.Env, textbuf.StrInt("ze_test_bgp_port=", int64(rec.Port)))
		}
		// Point ze at the test syslog server when one was started. Uses the
		// ze.log.backend / ze.log.destination convention from slogutil.go.
		// Only applied to ze: ze-peer and helper scripts do not need it.
		if binName == binNameZe && syslogSrv != nil {
			proc.Env = append(proc.Env,
				"ze.log.backend=syslog",
				textbuf.StrInt("ze.log.destination=127.0.0.1:", int64(syslogSrv.Port())),
			)
		}
		// Fix B (R-2 gate): tell ze which netns inode is the HOST so it refuses to
		// program nft if it somehow ended up there (netns isolation silently
		// failed). Only ze needs it -- it owns the firewall Apply chokepoint.
		if netnsMode && binName == binNameZe {
			proc.Env = append(proc.Env,
				textbuf.StrInt("ZE_TEST_NETNS_HOST=", int64(netnsHostInode)))
		}
		// Add test-specific environment variables (option=env:var=KEY:value=VALUE).
		// $PORT/$PORT2 expand like exec strings so per-test ports work in env
		// knobs too (e.g. ze.test.ike.port shared by a two-daemon IKE pair).
		for _, kv := range rec.EnvVars {
			kv = strings.ReplaceAll(kv, "$PORT2", strconv.Itoa(rec.Port+1))
			kv = strings.ReplaceAll(kv, "$PORT", strconv.Itoa(rec.Port))
			proc.Env = append(proc.Env, kv)
		}
		// Tell ze daemons to write a readiness file after signal handlers are
		// registered. The test runner waits for this file before writing
		// daemon.pid, eliminating a startup race condition. Armed for both
		// foreground (default daemon path) and background-daemon suites that poll
		// daemon.pid/daemon.ready; see zeReadyFileEnabled.
		if zeReadyFileEnabled(cmd.Mode, binName, rec.TmpfsTempDir) {
			proc.Env = append(proc.Env,
				"ZE_READY_FILE="+filepath.Join(rec.TmpfsTempDir, "daemon.ready"))
		}

		// Run the child in this test's own directory, so its tmpfs files resolve
		// by bare name and its runtime files land beside the test rather than in
		// the repository root.
		proc.Dir = r.childWorkingDirectory(binName, rec)

		// Set up stdin if specified (for ze and other commands)
		if stdinContent != nil {
			proc.Stdin = strings.NewReader(string(stdinContent))
		}

		// A foreground quick-exit ze subcommand (see quickExitZeVerbs) is awaited
		// in the switch below so sequential steps do not race. It writes to its
		// own buffers, folded into the shared client buffers only after Wait()
		// (see awaitQuickZe).
		quickZe := cmd.Mode == modeForeground && binName == binNameZe && isQuickExitZeCommand(args)
		var quickStdout, quickStderr strings.Builder

		// Capture output: each ze-peer gets its own syncWriter/stderr
		// so WaitFor works independently per process.
		switch {
		case isZePeerExec(execStr):
			po := peerOutput{
				stdout: newSyncWriter(),
				stderr: &lockedBuilder{},
				// Recorded at launch, not recovered at verdict time: only a
				// check-mode peer reports peerSuccessToken, and the verdict must
				// require every one of them individually (failedCheckPeers).
				checkMode: isCheckPeerExec(execStr),
				label:     peerLabel(cmd),
			}
			peerOutputs = append(peerOutputs, po)
			proc.Stdout = po.stdout
			proc.Stderr = po.stderr
		case quickZe:
			proc.Stdout = &quickStdout
			proc.Stderr = &quickStderr
		default:
			proc.Stdout = &clientStdout
			proc.Stderr = teeDaemonStderr(&clientStderr, awaitStderrSW, binName == binNameZe)
		}

		// Fix B: drop the ze daemon to a normal user so its readiness handshake
		// works (A-4: the readiness file is written after dropPrivileges, so a
		// root ze never writes it). The setcap'd binary keeps ambient CAP_NET_ADMIN
		// for nft. Peer and fixture helpers stay privileged (root under sudo) so
		// they can read nft state and signal the daemon.
		if netnsMode && netnsHasUID && binName == binNameZe {
			proc.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{Uid: uint32(netnsUID), Gid: uint32(netnsGID)}, //nolint:gosec // uid/gid from local dev env
			}
		}

		// Start the process, retrying on ETXTBSY (see startWithETXTBSYRetry).
		var startErr error
		if proc, startErr = startWithETXTBSYRetry(testCtx, binPath, args, proc); startErr != nil {
			rec.Error = fmt.Errorf("start %s: %w", binName, startErr)
			return false
		}

		switch {
		case cmd.Mode == modeBackground:
			bgProcs = append(bgProcs, proc)
			// Register the name a cmd=stop directive can target (Phase 2 naming
			// grammar). Only background processes are nameable; the binding lives
			// alongside the tracked *exec.Cmd, not a second registry.
			if cmd.Name != "" {
				namedBg[cmd.Name] = proc
			}
			switch {
			case isZePeerExec(execStr):
				// Wait for ze-peer to be ready (listening) instead of a fixed
				// sleep. A peer that never binds is a FAILURE, never a skip: ze
				// would dial a dead port, get connection refused, and back off,
				// which reads as an establishment stall.
				//
				// This barrier used to be skipped when rec.ExpectExitCode != nil
				// ("the peer may not start"). That turned the one condition that
				// detects a non-binding peer into a silent no-op for exactly the
				// tests that could not otherwise notice. See
				// spec-fixit-redistribute-establishment-stall (D1) and
				// ai/rules/evidence.md.
				//
				// The process handle is recorded for EVERY peer, dialing or
				// listening: the end-of-test loop waits on po.proc, so a peer with
				// none is a peer the run never waits for and never reads a verdict
				// from. A --dial peer left unrecorded made its test finish in 3ms
				// with an empty capture, reported as "check peer produced no
				// output": the run had started both processes and then waited for
				// neither.
				po := &peerOutputs[len(peerOutputs)-1]
				po.proc = proc

				// Only the BIND barrier is skipped for a dialing peer: it takes the
				// active role and never listens, so it prints no readiness token.
				// It waits for the daemon itself instead, by retrying the dial
				// (dialTarget, internal/test/peer/peer.go).
				if !strings.Contains(execStr, "--dial") {
					// 5s to bind is generous unloaded, but a parallel run oversubscribes
					// every core: a ze-peer process can take longer than 5s just to start
					// and listen, which reads as a spurious "peer never bound". Scale the
					// bind budget by the same parallel headroom the outer test budget gets
					// (identity for serial runs, so real bind failures still surface fast).
					peerBindTimeout := r.withParallelHeadroom(5 * time.Second)
					waitCtx, waitCancel := context.WithTimeout(testCtx, peerBindTimeout)
					if !po.stdout.waitFor(waitCtx) {
						waitCancel()
						rec.Error = peerBindFailure(peerBindTimeout, po.stderr.String(), po.stdout.String())
						rec.FailureType = FailTypePeerNeverBound
						return false
					}
					waitCancel()
				}
			case zeReadyFileEnabled(cmd.Mode, binName, rec.TmpfsTempDir):
				// Background-daemon suites poll daemon.pid/daemon.ready.
				// ZE_READY_FILE was armed above, so
				// mirror the foreground daemon path: when no ze-peer provides
				// BGP-level synchronization, wait for the readiness file, then
				// publish daemon.pid. Without this the poller times out by
				// construction (the runner never wrote either file).
				if proc.Process != nil {
					// An await=stderr fence provides its own synchronization (and
					// a plugin that aborts startup may never write daemon.ready),
					// so skip this 5s wait then -- mirrors the foreground path.
					if !hasPeer && rec.AwaitStderr == "" {
						readyPath := filepath.Join(rec.TmpfsTempDir, "daemon.ready")
						// Parallel-run headroom: a daemon that writes daemon.ready after
						// startup can be slow to reach it under oversubscription. Identity
						// for serial runs.
						waitReady(testCtx, readyPath, r.withParallelHeadroom(5*time.Second))
					}
					pidPath := filepath.Join(rec.TmpfsTempDir, "daemon.pid")
					_ = os.WriteFile(pidPath, fmt.Appendf(nil, "%d", proc.Process.Pid), 0o600)
				}
			default:
				// Other non-peer background process (helper script, or a ze
				// daemon with no tmpfs dir): brief sleep for startup.
				time.Sleep(100 * time.Millisecond)
			}
		case cmd.Mode == modeForeground && binName != binNameZe && binName != binNameZePeer && cmdIdx < len(cmds)-1:
			// Foreground setup script (non-daemon, e.g., create-marker.sh) that
			// precedes other commands: wait for completion before starting the
			// next command. Without this, the setup script may not finish before
			// ze reads its output, causing races under concurrent test load.
			// Only applies when followed by more commands; if it's the last
			// command, fall through to normal exit code handling.
			if err := proc.Wait(); err != nil {
				// Collect the output first. This early return skips the
				// collection at the bottom of the function. A barrier script
				// that exits non-zero was therefore reported as a bare
				// "setup script sh: exit status 1". The message it printed to
				// say WHY was nowhere in the report.
				//
				// That message is the whole reason a barrier is a script
				// rather than a sleep. The await=stderr arm below carries the
				// same repair.
				// ai/rules/evidence.md: the guard must speak.
				rec.ClientOutput = clientStdout.String() + clientStderr.String()
				rec.PeerOutput = collectPeerOutput(peerOutputs)
				rec.Duration = time.Since(rec.StartTime)
				rec.Error = fmt.Errorf("setup script %s: %w", binName, err)
				return false
			}
			continue // Already finished, don't track for cleanup
		case quickZe:
			// Foreground quick-exit ze: await completion so the next command does
			// not start (and race on the client buffers) before this one finishes.
			lastQuickZeErr = awaitQuickZe(proc, &quickStdout, &quickStderr, &clientStdout, &clientStderr)
			quickZeRan = true
			// Assert this command's own exit code when the test declared one.
			// The file-level expect=exit:code= below can only ever check the
			// LAST quick ze command (it compares one value against
			// lastQuickZeErr), so a file running several validations needs
			// exit= per command to assert the earlier ones at all.
			if cmd.ExitCode != nil {
				actual := 0
				if exitErr, ok := errors.AsType[*exec.ExitError](lastQuickZeErr); ok {
					actual = exitErr.ExitCode()
				}
				if actual != *cmd.ExitCode {
					rec.Error = fmt.Errorf("cmd seq=%d (%s): expected exit code %d, got %d", cmd.Seq, cmd.Exec, *cmd.ExitCode, actual)
					rec.FailureType = "exit_code_mismatch"
					return false
				}
			}
			continue // Already finished, don't track for cleanup
		default:
			// Foreground daemon (ze): start but don't wait - we wait for peer instead
			bgProcs = append(bgProcs, proc) // Track for cleanup

			// Write daemon PID to tmpfs dir so background helpers can send signals.
			// Guard on ze: a foreground helper that is the last command also lands
			// here and must NOT clobber daemon.pid with its own pid. A helper that
			// reads daemon.pid to signal the daemon would otherwise signal itself.
			if zeReadyFileEnabled(cmd.Mode, binName, rec.TmpfsTempDir) && proc.Process != nil {
				// When no ze-peer provides BGP-level synchronization, wait for
				// the process readiness file before writing daemon.pid. This
				// prevents a race where signal.sh sends SIGHUP before the
				// process has registered signal handlers. An await=stderr fence
				// provides its own synchronization (and a plugin that aborts
				// startup may never write daemon.ready), so skip this wait then.
				if !hasPeer && rec.AwaitStderr == "" {
					readyPath := filepath.Join(rec.TmpfsTempDir, "daemon.ready")
					// Same parallel-run headroom as the background-daemon path above:
					// a foreground ze slow to write daemon.ready under oversubscription
					// must not make the runner publish daemon.pid (and let signal.sh
					// SIGHUP) before handler registration. Identity for serial runs.
					waitReady(testCtx, readyPath, r.withParallelHeadroom(5*time.Second))
				}
				pidPath := filepath.Join(rec.TmpfsTempDir, "daemon.pid")
				_ = os.WriteFile(pidPath, fmt.Appendf(nil, "%d", proc.Process.Pid), 0o600)
			}
		}
	}

	// Execute HTTP waits (readiness polls) before assertion checks.
	httpStepN := 0
	if len(rec.HTTPWaits) > 0 {
		if waitErr := r.executeHTTPWaits(testCtx, rec); waitErr != nil {
			rec.Error = waitErr
			rec.FailureType = "http_check_failed"
			rec.Duration = time.Since(rec.StartTime)
			httpStepN++
			rec.StepTrace = append(rec.StepTrace, trace.StepResult{
				Step: httpStepN, Kind: stepKindExpect, Assert: "http-wait",
				Passed: false, Detail: waitErr.Error(),
			})
			return false
		}
		httpStepN++
		rec.StepTrace = append(rec.StepTrace, trace.StepResult{
			Step: httpStepN, Kind: stepKindExpect, Assert: "http-wait", Passed: true,
		})
	}

	// Execute HTTP checks (after background processes have started). A passing
	// HTTP check is a self-validation signal (folded into hasOutputAssertion
	// below) but it must NOT short-circuit the remaining assertions: a test may
	// combine http= with expect=stderr / expect=syslog / reject= / file checks,
	// and the universal observer-failure sentinel must still be evaluated in the
	// common success tail. The old early `return true` here skipped all of those,
	// so an http= test that also declared logging assertions (or whose observer
	// called runtime_fail) could pass on the HTTP check alone.
	if len(rec.HTTPChecks) > 0 {
		if httpErr := r.executeHTTPChecks(testCtx, rec); httpErr != nil {
			rec.Error = httpErr
			rec.FailureType = "http_check_failed"
			rec.Duration = time.Since(rec.StartTime)
			httpStepN++
			rec.StepTrace = append(rec.StepTrace, trace.StepResult{
				Step: httpStepN, Kind: stepKindExpect, Assert: "http-check",
				Passed: false, Detail: httpErr.Error(),
			})
			return false
		}
		httpStepN++
		rec.StepTrace = append(rec.StepTrace, trace.StepResult{
			Step: httpStepN, Kind: stepKindExpect, Assert: "http-check", Passed: true,
		})
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

	switch {
	case awaitStderrSW != nil:
		// await=stderr fence: block until the daemon's relayed stderr carries the
		// needle, then fall through to the graceful-stop teardown below. This is
		// the synchronization point for the reject-fence bucket, where the plugin
		// under test aborts startup and no in-daemon observer can run. On timeout
		// the helper tears the daemon down and records the failure.
		if !r.awaitDaemonStderr(testCtx, rec, awaitStderrSW, bgProcs, peerProcs, testBudget) {
			// The fence expired, and this early return skips the output collection
			// at the bottom of the function -- so the report used to print "expected
			// 0 / received 0" and NOTHING the daemon said, which is the one thing
			// that explains why the needle never arrived (a daemon that died in
			// startup looks identical to one that is merely slow).
			// ai/rules/evidence.md: the guard must speak.
			rec.ClientOutput = clientStdout.String() + clientStderr.String()
			rec.PeerOutput = collectPeerOutput(peerOutputs)
			rec.Duration = time.Since(rec.StartTime)
			return false
		}
	case rec.ExpectExitCode != nil && fgProc != nil:
		// Testing exit code: wait for foreground process
		err = fgProc.Wait()
	case rec.ExpectExitCode != nil && quickZeRan:
		// Exit-code test with no daemon fgProc: the exit code comes from the last
		// awaited quick-exit ze command (e.g. isis-config's `ze config validate -`
		// steps, already Wait()ed in the loop).
		err = lastQuickZeErr
	default:
		// Wait for all peer processes (each validates its own messages).
		// Daemons run until killed below. Collect all errors so no peer
		// failure is silently lost.
		var peerErrs []error
		for i := range peerOutputs {
			if peerOutputs[i].proc != nil {
				if waitErr := peerOutputs[i].proc.Wait(); waitErr != nil {
					peerErrs = append(peerErrs, waitErr)
				}
				peerOutputs[i].waited = true
			}
		}
		err = errors.Join(peerErrs...)
	}

	// Gracefully stop remaining processes (daemons). The daemon that the test's
	// own observer asks to stop is given that chance first: see
	// selfStopGrace and terminateAfterSelfExit.
	selfStop := fgProc != nil && tmpfsRequestsDaemonShutdown(rec.TmpfsFiles)
	for _, p := range bgProcs {
		if peerProcs[p] || p.Process == nil {
			continue
		}
		if selfStop && p == fgProc {
			terminateAfterSelfExit(p, selfStopGrace(r.withParallelHeadroom(testBudget)))
			continue
		}
		terminateGracefully(p)
	}

	// A scaffolding peer never ends itself, so it is signaled here and reaped by
	// the barrier below, which would otherwise burn its whole grace on a process
	// whose only exit is a signal (see terminateScaffoldPeers).
	//
	// After the daemon teardown above, never before the arms: the daemon is still
	// exchanging with these peers while an arm runs (event-predicate-wait.ci needs
	// its echo peer to reflect an UPDATE back before `ze` exits), so an earlier
	// signal would cut the exchange the test measures.
	terminateScaffoldPeers(peerOutputs)

	// Barrier: every peer's output must be complete before anything reads it. Only
	// the default arm above waits peers; the await=stderr and exit-code arms wait
	// the daemon alone, and ~290 .ci files pair a check peer with expect=exit.
	drainPeers(peerOutputs, peerDrainGrace)

	allPeerStdout, allPeerStderr := collectPeerStreams(peerOutputs)
	rec.PeerOutput = allPeerStdout + allPeerStderr
	rec.ClientOutput = clientStdout.String() + clientStderr.String()
	rec.Duration = time.Since(rec.StartTime)
	logger().Debug("collected output", "peerOutput", rec.PeerOutput, "clientOutput", rec.ClientOutput)

	// Parse received messages
	rec.ReceivedRaw = extractReceivedMessages(rec.PeerOutput)

	// Save outputs if requested
	if opts.SaveDir != "" {
		out := &testOutput{
			peerStdout:   allPeerStdout,
			peerStderr:   allPeerStderr,
			clientStdout: clientStdout.String(),
			clientStderr: clientStderr.String(),
		}
		if saveErr := r.saveTestOutput(rec, out, opts.SaveDir); saveErr != nil {
			logger().Warn("save test output failed", "nick", rec.Nick, "error", saveErr)
		}
	}

	// Observer sentinel takes precedence in the orchestrated path too.
	// Keep both paths in sync: a failure from a compiled observer must surface
	// as the authoritative reason even when the daemon subsequently times out.
	if sentinelErr := checkObserverSentinel(clientStderr.String()); sentinelErr != nil {
		rec.Error = sentinelErr
		rec.FailureType = FailTypeLoggingMismatch
		return false
	}
	if sentinelErr := observerSentinelInSyslog(syslogSrv); sentinelErr != nil {
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

	stepN := len(rec.StepTrace)
	recStep := func(assert string, passed bool, detail string) {
		stepN++
		rec.StepTrace = append(rec.StepTrace, trace.StepResult{
			Step: stepN, Kind: stepKindExpect, Assert: assert,
			Passed: passed, Detail: detail,
		})
	}

	// Validate the asserted exit code, if any. The comparison only makes sense
	// when the test declared one; the output/logging/file assertions below run
	// regardless.
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
			recStep("exit-code", false, rec.Error.Error())
			return false
		}
		recStep("exit-code", true, "")
	}

	// Output assertions (stdout/stderr substrings) run whenever the test
	// declares them, regardless of whether an exit code was asserted. These
	// checks used to be nested inside the `if rec.ExpectExitCode != nil` block
	// above, so a cmd=foreground test that only checked stdout/stderr/files (no
	// expect=exit) had every assertion silently skipped and then fell through to
	// a default "unknown" failure (handover §3).
	for _, expected := range rec.ExpectStderrMatch {
		if !strings.Contains(rec.ClientOutput, expected) {
			rec.Error = fmt.Errorf("stderr does not contain %q", expected)
			rec.FailureType = "stderr_mismatch"
			recStep("stderr-contains", false, rec.Error.Error())
			return false
		}
		recStep("stderr-contains", true, "")
	}
	for _, expected := range rec.ExpectStdoutMatch {
		if !strings.Contains(rec.ClientOutput, expected) {
			rec.Error = fmt.Errorf("stdout does not contain %q", expected)
			rec.FailureType = FailTypeStdoutMismatch
			recStep("stdout-contains", false, rec.Error.Error())
			return false
		}
		recStep("stdout-contains", true, "")
	}
	for _, forbidden := range rec.ExpectStdoutNotMatch {
		if strings.Contains(rec.ClientOutput, forbidden) {
			rec.Error = fmt.Errorf("stdout unexpectedly contains %q", forbidden)
			rec.FailureType = FailTypeStdoutMismatch
			recStep("stdout-not-contains", false, rec.Error.Error())
			return false
		}
		recStep("stdout-not-contains", true, "")
	}
	for _, pattern := range rec.ExpectStdoutRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			rec.Error = fmt.Errorf("invalid stdout regex %q: %w", pattern, err)
			rec.FailureType = FailTypeStdoutMismatch
			recStep("stdout-regex", false, rec.Error.Error())
			return false
		}
		if !re.MatchString(rec.ClientOutput) {
			rec.Error = fmt.Errorf("stdout does not match regex %q", pattern)
			rec.FailureType = FailTypeStdoutMismatch
			recStep("stdout-regex", false, rec.Error.Error())
			return false
		}
		recStep("stdout-regex", true, "")
	}
	for _, pattern := range rec.RejectStdoutRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			rec.Error = fmt.Errorf("invalid reject stdout regex %q: %w", pattern, err)
			rec.FailureType = FailTypeStdoutMismatch
			recStep("stdout-reject-regex", false, rec.Error.Error())
			return false
		}
		if re.MatchString(rec.ClientOutput) {
			rec.Error = fmt.Errorf("stdout matches forbidden regex %q", pattern)
			rec.FailureType = FailTypeStdoutMismatch
			recStep("stdout-reject-regex", false, rec.Error.Error())
			return false
		}
		recStep("stdout-reject-regex", true, "")
	}

	// Decide what governs this test's success: a test with a check-mode ze-peer is
	// always governed by the BGP exchange (the peer must print "successful");
	// any other test is governed by the exit/output/file/logging assertions
	// evaluated here. See isSelfValidated in peer_contract.go for why an
	// exit-code assertion must not disable the peer path, and hasCheckPeer for
	// why a sink/echo peer cannot govern. expect=json is NOT part of this
	// decision: it is a runner-side assertion and is evaluated below, for every
	// path.
	if !isSelfValidated(rec, hasCheckPeer(cmds)) {
		// BGP peer path: each check-mode peer validates its own messages and
		// prints peerSuccessToken on a clean exchange. EVERY one of them must,
		// which is why this asks failedCheckPeers rather than searching the
		// joined rec.PeerOutput -- see that function for the masking defect.
		failedPeers := failedCheckPeers(countCheckPeers(cmds), peerOutputs)
		if err != nil || len(failedPeers) > 0 {
			rec.FailedPeers = failedPeers
			switch {
			case strings.Contains(rec.PeerOutput, FailTypeMismatch):
				rec.FailureType = FailTypeMismatch
				rec.LastExpectedIdx, rec.LastReceivedIdx = extractMismatchIndices(rec.PeerOutput)
			case strings.Contains(rec.PeerOutput, "connection refused"):
				rec.FailureType = FailTypeConnectionRefuse
			default:
				rec.FailureType = stateUnknown
			}
			if isNearTimeout(float64(rec.Duration)/float64(timeout), rec.FailureType) {
				rec.FailureType = FailTypeNearTimeout
			}
			if err != nil {
				rec.Error = err
			}
			recStep("peer-exchange", false, rec.FailureType)
			return false
		}
		recStep("peer-exchange", true, "")
	}

	// expect=json is evaluated HERE, by the runner, and never by ze-peer:
	// LoadExpectFile forwards only the lines peer.ConsumesLine accepts and drops
	// json outright (internal/test/peer/expect.go). So it must not sit behind the
	// peer branch. It did, and that made the remedy validatePeerBlocks hands out
	// -- run the peer with --mode sink -- silently delete every JSON assertion in
	// the file: sinking clears hasCheckPeer, isSelfValidated then returns true for
	// any record with an exit-code or output assertion, and the whole branch was
	// skipped. A guard against vacuous greens must not prescribe one.
	// Evaluating it out here costs a peer test nothing: a failed peer exchange has
	// already returned above, so the order is unchanged. rec.ReceivedRaw is
	// populated from the peer capture in every mode (a sink peer prints
	// "msg  recv" too, internal/test/peer/peer.go), so the assertion is evaluable
	// wherever it is declared.
	if hasJSONExpectations(rec) {
		if jsonErr := r.validateJSON(rec); jsonErr != nil {
			rec.Error = jsonErr
			rec.FailureType = FailTypeJSONMismatch
			recStep("json-match", false, jsonErr.Error())
			return false
		}
		recStep("json-match", true, "")
	}

	// Common success tail: logging and file assertions apply to every passing
	// path (exit-code, peer-less foreground, and BGP peer).
	if logErr := r.validateLogging(rec, clientStderr.String(), syslogSrv); logErr != nil {
		rec.Error = logErr
		rec.FailureType = FailTypeLoggingMismatch
		recStep("logging", false, logErr.Error())
		return false
	}
	if len(rec.ExpectStderr) > 0 || len(rec.RejectStderr) > 0 ||
		len(rec.ExpectSyslog) > 0 || len(rec.RejectSyslog) > 0 {
		recStep("logging", true, "")
	}
	if fileErr := r.validateFileChecks(rec); fileErr != nil {
		rec.Error = fileErr
		rec.FailureType = fileCheckFailed
		recStep("file-check", false, fileErr.Error())
		return false
	}
	if len(rec.FileChecks) > 0 {
		recStep("file-check", true, "")
	}
	return true
}
