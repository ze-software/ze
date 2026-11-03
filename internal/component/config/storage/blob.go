// Design: docs/architecture/storage-backends.md -- explicit artifact boundary.
package storage

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

// OpenBlob opens an explicitly named artifact; it never detects a live backend.
// Writable artifacts take a stable sibling lock. opts set the spare policy the
// artifact's writes use; without one they pad by zefs.SpareDefault percent.
// Caller MUST Close the result.
func OpenBlob(path string, writable bool, opts ...zefs.Option) (Storage, error) {
	return openBlob(path, writable, openNode, opts...)
}

// openBlob takes the descriptor acquisition function per call so tests can
// order a cooperative writer's publication against acquisition without sleeps.
func openBlob(path string, writable bool, acquire func(*os.File, string, bool) (*os.File, error), opts ...zefs.Option) (Storage, error) {
	dir, name, err := splitStorePath(path)
	if err != nil {
		return nil, err
	}
	folder, err := openFolder(dir)
	if err != nil {
		return nil, err
	}
	defer folder.Close() //nolint:errcheck // read descriptor.
	root, err := blobRoot(folder)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // The blob retains its own root.
	path = filepath.Join(folder.Name(), name)
	file, err := acquire(folder, name, false)
	if err != nil {
		return nil, err
	}
	// Refuse unsafe entries before creating a lock file, but never consume
	// this observation: a cooperative writer may publish before we lock.
	if err := file.Close(); err != nil {
		return nil, err
	}
	mode := unix.LOCK_SH
	if writable {
		mode = unix.LOCK_EX
	}
	owner, err := lockStoreFile(folder, filepath.Base(path)+".lock", mode, (*os.File).Sync)
	if err != nil {
		return nil, err
	}
	// Acquire and validate the consumed descriptor under publication ownership.
	file, err = acquire(folder, name, false)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	blob, err := zefs.OpenFileIn(root, name, file, opts...)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return newStore(nil, blob, owner, !writable), nil
}

// CreateBlob creates an empty explicit artifact without replacing an existing
// file. opts set the spare policy of the returned handle's writes. Caller MUST
// Close the returned handle.
func CreateBlob(path string, opts ...zefs.Option) (Storage, error) {
	return CreateBlobPopulated(path, func(Storage) error { return nil }, false, opts...)
}

// CreateBlobPopulated publishes a completely seeded artifact. Replacement keeps
// a .replaced-<stamp> backup. opts set the spare policy of the seeding writes
// and of the returned handle. Caller MUST Close the returned handle.
func CreateBlobPopulated(path string, populate func(Storage) error, replace bool, opts ...zefs.Option) (Storage, error) {
	dir, name, err := splitStorePath(path)
	if err != nil {
		return nil, err
	}
	folder, err := ensureFolder(dir)
	if err != nil {
		return nil, err
	}
	defer folder.Close() //nolint:errcheck // publication sync checked explicitly.
	root, err := blobRoot(folder)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // The returned blob retains its own root.
	return createBlobPopulatedIn(root, folder, name, populate, replace, opts)
}

func createBlobPopulatedIn(root *os.Root, folder *os.File, name string, populate func(Storage) error, replace bool, opts []zefs.Option) (Storage, error) {
	path := filepath.Join(folder.Name(), name)
	if filepath.Base(path) == blobName {
		liveOwner, err := lockOwner(folder, lockName)
		if err != nil {
			return nil, err
		}
		defer liveOwner.Close() //nolint:errcheck // releases publication ownership.
		tree := filepath.Join(folder.Name(), treeName)
		if err := nodeStat(folder, treeName); err == nil {
			return nil, fmt.Errorf("live database already exists: %s: %w", tree, fs.ErrExist)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	owner, err := lockOwner(folder, filepath.Base(path)+".lock")
	if err != nil {
		return nil, err
	}
	result, err := populateBlob(root, folder, owner, name, populate, replace, opts)
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return result, nil
}

// blobRoot pairs the no-follow storage directory with an os.Root for ZeFS.
// Identity is checked after both opens, before any root-relative operation.
// The caller MUST close the returned root independently of folder.
func blobRoot(folder *os.File) (*os.Root, error) {
	root, err := os.OpenRoot(folder.Name())
	if err != nil {
		return nil, err
	}
	if err := matchBlobRoot(root, folder); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return root, nil
}

func matchBlobRoot(root *os.Root, folder *os.File) error {
	want, err := folder.Stat()
	if err != nil {
		return err
	}
	got, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(want, got) {
		return fmt.Errorf("%w: artifact directory changed: %s: retry using its current path", ErrPermissions, folder.Name())
	}
	return nil
}

func populateBlob(root *os.Root, folder, owner *os.File, name string, populate func(Storage) error, replace bool, opts []zefs.Option) (Storage, error) {
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
		if err := nodeStat(folder, name); err == nil {
			return nil, fmt.Errorf("database already exists: %s: %w", path, fs.ErrExist)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	stageName := ".ze-blob-" + rand.Text()
	stage, err := root.OpenFile(stageName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	if err := stage.Close(); err != nil {
		return nil, err
	}
	defer root.Remove(stageName) //nolint:errcheck // Private artifact staging.
	blob, err := zefs.CreateIn(root, stageName, opts...)
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
		if err := nodeStat(folder, name); err == nil {
			if err := moveAside(folder, name); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	if err := zefs.RenameNoReplace(folder, stageName, name); err != nil {
		return nil, err
	}
	if err := folder.Sync(); err != nil {
		return nil, err
	}
	file, err := openNode(folder, name, false)
	if err != nil {
		return nil, err
	}
	blob, err = zefs.OpenFileIn(root, name, file, opts...)
	if err != nil {
		return nil, err
	}
	return newStore(nil, blob, owner, false), nil
}
