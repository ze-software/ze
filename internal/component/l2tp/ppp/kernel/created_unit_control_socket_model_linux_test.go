// Design: docs/architecture/appliance/kernel-profiles.md -- original PPP control ABI models
// These are non-native layout/query-decision models, not positive file authority.

//go:build linux

package kernel

import (
	"testing"
	"unsafe"
)

// TestCreatedUnitControlSocketLayout checks both exact aligned ioctl structures
// and input-only request defaults, including every reserved word.
func TestCreatedUnitControlSocketLayout(t *testing.T) {
	request := createdUnitControlRequest{abi: 1, kind: 2, unit: 7, index: 37}
	if unsafe.Sizeof(request) != 32 {
		t.Fatal("ioctl43 request is not 32 bytes")
	}
	if unsafe.Alignof(request) != 4 {
		t.Fatal("ioctl43 request alignment is not four")
	}
	for _, entry := range []struct{ actual, wanted uintptr }{
		{unsafe.Offsetof(request.abi), 0},
		{unsafe.Offsetof(request.kind), 4},
		{unsafe.Offsetof(request.unit), 8},
		{unsafe.Offsetof(request.index), 12},
		{unsafe.Offsetof(request.flags), 16},
		{unsafe.Offsetof(request.reserved), 20},
	} {
		if entry.actual != entry.wanted {
			t.Fatalf("ioctl43 field offset=%d, want %d", entry.actual, entry.wanted)
		}
	}
	if request.flags != 0 {
		t.Fatal("ioctl43 input flags are not zero")
	}
	if request.reserved != [3]uint32{} {
		t.Fatal("ioctl43 input reserved words are not zero")
	}
	owner := pppPairUnitOwner{abi: 1, ifindex: 37, unit: 7}
	if unsafe.Sizeof(owner) != 20 {
		t.Fatal("query51 request is not 20 bytes")
	}
	if unsafe.Alignof(owner) != 4 {
		t.Fatal("query51 alignment is not four")
	}
	if owner.features != 0 {
		t.Fatal("query51 input has caller-minted feature bits")
	}
	if owner.reserved != 0 {
		t.Fatal("query51 input reserved field is not zero")
	}
	if createdUnitControlSocket != 0x4020742b {
		t.Fatal("constructor number differs from _IOW('t',43,32B)")
	}
}

// TestCreatedUnitControlSocketOwnerReply models the actual post-query decision.
// The useful model never reaches an ioctl or creates a positive owner object.
func TestCreatedUnitControlSocketOwnerReply(t *testing.T) {
	positive := pppPairUnitOwner{abi: 1, features: 1 << 9, ifindex: 37, unit: 7}
	if err := validateCreatedUnitControlOwner(positive, 37, 7); err != nil {
		t.Fatalf("well-formed positive query model refused: %v", err)
	}
	for _, change := range []struct {
		name  string
		apply func(*pppPairUnitOwner)
	}{
		{"version", func(reply *pppPairUnitOwner) { reply.abi++ }},
		{"index", func(reply *pppPairUnitOwner) { reply.ifindex++ }},
		{"unit", func(reply *pppPairUnitOwner) { reply.unit++ }},
		{"reserved", func(reply *pppPairUnitOwner) { reply.reserved = 1 }},
		{"no features", func(reply *pppPairUnitOwner) { reply.features = 0 }},
		{"foundation bits without complete control", func(reply *pppPairUnitOwner) { reply.features = 0x1ff }},
	} {
		t.Run(change.name, func(t *testing.T) {
			reply := positive
			change.apply(&reply)
			if err := validateCreatedUnitControlOwner(reply, 37, 7); err == nil {
				t.Fatal("malformed or unsupported query model permitted constructor43")
			}
		})
	}
}
