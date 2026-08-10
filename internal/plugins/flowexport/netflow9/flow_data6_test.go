package netflow9

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestNetflow9FlowData6(t *testing.T) {
	buf := make([]byte, 256)
	src := netip.MustParseAddr("2001:db8::1")
	dst := netip.MustParseAddr("2001:db8::2")
	flows := []FlowRecord{
		{
			SrcAddr:       src,
			DstAddr:       dst,
			SrcPort:       12345,
			DstPort:       80,
			Protocol:      6, // TCP
			Bytes:         1000,
			Packets:       10,
			SrcAS:         65001,
			DstAS:         65002,
			FirstSwitched: 100000,
			LastSwitched:  200000,
		},
	}

	written, count := writeFlowDataFlowSet6(buf, 0, flows)
	if count != 1 {
		t.Fatalf("record count = %d, want 1", count)
	}

	// FlowSet ID = FlowTemplateID6 (258)
	if got := binary.BigEndian.Uint16(buf[0:]); got != FlowTemplateID6 {
		t.Fatalf("FlowSet ID = %d, want %d", got, FlowTemplateID6)
	}

	// FlowSet Length
	if got := binary.BigEndian.Uint16(buf[2:]); got != uint16(written) {
		t.Fatalf("FlowSet Length = %d, want %d", got, written)
	}

	// First record starts at offset 4 (FlowSet header)
	off := 4

	// IPV6_SRC_ADDR (16 bytes)
	srcWant := src.As16()
	if !bytes.Equal(buf[off:off+16], srcWant[:]) {
		t.Fatalf("SrcAddr 16 bytes mismatch")
	}
	off += 16

	// IPV6_DST_ADDR (16 bytes)
	dstWant := dst.As16()
	if !bytes.Equal(buf[off:off+16], dstWant[:]) {
		t.Fatalf("DstAddr 16 bytes mismatch")
	}
	off += 16

	// L4_SRC_PORT
	if got := binary.BigEndian.Uint16(buf[off:]); got != 12345 {
		t.Fatalf("SrcPort = %d, want 12345", got)
	}
	off += 2

	// L4_DST_PORT
	if got := binary.BigEndian.Uint16(buf[off:]); got != 80 {
		t.Fatalf("DstPort = %d, want 80", got)
	}
	off += 2

	// PROTOCOL
	if buf[off] != 6 {
		t.Fatalf("Protocol = %d, want 6", buf[off])
	}
	off++

	// IN_BYTES
	if got := binary.BigEndian.Uint64(buf[off:]); got != 1000 {
		t.Fatalf("Bytes = %d, want 1000", got)
	}
	off += 8

	// IN_PKTS
	if got := binary.BigEndian.Uint32(buf[off:]); got != 10 {
		t.Fatalf("Packets = %d, want 10", got)
	}
	off += 4

	// SRC_AS
	if got := binary.BigEndian.Uint32(buf[off:]); got != 65001 {
		t.Fatalf("SrcAS = %d, want 65001", got)
	}
	off += 4

	// DST_AS
	if got := binary.BigEndian.Uint32(buf[off:]); got != 65002 {
		t.Fatalf("DstAS = %d, want 65002", got)
	}
	off += 4

	// FIRST_SWITCHED
	if got := binary.BigEndian.Uint32(buf[off:]); got != 100000 {
		t.Fatalf("FirstSwitched = %d, want 100000", got)
	}
	off += 4

	// LAST_SWITCHED
	if got := binary.BigEndian.Uint32(buf[off:]); got != 200000 {
		t.Fatalf("LastSwitched = %d, want 200000", got)
	}
}
