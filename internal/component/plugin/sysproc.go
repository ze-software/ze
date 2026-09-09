// Design: docs/architecture/api/process-protocol.md -- how an external plugin is started
// Related: sysproc_linux.go -- NewSysProcAttr, the Setpgid this signal relies on
// Related: declarations.go -- runQuery, the declaration query that forks with this
// Related: process/process.go -- (*Process).startExternal, the live start that forks with this

package plugin

import (
	"os/exec"
	"syscall"
)

// KillGroupOnCancel gives cmd the stop every forked plugin is stopped by: the
// fork puts the plugin in a process group of its own, and a canceled context
// signals that whole group. The live start and the declaration query both call
// it, which is what makes a plugin started two ways stopped one way.
//
// The two halves are one function because either half alone stops nothing
// extra. Attributes with no Cancel leave exec.CommandContext's own Cancel in
// place, which kills the direct child; that child is the shell, and a shell
// that did not exec-optimize the run string keeps the plugin as its own child,
// so the plugin runs on. A Cancel with no Setpgid signals a group that does not
// exist (KillProcessGroup).
//
// The caller MUST NOT write cmd.SysProcAttr or cmd.Cancel after this call.
// cmd.WaitDelay is left to the caller, because what bounds Wait depends on
// which pipes that caller gave the child.
func KillGroupOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = NewSysProcAttr()
	cmd.Cancel = func() error { return KillProcessGroup(cmd.Process.Pid) }
}

// KillProcessGroup stops the plugin started at pid and everything that plugin
// started, by signaling the process group pid heads.
//
// The negative pid is what makes the signal reach the group rather than the
// one process, and the group exists because the fork asked for it
// (NewSysProcAttr, Setpgid). A caller that has not set that attribute MUST NOT
// call this: the child then stays in ze's own group, so no group is headed by
// that pid, the kernel answers ESRCH, and the stop signals nothing at all. The
// failure is a stop that silently does nothing, not a signal that reaches ze
// (`ai/rules/principles.md`).
func KillProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
