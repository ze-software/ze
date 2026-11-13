// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact state-scope ABI.
// Related: xfrm_state_scope_linux.go -- immutable directional selector scope.
// Upstream: github.com/vishvananda/netlink v1.3.1, Ze's Linux 7.3 XFRM extension.

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

// TestXFRMStateScopeWirePreservesOrderedSelectors checks the actual wire offsets and
// reads back both families, exact zero and masked ranges without a covering selector.
func TestXFRMStateScopeWirePreservesOrderedSelectors(t *testing.T) {
	want := netlink.XfrmStateScope{Flags: netlink.XfrmStateScopeNonInitial, Selectors: scopeWireSelectors()}
	var buf [8 + 64*nl.SizeofXfrmSelector + 7]byte
	for i := range buf {
		buf[i] = 0xa5
	}
	n, err := want.WriteTo(buf[:], 3)
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
	for i := range want.Selectors {
		if !bytes.Equal(payload[8+i*56:8+(i+1)*56], want.Selectors[i].Serialize()) {
			t.Fatalf("selector %d lost its exact UAPI representation", i)
		}
	}
	got, err := netlink.ParseXfrmStateScope(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Fatalf("scope readback = %+v, want %+v", got, want)
	}
	clear(payload)
	if !reflect.DeepEqual(got, &want) {
		t.Fatal("readback aliases the netlink receive buffer")
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
			scope := netlink.XfrmStateScope{Selectors: []nl.XfrmSelector{selector}}
			var buf [64]byte
			if _, err := scope.WriteTo(buf[:], 0); err == nil {
				t.Fatal("writer accepted a noncanonical selector")
			}
			binary.NativeEndian.PutUint32(buf[4:], 1)
			copy(buf[8:], selector.Serialize())
			if _, err := netlink.ParseXfrmStateScope(buf[:]); err == nil {
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
		scope := netlink.XfrmStateScope{Selectors: selectors}
		buf := bytes.Repeat([]byte{0xa5}, 8+65*56)
		n, err := scope.WriteTo(buf, 0)
		if count == 0 || count == 65 {
			if err == nil || !bytes.Equal(buf, bytes.Repeat([]byte{0xa5}, len(buf))) {
				t.Fatalf("count %d did not refuse without writing", count)
			}
			continue
		}
		if err != nil {
			t.Fatalf("valid count %d: %v", count, err)
		}
		if _, err := netlink.ParseXfrmStateScope(buf[:n]); err != nil {
			t.Fatalf("valid count %d readback: %v", count, err)
		}
		if _, err := netlink.ParseXfrmStateScope(buf[:n-1]); err == nil {
			t.Fatal("reader accepted a truncated selector")
		}
		if _, err := netlink.ParseXfrmStateScope(buf[:n+1]); err == nil {
			t.Fatal("reader accepted trailing bytes")
		}
		if _, err := scope.WriteTo(buf[:n-1], 0); err == nil {
			t.Fatal("writer accepted a short buffer")
		}
	}
	scope := netlink.XfrmStateScope{Flags: 2, Selectors: scopeWireSelectors()}
	var buf [120]byte
	if _, err := scope.WriteTo(buf[:], 0); err == nil {
		t.Fatal("writer accepted unknown flags")
	}
	binary.NativeEndian.PutUint32(buf[:], 2)
	binary.NativeEndian.PutUint32(buf[4:], 2)
	for i := range scope.Selectors {
		copy(buf[8+i*56:], scope.Selectors[i].Serialize())
	}
	if _, err := netlink.ParseXfrmStateScope(buf[:]); err == nil {
		t.Fatal("reader accepted unknown flags")
	}
}

// FuzzXFRMStateScopeReadback checks that arbitrary netlink scope payloads return
// errors safely, including truncated selector records and attacker-controlled counts.
func FuzzXFRMStateScopeReadback(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0, 255, 255, 255, 255})
	f.Fuzz(func(t *testing.T, payload []byte) {
		_, err := netlink.ParseXfrmStateScope(payload)
		if err != nil {
			return
		}
	})
}
