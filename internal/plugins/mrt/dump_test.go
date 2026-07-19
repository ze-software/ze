// VALIDATES: MRT wire-parameter selection — TABLE_DUMP_V2 subtype per AFI/
// add-path, local-message subtype mapping for sent messages, peer-IP parsing,
// and header-size / BGP4MP type-subtype derivation from config.
// PREVENTS: RIB entries or BGP4MP records being tagged with the wrong MRT
// subtype/type (which makes dumps unparseable by downstream MRT readers).
package mrt

import (
	"path/filepath"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	mrtfmt "codeberg.org/thomas-mangin/ze/internal/mrt"
)

func TestRibSubtype(t *testing.T) {
	// RFC requirement: RFC6396-4.2-2 positive -- ze exports RIB snapshots as TABLE_DUMP_V2:
	// ribSubtype maps every AFI/add-path combination to a TABLE_DUMP_V2 RIB subtype
	// (TDV2RIB*, internal/plugins/mrt/dump.go:206-219), the dump path always stamps the
	// records TypeTableDumpV2 (dump.go:171,200), and peers are always written as 4-byte-AS
	// entries (PeerAS4, dump.go:133-135). This asserts the V2 subtype is selected for IPv4
	// and IPv6 (differing AFI) with and without add-path, so ze uses TABLE_DUMP_V2 rather
	// than the legacy TABLE_DUMP.
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

// fakeRIBDumper drives Component.writeTableDumpV2 with a scripted set of
// OnPeer/OnRoute callbacks, standing in for the real RIB manager bridge.
type fakeRIBDumper struct {
	fn func(registry.RIBDumpVisitor)
}

func (f fakeRIBDumper) DumpRIB(v registry.RIBDumpVisitor) { f.fn(v) }

// runRIBDumpV2 exercises the real TABLE_DUMP_V2 dump path (writeTableDumpV2) with a fake
// RIB source, writing records to a temp file, then reads them back in file order.
// It returns the per-record headers (in wire order) plus any decoded PEER_INDEX_TABLE
// and RIB records.
func runRIBDumpV2(t *testing.T, dump func(registry.RIBDumpVisitor)) ([]mrtfmt.Header, *mrtfmt.PeerIndexTable, []*mrtfmt.RIBRecord) {
	t.Helper()

	c := New(Config{}, nil)
	path := filepath.Join(t.TempDir(), "rib.mrt")
	c.routes = mrtfmt.NewWriter(path)
	c.ribDumper = fakeRIBDumper{fn: dump}

	c.writeTableDumpV2()
	if err := c.routes.Close(); err != nil {
		t.Fatalf("close routes writer: %v", err)
	}

	var order []mrtfmt.Header
	var pit *mrtfmt.PeerIndexTable
	var ribs []*mrtfmt.RIBRecord
	h := &mrtfmt.Handler{
		OnHeader: func(hd mrtfmt.Header, _ uint32, _ []byte) error {
			order = append(order, hd)
			return nil
		},
		OnPeerIndex: func(_ mrtfmt.Header, p *mrtfmt.PeerIndexTable) error {
			pit = p
			return nil
		},
		OnRIB: func(_ mrtfmt.Header, r *mrtfmt.RIBRecord) error {
			ribs = append(ribs, r)
			return nil
		},
	}
	if err := mrtfmt.ReadFile(path, h); err != nil {
		t.Fatalf("read back dump: %v", err)
	}
	return order, pit, ribs
}

// testWireASPath4 returns a single wire path-attribute blob containing only an AS_PATH
// (type 2) encoded with 4-byte ASNs: AS_SEQUENCE {65001, 200000}. 200000 exceeds 65535
// and so can only be represented in 4 bytes -- decoding it back to 200000 at 4-byte width
// proves the AS_PATH is 4-byte-encoded.
func testWireASPath4() []byte {
	return []byte{
		0x40, 0x02, 0x0a, // flags=0x40 (transitive), code=2 (AS_PATH), length=10
		0x02, 0x02, // segment type AS_SEQUENCE, count 2
		0x00, 0x00, 0xfd, 0xe9, // 65001
		0x00, 0x03, 0x0d, 0x40, // 200000
	}
}

// findASPath walks BGP path attributes and returns the AS_PATH (type 2) value bytes.
func findASPath(attrs []byte) ([]byte, bool) {
	off := 0
	for off+2 <= len(attrs) {
		flags := attrs[off]
		code := attrs[off+1]
		off += 2
		var vlen int
		if flags&0x10 != 0 { // extended length
			if off+2 > len(attrs) {
				return nil, false
			}
			vlen = int(attrs[off])<<8 | int(attrs[off+1])
			off += 2
		} else {
			if off+1 > len(attrs) {
				return nil, false
			}
			vlen = int(attrs[off])
			off++
		}
		if off+vlen > len(attrs) {
			return nil, false
		}
		if code == 2 {
			return attrs[off : off+vlen], true
		}
		off += vlen
	}
	return nil, false
}

// parse4ByteASPath decodes AS_PATH segment value bytes as 4-byte ASNs. It returns false
// unless the segments consume the value exactly at 4 bytes per ASN, which is what
// distinguishes a 4-byte-encoded AS_PATH from a 2-byte one (a non-empty 2-byte segment
// can never parse cleanly at 4-byte width).
func parse4ByteASPath(value []byte) ([]uint32, bool) {
	var asns []uint32
	off := 0
	for off+2 <= len(value) {
		count := int(value[off+1])
		off += 2
		if off+count*4 > len(value) {
			return nil, false
		}
		for range count {
			asns = append(asns, uint32(value[off])<<24|uint32(value[off+1])<<16|
				uint32(value[off+2])<<8|uint32(value[off+3]))
			off += 4
		}
	}
	if off != len(value) {
		return nil, false
	}
	return asns, true
}

func TestDumpV2PeerIndexBeforeFirstRIBEntry(t *testing.T) {
	// RFC requirement: RFC6396-4.3.1-3 positive -- in a TABLE_DUMP_V2 export the
	// PEER_INDEX_TABLE record is written before the first RIB entry record. The dump path
	// writes the PEER_INDEX_TABLE lazily on the first OnRoute callback, ahead of that
	// route's RIB entry (internal/plugins/mrt/dump.go:149-152,169-173). This drives the
	// real writeTableDumpV2 path and asserts the first record on the wire is the
	// PEER_INDEX_TABLE and the next is the RIB entry.
	order, pit, ribs := runRIBDumpV2(t, func(v registry.RIBDumpVisitor) {
		idx := v.OnPeer("192.0.2.1", 65001, [4]byte{1, 2, 3, 4}, false)
		v.OnRoute(idx, mrtfmt.AFIIPv4, 1, 24, []byte{203, 0, 113}, testWireASPath4())
	})

	if len(order) < 2 {
		t.Fatalf("record count = %d, want at least 2 (PEER_INDEX_TABLE + RIB entry)", len(order))
	}
	if order[0].Type != mrtfmt.TypeTableDumpV2 || order[0].Subtype != mrtfmt.TDV2PeerIndexTable {
		t.Fatalf("first record = (type %d, subtype %d), want PEER_INDEX_TABLE (%d,%d)",
			order[0].Type, order[0].Subtype, mrtfmt.TypeTableDumpV2, mrtfmt.TDV2PeerIndexTable)
	}
	if order[1].Type != mrtfmt.TypeTableDumpV2 || order[1].Subtype != mrtfmt.TDV2RIBIPv4Unicast {
		t.Fatalf("second record = (type %d, subtype %d), want RIB_IPV4_UNICAST (%d,%d)",
			order[1].Type, order[1].Subtype, mrtfmt.TypeTableDumpV2, mrtfmt.TDV2RIBIPv4Unicast)
	}
	if pit == nil {
		t.Fatal("no PEER_INDEX_TABLE decoded before the RIB entries")
	}
	if len(ribs) != 1 {
		t.Fatalf("RIB record count = %d, want 1", len(ribs))
	}
}

func TestDumpV2RIBEntryASPathIs4Byte(t *testing.T) {
	// RFC requirement: RFC6396-4.3.4-1 positive -- a produced TABLE_DUMP_V2 RIB entry
	// carries a 4-byte-ASN AS_PATH. The dump path copies the RIB-supplied path attributes
	// into the RIB entry verbatim (WriteRIBEntry, internal/mrt/encode.go:88-98), and the ze
	// RIB producer always supplies a 4-byte AS_PATH: canonicalizeASPath returns the 4-byte
	// AS4_PATH or expands a 2-byte AS_PATH to 4-byte
	// (internal/component/bgp/plugins/rib/storage/attrparse.go:198-208), reconstructed into
	// the attribute blob by rib_mrt.go:128-130. This drives writeTableDumpV2 with a 4-byte
	// AS_PATH and asserts the decoded RIB entry's AS_PATH is 4-byte (a >65535 ASN, which
	// cannot be 2-byte, round-trips intact).
	_, _, ribs := runRIBDumpV2(t, func(v registry.RIBDumpVisitor) {
		idx := v.OnPeer("192.0.2.1", 65001, [4]byte{1, 2, 3, 4}, false)
		v.OnRoute(idx, mrtfmt.AFIIPv4, 1, 24, []byte{203, 0, 113}, testWireASPath4())
	})

	if len(ribs) != 1 || len(ribs[0].Entries) != 1 {
		t.Fatalf("want 1 RIB record with 1 entry, got %d records", len(ribs))
	}
	asPath, ok := findASPath(ribs[0].Entries[0].Attributes)
	if !ok {
		t.Fatal("RIB entry has no AS_PATH attribute")
	}
	asns, ok := parse4ByteASPath(asPath)
	if !ok {
		t.Fatalf("AS_PATH %x is not 4-byte-ASN encoded", asPath)
	}
	want := []uint32{65001, 200000}
	if len(asns) != len(want) {
		t.Fatalf("AS_PATH ASNs = %v, want %v", asns, want)
	}
	for i := range want {
		if asns[i] != want[i] {
			t.Errorf("AS_PATH[%d] = %d, want %d", i, asns[i], want[i])
		}
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
