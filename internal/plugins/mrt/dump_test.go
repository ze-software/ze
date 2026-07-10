// VALIDATES: MRT wire-parameter selection — TABLE_DUMP_V2 subtype per AFI/
// add-path, local-message subtype mapping for sent messages, peer-IP parsing,
// and header-size / BGP4MP type-subtype derivation from config.
// PREVENTS: RIB entries or BGP4MP records being tagged with the wrong MRT
// subtype/type (which makes dumps unparseable by downstream MRT readers).
package mrt

import (
	"testing"

	mrtfmt "codeberg.org/thomas-mangin/ze/internal/mrt"
)

func TestRibSubtype(t *testing.T) {
	cases := []struct {
		afi     uint16
		addPath bool
		want    uint16
	}{
		{mrtfmt.AFIIPv4, false, mrtfmt.TDV2RIBIPv4Unicast},
		{mrtfmt.AFIIPv4, true, mrtfmt.TDV2RIBIPv4UnicastAP},
		{mrtfmt.AFIIPv6, false, mrtfmt.TDV2RIBIPv6Unicast},
		{mrtfmt.AFIIPv6, true, mrtfmt.TDV2RIBIPv6UnicastAP},
		{9999, false, mrtfmt.TDV2RIBGeneric},
	}
	for _, tc := range cases {
		if got := ribSubtype(tc.afi, tc.addPath); got != tc.want {
			t.Errorf("ribSubtype(afi=%d, addPath=%v) = %d, want %d", tc.afi, tc.addPath, got, tc.want)
		}
	}
}

func TestLocalSubtype(t *testing.T) {
	cases := map[uint16]uint16{
		mrtfmt.BGP4MPMessageAS4:   mrtfmt.BGP4MPMessageAS4Local,
		mrtfmt.BGP4MPMessageAS4AP: mrtfmt.BGP4MPMessageAS4LocalAP,
		mrtfmt.BGP4MPMessage:      mrtfmt.BGP4MPMessageLocal,
		mrtfmt.BGP4MPMessageAP:    mrtfmt.BGP4MPMessageLocalAP,
		0xBEEF:                    0xBEEF, // unknown subtype passes through unchanged
	}
	for in, want := range cases {
		if got := localSubtype(in); got != want {
			t.Errorf("localSubtype(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestParseIPIntoPeerIPv4(t *testing.T) {
	pe := mrtfmt.PeerEntry{IP: make([]byte, 4)}
	parseIPIntoPeer("192.168.1.5", &pe)
	if pe.IP[0] != 192 || pe.IP[1] != 168 || pe.IP[2] != 1 || pe.IP[3] != 5 {
		t.Errorf("IPv4 parse: got %v, want [192 168 1 5]", pe.IP)
	}
}

func TestParseIPIntoPeerIPv6(t *testing.T) {
	pe := mrtfmt.PeerEntry{IP: make([]byte, 16)}
	parseIPIntoPeer("2001:db8::1", &pe)
	if pe.IP[0] != 0x20 || pe.IP[1] != 0x01 || pe.IP[15] != 0x01 {
		t.Errorf("IPv6 parse: got %x, want 2001:db8::1", pe.IP)
	}
}

func TestParseIPIntoPeerInvalidLeavesZero(t *testing.T) {
	pe := mrtfmt.PeerEntry{IP: make([]byte, 4)}
	parseIPIntoPeer("not-an-ip", &pe)
	for _, b := range pe.IP {
		if b != 0 {
			t.Fatalf("invalid IP left non-zero bytes: %v", pe.IP)
		}
	}
}

func TestHeaderSize(t *testing.T) {
	base := (&Component{config: Config{ExtendedTimestamp: false}}).headerSize()
	ext := (&Component{config: Config{ExtendedTimestamp: true}}).headerSize()
	if base != mrtfmt.CommonHeaderLen {
		t.Errorf("headerSize(plain) = %d, want %d", base, mrtfmt.CommonHeaderLen)
	}
	if ext != mrtfmt.CommonHeaderLen+mrtfmt.ExtTimestampLen {
		t.Errorf("headerSize(ext) = %d, want %d", ext, mrtfmt.CommonHeaderLen+mrtfmt.ExtTimestampLen)
	}
}

func TestBGP4MPTypeSubtype(t *testing.T) {
	cases := []struct {
		extTS, addPath bool
		wantType       uint16
		wantSubtype    uint16
	}{
		{false, false, mrtfmt.TypeBGP4MP, mrtfmt.BGP4MPMessageAS4},
		{true, false, mrtfmt.TypeBGP4MPET, mrtfmt.BGP4MPMessageAS4},
		{false, true, mrtfmt.TypeBGP4MP, mrtfmt.BGP4MPMessageAS4AP},
		{true, true, mrtfmt.TypeBGP4MPET, mrtfmt.BGP4MPMessageAS4AP},
	}
	for _, tc := range cases {
		c := &Component{config: Config{ExtendedTimestamp: tc.extTS, AddPath: tc.addPath}}
		typ, sub := c.bgp4mpTypeSubtype()
		if typ != tc.wantType || sub != tc.wantSubtype {
			t.Errorf("bgp4mpTypeSubtype(extTS=%v,addPath=%v) = (%d,%d), want (%d,%d)",
				tc.extTS, tc.addPath, typ, sub, tc.wantType, tc.wantSubtype)
		}
	}
}
