// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact state-scope ABI.
// Related: xfrm_state_scope_abi_linux.go -- the scope writer and reader under test.

//go:build linux

package dataplane

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

func scopeWireSelectors() []nl.XfrmSelector {
	return []nl.XfrmSelector{
		{
			Saddr: nl.XfrmAddress{10, 1}, Daddr: nl.XfrmAddress{10, 2},
			Family: unix.AF_INET, PrefixlenS: 16, PrefixlenD: 16, Proto: unix.IPPROTO_UDP,
			Sport: nl.Swap16(0), SportMask: nl.Swap16(0xffff),
			Dport: nl.Swap16(4096), DportMask: nl.Swap16(0xfff0), Ifindex: 9,
		},
		{
			Saddr:  nl.XfrmAddress{0x20, 1, 0xd, 0xb8, 0, 1},
			Daddr:  nl.XfrmAddress{0x20, 1, 0xd, 0xb8, 0, 2},
			Family: unix.AF_INET6, PrefixlenS: 48, PrefixlenD: 48, Proto: unix.IPPROTO_ICMPV6,
			Sport: nl.Swap16(0), SportMask: nl.Swap16(0xffff),
			Dport: nl.Swap16(0), DportMask: nl.Swap16(0xfff8),
		},
	}
}

// scopeWirePayload hand-builds an XFRMA_STATE_SCOPE value: native u32 flags,
// u32 count, then each record as the kernel's struct xfrm_selector.
func scopeWirePayload(flags uint32, selectors []nl.XfrmSelector) []byte {
	payload := make([]byte, 8, 8+len(selectors)*nl.SizeofXfrmSelector)
	binary.NativeEndian.PutUint32(payload, flags)
	binary.NativeEndian.PutUint32(payload[4:], uint32(len(selectors)))
	for i := range selectors {
		payload = append(payload, selectors[i].Serialize()...)
	}
	return payload
}

// stateScopeValue returns the value of the one XFRMA_STATE_SCOPE attribute a
// built state carries. The literal 42 checks the private attribute number.
func stateScopeValue(t *testing.T, state *netlink.XfrmState) []byte {
	t.Helper()
	var value []byte
	found := 0
	for _, attr := range state.ExtraAttrs {
		if attr.Type == 42 {
			value = attr.Value
			found++
		}
	}
	if found != 1 {
		t.Fatalf("state carries %d scope attributes, want one: %+v", found, state.ExtraAttrs)
	}
	return value
}

// TestXFRMStateScopeWirePreservesOrderedSelectors checks the actual wire offsets and
// reads back both families, exact zero and masked ranges without a covering selector.
func TestXFRMStateScopeWirePreservesOrderedSelectors(t *testing.T) {
	selectors := scopeWireSelectors()
	want, err := NewSAStateScope(scopeParamSelectors(), true)
	if err != nil {
		t.Fatal(err)
	}
	var buf [8 + 64*nl.SizeofXfrmSelector + 7]byte
	for i := range buf {
		buf[i] = 0xa5
	}
	n, err := writeXFRMStateScope(buf[:], 3, xfrmStateScopeNonInitial, selectors)
	if err != nil {
		t.Fatal(err)
	}
	if n != 8+2*nl.SizeofXfrmSelector {
		t.Fatalf("encoded %d bytes, want two exact selectors", n)
	}
	if !bytes.Equal(buf[:3], []byte{0xa5, 0xa5, 0xa5}) || buf[3+n] != 0xa5 {
		t.Fatal("writer changed bytes outside its result")
	}
	payload := buf[3 : 3+n]
	if binary.NativeEndian.Uint32(payload) != 1 || binary.NativeEndian.Uint32(payload[4:]) != 2 {
		t.Fatalf("scope header = %x", payload[:8])
	}
	for i := range selectors {
		if !bytes.Equal(payload[8+i*56:8+(i+1)*56], selectors[i].Serialize()) {
			t.Fatalf("selector %d lost its exact UAPI representation", i)
		}
	}
	got, err := parseXFRMStateScope(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scope readback = %+v, want %+v", got, want)
	}
	sent := bytes.Clone(payload)
	clear(payload)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("readback aliases the netlink receive buffer")
	}
	again, err := got.xfrmStateScopeAttr()
	if err != nil {
		t.Fatal(err)
	}
	if again.Type != 42 || !bytes.Equal(again.Value, sent) {
		t.Fatalf("readback re-encodes as type %d %x, want 42 %x", again.Type, again.Value, sent)
	}
}

// TestXFRMStateScopeWritesSelectorFamilies checks that the scope writer accepts
// a canonical selector of each family case and writes it byte for byte, so a
// transport selector equals its scope record. The cases are the selector family
// cases of the vendored netlink library's TestXfrmSelectorFamilyFromMask.
func TestXFRMStateScopeWritesSelectorFamilies(t *testing.T) {
	cases := []struct {
		name   string
		prefix int
		family uint16
		saddr  nl.XfrmAddress
		daddr  nl.XfrmAddress
	}{
		{
			name: "ipv4", prefix: 32, family: nl.FAMILY_V4,
			saddr: nl.XfrmAddress{198, 51, 100, 9}, daddr: nl.XfrmAddress{192, 0, 2, 4},
		},
		{
			name: "ipv4-in-sixteen-bytes", prefix: 32, family: nl.FAMILY_V4,
			saddr: nl.XfrmAddress{198, 51, 100, 9}, daddr: nl.XfrmAddress{192, 0, 2, 4},
		},
		{
			name: "native-ipv6", prefix: 64, family: nl.FAMILY_V6,
			saddr: nl.XfrmAddress{0x20, 1, 0x0d, 0xb8, 0, 1}, daddr: nl.XfrmAddress{0x20, 1, 0x0d, 0xb8, 0, 2},
		},
		{
			name: "mapped-ipv6-host", prefix: 128, family: nl.FAMILY_V6,
			saddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 198, 13: 51, 14: 100, 15: 9},
			daddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 192, 14: 2, 15: 4},
		},
		{
			name: "mapped-ipv6-network", prefix: 120, family: nl.FAMILY_V6,
			saddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 198, 13: 51, 14: 100},
			daddr: nl.XfrmAddress{10: 0xff, 11: 0xff, 12: 192, 14: 2},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := nl.XfrmSelector{
				Family: tc.family, Saddr: tc.saddr, Daddr: tc.daddr,
				PrefixlenS: uint8(tc.prefix), PrefixlenD: uint8(tc.prefix), Proto: 17,
				Sport: 0, SportMask: nl.Swap16(0xffff), Dport: nl.Swap16(0x1200), DportMask: nl.Swap16(0xff00),
				Ifindex: 9,
			}
			var payload [8 + nl.SizeofXfrmSelector]byte
			if _, err := writeXFRMStateScope(payload[:], 0, 0, []nl.XfrmSelector{want}); err != nil {
				t.Fatalf("canonical scope rejected: %v", err)
			}
			if !bytes.Equal(payload[8:], want.Serialize()) {
				t.Fatalf("scope record: got %x, want %x", payload[8:], want.Serialize())
			}
		})
	}
}

// TestXFRMStateScopeWireRejectsInvalidRecords checks the kernel's canonical record
// admission rules at both Go wire boundaries, without accepting malformed tails.
func TestXFRMStateScopeWireRejectsInvalidRecords(t *testing.T) {
	cases := []struct {
		name   string
		change func(*nl.XfrmSelector)
	}{
		{"family", func(s *nl.XfrmSelector) { s.Family = 0 }},
		{"prefix", func(s *nl.XfrmSelector) { s.PrefixlenS = 33 }},
		{"host bits", func(s *nl.XfrmSelector) { s.Saddr[3] = 1 }},
		{"IPv4 union tail", func(s *nl.XfrmSelector) { s.Daddr[15] = 1 }},
		{"padding", func(s *nl.XfrmSelector) { s.Pad[1] = 1 }},
		{"user", func(s *nl.XfrmSelector) { s.User = 1 }},
		{"negative interface", func(s *nl.XfrmSelector) { s.Ifindex = -1 }},
		{"noncontiguous mask", func(s *nl.XfrmSelector) { s.DportMask = nl.Swap16(0xff05) }},
		{"port outside mask", func(s *nl.XfrmSelector) { s.Dport = nl.Swap16(4097) }},
		{"portless protocol", func(s *nl.XfrmSelector) { s.Proto = unix.IPPROTO_GRE }},
		{"ICMP family", func(s *nl.XfrmSelector) { s.Proto = unix.IPPROTO_ICMPV6 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selector := scopeWireSelectors()[0]
			tc.change(&selector)
			var buf [64]byte
			if _, err := writeXFRMStateScope(buf[:], 0, 0, []nl.XfrmSelector{selector}); err == nil {
				t.Fatal("writer accepted a noncanonical selector")
			}
			binary.NativeEndian.PutUint32(buf[4:], 1)
			copy(buf[8:], selector.Serialize())
			if _, err := parseXFRMStateScope(buf[:]); err == nil {
				t.Fatal("reader accepted a noncanonical selector")
			}
		})
	}
}

// TestXFRMStateScopeWireBounds checks the inclusive 1..64 bound, unknown flags,
// short output buffers and exact input lengths before any partial write.
func TestXFRMStateScopeWireBounds(t *testing.T) {
	for _, count := range []int{0, 1, 64, 65} {
		selectors := make([]nl.XfrmSelector, count)
		for i := range selectors {
			selectors[i] = scopeWireSelectors()[i%2]
		}
		buf := bytes.Repeat([]byte{0xa5}, 8+65*56)
		n, err := writeXFRMStateScope(buf, 0, 0, selectors)
		if count == 0 || count == 65 {
			if err == nil || !bytes.Equal(buf, bytes.Repeat([]byte{0xa5}, len(buf))) {
				t.Fatalf("count %d did not refuse without writing", count)
			}
			continue
		}
		if err != nil {
			t.Fatalf("valid count %d: %v", count, err)
		}
		if _, err := parseXFRMStateScope(buf[:n]); err != nil {
			t.Fatalf("valid count %d readback: %v", count, err)
		}
		if _, err := parseXFRMStateScope(buf[:n-1]); err == nil {
			t.Fatal("reader accepted a truncated selector")
		}
		if _, err := parseXFRMStateScope(buf[:n+1]); err == nil {
			t.Fatal("reader accepted trailing bytes")
		}
		if _, err := writeXFRMStateScope(buf[:n-1], 0, 0, selectors); err == nil {
			t.Fatal("writer accepted a short buffer")
		}
	}
	selectors := scopeWireSelectors()
	var buf [120]byte
	if _, err := writeXFRMStateScope(buf[:], 0, 2, selectors); err == nil {
		t.Fatal("writer accepted unknown flags")
	}
	binary.NativeEndian.PutUint32(buf[:], 2)
	binary.NativeEndian.PutUint32(buf[4:], 2)
	for i := range selectors {
		copy(buf[8+i*56:], selectors[i].Serialize())
	}
	if _, err := parseXFRMStateScope(buf[:]); err == nil {
		t.Fatal("reader accepted unknown flags")
	}
}

// FuzzXFRMStateScopeReadback checks that arbitrary netlink scope payloads return
// errors safely, including truncated selector records and attacker-controlled counts.
func FuzzXFRMStateScopeReadback(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0, 255, 255, 255, 255})
	f.Fuzz(func(t *testing.T, payload []byte) {
		_, err := parseXFRMStateScope(payload)
		if err != nil {
			return
		}
	})
}
