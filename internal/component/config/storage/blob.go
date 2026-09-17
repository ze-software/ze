// Design: docs/architecture/storage-backends.md -- explicit artifact boundary.
package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

// OpenBlob opens an explicitly named artifact; it never detects a live backend.
// Writable artifacts take a stable sibling lock. Caller MUST Close the result.
func OpenBlob(path string, writable bool) (Storage, error) {
	dir, name, err := splitStorePath(path)
	if err != nil {
		return nil, err
	}
	folder, err := openFolder(dir)
	if err != nil {
		return nil, err
	}
	defer folder.Close() //nolint:errcheck // read descriptor.
	path = filepath.Join(folder.Name(), name)
	file, err := openNode(folder, name, false)
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	mode := unix.LOCK_SH
	if writable {
		mode = unix.LOCK_EX
	}
	owner, err := lockStoreFile(folder, filepath.Base(path)+".lock", mode)
	if err != nil {
		return nil, err
	}
	blob, err := zefs.Open(path)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return newStore(nil, blob, owner, !writable), nil
}

// CreateBlob creates an empty explicit artifact without replacing an existing
// file. Caller MUST Close the returned handle.
func CreateBlob(path string) (Storage, error) {
	return CreateBlobPopulated(path, func(Storage) error { return nil }, false)
}

// CreateBlobPopulated publishes a completely seeded artifact. Replacement keeps
// a .replaced-<stamp> backup. Caller MUST Close the returned handle.
func CreateBlobPopulated(path string, populate func(Storage) error, replace bool) (Storage, error) {
	dir, name, err := splitStorePath(path)
	if err != nil {
		return nil, err
	}
	folder, err := ensureFolder(dir)
	if err != nil {
		return nil, err
	}
	defer folder.Close() //nolint:errcheck // publication sync checked explicitly.
	path = filepath.Join(folder.Name(), name)
	if filepath.Base(path) == blobName {
		liveOwner, err := lockOwner(folder, lockName)
		if err != nil {
			return nil, err
		}
		defer liveOwner.Close() //nolint:errcheck // releases publication ownership.
		tree := filepath.Join(folder.Name(), treeName)
		if _, err := os.Lstat(tree); err == nil {
			return nil, fmt.Errorf("live database already exists: %s: %w", tree, fs.ErrExist)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	owner, err := lockOwner(folder, filepath.Base(path)+".lock")
	if err != nil {
		return nil, err
	}
	result, err := populateBlob(folder, owner, filepath.Base(path), populate, replace)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return result, nil
}

func populateBlob(folder, owner *os.File, name string, populate func(Storage) error, replace bool) (Storage, error) {
	path := filepath.Join(folder.Name(), name)
	if replace {
		existing, err := openNode(folder, name, false)
		if err == nil {
			if err := existing.Close(); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	if !replace {
		if _, err := os.Lstat(path); err == nil {
			return nil, fmt.Errorf("database already exists: %s: %w", path, fs.ErrExist)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	stage, err := os.CreateTemp(folder.Name(), ".ze-blob-*")
	if err != nil {
		return nil, err
	}
	if err := stage.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(stage.Name()) //nolint:errcheck // private artifact staging.
	blob, err := zefs.Create(stage.Name())
	if err != nil {
		return nil, err
	}
	seed := newStore(nil, blob, nil, false)
	if err := populate(seed); err != nil {
		return nil, errors.Join(err, seed.Close())
	}
	if err := seed.Close(); err != nil {
		return nil, err
	}
	if replace {
		if _, err := os.Lstat(path); err == nil {
			if err := moveAside(folder, name); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	if err := zefs.RenameNoReplace(folder, filepath.Base(stage.Name()), name); err != nil {
		return nil, err
	}
	if err := folder.Sync(); err != nil {
		return nil, err
	}
	blob, err = zefs.Open(path)
	if err != nil {
		return nil, err
	}
	return newStore(nil, blob, owner, false), nil
}
