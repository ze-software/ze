//go:build linux

// Design: docs/architecture/core-design.md -- collision-safe scratch migration
// Overview: move.go -- staged cross-device moves
package scratch

import "golang.org/x/sys/unix"

func renameNoReplace(source, target string) error {
	return unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, target, unix.RENAME_NOREPLACE)
}
