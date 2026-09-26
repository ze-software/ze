// Design: docs/architecture/testing/ci-format.md — editor test storage modes
// Overview: runner.go — .et test execution

package testing

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// storageModeBlob selects a backup artifact as the editor's store
// (`option=storage:value=blob`), which is what `ze config edit --backup` edits.
const storageModeBlob = "blob"

// backupArtifactName is the artifact a blob-mode test edits, inside its
// temporary directory.
const backupArtifactName = "backup.zefs"

// createRunnerStore builds the store one .et test runs against and seeds its
// tmpfs blocks into it. Every session of the test shares it.
//
// The tree mode (the default) is a live store. The blob mode is a backup
// artifact reached the way `ze config edit --backup` reaches one: created and
// seeded, closed, then reopened writable through storage.OpenBlob at the
// editable spare policy, so the test edits what the command edits.
// Caller MUST Close the result.
func createRunnerStore(tmpDir, mode string, tmpfs []TmpfsBlock) (storage.Storage, error) {
	switch mode {
	case "", "tree":
		store, err := storage.Create(tmpDir)
		if err != nil {
			return nil, fmt.Errorf("creating tree storage: %w", err)
		}
		if err := seedRunnerStore(store, tmpfs); err != nil {
			return nil, errors.Join(err, store.Close())
		}
		return store, nil
	case storageModeBlob:
		path := filepath.Join(tmpDir, backupArtifactName)
		created, err := storage.CreateBlob(path)
		if err != nil {
			return nil, fmt.Errorf("creating backup artifact: %w", err)
		}
		if err := seedRunnerStore(created, tmpfs); err != nil {
			return nil, errors.Join(err, created.Close())
		}
		if err := created.Close(); err != nil {
			return nil, fmt.Errorf("closing seeded backup artifact: %w", err)
		}
		return storage.OpenBlob(path, true, zefs.Spare(zefs.SpareDefault))
	}
	return nil, fmt.Errorf("unknown option=storage:value=%s (want tree or blob)", mode)
}

// seedRunnerStore writes each tmpfs block into store under its path.
func seedRunnerStore(store storage.Storage, tmpfs []TmpfsBlock) error {
	for _, tf := range tmpfs {
		if err := store.WriteFile(tf.Path, []byte(tf.Content), 0o600); err != nil {
			return fmt.Errorf("seeding config %s: %w", tf.Path, err)
		}
	}
	return nil
}
