// Design: docs/architecture/config/syntax.md — config set/deactivate --reload opt-in
// Related: cmd_set.go, cmd_deactivate.go — the two callers that share this gate

package cli

import (
	"fmt"
	"os"

	editor "codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/core/cliio"
	sshclient "codeberg.org/thomas-mangin/ze/internal/core/ssh/client"
)

// loadReloadCredentials and execReloadCommand indirect over the SSH client so a
// test can observe whether --reload actually reaches for the daemon without
// opening a real connection. Production wiring is the real client; tests swap
// these to count invocations.
var (
	loadReloadCredentials = sshclient.LoadCredentialsWithFlags
	execReloadCommand     = func(creds sshclient.Credentials) error {
		_, err := sshclient.ExecCommand(creds, "reload")
		return err
	}
)

// notifyDaemonReload asks a running daemon to reload after a successful save,
// but ONLY when the operator opted in with --reload and the config is a real
// on-disk file. Editing a stored config does not contact the daemon by default;
// a stdin ("-") pipeline stage has no on-disk config for a daemon to reload.
// Best-effort: a missing credential store is silent, an unreachable daemon is a
// warning, and neither changes the command's exit code.
func notifyDaemonReload(ed *editor.Editor, reload bool, configPath, user string) {
	if !reload || cliio.IsStdin(configPath) {
		return
	}
	creds, err := loadReloadCredentials(user)
	if err != nil {
		return
	}
	ed.SetReloadNotifier(func() error { return execReloadCommand(creds) })
	if notifyErr := ed.NotifyReload(); notifyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not notify daemon: %v\n", notifyErr)
	}
}
