// Design: docs/architecture/testing/ci-format.md -- test execution utilities
// Overview: runner_exec.go -- test execution and process orchestration

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

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
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

// withParallelHeadroom widens a resolved per-test timeout by
// ParallelTimeoutHeadroom when this Run executes tests concurrently. Serial
// runs (-p 1 or a single selected test) keep the authored value so real
// slowdowns surface quickly. See ParallelTimeoutHeadroom for rationale.
func (r *Runner) withParallelHeadroom(timeout time.Duration) time.Duration {
	if r.concurrency > 1 {
		return timeout * ParallelTimeoutHeadroom
	}
	return timeout
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
	stderr *strings.Builder
	proc   *exec.Cmd
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
		case "-d", "--debug", "--insecure-web", "--color", "--no-color":
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
func awaitQuickZe(proc *exec.Cmd, quickStdout, quickStderr, clientStdout, clientStderr *strings.Builder) error {
	waitErr := proc.Wait()
	clientStdout.WriteString(quickStdout.String())
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
