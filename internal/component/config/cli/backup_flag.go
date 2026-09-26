// Design: docs/guide/config-editor.md — editing a backup artifact offline
// Overview: main.go — dispatch and exit codes
// Related: cmd_edit.go — the interactive editor over a backup

package cli

import (
	"errors"
	"fmt"
	"time"

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
// It publishes content the way a daemon commit does: a candidate version,
// then the promotion that moves the active and rollback pointers and writes
// the file/active mirror. Writing the mirror alone would leave the active
// pointer on the old version, and every reader that follows the pointer
// (storage.ReadActiveConfig, `ze data restore`) would still answer the old
// config.
//
// expected is not compared: the writer holds the artifact's exclusive lock,
// so nothing else can have changed the config since the editor read it.
func publishInBackup(store storage.Storage, configPath string) func(expected, content []byte) error {
	return func(_, content []byte) error {
		if _, err := storage.WriteCandidateVersion(store, configPath, content, time.Now()); err != nil {
			return err
		}
		if err := storage.PromoteCandidate(store, configPath); err != nil {
			return errors.Join(err, storage.ClearCandidate(store, configPath))
		}
		return nil
	}
}
