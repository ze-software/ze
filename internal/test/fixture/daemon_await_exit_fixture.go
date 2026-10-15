// Design: docs/functional-tests.md — process orchestration, a second daemon started on the first one's file
// Related: register_daemon_await_exit.go — the registration of this barrier

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// daemonAwaitExit is a barrier between two daemons of one .ci: it returns once
// the daemon whose pid the runner published in daemon.pid has exited, so the
// next command can START a fresh daemon on the file the first one left behind.
//
// The runner does not reap a foreground daemon until the test ends, so the
// exited daemon stays a zombie and `kill(pid, 0)` still succeeds on it. The
// process state is read instead: no process, or state Z, is an exit. Its
// sockets and files are closed either way.
func daemonAwaitExit(ctx context.Context, _ []string) error {
	pid, err := readPID("daemon.pid")
	if err != nil {
		return fmt.Errorf("read daemon pid: %w", err)
	}
	var state string
	if !Poll(ctx, 600, 100*time.Millisecond, func() bool {
		state, err = daemonProcessState(ctx, pid)
		if err != nil {
			return false
		}
		if state == "" {
			return true
		}
		return strings.HasPrefix(state, "Z")
	}) {
		return fmt.Errorf("daemon %d never exited: state %q: %w", pid, state, err)
	}
	fmt.Fprintln(os.Stderr, "OK: the first daemon exited")
	return nil
}

// daemonProcessState answers the state letters of pid, and an empty state when
// no such process exists. A host with a procfs answers from it; any other host
// asks `ps`, which exits 1 for a pid it does not find and prints nothing. Any
// other failure is returned, never read as an exit.
func daemonProcessState(ctx context.Context, pid int) (string, error) {
	if state, answered, err := procProcessState(pid); answered {
		return state, err
	}
	output, err := exec.CommandContext(ctx, "ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output() //nolint:gosec // the fixture chooses the program and its arguments
	state := strings.TrimSpace(string(output))
	if err == nil {
		return state, nil
	}
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		return "", fmt.Errorf("ps -p %d: %w", pid, err)
	}
	if exitErr.ExitCode() != 1 {
		return "", fmt.Errorf("ps -p %d: %w", pid, err)
	}
	if state != "" {
		return "", fmt.Errorf("ps -p %d exited 1 and printed %q", pid, state)
	}
	return "", nil
}

// procProcessState reads pid's state from /proc/<pid>/stat. answered is false
// on a host with no procfs (darwin), and the caller then asks ps.
//
// The procfs is read first because `ps -p` is not portable to every Linux the
// tests run on: the QEMU guest's Alpine carries BusyBox ps, which has no -p. It
// exits 1 with its usage on stderr and nothing on stdout, which is exactly the
// "no such process" answer, so the barrier released while the daemon still ran
// and the next command tore its links down under it (measured 2026-10-15,
// test/bfd/bfd-first-packet-link-local.ci in the guest).
func procProcessState(pid int) (state string, answered bool, err error) {
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		return "", false, nil //nolint:nilerr // no procfs is the answer "ask ps", not a failure
	}
	path := "/proc/" + strconv.Itoa(pid) + "/stat"
	raw, err := os.ReadFile(path) //nolint:gosec // the path is built from a pid the runner published
	if errors.Is(err, fs.ErrNotExist) {
		return "", true, nil
	}
	if err != nil {
		return "", true, fmt.Errorf("read %s: %w", path, err)
	}
	// proc(5): "pid (comm) state ...". comm may hold spaces and parentheses,
	// so the state is the field after the LAST ')'.
	end := bytes.LastIndexByte(raw, ')')
	if end < 0 {
		return "", true, fmt.Errorf("%s has no state field: %q", path, raw)
	}
	if end+2 >= len(raw) {
		return "", true, fmt.Errorf("%s has no state field: %q", path, raw)
	}
	return string(raw[end+2]), true, nil
}
