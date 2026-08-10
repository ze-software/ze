package ipfix

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestIPFIXFlowData6(t *testing.T) {
	buf := make([]byte, 256)
	src := netip.MustParseAddr("2001:db8::1")
	dst := netip.MustParseAddr("2001:db8::2")
	flows := []FlowRecord{
		{
			SrcAddr:     src,
			DstAddr:     dst,
			SrcPort:     12345,
			DstPort:     443,
			Protocol:    6,
			Bytes:       2000,
			Packets:     20,
			SrcAS:       65001,
			DstAS:       65002,
			StartTimeMs: 1700000000000,
			EndTimeMs:   1700000060000,
		},
	}

	written, count := writeFlowDataSet6(buf, 0, flows, FlowTemplateID6)
	if count != 1 {
		t.Fatalf("record count = %d, want 1", count)
	}

	// Set ID = FlowTemplateID6 (258)
	if got := binary.BigEndian.Uint16(buf[0:]); got != FlowTemplateID6 {
		t.Fatalf("Set ID = %d, want %d", got, FlowTemplateID6)
	}

	// Set Length
	if got := binary.BigEndian.Uint16(buf[2:]); got != uint16(written) {
		t.Fatalf("Set Length = %d, want %d", got, written)
	}

	// First record at offset 4
	off := 4

	// sourceIPv6Address (16 bytes)
	srcWant := src.As16()
	if !bytes.Equal(buf[off:off+16], srcWant[:]) {
		t.Fatalf("SrcAddr 16 bytes mismatch")
	}
	off += 16

	// destinationIPv6Address (16 bytes)
	dstWant := dst.As16()
	if !bytes.Equal(buf[off:off+16], dstWant[:]) {
		t.Fatalf("DstAddr 16 bytes mismatch")
	}
	off += 16

	// sourceTransportPort
	if got := binary.BigEndian.Uint16(buf[off:]); got != 12345 {
		t.Fatalf("SrcPort = %d, want 12345", got)
	}
	off += 2

	// destinationTransportPort
	if got := binary.BigEndian.Uint16(buf[off:]); got != 443 {
		t.Fatalf("DstPort = %d, want 443", got)
	}
	off += 2

	// protocolIdentifier
	if buf[off] != 6 {
		t.Fatalf("Protocol = %d, want 6", buf[off])
	}
	off++

	// octetDeltaCount
	if got := binary.BigEndian.Uint64(buf[off:]); got != 2000 {
		t.Fatalf("Bytes = %d, want 2000", got)
	}
	off += 8

	// packetDeltaCount
	if got := binary.BigEndian.Uint64(buf[off:]); got != 20 {
		t.Fatalf("Packets = %d, want 20", got)
	}
	off += 8

	// bgpSourceAsNumber
	if got := binary.BigEndian.Uint32(buf[off:]); got != 65001 {
		t.Fatalf("SrcAS = %d, want 65001", got)
	}
	off += 4

	// bgpDestinationAsNumber
	if got := binary.BigEndian.Uint32(buf[off:]); got != 65002 {
		t.Fatalf("DstAS = %d, want 65002", got)
	}
	off += 4

	// flowStartMilliseconds
	if got := binary.BigEndian.Uint64(buf[off:]); got != 1700000000000 {
		t.Fatalf("StartTimeMs = %d, want 1700000000000", got)
	}
	off += 8

	// flowEndMilliseconds
	if got := binary.BigEndian.Uint64(buf[off:]); got != 1700000060000 {
		t.Fatalf("EndTimeMs = %d, want 1700000060000", got)
	}
}
