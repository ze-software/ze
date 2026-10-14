// Design: docs/guide/plugins.md -- how to restart ze to apply a refused change
// Overview: sdk.go -- plugin SDK core

package sdk

import "strings"

// restartStepLines are the routes that stop the ze daemon and start it again,
// one per line, each under the supervisor that runs ze.
//
// None of them is `ze signal restart`: that command stops ze and does not bring
// it back (hub.Run prints "Ze stopped." and returns 0), and neither systemd
// (Restart=on-failure) nor the appliance's init restarts a clean exit. A bare
// `reboot` is not one either: it is an SSH exec verb, never a ze CLI command,
// so the appliance route names `request reboot` and `ze signal reboot`.
var restartStepLines = []string{
	"To apply the change, stop the ze daemon and start it again:",
	"  under systemd: systemctl restart ze",
	"  without systemd: ze signal stop, then start ze the way it was started (ze start, or ze start <config-file>)",
	"  on the ze appliance: request reboot in the ze CLI, or ze signal reboot from a shell",
}

// RestartSteps returns the restart advice a refusal appends when a change
// cannot apply while ze runs: four lines, one route per line, with no trailing
// newline. It is the one copy of that advice; a refusal MUST append it rather
// than spell its own.
func RestartSteps() string {
	return strings.Join(restartStepLines, "\n")
}
