// Design: docs/architecture/cli/plugin-modes.md — local install/uninstall implementation

package local

const (
	exitOK    = 0
	exitError = 1

	// flagDryRun is the one flag token the help pages and the completion
	// inventory each spell.
	flagDryRun = "--dry-run"
)

func RunInstall(args []string) int   { return cmdInstall(args) }
func RunUninstall(args []string) int { return cmdUninstall(args) }
