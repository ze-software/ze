// Goal: prove the file-format writer emits the bytes a pcap reader expects, and
// that the synthetic IP and TCP framing carries correct lengths, correct
// checksums and a sequence number that advances by the payload.
// Method: write into a bytes.Buffer and assert on the octets, because every
// consumer of these files is a byte-level parser.

package pcap

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"
)

// testStamp is a fixed capture time, so a generated file is reproducible.
var testStamp = time.Unix(1_700_000_000, 123_456_000).UTC()

func TestPcapGlobalHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFileHeader(&buf, 4096, LinkTypeRaw); err != nil {
		t.Fatalf("WriteFileHeader: %v", err)
	}

	got := buf.Bytes()
	if len(got) != FileHeaderLen {
		t.Fatalf("file header is %d bytes, want %d", len(got), FileHeaderLen)
	}
	if magic := binary.LittleEndian.Uint32(got[0:4]); magic != magicMicro {
		t.Errorf("magic = %#x, want %#x", magic, magicMicro)
	}
	if major := binary.LittleEndian.Uint16(got[4:6]); major != versionMajor {
		t.Errorf("version major = %d, want %d", major, versionMajor)
	}
	if minor := binary.LittleEndian.Uint16(got[6:8]); minor != versionMinor {
		t.Errorf("version minor = %d, want %d", minor, versionMinor)
	}
	for i := 8; i < 16; i++ {
		if got[i] != 0 {
			t.Errorf("byte %d of the timezone and sigfigs fields = %d, want 0", i, got[i])
		}
	}
	if snap := binary.LittleEndian.Uint32(got[16:20]); snap != 4096 {
		t.Errorf("snaplen = %d, want 4096", snap)
	}
	if link := binary.LittleEndian.Uint32(got[20:24]); link != LinkTypeRaw {
		t.Errorf("link type = %d, want %d", link, LinkTypeRaw)
	}
}

func TestPcapRecordWithOrigLen(t *testing.T) {
	var buf bytes.Buffer
	captured := []byte{1, 2, 3, 4, 5}
	if err := WriteRecord(&buf, testStamp, captured, 900); err != nil {
		t.Fatalf("WriteRecord: %v", err)
	}

	got := buf.Bytes()
	if incl := binary.LittleEndian.Uint32(got[8:12]); incl != uint32(len(captured)) {
		t.Errorf("captured length = %d, want %d", incl, len(captured))
	}
	if orig := binary.LittleEndian.Uint32(got[12:16]); orig != 900 {
		t.Errorf("original length = %d, want 900", orig)
	}
	if !bytes.Equal(got[RecordHeaderLen:], captured) {
		t.Errorf("record bytes = % x, want % x", got[RecordHeaderLen:], captured)
	}
}

// TestWriteRecordShorterOriginalRefused proves the writer fails closed rather
// than emitting a record whose original length is below its captured length,
// which no reader can make sense of.
func TestWriteRecordShorterOriginalRefused(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteRecord(&buf, testStamp, []byte{1, 2, 3}, 2); err == nil {
		t.Fatal("WriteRecord accepted an original length below the captured length")
	}
	if buf.Len() != 0 {
		t.Errorf("refused record still wrote %d bytes", buf.Len())
	}
}

// TestWriteRecordMatchesLegacyLayout pins the record header against the layout
// the diag plugin's writePcapPacket wrote before this package absorbed it, so
// the L2TP, BFD and interface captures stay byte-identical (A-6, AC-7).
func TestWriteRecordMatchesLegacyLayout(t *testing.T) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}

	var got bytes.Buffer
	if err := WriteRecord(&got, testStamp, data, len(data)); err != nil {
		t.Fatalf("WriteRecord: %v", err)
	}

	var want bytes.Buffer
	var hdr [16]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(testStamp.Unix()))
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(testStamp.Nanosecond()/1000))
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(len(data)))
	binary.LittleEndian.PutUint32(hdr[12:16], uint32(len(data)))
	want.Write(hdr[:])
	want.Write(data)

	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Errorf("record = % x, want % x", got.Bytes(), want.Bytes())
	}
}

// checksumOK reports whether an internet checksum over data verifies. A header
// carrying its correct checksum sums to all ones, whose complement is zero.
func checksumOK(data []byte) bool { return checksum(data) == 0 }

func TestFrameIPv4TCP(t *testing.T) {
	flow := Flow{
		SourceAddr: netip.MustParseAddr("192.0.2.1"),
		TargetAddr: netip.MustParseAddr("192.0.2.2"),
		SourcePort: 179,
		TargetPort: 50000,
	}
	payload := []byte{0xFF, 0x00, 0x11, 0x22, 0x33}

	var buf bytes.Buffer
	if err := NewFramer().WriteMessage(&buf, testStamp, flow, payload, len(payload)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	record := buf.Bytes()[RecordHeaderLen:]
	if len(record) != IPv4HeaderLen+TCPHeaderLen+len(payload) {
		t.Fatalf("record is %d bytes, want %d", len(record), IPv4HeaderLen+TCPHeaderLen+len(payload))
	}
	if record[0] != 0x45 {
		t.Errorf("version and IHL = %#x, want 0x45", record[0])
	}
	if total := binary.BigEndian.Uint16(record[2:4]); int(total) != len(record) {
		t.Errorf("IP total length = %d, want %d", total, len(record))
	}
	if record[9] != protocolTCP {
		t.Errorf("IP protocol = %d, want %d", record[9], protocolTCP)
	}
	if !checksumOK(record[:IPv4HeaderLen]) {
		t.Errorf("IPv4 header checksum does not verify: % x", record[:IPv4HeaderLen])
	}

	tcp := record[IPv4HeaderLen:]
	if source := binary.BigEndian.Uint16(tcp[0:2]); source != 179 {
		t.Errorf("TCP source port = %d, want 179", source)
	}
	if target := binary.BigEndian.Uint16(tcp[2:4]); target != 50000 {
		t.Errorf("TCP target port = %d, want 50000", target)
	}
	if tcp[12] != 0x50 {
		t.Errorf("TCP data offset byte = %#x, want 0x50", tcp[12])
	}
	if tcp[13] != 0x18 {
		t.Errorf("TCP flags = %#x, want 0x18 (PSH and ACK)", tcp[13])
	}
	if !bytes.Equal(tcp[TCPHeaderLen:], payload) {
		t.Errorf("payload = % x, want % x", tcp[TCPHeaderLen:], payload)
	}

	var pseudo [12]byte
	copy(pseudo[0:4], record[12:16])
	copy(pseudo[4:8], record[16:20])
	pseudo[9] = protocolTCP
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(tcp)))
	if !checksumOK(append(pseudo[:], tcp...)) {
		t.Errorf("TCP checksum does not verify over the IPv4 pseudo-header")
	}
}

func TestFrameIPv6TCP(t *testing.T) {
	flow := Flow{
		SourceAddr: netip.MustParseAddr("2001:db8::1"),
		TargetAddr: netip.MustParseAddr("2001:db8::2"),
		SourcePort: 50000,
		TargetPort: 179,
	}
	payload := []byte{0xFF, 0xFF, 0x00, 0x13, 0x04}

	var buf bytes.Buffer
	if err := NewFramer().WriteMessage(&buf, testStamp, flow, payload, len(payload)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	record := buf.Bytes()[RecordHeaderLen:]
	if len(record) != IPv6HeaderLen+TCPHeaderLen+len(payload) {
		t.Fatalf("record is %d bytes, want %d", len(record), IPv6HeaderLen+TCPHeaderLen+len(payload))
	}
	if record[0]>>4 != 6 {
		t.Errorf("IP version nibble = %d, want 6", record[0]>>4)
	}
	if want := TCPHeaderLen + len(payload); int(binary.BigEndian.Uint16(record[4:6])) != want {
		t.Errorf("IPv6 payload length = %d, want %d", binary.BigEndian.Uint16(record[4:6]), want)
	}
	if record[6] != protocolTCP {
		t.Errorf("IPv6 next header = %d, want %d", record[6], protocolTCP)
	}

	tcp := record[IPv6HeaderLen:]
	var pseudo [40]byte
	copy(pseudo[0:16], record[8:24])
	copy(pseudo[16:32], record[24:40])
	binary.BigEndian.PutUint32(pseudo[32:36], uint32(len(tcp)))
	pseudo[39] = protocolTCP
	if !checksumOK(append(pseudo[:], tcp...)) {
		t.Errorf("TCP checksum does not verify over the IPv6 pseudo-header")
	}

	// The whole point of link type 101 for both families: the reader takes the
	// family off the version nibble, so an IPv6 record needs no second file.
	seg, err := dissect(LinkTypeRaw, record)
	if err != nil {
		t.Fatalf("dissect of an IPv6 record under link type %d: %v", LinkTypeRaw, err)
	}
	if seg.flow != flow {
		t.Errorf("flow = %s, want %s", seg.flow, flow)
	}
	if !bytes.Equal(seg.data, payload) {
		t.Errorf("payload = % x, want % x", seg.data, payload)
	}
}

func TestSequenceMonotonicPerDirection(t *testing.T) {
	forward := Flow{
		SourceAddr: netip.MustParseAddr("192.0.2.1"),
		TargetAddr: netip.MustParseAddr("192.0.2.2"),
		SourcePort: 179,
		TargetPort: 50000,
	}
	reverse := forward.Reverse()

	first := bytes.Repeat([]byte{0xA1}, 19)
	second := bytes.Repeat([]byte{0xA2}, 29)
	back := bytes.Repeat([]byte{0xB1}, 23)

	var buf bytes.Buffer
	framer := NewFramer()
	for _, step := range []struct {
		flow    Flow
		payload []byte
	}{{forward, first}, {reverse, back}, {forward, second}} {
		if err := framer.WriteMessage(&buf, testStamp, step.flow, step.payload, len(step.payload)); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}

	sequences, acknowledgements := readSequences(t, buf.Bytes())
	if len(sequences) != 3 {
		t.Fatalf("read %d records, want 3", len(sequences))
	}
	if sequences[0] != sequenceInitial {
		t.Errorf("first forward sequence = %d, want %d", sequences[0], sequenceInitial)
	}
	if want := sequenceInitial + uint32(len(first)); sequences[2] != want {
		t.Errorf("second forward sequence = %d, want %d (advanced by the %d-byte first message)", sequences[2], want, len(first))
	}
	if sequences[1] != sequenceInitial {
		t.Errorf("reverse sequence = %d, want %d: the two directions count separately", sequences[1], sequenceInitial)
	}
	// The acknowledgement of the second forward record names every byte the
	// reverse direction has sent, which is what stops a reader treating the
	// record as a retransmission.
	if want := sequenceInitial + uint32(len(back)); acknowledgements[2] != want {
		t.Errorf("second forward acknowledgement = %d, want %d", acknowledgements[2], want)
	}
}

// readSequences pulls the TCP sequence and acknowledgement out of every record
// in a framed capture body (the records only, with no file header).
func readSequences(t *testing.T, body []byte) (sequences, acknowledgements []uint32) {
	t.Helper()
	for offset := 0; offset < len(body); {
		if offset+RecordHeaderLen > len(body) {
			t.Fatalf("record header runs past the %d-byte body at offset %d", len(body), offset)
		}
		captured := int(binary.LittleEndian.Uint32(body[offset+8 : offset+12]))
		record := body[offset+RecordHeaderLen : offset+RecordHeaderLen+captured]
		tcp := record[IPv4HeaderLen:]
		sequences = append(sequences, binary.BigEndian.Uint32(tcp[4:8]))
		acknowledgements = append(acknowledgements, binary.BigEndian.Uint32(tcp[8:12]))
		offset += RecordHeaderLen + captured
	}
	return sequences, acknowledgements
}

// TestFrameTruncatedDeclaresTrueLength covers AC-6: a message longer than the
// ring slot is captured short, and both the record and the IP header still
// declare what was on the network, so a reader shows truncation.
func TestFrameTruncatedDeclaresTrueLength(t *testing.T) {
	flow := Flow{
		SourceAddr: netip.MustParseAddr("192.0.2.1"),
		TargetAddr: netip.MustParseAddr("192.0.2.2"),
		SourcePort: 179,
		TargetPort: 50000,
	}
	captured := bytes.Repeat([]byte{0xCC}, 4096)
	const original = 9000

	var buf bytes.Buffer
	framer := NewFramer()
	if err := framer.WriteMessage(&buf, testStamp, flow, captured, original); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	// A second message in the same direction proves the sequence advanced by
	// the ON-WIRE length, not by the captured length.
	next := []byte{0x01, 0x02}
	if err := framer.WriteMessage(&buf, testStamp, flow, next, len(next)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	body := buf.Bytes()
	incl := binary.LittleEndian.Uint32(body[8:12])
	orig := binary.LittleEndian.Uint32(body[12:16])
	if want := uint32(IPv4HeaderLen + TCPHeaderLen + len(captured)); incl != want {
		t.Errorf("captured length = %d, want %d", incl, want)
	}
	if want := uint32(IPv4HeaderLen + TCPHeaderLen + original); orig != want {
		t.Errorf("original length = %d, want %d", orig, want)
	}

	record := body[RecordHeaderLen : RecordHeaderLen+incl]
	if total := binary.BigEndian.Uint16(record[2:4]); int(total) != IPv4HeaderLen+TCPHeaderLen+original {
		t.Errorf("IP total length = %d, want %d", total, IPv4HeaderLen+TCPHeaderLen+original)
	}

	sequences, _ := readSequences(t, body)
	if want := sequenceInitial + original; sequences[1] != want {
		t.Errorf("sequence after a truncated message = %d, want %d", sequences[1], want)
	}
}

// TestFrameRefusesMixedFamilies proves the framer fails closed rather than
// writing a record whose two addresses cannot both fit one IP header.
func TestFrameRefusesMixedFamilies(t *testing.T) {
	flow := Flow{
		SourceAddr: netip.MustParseAddr("192.0.2.1"),
		TargetAddr: netip.MustParseAddr("2001:db8::2"),
		SourcePort: 179,
		TargetPort: 50000,
	}
	var buf bytes.Buffer
	if err := NewFramer().WriteMessage(&buf, testStamp, flow, []byte{1}, 1); err == nil {
		t.Fatal("WriteMessage accepted a flow mixing an IPv4 and an IPv6 address")
	}
	if buf.Len() != 0 {
		t.Errorf("refused record still wrote %d bytes", buf.Len())
	}
}
