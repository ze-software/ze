// Design: docs/architecture/storage-backends.md -- native import through one configuration guard.
// Related: restore.go -- existing configurations use the shared version publication.
package storage

import (
	"fmt"

	"github.com/ze-software/ze/pkg/zefs"
)

// ImportConfig imports raw bytes under a configuration basename, preserving
// native standalone semantics: an existing active pointer or mirror gains a
// version, while a new name gains its active file key. No configuration parser
// validates the input. Unfinished file publication refuses before any write.
// The caller MUST own this store exclusively and MUST check its recorded
// confirmed-commit window before calling; live owners MUST use their reload.
// Admission and publication retain one guard, released before this returns.
func ImportConfig(target Storage, name string, data []byte) (err error) {
	base, err := StoredConfigName(target, name)
	if err != nil {
		return err
	}
	held, err := target.AcquireLock(base)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(held, err) }()
	if err := refuseConfigIntentLocked(held, zefs.KeyConfigFileCommit.Key(base), "file commit"); err != nil {
		return fmt.Errorf("import configuration %s: %w", base, err)
	}
	_, pointed, err := readPointerLocked(target, held, base, pointerActive)
	if err != nil {
		return fmt.Errorf("active pointer of %s: %w", base, err)
	}
	if pointed || held.Has(base) {
		_, err = restoreConfigLocked(target, held, base, data)
		return err
	}
	return held.WriteKey(zefs.KeyFileActive.Key(base), data)
}
