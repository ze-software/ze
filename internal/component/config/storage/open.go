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

	"github.com/ze-software/ze/internal/core/slogutil"
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

// TreeName and BlobName are the live tree's and the canonical seed's folder
// entries, for a caller outside the package that must recognize them.
const (
	TreeName = treeName
	BlobName = blobName
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
		// An offline tree sweeps only the frames named for itself. A legacy
		// frame carries no lock name, so only the live owner sweeps it.
		sweepStages(folder, func(entry string) bool { return stagedFor(entry, name) }, slogutil.Logger("storage"))
	}
	result, err := openTree(folder, name, name, owner, !writable)
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

// detect answers whether dir holds a live tree an ordinary opener may use. An
// import intent is checked first, even beside an existing tree or canonical
// seed: an unfinished import or restore reports ErrImportPending naming its
// recovery command, and nothing is moved here.
func detect(dir string) error {
	if err := pendingImport(dir); err != nil {
		return err
	}
	blob := filepath.Join(dir, blobName)
	if _, err := os.Lstat(blob); err == nil {
		return fmt.Errorf("blob artifact %s is not a live store; run ze init --from %s", blob, blob)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	path := filepath.Join(dir, treeName)
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
	// Validation precedes the lock so a refused open creates no lock file.
	result, err := openTree(folder, treeName, treeName, nil, readonly)
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
	// The intent check is repeated under the owner lock, so an import that
	// started between detect and the lock is never handed a writer.
	if err := pendingImport(dir); err != nil {
		return nil, errors.Join(err, owner.Close(), result.Close())
	}
	// Only the owner sweeps, and only once it holds the lock: a reader open
	// beside a live owner would otherwise remove that owner's in-flight frames.
	sweepStages(folder, liveLeftover, slogutil.Logger("storage"))
	result.owner = owner
	return result, nil
}

// openTree opens tree name in folder. lock is the tree whose owner lock covers
// its writes, which names the frames they stage (stageName): name itself for a
// tree with its own lock, treeName for a stage built under the live lock.
// liveLeftover matches the frames the live owner sweeps: those named for the
// live tree, and the legacy form no write produces any more.
func liveLeftover(entry string) bool {
	return stagedFor(entry, treeName) || stagedLegacy(entry)
}

func openTree(folder *os.File, name, lock string, owner *os.File, readonly bool) (*store, error) {
	root, err := openNode(folder, name, true)
	if err != nil {
		return nil, err
	}
	tree := &treeEncoding{root: root, folder: folder, lock: lock}
	if _, err := tree.list(""); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return newStore(tree, nil, owner, readonly), nil
}

func lockOwner(folder *os.File, name string) (*os.File, error) {
	return lockStoreFile(folder, name, unix.LOCK_EX, (*os.File).Sync)
}

// lockStoreFile opens or creates the lock file name in folder and takes mode
// on it without blocking. syncFolder is the directory barrier, a parameter so
// a test can count it; every production caller passes (*os.File).Sync.
func lockStoreFile(folder *os.File, name string, mode int, syncFolder func(*os.File) error) (*os.File, error) {
	fd, created, err := openLockNode(folder, name)
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
	// A flock lives in the kernel, never on disk, so taking it makes nothing
	// durable. Only the exclusive create added a directory entry; a lock file
	// that already existed changed nothing in folder, so its sync is skipped.
	if !created {
		return file, nil
	}
	if err := syncFolder(folder); err != nil {
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
//
// created reports that this call's exclusive create made the file, which is
// the one case where the folder gained an entry its caller must make durable.
func openLockNode(folder *os.File, name string) (fd int, created bool, err error) {
	flags := unix.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	fd, err = unix.Openat(int(folder.Fd()), name, flags|unix.O_CREAT|unix.O_EXCL, 0o600)
	if err == nil {
		return fd, true, nil
	}
	if !errors.Is(err, unix.EEXIST) {
		return fd, false, err
	}
	fd, err = unix.Openat(int(folder.Fd()), name, flags, 0)
	return fd, false, err
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
		// An unfinished import or restore refuses before the existence
		// check, even beside an existing tree: publishing over it would bury
		// the nodes its intent recorded. Replace is the operator's explicit
		// choice and skips this.
		if err := pendingImport(folder.Name()); err != nil {
			return nil, err
		}
		for _, name := range []string{treeName, blobName} {
			if err := nodeStat(folder, name); err == nil {
				return nil, fmt.Errorf("database already exists: %s: %w", filepath.Join(folder.Name(), name), fs.ErrExist)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
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
				tree := &treeEncoding{root: node, folder: folder, lock: treeName}
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
	result, err := openTree(stageFolder, stage, treeName, nil, false)
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

// WriteConfigFile atomically publishes the explicitly selected loose config at
// path, not a storage backend, through s, the store that records it. Its
// caller owns source-conflict detection and commit intent.
//
// For a live tree the frame is staged under the store's lock name, so the next
// owner open sweeps a frame a crash leaves. That is only sound while s holds
// the owner lock and path sits in the store's own folder, so both are refused
// otherwise. A memory store has no folder, no lock and no sweep: its frame
// takes the reserved name stageName gives an empty lock, which no sweep
// matches, so a crash leftover of it stays (storage-backends.md).
func WriteConfigFile(s Storage, path string, data []byte) error {
	return writeConfigFile(s, path, data, func(file *os.File) error { return file.Sync() })
}

// writeConfigFile is WriteConfigFile with its barrier as a parameter, so a test
// can see the frame it stages; every production caller passes (*os.File).Sync.
func writeConfigFile(s Storage, path string, data []byte, syncFile func(*os.File) error) error {
	owned, ok := ownedStore(s)
	if !ok {
		return notLiveTree(path)
	}
	// The read lock keeps Close from releasing the folder and the owner lock
	// while the frame is staged under that lock's name.
	owned.mu.RLock()
	defer owned.mu.RUnlock()
	if owned.closed {
		return fmt.Errorf("publish config %s: its store is already closed: %w; publish before the daemon closes it", path, fs.ErrClosed)
	}
	lock, err := configStageLock(owned, path)
	if err != nil {
		return err
	}
	dir, name, err := splitStorePath(path)
	if err != nil {
		return err
	}
	parent, err := openFolder(dir)
	if err != nil {
		return err
	}
	defer parent.Close() //nolint:errcheck // sync reports publication errors.
	if owned.tree != nil {
		same, err := sameNode(parent, owned.tree.folder)
		if err != nil {
			return fmt.Errorf("publish config %s: %w", path, err)
		}
		if !same {
			return fmt.Errorf("publish config %s: it is not in the store folder %s, so the store's owner lock does not cover its staged write; start ze with the config file in %s", path, owned.tree.folder.Name(), owned.tree.folder.Name())
		}
	}
	return installBytes(parent, parent, lock, name, data, syncFile)
}

// configStageLock answers the lock name a loose config's frame is staged
// under: the live tree's lock, or "" for a memory store, which holds none. A
// blob store and a read-only tree are refused. The caller MUST hold owned.mu.
func configStageLock(owned *store, path string) (string, error) {
	if owned.tree == nil {
		if owned.blob != nil {
			return "", notLiveTree(path)
		}
		return "", nil
	}
	if owned.owner == nil {
		return "", fmt.Errorf("publish config %s: the store is open read-only, so it does not hold the owner lock; publish through the daemon that owns %s", path, owned.tree.folder.Name())
	}
	return owned.tree.lock, nil
}

// notLiveTree refuses a config publication whose store is no live tree.
func notLiveTree(path string) error {
	return fmt.Errorf("publish config %s: its store is not a live tree, so no owner lock covers the write; start ze with the config file in its store folder", path)
}

// sameNode answers whether two open descriptors name the same inode.
func sameNode(a, b *os.File) (bool, error) {
	var first, second unix.Stat_t
	if err := unix.Fstat(int(a.Fd()), &first); err != nil {
		return false, err
	}
	if err := unix.Fstat(int(b.Fd()), &second); err != nil {
		return false, err
	}
	if first.Dev != second.Dev {
		return false, nil
	}
	return first.Ino == second.Ino, nil
}
