// Design: docs/architecture/zefs-format.md -- exclusive tree repair publication.
// Related: check_tree_unix.go -- shared Linux, Darwin and FreeBSD traversal.

package zefs

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// RenameNoReplace renames source to target beneath parent and fails with
// EEXIST when target exists. FreeBSD has no renameat2: stable/14
// sys/sys/syscall.h ends at SYS_MAXSYSCALL 595 and names no renameat2
// (verified 2026-09-17), so the no-replace guarantee comes from the primitive
// that refuses an existing name. A regular file is hard-linked to target, which
// is atomic and refuses an existing target, then the source name is removed.
// A directory cannot be linked: target is claimed with mkdirat, which refuses
// an existing name, and renameat then replaces only that empty placeholder.
// Live-store publication runs under the store's ownership lock; RepairPath
// (check.go, repairFrameTree) holds no lock and needs none, because a second
// publisher's mkdirat fails with EEXIST on the placeholder, so nothing else can
// fill it between the two calls. A crash between them leaves an empty target
// directory beside the stage, which the next open accepts as an empty store
// (docs/architecture/storage-backends.md, FreeBSD note).
func RenameNoReplace(parent *os.File, source, target string) error {
	fd := int(parent.Fd())
	var st unix.Stat_t
	if err := unix.Fstatat(fd, source, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		if err := unix.Linkat(fd, source, fd, target, 0); err != nil {
			// An interrupted earlier call left both names on one inode; only
			// that exact case finishes by removing the source name.
			var linked unix.Stat_t
			if !errors.Is(err, unix.EEXIST) || unix.Fstatat(fd, target, &linked, unix.AT_SYMLINK_NOFOLLOW) != nil {
				return err
			}
			if linked.Dev != st.Dev {
				return err
			}
			if linked.Ino != st.Ino {
				return err
			}
		}
		return unix.Unlinkat(fd, source, 0)
	}
	if err := unix.Mkdirat(fd, target, 0o700); err != nil {
		return err
	}
	if err := unix.Renameat(fd, source, fd, target); err != nil {
		return errors.Join(err, unix.Unlinkat(fd, target, unix.AT_REMOVEDIR))
	}
	return nil
}

func trustedFramePath(path string) string {
	return path
}
