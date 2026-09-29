// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- NetFlow v9 export
// Related: flow_adapter_test.go -- recvDatagrams and newLoopbackEncoderTarget
//
// VALIDATES: RFC 3954 Sections 4, 5.1 and 7 on the packets the real counter and
// flow encoders put on one sender: the Template and per-flow Data FlowSets are
// big-endian, the Packet Header sequence counts every Export Packet of the
// Observation Domain, and each Template ID stays the same across refreshes.
// PREVENTS: a little-endian FlowSet field, a template-only packet left out of
// the sequence, and a Template ID that changes when the template is resent.

package netflow9

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// mixedExportRound sends, on one sender, the counter Template, the two flow
// Templates, one counter Data packet and one IPv4 flow Data packet: five Export
// Packets of one Observation Domain.
func mixedExportRound(t *testing.T, s *flowexport.Sender, counter *CounterEncoder, flow *FlowEncoder) {
	t.Helper()
	if err := counter.EncodeTemplate(s); err != nil {
		t.Fatal(err)
	}
	if err := flow.EncodeFlowTemplate(s); err != nil {
		t.Fatal(err)
	}
	snap := flowexport.CounterSnapshot{
		Time:       time.Now(),
		Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1, IfInOctets: 10}},
	}
	if n, err := counter.Encode(snap, s); err != nil || n != 1 {
		t.Fatalf("counter Encode = (%d, %v), want (1, nil)", n, err)
	}
	flows := []flowexport.ConntrackFlow{{
		SrcAddr:  netip.MustParseAddr("192.0.2.1"),
		DstAddr:  netip.MustParseAddr("198.51.100.1"),
		Protocol: 6,
		Bytes:    100,
		Packets:  2,
	}}
	if n, err := flow.EncodeFlows(flows, s); err != nil || n != 1 {
		t.Fatalf("EncodeFlows = (%d, %v), want (1, nil)", n, err)
	}
}

// RFC requirement: RFC3954-x-8 positive -- template-only packets, counter Data
// packets and flow Data packets sent by two encoders over one sender of one
// Observation Domain each take the next Packet Header sequence number: ten
// Export Packets over two rounds carry ten consecutive values, none skipped.
func TestRFC3954SequenceCountsEveryExportPacket(t *testing.T) {
	pc, s := newLoopbackEncoderTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	counter := NewCounterEncoder(0, time.Now())
	flow := NewFlowEncoder(0, time.Now())

	mixedExportRound(t, s, counter, flow)
	mixedExportRound(t, s, counter, flow)

	dgs := recvDatagrams(t, pc, 10)
	if len(dgs) != 10 {
		t.Fatalf("received %d Export Packets, want 10", len(dgs))
	}
	first := binary.BigEndian.Uint32(dgs[0][12:])
	for i, dg := range dgs {
		if got := binary.BigEndian.Uint32(dg[12:]); got != first+uint32(i) {
			t.Errorf("packet %d (FlowSet ID %d): sequence %d, want %d",
				i, binary.BigEndian.Uint16(dg[HeaderSize:]), got, first+uint32(i))
		}
	}
}

// RFC requirement: RFC3954-x-9 positive -- over two send rounds the counter
// Template and the IPv4 and IPv6 flow Templates are resent with the same
// Template IDs, 256, 257 and 258, and each Data FlowSet names the ID its
// Template carried.
func TestRFC3954TemplateIDsConstantAcrossRefresh(t *testing.T) {
	pc, s := newLoopbackEncoderTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	counter := NewCounterEncoder(0, time.Now())
	flow := NewFlowEncoder(0, time.Now())

	mixedExportRound(t, s, counter, flow)
	mixedExportRound(t, s, counter, flow)

	dgs := recvDatagrams(t, pc, 10)
	if len(dgs) != 10 {
		t.Fatalf("received %d Export Packets, want 10", len(dgs))
	}
	// Per round: three Template FlowSets (FlowSet ID 0, Template ID after the
	// 4-octet FlowSet header), then the counter and the flow Data FlowSets.
	want := []uint16{256, 257, 258, 256, 257}
	for round := range 2 {
		for i, id := range want {
			dg := dgs[round*5+i]
			got := binary.BigEndian.Uint16(dg[HeaderSize:])
			if i < 3 {
				if got != FlowSetIDTemplate {
					t.Fatalf("round %d packet %d: FlowSet ID %d, want Template FlowSet", round, i, got)
				}
				got = binary.BigEndian.Uint16(dg[HeaderSize+4:])
			}
			if got != id {
				t.Errorf("round %d packet %d: Template ID %d, want %d", round, i, got, id)
			}
		}
	}
}

// RFC requirement: RFC3954-x-3 positive -- the IPv4 flow Template FlowSet writes
// its FlowSet ID, Length, Template ID, Field Count and every field type and
// length most significant octet first.
func TestRFC3954TemplateFlowSetBigEndian(t *testing.T) {
	tmpl := BuildFlowTemplate()
	want := []byte{0x00, 0x00, byte(len(tmpl) >> 8), byte(len(tmpl)), 0x01, 0x01, 0x00, byte(len(flowFields))}
	for _, f := range flowFields {
		want = append(want, byte(f[0]>>8), byte(f[0]), byte(f[1]>>8), byte(f[1]))
	}
	for len(want) < len(tmpl) {
		want = append(want, 0)
	}
	if !bytes.Equal(tmpl, want) {
		t.Fatalf("flow Template FlowSet = %x, want %x", tmpl, want)
	}
}

// RFC requirement: RFC3954-x-3 positive -- the IPv4 flow Data FlowSet writes its
// FlowSet ID and Length, and the record's L4_SRC_PORT, L4_DST_PORT, IN_BYTES and
// IN_PKTS, most significant octet first.
func TestRFC3954FlowDataFlowSetBigEndian(t *testing.T) {
	var buf [128]byte
	recs := []FlowRecord{{
		SrcAddr:  netip.MustParseAddr("192.0.2.1"),
		DstAddr:  netip.MustParseAddr("198.51.100.1"),
		SrcPort:  0x1234,
		DstPort:  0xabcd,
		Protocol: 6,
		Bytes:    0x0102030405060708,
		Packets:  0x11121314,
	}}
	n, count := writeFlowDataFlowSet(buf[:], 0, recs)
	if count != 1 {
		t.Fatalf("records = %d, want 1", count)
	}
	if got := buf[0:4]; !bytes.Equal(got, []byte{0x01, 0x01, byte(n >> 8), byte(n)}) {
		t.Fatalf("Data FlowSet header = %x, want ID 257 and Length %d", got, n)
	}
	tail := buf[4+8 : 4+8+19] // after the FlowSet header and the two IPv4 addresses
	want := []byte{
		0x12, 0x34, 0xab, 0xcd, 0x06,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x11, 0x12, 0x13, 0x14,
	}
	if !bytes.Equal(tail[:len(want)], want) {
		t.Fatalf("flow record fields = %x, want %x", tail[:len(want)], want)
	}
}

// flowSetIDs walks the FlowSets of one Export Packet and returns, in order,
// the Template IDs its Template FlowSets define and the IDs of its Data FlowSets.
func flowSetIDs(t *testing.T, pkt []byte) (defined, data []uint16) {
	t.Helper()
	for off := HeaderSize; off+4 <= len(pkt); {
		id := binary.BigEndian.Uint16(pkt[off:])
		length := int(binary.BigEndian.Uint16(pkt[off+2:]))
		if length < 4 || off+length > len(pkt) {
			t.Fatalf("FlowSet at %d has bad length %d in a %d-octet packet", off, length, len(pkt))
		}
		if id == FlowSetIDTemplate {
			defined = append(defined, binary.BigEndian.Uint16(pkt[off+4:]))
		} else {
			data = append(data, id)
		}
		off += length
	}
	return defined, data
}

// RFC requirement: RFC3954-x-1 positive -- freshly started counter and flow
// encoders, driven in the exporter's order (Templates, then data), send every
// Data FlowSet (counter 256, IPv4 flow 257, IPv6 flow 258) only after a packet
// whose Template FlowSet defines that same Template ID. Ze emits no Options Data,
// so no Options Template is required before it.
func TestRFC3954RestartedEncodersSendTemplateBeforeData(t *testing.T) {
	pc, s := newLoopbackEncoderTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	counter := NewCounterEncoder(0, time.Now())
	flow := NewFlowEncoder(0, time.Now())
	if err := counter.EncodeTemplate(s); err != nil {
		t.Fatal(err)
	}
	if err := flow.EncodeFlowTemplate(s); err != nil {
		t.Fatal(err)
	}
	snap := flowexport.CounterSnapshot{Time: time.Now(), Interfaces: []flowexport.InterfaceCounters{{IfIndex: 1}}}
	if _, err := counter.Encode(snap, s); err != nil {
		t.Fatal(err)
	}
	flows := []flowexport.ConntrackFlow{
		{SrcAddr: netip.MustParseAddr("192.0.2.1"), DstAddr: netip.MustParseAddr("198.51.100.1"), Protocol: 6},
		{SrcAddr: netip.MustParseAddr("2001:db8::1"), DstAddr: netip.MustParseAddr("2001:db8::2"), Protocol: 6},
	}
	if _, err := flow.EncodeFlows(flows, s); err != nil {
		t.Fatal(err)
	}

	dgs := recvDatagrams(t, pc, 6)
	if len(dgs) != 6 {
		t.Fatalf("received %d Export Packets, want 6", len(dgs))
	}
	sent := make(map[uint16]bool)
	seenData := make(map[uint16]bool)
	for i, dg := range dgs {
		defined, data := flowSetIDs(t, dg)
		for _, id := range data {
			if !sent[id] {
				t.Errorf("packet %d: Data FlowSet %d sent before any Template FlowSet defining %d", i, id, id)
			}
			seenData[id] = true
		}
		for _, id := range defined {
			sent[id] = true
		}
	}
	for _, id := range []uint16{CounterTemplateID, FlowTemplateID, FlowTemplateID6} {
		if !seenData[id] {
			t.Errorf("no Data FlowSet %d was sent, so its Template ordering is unproven", id)
		}
	}
}

// RFC requirement: RFC3954-x-1 positive -- when the counter Template rides in the
// same Export Packet as its data, the Template FlowSet defining 256 comes first
// and the Data FlowSet that follows names 256.
func TestRFC3954TemplateFlowSetPrecedesItsDataInOnePacket(t *testing.T) {
	var buf [1400]byte
	ifaces := []flowexport.InterfaceCounters{{IfIndex: 1, IfInOctets: 1000}}
	n := writeExportPacket(buf[:], 5000, 1716900000, 1, 0, BuildCounterTemplate(), true, ifaces)
	defined, data := flowSetIDs(t, buf[:n])
	if len(defined) != 1 || defined[0] != CounterTemplateID {
		t.Fatalf("Template FlowSets define %v, want [%d]", defined, CounterTemplateID)
	}
	if len(data) != 1 || data[0] != CounterTemplateID {
		t.Fatalf("Data FlowSets = %v, want [%d]", data, CounterTemplateID)
	}
	if first := binary.BigEndian.Uint16(buf[HeaderSize:]); first != FlowSetIDTemplate {
		t.Fatalf("first FlowSet ID = %d, want the Template FlowSet ahead of the data", first)
	}
}
