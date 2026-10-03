// RFC 3954 Section 4 byte order for the FlowSets the earlier x-3 units did not
// reach: the counter Template, the IPv6 flow Template, the IPv6 flow Data
// FlowSet, and the SRC_AS, DST_AS, FIRST_SWITCHED and LAST_SWITCHED fields.
//
// VALIDATES: every multi-octet integer of those FlowSets is written most
// significant octet first. Each value has distinct octets, so a little-endian
// or swapped write changes the compared bytes.
// PREVENTS: a writer the existing units never compared going out of network
// byte order unnoticed.

package netflow9

import (
	"bytes"
	"net/netip"
	"testing"
)

// templateFlowSetWant builds the expected Template FlowSet octets for one
// template: FlowSet ID 0, Length, Template ID, Field Count, then each field's
// type and length, all big-endian, padded with zeros to size.
func templateFlowSetWant(templateID uint16, fields [][2]uint16, size int) []byte {
	want := []byte{0x00, 0x00, byte(size >> 8), byte(size), byte(templateID >> 8), byte(templateID), 0x00, byte(len(fields))}
	for _, f := range fields {
		want = append(want, byte(f[0]>>8), byte(f[0]), byte(f[1]>>8), byte(f[1]))
	}
	for len(want) < size {
		want = append(want, 0)
	}
	return want
}

// RFC requirement: RFC3954-x-3 positive -- the counter Template FlowSet writes
// its FlowSet ID, Length, Template ID 256 (octets 01 00), Field Count and every
// field type and length most significant octet first.
func TestRFC3954CounterTemplateFlowSetBigEndian(t *testing.T) {
	tmpl := BuildCounterTemplate()
	if want := templateFlowSetWant(CounterTemplateID, counterFields, len(tmpl)); !bytes.Equal(tmpl, want) {
		t.Fatalf("counter Template FlowSet = %x, want %x", tmpl, want)
	}
}

// RFC requirement: RFC3954-x-3 positive -- the IPv6 flow Template FlowSet writes
// its FlowSet ID, Length, Template ID 258 (octets 01 02), Field Count and every
// field type and length most significant octet first.
func TestRFC3954IPv6TemplateFlowSetBigEndian(t *testing.T) {
	tmpl := BuildFlowTemplate6()
	if want := templateFlowSetWant(FlowTemplateID6, flowFields6, len(tmpl)); !bytes.Equal(tmpl, want) {
		t.Fatalf("IPv6 flow Template FlowSet = %x, want %x", tmpl, want)
	}
}

// flowTailRecord carries distinct octets in every integer of the shared tail.
var flowTailRecord = FlowRecord{
	SrcPort:       0x1234,
	DstPort:       0xabcd,
	Protocol:      17,
	Bytes:         0x0102030405060708,
	Packets:       0x11121314,
	SrcAS:         0x21222324,
	DstAS:         0x31323334,
	FirstSwitched: 0x41424344,
	LastSwitched:  0x51525354,
}

// flowTailWant is flowTailRecord's tail in network byte order.
var flowTailWant = []byte{
	0x12, 0x34, 0xab, 0xcd, 17,
	0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	0x11, 0x12, 0x13, 0x14,
	0x21, 0x22, 0x23, 0x24,
	0x31, 0x32, 0x33, 0x34,
	0x41, 0x42, 0x43, 0x44,
	0x51, 0x52, 0x53, 0x54,
}

// RFC requirement: RFC3954-x-3 positive -- the IPv4 flow Data FlowSet writes
// SRC_AS, DST_AS, FIRST_SWITCHED and LAST_SWITCHED, with non-zero values of
// distinct octets, most significant octet first, after the ports, IN_BYTES and
// IN_PKTS.
func TestRFC3954FlowTailBigEndian(t *testing.T) {
	var buf [128]byte
	rec := flowTailRecord
	rec.SrcAddr = netip.MustParseAddr("192.0.2.1")
	rec.DstAddr = netip.MustParseAddr("198.51.100.1")
	if _, count := writeFlowDataFlowSet(buf[:], 0, []FlowRecord{rec}); count != 1 {
		t.Fatalf("records = %d, want 1", count)
	}
	tail := buf[4+8 : 4+8+len(flowTailWant)]
	if !bytes.Equal(tail, flowTailWant) {
		t.Fatalf("IPv4 flow record tail = %x, want %x", tail, flowTailWant)
	}
}

// RFC requirement: RFC3954-x-3 positive -- the IPv6 flow Data FlowSet writes its
// FlowSet ID 258 and Length, then the record's ports, IN_BYTES, IN_PKTS, SRC_AS,
// DST_AS, FIRST_SWITCHED and LAST_SWITCHED, most significant octet first.
func TestRFC3954IPv6FlowDataFlowSetBigEndian(t *testing.T) {
	var buf [256]byte
	rec := flowTailRecord
	rec.SrcAddr = netip.MustParseAddr("2001:db8::1")
	rec.DstAddr = netip.MustParseAddr("2001:db8::2")
	n, count := writeFlowDataFlowSet6(buf[:], 0, []FlowRecord{rec})
	if count != 1 {
		t.Fatalf("records = %d, want 1", count)
	}
	if got := buf[0:4]; !bytes.Equal(got, []byte{0x01, 0x02, byte(n >> 8), byte(n)}) {
		t.Fatalf("IPv6 Data FlowSet header = %x, want ID 258 and Length %d", got, n)
	}
	tail := buf[4+32 : 4+32+len(flowTailWant)]
	if !bytes.Equal(tail, flowTailWant) {
		t.Fatalf("IPv6 flow record tail = %x, want %x", tail, flowTailWant)
	}
}
