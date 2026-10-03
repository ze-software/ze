// Design: docs/architecture/mrt.md — independent RFC 6396 byte expectations.
package mrt_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// TestRFC6396NumericFieldsUseExternalNetworkOrder compares encoder output to
// literal octets and decodes those independent octets, not the encoder's output.
// RFC requirement: RFC6396-1-1 positive -- PEER_INDEX_TABLE ViewNameLength, PeerCount and both PeerAS widths; RIB SeqNumber, EntryCount, PeerIndex, OriginatedTime and AttributeLength; and BGP4MP PeerAS, LocalAS, InterfaceIndex and AFI have exact MSB-first octets and independently decode to the specified numbers.
func TestRFC6396NumericFieldsUseExternalNetworkOrder(t *testing.T) {
	peers := make([]mrt.PeerEntry, 0x0102)
	for i := range peers {
		peers[i] = mrt.PeerEntry{IP: []byte{192, 0, 2, 1}, ASN: 0x1234}
	}
	peers[0].Type = mrt.PeerAS4
	peers[0].ASN = 0x12345678
	name := strings.Repeat("a", 0x0102)
	pitWire := []byte{192, 0, 2, 9, 1, 2}
	pitWire = append(pitWire, name...)
	pitWire = append(pitWire, 1, 2)
	pitWire = append(pitWire, 2, 0, 0, 0, 0, 192, 0, 2, 1, 0x12, 0x34, 0x56, 0x78)
	for range 0x0101 {
		pitWire = append(pitWire, 0, 0, 0, 0, 0, 192, 0, 2, 1, 0x12, 0x34)
	}
	buf := make([]byte, len(pitWire))
	n, err := mrt.WritePeerIndexTable(buf, 0, [4]byte{192, 0, 2, 9}, name, peers)
	if err != nil || n != len(pitWire) || !bytes.Equal(buf, pitWire) {
		t.Fatalf("PEER_INDEX_TABLE differs from external octets: count=%d err=%v", n, err)
	}
	pit, err := mrt.DecodePeerIndexTable(pitWire)
	if err != nil {
		t.Fatal(err)
	}
	if pit.ViewName != name || len(pit.Peers) != 0x0102 || pit.Peers[0].ASN != 0x12345678 || pit.Peers[1].ASN != 0x1234 {
		t.Fatalf("external PEER_INDEX_TABLE decoded incorrectly: %+v", pit)
	}

	entries := make([]mrt.RIBEntry, 0x0102)
	attrs := bytes.Repeat([]byte{0x42}, 0x0102)
	ribWire := []byte{0x12, 0x34, 0x56, 0x78, 24, 10, 20, 30, 1, 2}
	for i := range entries {
		entries[i] = mrt.RIBEntry{PeerIndex: 0x1234, OrigTime: 0x23456789, Attributes: attrs}
		ribWire = append(ribWire, 0x12, 0x34, 0x23, 0x45, 0x67, 0x89, 1, 2)
		ribWire = append(ribWire, attrs...)
	}
	buf = make([]byte, len(ribWire))
	n = mrt.WriteRIBHeader(buf, 0, 0x12345678, 24, []byte{10, 20, 30})
	n += mrt.WriteRIBEntries(buf, n, entries, false)
	if n != len(ribWire) || !bytes.Equal(buf, ribWire) {
		t.Fatal("RIB fields differ from external network-order octets")
	}
	rib, err := mrt.DecodeRIBRecord(mrt.TDV2RIBIPv4Unicast, ribWire)
	if err != nil {
		t.Fatal(err)
	}
	if rib.SequenceNumber != 0x12345678 || len(rib.Entries) != 0x0102 {
		t.Fatalf("RIB sequence/count=%x/%d", rib.SequenceNumber, len(rib.Entries))
	}
	for _, entry := range rib.Entries {
		if entry.PeerIndex != 0x1234 || entry.OrigTime != 0x23456789 || !bytes.Equal(entry.Attributes, attrs) {
			t.Fatalf("external RIB entry=%+v", entry)
		}
	}

	for _, as4 := range []bool{false, true} {
		hdr := mrt.BGP4MPHeader{PeerAS: 0x1234, LocalAS: 0x5678, IfIndex: 0x2345, AFI: 1, PeerIP: []byte{192, 0, 2, 1}, LocalIP: []byte{192, 0, 2, 2}}
		fields := []byte{0x12, 0x34, 0x56, 0x78}
		subtype := mrt.BGP4MPMessage
		if as4 {
			hdr.PeerAS, hdr.LocalAS = 0x12345678, 0x3456789a
			fields = []byte{0x12, 0x34, 0x56, 0x78, 0x34, 0x56, 0x78, 0x9a}
			subtype = mrt.BGP4MPMessageAS4
		}
		fields = append(fields, 0x23, 0x45, 0, 1, 192, 0, 2, 1, 192, 0, 2, 2)
		wire := append(fields, buildBGPMessage(4, nil)...)
		buf = make([]byte, len(wire))
		n = mrt.WriteBGP4MPMessage(buf, 0, &hdr, as4, buildBGPMessage(4, nil))
		if n != len(wire) || !bytes.Equal(buf, wire) {
			t.Fatalf("BGP4MP as4=%v wire=%x want=%x", as4, buf, wire)
		}
		record, err := mrt.DecodeBGP4MPMessage(subtype, wire)
		if err != nil {
			t.Fatal(err)
		}
		if record.PeerAS != hdr.PeerAS || record.LocalAS != hdr.LocalAS || record.IfIndex != 0x2345 || record.AFI != 1 {
			t.Fatalf("external BGP4MP fields=%+v", record.BGP4MPHeader)
		}
	}
}

// TestRFC6396ViewNameUTF8 validates nonempty names at both wire boundaries and
// proves refusal leaves the output untouched rather than replacing bad bytes.
// RFC requirement: RFC6396-4.3.1-2 positive -- nonempty ASCII and multibyte UTF-8 names are written byte-for-byte and decoded from literal PEER_INDEX_TABLE fields, with the octet length rather than rune count.
// RFC requirement: RFC6396-4.3.1-2 negative -- invalid leading bytes, overlong sequences, surrogate encodings, out-of-range code points and truncated UTF-8 are refused by encoder and decoder; the encoder writes no bytes.
func TestRFC6396ViewNameUTF8(t *testing.T) {
	for _, name := range []string{"view", "r\u00e9seau-\u6771\u4eac"} {
		wire := append([]byte{0, 0, 0, 0, 0, byte(len(name))}, name...)
		wire = append(wire, 0, 0)
		buf := make([]byte, len(wire))
		n, err := mrt.WritePeerIndexTable(buf, 0, [4]byte{}, name, nil)
		if err != nil || n != len(wire) || !bytes.Equal(buf, wire) {
			t.Fatalf("valid UTF-8 %q: bytes=%x count=%d err=%v", name, buf, n, err)
		}
		pit, err := mrt.DecodePeerIndexTable(wire)
		if err != nil || pit.ViewName != name {
			t.Fatalf("literal UTF-8 decode: %+v %v", pit, err)
		}
	}
	for _, invalid := range [][]byte{{0xff}, {0xc0, 0x80}, {0xed, 0xa0, 0x80}, {0xf4, 0x90, 0x80, 0x80}, {0xe2, 0x82}} {
		buf := bytes.Repeat([]byte{0xa5}, 64)
		n, err := mrt.WritePeerIndexTable(buf, 3, [4]byte{}, string(invalid), nil)
		if err == nil || n != 0 || !bytes.Equal(buf, bytes.Repeat([]byte{0xa5}, 64)) {
			t.Fatalf("invalid name %x emitted: count=%d err=%v bytes=%x", invalid, n, err, buf)
		}
		wire := append([]byte{0, 0, 0, 0, 0, byte(len(invalid))}, invalid...)
		wire = append(wire, 0, 0)
		if _, err := mrt.DecodePeerIndexTable(wire); err == nil {
			t.Fatalf("decoder accepted invalid UTF-8 %x", invalid)
		}
	}
	buf := bytes.Repeat([]byte{0xa5}, 8)
	if n, err := mrt.WritePeerIndexTable(buf, 0, [4]byte{}, strings.Repeat("a", 65536), nil); n != 0 || err == nil {
		t.Fatalf("overlength view name: n=%d err=%v", n, err)
	}
}
