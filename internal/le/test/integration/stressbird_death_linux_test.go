//go:build linux

// Design: docs/functional-tests.md -- the stress harness cleanup: every child dies with the harness.
// Related: stressbird_system.go -- realStressBirdSystem.Start and Run, the forks these tests drive.
// Related: ../../../test/runner/process_death_linux_test.go -- the .ci runner's twin of these tests.

package testintegration

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// stressDeathRoleEnv selects the role of a re-executed test binary. Only
// TestStressHarnessKilledOutrightTakesItsChildren sets it, on the stand-in
// harness it starts; every other run of TestStressHarnessDeathStandIn skips.
const stressDeathRoleEnv = "STRESS_DEATH_ROLE"

// stressDeathDirEnv names the parent test's temporary directory, where the
// stand-in harness writes its child's log and pid file. The parent owns the
// directory, so its cleanup removes them even though the stand-in is killed.
const stressDeathDirEnv = "STRESS_DEATH_DIR"

// The stand-in harness's roles, one per fork the stress system offers.
const (
	// stressDeathRoleStart starts a long-lived child through Start, as the
	// harness starts ze, BIRD, the test peers, tcpdump and the profile curl.
	stressDeathRoleStart = "start"
	// stressDeathRoleRun runs a command through Run, as the harness runs ip,
	// ethtool, birdc, ss and the DUT build.
	stressDeathRoleRun = "run"
)

// stressChildPIDPrefix marks the stand-in harness's stdout line carrying its
// child's pid, so the parent finds it among anything else the binary prints.
const stressChildPIDPrefix = "stress-death-child-pid="

// stressDeathBound is how long a child may outlive the event that must end it:
// the harness's death, a stop, or a Run timeout.
const stressDeathBound = 5 * time.Second

// TestStressHarnessKilledOutrightTakesItsChildren proves a child the stress
// harness forked dies when the harness dies without running any Go code
// (AC-1 for Start, AC-2 for Run).
//
// Method: re-execute this test binary as a stand-in harness. It forks a
// `sleep 300` through the real realStressBirdSystem, in the role the subtest
// names, prints the child's pid, and blocks. The test reads the pid and the
// child's start time from /proc, SIGKILLs the stand-in, and requires the child
// to be gone within 5 s. The child is not this process's child once its
// harness dies (it is reparented), so gone means its /proc entry is absent, or
// the pid now belongs to a process with another start time, or it is a zombie
// whose wait status is death by SIGKILL, the signal the kernel sends on its
// parent's death. When the child survives, the cleanup kills it, so a red run
// leaks nothing.
func TestStressHarnessKilledOutrightTakesItsChildren(t *testing.T) {
	for _, role := range []string{stressDeathRoleStart, stressDeathRoleRun} {
		t.Run(role, func(t *testing.T) {
			stressHarnessKilledOutrightTakesItsChild(t, role)
		})
	}
}

// stressHarnessKilledOutrightTakesItsChild runs
// TestStressHarnessKilledOutrightTakesItsChildren against a stand-in harness
// in role.
func stressHarnessKilledOutrightTakesItsChild(t *testing.T, role string) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	harness := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStressHarnessDeathStandIn$")
	harness.Env = append(os.Environ(), stressDeathRoleEnv+"="+role, stressDeathDirEnv+"="+t.TempDir())
	harness.Stderr = os.Stderr
	output, err := harness.StdoutPipe()
	if err != nil {
		t.Fatalf("pipe the stand-in harness's stdout: %v", err)
	}
	if err := harness.Start(); err != nil {
		t.Fatalf("start the stand-in harness: %v", err)
	}
	t.Cleanup(func() { reapStressStandIn(harness) })

	pid, err := readStressChildPID(output)
	if err != nil {
		t.Fatalf("read the stand-in harness's child pid: %v", err)
	}
	child := requireStressChildRunning(t, pid, "before its harness was killed")

	if err := harness.Process.Kill(); err != nil {
		t.Fatalf("SIGKILL the stand-in harness: %v", err)
	}
	awaitStressChildGone(t, pid, child.startTime, stressDeathSIGKILL, "its SIGKILLed harness")
}

// TestStressHarnessDeathStandIn is the stand-in harness of
// TestStressHarnessKilledOutrightTakesItsChildren and does nothing in any
// other run. It forks its child through the real realStressBirdSystem in the
// role the parent names, reports the child's pid, then blocks until the parent
// SIGKILLs it.
func TestStressHarnessDeathStandIn(t *testing.T) {
	dir := os.Getenv(stressDeathDirEnv)
	switch os.Getenv(stressDeathRoleEnv) {
	case stressDeathRoleStart:
		standInStartsAChild(t, dir)
	case stressDeathRoleRun:
		standInRunsACommand(t, dir)
	default:
		t.Skip("helper process for TestStressHarnessKilledOutrightTakesItsChildren")
	}
}

// standInStartsAChild forks `sleep 300` through Start, reports its pid, and
// waits on it. A return means the child ended first, which the parent then
// reports as a child that was gone before its harness was killed.
func standInStartsAChild(t *testing.T, dir string) {
	process, err := realStressBirdSystem{}.Start(t.Context(), stressBirdCommand{
		argv: []string{"sleep", "300"}, environ: os.Environ(),
		outputPath: filepath.Join(dir, "child.log"),
	})
	if err != nil {
		t.Fatalf("start the child: %v", err)
	}
	reportStressChildPID(t, startedStressChildPID(t, process))
	code, waitErr := process.Wait(time.Hour)
	t.Fatalf("the child ended before the stand-in harness was killed: code %d, %v", code, waitErr)
}

// standInRunsACommand runs a shell that writes its own pid and execs
// `sleep 300` through Run, which blocks, so the pid comes back through the
// pid file rather than through Run's result. It reports the pid, then blocks
// until the parent kills it.
func standInRunsACommand(t *testing.T, dir string) {
	pidFile := filepath.Join(dir, "child.pid")
	ended := make(chan error, 1)
	go runStressCommandInto(t.Context(), stressBirdCommand{
		argv:    []string{"sh", "-c", "echo $$ > " + pidFile + "; exec sleep 300"},
		environ: os.Environ(),
	}, ended)
	pid, err := awaitStressPIDFile(t.Context(), pidFile)
	if err != nil {
		t.Fatalf("read the command's pid: %v", err)
	}
	reportStressChildPID(t, pid)
	err = <-ended
	t.Fatalf("the command ended before the stand-in harness was killed: %v", err)
}

// runStressCommandInto runs command through the real Run and hands its error
// to ended, so a caller can block on Run from another goroutine.
func runStressCommandInto(ctx context.Context, command stressBirdCommand, ended chan<- error) {
	_, err := realStressBirdSystem{}.Run(ctx, command)
	ended <- err
}

// TestStressStopReachesTheChildsWholeGroup proves the harness's stop ends the
// child's whole tree, not only the pid it forked (AC-3). ze forks its plugins
// and BIRD may fork helpers, so a stop aimed at the direct pid leaves them.
//
// Method: Start a shell that backgrounds `sleep 300`, writes the sleep's pid,
// and waits. Stop it as cleanup does: Terminate, a bounded Wait, then Kill and
// Wait when the bound expires. Once Wait has returned, the backgrounded sleep
// must be gone within 5 s. A zombie counts as gone whatever its status, since
// the stop sends SIGTERM first.
func TestStressStopReachesTheChildsWholeGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "grandchild.pid")
	process, err := realStressBirdSystem{}.Start(t.Context(), stressBirdCommand{
		argv:       []string{"sh", "-c", "sleep 300 & echo $! > " + pidFile + "; wait"},
		environ:    os.Environ(),
		outputPath: filepath.Join(dir, "child.log"),
	})
	if err != nil {
		t.Fatalf("start the child: %v", err)
	}
	t.Cleanup(func() { stopStressProcess(t, process) })

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	pid, err := awaitStressPIDFile(ctx, pidFile)
	if err != nil {
		t.Fatalf("read the grandchild's pid: %v", err)
	}
	grandchild := requireStressChildRunning(t, pid, "before the stop")

	stopStressProcess(t, process)
	awaitStressChildGone(t, pid, grandchild.startTime, stressDeathAnySignal, "its parent's stop")
}

// TestStressChildLivesWhileItsHarnessLives proves a stress child lives while
// its harness lives, even after the goroutine that asked for the start has
// returned from a locked OS thread (AC-4, R-1).
//
// The kernel delivers a parent-death signal when the THREAD that forked the
// child exits, and the Go runtime destroys a thread whose goroutine returns
// while locked to it. A start that sets the death signal and forks on the
// caller's thread therefore kills its child mid-round.
//
// Method: ask for the start from a goroutine that locks its thread and returns
// still locked, give the kernel a second to deliver a death signal, and require
// the child to be running. Then stop it as cleanup does.
func TestStressChildLivesWhileItsHarnessLives(t *testing.T) {
	started := make(chan stressStart, 1)
	go startStressFromDoomedThread(t.Context(), filepath.Join(t.TempDir(), "child.log"), started)
	result := <-started
	if result.err != nil {
		t.Fatalf("start the child: %v", result.err)
	}
	process := result.process
	t.Cleanup(func() { stopStressProcess(t, process) })

	time.Sleep(time.Second)
	exited, code, err := process.Exited()
	if err != nil {
		t.Fatalf("inspect the child: %v", err)
	}
	if exited {
		t.Fatalf("the child ended with code %d while its harness lived, after the thread that asked for its start exited", code)
	}
	stopStressProcess(t, process)
}

// stressStart carries the result of a start requested from another goroutine.
type stressStart struct {
	process stressBirdProcess
	err     error
}

// startStressFromDoomedThread asks for a long-lived child's start through the
// real Start from a goroutine that leaves its OS thread locked, so the runtime
// destroys that thread when it returns.
func startStressFromDoomedThread(ctx context.Context, outputPath string, started chan<- stressStart) {
	runtime.LockOSThread()
	process, err := realStressBirdSystem{}.Start(ctx, stressBirdCommand{
		argv: []string{"sleep", "300"}, environ: os.Environ(), outputPath: outputPath,
	})
	started <- stressStart{process: process, err: err}
}

// TestStressRunTimeoutReachesTheWholeGroup proves a Run that its timeout ends
// leaves nothing of the command's tree running (AC-5).
//
// Method: Run a shell that backgrounds `sleep 300`, writes the sleep's pid, and
// waits, under a 300 ms timeout. Run must answer the deadline error, and the
// backgrounded sleep must then be gone within 5 s. The sleep's output goes to
// /dev/null, so it never holds Run's output pipes: the red this test owes is
// the surviving sleep, not a Run that hangs on a pipe. The whole test is
// bounded, and the cleanup kills a surviving sleep.
func TestStressRunTimeoutReachesTheWholeGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	_, err := realStressBirdSystem{}.Run(ctx, stressBirdCommand{
		argv: []string{
			"sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $! > " + pidFile + "; wait",
		},
		environ: os.Environ(),
		timeout: 300 * time.Millisecond,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run answered %v, want the deadline error", err)
	}
	pid, err := awaitStressPIDFile(ctx, pidFile)
	if err != nil {
		t.Fatalf("read the grandchild's pid: %v", err)
	}
	grandchild, found, err := readStressProcStat(pid)
	if err != nil {
		t.Fatalf("read the grandchild's /proc entry: %v", err)
	}
	if !found {
		return
	}
	t.Cleanup(func() { killSurvivingStressChild(t, pid, grandchild.startTime) })
	awaitStressChildGone(t, pid, grandchild.startTime, stressDeathAnySignal, "its command's timeout")
}

// stressDeathSignal names the end a zombie child must show.
type stressDeathSignal int

const (
	stressDeathUnspecified stressDeathSignal = iota
	// stressDeathSIGKILL is the kernel's parent-death signal: any other end is
	// not the route under test.
	stressDeathSIGKILL
	// stressDeathAnySignal accepts any end: a stop sends SIGTERM before SIGKILL.
	stressDeathAnySignal
)

// requireStressChildRunning reads pid's /proc entry and fails unless it is a
// live process. It registers a cleanup that kills the process if it survives
// the test, and answers its stat.
func requireStressChildRunning(t *testing.T, pid int, when string) stressProcStat {
	t.Helper()
	child, found, err := readStressProcStat(pid)
	if err != nil {
		t.Fatalf("read the child's /proc entry: %v", err)
	}
	if !found {
		t.Fatalf("the child %d was gone %s", pid, when)
	}
	if child.state == 'Z' {
		t.Fatalf("the child %d was a zombie %s", pid, when)
	}
	t.Cleanup(func() { killSurvivingStressChild(t, pid, child.startTime) })
	return child
}

// awaitStressChildGone polls pid's /proc entry for stressDeathBound and fails
// unless the process it named is gone: absent, replaced by a process with
// another start time, or a zombie whose end matches want.
func awaitStressChildGone(t *testing.T, pid int, startTime string, want stressDeathSignal, cause string) {
	t.Helper()
	deadline := time.Now().Add(stressDeathBound)
	for {
		now, found, err := readStressProcStat(pid)
		if err != nil {
			t.Fatalf("read the child's /proc entry after %s: %v", cause, err)
		}
		if !found {
			return
		}
		if now.startTime != startTime {
			return
		}
		if now.state == 'Z' {
			requireStressZombieEnd(t, pid, now.exitCode, want)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the child %d outlived %s: still in state %c %v later", pid, cause, now.state, stressDeathBound)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// requireStressZombieEnd checks a zombie's wait status against want.
func requireStressZombieEnd(t *testing.T, pid, exitCode int, want stressDeathSignal) {
	t.Helper()
	status := syscall.WaitStatus(exitCode)
	switch want {
	case stressDeathAnySignal:
		return
	case stressDeathSIGKILL:
		if !status.Signaled() {
			t.Fatalf("the child %d exited with status %d, not by SIGKILL", pid, status.ExitStatus())
		}
		if status.Signal() != syscall.SIGKILL {
			t.Fatalf("the child %d died of %v, not SIGKILL", pid, status.Signal())
		}
	case stressDeathUnspecified:
		t.Fatalf("BUG: no expected end named for the child %d", pid)
	}
}

// startedStressChildPID answers the pid of a child the real Start forked.
func startedStressChildPID(t *testing.T, process stressBirdProcess) int {
	t.Helper()
	started, ok := process.(*realStressBirdProcess)
	if !ok {
		t.Fatalf("Start answered %T, want *realStressBirdProcess", process)
	}
	return started.cmd.Process.Pid
}

// reportStressChildPID writes the pid line the parent test reads.
func reportStressChildPID(t *testing.T, pid int) {
	t.Helper()
	line := stressChildPIDPrefix + strconv.Itoa(pid) + "\n"
	if _, err := os.Stdout.WriteString(line); err != nil {
		t.Fatalf("report the child pid: %v", err)
	}
}

// stopStressProcess stops process as stressBirdRunner.cleanup does:
// Terminate, a bounded Wait, then Kill and Wait when the bound expires. A
// second call on a finished process returns at once.
func stopStressProcess(t *testing.T, process stressBirdProcess) {
	t.Helper()
	if err := process.Terminate(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("terminate the child: %v", err)
	}
	_, err := process.Wait(stressBirdStopTimeout)
	if !errors.Is(err, errStressBirdWaitTimeout) {
		return
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("kill the child: %v", err)
	}
	if _, err := process.Wait(stressBirdStopTimeout); errors.Is(err, errStressBirdWaitTimeout) {
		t.Errorf("the child was not reaped %v after SIGKILL", stressBirdStopTimeout)
	}
}

// awaitStressPIDFile polls path until it holds a complete pid line, or ctx
// ends.
func awaitStressPIDFile(ctx context.Context, path string) (int, error) {
	for {
		raw, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return 0, err
		}
		if text, complete := strings.CutSuffix(string(raw), "\n"); complete {
			return strconv.Atoi(text)
		}
		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("no pid in %s: %w", path, ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// stressProcStat holds the fields of /proc/<pid>/stat the death tests read.
type stressProcStat struct {
	state     byte
	startTime string // field 22: clock ticks after boot, the pid's identity
	exitCode  int    // field 52: the wait status, meaningful for a zombie
}

// readStressProcStat reads /proc/<pid>/stat. found is false only when the
// process does not exist; any other failure to read is an error, never "gone".
func readStressProcStat(pid int) (stat stressProcStat, found bool, err error) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, fs.ErrNotExist) {
		return stressProcStat{}, false, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return stressProcStat{}, false, nil
	}
	if err != nil {
		return stressProcStat{}, false, err
	}
	// The command name in field 2 is parenthesised and may hold spaces, so
	// field 3 onwards is everything after the last closing parenthesis.
	end := strings.LastIndexByte(string(raw), ')')
	if end < 0 {
		return stressProcStat{}, false, fmt.Errorf("no command name in /proc/%d/stat", pid)
	}
	fields := strings.Fields(string(raw[end+1:]))
	// Field N of stat(5) is fields[N-3].
	if len(fields) < 52-2 {
		return stressProcStat{}, false, fmt.Errorf("short /proc/%d/stat: %d fields", pid, len(fields)+2)
	}
	exitCode, err := strconv.Atoi(fields[52-3])
	if err != nil {
		return stressProcStat{}, false, fmt.Errorf("exit code field of /proc/%d/stat: %w", pid, err)
	}
	return stressProcStat{state: fields[0][0], startTime: fields[22-3], exitCode: exitCode}, true, nil
}

// readStressChildPID reads the stand-in harness's stdout until the line
// carrying its child's pid. The caller bounds it: the harness's context kills
// the harness, which closes the pipe and ends the read.
func readStressChildPID(output io.Reader) (int, error) {
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		value, ok := strings.CutPrefix(scanner.Text(), stressChildPIDPrefix)
		if !ok {
			continue
		}
		return strconv.Atoi(value)
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, errors.New("the stand-in harness exited without reporting its child pid")
}

// killSurvivingStressChild SIGKILLs the child when it is still the process the
// test recorded and still alive, so a red run never leaks it. A pid now held by
// a process with another start time belongs to someone else and is left alone.
func killSurvivingStressChild(t *testing.T, pid int, startTime string) {
	t.Helper()
	now, found, err := readStressProcStat(pid)
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

// reapStressStandIn kills the stand-in harness if it still runs and reaps it.
// Its exit is the SIGKILL the test sent, so the wait error carries nothing to
// assert.
func reapStressStandIn(harness *exec.Cmd) {
	_ = harness.Process.Kill() //nolint:errcheck // the harness is normally dead already
	_ = harness.Wait()         //nolint:errcheck // the exit is the test's own SIGKILL
}
