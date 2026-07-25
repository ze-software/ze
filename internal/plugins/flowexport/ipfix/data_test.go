package ipfix

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func TestIPFIXDataSet(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{
		{
			IfIndex:       42,
			IfInOctets:    1000,
			IfOutOctets:   2000,
			IfInUcastPkts: 10, IfInMulticastPkts: 2, IfInBroadcastPkts: 1,
			IfOutUcastPkts: 20, IfOutMulticastPkts: 3, IfOutBroadcastPkts: 1,
		},
	}

	var buf [256]byte
	n, count := WriteDataSet(buf[:], 0, CounterTemplateID, ifaces, 1716000000, 1716000020)

	if count != 1 {
		t.Errorf("record count = %d, want 1", count)
	}

	// Set ID = template ID (256).
	// RFC 7011 Section 3.4.3: Data Set ID equals the Template ID.
	setID := binary.BigEndian.Uint16(buf[0:])
	if setID != CounterTemplateID {
		t.Errorf("Set ID = %d, want %d", setID, CounterTemplateID)
	}

	// Set Length includes header (4) and records.
	setLength := binary.BigEndian.Uint16(buf[2:])
	if int(setLength) != n {
		t.Errorf("Set Length = %d, actual written = %d", setLength, n)
	}

	recOff := 4 // after set header

	// IE 10: ingressInterface = 42.
	ifIdx := binary.BigEndian.Uint32(buf[recOff:])
	if ifIdx != 42 {
		t.Errorf("ingressInterface = %d, want 42", ifIdx)
	}
	recOff += 4

	// IE 85: octetTotalCount = 1000 + 2000 = 3000.
	octets := binary.BigEndian.Uint64(buf[recOff:])
	if octets != 3000 {
		t.Errorf("octetTotalCount = %d, want 3000", octets)
	}
	recOff += 8

	// IE 86: packetTotalCount = 10+2+1+20+3+1 = 37.
	pkts := binary.BigEndian.Uint64(buf[recOff:])
	if pkts != 37 {
		t.Errorf("packetTotalCount = %d, want 37", pkts)
	}
	recOff += 8

	// IE 14: egressInterface = 42.
	egress := binary.BigEndian.Uint32(buf[recOff:])
	if egress != 42 {
		t.Errorf("egressInterface = %d, want 42", egress)
	}
	recOff += 4

	// IE 150: flowStartSeconds.
	start := binary.BigEndian.Uint32(buf[recOff:])
	if start != 1716000000 {
		t.Errorf("flowStartSeconds = %d, want 1716000000", start)
	}
	recOff += 4

	// IE 151: flowEndSeconds.
	end := binary.BigEndian.Uint32(buf[recOff:])
	if end != 1716000020 {
		t.Errorf("flowEndSeconds = %d, want 1716000020", end)
	}
}

func TestIPFIXDataSetMultiRecord(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{
		{IfIndex: 1, IfInOctets: 100, IfOutOctets: 200},
		{IfIndex: 2, IfInOctets: 300, IfOutOctets: 400},
		{IfIndex: 3, IfInOctets: 500, IfOutOctets: 600},
	}

	var buf [512]byte
	n, count := WriteDataSet(buf[:], 0, CounterTemplateID, ifaces, 1000, 1020)

	if count != 3 {
		t.Errorf("record count = %d, want 3", count)
	}

	// Set header (4) + 3 records x 32 bytes = 100 bytes. 100 % 4 == 0, no padding.
	expected := 4 + 3*CounterRecordSize()
	if n != expected {
		t.Errorf("bytes written = %d, want %d", n, expected)
	}

	// Verify each record has the right ifIndex.
	recSize := CounterRecordSize()
	for i, ifc := range ifaces {
		recOff := 4 + i*recSize
		ifIdx := binary.BigEndian.Uint32(buf[recOff:])
		if ifIdx != ifc.IfIndex {
			t.Errorf("record %d: ingressInterface = %d, want %d", i, ifIdx, ifc.IfIndex)
		}
	}
}

func TestIPFIXDataSetPadding(t *testing.T) {
	// CounterRecordSize() = 32 (divisible by 4), so with any number of
	// records plus the 4-byte header, the total is always 4-byte aligned.
	// Verify no extra padding is added in the aligned case.
	ifaces := []flowexport.InterfaceCounters{
		{IfIndex: 1},
	}

	var buf [256]byte
	n, _ := WriteDataSet(buf[:], 0, CounterTemplateID, ifaces, 1000, 1020)

	// 4 (header) + 32 (record) = 36. 36 % 4 == 0, so no padding.
	expected := 4 + CounterRecordSize()
	if n != expected {
		t.Errorf("bytes written = %d, want %d (no padding needed)", n, expected)
	}
}

func TestIPFIXDataSetEmpty(t *testing.T) {
	var buf [256]byte
	n, count := WriteDataSet(buf[:], 0, CounterTemplateID, nil, 1000, 1020)

	if n != 0 {
		t.Errorf("empty: bytes written = %d, want 0", n)
	}
	if count != 0 {
		t.Errorf("empty: record count = %d, want 0", count)
	}
}
