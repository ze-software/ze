//go:build linux

// Design: docs/architecture/testing/runner-architecture.md -- three things end a process: the runner dying outright.
// Related: runner_exec_util.go -- startWithETXTBSYRetry, the start every long-lived .ci child takes.

package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/userns"
)

// runnerDeathRoleEnv selects the role of a re-executed test binary. Only
// TestRunnerKilledOutrightTakesItsChildren sets it, on the stand-in runner it
// starts; every other run of TestRunnerDeathStandIn skips.
const runnerDeathRoleEnv = "RUNNER_DEATH_ROLE"

// The stand-in runner's roles, one per fork thread the .ci start path accepts.
const (
	// runnerDeathRoleSpawner forks through plugin.StartChild's spawner thread,
	// as every start outside netns mode does.
	runnerDeathRoleSpawner = "runner"
	// runnerDeathRoleLocked locks the test goroutine's thread and forks on it,
	// as a netns-mode test does on the thread enterTestNetns locked (AC-4).
	runnerDeathRoleLocked = "runner-locked"
)

// childPIDPrefix marks the stand-in runner's stdout line carrying its child's
// pid, so the parent finds it among anything else the test binary prints.
const childPIDPrefix = "runner-death-child-pid="

// TestRunnerKilledOutrightTakesItsChildren proves a child started through the
// .ci start path dies when its runner dies without running any Go code (AC-1).
//
// Method: re-execute this test binary as a stand-in runner. It starts
// `sleep 300` through the real startWithETXTBSYRetry, prints the child's pid,
// and waits on it. The test reads the pid and the child's start time from
// /proc, SIGKILLs the stand-in, and requires the child to be gone within 5 s.
// The child is not this process's child once its runner dies (it is
// reparented), so gone means its /proc entry is absent, its state is Z, or the
// pid now belongs to a process with another start time. A zombie also carries
// its wait status, and that status must be death by SIGKILL. When the child
// survives, the cleanup kills it, so a red run leaks nothing.
//
// It runs once per fork thread: the spawner thread every start outside netns
// mode takes, and a locked test thread, the route a netns-mode child takes
// (AC-4). The namespace itself needs privilege and is not entered here; the
// locked case proves the death signal on the thread netns mode forks from.
func TestRunnerKilledOutrightTakesItsChildren(t *testing.T) {
	for _, role := range []string{runnerDeathRoleSpawner, runnerDeathRoleLocked} {
		t.Run(role, func(t *testing.T) {
			runnerKilledOutrightTakesItsChild(t, role)
		})
	}
}

// runnerKilledOutrightTakesItsChild runs TestRunnerKilledOutrightTakesItsChildren
// against a stand-in runner in role.
func runnerKilledOutrightTakesItsChild(t *testing.T, role string) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	runner := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunnerDeathStandIn$")
	runner.Env = append(os.Environ(), runnerDeathRoleEnv+"="+role)
	runner.Stderr = os.Stderr
	output, err := runner.StdoutPipe()
	if err != nil {
		t.Fatalf("pipe the stand-in runner's stdout: %v", err)
	}
	if err := runner.Start(); err != nil {
		t.Fatalf("start the stand-in runner: %v", err)
	}
	t.Cleanup(func() { reapStandIn(runner) })

	pid, err := readChildPID(output)
	if err != nil {
		t.Fatalf("read the stand-in runner's child pid: %v", err)
	}
	child, found, err := readProcStat(pid)
	if err != nil {
		t.Fatalf("read the child's /proc entry: %v", err)
	}
	if !found {
		t.Fatalf("the child %d was gone before its runner was killed", pid)
	}
	if child.state == 'Z' {
		t.Fatalf("the child %d was a zombie before its runner was killed", pid)
	}
	t.Cleanup(func() { killSurvivingChild(t, pid, child.startTime) })

	if err := runner.Process.Kill(); err != nil {
		t.Fatalf("SIGKILL the stand-in runner: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		now, found, err := readProcStat(pid)
		if err != nil {
			t.Fatalf("read the child's /proc entry after its runner died: %v", err)
		}
		if !found {
			return
		}
		if now.startTime != child.startTime {
			return
		}
		if now.state == 'Z' {
			requireKilledBySIGKILL(t, pid, now.exitCode)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the child %d outlived its SIGKILLed runner: still in state %c 5s after the runner died", pid, now.state)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestRunnerDeathStandIn is the stand-in runner of
// TestRunnerKilledOutrightTakesItsChildren and does nothing in any other run.
// It starts its child through the real .ci start path, reports the child's
// pid, then waits on the child until the parent test SIGKILLs it. The role
// names the fork thread; the locked role holds its thread locked until it is
// killed, as enterTestNetns holds a netns-mode test's thread.
func TestRunnerDeathStandIn(t *testing.T) {
	thread := forkThreadUnspecified
	switch os.Getenv(runnerDeathRoleEnv) {
	case runnerDeathRoleSpawner:
		thread = forkThreadSpawner
	case runnerDeathRoleLocked:
		runtime.LockOSThread()
		thread = forkThreadLocked
	default:
		t.Skip("helper process for TestRunnerKilledOutrightTakesItsChildren")
	}
	ctx := t.Context()
	proc := exec.CommandContext(ctx, "sleep", "300")
	owned, err := startWithETXTBSYRetry(ctx, proc.Path, proc.Args[1:], proc, nil, thread)
	if err != nil {
		t.Fatalf("start the child: %v", err)
	}
	defer owned.cleanup()

	line := childPIDPrefix + strconv.Itoa(owned.cmd.Process.Pid) + "\n"
	if _, err := os.Stdout.WriteString(line); err != nil {
		t.Fatalf("report the child pid: %v", err)
	}
	// The parent SIGKILLs this process while it waits here. A return means the
	// child ended first, which the parent then reports as a missing child.
	waitErr := owned.Wait()
	t.Fatalf("the child ended before the stand-in runner was killed: %v", waitErr)
}

// TestOwnedChildLivesWhileItsRunnerLives proves a child started through the
// .ci start path lives while its runner lives, even after the goroutine that
// asked for the start has returned from a locked OS thread (AC-2, R-3).
//
// The kernel delivers a parent-death signal when the THREAD that forked the
// child exits, and the Go runtime destroys a thread whose goroutine returns
// while locked to it. A start that sets the death signal and forks on the
// caller's thread therefore kills its child mid-test.
//
// Method: ask for the start from a goroutine that locks its thread and returns
// still locked, give the kernel a second to deliver a death signal, and require
// the child to be running. Then stop it through its owner and join it.
func TestOwnedChildLivesWhileItsRunnerLives(t *testing.T) {
	started := make(chan ownedStart, 1)
	go startOwnedFromDoomedThread(t.Context(), started)
	result := <-started
	if result.err != nil {
		t.Fatalf("start the child: %v", result.err)
	}
	owned := result.owned
	defer owned.cleanup()

	select {
	case <-owned.done:
		t.Fatalf("the child %d ended while its runner lived, after the thread that asked for its start exited: %v", owned.cmd.Process.Pid, owned.err)
	case <-time.After(time.Second):
	}

	owned.cleanup()
	if err := owned.lifecycleError(); err != nil {
		t.Errorf("stop the child through its owner: %v", err)
	}
}

// TestRetriedStartKeepsTheDeathSignal proves a start retried after ETXTBSY
// forks its fresh Cmd with the same attributes as the first attempt: the
// caller's credential, its own process group and the death signal (AC-3, R-4).
//
// Method: a real ETXTBSY needs another goroutine's fork to race this exec, so
// the test drives the real retry loop (startWithRetry) with a fork operation
// that answers ETXTBSY to the first attempt and forks the second through
// plugin.StartChild for real. It then reads the attributes off the Cmd each
// attempt was handed, requires the owner to hold the second one, and checks
// the started child leads its own process group.
func TestRetriedStartKeepsTheDeathSignal(t *testing.T) {
	// The test's own ids, with no setgroups, so the credential is accepted
	// without privilege and still has to survive the retry.
	credential := &syscall.Credential{
		Uid:         uint32(os.Getuid()), //nolint:gosec // a uid fits uint32 on Linux
		Gid:         uint32(os.Getgid()), //nolint:gosec // a gid fits uint32 on Linux
		NoSetGroups: true,
	}
	proc := exec.CommandContext(t.Context(), "sleep", "60")
	proc.SysProcAttr = &syscall.SysProcAttr{Credential: credential}

	var attempts []*exec.Cmd
	fork := func(cmd *exec.Cmd) error {
		attempts = append(attempts, cmd)
		if len(attempts) == 1 {
			return &os.PathError{Op: "fork/exec", Path: cmd.Path, Err: syscall.ETXTBSY}
		}
		return plugin.StartChild(cmd)
	}
	owned, err := startWithRetry(t.Context(), proc.Path, proc.Args[1:], proc, nil, fork)
	if err != nil {
		t.Fatalf("start through the retry: %v", err)
	}
	defer owned.cleanup()

	if len(attempts) != 2 {
		t.Fatalf("want 2 attempts (ETXTBSY, then the retry), got %d", len(attempts))
	}
	retried := attempts[1]
	if retried == attempts[0] {
		t.Fatal("the retry reused the first attempt's Cmd")
	}
	if owned.cmd != retried {
		t.Fatal("the owner holds a Cmd the retry did not start")
	}
	want := plugin.NewSysProcAttr()
	for attempt, cmd := range attempts {
		attr := cmd.SysProcAttr
		if attr == nil {
			t.Fatalf("attempt %d: no SysProcAttr", attempt+1)
		}
		if attr.Pdeathsig != want.Pdeathsig {
			t.Errorf("attempt %d: Pdeathsig %v, want %v", attempt+1, attr.Pdeathsig, want.Pdeathsig)
		}
		if attr.Setpgid != want.Setpgid {
			t.Errorf("attempt %d: Setpgid %v, want %v", attempt+1, attr.Setpgid, want.Setpgid)
		}
		if attr.Credential != credential {
			t.Errorf("attempt %d: the caller's credential was not kept", attempt+1)
		}
	}
	pid := retried.Process.Pid
	group, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatalf("read the retried child's process group: %v", err)
	}
	if group != pid {
		t.Errorf("the retried child %d is in group %d, want a group of its own", pid, group)
	}

	owned.cleanup()
	if err := owned.lifecycleError(); err != nil {
		t.Errorf("stop the child through its owner: %v", err)
	}
}

// TestLockedStartForksInTheThreadsNamespace proves the .ci start path forks a
// forkThreadLocked child in the network namespace of the locked calling thread,
// and a forkThreadSpawner child in the spawner's (AC-4, A-2).
//
// netns mode depends on the first: ze and its peer must share the test's
// namespace, which only the test's locked thread is in. The second is the
// control that makes the first discriminate: a netns-mode start routed to the
// spawner would land in the host namespace, and this test would go red.
//
// Method: enter a user namespace (userns.Enter grants CAP_SYS_ADMIN without
// root), lock this thread into a fresh network namespace, start `sleep`
// through startWithETXTBSYRetry with each fork thread, and compare each
// child's /proc/<pid>/ns/net with the thread's and the host's.
func TestLockedStartForksInTheThreadsNamespace(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	hostLink, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatalf("readlink host /proc/self/ns/net: %v", err)
	}

	// The restore runs in one t.Cleanup while the thread is still locked, after
	// every child below is finished (each defer runs before the Cleanup).
	runtime.LockOSThread()
	origNS, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("get current network namespace: %v", err)
	}
	const nsName = "ze-locked-start"
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close() //nolint:errcheck // best-effort close on the failure path
		runtime.UnlockOSThread()
		t.Fatalf("create network namespace %q: %v", nsName, err)
	}
	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("restore original namespace: %v", restoreErr)
		}
		origNS.Close()            //nolint:errcheck // best-effort close on cleanup path
		newNS.Close()             //nolint:errcheck // best-effort close on cleanup path
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort cleanup
		runtime.UnlockOSThread()
	})
	threadLink, err := os.Readlink("/proc/thread-self/ns/net")
	if err != nil {
		t.Fatalf("readlink /proc/thread-self/ns/net: %v", err)
	}
	if threadLink == hostLink {
		t.Fatalf("the thread did not enter the new namespace: still %s (host)", threadLink)
	}

	locked := childNamespace(t, forkThreadLocked)
	if locked != threadLink {
		t.Errorf("a forkThreadLocked child is in namespace %s, want the locked thread's %s (host %s)", locked, threadLink, hostLink)
	}
	spawned := childNamespace(t, forkThreadSpawner)
	if spawned == threadLink {
		t.Errorf("a forkThreadSpawner child is in the locked thread's namespace %s, want the spawner's own", spawned)
	}
}

// childNamespace starts `sleep` through the .ci start path on thread and
// answers the network namespace link of the started child, then stops it
// through its owner.
func childNamespace(t *testing.T, thread forkThread) string {
	t.Helper()
	proc := exec.CommandContext(t.Context(), "sleep", "60")
	owned, err := startWithETXTBSYRetry(t.Context(), proc.Path, proc.Args[1:], proc, nil, thread)
	if err != nil {
		t.Fatalf("start the child on fork thread %d: %v", thread, err)
	}
	defer owned.cleanup()

	link, err := os.Readlink("/proc/" + strconv.Itoa(owned.cmd.Process.Pid) + "/ns/net")
	if err != nil {
		t.Fatalf("readlink the child's /proc/%d/ns/net: %v", owned.cmd.Process.Pid, err)
	}
	owned.cleanup()
	if err := owned.lifecycleError(); err != nil {
		t.Errorf("stop the child through its owner: %v", err)
	}
	return link
}

// ownedStart carries the result of a start requested from another goroutine.
type ownedStart struct {
	owned *ownedProcess
	err   error
}

// startOwnedFromDoomedThread asks for a long-lived child's start through the
// .ci start path from a goroutine that leaves its OS thread locked, so the
// runtime destroys that thread when it returns.
func startOwnedFromDoomedThread(ctx context.Context, started chan<- ownedStart) {
	runtime.LockOSThread()
	proc := exec.CommandContext(ctx, "sleep", "300")
	owned, err := startWithETXTBSYRetry(ctx, proc.Path, proc.Args[1:], proc, nil, forkThreadSpawner)
	started <- ownedStart{owned: owned, err: err}
}

// procStat holds the fields of /proc/<pid>/stat the death test reads.
type procStat struct {
	state     byte
	startTime string // field 22: clock ticks after boot, the pid's identity
	exitCode  int    // field 52: the wait status, meaningful for a zombie
}

// readProcStat reads /proc/<pid>/stat. found is false only when the process
// does not exist; any other failure to read is an error, never "gone".
func readProcStat(pid int) (stat procStat, found bool, err error) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, fs.ErrNotExist) {
		return procStat{}, false, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return procStat{}, false, nil
	}
	if err != nil {
		return procStat{}, false, err
	}
	// The command name in field 2 is parenthesised and may hold spaces, so
	// field 3 onwards is everything after the last closing parenthesis.
	end := strings.LastIndexByte(string(raw), ')')
	if end < 0 {
		return procStat{}, false, fmt.Errorf("no command name in /proc/%d/stat", pid)
	}
	fields := strings.Fields(string(raw[end+1:]))
	// Field N of stat(5) is fields[N-3].
	if len(fields) < 52-2 {
		return procStat{}, false, fmt.Errorf("short /proc/%d/stat: %d fields", pid, len(fields)+2)
	}
	exitCode, err := strconv.Atoi(fields[52-3])
	if err != nil {
		return procStat{}, false, fmt.Errorf("exit code field of /proc/%d/stat: %w", pid, err)
	}
	return procStat{state: fields[0][0], startTime: fields[22-3], exitCode: exitCode}, true, nil
}

// readChildPID reads the stand-in runner's stdout until the line carrying its
// child's pid. The caller bounds it: the runner's context kills the runner,
// which closes the pipe and ends the read.
func readChildPID(output io.Reader) (int, error) {
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		value, ok := strings.CutPrefix(scanner.Text(), childPIDPrefix)
		if !ok {
			continue
		}
		return strconv.Atoi(value)
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, errors.New("the stand-in runner exited without reporting its child pid")
}

// requireKilledBySIGKILL checks a zombie child's wait status: the kernel's
// death signal is SIGKILL, so any other end is not the route under test.
func requireKilledBySIGKILL(t *testing.T, pid, exitCode int) {
	t.Helper()
	status := syscall.WaitStatus(exitCode)
	if !status.Signaled() {
		t.Fatalf("the child %d exited with status %d after its runner died, not by SIGKILL", pid, status.ExitStatus())
	}
	if status.Signal() != syscall.SIGKILL {
		t.Fatalf("the child %d died of %v after its runner died, not SIGKILL", pid, status.Signal())
	}
}

// killSurvivingChild SIGKILLs the child when it is still the process the test
// recorded and still alive, so a red run never leaks it. A pid now held by a
// process with another start time belongs to someone else and is left alone.
func killSurvivingChild(t *testing.T, pid int, startTime string) {
	t.Helper()
	now, found, err := readProcStat(pid)
	if err != nil {
		t.Errorf("read the child's /proc entry before cleanup: %v", err)
		return
	}
	if !found {
		return
	}
	if now.startTime != startTime {
		return
	}
	if now.state == 'Z' {
		return
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Errorf("SIGKILL the surviving child %d: %v", pid, err)
	}
}

// reapStandIn kills the stand-in runner if it still runs and reaps it. Its exit
// is the SIGKILL the test sent, so the wait error carries nothing to assert.
func reapStandIn(runner *exec.Cmd) {
	_ = runner.Process.Kill() //nolint:errcheck // the runner is normally dead already
	_ = runner.Wait()         //nolint:errcheck // the exit is the test's own SIGKILL
}
