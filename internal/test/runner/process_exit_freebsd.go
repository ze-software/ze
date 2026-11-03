// Design: docs/architecture/testing/runner-architecture.md -- identity before reap.
// Related: process_exit_linux.go -- the same retained-child ordering.

package runner

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// beforeReap observes without consuming the status. FreeBSD 14.3 kern_exit.c
// sys_wait4 forwards WNOWAIT through kern_wait and kern_wait6 to proc_reap,
// which returns before releasing the child's PID or process-group membership.
func (p *ownedProcess) beforeReap() error {
	return p.observeExit(func() error {
		var status unix.WaitStatus
		pid, err := unix.Wait4(p.cmd.Process.Pid, &status, unix.WNOWAIT, nil)
		if err != nil {
			return err
		}
		if pid != p.cmd.Process.Pid {
			return fmt.Errorf("exit observation returned child %d", pid)
		}
		if status.Exited() {
			return nil
		}
		if status.Signaled() {
			return nil
		}
		return fmt.Errorf("exit observation returned nonterminal status %d", status)
	})
}
