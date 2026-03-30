package iface

import (
	"testing"
)

func TestSetupMirrorInvalidSrc(t *testing.T) {
	// VALIDATES: SetupMirror rejects empty source interface name.
	// PREVENTS: Invalid names reaching netlink syscalls.
	err := SetupMirror("", "dst0", true, false)
	if err == nil {
		t.Fatal("expected error for empty src name, got nil")
	}
}

func TestSetupMirrorInvalidDst(t *testing.T) {
	// VALIDATES: SetupMirror rejects empty destination interface name.
	// PREVENTS: Invalid names reaching netlink syscalls.
	err := SetupMirror("src0", "", true, false)
	if err == nil {
		t.Fatal("expected error for empty dst name, got nil")
	}
}

func TestRemoveMirrorInvalidName(t *testing.T) {
	// VALIDATES: RemoveMirror rejects empty interface name.
	// PREVENTS: Invalid names reaching netlink syscalls.
	err := RemoveMirror("")
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}
