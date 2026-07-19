package mrt_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/mrt"
)

func TestCommonHeaderRoundTrip(t *testing.T) {
	ts := uint32(1700000000)
	typ := mrt.TypeBGP4MP
	sub := mrt.BGP4MPMessageAS4
	msgLen := uint32(82)

	buf := make([]byte, 4096)
	n := mrt.WriteCommonHeader(buf, 0, ts, typ, sub, msgLen)
	if n != mrt.CommonHeaderLen {
		t.Fatalf("WriteCommonHeader returned %d, want %d", n, mrt.CommonHeaderLen)
	}

	h, err := mrt.DecodeHeader(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if h.Timestamp != ts {
		t.Errorf("Timestamp = %d, want %d", h.Timestamp, ts)
	}
	if h.Type != typ {
		t.Errorf("Type = %d, want %d", h.Type, typ)
	}
	if h.Subtype != sub {
		t.Errorf("Subtype = %d, want %d", h.Subtype, sub)
	}
	if h.Length != msgLen {
		t.Errorf("Length = %d, want %d", h.Length, msgLen)
	}
}

func TestExtendedHeaderRoundTrip(t *testing.T) {
	ts := uint32(1700000000)
	usec := uint32(123456)
	typ := mrt.TypeBGP4MPET
	sub := mrt.BGP4MPMessageAS4
	bodyLen := uint32(90)

	buf := make([]byte, 4096)
	n := mrt.WriteExtendedHeader(buf, 0, ts, usec, typ, sub, bodyLen)
	if n != mrt.CommonHeaderLen+mrt.ExtTimestampLen {
		t.Fatalf("WriteExtendedHeader returned %d, want %d", n, mrt.CommonHeaderLen+mrt.ExtTimestampLen)
	}

	h, err := mrt.DecodeHeader(buf[:mrt.CommonHeaderLen])
	if err != nil {
		t.Fatal(err)
	}
	if h.Timestamp != ts {
		t.Errorf("Timestamp = %d, want %d", h.Timestamp, ts)
	}
	if h.Type != typ {
		t.Errorf("Type = %d, want %d", h.Type, typ)
	}

	us, err := mrt.DecodeMicrosecond(buf[mrt.CommonHeaderLen:])
	if err != nil {
		t.Fatal(err)
	}
	if us != usec {
		t.Errorf("Microsecond = %d, want %d", us, usec)
	}
}

func testPeers() []mrt.PeerEntry {
	return []mrt.PeerEntry{
		{
			Type:  0,
			BGPID: [4]byte{1, 2, 3, 4},
			IP:    []byte{10, 0, 0, 1},
			ASN:   65000,
		},
		{
			Type:  mrt.PeerAS4,
			BGPID: [4]byte{5, 6, 7, 8},
			IP:    []byte{10, 0, 0, 2},
			ASN:   400000,
		},
		{
			Type:  mrt.PeerAS4 | mrt.PeerIPv6,
			BGPID: [4]byte{9, 10, 11, 12},
			IP: []byte{
				0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
			},
			ASN: 500000,
		},
	}
}

func TestPeerIndexTableRoundTrip(t *testing.T) {
	collectorID := [4]byte{198, 51, 100, 4}
	viewName := "test-view"
	peers := testPeers()

	buf := make([]byte, 4096)
	n := mrt.WritePeerIndexTable(buf, 0, collectorID, viewName, peers)
	if n == 0 {
		t.Fatal("WritePeerIndexTable returned 0")
	}

	pit, err := mrt.DecodePeerIndexTable(buf[:n])
	if err != nil {
		t.Fatal(err)
	}

	if pit.CollectorBGPID != collectorID {
		t.Errorf("CollectorBGPID = %v, want %v", pit.CollectorBGPID, collectorID)
	}
	if pit.ViewName != viewName {
		t.Errorf("ViewName = %q, want %q", pit.ViewName, viewName)
	}
	if len(pit.Peers) != len(peers) {
		t.Fatalf("Peer count = %d, want %d", len(pit.Peers), len(peers))
	}

	for i, got := range pit.Peers {
		want := peers[i]
		if got.Type != want.Type {
			t.Errorf("Peer[%d].Type = %d, want %d", i, got.Type, want.Type)
		}
		if got.BGPID != want.BGPID {
			t.Errorf("Peer[%d].BGPID = %v, want %v", i, got.BGPID, want.BGPID)
		}
		if !bytes.Equal(got.IP, want.IP) {
			t.Errorf("Peer[%d].IP = %v, want %v", i, got.IP, want.IP)
		}
		if got.ASN != want.ASN {
			t.Errorf("Peer[%d].ASN = %d, want %d", i, got.ASN, want.ASN)
		}
	}
}

func testAttrs() []byte {
	return []byte{0x40, 0x01, 0x01, 0x00}
}

func testRIBEntries() []mrt.RIBEntry {
	attrs := testAttrs()
	return []mrt.RIBEntry{
		{PeerIndex: 0, OrigTime: 1700000000, Attributes: attrs},
		{PeerIndex: 1, OrigTime: 1700000001, Attributes: attrs},
	}
}

// RFC requirement: RFC8050-4.1-1 negative -- a base RIB subtype (RIB_IPV4_UNICAST, 2)
// has no Path Identifier field, so every decoded entry reports PathID 0: DecodeRIBEntries
// reads no path id for a non-add-path subtype (internal/mrt/decode.go:294).
func TestRIBRecordRoundTrip(t *testing.T) {
	seq := uint32(42)
	prefixLen := uint8(24)
	prefix := []byte{203, 0, 113}
	entries := testRIBEntries()

	buf := make([]byte, 4096)
	off := 0
	off += mrt.WriteRIBHeader(buf, off, seq, prefixLen, prefix)
	off += mrt.WriteRIBEntries(buf, off, entries, false)

	rec, err := mrt.DecodeRIBRecord(mrt.TDV2RIBIPv4Unicast, buf[:off])
	if err != nil {
		t.Fatal(err)
	}

	if rec.SequenceNumber != seq {
		t.Errorf("SequenceNumber = %d, want %d", rec.SequenceNumber, seq)
	}
	if rec.PrefixLength != prefixLen {
		t.Errorf("PrefixLength = %d, want %d", rec.PrefixLength, prefixLen)
	}
	if !bytes.Equal(rec.Prefix, prefix) {
		t.Errorf("Prefix = %v, want %v", rec.Prefix, prefix)
	}
	if len(rec.Entries) != len(entries) {
		t.Fatalf("Entry count = %d, want %d", len(rec.Entries), len(entries))
	}
	for i, got := range rec.Entries {
		want := entries[i]
		if got.PeerIndex != want.PeerIndex {
			t.Errorf("Entry[%d].PeerIndex = %d, want %d", i, got.PeerIndex, want.PeerIndex)
		}
		if got.OrigTime != want.OrigTime {
			t.Errorf("Entry[%d].OrigTime = %d, want %d", i, got.OrigTime, want.OrigTime)
		}
		if !bytes.Equal(got.Attributes, want.Attributes) {
			t.Errorf("Entry[%d].Attributes = %v, want %v", i, got.Attributes, want.Attributes)
		}
		if got.PathID != 0 {
			t.Errorf("Entry[%d].PathID = %d, want 0 (non-addpath)", i, got.PathID)
		}
	}
}

// RFC requirement: RFC8050-4.1-1 positive -- an add-path RIB subtype (RIB_IPV4_UNICAST_ADDPATH,
// 8) carries a 4-byte Path Identifier between Originated Time and Attribute Length:
// WriteRIBEntryAddPath places it there (internal/mrt/encode.go:107) and DecodeRIBEntries reads
// it back (decode.go:295), so PathID 1 and 2 round-trip.
// RFC requirement: RFC8050-x-2 positive -- the 4-byte Path Identifier is encoded and decoded
// big-endian: WriteRIBEntryAddPath writes it with be.PutUint32 (internal/mrt/encode.go:107) and
// DecodeRIBEntries reads it with binary.BigEndian.Uint32 (decode.go:295), so PathID 1 and 2
// survive the round-trip byte-for-byte.
func TestRIBRecordAddPathRoundTrip(t *testing.T) {
	seq := uint32(100)
	prefixLen := uint8(32)
	prefix := []byte{10, 1, 2, 3}
	attrs := testAttrs()
	entries := []mrt.RIBEntry{
		{PeerIndex: 0, OrigTime: 1700000000, PathID: 1, Attributes: attrs},
		{PeerIndex: 1, OrigTime: 1700000001, PathID: 2, Attributes: attrs},
	}

	buf := make([]byte, 4096)
	off := 0
	off += mrt.WriteRIBHeader(buf, off, seq, prefixLen, prefix)
	off += mrt.WriteRIBEntries(buf, off, entries, true)

	rec, err := mrt.DecodeRIBRecord(mrt.TDV2RIBIPv4UnicastAP, buf[:off])
	if err != nil {
		t.Fatal(err)
	}

	if rec.SequenceNumber != seq {
		t.Errorf("SequenceNumber = %d, want %d", rec.SequenceNumber, seq)
	}
	if len(rec.Entries) != len(entries) {
		t.Fatalf("Entry count = %d, want %d", len(rec.Entries), len(entries))
	}
	for i, got := range rec.Entries {
		want := entries[i]
		if got.PathID != want.PathID {
			t.Errorf("Entry[%d].PathID = %d, want %d", i, got.PathID, want.PathID)
		}
		if got.PeerIndex != want.PeerIndex {
			t.Errorf("Entry[%d].PeerIndex = %d, want %d", i, got.PeerIndex, want.PeerIndex)
		}
		if !bytes.Equal(got.Attributes, want.Attributes) {
			t.Errorf("Entry[%d].Attributes = %v, want %v", i, got.Attributes, want.Attributes)
		}
	}
}

// RFC requirement: RFC8050-4.2-1 negative -- the base RIB_GENERIC subtype (6) carries no Path
// Identifier in its NLRI blob: DecodeRIBGenericRecord skips no bytes before the prefix length
// for a non-add-path subtype (internal/mrt/decode.go:235), so the NLRI decodes verbatim and the
// entries stay in their RFC 6396 form.
func TestRIBGenericRoundTrip(t *testing.T) {
	seq := uint32(7)
	afi := uint16(1)
	safi := uint8(128)
	nlri := []byte{24, 10, 0, 0}
	attrs := testAttrs()
	entries := []mrt.RIBEntry{
		{PeerIndex: 0, OrigTime: 1700000000, Attributes: attrs},
	}

	buf := make([]byte, 4096)
	off := 0
	off += mrt.WriteRIBGenericHeader(buf, off, seq, afi, safi, nlri)
	off += mrt.WriteRIBEntries(buf, off, entries, false)

	rec, err := mrt.DecodeRIBGenericRecord(mrt.TDV2RIBGeneric, buf[:off])
	if err != nil {
		t.Fatal(err)
	}

	if rec.SequenceNumber != seq {
		t.Errorf("SequenceNumber = %d, want %d", rec.SequenceNumber, seq)
	}
	if rec.AFI != afi {
		t.Errorf("AFI = %d, want %d", rec.AFI, afi)
	}
	if rec.SAFI != safi {
		t.Errorf("SAFI = %d, want %d", rec.SAFI, safi)
	}
	if !bytes.Equal(rec.NLRI, nlri) {
		t.Errorf("NLRI = %v, want %v", rec.NLRI, nlri)
	}
	if len(rec.Entries) != 1 {
		t.Fatalf("Entry count = %d, want 1", len(rec.Entries))
	}
	if !bytes.Equal(rec.Entries[0].Attributes, attrs) {
		t.Errorf("Entry[0].Attributes = %v, want %v", rec.Entries[0].Attributes, attrs)
	}
}

func testBGP4MPHeader() mrt.BGP4MPHeader {
	return mrt.BGP4MPHeader{
		PeerAS:  64496,
		LocalAS: 64497,
		IfIndex: 0,
		AFI:     mrt.AFIIPv4,
		PeerIP:  []byte{192, 0, 2, 85},
		LocalIP: []byte{198, 51, 100, 4},
	}
}

// RFC requirement: RFC8050-x-3 positive -- a BGP4MP MESSAGE record copies the encapsulated BGP
// message verbatim (WriteBGP4MPMessage internal/mrt/encode.go:170, DecodeBGP4MPMessage
// decode.go:325-328), so a Path Identifier carried in the message's own NLRI is preserved inside
// the message body and the MRT layer never relocates it into the header.
func TestBGP4MPMessageRoundTrip(t *testing.T) {
	hdr := testBGP4MPHeader()
	bgpMsg := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x17, 0x04,
	}

	buf := make([]byte, 4096)
	n := mrt.WriteBGP4MPMessage(buf, 0, &hdr, true, bgpMsg)

	msg, err := mrt.DecodeBGP4MPMessage(mrt.BGP4MPMessageAS4, buf[:n])
	if err != nil {
		t.Fatal(err)
	}

	if msg.PeerAS != hdr.PeerAS {
		t.Errorf("PeerAS = %d, want %d", msg.PeerAS, hdr.PeerAS)
	}
	if msg.LocalAS != hdr.LocalAS {
		t.Errorf("LocalAS = %d, want %d", msg.LocalAS, hdr.LocalAS)
	}
	if msg.AFI != hdr.AFI {
		t.Errorf("AFI = %d, want %d", msg.AFI, hdr.AFI)
	}
	if !bytes.Equal(msg.PeerIP, hdr.PeerIP) {
		t.Errorf("PeerIP = %v, want %v", msg.PeerIP, hdr.PeerIP)
	}
	if !bytes.Equal(msg.LocalIP, hdr.LocalIP) {
		t.Errorf("LocalIP = %v, want %v", msg.LocalIP, hdr.LocalIP)
	}
	if !bytes.Equal(msg.BGPMessage, bgpMsg) {
		t.Errorf("BGPMessage = %v, want %v", msg.BGPMessage, bgpMsg)
	}
}

func TestBGP4MPMessage2ByteAS(t *testing.T) {
	hdr := testBGP4MPHeader()
	hdr.PeerAS = 65000
	hdr.LocalAS = 65001
	bgpMsg := []byte{0xff, 0xff, 0xff, 0xff, 0x00, 0x17, 0x04}

	buf := make([]byte, 4096)
	n := mrt.WriteBGP4MPMessage(buf, 0, &hdr, false, bgpMsg)

	msg, err := mrt.DecodeBGP4MPMessage(mrt.BGP4MPMessage, buf[:n])
	if err != nil {
		t.Fatal(err)
	}

	if msg.PeerAS != hdr.PeerAS {
		t.Errorf("PeerAS = %d, want %d", msg.PeerAS, hdr.PeerAS)
	}
	if msg.LocalAS != hdr.LocalAS {
		t.Errorf("LocalAS = %d, want %d", msg.LocalAS, hdr.LocalAS)
	}
	if !bytes.Equal(msg.BGPMessage, bgpMsg) {
		t.Errorf("BGPMessage mismatch")
	}
}

// RFC requirement: RFC8050-x-3 negative -- the BGP4MP MRT header carries no Path Identifier
// field: decodeBGP4MPHeader (internal/mrt/decode.go:352-403), shared by the message and
// state-change paths, consumes only the AS, interface, AFI and IP fields, so a state-change
// record (which has no encapsulated message) decodes with no path id anywhere -- the path id
// exists only inside an encapsulated message's NLRI.
func TestBGP4MPStateChangeRoundTrip(t *testing.T) {
	hdr := testBGP4MPHeader()
	oldState := mrt.FSMActive
	newState := mrt.FSMOpenSent

	buf := make([]byte, 4096)
	n := mrt.WriteBGP4MPStateChange(buf, 0, &hdr, true, oldState, newState)

	sc, err := mrt.DecodeBGP4MPStateChange(mrt.BGP4MPStateChangeAS4, buf[:n])
	if err != nil {
		t.Fatal(err)
	}

	if sc.PeerAS != hdr.PeerAS {
		t.Errorf("PeerAS = %d, want %d", sc.PeerAS, hdr.PeerAS)
	}
	if sc.LocalAS != hdr.LocalAS {
		t.Errorf("LocalAS = %d, want %d", sc.LocalAS, hdr.LocalAS)
	}
	if !bytes.Equal(sc.PeerIP, hdr.PeerIP) {
		t.Errorf("PeerIP = %v, want %v", sc.PeerIP, hdr.PeerIP)
	}
	if sc.OldState != oldState {
		t.Errorf("OldState = %d, want %d", sc.OldState, oldState)
	}
	if sc.NewState != newState {
		t.Errorf("NewState = %d, want %d", sc.NewState, newState)
	}
}

func TestTableDumpRoundTrip(t *testing.T) {
	rec := mrt.TableDumpRecord{
		ViewNumber: 0,
		SeqNumber:  1,
		Prefix:     []byte{203, 0, 113, 0},
		PrefixLen:  24,
		Status:     1,
		OrigTime:   1700000000,
		PeerIP:     []byte{10, 0, 0, 1},
		PeerAS:     65000,
		Attributes: testAttrs(),
	}

	buf := make([]byte, 4096)
	n := mrt.WriteTableDump(buf, 0, &rec)

	got, err := mrt.DecodeTableDump(mrt.TableDumpAFIIPv4, buf[:n])
	if err != nil {
		t.Fatal(err)
	}

	if got.ViewNumber != rec.ViewNumber {
		t.Errorf("ViewNumber = %d, want %d", got.ViewNumber, rec.ViewNumber)
	}
	if got.SeqNumber != rec.SeqNumber {
		t.Errorf("SeqNumber = %d, want %d", got.SeqNumber, rec.SeqNumber)
	}
	if !bytes.Equal(got.Prefix, rec.Prefix) {
		t.Errorf("Prefix = %v, want %v", got.Prefix, rec.Prefix)
	}
	if got.PrefixLen != rec.PrefixLen {
		t.Errorf("PrefixLen = %d, want %d", got.PrefixLen, rec.PrefixLen)
	}
	if got.Status != rec.Status {
		t.Errorf("Status = %d, want %d", got.Status, rec.Status)
	}
	if got.OrigTime != rec.OrigTime {
		t.Errorf("OrigTime = %d, want %d", got.OrigTime, rec.OrigTime)
	}
	if !bytes.Equal(got.PeerIP, rec.PeerIP) {
		t.Errorf("PeerIP = %v, want %v", got.PeerIP, rec.PeerIP)
	}
	if got.PeerAS != rec.PeerAS {
		t.Errorf("PeerAS = %d, want %d", got.PeerAS, rec.PeerAS)
	}
	if !bytes.Equal(got.Attributes, rec.Attributes) {
		t.Errorf("Attributes = %v, want %v", got.Attributes, rec.Attributes)
	}
}

// Regression: BLOCKER 1 — RIB_GENERIC_ADDPATH must skip the 4-byte Path ID
// before reading the prefix length byte. Previously data[off] read the first
// byte of the Path ID as the prefix length, corrupting the parse.
// RFC requirement: RFC8050-4.2-1 positive -- RIB_GENERIC_ADDPATH (subtype 12) does not redefine
// the RIB Entry: the 4-byte Path Identifier lives in the raw NLRI blob (decoded by skipping it
// before the prefix length, internal/mrt/decode.go:235-239) while the entries are parsed with
// add-path off (decode.go:261), so the NLRI keeps the Path ID and the entry carries only attributes.
func TestRIBGenericAddPathRoundTrip(t *testing.T) {
	// Build a RIB_GENERIC_ADDPATH record manually.
	// NLRI blob for add-path: [PathID(4)][PrefixLen(1)][Prefix(bytes)]
	nlri := []byte{
		0x00, 0x00, 0x00, 0x2A, // Path ID = 42
		24,       // prefix length
		10, 0, 0, // 10.0.0.0/24
	}

	buf := make([]byte, 4096)
	off := 0

	// Sequence Number
	be := binary.BigEndian
	be.PutUint32(buf[off:], 1)
	off += 4
	// AFI
	be.PutUint16(buf[off:], mrt.AFIIPv4)
	off += 2
	// SAFI (unicast)
	buf[off] = 1
	off++
	// NLRI blob
	copy(buf[off:], nlri)
	off += len(nlri)
	// Entry count = 1
	be.PutUint16(buf[off:], 1)
	off += 2
	// RIB entry (no add-path in RIB_GENERIC_ADDPATH entries per RFC 8050 Section 4.2)
	be.PutUint16(buf[off:], 0) // Peer Index
	off += 2
	be.PutUint32(buf[off:], 1700000000) // Originated Time
	off += 4
	attrs := testAttrs()
	be.PutUint16(buf[off:], uint16(len(attrs)))
	off += 2
	copy(buf[off:], attrs)
	off += len(attrs)

	rec, err := mrt.DecodeRIBGenericRecord(mrt.TDV2RIBGenericAP, buf[:off])
	if err != nil {
		t.Fatal(err)
	}
	if rec.AFI != mrt.AFIIPv4 {
		t.Errorf("AFI = %d, want %d", rec.AFI, mrt.AFIIPv4)
	}
	if rec.SAFI != 1 {
		t.Errorf("SAFI = %d, want 1", rec.SAFI)
	}
	if !bytes.Equal(rec.NLRI, nlri) {
		t.Errorf("NLRI = %x, want %x", rec.NLRI, nlri)
	}
	if len(rec.Entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(rec.Entries))
	}
	if !bytes.Equal(rec.Entries[0].Attributes, attrs) {
		t.Errorf("attrs mismatch")
	}
}

// Regression: BLOCKER 2 — encoder must not panic on nil or short IP slices.
func TestWriteBGP4MPNilIP(t *testing.T) {
	hdr := mrt.BGP4MPHeader{
		PeerAS:  64496,
		LocalAS: 64497,
		AFI:     mrt.AFIIPv4,
		PeerIP:  nil,
		LocalIP: nil,
	}
	buf := make([]byte, 4096)
	// Must not panic; zero-pads missing IPs.
	n := mrt.WriteBGP4MPMessage(buf, 0, &hdr, true, []byte{0x00, 0x17, 0x04})
	if n == 0 {
		t.Fatal("expected nonzero write length")
	}

	msg, err := mrt.DecodeBGP4MPMessage(mrt.BGP4MPMessageAS4, buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	// Decoded IPs should be all zeros (4 bytes for IPv4)
	if !bytes.Equal(msg.PeerIP, []byte{0, 0, 0, 0}) {
		t.Errorf("PeerIP = %v, want all zeros", msg.PeerIP)
	}
	if !bytes.Equal(msg.LocalIP, []byte{0, 0, 0, 0}) {
		t.Errorf("LocalIP = %v, want all zeros", msg.LocalIP)
	}
}

// Regression: BLOCKER 2 — encoder must not panic on short PeerEntry IP slices.
func TestWritePeerEntryShortIP(t *testing.T) {
	p := mrt.PeerEntry{
		Type:  mrt.PeerAS4 | mrt.PeerIPv6,
		BGPID: [4]byte{1, 2, 3, 4},
		IP:    []byte{0x20, 0x01}, // only 2 bytes, should be 16
		ASN:   65000,
	}
	buf := make([]byte, 4096)
	// Must not panic; zero-pads the short slice.
	n := mrt.WritePeerEntry(buf, 0, &p)
	if n == 0 {
		t.Fatal("expected nonzero write length")
	}
}

func TestDecodeHeaderShortData(t *testing.T) {
	_, err := mrt.DecodeHeader([]byte{0, 1, 2})
	if err == nil {
		t.Fatal("expected error for short header data")
	}
}

func TestRIBSubtypeAFI(t *testing.T) {
	tests := []struct {
		subtype uint16
		want    uint16
	}{
		{mrt.TDV2RIBIPv4Unicast, mrt.AFIIPv4},
		{mrt.TDV2RIBIPv4Multicast, mrt.AFIIPv4},
		{mrt.TDV2RIBIPv6Unicast, mrt.AFIIPv6},
		{mrt.TDV2RIBIPv6Multicast, mrt.AFIIPv6},
		{mrt.TDV2RIBIPv4UnicastAP, mrt.AFIIPv4},
		{mrt.TDV2RIBIPv4MulticastAP, mrt.AFIIPv4},
		{mrt.TDV2RIBIPv6UnicastAP, mrt.AFIIPv6},
		{mrt.TDV2RIBIPv6MulticastAP, mrt.AFIIPv6},
		{mrt.TDV2RIBGeneric, 0},
		{mrt.TDV2PeerIndexTable, 0},
		{mrt.TDV2GeoPeerTable, 0},
	}
	for _, tc := range tests {
		if got := mrt.RIBSubtypeAFI(tc.subtype); got != tc.want {
			t.Errorf("RIBSubtypeAFI(%d) = %d, want %d", tc.subtype, got, tc.want)
		}
	}
}

func TestIsAddPathHelpers(t *testing.T) {
	addPathRIB := []uint16{
		mrt.TDV2RIBIPv4UnicastAP, mrt.TDV2RIBIPv4MulticastAP,
		mrt.TDV2RIBIPv6UnicastAP, mrt.TDV2RIBIPv6MulticastAP,
		mrt.TDV2RIBGenericAP,
	}
	notAddPathRIB := []uint16{
		mrt.TDV2PeerIndexTable, mrt.TDV2RIBIPv4Unicast, mrt.TDV2RIBIPv6Unicast,
		mrt.TDV2RIBGeneric, mrt.TDV2GeoPeerTable,
	}
	// RFC requirement: RFC8050-x-4 positive -- add-path is distinguished purely by the MRT
	// subtype value: IsAddPathRIBSubtype (internal/mrt/types.go:131) and IsAddPathBGP4MPSubtype
	// (types.go:161) take only a subtype and classify the RFC 8050 add-path subtypes as add-path,
	// with no session or capability argument.
	for _, s := range addPathRIB {
		if !mrt.IsAddPathRIBSubtype(s) {
			t.Errorf("IsAddPathRIBSubtype(%d) = false, want true", s)
		}
	}
	// RFC requirement: RFC8050-x-4 negative -- the base (non-add-path) RIB and BGP4MP subtypes are
	// classified not-add-path by the same subtype-only helpers (internal/mrt/types.go:131,161),
	// confirming the add-path decision comes from the subtype code alone and never from a
	// negotiated capability lookup.
	for _, s := range notAddPathRIB {
		if mrt.IsAddPathRIBSubtype(s) {
			t.Errorf("IsAddPathRIBSubtype(%d) = true, want false", s)
		}
	}

	addPathBGP4MP := []uint16{
		mrt.BGP4MPMessageAP, mrt.BGP4MPMessageAS4AP,
		mrt.BGP4MPMessageLocalAP, mrt.BGP4MPMessageAS4LocalAP,
	}
	notAddPathBGP4MP := []uint16{
		mrt.BGP4MPMessage, mrt.BGP4MPMessageAS4,
		mrt.BGP4MPStateChange, mrt.BGP4MPStateChangeAS4,
	}
	for _, s := range addPathBGP4MP {
		if !mrt.IsAddPathBGP4MPSubtype(s) {
			t.Errorf("IsAddPathBGP4MPSubtype(%d) = false, want true", s)
		}
	}
	for _, s := range notAddPathBGP4MP {
		if mrt.IsAddPathBGP4MPSubtype(s) {
			t.Errorf("IsAddPathBGP4MPSubtype(%d) = true, want false", s)
		}
	}
}
