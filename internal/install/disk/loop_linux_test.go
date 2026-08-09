// Design: docs/architecture/appliance/installer-initrd.md -- loop device ioctl tests

//go:build linux

package disk

import "testing"

func TestLoopFnsWired(t *testing.T) {
	if loopAttach == nil {
		t.Fatal("loopAttach not wired on linux")
	}
	if loopDetach == nil {
		t.Fatal("loopDetach not wired on linux")
	}
	if ensureLoopDevices == nil {
		t.Fatal("ensureLoopDevices not wired on linux")
	}
}
