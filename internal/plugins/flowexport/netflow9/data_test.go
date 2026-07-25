package netflow9

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// RFC requirement: RFC3954-x-3 positive -- counter record integers (INPUT_SNMP, IN_BYTES, IN_PKTS, OUT_BYTES, OUT_PKTS) decode big-endian to their written values (data.go writeCounterRecord); a little-endian encode would fail these BigEndian reads.
func TestNetflow9DataFlowSet(t *testing.T) {
	buf := make([]byte, 1400)

	ifaces := []flowexport.InterfaceCounters{
		{
			IfIndex:        1,
			IfInOctets:     1000,
			IfInUcastPkts:  10,
			IfOutOctets:    2000,
			IfOutUcastPkts: 20,
		},
		{
			IfIndex:        2,
			IfInOctets:     3000,
			IfInUcastPkts:  30,
			IfOutOctets:    4000,
			IfOutUcastPkts: 40,
		},
	}

	n, count := WriteDataFlowSet(buf, 0, CounterTemplateID, ifaces)
	if count != 2 {
		t.Errorf("record count: got %d, want 2", count)
	}

	// FlowSet ID = template ID
	fsID := binary.BigEndian.Uint16(buf[0:])
	if fsID != CounterTemplateID {
		t.Errorf("FlowSet ID: got %d, want %d", fsID, CounterTemplateID)
	}

	// FlowSet length
	fsLen := binary.BigEndian.Uint16(buf[2:])
	if int(fsLen) != n {
		t.Errorf("FlowSet length: got %d, want %d", fsLen, n)
	}

	// 4-byte alignment
	if n%4 != 0 {
		t.Errorf("FlowSet size %d not 4-byte aligned", n)
	}

	// First record starts at offset 4 (after FlowSet header)
	off := FlowSetHeaderSize

	// Record 1: INPUT_SNMP = 1
	ifIdx := binary.BigEndian.Uint32(buf[off:])
	if ifIdx != 1 {
		t.Errorf("record 1 INPUT_SNMP: got %d, want 1", ifIdx)
	}

	// Record 1: IN_BYTES = 1000
	inBytes := binary.BigEndian.Uint64(buf[off+4:])
	if inBytes != 1000 {
		t.Errorf("record 1 IN_BYTES: got %d, want 1000", inBytes)
	}

	// Record 1: IN_PKTS = 10
	inPkts := binary.BigEndian.Uint32(buf[off+12:])
	if inPkts != 10 {
		t.Errorf("record 1 IN_PKTS: got %d, want 10", inPkts)
	}

	// Record 1: OUT_BYTES = 2000
	outBytes := binary.BigEndian.Uint64(buf[off+16:])
	if outBytes != 2000 {
		t.Errorf("record 1 OUT_BYTES: got %d, want 2000", outBytes)
	}

	// Record 1: OUT_PKTS = 20
	outPkts := binary.BigEndian.Uint32(buf[off+24:])
	if outPkts != 20 {
		t.Errorf("record 1 OUT_PKTS: got %d, want 20", outPkts)
	}

	// Record 1: OUTPUT_SNMP = 1 (same as INPUT_SNMP for counter export)
	outIdx := binary.BigEndian.Uint32(buf[off+28:])
	if outIdx != 1 {
		t.Errorf("record 1 OUTPUT_SNMP: got %d, want 1", outIdx)
	}

	// Record 2 starts at offset 4 + 32
	off += CounterRecordSize()
	ifIdx2 := binary.BigEndian.Uint32(buf[off:])
	if ifIdx2 != 2 {
		t.Errorf("record 2 INPUT_SNMP: got %d, want 2", ifIdx2)
	}
}

func TestNetflow9DataFlowSetSingleRecord(t *testing.T) {
	buf := make([]byte, 512)

	ifaces := []flowexport.InterfaceCounters{
		{
			IfIndex:        42,
			IfInOctets:     999,
			IfInUcastPkts:  9,
			IfOutOctets:    888,
			IfOutUcastPkts: 8,
		},
	}

	n, count := WriteDataFlowSet(buf, 0, CounterTemplateID, ifaces)
	if count != 1 {
		t.Errorf("record count: got %d, want 1", count)
	}

	// Expected: header(4) + record(32) = 36, padded to 4-byte = 36
	expectedSize := FlowSetHeaderSize + CounterRecordSize()
	if expectedSize%4 != 0 {
		expectedSize += 4 - expectedSize%4
	}
	if n != expectedSize {
		t.Errorf("FlowSet size: got %d, want %d", n, expectedSize)
	}
}

func TestNetflow9DataFlowSetOffset(t *testing.T) {
	buf := make([]byte, 1400)
	off := 20 // simulate writing after header

	ifaces := []flowexport.InterfaceCounters{
		{IfIndex: 7, IfInOctets: 100, IfOutOctets: 200},
	}

	n, count := WriteDataFlowSet(buf, off, CounterTemplateID, ifaces)
	if count != 1 {
		t.Errorf("record count: got %d, want 1", count)
	}

	// Verify FlowSet ID at the correct offset
	fsID := binary.BigEndian.Uint16(buf[off:])
	if fsID != CounterTemplateID {
		t.Errorf("FlowSet ID at offset %d: got %d, want %d", off, fsID, CounterTemplateID)
	}

	// Verify FlowSet length matches returned size
	fsLen := binary.BigEndian.Uint16(buf[off+2:])
	if int(fsLen) != n {
		t.Errorf("FlowSet length: got %d, want %d", fsLen, n)
	}
}

func TestNetflow9DataFlowSetEmpty(t *testing.T) {
	buf := make([]byte, 512)

	n, count := WriteDataFlowSet(buf, 0, CounterTemplateID, nil)
	if count != 0 {
		t.Errorf("record count for empty: got %d, want 0", count)
	}

	// Should still have the FlowSet header (4 bytes), padded
	if n < FlowSetHeaderSize {
		t.Errorf("FlowSet size for empty: got %d, want >= %d", n, FlowSetHeaderSize)
	}
}
