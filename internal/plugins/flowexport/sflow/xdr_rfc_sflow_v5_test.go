// sFlow v5 Section 5 XDR encoding for the structures TestSFlowV5FlowDatagramIsXDR
// does not compare: the counters_sample wrapper, every word of the flow_sample,
// and the extended_gateway record with its variable-length arrays and next-hop
// union.
//
// VALIDATES: each structure is a sequence of big-endian 32-bit XDR words, a
// variable-length array is its count word followed by its elements, a union is
// its discriminant word followed by the arm, and every length is a multiple of 4.
// Values carry distinct octets, so a little-endian or reordered write changes
// the compared bytes.
// PREVENTS: a writer the earlier XDR units never compared leaving the format.

package sflow

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// xdrWords renders 32-bit words as XDR (big-endian) octets.
func xdrWords(words ...uint32) []byte {
	out := make([]byte, 4*len(words))
	for i, w := range words {
		binary.BigEndian.PutUint32(out[4*i:], w)
	}
	return out
}

// RFC requirement: SFLOW-V5-x-7 positive -- the counters_sample wrapper is written
// as XDR words: tag 4 (expanded), sample_length (octets after the field, a
// multiple of 4), sequence_number, source_id type and index, and the record
// count 1, followed by the if_counters record tag.
func TestSFlowV5CounterSampleWrapperIsXDR(t *testing.T) {
	var buf [512]byte
	c := flowexport.InterfaceCounters{IfIndex: 0x01020304}
	n := writeCounterSample(buf[:], 0, 0x01020304, 0x0a0b0c0d, &c)
	if n%4 != 0 {
		t.Fatalf("counters_sample length %d is not a multiple of 4", n)
	}
	want := xdrWords(DataFormatCountersSampleExpanded, uint32(n-8), 0x0a0b0c0d, 0, 0x01020304, 1, DataFormatIfCounters)
	if got := buf[:len(want)]; !bytes.Equal(got, want) {
		t.Fatalf("counters_sample wrapper = %x, want %x", got, want)
	}
}

// RFC requirement: SFLOW-V5-x-7 positive -- every word of the expanded
// flow_sample is written as an XDR word: tag 3, sample_length, sequence_number,
// source_id type and index, sampling_rate, sample_pool, drops, input format
// and index, output format and index, and the flow_records count.
func TestSFlowV5FlowSampleWordsAreXDR(t *testing.T) {
	var buf [128]byte
	off, lengthOff, recordsOff := writeFlowSample(buf[:], 0,
		0x11121314, 0x21222324, 0x31323334, 0x41424344, 0x51525354, 0x61626364, 0x71727374)
	backfillFlowSample(buf[:], lengthOff, recordsOff, off, 0x0badcafe)
	want := xdrWords(DataFormatFlowSampleExpanded, uint32(off-8), 0x11121314, 0, 0x21222324,
		0x31323334, 0x41424344, 0x51525354, 0, 0x61626364, 0, 0x71727374, 0x0badcafe)
	if got := buf[:off]; !bytes.Equal(got, want) {
		t.Fatalf("flow_sample = %x, want %x", got, want)
	}
}

// RFC requirement: SFLOW-V5-x-7 positive -- the extended_gateway record is XDR:
// the next-hop address union is its type word then 4 (IPv4) or 16 (IPv6)
// octets, the AS words follow, dst_as_path is a segment count then each
// segment's type, ASN count and ASNs, communities is a count then its words,
// and the record ends with localpref; record_length counts the octets after it.
func TestSFlowV5ExtendedGatewayIsXDR(t *testing.T) {
	var buf [256]byte
	n := writeExtendedGateway(buf[:], 0, netip.MustParseAddr("192.0.2.1"),
		0x01020304, 0x11121314, 0x21222324, []uint32{0x31323334, 0x41424344}, []uint32{0x51525354}, 0x61626364)
	want := xdrWords(DataFormatExtendedGateway, uint32(n-8), AddressTypeIPv4, 0xc0000201,
		0x01020304, 0x11121314, 0x21222324,
		1, 2, 2, 0x31323334, 0x41424344,
		1, 0x51525354,
		0x61626364)
	if got := buf[:n]; !bytes.Equal(got, want) {
		t.Fatalf("extended_gateway IPv4 = %x, want %x", got, want)
	}

	n = writeExtendedGateway(buf[:], 0, netip.MustParseAddr("2001:db8::1"), 1, 2, 3, nil, nil, 4)
	want = xdrWords(DataFormatExtendedGateway, uint32(n-8), AddressTypeIPv6, 0x20010db8, 0, 0, 1, 1, 2, 3, 0, 0, 4)
	if got := buf[:n]; !bytes.Equal(got, want) {
		t.Fatalf("extended_gateway IPv6, empty arrays = %x, want %x", got, want)
	}
}
