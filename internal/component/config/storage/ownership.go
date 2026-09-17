//go:build linux

// Design: docs/architecture/storage-backends.md -- privileged, locked ownership transfer.
// Related: open.go retains strict effective-UID checks for ordinary store access.
package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"
)

// TransferOwnership transfers a config folder, including loose configs and
// artifacts, to uid and gid. The caller MUST run as root with the daemon stopped.
// Concurrent calls are safe: the stable database.lock refuses a second owner.
// Every node is validated and pinned before ownership changes; no file data is read.
func TransferOwnership(dir string, uid, gid int) (err error) {
	if os.Geteuid() != 0 {
		return fmt.Errorf("%w: transfer %s: run ownership maintenance as root", ErrPermissions, dir)
	}
	if uid < 0 || uint64(uid) >= uint64(^uint32(0)) {
		return fmt.Errorf("invalid target owner %d", uid)
	}
	if gid < 0 || uint64(gid) >= uint64(^uint32(0)) {
		return fmt.Errorf("invalid target group %d", gid)
	}

	folder, err := ownershipOpenFolder(dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, folder.Close()) }()

	owner, created, err := ownershipLock(folder)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, owner.Close()) }()

	var root, lock unix.Stat_t
	if err := unix.Fstat(int(folder.Fd()), &root); err != nil {
		return fmt.Errorf("stat %s: %w", folder.Name(), err)
	}
	transferred := false
	defer func() {
		if !created || transferred {
			return
		}
		// Keep the stable inode, but do not leave a root-owned lock behind
		// when transfer fails for a store that belongs to another account.
		if repairErr := unix.Fchown(int(owner.Fd()), int(root.Uid), int(root.Gid)); repairErr != nil {
			err = errors.Join(err, fmt.Errorf("restore newly created lock ownership %s: %w; stop the daemon and chown this path to %d:%d before retrying",
				owner.Name(), repairErr, root.Uid, root.Gid))
		}
		err = errors.Join(err, owner.Sync(), folder.Sync())
	}()
	if err := unix.Fstat(int(owner.Fd()), &lock); err != nil {
		return fmt.Errorf("stat %s: %w", owner.Name(), err)
	}
	if err := ownershipValidate(folder.Name(), &root, root.Uid, false); err != nil {
		return err
	}
	lockUID := root.Uid
	if created {
		// A missing lock is created by this root maintenance process, not the
		// store's previous owner. It still joins the prepared transfer plan.
		lockUID = uint32(os.Geteuid())
	}
	if err := ownershipValidate(owner.Name(), &lock, lockUID, true); err != nil {
		return err
	}

	plan := []ownershipNode{{file: folder, uid: root.Uid, gid: root.Gid, directory: true}}
	defer func() {
		// The root and lock have their own defers; all other pinned descriptors
		// remain open through rollback and are released before the lock.
		for i := 1; i < len(plan); i++ {
			err = errors.Join(err, plan[i].file.Close())
		}
	}()
	if err := ownershipPrepare(root.Uid, &lock, lockUID, &plan); err != nil {
		return err
	}

	// Transfer descendants before the root, retaining every descriptor so a
	// pathname replacement cannot redirect a chown or its rollback.
	for i := len(plan) - 1; i >= 0; i-- {
		node := &plan[i]
		if node.uid == uint32(uid) && node.gid == uint32(gid) {
			continue
		}
		if err := unix.Fchownat(int(node.file.Fd()), "", uid, gid, unix.AT_EMPTY_PATH); err != nil {
			failure := fmt.Errorf("transfer ownership %s: %w; stop the daemon and restore owner %d:%d with chown before retrying",
				node.file.Name(), err, node.uid, node.gid)
			return errors.Join(failure, ownershipRollback(plan[i+1:], uid, gid))
		}
	}
	if err := ownershipSync(plan); err != nil {
		failure := fmt.Errorf("persist ownership metadata for %s: %w; stop the daemon and inspect ownership before retrying",
			folder.Name(), err)
		return errors.Join(failure, ownershipRollback(plan, uid, gid))
	}
	transferred = true
	return nil
}

type ownershipNode struct {
	file      *os.File
	uid       uint32
	gid       uint32
	directory bool
	private   bool
}

// ownershipOpenFolder rejects symlinks in ancestors as well as at the root.
func ownershipOpenFolder(dir string) (*os.File, error) {
	path := dir
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		// Preserve components such as link/.. until nofollow traversal has
		// checked them; lexical cleaning would erase the unsafe ancestor.
		path = cwd + string(filepath.Separator) + path
	}
	parts := strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator))
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	folder := os.NewFile(uintptr(fd), "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		name := filepath.Join(folder.Name(), part)
		fd, err := unix.Openat(int(folder.Fd()), part,
			unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		closeErr := folder.Close()
		if err != nil {
			return nil, errors.Join(fmt.Errorf("open store folder %s: %w", name, err), closeErr)
		}
		folder = os.NewFile(uintptr(fd), name)
		if closeErr != nil {
			return nil, errors.Join(closeErr, folder.Close())
		}
	}
	return folder, nil
}

func ownershipLock(folder *os.File) (*os.File, bool, error) {
	name := filepath.Join(folder.Name(), lockName)
	flags := unix.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	fd, err := unix.Openat(int(folder.Fd()), lockName, flags|unix.O_CREAT|unix.O_EXCL, 0o600)
	created := err == nil
	if errors.Is(err, unix.EEXIST) {
		// Refuse special nodes before opening them, then check the opened inode
		// again. O_NONBLOCK prevents a replaced FIFO from blocking maintenance.
		var info unix.Stat_t
		if err := unix.Fstatat(int(folder.Fd()), lockName, &info, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return nil, false, fmt.Errorf("stat lock %s: %w", name, err)
		}
		if info.Mode&unix.S_IFMT != unix.S_IFREG {
			return nil, false, fmt.Errorf("%w: lock %s is not regular: remove the unsafe node", ErrPermissions, name)
		}
		fd, err = unix.Openat(int(folder.Fd()), lockName, flags, 0)
	}
	if err != nil {
		return nil, false, fmt.Errorf("open lock %s: %w", name, err)
	}
	owner := os.NewFile(uintptr(fd), name)
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		return nil, false, errors.Join(err, owner.Close())
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, false, errors.Join(fmt.Errorf("%w: lock %s is not regular: remove the unsafe node", ErrPermissions, name), owner.Close())
	}
	// Ownership and mode validation deliberately follow the shared lock, so a
	// live daemon wins even when maintenance runs with root privileges.
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, false, errors.Join(fmt.Errorf("%w: %s; stop its daemon before maintenance: %w", ErrBusy, folder.Name(), err), owner.Close())
	}
	return owner, created, nil
}

func ownershipValidate(path string, info *unix.Stat_t, uid uint32, private bool) error {
	mode := uint32(0o600)
	publicBits := uint32(0o044)
	switch info.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		mode = 0o700
		publicBits = 0o055
	case unix.S_IFREG:
		// Chowning an externally linked inode would change ownership outside
		// the store, even though this path itself contains no symlink.
		if info.Nlink != 1 {
			return fmt.Errorf("%w: %s has %d links: replace it with a private regular file", ErrPermissions, path, info.Nlink)
		}
	default:
		return fmt.Errorf("%w: %s is not a directory or regular file: remove the unsafe node", ErrPermissions, path)
	}
	permissions := info.Mode & 0o7777
	if !private {
		// Ordinary config files and their containing folders can be readable
		// by other accounts, but never writable or executable regular files.
		permissions &^= publicBits
	}
	if permissions != mode {
		return fmt.Errorf("%w: %s mode %04o: chmod %04o before retrying", ErrPermissions, path, info.Mode&0o7777, mode)
	}
	if info.Uid != uid {
		return fmt.Errorf("%w: %s owner %d, expected %d: restore uniform store ownership before retrying", ErrPermissions, path, info.Uid, uid)
	}
	return nil
}

// ownershipPrepare visits directories iteratively in fixed-size listing chunks.
// Pinned descriptors cost one per node; descriptor exhaustion fails before chown.
func ownershipPrepare(uid uint32, lock *unix.Stat_t, lockUID uint32, plan *[]ownershipNode) error {
	// The plan grows while visiting parents; range would snapshot its initial
	// length and omit directories appended during traversal.
	for index := 0; index < len(*plan); index++ { //nolint:intrange // the plan grows during the walk
		if !(*plan)[index].directory {
			continue
		}
		folder := (*plan)[index].file
		private := (*plan)[index].private
		// Each iteration consumes the next directory chunk, until EOF or an
		// operating-system error. The complete node plan remains pinned.
		for {
			entries, err := folder.ReadDir(128)
			if err != nil && !errors.Is(err, io.EOF) {
				return fmt.Errorf("list %s: %w", folder.Name(), err)
			}
			for _, entry := range entries {
				name := entry.Name()
				path := filepath.Join(folder.Name(), name)
				fd, openErr := unix.Openat(int(folder.Fd()), name, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
				if openErr != nil {
					return fmt.Errorf("open node %s: %w", path, openErr)
				}
				file := os.NewFile(uintptr(fd), path)
				var info unix.Stat_t
				if err := unix.Fstat(fd, &info); err != nil {
					return errors.Join(fmt.Errorf("stat %s: %w", path, err), file.Close())
				}
				ownerUID := uid
				if index == 0 && name == lockName {
					if info.Dev != lock.Dev || info.Ino != lock.Ino {
						return errors.Join(fmt.Errorf("%w: %s changed during ownership maintenance: stop other maintenance processes", ErrBusy, path), file.Close())
					}
					ownerUID = lockUID
				}
				childPrivate := private || ownershipPrivate(name)
				if err := ownershipValidate(path, &info, ownerUID, childPrivate); err != nil {
					return errors.Join(err, file.Close())
				}
				directory := info.Mode&unix.S_IFMT == unix.S_IFDIR
				flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
				parent, leaf := int(folder.Fd()), name
				if directory {
					// Directory identity is pinned; regular files reopen by
					// name and must match the validated inode before use.
					parent, leaf = int(file.Fd()), "."
					flags |= unix.O_DIRECTORY
				}
				fd, reopenErr := unix.Openat(parent, leaf, flags, 0)
				closeErr := file.Close()
				if reopenErr != nil {
					return errors.Join(fmt.Errorf("reopen node %s: %w", path, reopenErr), closeErr)
				}
				file = os.NewFile(uintptr(fd), path)
				if closeErr != nil {
					return errors.Join(closeErr, file.Close())
				}
				var pinned unix.Stat_t
				if err := unix.Fstat(fd, &pinned); err != nil {
					return errors.Join(fmt.Errorf("stat reopened node %s: %w", path, err), file.Close())
				}
				if pinned.Dev != info.Dev || pinned.Ino != info.Ino {
					return errors.Join(fmt.Errorf("%w: %s changed during ownership preparation: stop other maintenance processes", ErrBusy, path), file.Close())
				}
				if err := ownershipValidate(path, &pinned, ownerUID, childPrivate); err != nil {
					return errors.Join(err, file.Close())
				}
				*plan = append(*plan, ownershipNode{
					file: file, uid: pinned.Uid, gid: pinned.Gid,
					directory: directory, private: childPrivate,
				})
			}
			if errors.Is(err, io.EOF) {
				break
			}
		}
	}
	return nil
}

// Known storage names remain private even beside publicly readable loose configs.
// Descendants inherit this classification, including replaced and staged trees.
func ownershipPrivate(name string) bool {
	return name == treeName || name == importIntentName ||
		strings.HasPrefix(name, treeName+replacedInfix) ||
		strings.HasPrefix(name, initStagePrefix) ||
		strings.HasPrefix(name, importStagePrefix) ||
		strings.HasSuffix(name, ".zefs") || strings.HasSuffix(name, ".lock") ||
		strings.Contains(name, ".zefs"+replacedInfix) ||
		strings.HasPrefix(name, ".ze-storage-") || strings.HasPrefix(name, ".ze-blob-")
}

// ownershipSync persists every prepared inode without reading file contents.
func ownershipSync(plan []ownershipNode) error {
	var result error
	for i := range slices.Backward(plan) {
		result = errors.Join(result, plan[i].file.Sync())
	}
	return result
}

func ownershipRollback(plan []ownershipNode, uid, gid int) error {
	var result error
	for i := range plan {
		node := &plan[i]
		if node.uid == uint32(uid) && node.gid == uint32(gid) {
			continue
		}
		if err := unix.Fchownat(int(node.file.Fd()), "", int(node.uid), int(node.gid), unix.AT_EMPTY_PATH); err != nil {
			result = errors.Join(result, fmt.Errorf("rollback ownership %s: %w; stop the daemon and chown this path to %d:%d before retrying",
				node.file.Name(), err, node.uid, node.gid))
		}
		result = errors.Join(result, node.file.Sync())
	}
	return result
}
