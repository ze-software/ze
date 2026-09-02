// Design: docs/architecture/hub-architecture.md -- edit session identity
// Related: session_factory.go -- the SSH session model factory (//go:build ze_ssh)
// Related: main.go -- attachedConsoleEditor, the `ze start --cli` console
//
// Untagged on purpose: `ze start --cli` reaches config mode in a build with SSH
// compiled out, so the two surfaces share one definition of an edit session.

package hub

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Session origins. The origin goes into the edit session identity
// ("user@origin%time"). The draft metadata uses it to tell two concurrent
// sessions of one user apart.
const (
	sessionOriginSSH   = "ssh"
	sessionOriginLocal = "local"
)

// newSessionEditor builds the storage-backed editor for one edit session. It
// validates the user and stamps the given origin into the session identity. A
// reload function, when one is given, becomes the editor's reload notifier.
//
// The notifier routes `commit` through the transactional
// CommitSessionCandidate + NotifyReload path. A session commit then reaches the
// running daemons instead of only writing config.conf.
func newSessionEditor(store storage.Storage, configPath, username, origin string, reloadFn func() error) (*cli.Editor, error) {
	if err := cli.ValidateUser(username); err != nil {
		return nil, fmt.Errorf("invalid username: %w", err)
	}
	ed, err := cli.NewEditorWithStorage(store, configPath)
	if err != nil {
		return nil, err
	}
	ed.SetSession(cli.NewEditSession(username, origin))
	if reloadFn != nil {
		ed.SetReloadNotifier(reloadFn)
	}
	return ed, nil
}

// attachedConsoleEditor builds the edit session for the `ze start --cli`
// console, so `configure` reaches config mode there as it does over SSH. It
// returns nil when there is no config to edit, which leaves the command-only
// console and its refusal.
//
// reloadFn is the notifier the SSH sessions get. A `commit` typed at the
// attached console therefore reaches the running daemons.
func attachedConsoleEditor(store storage.Storage, configPath string, reloadFn func() error) *cli.Editor {
	if store == nil || configPath == "" || configPath == "-" {
		return nil
	}
	ed, err := newSessionEditor(store, configPath, attachedConsoleUser(), sessionOriginLocal, reloadFn)
	if err != nil {
		slogutil.Logger("hub.cli").Warn("attached console config mode unavailable", "error", err)
		return nil
	}
	return ed
}

// attachedConsoleUser names the operator whose changes the attached console
// writes. The console authenticates nobody, because an operator starts it at a
// terminal and it dispatches as root. The operating system user is the identity
// available, and the console's history and transcript already carry it.
//
// "unknown" is the fallback for two reasons. ValidateUser refuses an empty
// name, and a name nobody read MUST NOT claim an identity. `ze config edit`
// names the same operator the same way
// (internal/component/config/cli/cmd_edit.go).
func attachedConsoleUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "unknown"
}
