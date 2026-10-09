// Design: docs/functional-tests.md — process orchestration, a second daemon started on the first one's file
// Related: register_daemon_await_exit.go — the registration of this barrier

package fixture

import (
	"context"
	"errors"
	"fmt"
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

// daemonProcessState answers the state letters `ps` reports for pid, and an
// empty state when no such process exists. ps exits 1 for a pid it does not
// find, and prints nothing; any other failure is returned, never read as an
// exit.
func daemonProcessState(ctx context.Context, pid int) (string, error) {
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
