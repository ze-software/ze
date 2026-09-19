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

	"github.com/ze-software/ze/pkg/zefs"
)

// treeName is the published live tree inside a storage folder, and blobName the
// legacy framed blob beside it that the live opener refuses. lockName is the
// tree's owner lock, initStagePrefix the private stage Create builds before it
// publishes, and replacedInfix the stamp separator moveAside and the importer
// put between a retired name and its timestamp. All are folder entries, never
// paths: every caller joins them onto the folder it holds.
const (
	treeName        = "database"
	blobName        = "database.zefs"
	lockName        = "database.lock"
	initStagePrefix = "database.init-tmp-"
	replacedInfix   = ".replaced-"
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
	if name == treeName {
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
			return nil, errors.Join(err, folder.Close())
		}
	}
	result, err := openTree(folder, name, owner, !writable)
	if err != nil {
		if owner != nil {
			err = errors.Join(err, owner.Close())
		}
		return nil, errors.Join(err, folder.Close())
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
	blob := filepath.Join(dir, blobName)
	if _, err := os.Lstat(blob); err == nil {
		return fmt.Errorf("blob artifact %s is not a live store; run ze init --from %s", blob, blob)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	path := filepath.Join(dir, treeName)
	if _, err := os.Lstat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return missingTree(dir, path)
		}
		return err
	}
	return nil
}

// missingTree answers for an absent tree. With no import intent beside it the
// store is genuinely absent and Create may build one. With an intent beside it
// an import crashed after moving the old tree to database.replaced-* and
// before publishing its stage, so the answer is ErrImportPending, never
// ErrNoStore: auto-creating an empty tree there would hide the operator's
// store, and the next import would then be refused as "names another tree".
// An intent that cannot be read refuses the same way, because a store whose
// intent is unreadable is not known to be absent.
func missingTree(dir, path string) error {
	intentPath := filepath.Join(dir, importIntentName)
	if _, err := os.Lstat(intentPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s; run ze init", ErrNoStore, path)
		}
		return err
	}
	folder, err := openFolder(dir)
	if err != nil {
		return err
	}
	intent, exists, err := readImportIntent(folder)
	err = errors.Join(err, folder.Close())
	if err != nil {
		return fmt.Errorf("%w: %s cannot be read and %s is absent: %w; the tree in place before the import sits in %s.replaced-*: re-run ze init --from with the original blob, or remove that intent file once %s holds the wanted tree", ErrImportPending, intentPath, path, err, path, path)
	}
	if !exists {
		return fmt.Errorf("%w: %s; run ze init", ErrNoStore, path)
	}
	return fmt.Errorf("%w: %s records an import of %s and %s is absent; the tree in place before the import sits in %s.replaced-*: finish the import with ze init --from %s, or remove that intent file once %s holds the wanted tree", ErrImportPending, intentPath, intent.Source, path, path, intent.Source, path)
}

func openLive(dir string, readonly bool) (Storage, error) {
	if err := detect(dir); err != nil {
		return nil, err
	}
	folder, err := openFolder(dir)
	if err != nil {
		return nil, err
	}
	// Validation precedes the lock so a refused open creates no lock file.
	result, err := openTree(folder, treeName, nil, readonly)
	if err != nil {
		return nil, errors.Join(err, folder.Close())
	}
	if readonly {
		return result, nil
	}
	owner, err := lockOwner(folder, lockName)
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	result.owner = owner
	return result, nil
}

func openTree(folder *os.File, name string, owner *os.File, readonly bool) (*store, error) {
	root, err := openNode(folder, name, true)
	if err != nil {
		return nil, err
	}
	tree := &treeEncoding{root: root, folder: folder}
	if _, err := tree.list(""); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return newStore(tree, nil, owner, readonly), nil
}

func lockOwner(folder *os.File, name string) (*os.File, error) {
	return lockStoreFile(folder, name, unix.LOCK_EX)
}

func lockStoreFile(folder *os.File, name string, mode int) (*os.File, error) {
	fd, err := openLockNode(folder, name)
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", filepath.Join(folder.Name(), name), err)
	}
	file := os.NewFile(uintptr(fd), filepath.Join(folder.Name(), name))
	if err := secureNode(file, false); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if err := unix.Flock(fd, mode|unix.LOCK_NB); err != nil {
		return nil, errors.Join(fmt.Errorf("%w: %s; close its owner before maintenance: %w", ErrBusy, folder.Name(), err), file.Close())
	}
	if err := folder.Sync(); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

// openLockNode creates the lock beside the store, or opens the one a racing
// process created first.
//
// The exclusive create is what tells the two apart, and it is not an
// optimisation: darwin does not make open(O_CREAT) atomic against a concurrent
// create of the same name, and reports ENOENT to the loser for a file that
// exists (measured 2026-09-19: two threads racing one openat on APFS failed the
// loser 200 times in 200 rounds, with and without O_NOFOLLOW; a plain O_EXCL
// create in the same race reported EEXIST every time). A daemon start that
// coincided with another was therefore refused for a missing lock file instead
// of being told the store has an owner. ownershipLock reads the lock the same
// way, so the package has one shape for a create-or-open.
func openLockNode(folder *os.File, name string) (int, error) {
	flags := unix.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	fd, err := unix.Openat(int(folder.Fd()), name, flags|unix.O_CREAT|unix.O_EXCL, 0o600)
	if !errors.Is(err, unix.EEXIST) {
		return fd, err
	}
	return unix.Openat(int(folder.Fd()), name, flags, 0)
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
	owner, err := lockOwner(folder, lockName)
	if err != nil {
		return nil, errors.Join(err, folder.Close())
	}
	result, err := populateOwned(folder, owner, populate, replace)
	if err != nil {
		return nil, errors.Join(err, owner.Close(), folder.Close())
	}
	return result, nil
}

func populateOwned(folder, owner *os.File, populate func(Storage) error, replace bool) (_ *store, retErr error) {
	if !replace {
		for _, name := range []string{treeName, blobName} {
			if err := nodeStat(folder, name); err == nil {
				return nil, fmt.Errorf("database already exists: %s: %w", filepath.Join(folder.Name(), name), fs.ErrExist)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
		// The tree is absent, so missingTree decides whether that absence is
		// a store to create (ErrNoStore) or an import that crashed after
		// moving the operator's tree to database.replaced-* (ErrImportPending).
		// Publishing an empty tree over the second would bury that tree, so
		// the same refusal detect gives every opener applies here. Replace is
		// the operator's explicit choice and skips this.
		if err := missingTree(folder.Name(), filepath.Join(folder.Name(), treeName)); !errors.Is(err, ErrNoStore) {
			return nil, err
		}
	}
	if replace {
		for _, name := range []string{treeName, blobName} {
			node, err := openNode(folder, name, name == treeName)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if name == treeName {
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
	stage, err := makeStage(folder, initStagePrefix)
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
		return nil, errors.Join(err, stageFolder.Close())
	}
	if err := populate(result); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if err := result.tree.barrier(result.tree.root); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if replace {
		for _, name := range []string{treeName, blobName} {
			if err := nodeStat(folder, name); err == nil {
				if err := moveAside(folder, name); err != nil {
					return nil, errors.Join(err, result.Close())
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, errors.Join(err, result.Close())
			}
		}
	}
	if err := zefs.RenameNoReplace(folder, stage, treeName); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	published = true
	if err := folder.Sync(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if err := result.tree.published(treeName); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	result.owner = owner
	if err := folder.Close(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

// moveAside renames name to a stamped .replaced- sibling and syncs the folder,
// so a replace never unlinks the tree or blob it supersedes.
func moveAside(folder *os.File, name string) error {
	backup := name + replacedInfix + time.Now().Format("20060102T150405.000000000")
	if err := zefs.RenameNoReplace(folder, name, backup); err != nil {
		return err
	}
	return folder.Sync()
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
