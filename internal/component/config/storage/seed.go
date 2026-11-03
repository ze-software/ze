// Design: docs/architecture/storage-backends.md -- inherited session init handles.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CreateSeedIn creates the canonical seed through already admitted handles.
// The caller MUST retain both handles until return and Close the returned store.
// folder.Name MUST be the absolute durable label, not an inherited fd spelling.
func CreateSeedIn(root *os.Root, folder *os.File, populate func(Storage) error) (Storage, error) {
	if err := seedHandles(root, folder); err != nil {
		return nil, err
	}
	return createBlobPopulatedIn(root, folder, blobName, populate, false, nil)
}

// ImportSeedIn imports or replays only the canonical seed beside the target
// tree. Both handles are borrowed until return. The returned store owns its
// duplicate folder and lock; the caller MUST Close it.
func ImportSeedIn(root *os.Root, folder *os.File) (Storage, error) {
	if err := seedHandles(root, folder); err != nil {
		return nil, err
	}
	source := &importSource{path: filepath.Join(folder.Name(), blobName), folder: folder, root: root}
	owner, err := lockOwner(folder, lockName)
	if err != nil {
		return nil, err
	}
	result, err := importOwnedSource(source, folder, owner, policyRetireSource, false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return result, nil
}

func seedHandles(root *os.Root, folder *os.File) error {
	if root == nil {
		return fmt.Errorf("seed root is missing; launch through le session seed-store")
	}
	if folder == nil {
		return fmt.Errorf("seed directory is missing; launch through le session seed-store")
	}
	if !filepath.IsAbs(folder.Name()) {
		return fmt.Errorf("seed directory needs its durable absolute label; launch through le session seed-store")
	}
	return matchBlobRoot(root, folder)
}
