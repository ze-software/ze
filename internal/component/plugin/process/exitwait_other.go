// Design: docs/architecture/api/process-protocol.md -- how an external plugin is stopped
// Related: exitwait_linux.go -- the same declarations where waitid can leave a child unreaped
// Related: process.go -- startExternal and monitorCmd, the callers; pluginGroup, the kill

//go:build !linux

package process

import (
	"context"
	"os/exec"

	"github.com/ze-software/ze/internal/component/plugin"
)

// stopGivesExitGrace says a stop kills the plugin's group at once here. This
// platform has no waitid P_PID with WNOWAIT, so nothing holds an exited
// plugin's pid until a later kill is sent, and a kill that came pluginExitGrace
// after the stop could reach a stranger that took the freed pid. The cost is
// the one Linux does not pay: the stop can lose the last stderr line a plugin
// wrote just before it (pluginExitGrace).
const stopGivesExitGrace = false

// newPluginShell answers the fork of the plugin's run string, whose group is
// SIGKILLed the moment stopCtx ends (KillGroupOnCancel). exec runs that Cancel
// from a goroutine of its own, which can still fire just after the reap; no
// later and safer kill exists on this platform.
func newPluginShell(stopCtx context.Context, run string) *exec.Cmd {
	// #nosec G204 - Run command is from trusted configuration, not user input
	cmd := exec.CommandContext(stopCtx, plugin.Shell, "-c", run)
	plugin.KillGroupOnCancel(cmd)
	return cmd
}

// waitExitedUnreaped answers false at once on a platform without Linux's
// waitid P_PID and WNOWAIT: there is no way here to hold the exited plugin's
// pid until the group is signaled, so the caller reaps without signaling it.
// The stop has already killed the group, descendants included, the moment it
// came (newPluginShell); a plugin that exits on its own outside a stop leaves
// its descendants running, as on Linux.
func waitExitedUnreaped(_ int) bool {
	return false
}
