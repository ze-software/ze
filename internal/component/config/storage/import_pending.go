// Design: docs/architecture/storage-backends.md -- Restartable Full Import
// Related: import.go -- importIntent, readImportIntent

package storage

import (
	"errors"
	"io/fs"
)

// PendingImportSource returns the source path an unfinished import in dir
// records, and whether one is recorded. `ze init --from <url>` asks it after a
// failed import: the recovery command names the fetched copy, so that copy
// stays while an intent records it. A missing dir holds no import. A
// malformed intent is an error, never "no import".
func PendingImportSource(dir string) (string, bool, error) {
	folder, err := openFolder(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer folder.Close() //nolint:errcheck // read-only directory handle
	intent, pending, err := readImportIntent(folder)
	if err != nil {
		return "", false, err
	}
	return intent.Source, pending, nil
}
