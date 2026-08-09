// Design: docs/architecture/appliance/installer-initrd.md -- block device ioctl tests

//go:build linux

package disk

import "testing"

func TestBlockdevRereadPartSignature(t *testing.T) {
	var fn = blkRereadPart
	_ = fn
}

func TestSyscallVarsWired(t *testing.T) {
	if syncFS == nil {
		t.Fatal("syncFS not wired on linux")
	}
	if rebootFS == nil {
		t.Fatal("rebootFS not wired on linux")
	}
	if poweroffFS == nil {
		t.Fatal("poweroffFS not wired on linux")
	}
}
