// Design: docs/architecture/mrt.md — TABLE_DUMP_V2 RIB snapshots.

package mrt

import (
	"bytes"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// TestDumpV2PeerWidthAndPrefixFamily checks actual file records for both peer
// address families, both prefix families, and ordinary and four-byte peer ASNs.
// MUTATION: Stamp TypeTableDump instead of TypeTableDumpV2 in the dump producer;
// the common-header assertion must fail before the reader dispatches the body.
func TestDumpV2PeerWidthAndPrefixFamily(t *testing.T) {
	// RFC requirement: RFC6396-4.2-2 positive -- actual RIB dump files use
	// TABLE_DUMP_V2 for four-byte Peer AS numbers and both directions of a
	// peer/prefix AFI mismatch, as well as same-family ordinary-AS controls.
	// The decoded PIT retains the exact ASN/address and the RIB retains the
	// exact prefix, peer index, and attributes supplied to the dump producer.
	peers := []struct {
		name string
		addr netip.Addr
	}{
		{"peer4", netip.MustParseAddr("192.0.2.9")},
		{"peer6", netip.MustParseAddr("2001:db8::9")},
	}
	prefixes := []struct {
		name    string
		afi     uint16
		bits    uint8
		wire    []byte
		subtype uint16
	}{
		{"prefix4", mrtfmt.AFIIPv4, 24, []byte{203, 0, 113}, mrtfmt.TDV2RIBIPv4Unicast},
		{"prefix6", mrtfmt.AFIIPv6, 48, []byte{0x20, 0x01, 0x0d, 0xb8, 0x12, 0x34}, mrtfmt.TDV2RIBIPv6Unicast},
	}
	asns := []struct {
		name string
		asn  uint32
	}{
		{"ordinary-as", 65001},
		{"four-byte-as", 0x01020304},
	}
	for _, peer := range peers {
		for _, prefix := range prefixes {
			for _, asn := range asns {
				t.Run(peer.name+"/"+prefix.name+"/"+asn.name, func(t *testing.T) {
					attrs := testWireASPath4()
					bgpID := [4]byte{198, 51, 100, 9}
					c := New(Config{}, nil)
					path := filepath.Join(t.TempDir(), "rib.mrt")
					c.routes = mrtfmt.NewWriter(path)
					c.ribDumper = fakeRIBDumper{fn: func(v registry.RIBDumpVisitor) {
						index := v.OnPeer(peer.addr.String(), asn.asn, bgpID, peer.addr.Is6())
						v.OnRoute(index, prefix.afi, 1, prefix.bits, prefix.wire, attrs)
					}}
					// RFC 6396 Section 4.2: "The TABLE_DUMP_V2 Type MUST be used in these situations."
					c.writeTableDumpV2()
					if err := c.routes.Close(); err != nil {
						t.Fatalf("close dump: %v", err)
					}

					var headers, pits, ribs int
					handler := &mrtfmt.Handler{
						OnHeader: func(h mrtfmt.Header, _ uint32, _ []byte) error {
							if h.Type != mrtfmt.TypeTableDumpV2 {
								t.Fatalf("record %d type = %d, want TABLE_DUMP_V2 (%d)", headers, h.Type, mrtfmt.TypeTableDumpV2)
							}
							wantSubtype := prefix.subtype
							if headers == 0 {
								wantSubtype = mrtfmt.TDV2PeerIndexTable
							}
							if h.Subtype != wantSubtype {
								t.Fatalf("record %d subtype = %d, want %d", headers, h.Subtype, wantSubtype)
							}
							headers++
							return nil
						},
						OnPeerIndex: func(_ mrtfmt.Header, pit *mrtfmt.PeerIndexTable) error {
							pits++
							if len(pit.Peers) != 1 {
								t.Fatalf("PIT peer count = %d, want 1", len(pit.Peers))
							}
							got := pit.Peers[0]
							wantType := mrtfmt.PeerAS4
							if peer.addr.Is6() {
								wantType |= mrtfmt.PeerIPv6
							}
							if got.Type != wantType {
								t.Errorf("PIT peer type = %d, want %d", got.Type, wantType)
							}
							if got.ASN != asn.asn {
								t.Errorf("PIT peer ASN = %d, want %d", got.ASN, asn.asn)
							}
							if !bytes.Equal(got.IP, peer.addr.AsSlice()) {
								t.Errorf("PIT peer address = %x, want %x", got.IP, peer.addr.AsSlice())
							}
							if got.BGPID != bgpID {
								t.Errorf("PIT BGP ID = %v, want %v", got.BGPID, bgpID)
							}
							return nil
						},
						OnRIB: func(_ mrtfmt.Header, rib *mrtfmt.RIBRecord) error {
							ribs++
							if rib.SequenceNumber != 1 {
								t.Errorf("RIB sequence = %d, want 1", rib.SequenceNumber)
							}
							if rib.PrefixLength != prefix.bits {
								t.Errorf("RIB prefix length = %d, want %d", rib.PrefixLength, prefix.bits)
							}
							if !bytes.Equal(rib.Prefix, prefix.wire) {
								t.Errorf("RIB prefix = %x, want %x", rib.Prefix, prefix.wire)
							}
							if len(rib.Entries) != 1 {
								t.Fatalf("RIB entry count = %d, want 1", len(rib.Entries))
							}
							if rib.Entries[0].PeerIndex != 0 {
								t.Errorf("RIB peer index = %d, want 0", rib.Entries[0].PeerIndex)
							}
							if !bytes.Equal(rib.Entries[0].Attributes, attrs) {
								t.Errorf("RIB attributes = %x, want %x", rib.Entries[0].Attributes, attrs)
							}
							return nil
						},
					}
					// RFC 6396 Sections 2, 4.3.1, and 4.3.2: Decode the producer's file.
					if err := mrtfmt.ReadFile(path, handler); err != nil {
						t.Fatalf("read dump: %v", err)
					}
					if headers != 2 {
						t.Errorf("record count = %d, want 2", headers)
					}
					if pits != 1 {
						t.Errorf("PIT count = %d, want 1", pits)
					}
					if ribs != 1 {
						t.Errorf("RIB count = %d, want 1", ribs)
					}
				})
			}
		}
	}
}
