// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- explicit capability receipt.
// Related: xfrm_state_scope_abi_linux.go -- the receipt parser under test.

//go:build linux

package dataplane

import (
	"encoding/binary"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

func scopeCapabilitiesReply(features xfrmZEFeature) []byte {
	var payload [16]byte
	binary.NativeEndian.PutUint32(payload[:], uint32(features))
	binary.NativeEndian.PutUint32(payload[4:], 64)
	binary.NativeEndian.PutUint32(payload[8:], 17)
	binary.NativeEndian.PutUint32(payload[12:], 0x5a450001)
	return append([]byte{1, 2, 1, 0}, nl.NewRtAttr(43, payload[:]).Serialize()...)
}

// TestXFRMStateScopeCapabilitiesPreservePartialReceipt checks all five individual
// feature bits and retains K1's partial receipt without claiming fragment support.
func TestXFRMStateScopeCapabilitiesPreservePartialReceipt(t *testing.T) {
	bits := []xfrmZEFeature{
		xfrmZECapStateScope, xfrmZECapICMPQuote,
		xfrmZECapFragmentContext, xfrmZECapFragmentGuard,
		xfrmZECapICMPFinal,
	}
	for _, bit := range bits {
		caps, err := parseXFRMZECapabilities(scopeCapabilitiesReply(bit))
		if err != nil {
			t.Fatal(err)
		}
		if caps.Features != bit || caps.MaxSelectors != 64 || caps.GuardRefs != 17 || caps.ABI != 0x5a450001 {
			t.Fatalf("capability receipt lost fields: %+v", caps)
		}
		for _, candidate := range bits {
			if caps.Features.Has(candidate) != (candidate == bit) {
				t.Fatalf("features %#x reports %#x incorrectly", bit, candidate)
			}
		}
	}
	partial, err := parseXFRMZECapabilities(scopeCapabilitiesReply(3))
	if err != nil {
		t.Fatal(err)
	}
	if partial.Features.Has(15) {
		t.Fatal("K1 receipt claims the full scope and fragment contract")
	}
}

// TestXFRMStateScopeCapabilitiesRejectMissingOrForeignReceipt checks the exact ABI
// receipt rather than treating an ignored request attribute or a three-byte default
// reply as proof that the kernel enforces a scope.
func TestXFRMStateScopeCapabilitiesRejectMissingOrForeignReceipt(t *testing.T) {
	cases := []struct {
		name   string
		change func([]byte) []byte
	}{
		{"absent", func(b []byte) []byte { return b[:3] }},
		{"alignment only", func(b []byte) []byte { return b[:4] }},
		{"short payload", func(b []byte) []byte { return b[:len(b)-1] }},
		{"foreign ABI", func(b []byte) []byte { binary.NativeEndian.PutUint32(b[20:], 0x5a450002); return b }},
		{"zero ABI", func(b []byte) []byte { clear(b[20:]); return b }},
		{"zero maximum", func(b []byte) []byte { clear(b[12:16]); return b }},
		{"foreign maximum", func(b []byte) []byte { binary.NativeEndian.PutUint32(b[12:], 65); return b }},
		{"unknown feature", func(b []byte) []byte { binary.NativeEndian.PutUint32(b[8:], 32); return b }},
		{"ignored scope echo", func(b []byte) []byte { binary.NativeEndian.PutUint16(b[6:], 42); return b }},
		{"duplicate receipt", func(b []byte) []byte { return append(b, b[4:]...) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseXFRMZECapabilities(tc.change(scopeCapabilitiesReply(15))); err == nil {
				t.Fatal("accepted no exact capability receipt")
			}
		})
	}
}
