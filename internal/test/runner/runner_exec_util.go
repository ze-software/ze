// Design: docs/architecture/testing/ci-format.md -- test execution utilities
// Overview: runner_exec.go -- test execution and process orchestration
// Related: plugin_stage_stall.go -- derives the plugin stall window, then takes withParallelHeadroom

package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var errEmptyExecCommand = errors.New("empty exec command")

// errNetnsNeedsUID is returned when ZE_TEST_NETNS is active but no valid
// non-root ZE_TEST_UID is configured. ze must run as a normal user in the netns
// (A-4), so this is a setup error, not a silent root run.
var errNetnsNeedsUID = errors.New("ZE_TEST_NETNS is set but no valid non-root ZE_TEST_UID is configured; ze must run as a normal user (set ZE_TEST_UID=$(id -u) ZE_TEST_GID=$(id -g))")

// netnsModeActive reports whether the opt-in per-test network-namespace launch
// mode (Fix B) is active: ZE_TEST_NETNS is set in the runner's environment and
// the host is Linux. Off by default, so the standard host-netns/unprivileged
// launch path is byte-identical for every suite that passes today (AC-6).
func netnsModeActive() bool {
	return runtime.GOOS == "linux" && os.Getenv("ZE_TEST_NETNS") != ""
}

// netnsActive is the seam the parse-time netns-link skip gate reads (see
// applyNetnsLinkGate in caps.go). The EXECUTION path deliberately keeps calling
// netnsModeActive() directly (runner_exec.go), so flipping this seam changes
// which tests are gated, never what a running test actually gets. It
// is a package var, not a direct call, so a unit test can drive BOTH polarities
// of the gate on any host: netnsModeActive() is false on every non-Linux dev
// box, so asserting only the host's real answer would leave the "runs under
// netns mode" branch vacuous exactly where these tests are authored.
//
// Deliberate, precedented exception to the "no global mutable state" rule in
// ai/rules/go-standards.md: it is a test seam, never written in production
// (only netnsModeActive's result is read), and it mirrors hasNetAdmin in
// caps.go and interfaceByName in internal/component/ike/engine/doctor.go.
var netnsActive = netnsModeActive

// copyTestScripts copies the Python test-support modules (test/scripts/*.py,
// notably ze_api.py) from baseDir into dstDir. In netns launch mode ze runs as a
// normal user and forks observer plugins as that user; PYTHONPATH points at
// test/scripts under the repo, but a uid-dropped observer cannot traverse a 0700
// repo root to reach it. The observer's own script directory (dstDir, the tmpfs
// workdir) is on sys.path[0], so placing the modules there makes `import ze_api`
// work regardless of repo-root permissions. A missing source dir is not an error
// (a test with no observer needs nothing).
func copyTestScripts(baseDir, dstDir string) error {
	srcDir := filepath.Join(baseDir, "test", "scripts")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".py") {
			continue
		}
		dst := filepath.Join(dstDir, e.Name())
		// A test may ship its own .py of the same name (materialized earlier from a
		// tmpfs= block); it wins. Only fill in modules the test did not provide.
		if _, statErr := os.Stat(dst); statErr == nil {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(srcDir, e.Name())) //nolint:gosec // fixed repo path test/scripts/*.py, not user input
		if readErr != nil {
			return fmt.Errorf("read %s: %w", e.Name(), readErr)
		}
		if writeErr := os.WriteFile(dst, data, 0o644); writeErr != nil { //nolint:gosec // test-support module, world-readable by design so the uid-dropped observer can import it
			return fmt.Errorf("write %s: %w", e.Name(), writeErr)
		}
	}
	return nil
}

// netnsChildIDs parses the normal-user uid/gid the netns mode drops the ze
// daemon to (ZE_TEST_UID, ZE_TEST_GID). ze must NOT run as root in the netns:
// its readiness file is created after dropPrivileges, so a root ze never writes
// it and the handshake times out (assumption A-4). The make target passes
// ZE_TEST_UID=$(id -u) ZE_TEST_GID=$(id -g); GID defaults to UID when unset.
// ok is false when no valid non-root uid is configured, in which case the caller
// leaves the child as-is (runner-privileged) rather than silently mis-dropping.
func netnsChildIDs() (uid, gid int, ok bool) {
	us := os.Getenv("ZE_TEST_UID")
	if us == "" {
		return 0, 0, false
	}
	u, err := strconv.Atoi(us)
	if err != nil || u <= 0 {
		return 0, 0, false
	}
	g := u
	if gs := os.Getenv("ZE_TEST_GID"); gs != "" {
		if parsed, gerr := strconv.Atoi(gs); gerr == nil && parsed >= 0 {
			g = parsed
		}
	}
	return u, g, true
}

// chownTree recursively chowns root to uid:gid. In netns mode the runner (root,
// under sudo) has already written the per-test tmpfs dir and its config files;
// the ze daemon is dropped to a normal user and must be able to chdir into that
// dir, read its config, and write daemon.ready there. os.MkdirTemp creates the
// dir 0700-root, so without this the dropped ze cannot enter its own workdir.
func chownTree(root string, uid, gid int) error {
	return filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}

// zeRepoRootEnv renders the ZE_REPO_ROOT=<baseDir> environment entry exported
// to every exec'd test process. Shell-script tests use it to locate the source
// tree (the ze binary's own directory is a temp build dir, not <repo>/bin).
func zeRepoRootEnv(baseDir string) string {
	var tb textbuf.Buffer
	return tb.Str("ZE_REPO_ROOT=").Str(baseDir).String()
}

// parallelFactor is the multiplier withParallelHeadroom applies:
// ParallelTimeoutHeadroom under concurrent execution, 1 for a serial run.
// Exposed so COUNT-based budgets (an HTTP readiness poll's retry attempts)
// scale by the same factor as the duration budgets, from one source of truth.
func (r *Runner) parallelFactor() int {
	if r.concurrency > 1 {
		return ParallelTimeoutHeadroom
	}
	return 1
}

// withParallelHeadroom widens a resolved per-test timeout by
// ParallelTimeoutHeadroom when this Run executes tests concurrently. Serial
// runs (-p 1 or a single selected test) keep the authored value so real
// slowdowns surface quickly. See ParallelTimeoutHeadroom for rationale.
//
// This applies to the OUTER per-test budget AND to the fixed inner readiness
// gates it used to leave untouched: BOTH ze-peer bind barriers (orchestrated and
// non-orchestrated), BOTH daemon.ready waits (background and foreground), the
// await=stderr fence, the per-message `ze bgp decode` fork, and the HTTP
// readiness wait / retry-count / per-request budgets (runner_exec.go,
// await_stderr.go, runner_validate.go). Those gates are what a contended run
// blows first while the widened outer budget still has room -- the flaky-under-load
// class archived in plan/known-failures/RESOLVED.md ("fixed startup deadlines
// fail under CPU oversubscription", resolved 2026-07-25 by this helper).
func (r *Runner) withParallelHeadroom(timeout time.Duration) time.Duration {
	return timeout * time.Duration(r.parallelFactor())
}

// engineStepsForRun returns a copy of steps with each step's timeout widened by
// the parallel headroom. The engine-steps executor runs its per-step polls (an
// establishment wait, a rekey-count poll) inside the spawned daemon against
// these authored timeouts; unlike the outer daemon budget they are not otherwise
// widened, so under contention a poll can expire while the daemon budget is
// still fine. Applying the same headroom keeps two-daemon tests reliable in
// parallel while serial runs (-p 1) stay tight. The original slice is untouched.
func (r *Runner) engineStepsForRun(steps []EngineStep) []EngineStep {
	out := make([]EngineStep, len(steps))
	for i, s := range steps {
		s.Timeout = r.withParallelHeadroom(s.Timeout)
		out[i] = s
	}
	return out
}

// peerDrainGrace bounds the post-run wait for peer processes to exit so their
// captured output is complete. Generous: a check peer that has satisfied its
// expectations exits immediately, so this is only ever paid by a peer that is
// already failing or hung, where the test is red regardless.
const peerDrainGrace = 10 * time.Second

const (
	modeForeground  = "foreground"
	modeBackground  = "background"
	modeStop        = "stop"
	fileCheckFailed = "file_check_failed"
	failParseError  = "parse_error"
)

// Signal names accepted by a cmd=stop directive. Default is SIGKILL: a killed
// responder goes silent (the exact condition IKEv2 liveness detection expects),
// whereas SIGTERM would let the process send a clean protocol teardown and defeat
// the DPD proof. See plan/spec-fixit-runner-kill-background.md (R-1).
const (
	signalKill = "kill"
	signalTerm = "term"
)

// zeReadyFileEnabled reports whether a launched command is a ze daemon whose
// readiness handshake the runner should arm: set ZE_READY_FILE so the daemon
// writes daemon.ready after startup, and track daemon.pid. It is true for a ze
// daemon started either foreground (the default daemon path) or background
// (driver.py-style suites that poll daemon.pid/daemon.ready). A TmpfsTempDir is
// required because that is where the handshake files live. Non-ze binaries
// (ze-peer, helper scripts) are never armed.
func zeReadyFileEnabled(mode, binName, tmpfsTempDir string) bool {
	if binName != "ze" || tmpfsTempDir == "" {
		return false
	}
	return mode == modeForeground || mode == modeBackground
}

// lockedBuilder is a mutex-guarded output accumulator.
//
// A .ci test's non-peer client processes -- the ze daemon plus every
// cmd=background helper (python collectors, mock servers, observers) -- all
// write into ONE stdout and ONE stderr accumulator, and os/exec spawns a copy
// goroutine per stream per process whenever Cmd.Stdout/Stderr is not an
// *os.File. A bare strings.Builder is not safe for concurrent use: two copy
// goroutines appending at once lose whole lines, so an expect=stderr:pattern=
// that the process really did print fails spuriously under load.
//
// Safe for concurrent use. Deliberately not a syncWriter: syncWriter re-scans
// the whole buffer for its pattern on every Write, which is quadratic for a
// general-purpose accumulator.
type lockedBuilder struct {
	mu        sync.Mutex
	buf       strings.Builder
	truncated bool
}

// truncationMarker is appended, exactly once, the first time an accumulator
// drops output at maxOutputBytes.
//
// The cap must SAY it fired. A silent cap is a guard that neither denies nor
// speaks (ai/rules/fail-closed-guards.md): a positive `expect=stdout:pattern=`
// whose needle lands past the cap fails over a capture that looks complete, and
// the failure reads as "the daemon never printed it". The runner sets
// SLOG_LEVEL=DEBUG for every client, so 10 MB is reachable.
const truncationMarker = "\n[ze-test: output truncated at maxOutputBytes; the capture below this line is incomplete]\n"

// appendCapped stores as much of s as the cap allows and records whether it had
// to drop anything. Caller MUST hold b.mu.
func (b *lockedBuilder) appendCapped(s string) {
	if b.truncated {
		return
	}
	remaining := maxOutputBytes - b.buf.Len()
	if len(s) <= remaining {
		b.buf.WriteString(s) //nolint:errcheck // strings.Builder.WriteString never fails
		return
	}
	if remaining > 0 {
		b.buf.WriteString(s[:remaining]) //nolint:errcheck // strings.Builder.WriteString never fails
	}
	b.buf.WriteString(truncationMarker) //nolint:errcheck // strings.Builder.WriteString never fails
	b.truncated = true
}

// Write appends p, capped at maxOutputBytes so a runaway process cannot exhaust
// memory. It reports the caller's FULL length even when the cap truncates: a
// short count makes os/exec's io.Copy goroutine abort with ErrShortWrite, which
// then kills the child with EPIPE and surfaces as a bogus test failure.
func (b *lockedBuilder) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.appendCapped(string(p))
	return len(p), nil
}

// WriteString appends s under the same lock as Write. It keeps the stdlib
// io.StringWriter signature: a divergent one would shadow the embedded
// strings.Builder's and make io.WriteString / io.MultiWriter silently fall back
// to Write after a failed interface probe.
func (b *lockedBuilder) WriteString(s string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.appendCapped(s)
	return len(s), nil
}

// String returns everything captured so far.
func (b *lockedBuilder) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// syncWriter is an io.Writer that captures output and supports waiting for patterns.
// Used to wait for ze-peer's "listening on" message before starting the client.
type syncWriter struct {
	mu        sync.Mutex
	buf       strings.Builder
	pattern   string
	found     bool
	truncated bool
}

// peerListeningPattern is the string ze-peer prints to stdout when ready.
const peerListeningPattern = "listening on"

// newSyncWriter creates a writer that waits for ze-peer's "listening on" output.
func newSyncWriter() *syncWriter {
	return &syncWriter{pattern: peerListeningPattern}
}

// newSyncWriterPattern creates a writer that waits for a caller-supplied
// substring rather than ze-peer's fixed "listening on" marker. Used by the
// await=stderr:contains= fence to block until a daemon's relayed stderr carries
// a given line (e.g. an external plugin's refuse/warn message).
func newSyncWriterPattern(pattern string) *syncWriter {
	return &syncWriter{pattern: pattern}
}

// maxOutputBytes caps captured output to prevent OOM from runaway processes.
const maxOutputBytes = 10 << 20 // 10 MB

// Write captures data and checks for the pattern.
// Output is capped at maxOutputBytes to prevent unbounded memory growth.
func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Report the caller's FULL length even when the cap truncates. These writers
	// are driven by os/exec's per-stream io.Copy goroutine, and io.Copy treats a
	// short count as io.ErrShortWrite: it aborts the copy, os/exec closes the
	// pipe read end, the child then gets EPIPE on every later write, and
	// cmd.Wait() surfaces "short write" -- which an expect=exit:code=0 reads as a
	// test failure. Worse, teeDaemonStderr fans stderr through an io.MultiWriter
	// (await_stderr.go), which propagates one leg's ErrShortWrite to the whole
	// daemon stderr stream. The cap must bound memory via the return value never,
	// and announce itself in the buffer instead (see truncationMarker).
	n := len(p)
	if !sw.truncated {
		s := string(p)
		remaining := maxOutputBytes - sw.buf.Len()
		if len(s) <= remaining {
			sw.buf.WriteString(s) //nolint:errcheck // strings.Builder.WriteString never fails
		} else {
			if remaining > 0 {
				sw.buf.WriteString(s[:remaining]) //nolint:errcheck // strings.Builder.WriteString never fails
			}
			sw.buf.WriteString(truncationMarker) //nolint:errcheck // strings.Builder.WriteString never fails
			sw.truncated = true
		}
	}
	if !sw.found && strings.Contains(sw.buf.String(), sw.pattern) {
		sw.found = true
	}
	return n, nil
}

// waitFor waits until the pattern is found or context is canceled.
// Returns true if pattern was found, false on timeout/cancel.
func (sw *syncWriter) waitFor(ctx context.Context) bool {
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
	// stderr must be concurrency-safe, not a bare strings.Builder: the
	// peer-never-bound path reads it (runner_exec.go peerBindFailure) while the
	// peer is STILL RUNNING and its os/exec copy goroutine is still appending --
	// only the readiness WaitFor timed out, nothing has Wait()ed the process.
	stderr *lockedBuilder
	proc   *exec.Cmd
	// checkMode records whether this peer validates what it receives. Only a
	// check-mode peer reports "successful", so only a check-mode peer may be
	// required to (peer_contract.go failedCheckPeers).
	checkMode bool
	// label identifies this peer in a failure message. Per-peer attribution is
	// the point of evaluating peers individually, so it has to survive to the
	// verdict rather than be reconstructed from the joined output.
	label string
	// waited records that proc.Wait() has already run, so the drain barrier below
	// does not Wait twice (the second call returns an error and tells us nothing).
	waited bool
}

// collectPeerStreams concatenates every peer's capture in process start order,
// returning the joined stdout and the joined stderr separately (the save-output
// path writes them to different files).
//
// A function rather than an inline loop because the await=stderr fence's timeout
// path returns early and must report the same capture the normal tail does;
// duplicating the loop is how the two drift.
func collectPeerStreams(peers []peerOutput) (stdout, stderr string) {
	var out, errOut strings.Builder
	for i := range peers {
		out.WriteString(peers[i].stdout.String())
		errOut.WriteString(peers[i].stderr.String())
	}
	return out.String(), errOut.String()
}

// collectPeerOutput is the rec.PeerOutput shape: all peer stdout followed by all
// peer stderr (extractReceivedMessages parses it).
func collectPeerOutput(peers []peerOutput) string {
	stdout, stderr := collectPeerStreams(peers)
	return stdout + stderr
}

// drainPeers waits every launched peer process that has not been waited yet, so
// os/exec's per-stream copy goroutines have finished and combined() returns the
// peer's COMPLETE output.
//
// The verdict reads each check peer's capture, and a peer that is still running
// may not have had its final "successful" line copied out of the pipe. That was
// harmless while any one peer's success carried the whole test; once every check
// peer must report for itself, an undrained capture is a spurious red -- exactly
// the flakiness the per-peer verdict exists to remove.
//
// Bounded: a peer that never exits must not hang the run. On expiry the capture
// is read as-is, which is the pre-existing behavior, and the peer's own failure
// (or the test timeout) reports it.
func drainPeers(peers []peerOutput, grace time.Duration) {
	done := make(chan int, len(peers))
	pending := 0
	for i := range peers {
		if peers[i].proc == nil || peers[i].waited {
			continue
		}
		pending++
		go func(idx int) {
			_ = peers[idx].proc.Wait() //nolint:errcheck // the verdict reads output, not this status
			done <- idx
		}(i)
	}
	if pending == 0 {
		return
	}
	timer := time.NewTimer(grace)
	defer timer.Stop()
	for range pending {
		select {
		case idx := <-done:
			peers[idx].waited = true
		case <-timer.C:
			return
		}
	}
}

// combined returns this ONE peer's stdout followed by its stderr.
//
// Deliberately per-peer. rec.PeerOutput joins every peer's output, which is fine
// for diagnostics but must never be what a pass/fail decision reads: see
// peer_contract.go failedCheckPeers.
func (p *peerOutput) combined() string {
	return p.stdout.String() + p.stderr.String()
}

// waitReady polls for a readiness file, returning when found or timeout expires.
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

func zeDaemonConfigArgIndex(args []string) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		// "start" verb precedes a config path after spec-fixit-config-file-positional-grammar
		case "start", "-d", "--debug", "--insecure-web", "--color", "--no-color":
			continue
		case "-f", "--server", "--name", "--token", "--plugin", "--pprof", "--chaos-seed", "--chaos-rate", "--mcp", "--mcp-token", "--web":
			i++
			continue
		}

		if arg == "-" || strings.HasSuffix(arg, ".conf") || strings.HasSuffix(arg, ".cfg") || strings.HasSuffix(arg, ".yaml") || strings.HasSuffix(arg, ".yml") || strings.HasSuffix(arg, ".json") {
			return i
		}
		if strings.Contains(arg, string(filepath.Separator)) || strings.HasPrefix(arg, ".") {
			return i
		}
		return -1
	}
	return -1
}

func zeDaemonUsesWeb(args []string) bool {
	for _, arg := range args {
		if arg == "--web" || strings.HasPrefix(arg, "--web=") {
			return true
		}
	}
	return false
}

func zeDaemonShouldForceFileStorage(args []string) bool {
	return zeDaemonConfigArgIndex(args) >= 0 && !zeDaemonUsesWeb(args)
}

// zeDaemonVerbs are the ze subcommands that start a long-running or blocking
// process (they run until the runner kills them), as opposed to the many
// offline/one-shot subcommands (config, show, bgp decode, format, doctor,
// explain, ...) that run to completion and exit. Config-file and web-server
// invocations (`ze -`, `ze x.conf`, `ze --web 8080 ...`) are also daemons; those
// are detected structurally (zeDaemonConfigArgIndex / zeDaemonUsesWeb) rather
// than by verb. cli/monitor block on stdin or stream continuously.
var zeDaemonVerbs = map[string]bool{
	"hub":     true,
	"start":   true,
	"cli":     true,
	"monitor": true,
}

// firstZeSubcommand returns the first non-flag token in a ze argument list (the
// subcommand verb), skipping leading option flags. A bare "-" is the daemon
// "read config from stdin" sentinel, not a verb, so it is treated as a flag and
// produces an empty result. Returns "" when no verb is present.
func firstZeSubcommand(args []string) string {
	for _, arg := range args {
		if arg == "-" || strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return ""
}

// isQuickExitZeCommand reports whether a foreground ze invocation is a
// quick-exit (non-daemon) subcommand that must be AWAITED before the next
// command in the same test starts. Two un-awaited quick-exit ze steps in one
// .ci file (e.g. a valid then an invalid `ze config validate -`, or the 14
// `ze pipe ...` steps in pipe-operators) otherwise run concurrently and
// race on the shared client stdout/stderr buffers, so a later step's output
// clobbers (or loses) an earlier step's and the per-test expect=stdout/stderr
// check sees the wrong text.
//
// The classification is by exclusion, so it covers every offline subcommand
// without enumerating them: a foreground ze is quick-exit unless it is a daemon.
// It is a daemon when it carries a config-file argument (zeDaemonConfigArgIndex
// >= 0: `ze -`, `ze x.conf`, `ze --plugin ... -`), runs a web server
// (zeDaemonUsesWeb: `ze --web 8080 --insecure-web`, with HTTP checks polling
// it), or names an explicit daemon verb (zeDaemonVerbs: hub/start/cli/monitor).
// Daemons are NOT awaited here: they are synchronized via a ze-peer, readiness
// file, or HTTP wait and torn down at the end of the run. Mis-classifying a
// daemon as quick-exit would block the loop forever, so the daemon guards come
// first.
func isQuickExitZeCommand(args []string) bool {
	if zeDaemonConfigArgIndex(args) >= 0 || zeDaemonUsesWeb(args) {
		return false
	}
	return !zeDaemonVerbs[firstZeSubcommand(args)]
}

// startWithETXTBSYRetry starts proc, retrying on ETXTBSY -- which occurs when a
// concurrent fork+exec in another test goroutine holds a write-open fd to the
// binary between fork and execve (https://go.dev/issue/22315). Go 1.25+ marks a
// Cmd as started even on failure (https://go.dev/issue/77075), so a fresh Cmd is
// created for each retry; the (possibly recreated) proc is returned along with
// the final start error.
func startWithETXTBSYRetry(ctx context.Context, binPath string, args []string, proc *exec.Cmd) (*exec.Cmd, error) {
	var startErr error
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(10 * time.Millisecond)
			old := proc
			proc = exec.CommandContext(ctx, binPath, args...) //nolint:gosec // test runner
			proc.Env = old.Env
			proc.Dir = old.Dir
			proc.Stdin = old.Stdin
			proc.Stdout = old.Stdout
			proc.Stderr = old.Stderr
			// Preserve the netns-mode credential drop (Fix B); losing it on retry
			// would silently re-run ze as root and break the readiness handshake.
			proc.SysProcAttr = old.SysProcAttr
		}
		startErr = proc.Start()
		if startErr == nil || !errors.Is(startErr, syscall.ETXTBSY) {
			break
		}
	}
	return proc, startErr
}

// awaitQuickZe waits for a foreground quick-exit ze command to finish, then
// folds its isolated stdout/stderr into the shared client buffers in start
// order. Because the fold happens only after Wait() returns, sequential
// quick-exit steps never write the shared builders concurrently. It returns the
// command's exit error for the expect=exit check.
// The client accumulators are *lockedBuilder, not *strings.Builder: this merge
// runs on the runner's own goroutine while background processes' os/exec copy
// goroutines are still appending to the same two buffers.
func awaitQuickZe(proc *exec.Cmd, quickStdout, quickStderr *strings.Builder, clientStdout, clientStderr *lockedBuilder) error {
	waitErr := proc.Wait()
	//nolint:errcheck // lockedBuilder.WriteString never returns a non-nil error
	clientStdout.WriteString(quickStdout.String())
	//nolint:errcheck // lockedBuilder.WriteString never returns a non-nil error
	clientStderr.WriteString(quickStderr.String())
	return waitErr
}

// teardownGraceTimeout is how long a SIGTERM'd process is given to exit before it
// is forcefully killed. It is a single shared constant: every teardown/stop site
// uses the same grace period, so it is not a per-call parameter.
const teardownGraceTimeout = 2 * time.Second

// terminateGracefully sends SIGTERM to a process and waits for it to exit.
// If it doesn't exit within teardownGraceTimeout, it is forcefully killed.
func terminateGracefully(cmd *exec.Cmd) {
	if cmd.Process == nil {
		_ = cmd.Wait()
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	timer := time.AfterFunc(teardownGraceTimeout, func() {
		_ = cmd.Process.Kill()
	})
	_ = cmd.Wait()
	timer.Stop()
}

// stopNamedBackground executes a cmd=stop step: it looks the named process up in
// namedBg, terminates and reaps it, then removes it from both namedBg and the
// bgProcs tracking slice so teardown never double-handles it and fgProc selection
// skips it. It returns the stopped *exec.Cmd (so the caller can also drop it from
// any other tracking, e.g. peerOutputs, and never Wait() a reaped process twice)
// and the pruned bgProcs slice. A name that matches no tracked background process
// is a hard error (AC-2, fail-closed): the directive can only ever signal a
// process the runner itself started, never an arbitrary PID.
func stopNamedBackground(cmd RunCommand, bgProcs []*exec.Cmd, namedBg map[string]*exec.Cmd) (*exec.Cmd, []*exec.Cmd, error) {
	proc, ok := namedBg[cmd.Name]
	if !ok {
		return nil, bgProcs, fmt.Errorf("cmd seq=%d: stop names unknown background process %q "+
			"(no cmd=background started with name=%s)", cmd.Seq, cmd.Name, cmd.Name)
	}
	stopBackgroundProcess(proc, cmd.Signal)
	delete(namedBg, cmd.Name)
	for i := range bgProcs {
		if bgProcs[i] == proc {
			bgProcs = append(bgProcs[:i], bgProcs[i+1:]...)
			break
		}
	}
	return proc, bgProcs, nil
}

// stopBackgroundProcess terminates a tracked background process mid-test on
// behalf of a cmd=stop directive, then reaps it so the next step runs against a
// process the OS has actually torn down (AC-1). With signalKill (the default) it
// SIGKILLs the process -- the killed peer goes silent, the condition IKEv2 DPD
// (dead-peer detection) observes. With signalTerm it delegates to
// terminateGracefully (SIGTERM, then SIGKILL after teardownGraceTimeout) so a test
// wanting a clean stop gets one and the runner never hangs. Errors are ignored
// throughout: the whole point is that the process is gone, and a double reap in
// teardown is harmless (AC-3).
func stopBackgroundProcess(cmd *exec.Cmd, signal string) {
	if cmd.Process == nil {
		_ = cmd.Wait()
		return
	}
	if signal == signalTerm {
		terminateGracefully(cmd)
		return
	}
	// Default: SIGKILL. The process cannot flush or send a protocol teardown,
	// which is exactly what the DPD proof requires (a clean DELETE would defeat
	// AC-4). Wait reaps the killed process; it returns fast because the kill has
	// already fired.
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// resolveOrchestratedTimeout picks the timeout for an orchestrated (cmd=) test.
// Most specific wins: a per-command timeout= on a foreground cmd, then the
// record-level `option=timeout:value=`, then the caller's suggested value
// (baseline-derived, itself capped by the global default).
//
// The record-level option was previously read only on the non-orchestrated path,
// which Run returns from before reaching it. Every test that declared
// `option=timeout:` alongside a `cmd=` directive therefore ran on the global
// default with its declaration silently ignored -- a stated timeout that did
// nothing. An option that is accepted and then discarded is worse than one that
// is rejected, because the .ci file reads as if the budget were set.
func resolveOrchestratedTimeout(suggested time.Duration, recordTimeout string, cmds []RunCommand) time.Duration {
	timeout := suggested
	if d, err := time.ParseDuration(recordTimeout); recordTimeout != "" && err == nil {
		timeout = d
	}
	for _, cmd := range cmds {
		if cmd.Mode == modeForeground && cmd.Timeout != "" {
			if d, err := time.ParseDuration(cmd.Timeout); err == nil {
				timeout = d
			}
		}
	}
	return timeout
}
