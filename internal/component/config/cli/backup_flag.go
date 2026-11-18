// Design: docs/guide/config-editor.md — editing a backup artifact offline
// Overview: main.go — dispatch and exit codes
// Related: cmd_edit.go — the interactive editor over a backup

package cli

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// flagBackup is the flag that points `ze config edit|show|diff|set|list` at a
// backup artifact instead of the live store. One spelling for every command.
const flagBackup = "backup"

// openBackup opens the backup artifact a --backup flag names.
//
// storage.OpenBlob takes the artifact's <artifact>.lock sidecar for as long as
// the handle lives: exclusive for a writer, shared for a reader. A writer
// therefore refuses every other command on the same file, and a reader
// refuses a writer, before either reads a byte.
//
// Writes pad at zefs.SpareDefault, stated here rather than left to the
// default. An artifact that is being edited is expected to change again, and
// a slot with spare takes an equal or slightly longer value in place; `ze data
// backup` writes exact-fit artifacts because nothing edits them.
func openBackup(path string, writable bool) (storage.Storage, error) {
	store, err := storage.OpenBlob(path, writable, zefs.Spare(zefs.SpareDefault))
	if errors.Is(err, storage.ErrBusy) {
		return nil, fmt.Errorf("backup %s is in use by another ze config command: %w", path, err)
	}
	if err != nil {
		return nil, fmt.Errorf("backup %s: %w", path, err)
	}
	return store, nil
}

// publishInBackup returns the commit writer of `ze config set --backup`.
//
// It shares complete artifact publication with daemon-owned backup commands:
// a version, active and rollback pointers, and the file/active mirror change
// together while an existing pending candidate and saved drafts stay separate.
//
// expected is not compared: the writer holds the artifact's exclusive lock,
// so nothing else can have changed the config since the editor read it.
func publishInBackup(store storage.Storage, configPath string) func(expected, content []byte) error {
	return func(_, content []byte) error {
		_, err := storage.PublishArtifactConfig(store, configPath, content)
		return err
	}
}
