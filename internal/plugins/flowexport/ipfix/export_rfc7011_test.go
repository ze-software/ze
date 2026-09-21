// RFC 7011 conformance tests for the IPFIX exporter's Set composition, data
// type encoding and Template sequencing. Each test is bound to a Compliance
// Checklist requirement in rfc/short/rfc7011.md via an `RFC requirement:` tag
// scanned by internal/le/rfc/tags.go.
//
// VALIDATES: one record type per Set, integral and address IEs in network byte
// order at their full width, the Template Set ahead of the Data Set it
// describes, and a Data message never stamped before its Template message.
// PREVENTS: an IPv6 flow leaking into the IPv4 Set, a little-endian or
// mapped-address field, a Data Set encoded before its Template, and the
// one-second inversion a snapshot taken before the Template send produced.

package ipfix

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// captureSender opens a loopback UDP listener and a Sender dialing it, so a test
// reads the exact datagrams the encoder put on the wire.
func captureSender(t *testing.T) (*flowexport.Sender, net.PacketConn) {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, pc
}

// readDatagram returns the next datagram the listener holds, or fails after
// two seconds so a missing send is a red rather than a hang.
func readDatagram(t *testing.T, pc net.PacketConn) []byte {
	t.Helper()
	buf := make([]byte, 2*flowexport.MaxDatagramSize)
	if err := pc.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatalf("no datagram arrived: %v", err)
	}
	return buf[:n]
}

// exportTimeOf reads the Export Time field of an IPFIX message header.
func exportTimeOf(msg []byte) uint32 {
	return binary.BigEndian.Uint32(msg[4:])
}

// firstSet returns the Set ID and Set Length of the first Set after the header.
func firstSet(t *testing.T, msg []byte) (id, length int) {
	t.Helper()
	if len(msg) < MessageHeaderSize+4 {
		t.Fatalf("message of %d octets carries no Set", len(msg))
	}
	return int(binary.BigEndian.Uint16(msg[MessageHeaderSize:])),
		int(binary.BigEndian.Uint16(msg[MessageHeaderSize+2:]))
}

// mixedFlows is one IPv4 and one IPv6 conntrack flow in a single batch.
func mixedFlows() []flowexport.ConntrackFlow {
	return []flowexport.ConntrackFlow{
		{SrcAddr: netip.MustParseAddr("10.1.2.3"), DstAddr: netip.MustParseAddr("10.4.5.6"), SrcPort: 1, DstPort: 2, Protocol: 6, Bytes: 10, Packets: 1},
		{SrcAddr: netip.MustParseAddr("2001:db8::1"), DstAddr: netip.MustParseAddr("2001:db8::2"), SrcPort: 3, DstPort: 4, Protocol: 17, Bytes: 20, Packets: 2},
	}
}

// RFC requirement: RFC7011-3.3.1-3 positive -- a batch holding one IPv4 and one
// IPv6 flow leaves EncodeFlows as two messages, Set 257 whose length is exactly
// one 53-octet IPv4 record plus header and padding, and Set 258 whose length is
// exactly one 77-octet IPv6 record plus header and padding.
func TestRFC7011OneRecordTypePerSet(t *testing.T) {
	s, pc := captureSender(t)
	enc := NewFlowEncoder(1)
	n, err := enc.EncodeFlows(mixedFlows(), s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("records exported = %d, want 2", n)
	}
	v4 := readDatagram(t, pc)
	v6 := readDatagram(t, pc)
	id, length := firstSet(t, v4)
	if id != FlowTemplateID {
		t.Fatalf("first message Set ID = %d, want %d", id, FlowTemplateID)
	}
	if want := 4 + FlowRecordSize() + 3; length != want {
		t.Fatalf("IPv4 Set length = %d, want %d (one 53-octet record)", length, want)
	}
	id, length = firstSet(t, v6)
	if id != FlowTemplateID6 {
		t.Fatalf("second message Set ID = %d, want %d", id, FlowTemplateID6)
	}
	if want := 4 + FlowRecordSize6() + 3; length != want {
		t.Fatalf("IPv6 Set length = %d, want %d (one 77-octet record)", length, want)
	}
}

// RFC requirement: RFC7011-3.3.1-3 negative -- the IPv6 flow's 16-octet source
// address never appears inside Set 257, and the IPv4 flow's 4-octet source
// address never appears inside Set 258, so neither Set carries the other
// template's record type.
func TestRFC7011RecordTypesNotMixedInSet(t *testing.T) {
	s, pc := captureSender(t)
	enc := NewFlowEncoder(1)
	if _, err := enc.EncodeFlows(mixedFlows(), s); err != nil {
		t.Fatal(err)
	}
	v4 := readDatagram(t, pc)
	v6 := readDatagram(t, pc)
	_, l4 := firstSet(t, v4)
	_, l6 := firstSet(t, v6)
	set4 := v4[MessageHeaderSize : MessageHeaderSize+l4]
	set6 := v6[MessageHeaderSize : MessageHeaderSize+l6]
	src6 := netip.MustParseAddr("2001:db8::1").As16()
	if bytes.Contains(set4, src6[:]) {
		t.Fatalf("Set %d carries an IPv6 record: %x", FlowTemplateID, set4)
	}
	src4 := netip.MustParseAddr("10.1.2.3").As4()
	if bytes.Contains(set6, src4[:]) {
		t.Fatalf("Set %d carries an IPv4 record: %x", FlowTemplateID6, set6)
	}
}

// oneFlow is a single IPv4 record with values whose byte order is visible.
func oneFlow() []FlowRecord {
	return []FlowRecord{{
		SrcAddr:     netip.MustParseAddr("10.1.2.3"),
		DstAddr:     netip.MustParseAddr("10.4.5.6"),
		SrcPort:     0x1234,
		DstPort:     0xabcd,
		Protocol:    6,
		Bytes:       0x0102030405060708,
		Packets:     0x1112131415161718,
		SrcAS:       0x21222324,
		DstAS:       0x31323334,
		StartTimeMs: 0x4142434445464748,
		EndTimeMs:   0x5152535455565758,
	}}
}

// RFC requirement: RFC7011-6.1.1-1 positive -- the unsigned16 ports, unsigned8
// protocol, unsigned64 counters and timestamps, and unsigned32 AS numbers of a
// flow record are written most significant octet first, so the record's tail
// after the two addresses is the big-endian concatenation of its fields.
func TestRFC7011IntegralNetworkByteOrder(t *testing.T) {
	var buf [256]byte
	n, count := WriteFlowDataSet(buf[:], 0, oneFlow(), FlowTemplateID)
	if count != 1 {
		t.Fatalf("records = %d, want 1", count)
	}
	tail := buf[4+8 : n-3] // after Set header and the two 4-octet addresses, before padding
	want := []byte{
		0x12, 0x34, 0xab, 0xcd, 0x06,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
		0x21, 0x22, 0x23, 0x24, 0x31, 0x32, 0x33, 0x34,
		0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48,
		0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58,
	}
	if !bytes.Equal(tail, want) {
		t.Fatalf("record tail = %x, want %x", tail, want)
	}
}

// RFC requirement: RFC7011-6.1.1-1 negative -- no integral field of the record
// appears in little-endian order: the byte-reversed sourceTransportPort and the
// byte-reversed octetDeltaCount are both absent from the encoded record.
func TestRFC7011IntegralNeverLittleEndian(t *testing.T) {
	var buf [256]byte
	n, _ := WriteFlowDataSet(buf[:], 0, oneFlow(), FlowTemplateID)
	rec := buf[4:n]
	if bytes.Contains(rec, []byte{0x34, 0x12}) {
		t.Fatalf("little-endian port 0x1234 found in record %x", rec)
	}
	if bytes.Contains(rec, []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}) {
		t.Fatalf("little-endian octetDeltaCount found in record %x", rec)
	}
}

// RFC requirement: RFC7011-6.1.2-1 positive -- an IPv4 record opens with the
// 4-octet source and destination addresses and an IPv6 record with the 16-octet
// ones, each in network byte order, matching the 4 and 16 the templates declare.
func TestRFC7011AddressOctetsNetworkByteOrder(t *testing.T) {
	var buf [256]byte
	n, _ := WriteFlowDataSet(buf[:], 0, oneFlow(), FlowTemplateID)
	if got, want := buf[4:12], []byte{10, 1, 2, 3, 10, 4, 5, 6}; !bytes.Equal(got, want) {
		t.Fatalf("IPv4 addresses = %x, want %x", got, want)
	}
	if n-3 != 4+FlowRecordSize() {
		t.Fatalf("IPv4 Set payload = %d, want %d", n-3, 4+FlowRecordSize())
	}
	six := oneFlow()
	six[0].SrcAddr = netip.MustParseAddr("2001:db8::1")
	six[0].DstAddr = netip.MustParseAddr("2001:db8::2")
	n, _ = writeFlowDataSet6(buf[:], 0, six, FlowTemplateID6)
	src := six[0].SrcAddr.As16()
	dst := six[0].DstAddr.As16()
	if got := buf[4:20]; !bytes.Equal(got, src[:]) {
		t.Fatalf("IPv6 source = %x, want %x", got, src)
	}
	if got := buf[20:36]; !bytes.Equal(got, dst[:]) {
		t.Fatalf("IPv6 destination = %x, want %x", got, dst)
	}
	if n-3 != 4+FlowRecordSize6() {
		t.Fatalf("IPv6 Set payload = %d, want %d", n-3, 4+FlowRecordSize6())
	}
}

// RFC requirement: RFC7011-6.1.2-1 negative -- an IPv4 flow is never widened to
// a 16-octet address: its record is 53 octets, not 77, and the IPv4-mapped form
// ::ffff:10.1.2.3 does not appear in the Set.
func TestRFC7011IPv4NeverWidenedToSixteenOctets(t *testing.T) {
	var buf [256]byte
	n, _ := WriteFlowDataSet(buf[:], 0, oneFlow(), FlowTemplateID)
	if n-3 == 4+FlowRecordSize6() {
		t.Fatalf("IPv4 record encoded at the IPv6 width (%d octets)", FlowRecordSize6())
	}
	mapped := netip.AddrFrom16(netip.MustParseAddr("10.1.2.3").As16()).As16()
	if bytes.Contains(buf[:n], mapped[:]) {
		t.Fatalf("IPv4-mapped 16-octet address found in %x", buf[:n])
	}
}

// RFC requirement: RFC7011-8.2-1 positive -- each Template message carries the
// wall-clock second of its send as Export Time, so the collector can order
// Template actions by it.
func TestRFC7011TemplateExportTimeIsSendTime(t *testing.T) {
	s, pc := captureSender(t)
	enc := NewFlowEncoder(1)
	before := uint32(time.Now().Unix())
	if err := enc.EncodeFlowTemplate(s); err != nil {
		t.Fatal(err)
	}
	after := uint32(time.Now().Unix())
	for range 2 {
		got := exportTimeOf(readDatagram(t, pc))
		if got < before || got > after {
			t.Fatalf("Template Export Time = %d, want within [%d, %d]", got, before, after)
		}
	}
}

// RFC requirement: RFC7011-8.2-1 negative -- a later Template message never
// carries an Export Time earlier than the one before it, so the sequence of
// Template actions cannot invert.
func TestRFC7011TemplateExportTimeNeverRegresses(t *testing.T) {
	s, pc := captureSender(t)
	enc := NewFlowEncoder(1)
	var last uint32
	for round := range 3 {
		if err := enc.EncodeFlowTemplate(s); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			got := exportTimeOf(readDatagram(t, pc))
			if got < last {
				t.Fatalf("round %d: Template Export Time %d is before the previous %d", round, got, last)
			}
			last = got
		}
	}
}

// RFC requirement: RFC7011-8.2-2 positive -- a counter snapshot taken at the
// Template's own second is exported with an Export Time equal to the Template
// message's Export Time, never before it.
func TestRFC7011DataExportTimeNotBeforeTemplate(t *testing.T) {
	s, pc := captureSender(t)
	enc := NewCounterEncoder(1)
	if err := enc.EncodeTemplate(s); err != nil {
		t.Fatal(err)
	}
	tmplTime := exportTimeOf(readDatagram(t, pc))
	snap := flowexport.CounterSnapshot{
		Time:       time.Unix(int64(tmplTime), 0),
		Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}},
	}
	if _, err := enc.Encode(snap, s); err != nil {
		t.Fatal(err)
	}
	if got := exportTimeOf(readDatagram(t, pc)); got != tmplTime {
		t.Fatalf("Data Export Time = %d, want the Template's %d", got, tmplTime)
	}
}

// RFC requirement: RFC7011-8.2-2 negative -- a snapshot whose time sits five
// seconds before the Template message is not exported at that earlier time: the
// Data message's Export Time is clamped to the Template's, so no Data Set is
// ever stamped before the Template describing it.
func TestRFC7011StaleSnapshotClampedToTemplateTime(t *testing.T) {
	s, pc := captureSender(t)
	enc := NewCounterEncoder(1)
	if err := enc.EncodeTemplate(s); err != nil {
		t.Fatal(err)
	}
	tmplTime := exportTimeOf(readDatagram(t, pc))
	snap := flowexport.CounterSnapshot{
		Time:       time.Unix(int64(tmplTime)-5, 0),
		Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}},
	}
	if _, err := enc.Encode(snap, s); err != nil {
		t.Fatal(err)
	}
	got := exportTimeOf(readDatagram(t, pc))
	if got < tmplTime {
		t.Fatalf("Data Export Time %d is before the Template's %d", got, tmplTime)
	}
	if got != tmplTime {
		t.Fatalf("Data Export Time = %d, want clamped to %d", got, tmplTime)
	}
}

// RFC requirement: RFC7011-8.2-3 positive -- a message built with its Template
// carries the Template Set (ID 2) as the first Set and the Data Set (ID 256)
// as the second, so the Template precedes the Data Set that references it.
func TestRFC7011TemplateSetPrecedesDataSet(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{{IfIndex: 1}}
	var buf [1400]byte
	n, _ := WriteMessage(buf[:], 1716000000, 0, 1, BuildCounterTemplate(), true, ifaces, 1000, 1020)
	id, length := firstSet(t, buf[:n])
	if id != TemplateSetID {
		t.Fatalf("first Set ID = %d, want %d", id, TemplateSetID)
	}
	second := int(binary.BigEndian.Uint16(buf[MessageHeaderSize+length:]))
	if second != CounterTemplateID {
		t.Fatalf("second Set ID = %d, want %d", second, CounterTemplateID)
	}
}

// RFC requirement: RFC7011-8.2-3 negative -- when the Template rides in the
// message, the Data Set (ID 256) is never the first Set: the octets at the
// first Set position are the Template Set header, not a Data Set header.
func TestRFC7011DataSetNeverPrecedesTemplate(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{{IfIndex: 1}}
	var buf [1400]byte
	n, _ := WriteMessage(buf[:], 1716000000, 0, 1, BuildCounterTemplate(), true, ifaces, 1000, 1020)
	id, _ := firstSet(t, buf[:n])
	if id == CounterTemplateID {
		t.Fatalf("Data Set %d appears before the Template Set", CounterTemplateID)
	}
	// Walk the Sets: the Data Set's position must be after the Template Set's.
	dataAt, tmplAt := -1, -1
	for off := MessageHeaderSize; off+4 <= n; {
		switch int(binary.BigEndian.Uint16(buf[off:])) {
		case TemplateSetID:
			tmplAt = off
		case CounterTemplateID:
			dataAt = off
		}
		off += int(binary.BigEndian.Uint16(buf[off+2:]))
	}
	if dataAt < 0 || tmplAt < 0 || dataAt < tmplAt {
		t.Fatalf("Data Set at %d, Template Set at %d", dataAt, tmplAt)
	}
}

// TestFlowBatchSplitAcrossDatagrams pins the chunking fix: a batch of IPv4
// flows larger than one datagram is split, every datagram stays within
// MaxDatagramSize, and every record arrives.
func TestFlowBatchSplitAcrossDatagrams(t *testing.T) {
	s, pc := captureSender(t)
	enc := NewFlowEncoder(1)
	const flows = 100
	batch := make([]flowexport.ConntrackFlow, flows)
	for i := range batch {
		batch[i] = flowexport.ConntrackFlow{
			SrcAddr: netip.MustParseAddr("10.0.0.1"), DstAddr: netip.MustParseAddr("10.0.0.2"),
			SrcPort: uint16(i), DstPort: 80, Protocol: 6, Bytes: 1, Packets: 1,
		}
	}
	n, err := enc.EncodeFlows(batch, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != flows {
		t.Fatalf("records exported = %d, want %d", n, flows)
	}
	datagrams, _, _ := s.Stats()
	if datagrams < 2 {
		t.Fatalf("datagrams = %d, want the batch split across at least 2", datagrams)
	}
	var seen uint32
	for range datagrams {
		msg := readDatagram(t, pc)
		if len(msg) > flowexport.MaxDatagramSize {
			t.Fatalf("datagram of %d octets exceeds %d", len(msg), flowexport.MaxDatagramSize)
		}
		_, length := firstSet(t, msg)
		seen += uint32((length - 4) / FlowRecordSize())
	}
	if seen != flows {
		t.Fatalf("records on the wire = %d, want %d", seen, flows)
	}
}
