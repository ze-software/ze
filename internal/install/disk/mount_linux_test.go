// Design: docs/architecture/appliance/installer-initrd.md -- mount/umount wrapper tests

//go:build linux

package disk

import "testing"

func TestMountFSWired(t *testing.T) {
	// On Linux, init() in mount_linux.go replaces the mountFS/umountFS
	// defaults with unix.Mount/Unmount implementations. Verify the vars
	// are non-nil (wired). Actual mount requires root; validated by QEMU.
	if mountFS == nil {
		t.Fatal("mountFS not wired on linux")
	}
	if umountFS == nil {
		t.Fatal("umountFS not wired on linux")
	}
}
