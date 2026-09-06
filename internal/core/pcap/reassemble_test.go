// Goal: prove the reassembler puts a TCP byte stream back together the way a
// capture from a real network arrives -- out of order, retransmitted,
// overlapping, with holes -- and that it reports what it could not see instead
// of returning an empty result that reads as "no traffic here".
// Method: synthesize records segment by segment, so each test states exactly
// the arrival pattern it is about.

package pcap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"
	"time"
)

// testFlow is one direction of a BGP session between two documentation
// addresses. Its two ends carry different addresses, so no test can pass by
// confusing a direction with its reverse.
var testFlow = Flow{
	SourceAddr: netip.MustParseAddr("192.0.2.1"),
	TargetAddr: netip.MustParseAddr("192.0.2.2"),
	SourcePort: 179,
	TargetPort: 50000,
}

// testSegment is one TCP segment to place in a synthesized capture.
type testSegment struct {
	flow     Flow
	sequence uint32
	syn      bool
	data     []byte
	stamp    time.Time
}

// buildTCPCapture writes a LinkTypeRaw pcap holding one record per segment, in
// the order given. The order is the arrival order, which is what the
// reassembler must not depend on.
func buildTCPCapture(t *testing.T, segments []testSegment) []byte {
	t.Helper()

	var file bytes.Buffer
	if err := WriteFileHeader(&file, 65535, LinkTypeRaw); err != nil {
		t.Fatalf("WriteFileHeader: %v", err)
	}
	for i := range segments {
		seg := &segments[i]
		stamp := seg.stamp
		if stamp.IsZero() {
			stamp = testStamp.Add(time.Duration(i) * time.Millisecond)
		}
		record := buildIPv4TCP(seg)
		if err := WriteRecord(&file, stamp, record, len(record)); err != nil {
			t.Fatalf("WriteRecord: %v", err)
		}
	}
	return file.Bytes()
}

// buildIPv4TCP assembles one IPv4 and TCP packet around a segment's payload.
func buildIPv4TCP(seg *testSegment) []byte {
	record := make([]byte, IPv4HeaderLen+TCPHeaderLen+len(seg.data))
	source := seg.flow.SourceAddr.As4()
	target := seg.flow.TargetAddr.As4()

	record[0] = 0x45
	binary.BigEndian.PutUint16(record[2:4], uint16(len(record)))
	record[8] = 64
	record[9] = protocolTCP
	copy(record[12:16], source[:])
	copy(record[16:20], target[:])
	binary.BigEndian.PutUint16(record[10:12], checksum(record[:IPv4HeaderLen]))

	tcp := record[IPv4HeaderLen:]
	binary.BigEndian.PutUint16(tcp[0:2], seg.flow.SourcePort)
	binary.BigEndian.PutUint16(tcp[2:4], seg.flow.TargetPort)
	binary.BigEndian.PutUint32(tcp[4:8], seg.sequence)
	tcp[12] = 0x50
	tcp[13] = 0x18
	if seg.syn {
		tcp[13] = 0x02
	}
	copy(tcp[TCPHeaderLen:], seg.data)
	return record
}

// singleStream runs a reassembly and insists on exactly one contiguous run, so
// a test about content is never quietly passed by a split.
func singleStream(t *testing.T, file []byte) (*Stream, *Report) {
	t.Helper()
	streams, report, err := Reassemble(bytes.NewReader(file), 179)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("got %d streams, want 1: %s", len(streams), report)
	}
	return &streams[0], report
}

func TestReassembleInOrder(t *testing.T) {
	file := buildTCPCapture(t, []testSegment{
		{flow: testFlow, sequence: 1000, syn: true},
		{flow: testFlow, sequence: 1001, data: []byte("one")},
		{flow: testFlow, sequence: 1004, data: []byte("two")},
		{flow: testFlow, sequence: 1007, data: []byte("three")},
	})

	stream, report := singleStream(t, file)
	if got := string(stream.Bytes); got != "onetwothree" {
		t.Errorf("stream = %q, want %q", got, "onetwothree")
	}
	if len(report.Gaps) != 0 {
		t.Errorf("report holds %d gaps, want none: %s", len(report.Gaps), report)
	}
	if report.BytesReassembled != 11 {
		t.Errorf("bytes reassembled = %d, want 11", report.BytesReassembled)
	}
}

func TestReassembleOutOfOrder(t *testing.T) {
	// The three segments arrive last, first, middle, and one of them arrives
	// twice. A reassembly that concatenated arrival order would produce
	// "threeonetwoone".
	file := buildTCPCapture(t, []testSegment{
		{flow: testFlow, sequence: 1007, data: []byte("three")},
		{flow: testFlow, sequence: 1001, data: []byte("one")},
		{flow: testFlow, sequence: 1004, data: []byte("two")},
		{flow: testFlow, sequence: 1001, data: []byte("one")},
	})

	stream, report := singleStream(t, file)
	if got := string(stream.Bytes); got != "onetwothree" {
		t.Errorf("stream = %q, want %q", got, "onetwothree")
	}
	if len(report.Gaps) != 0 {
		t.Errorf("report holds %d gaps, want none: %s", len(report.Gaps), report)
	}
}

// TestReassembleOverlappingRetransmission covers the retransmission that
// repeats some held bytes and carries new ones, which is the shape a real
// network produces and the shape a naive "drop duplicates" rule gets wrong.
func TestReassembleOverlappingRetransmission(t *testing.T) {
	file := buildTCPCapture(t, []testSegment{
		{flow: testFlow, sequence: 1, data: []byte("abcdef")},
		{flow: testFlow, sequence: 4, data: []byte("defghi")},
	})

	stream, _ := singleStream(t, file)
	if got := string(stream.Bytes); got != "abcdefghi" {
		t.Errorf("stream = %q, want %q", got, "abcdefghi")
	}
}

func TestReassembleGapFailsClosed(t *testing.T) {
	// Six bytes are missing between the two segments. Concatenating them would
	// present "abcxyz" as a contiguous stream, which it never was.
	file := buildTCPCapture(t, []testSegment{
		{flow: testFlow, sequence: 1, data: []byte("abc")},
		{flow: testFlow, sequence: 10, data: []byte("xyz")},
	})

	streams, report, err := Reassemble(bytes.NewReader(file), 179)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 2 {
		t.Fatalf("got %d streams, want 2: the hole must end the run", len(streams))
	}
	if got := string(streams[0].Bytes); got != "abc" {
		t.Errorf("first run = %q, want %q", got, "abc")
	}
	if got := string(streams[1].Bytes); got != "xyz" {
		t.Errorf("second run = %q, want %q", got, "xyz")
	}
	if len(report.Gaps) != 1 {
		t.Fatalf("report holds %d gaps, want 1: %s", len(report.Gaps), report)
	}
	if report.Gaps[0].Bytes != 6 {
		t.Errorf("gap = %d bytes, want 6", report.Gaps[0].Bytes)
	}
	if report.Gaps[0].AfterOffset != 3 {
		t.Errorf("gap sits after %d bytes, want 3", report.Gaps[0].AfterOffset)
	}
	if report.Gaps[0].Flow != testFlow {
		t.Errorf("gap flow = %s, want %s", report.Gaps[0].Flow, testFlow)
	}
}

func TestReassembleBounded(t *testing.T) {
	// Enough segments to pass FlowBytesMax. The reassembly must stop holding
	// bytes and say so, rather than growing with the file. The chunk stays
	// below 65535 so the IPv4 total-length field can state it.
	const chunk = 32 << 10
	payload := bytes.Repeat([]byte{0x5A}, chunk)
	segments := make([]testSegment, 0, FlowBytesMax/chunk+4)
	sequence := uint32(1)
	for held := 0; held < FlowBytesMax+2*chunk; held += chunk {
		segments = append(segments, testSegment{flow: testFlow, sequence: sequence, data: payload})
		sequence += chunk
	}

	file := buildTCPCapture(t, segments)
	_, report, err := Reassemble(bytes.NewReader(file), 179)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if report.BytesReassembled > FlowBytesMax {
		t.Errorf("held %d bytes, above the %d-byte bound", report.BytesReassembled, FlowBytesMax)
	}
	if len(report.Truncated) != 1 {
		t.Fatalf("report names %d truncated flows, want 1: %s", len(report.Truncated), report)
	}
	if report.Truncated[0] != testFlow {
		t.Errorf("truncated flow = %s, want %s", report.Truncated[0], testFlow)
	}
}

// TestFrameMultipleMessagesPerSegment proves one segment carrying three BGP
// messages reaches the caller as one contiguous stream, which is what lets the
// caller frame all three (AC-10).
func TestFrameMultipleMessagesPerSegment(t *testing.T) {
	three := bytes.Join([][]byte{
		bgpKeepalive(),
		bgpKeepalive(),
		bgpKeepalive(),
	}, nil)

	file := buildTCPCapture(t, []testSegment{
		{flow: testFlow, sequence: 1, data: three},
	})

	stream, _ := singleStream(t, file)
	if !bytes.Equal(stream.Bytes, three) {
		t.Errorf("stream is %d bytes, want the %d bytes of three KEEPALIVEs", len(stream.Bytes), len(three))
	}
}

// bgpKeepalive returns the 19 bytes of a BGP KEEPALIVE: the all-ones marker,
// length 19, type 4 (RFC 4271 Section 4.4).
func bgpKeepalive() []byte {
	msg := bytes.Repeat([]byte{0xFF}, 16)
	return append(msg, 0x00, 0x13, 0x04)
}

// TestReassembleAcrossThreeSegments covers AC-9: one message split over three
// segments is one message in the stream, not three and not none.
func TestReassembleAcrossThreeSegments(t *testing.T) {
	message := bgpKeepalive()
	file := buildTCPCapture(t, []testSegment{
		{flow: testFlow, sequence: 1, data: message[0:5]},
		{flow: testFlow, sequence: 6, data: message[5:12]},
		{flow: testFlow, sequence: 13, data: message[12:19]},
	})

	stream, _ := singleStream(t, file)
	if !bytes.Equal(stream.Bytes, message) {
		t.Errorf("stream = % x, want % x", stream.Bytes, message)
	}
}

// TestNonSelectedFlowsIgnored covers AC-12: traffic on other ports is skipped
// without an error, and the report still says it was seen.
func TestNonSelectedFlowsIgnored(t *testing.T) {
	other := Flow{
		SourceAddr: netip.MustParseAddr("192.0.2.3"),
		TargetAddr: netip.MustParseAddr("192.0.2.4"),
		SourcePort: 443,
		TargetPort: 40000,
	}
	file := buildTCPCapture(t, []testSegment{
		{flow: other, sequence: 1, data: []byte("tls handshake")},
		{flow: testFlow, sequence: 1, data: bgpKeepalive()},
		{flow: other, sequence: 14, data: []byte("more tls")},
	})

	stream, report := singleStream(t, file)
	if !bytes.Equal(stream.Bytes, bgpKeepalive()) {
		t.Errorf("stream = % x, want the KEEPALIVE", stream.Bytes)
	}
	if report.FlowsSeen != 2 {
		t.Errorf("flows seen = %d, want 2", report.FlowsSeen)
	}
	if report.FlowsSelected != 1 {
		t.Errorf("flows selected = %d, want 1", report.FlowsSelected)
	}
	if report.RecordsSkipped != 0 {
		t.Errorf("records skipped = %d, want 0: a flow on another port is not a parse failure", report.RecordsSkipped)
	}
}

// TestReassembleEmptyReportsWhatItSaw covers AC-14 and R-4: a capture with no
// selected traffic returns no stream AND a report naming what was examined, so
// a caller never presents silence as a successful decode.
func TestReassembleEmptyReportsWhatItSaw(t *testing.T) {
	other := testFlow
	other.SourcePort, other.TargetPort = 443, 40000
	file := buildTCPCapture(t, []testSegment{
		{flow: other, sequence: 1, data: []byte("nothing to see")},
	})

	streams, report, err := Reassemble(bytes.NewReader(file), 179)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != 0 {
		t.Fatalf("got %d streams, want none", len(streams))
	}
	if report.RecordsRead != 1 {
		t.Errorf("records read = %d, want 1", report.RecordsRead)
	}
	if report.FlowsSeen != 1 {
		t.Errorf("flows seen = %d, want 1", report.FlowsSeen)
	}
	if report.FlowsSelected != 0 {
		t.Errorf("flows selected = %d, want 0", report.FlowsSelected)
	}
}

// TestReassembleFlowBound covers R-7's second bound: past FlowMax directions,
// further ones are refused and counted rather than allocated.
func TestReassembleFlowBound(t *testing.T) {
	segments := make([]testSegment, 0, FlowMax+8)
	for i := range FlowMax + 8 {
		flow := testFlow
		flow.TargetPort = uint16(20000 + i) //nolint:gosec // bounded by the loop
		segments = append(segments, testSegment{flow: flow, sequence: 1, data: []byte("x")})
	}

	file := buildTCPCapture(t, segments)
	streams, report, err := Reassemble(bytes.NewReader(file), 179)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	if len(streams) != FlowMax {
		t.Errorf("got %d streams, want the %d-flow bound", len(streams), FlowMax)
	}
	if report.FlowsDropped != 8 {
		t.Errorf("flows dropped = %d, want 8", report.FlowsDropped)
	}
}

// TestTimestampAtNamesTheCarryingSegment proves a message's timestamp comes
// from the segment that carried its bytes, so messages from two directions sort
// into wire order rather than file order.
func TestTimestampAtNamesTheCarryingSegment(t *testing.T) {
	first := time.Unix(1_700_000_000, 0).UTC()
	second := time.Unix(1_700_000_005, 0).UTC()

	file := buildTCPCapture(t, []testSegment{
		{flow: testFlow, sequence: 1, data: []byte("abc"), stamp: first},
		{flow: testFlow, sequence: 4, data: []byte("def"), stamp: second},
	})

	stream, _ := singleStream(t, file)
	if got := stream.TimestampAt(0); !got.Equal(first) {
		t.Errorf("timestamp at offset 0 = %s, want %s", got, first)
	}
	if got := stream.TimestampAt(2); !got.Equal(first) {
		t.Errorf("timestamp at offset 2 = %s, want %s", got, first)
	}
	if got := stream.TimestampAt(3); !got.Equal(second) {
		t.Errorf("timestamp at offset 3 = %s, want %s", got, second)
	}
	if got := stream.TimestampAt(99); !got.Equal(second) {
		t.Errorf("timestamp past the end = %s, want the last segment's %s", got, second)
	}
}

// TestDissectEthernetAndCooked proves a capture from somebody else's tcpdump
// reaches the same reassembly, whichever link layer it recorded (AC-8).
func TestDissectEthernetAndCooked(t *testing.T) {
	payload := bgpKeepalive()
	seg := testSegment{flow: testFlow, sequence: 1, data: payload}
	packet := buildIPv4TCP(&seg)

	cases := []struct {
		name     string
		linkType uint32
		frame    []byte
	}{
		{"ethernet", LinkTypeEthernet, append(ethernetHeader(etherTypeIPv4), packet...)},
		{"ethernet with a VLAN tag", LinkTypeEthernet, append(vlanHeader(etherTypeIPv4), packet...)},
		{"linux cooked", LinkTypeLinuxSLL, append(cookedHeader(linuxSLLHeaderLen, 14, etherTypeIPv4), packet...)},
		{"linux cooked v2", LinkTypeLinuxSLL2, append(cookedHeader(linuxSLL2HeaderLen, 0, etherTypeIPv4), packet...)},
		{"raw", LinkTypeRaw, packet},
		{"raw IPv4", LinkTypeIPv4, packet},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dissect(tc.linkType, tc.frame)
			if err != nil {
				t.Fatalf("dissect: %v", err)
			}
			if got.flow != testFlow {
				t.Errorf("flow = %s, want %s", got.flow, testFlow)
			}
			if !bytes.Equal(got.data, payload) {
				t.Errorf("payload = % x, want % x", got.data, payload)
			}
		})
	}
}

func ethernetHeader(etherType uint16) []byte {
	hdr := make([]byte, ethernetHeaderLen)
	binary.BigEndian.PutUint16(hdr[12:14], etherType)
	return hdr
}

func vlanHeader(etherType uint16) []byte {
	hdr := make([]byte, ethernetHeaderLen+vlanTagLen)
	binary.BigEndian.PutUint16(hdr[12:14], etherTypeVLAN)
	binary.BigEndian.PutUint16(hdr[16:18], etherType)
	return hdr
}

func cookedHeader(headerLen, protocolOffset int, protocol uint16) []byte {
	hdr := make([]byte, headerLen)
	binary.BigEndian.PutUint16(hdr[protocolOffset:protocolOffset+2], protocol)
	return hdr
}

// TestDissectRefusesCraftedHeaders proves each attacker-controlled length field
// is checked before it indexes: a short IHL, a data offset past the record, and
// an IPv4 fragment are each named rather than read.
func TestDissectRefusesCraftedHeaders(t *testing.T) {
	seg := testSegment{flow: testFlow, sequence: 1, data: []byte("payload")}

	t.Run("IHL below the minimum", func(t *testing.T) {
		packet := buildIPv4TCP(&seg)
		packet[0] = 0x44 // Version 4, IHL 4 words: shorter than a header.
		if _, err := dissect(LinkTypeRaw, packet); err == nil {
			t.Fatal("dissect accepted an IHL below 5 words")
		}
	})

	t.Run("IHL past the record", func(t *testing.T) {
		packet := buildIPv4TCP(&seg)
		packet[0] = 0x4F // IHL 15 words = 60 bytes, past this packet.
		if _, err := dissect(LinkTypeRaw, packet[:24]); err == nil {
			t.Fatal("dissect accepted an IHL running past the captured bytes")
		}
	})

	t.Run("TCP data offset past the record", func(t *testing.T) {
		packet := buildIPv4TCP(&seg)
		packet[IPv4HeaderLen+12] = 0xF0 // Data offset 15 words = 60 bytes.
		if _, err := dissect(LinkTypeRaw, packet); err == nil {
			t.Fatal("dissect accepted a data offset running past the captured bytes")
		}
	})

	t.Run("TCP data offset below the minimum", func(t *testing.T) {
		packet := buildIPv4TCP(&seg)
		packet[IPv4HeaderLen+12] = 0x40 // Data offset 4 words.
		if _, err := dissect(LinkTypeRaw, packet); err == nil {
			t.Fatal("dissect accepted a data offset below 5 words")
		}
	})

	t.Run("IPv4 fragment", func(t *testing.T) {
		packet := buildIPv4TCP(&seg)
		binary.BigEndian.PutUint16(packet[6:8], 0x2000) // More Fragments.
		if _, err := dissect(LinkTypeRaw, packet); !errors.Is(err, ErrFragmented) {
			t.Fatalf("dissect of a fragment: %v, want ErrFragmented", err)
		}
	})

	t.Run("unknown link type", func(t *testing.T) {
		if _, err := dissect(9999, buildIPv4TCP(&seg)); err == nil {
			t.Fatal("dissect accepted a link type it has no dissector for")
		}
	})
}
