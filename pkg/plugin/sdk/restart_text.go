// Design: docs/guide/plugins.md -- how to restart ze to apply a refused change
// Overview: sdk.go -- plugin SDK core

package sdk

import "github.com/ze-software/ze/internal/core/privilege"

// applianceRestartRoute is the route that stops ze on the ze appliance and
// starts it again. It is a line of its own, beside the route for this process,
// whatever the process's service membership, because the SDK cannot tell an
// appliance: the switch that marks one, ze.gokrazy.enabled, is registered by
// internal/component/config, which a plugin built on the SDK need not link, and
// env.Get of an unregistered key ends the process.
//
// A bare `reboot` is not a route: it is an SSH exec verb, never a ze CLI
// command, so the route names `request reboot` and `ze signal reboot`.
const applianceRestartRoute = "  on the ze appliance: request reboot in the ze CLI, or ze signal reboot from a shell"

// RestartSteps returns the restart advice a refusal appends when a change
// cannot apply while ze runs: three lines, with no trailing newline. The second
// line is the route for this process, privilege.RestartRemedy, which names
// `sudo systemctl restart ze` only when ze runs under ze.service; the third is
// the appliance route. It is the one copy of that advice; a refusal MUST append
// it rather than spell its own. A plugin the daemon forks inherits the daemon's
// cgroup, and an internal plugin is the daemon, so the plugin's own membership
// is the daemon's. Safe for concurrent use.
//
// No route is `ze signal restart`: it exits with rpc.DaemonRestartExitStatus
// and starts no new process, so it brings ze back only under a supervisor that
// restarts on that status, and a ze started any other way stays stopped
// (privilege.RestartRemedy).
func RestartSteps() string {
	return "To apply the change, stop the ze daemon and start it again:\n" + restartRoutes()
}

// restartRoutes is the route for this process, then the appliance route, one
// per line, with no trailing newline. RestartSteps and RunningConfigLostRestart
// append it.
func restartRoutes() string {
	return "  " + privilege.RestartRemedy() + "\n" + applianceRestartRoute
}
