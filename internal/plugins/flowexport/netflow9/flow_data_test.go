package netflow9

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestNetflow9FlowData(t *testing.T) {
	buf := make([]byte, 256)
	flows := []FlowRecord{
		{
			SrcAddr:       netip.MustParseAddr("192.168.1.1"),
			DstAddr:       netip.MustParseAddr("10.0.0.1"),
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

	written, count := writeFlowDataFlowSet(buf, 0, flows)
	if count != 1 {
		t.Fatalf("record count = %d, want 1", count)
	}

	// FlowSet ID = FlowTemplateID (257)
	if got := binary.BigEndian.Uint16(buf[0:]); got != FlowTemplateID {
		t.Fatalf("FlowSet ID = %d, want %d", got, FlowTemplateID)
	}

	// FlowSet Length
	if got := binary.BigEndian.Uint16(buf[2:]); got != uint16(written) {
		t.Fatalf("FlowSet Length = %d, want %d", got, written)
	}

	// First record starts at offset 4 (FlowSet header)
	off := 4

	// IPV4_SRC_ADDR
	if buf[off] != 192 || buf[off+1] != 168 || buf[off+2] != 1 || buf[off+3] != 1 {
		t.Fatalf("SrcAddr = %d.%d.%d.%d, want 192.168.1.1", buf[off], buf[off+1], buf[off+2], buf[off+3])
	}
	off += 4

	// IPV4_DST_ADDR
	if buf[off] != 10 || buf[off+1] != 0 || buf[off+2] != 0 || buf[off+3] != 1 {
		t.Fatalf("DstAddr = %d.%d.%d.%d, want 10.0.0.1", buf[off], buf[off+1], buf[off+2], buf[off+3])
	}
	off += 4

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

func TestNetflow9FlowDataMultiple(t *testing.T) {
	buf := make([]byte, 512)
	flows := []FlowRecord{
		{
			SrcAddr:  netip.MustParseAddr("1.1.1.1"),
			DstAddr:  netip.MustParseAddr("2.2.2.2"),
			Protocol: 17,
			Bytes:    500,
			Packets:  5,
		},
		{
			SrcAddr:  netip.MustParseAddr("3.3.3.3"),
			DstAddr:  netip.MustParseAddr("4.4.4.4"),
			Protocol: 6,
			Bytes:    1500,
			Packets:  15,
		},
	}

	_, count := writeFlowDataFlowSet(buf, 0, flows)
	if count != 2 {
		t.Fatalf("record count = %d, want 2", count)
	}
}
