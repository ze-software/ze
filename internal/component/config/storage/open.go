// Design: docs/architecture/storage-backends.md -- ownership and atomic publication.
package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Open opens a live tree for its sole writer. The caller MUST Close the handle.
// Blob detection is stat-only and always refuses, including a both-present state.
func Open(dir string) (Storage, error) { return openLive(dir, false) }

// OpenReadOnly allows inspection beside a live owner and refuses every mutation.
// The caller MUST Close the handle.
func OpenReadOnly(dir string) (Storage, error) { return openLive(dir, true) }

// OpenTree opens the exact offline tree path. The canonical database name uses
// the live opener and ownership lock. Caller MUST Close the result.
func OpenTree(path string, writable bool) (Storage, error) {
	dir, name, err := splitStorePath(strings.TrimRight(path, "/"))
	if err != nil {
		return nil, err
	}
	if name == "database" {
		if writable {
			return Open(dir)
		}
		return OpenReadOnly(dir)
	}
	folder, err := openFolder(dir)
	if err != nil {
		return nil, err
	}
	var owner *os.File
	if writable {
		owner, err = lockOwner(folder, name+".lock")
		if err != nil {
			folder.Close()
			return nil, err
		} //nolint:errcheck // ownership error retained.
	}
	result, err := openTree(folder, name, owner, !writable)
	if err != nil {
		if owner != nil {
			owner.Close()
		} //nolint:errcheck // open error retained.
		folder.Close() //nolint:errcheck // open error retained.
		return nil, err
	}
	return result, nil
}

func splitStorePath(path string) (string, string, error) {
	dir, name := filepath.Split(path)
	if name == "" {
		return "", "", fmt.Errorf("missing store name: %s", path)
	}
	if name == "." {
		return "", "", fmt.Errorf("invalid store name: %s", path)
	}
	if name == ".." {
		return "", "", fmt.Errorf("invalid store name: %s", path)
	}
	if dir == "" {
		dir = "."
	}
	return dir, name, nil
}

func detect(dir string) error {
	blob := filepath.Join(dir, "database.zefs")
	if _, err := os.Lstat(blob); err == nil {
		return fmt.Errorf("blob artifact %s is not a live store; run ze init from %s", blob, blob)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	path := filepath.Join(dir, "database")
	if _, err := os.Lstat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s; run ze init", ErrNoStore, path)
		}
		return err
	}
	return nil
}
func openLive(dir string, readonly bool) (Storage, error) {
	if err := detect(dir); err != nil {
		return nil, err
	}
	folder, err := openFolder(dir)
	if err != nil {
		return nil, err
	}
	var owner *os.File
	if !readonly {
		owner, err = lockOwner(folder, "database.lock")
		if err != nil {
			folder.Close()
			return nil, err
		}
	} //nolint:errcheck // primary lock error.
	result, err := openTree(folder, "database", owner, readonly)
	if err != nil {
		if owner != nil {
			owner.Close()
		}
		folder.Close()
		return nil, err
	} //nolint:errcheck // primary open error.
	return result, nil
}
func openTree(folder *os.File, name string, owner *os.File, readonly bool) (*store, error) {
	root, err := openNode(folder, name, true)
	if err != nil {
		return nil, err
	}
	tree := &treeEncoding{root: root, folder: folder}
	if _, err := tree.list(""); err != nil {
		root.Close()
		return nil, err
	} //nolint:errcheck // primary validation error.
	return newStore(tree, nil, owner, readonly), nil
}
func lockOwner(folder *os.File, name string) (*os.File, error) {
	return lockStoreFile(folder, name, unix.LOCK_EX)
}

func lockStoreFile(folder *os.File, name string, mode int) (*os.File, error) {
	fd, err := unix.Openat(int(folder.Fd()), name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", filepath.Join(folder.Name(), name), err)
	}
	file := os.NewFile(uintptr(fd), filepath.Join(folder.Name(), name))
	if err := secureNode(file, false); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if err := unix.Flock(fd, mode|unix.LOCK_NB); err != nil {
		return nil, errors.Join(fmt.Errorf("%w: %s; close its owner before maintenance: %v", ErrBusy, folder.Name(), err), file.Close())
	}
	if err := folder.Sync(); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

// Create auto-creates an empty tree, or reopens the completed winner. A busy
// winner still owns its lock and is refused. The caller MUST Close the result.
func Create(dir string) (Storage, error) {
	if err := detect(dir); err == nil {
		return Open(dir)
	} else if !errors.Is(err, ErrNoStore) {
		return nil, err
	}
	result, err := CreatePopulated(dir, func(Storage) error { return nil })
	if errors.Is(err, fs.ErrExist) {
		return Open(dir)
	}
	return result, err
}

// CreatePopulated seeds a private tree and publishes it only after success.
// It strictly refuses an existing tree or blob. The caller MUST Close the result.
func CreatePopulated(dir string, populate func(Storage) error) (Storage, error) {
	return populateTree(dir, populate, false)
}

// ReplacePopulated seeds privately, then replaces under the lifetime owner lock.
// It preserves the old tree/blob as .replaced-<stamp>. Caller MUST Close the result.
func ReplacePopulated(dir string, populate func(Storage) error) (Storage, error) {
	return populateTree(dir, populate, true)
}

func ensureFolder(dir string) (*os.File, error) {
	return openFolderMode(dir, true)
}
func populateTree(dir string, populate func(Storage) error, replace bool) (Storage, error) {
	folder, err := ensureFolder(dir)
	if err != nil {
		return nil, err
	}
	owner, err := lockOwner(folder, "database.lock")
	if err != nil {
		folder.Close()
		return nil, err
	} //nolint:errcheck // primary ownership error.
	result, err := populateOwned(folder, owner, populate, replace)
	if err != nil {
		owner.Close()
		folder.Close()
	} //nolint:errcheck // primary operation error.
	return result, err
}
func populateOwned(folder, owner *os.File, populate func(Storage) error, replace bool) (_ *store, retErr error) {
	if !replace {
		for _, name := range []string{"database", "database.zefs"} {
			if err := nodeStat(folder, name); err == nil {
				return nil, fmt.Errorf("database already exists: %s: %w", filepath.Join(folder.Name(), name), fs.ErrExist)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
	}
	if replace {
		for _, name := range []string{"database", "database.zefs"} {
			node, err := openNode(folder, name, name == "database")
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if name == "database" {
				tree := &treeEncoding{root: node, folder: folder}
				if _, err := tree.list(""); err != nil {
					return nil, errors.Join(err, node.Close())
				}
			}
			if err := node.Close(); err != nil {
				return nil, err
			}
		}
	}
	stage, err := makeStage(folder, "database.init-tmp-")
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			retErr = errors.Join(retErr, removeStage(folder, stage))
		}
	}()
	stageFolder, err := duplicateFolder(folder)
	if err != nil {
		return nil, err
	}
	result, err := openTree(stageFolder, stage, nil, false)
	if err != nil {
		stageFolder.Close()
		return nil, err
	} //nolint:errcheck // primary staging error.
	if err := populate(result); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if err := result.tree.barrier(result.tree.root); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if replace {
		for _, name := range []string{"database", "database.zefs"} {
			if err := nodeStat(folder, name); err == nil {
				if _, err := moveAside(folder, name); err != nil {
					return nil, errors.Join(err, result.Close())
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, errors.Join(err, result.Close())
			}
		}
	}
	if err := renameNoReplace(folder, stage, "database"); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	published = true
	if err := folder.Sync(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if err := result.tree.published("database"); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	result.owner = owner
	if err := folder.Close(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}
func moveAside(folder *os.File, name string) (string, error) {
	backup := name + ".replaced-" + time.Now().Format("20060102T150405.000000000")
	if err := renameNoReplace(folder, name, backup); err != nil {
		return "", err
	}
	if err := folder.Sync(); err != nil {
		return "", err
	}
	return filepath.Join(folder.Name(), backup), nil
}

// WriteConfigFile atomically publishes an explicitly selected loose config, not
// a storage backend. Its caller owns source-conflict detection and commit intent.
func WriteConfigFile(path string, data []byte) error {
	dir, name, err := splitStorePath(path)
	if err != nil {
		return err
	}
	parent, err := openFolder(dir)
	if err != nil {
		return err
	}
	defer parent.Close() //nolint:errcheck // sync reports publication errors.
	return installBytes(parent, parent, name, data, func(file *os.File) error { return file.Sync() })
}
