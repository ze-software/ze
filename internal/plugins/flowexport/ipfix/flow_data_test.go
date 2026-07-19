package ipfix

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestIPFIXFlowData(t *testing.T) {
	// RFC requirement: RFC7012-x-5 positive -- each data record field is written at exactly the width declared for its IE in the template

	buf := make([]byte, 256)
	flows := []FlowRecord{
		{
			SrcAddr:     netip.MustParseAddr("192.168.1.1"),
			DstAddr:     netip.MustParseAddr("10.0.0.1"),
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

	written, count := WriteFlowDataSet(buf, 0, flows, FlowTemplateID)
	if count != 1 {
		t.Fatalf("record count = %d, want 1", count)
	}

	// Set ID = FlowTemplateID (257)
	if got := binary.BigEndian.Uint16(buf[0:]); got != FlowTemplateID {
		t.Fatalf("Set ID = %d, want %d", got, FlowTemplateID)
	}

	// Set Length
	if got := binary.BigEndian.Uint16(buf[2:]); got != uint16(written) {
		t.Fatalf("Set Length = %d, want %d", got, written)
	}

	// First record at offset 4
	off := 4

	// sourceIPv4Address
	if buf[off] != 192 || buf[off+1] != 168 || buf[off+2] != 1 || buf[off+3] != 1 {
		t.Fatalf("SrcAddr = %d.%d.%d.%d, want 192.168.1.1", buf[off], buf[off+1], buf[off+2], buf[off+3])
	}
	off += 4

	// destinationIPv4Address
	if buf[off] != 10 || buf[off+1] != 0 || buf[off+2] != 0 || buf[off+3] != 1 {
		t.Fatalf("DstAddr = %d.%d.%d.%d, want 10.0.0.1", buf[off], buf[off+1], buf[off+2], buf[off+3])
	}
	off += 4

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

func TestIPFIXFlowDataMultiple(t *testing.T) {
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

	_, count := WriteFlowDataSet(buf, 0, flows, FlowTemplateID)
	if count != 2 {
		t.Fatalf("record count = %d, want 2", count)
	}
}

func TestIPFIXFlowDataEmpty(t *testing.T) {
	buf := make([]byte, 64)
	written, count := WriteFlowDataSet(buf, 0, nil, FlowTemplateID)
	if written != 0 || count != 0 {
		t.Fatalf("empty: written=%d count=%d, want 0/0", written, count)
	}
}
