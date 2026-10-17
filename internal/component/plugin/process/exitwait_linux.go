// Design: docs/architecture/api/process-protocol.md -- how an external plugin is stopped
// Related: exitwait_other.go -- the same declarations where waitid cannot leave a child unreaped
// Related: process.go -- startExternal and monitorCmd, the callers; pluginGroup, the kill

//go:build linux

package process

import (
	"context"
	"errors"
	"os/exec"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/plugin"
)

// stopGivesExitGrace says a stop closes the plugin's connection and lets the
// plugin exit on its own before its group is killed (superviseExit). Linux can
// hold an exited plugin's pid until the group is signaled (waitExitedUnreaped),
// and that is what makes a kill that comes later than the stop safe.
const stopGivesExitGrace = true

// newPluginShell answers the fork of the plugin's run string, with no context.
//
// exec.CommandContext signals from a goroutine of its own, and that goroutine
// can still run its Cancel after Process.Wait has reaped the plugin, so a group
// kill sent from it can reach whatever process took the freed pid. Here nothing
// but pluginGroup signals the plugin, and only while monitorCmd has not
// recorded its exit. The stop context is unused: superviseExit reads the stop
// from the process context instead.
//
// Setpgid (NewSysProcAttr) makes the plugin head the group pluginGroup signals.
// The attribute and the kill are one pair: the kill alone signals a group that
// does not exist, and the attribute alone leaves a stop that reaches nothing.
func newPluginShell(_ context.Context, run string) *exec.Cmd {
	// #nosec G204 - Run command is from trusted configuration, not user input
	cmd := exec.Command(plugin.Shell, "-c", run) //nolint:noctx // a context's Cancel can fire past the reap; pluginGroup is the stop (see above)
	cmd.SysProcAttr = plugin.NewSysProcAttr()
	return cmd
}

// waitExitedUnreaped blocks until the plugin process at pid has exited, and
// answers true with the process still unreaped: a zombie that keeps pid, and
// so the process group pid heads, from being handed to another process.
//
// That is what lets a stop signal the group after the plugin exited on its
// own. Once cmd.Wait reaps the process the number is free, and a group signal
// sent then could reach a stranger that took it. WNOWAIT leaves the reap to
// cmd.Wait, which the caller runs next.
//
// It answers false when the kernel gave no answer, and the caller then reaps
// without signaling the group.
func waitExitedUnreaped(pid int) bool {
	var info unix.Siginfo
	for {
		err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if err == nil {
			return true
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		logger().Debug("wait for plugin exit without reaping it", "pid", pid, "error", err)
		return false
	}
}
