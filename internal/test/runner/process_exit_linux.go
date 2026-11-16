// Design: docs/architecture/testing/runner-architecture.md -- identity before reap.
// Related: internal/component/plugin/process/exitwait_linux.go -- WNOWAIT ownership.

package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// beforeReap holds the leader's identity through the last descendant signal
// and the observation that its group has no live members. SIGKILL returning
// does not mean every recipient has exited, and a descendant can close every
// inherited output descriptor before that signal, leaving no pipe to join.
// Only the owner calls Cmd.Wait. A canceled CommandContext can still call Cancel
// after that reap, but cancel observes retirement under the same signal lock.
func (p *ownedProcess) beforeReap() error {
	var info unix.Siginfo
	if err := p.observeExit(func() error {
		return unix.Waitid(unix.P_PID, p.cmd.Process.Pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
	}); err != nil {
		return err
	}
	return waitLinuxProcessGroup("/proc", p.cmd.Process.Pid, 2*time.Second)
}

// waitLinuxProcessGroup observes only; the unreaped leader still reserves pgid,
// and the existing owner has retired every numeric signaling alias. A lone
// zombie task has exited; its reaping belongs to its surviving parent.
// An unreadable process table cannot establish that the group has exited.
func waitLinuxProcessGroup(procRoot string, pgid int, budget time.Duration) error {
	timer := time.NewTimer(budget)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		live, err := linuxProcessGroupLive(procRoot, pgid)
		if err != nil {
			return fmt.Errorf("observe process group %d exit: %w", pgid, err)
		}
		if !live {
			return nil
		}
		select {
		case <-timer.C:
			return fmt.Errorf("process group %d still has live members after %s", pgid, budget)
		case <-ticker.C:
		}
	}
}

func linuxProcessGroupLive(procRoot string, pgid int) (bool, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return false, err
	}
	leader := strconv.Itoa(pgid)
	leaderFound := false
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		path := filepath.Join(procRoot, entry.Name(), "stat")
		state, group, err := readLinuxProcessState(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return false, err
		}
		if entry.Name() == leader {
			if group != pgid {
				return false, fmt.Errorf("held leader %d belongs to process group %d", pgid, group)
			}
			leaderFound = true
		}
		if group != pgid {
			continue
		}
		if state != 'Z' {
			return true, nil
		}
		exited, err := linuxProcessTasksExited(procRoot, entry.Name(), pgid)
		if err != nil {
			return false, err
		}
		if !exited {
			return true, nil
		}
	}
	if !leaderFound {
		return false, fmt.Errorf("held process group leader %d is absent from %s", pgid, procRoot)
	}
	return false, nil
}

// A zombie thread-group leader can still have running threads. Only its lone
// zombie task proves the process exited; any other task keeps the wait open.
func linuxProcessTasksExited(procRoot, pid string, pgid int) (bool, error) {
	tasks := filepath.Join(procRoot, pid, "task")
	entries, err := os.ReadDir(tasks)
	if err != nil {
		return linuxProcessDisappearance(procRoot, pid, pgid, err)
	}
	leaderFound := false
	otherTasks := false
	for _, entry := range entries {
		if entry.Name() == pid {
			leaderFound = true
		} else {
			otherTasks = true
		}
	}
	if !leaderFound {
		err := fmt.Errorf("process %s task table has no leader in %s", pid, tasks)
		return linuxProcessDisappearance(procRoot, pid, pgid, err)
	}
	state, group, err := readLinuxProcessState(filepath.Join(tasks, pid, "stat"))
	if err != nil {
		return linuxProcessDisappearance(procRoot, pid, pgid, err)
	}
	if group != pgid {
		return false, fmt.Errorf("process %s task leader belongs to group %d, want %d", pid, group, pgid)
	}
	if state != 'Z' {
		return false, nil
	}
	return !otherTasks, nil
}

// A missing task table is not an exit observation. Verify that the process
// itself disappeared, and never accept disappearance of our unreaped parent.
func linuxProcessDisappearance(procRoot, pid string, pgid int, taskErr error) (bool, error) {
	_, err := os.Stat(filepath.Join(procRoot, pid))
	if errors.Is(err, os.ErrNotExist) {
		if pid == strconv.Itoa(pgid) {
			return false, fmt.Errorf("held process group leader %s disappeared: %w", pid, taskErr)
		}
		return true, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		if pid == strconv.Itoa(pgid) {
			return false, fmt.Errorf("held process group leader %s disappeared: %w", pid, taskErr)
		}
		return true, nil
	}
	return false, errors.Join(taskErr, err)
}

func readLinuxProcessState(path string) (byte, int, error) {
	stat, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	// The parenthesized command name can itself contain a closing
	// parenthesis; fields after its last one start at stat field 3.
	end := strings.LastIndexByte(string(stat), ')')
	if end < 0 {
		return 0, 0, fmt.Errorf("invalid process state in %s", path)
	}
	fields := strings.Fields(string(stat[end+1:]))
	if len(fields) < 3 {
		return 0, 0, fmt.Errorf("incomplete process state in %s", path)
	}
	if len(fields[0]) != 1 {
		return 0, 0, fmt.Errorf("invalid process state in %s", path)
	}
	group, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, 0, fmt.Errorf("read process group in %s: %w", path, err)
	}
	return fields[0][0], group, nil
}

// exitedBeforeStop MUST run under signalMu while the parent identity is held.
// WNOWAIT observes an already-exited parent without consuming its sole Wait.
func (p *ownedProcess) exitedBeforeStop() (bool, error) {
	var info unix.Siginfo
	err := unix.Waitid(unix.P_PID, p.cmd.Process.Pid, &info, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil)
	return info.Signo != 0, err
}
