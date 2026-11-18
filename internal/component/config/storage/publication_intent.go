// Design: docs/architecture/storage-backends.md -- preserve unfinished configuration publication.
package storage

import (
	"errors"
	"fmt"
	"io/fs"
)

// refuseConfigIntentLocked is the shared retained-intent admission for import
// and rollback. The caller MUST hold held through its following publication,
// and MUST use held rather than reentering Storage while the guard is held.
func refuseConfigIntentLocked(held WriteGuard, key, explanation string) error {
	_, err := held.ReadKey(key)
	if err == nil {
		return fmt.Errorf("an unfinished %s must be recovered first", explanation)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read %s: %w", key, err)
	}
	return nil
}
