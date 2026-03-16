// Design: docs/architecture/system-architecture.md — daemon privilege dropping
// Overview: drop.go — user/group resolution

//go:build linux || darwin || freebsd || openbsd || netbsd

package privilege

import (
	"fmt"
	"os"
	"syscall"
)

// Drop resolves the configured user/group and drops privileges via
// setgroups, setgid, then setuid. Must be called after binding privileged ports.
// Returns nil if not running as root (nothing to drop).
//
// The canonical sequence is: setgroups -> setgid -> setuid.
// setgroups clears root's supplementary groups (wheel, docker, adm, etc.).
// setgid must happen while still root.
// setuid is irreversible and must be last.
func Drop(cfg DropConfig) error {
	if os.Getuid() != 0 {
		return nil
	}

	uid, gid, suppGroups, err := resolveIDs(cfg)
	if err != nil {
		return err
	}

	// 1. setgroups: replace root's supplementary groups with the target user's
	if err := syscall.Setgroups(suppGroups); err != nil {
		return fmt.Errorf("setgroups(%v): %w", suppGroups, err)
	}

	// 2. setgid: must happen while still root
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("setgid(%d): %w", gid, err)
	}

	// 3. setuid: irreversible, must be last
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("setuid(%d): %w", uid, err)
	}

	// Verify the drop actually took effect (defense-in-depth).
	// Catches edge cases where syscall returns nil but credentials didn't change
	// (e.g., capabilities-based systems, saved set-user-ID remaining 0).
	if os.Getuid() != uid || os.Geteuid() != uid {
		return fmt.Errorf("privilege drop verification failed: want uid %d, got real=%d effective=%d", uid, os.Getuid(), os.Geteuid())
	}
	if os.Getgid() != gid || os.Getegid() != gid {
		return fmt.Errorf("privilege drop verification failed: want gid %d, got real=%d effective=%d", gid, os.Getgid(), os.Getegid())
	}

	return nil
}
