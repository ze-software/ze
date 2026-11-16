// Design: docs/architecture/wire/attributes.md -- registered next-hop field contracts
// Related: mpnexthop.go -- shared address selection, framing and inline decoding

package attribute

import (
	"bytes"
	"maps"
	"net/netip"
	"slices"
	"testing"
)

// TestMPNextHopProfileRegistration refuses conflicting and inconsistent
// declarations, then exercises the accepted profile through actual wire bytes.
func TestMPNextHopProfileRegistration(t *testing.T) {
	saved := maps.Clone(mpNextHopProfiles)
	t.Cleanup(func() { mpNextHopProfiles = saved })
	afi, safi := AFI(65000), SAFI(241)
	profile := NextHopProfile{Lengths: []int{12, 24, 48}, RDPerAddress: true, IPv6Roles: true}
	if err := RegisterMPNextHopProfile(afi, safi, profile); err != nil {
		t.Fatal(err)
	}
	if err := RegisterMPNextHopProfile(afi, safi, profile); err == nil {
		t.Fatal("duplicate profile accepted")
	}
	if err := RegisterMPNextHopProfile(AFIIPv6, SAFIVPN, profile); err == nil {
		t.Fatal("built-in profile shadowed")
	}
	if err := RegisterMPNextHopProfile(AFIIPv4, 129, profile); err == nil {
		t.Fatal("built-in capability contract without lengths shadowed")
	}
	invalid := []NextHopProfile{
		{},
		{Lengths: []int{8}},
		{Lengths: []int{40}, RDPerAddress: true},
		{Lengths: []int{16}, RDPerAddress: true},
		{Lengths: []int{16, 16}},
		{Lengths: []int{16}, EncodeIPv4Mapped: true},
		{Lengths: []int{4}, MappedIPv4: true},
		{Lengths: []int{12}, RDPerAddress: true, MappedIPv4: true},
		{Lengths: []int{12}, RDPerAddress: true, MappedIPv4: true, EncodeIPv4Mapped: true},
		{Lengths: []int{16}, DecodeUnprefixedIPv6: true},
	}
	for i, input := range invalid {
		if err := RegisterMPNextHopProfile(afi, SAFI(242+i), input); err == nil {
			t.Errorf("invalid profile %d accepted", i)
		}
	}
	got := MPNextHopProfile(afi, safi)
	first := netip.MustParseAddr("2001:db8::1")
	second := netip.MustParseAddr("fe80::2")
	var field [48]byte
	for i := range field {
		field[i] = 0xaa
	}
	if n := got.WriteNextHops(field[:], 0, first, second); n != 48 {
		t.Fatalf("written=%d, want 48", n)
	}
	want := []byte{
		0, 0, 0, 0, 0, 0, 0, 0,
		0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
		0, 0, 0, 0, 0, 0, 0, 0,
		0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2,
	}
	if !bytes.Equal(field[:], want) {
		t.Fatalf("field=%x, want %x", field, want)
	}
	// Received nonzero RDs do not change the address representation or add a
	// refusal policy that the profile does not declare.
	field[0], field[24] = 7, 9
	hops, err := got.DecodeNextHops(afi, field[:])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hops.Slice(), []netip.Addr{first, second}) {
		t.Fatalf("decoded=%v", hops.Slice())
	}
	if _, err := got.DecodeNextHops(afi, field[:40]); err == nil {
		t.Fatal("40-octet RD pair accepted")
	}
	unknown := MPNextHopProfile(afi, 240)
	if unknown.Lengths != nil {
		t.Fatalf("unknown lengths=%v", unknown.Lengths)
	}
	hops, err = unknown.DecodeNextHops(afi, []byte{192, 0, 2, 1})
	if err != nil {
		t.Fatal(err)
	}
	if hops.addrs[0] != netip.MustParseAddr("192.0.2.1") {
		t.Fatalf("unknown native IPv4=%v", hops.Slice())
	}
}

// TestMPNextHopProfileInputOwnership mutates registration input, never the
// documented shared read-only lookup slice, and checks the retained lengths.
func TestMPNextHopProfileInputOwnership(t *testing.T) {
	saved := maps.Clone(mpNextHopProfiles)
	t.Cleanup(func() { mpNextHopProfiles = saved })
	lengths := []int{4, 16, 32}
	if err := RegisterMPNextHopProfile(65001, 241, NextHopProfile{Lengths: lengths}); err != nil {
		t.Fatal(err)
	}
	lengths[0] = 99
	if got := ValidNextHopLens(65001, 241); !slices.Equal(got, []int{4, 16, 32}) {
		t.Fatalf("registered lengths alias input: %v", got)
	}
}

// TestMPNextHopMappingIntent distinguishes mapped permission from conversion,
// preserving direct codec inputs and explicit mapping for unknown profiles.
func TestMPNextHopMappingIntent(t *testing.T) {
	saved := maps.Clone(mpNextHopProfiles)
	t.Cleanup(func() { mpNextHopProfiles = saved })
	if err := RegisterMPNextHopProfile(65002, 241, NextHopProfile{
		Lengths: []int{4, 16, 32}, MappedIPv4: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := RegisterMPNextHopProfile(65002, 242, NextHopProfile{
		Lengths: []int{24, 48}, RDPerAddress: true, MappedIPv4: true, EncodeIPv4Mapped: true,
	}); err != nil {
		t.Fatal(err)
	}
	native := netip.MustParseAddr("192.0.2.1")
	mapped := netip.MustParseAddr("::ffff:192.0.2.1")
	cases := []struct {
		name    string
		afi     AFI
		safi    SAFI
		request bool
		input   netip.Addr
		want    netip.Addr
		width   int
	}{
		{"permission only", 65002, 241, false, native, native, 4},
		{"forced RD mapping", 65002, 242, false, native, mapped, 24},
		{"unknown explicit mapping", 65002, 240, true, native, mapped, 16},
		{"supplied mapped", 65002, 240, false, mapped, mapped, 16},
		{"VPNv6 default", AFIIPv6, SAFIVPN, false, native, mapped, 24},
		{"IPv6 unicast direct", AFIIPv6, SAFIUnicast, false, native, native, 4},
		{"IPv6 multicast direct", AFIIPv6, SAFIMulticast, false, native, native, 4},
		{"IPv6 labeled direct", AFIIPv6, SAFIMPLSLabel, false, native, native, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := MPNextHopProfile(tc.afi, tc.safi)
			selected := profile.SelectAddress(tc.input, tc.request)
			if selected != tc.want {
				t.Fatalf("selected=%v, want %v", selected, tc.want)
			}
			if got := profile.NextHopLen(selected, netip.Addr{}); got != tc.width {
				t.Fatalf("width=%d, want %d", got, tc.width)
			}
			var out [50]byte
			for i := range out {
				out[i] = 0xaa
			}
			if got := profile.WriteNextHops(out[:], 1, selected, netip.Addr{}); got != tc.width {
				t.Fatalf("write=%d, want %d", got, tc.width)
			}
			start := 1
			if profile.RDPerAddress {
				if !bytes.Equal(out[1:9], make([]byte, 8)) {
					t.Fatalf("RD=%x", out[1:9])
				}
				start += 8
			}
			if !bytes.Equal(out[start:1+tc.width], tc.want.AsSlice()) {
				t.Fatalf("address=%x, want %x", out[start:1+tc.width], tc.want.AsSlice())
			}
			if out[0] != 0xaa {
				t.Fatal("writer changed leading canary")
			}
			if out[1+tc.width] != 0xaa {
				t.Fatal("writer changed trailing canary")
			}
			attr := NewMPReachNLRI(tc.afi, tc.safi, []netip.Addr{tc.input}, nil)
			if !tc.request {
				var value [64]byte
				attr.WriteTo(value[:], 0)
				if int(value[3]) != tc.width {
					t.Fatalf("direct codec width=%d, want %d", value[3], tc.width)
				}
			}
		})
	}
}

// TestMPNextHopProfileHotPath checks that lookup, selection, sizing, writing
// and inline decoding allocate no per-field storage for either layout.
func TestMPNextHopProfileHotPath(t *testing.T) {
	saved := maps.Clone(mpNextHopProfiles)
	t.Cleanup(func() { mpNextHopProfiles = saved })
	if err := RegisterMPNextHopProfile(65003, 241, NextHopProfile{
		Lengths: []int{12, 24, 48}, RDPerAddress: true,
	}); err != nil {
		t.Fatal(err)
	}
	nextHop := netip.MustParseAddr("2001:db8::1")
	linkLocal := netip.MustParseAddr("fe80::2")
	for _, key := range []struct {
		afi  AFI
		safi SAFI
	}{
		{AFIIPv6, SAFIUnicast}, {AFIIPv6, SAFIVPN}, {65003, 241},
	} {
		var field [48]byte
		var decoded NextHopAddrs
		var decodeErr error
		allocs := testing.AllocsPerRun(100, func() {
			profile := MPNextHopProfile(key.afi, key.safi)
			selected := profile.SelectAddress(nextHop, false)
			length := profile.NextHopLen(selected, linkLocal)
			profile.WriteNextHops(field[:], 0, selected, linkLocal)
			decoded, decodeErr = profile.DecodeNextHops(key.afi, field[:length])
		})
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if decoded.Len() != 2 {
			t.Fatalf("decoded count=%d", decoded.Len())
		}
		if allocs != 0 {
			t.Fatalf("AFI/SAFI %d/%d allocations=%v", key.afi, key.safi, allocs)
		}
	}
}

// TestMPNextHopInlineCount poisons unused inline slots through the public
// slice's capacity. Only addresses inside its count may affect sizing or wire
// output, including the reserved octet, NLRI and surrounding canaries.
func TestMPNextHopInlineCount(t *testing.T) {
	t.Parallel()
	for _, count := range []int{0, 1} {
		hops := []netip.Addr{netip.MustParseAddr("192.0.2.1")}
		mp := NewMPReachNLRI(AFIIPv4, SAFIUnicast, hops[:count], []byte{24, 203, 0, 113})
		unused := mp.NextHops.Slice()[:2]
		for i := count; i < len(unused); i++ {
			unused[i] = netip.MustParseAddr("2001:db8::bad")
		}
		wantField := []byte{192, 0, 2, 1}
		wantField = wantField[:count*4]
		wantLen := 5 + len(wantField) + len(mp.NLRI)
		if got := mp.Len(); got != wantLen {
			t.Fatalf("count=%d length=%d, want %d", count, got, wantLen)
		}
		buf := bytes.Repeat([]byte{0xa5}, 4+wantLen+48)
		if got := mp.WriteTo(buf, 4); got != wantLen {
			t.Fatalf("count=%d written=%d, want %d", count, got, wantLen)
		}
		value := buf[4 : 4+wantLen]
		if int(value[3]) != len(wantField) {
			t.Fatalf("count=%d field length=%d, want %d", count, value[3], len(wantField))
		}
		if !bytes.Equal(value[4:4+len(wantField)], wantField) {
			t.Fatalf("count=%d next hop=%x, want %x", count, value[4:4+len(wantField)], wantField)
		}
		if value[4+len(wantField)] != 0 {
			t.Fatalf("count=%d reserved=%x", count, value[4+len(wantField)])
		}
		if !bytes.Equal(value[5+len(wantField):], mp.NLRI) {
			t.Fatalf("count=%d NLRI=%x, want %x", count, value[5+len(wantField):], mp.NLRI)
		}
		if !bytes.Equal(buf[:4], bytes.Repeat([]byte{0xa5}, 4)) {
			t.Fatalf("count=%d leading canary changed", count)
		}
		if !bytes.Equal(buf[4+wantLen:], bytes.Repeat([]byte{0xa5}, 48)) {
			t.Fatalf("count=%d trailing canary changed", count)
		}
	}
}

// TestMPNextHopUnknownIPv6ProfileNativeIPv4 preserves generic native IPv4
// decoding without admitting it for known IPv6-only field contracts.
func TestMPNextHopUnknownIPv6ProfileNativeIPv4(t *testing.T) {
	for _, tc := range []struct {
		name string
		safi SAFI
		want bool
	}{
		{"unknown", 250, true},
		{"mup", 85, true},
		{"unicast", SAFIUnicast, false},
		{"multicast", SAFIMulticast, false},
		{"labeled", SAFIMPLSLabel, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := []byte{0xc0, 0x00, 0x02, 0x01}
			before := bytes.Clone(field)
			profile := MPNextHopProfile(AFIIPv6, tc.safi)
			if tc.want && len(profile.Lengths) != 0 {
				t.Fatalf("generic control has declared lengths: %v", profile.Lengths)
			}
			hops, err := profile.DecodeNextHops(AFIIPv6, field)
			if tc.want {
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(hops.Slice(), []netip.Addr{netip.MustParseAddr("192.0.2.1")}) {
					t.Fatalf("next hops=%v, want native 192.0.2.1", hops.Slice())
				}
			} else {
				if err != ErrInvalidNextHopLen {
					t.Fatalf("error=%v, want %v", err, ErrInvalidNextHopLen)
				}
				if hops.Len() != 0 {
					t.Fatalf("refused field retained next hops: %v", hops.Slice())
				}
			}
			if !bytes.Equal(field, before) {
				t.Fatal("decoding changed the received field")
			}
		})
	}
}
