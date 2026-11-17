//go:build integration && linux

package imagestore

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// TestApplyOtherOwnersKept proves entries of several numeric owners read
// back as written, each kind of entry, and that the set-user and set-group
// bits survive the change of owner.
//
// Method: as root in the QEMU guest, apply a file, a set-user file, a
// folder, a symbolic link and a pipe, each owned by another user and
// group, then read each owner, group and mode back with no link followed.
// The kernel clears the set-user bit when an owner changes, so a store that
// set the mode before the owner would read back 0755.
func TestApplyOtherOwnersKept(t *testing.T) {
	if unix.Geteuid() != 0 {
		t.Fatalf("this test gives entries other owners, so it runs as root in the QEMU guest, not as user %d", unix.Geteuid())
	}
	owned := func(entry testEntry, uid, gid int) testEntry {
		entry.uid = uid
		entry.gid = gid
		return entry
	}
	store := newAppliedStore(t, testLimits())
	store.mustApply(t,
		owned(fileEntry("root-file", "root\n", 0o644), 0, 0),
		owned(fileEntry("user-file", "user\n", 0o640), 1000, 1001),
		owned(fileEntry("setuid-file", "setuid\n", 0o4755), 1000, 1000),
		owned(fileEntry("setgid-file", "setgid\n", 0o2755), 0, 50),
		owned(folderEntry("nobody-folder", 0o750), 65534, 65534),
		owned(symlinkEntry("user-link", "user-file"), 1002, 1003),
		owned(fifoEntry("user-pipe", 0o600), 1004, 1005),
	)
	cases := []struct {
		name     string
		uid, gid uint32
		mode     uint32
	}{
		{"root-file", 0, 0, 0o644},
		{"user-file", 1000, 1001, 0o640},
		{"setuid-file", 1000, 1000, 0o4755},
		{"setgid-file", 0, 50, 0o2755},
		{"nobody-folder", 65534, 65534, 0o750},
		{"user-link", 1002, 1003, 0o777},
		{"user-pipe", 1004, 1005, 0o600},
	}
	for _, c := range cases {
		var stat unix.Stat_t
		if err := unix.Lstat(filepath.Join(store.rootPath, c.name), &stat); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if stat.Uid != c.uid {
			t.Errorf("%s: owner %d, want %d", c.name, stat.Uid, c.uid)
		}
		if stat.Gid != c.gid {
			t.Errorf("%s: group %d, want %d", c.name, stat.Gid, c.gid)
		}
		if got := stat.Mode & 0o7777; got != c.mode {
			t.Errorf("%s: mode %#o, want %#o", c.name, got, c.mode)
		}
	}
}
