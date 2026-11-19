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
	_, present, err := readAdmissionKeyLocked(held, key)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("an unfinished %s must be recovered first", explanation)
	}
	return nil
}

// readAdmissionKeyLocked reads one key an admission decides on, under the
// guard its publication already holds. The caller MUST hold held through the
// publication the answer admits.
func readAdmissionKeyLocked(held WriteGuard, key string) (value []byte, present bool, err error) {
	return readAdmissionKey(held.ReadKey, key)
}

// readAdmissionKey reads one key an admission decides on through read. The
// retained-intent admission above and the freeze (freeze.go) share it, so both
// tell an absent key from a key the store could not read in one place: absent
// answers present false, and any other failure is an error naming the key,
// never "absent".
func readAdmissionKey(read func(key string) ([]byte, error), key string) (value []byte, present bool, err error) {
	value, err = read(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", key, err)
	}
	return value, true, nil
}
