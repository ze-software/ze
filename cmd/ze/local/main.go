// Design: docs/architecture/cli/plugin-modes.md — local install/uninstall implementation

package local

const (
	exitOK    = 0
	exitError = 1
)

func RunInstall(args []string) int   { return cmdInstall(args) }
func RunUninstall(args []string) int { return cmdUninstall(args) }
