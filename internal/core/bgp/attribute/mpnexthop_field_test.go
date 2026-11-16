// Design: docs/architecture/wire/attributes.md -- complete fields and codec address lists
// Related: mpnexthop.go -- one shared RD layout decoder for both contracts

package attribute

import (
	"bytes"
	"net/netip"
	"slices"
	"testing"
)

// TestMPNextHopCompleteField keeps the unknown six-width field contract for
// both AFIs, then distinguishes declared plain/RD framing and VPN codec forms.
func TestMPNextHopCompleteField(t *testing.T) {
	v4 := netip.MustParseAddr("192.0.2.9")
	v6 := netip.MustParseAddr("::ffff:192.0.2.9")
	global := netip.MustParseAddr("2001:db8::9")
	ll := netip.MustParseAddr("fe80::9")
	rd12 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 192, 0, 2, 9}
	rd24 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 0, 2, 9}
	pair := []byte{0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}
	rd48 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}
	for _, afi := range []AFI{AFIIPv4, AFIIPv6} {
		profile := MPNextHopProfile(afi, 250)
		for _, tc := range []struct {
			field []byte
			want  []netip.Addr
		}{
			{[]byte{192, 0, 2, 9}, []netip.Addr{v4}},
			{rd12, []netip.Addr{v4}},
			{rd24[8:], []netip.Addr{v6}},
			{rd24, []netip.Addr{v6}},
			{pair, []netip.Addr{global, ll}},
			{rd48, []netip.Addr{global, ll}},
		} {
			original := bytes.Clone(tc.field)
			got, err := profile.DecodeNextHopField(afi, tc.field)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.Slice(), tc.want) {
				t.Fatalf("AFI %d field %x: got %v, want %v", afi, tc.field, got.Slice(), tc.want)
			}
			if !bytes.Equal(tc.field, original) {
				t.Fatal("decoder changed borrowed bytes")
			}
		}
	}
	plain := MPNextHopProfile(AFIIPv4, SAFIUnicast)
	if _, err := plain.DecodeNextHopField(AFIIPv4, rd12); err == nil {
		t.Fatal("plain field decoded RD bytes as an IPv4 address list")
	}
	strictRD := NextHopProfile{Lengths: []int{12, 24, 48}, RDPerAddress: true}
	if _, err := strictRD.DecodeNextHopField(AFIIPv4, pair[:16]); err == nil {
		t.Fatal("RD profile acquired unprefixed IPv6 compatibility")
	}
	legacy := MPNextHopProfile(AFIIPv6, SAFIVPN)
	got, err := legacy.DecodeNextHopField(AFIIPv6, pair)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Slice(), []netip.Addr{global, ll}) {
		t.Fatalf("legacy unprefixed VPN field=%v", got.Slice())
	}
	for _, invalid := range []int{1, 8, 40, 49} {
		if _, err := (NextHopProfile{}).DecodeNextHopField(AFIIPv4, make([]byte, invalid)); err == nil {
			t.Fatalf("complete field accepted width %d", invalid)
		}
	}
}

// TestMPNextHopFieldAndAddressList preserves the distinct generic codec
// interpretation while the same literal RD fields classify by complete width.
func TestMPNextHopFieldAndAddressList(t *testing.T) {
	field := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}
	profile := NextHopProfile{}
	for _, afi := range []AFI{AFIIPv4, AFIIPv6} {
		got, err := profile.DecodeNextHopField(afi, field)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.Slice(), []netip.Addr{netip.MustParseAddr("2001:db8::9")}) {
			t.Fatalf("complete field AFI %d=%v", afi, got.Slice())
		}
	}
	list, err := profile.DecodeNextHops(AFIIPv4, field)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(list.Slice(), []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("5.6.7.8")}) {
		t.Fatalf("generic IPv4 address list=%v", list.Slice())
	}
	if _, err := profile.DecodeNextHops(AFIIPv6, field); err == nil {
		t.Fatal("generic IPv6 address list accepted a 24-octet field")
	}
}

// TestMPNextHopFieldAdmission checks generic and declared widths separately
// from decoding, which must retain codec-only VPN compatibility.
func TestMPNextHopFieldAdmission(t *testing.T) {
	for _, tc := range []struct {
		profile NextHopProfile
		want    []int
	}{
		{NextHopProfile{}, []int{4, 12, 16, 24, 32, 48}},
		{MPNextHopProfile(AFIIPv6, SAFIUnicast), []int{16, 32}},
		{MPNextHopProfile(AFIIPv6, SAFIVPN), []int{24, 48}},
	} {
		for width := range 50 {
			if got := tc.profile.AdmitsNextHopOctets(width); got != slices.Contains(tc.want, width) {
				t.Fatalf("lengths %v width %d: admitted=%v", tc.profile.Lengths, width, got)
			}
		}
	}
}

// TestMPNextHopCompleteFieldHotPath checks inline storage on the new field
// contract for unknown and declared RD profiles without per-field allocation.
func TestMPNextHopCompleteFieldHotPath(t *testing.T) {
	field := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9}
	for _, profile := range []NextHopProfile{{}, MPNextHopProfile(AFIIPv6, SAFIVPN)} {
		var decoded NextHopAddrs
		var decodeErr error
		allocs := testing.AllocsPerRun(100, func() {
			decoded, decodeErr = profile.DecodeNextHopField(AFIIPv6, field)
		})
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if !slices.Equal(decoded.Slice(), []netip.Addr{netip.MustParseAddr("2001:db8::9"), netip.MustParseAddr("fe80::9")}) {
			t.Fatalf("decoded=%v", decoded.Slice())
		}
		if allocs != 0 {
			t.Fatalf("field decoding allocations=%v", allocs)
		}
	}
}
