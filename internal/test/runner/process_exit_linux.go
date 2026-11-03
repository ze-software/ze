// Design: docs/architecture/testing/runner-architecture.md -- identity before reap.
// Related: internal/component/plugin/process/exitwait_linux.go -- WNOWAIT ownership.

package runner

import "golang.org/x/sys/unix"

// beforeReap holds the leader's identity through the last descendant signal.
// Only the owner calls Cmd.Wait. A canceled CommandContext can still call Cancel
// after that reap, but cancel observes retirement under the same signal lock.
func (p *ownedProcess) beforeReap() error {
	var info unix.Siginfo
	return p.observeExit(func() error {
		return unix.Waitid(unix.P_PID, p.cmd.Process.Pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
	})
}
