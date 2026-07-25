package sflow

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func testCounters() *flowexport.InterfaceCounters {
	return &flowexport.InterfaceCounters{
		IfIndex:            7,
		IfType:             6, // ethernetCsmacd
		IfSpeed:            1000000000,
		IfDirection:        1, // full-duplex
		IfStatus:           3, // admin up + oper up
		IfInOctets:         123456789,
		IfInUcastPkts:      1000,
		IfInMulticastPkts:  50,
		IfInBroadcastPkts:  10,
		IfInDiscards:       2,
		IfInErrors:         1,
		IfInUnknownProtos:  0,
		IfOutOctets:        987654321,
		IfOutUcastPkts:     2000,
		IfOutMulticastPkts: 30,
		IfOutBroadcastPkts: 5,
		IfOutDiscards:      0,
		IfOutErrors:        0,
		IfPromiscuousMode:  0,
		Name:               "eth0",
	}
}

// RFC requirement: SFLOW-V5-x-7 positive -- every if_counters field is written XDR big-endian at its 4-byte-aligned offset: the test decodes each field with binary.BigEndian at fixed offsets (base+0, +4, +8, ...) and a little-endian or misaligned write would fail these reads (counter.go:71-156).
func TestSFlowIfCounters(t *testing.T) {
	buf := make([]byte, 256)
	c := testCounters()

	off := WriteIfCounters(buf, 0, c)

	// Total size: record_data_format(4) + record_length(4) + 88 = 96
	expectedOff := ifCountersRecordHeaderSize + flowexport.IfCountersSize
	if off != expectedOff {
		t.Fatalf("expected offset %d, got %d", expectedOff, off)
	}

	// record_data_format = 0x00000001 (if_counters)
	if v := binary.BigEndian.Uint32(buf[0:]); v != DataFormatIfCounters {
		t.Errorf("record_data_format: expected 0x%08x, got 0x%08x", DataFormatIfCounters, v)
	}
	// record_length = 88
	if v := binary.BigEndian.Uint32(buf[4:]); v != flowexport.IfCountersSize {
		t.Errorf("record_length: expected %d, got %d", flowexport.IfCountersSize, v)
	}

	base := 8 // after record header
	// 1. ifIndex = 7
	if v := binary.BigEndian.Uint32(buf[base:]); v != 7 {
		t.Errorf("ifIndex: expected 7, got %d", v)
	}
	// 2. ifType = 6
	if v := binary.BigEndian.Uint32(buf[base+4:]); v != 6 {
		t.Errorf("ifType: expected 6, got %d", v)
	}
	// 3. ifSpeed = 1000000000 (1 Gbps)
	if v := binary.BigEndian.Uint64(buf[base+8:]); v != 1000000000 {
		t.Errorf("ifSpeed: expected 1000000000, got %d", v)
	}
	// 4. ifDirection = 1 (full-duplex)
	if v := binary.BigEndian.Uint32(buf[base+16:]); v != 1 {
		t.Errorf("ifDirection: expected 1, got %d", v)
	}
	// 5. ifStatus = 3 (admin up + oper up)
	if v := binary.BigEndian.Uint32(buf[base+20:]); v != 3 {
		t.Errorf("ifStatus: expected 3, got %d", v)
	}
	// 6. ifInOctets = 123456789
	if v := binary.BigEndian.Uint64(buf[base+24:]); v != 123456789 {
		t.Errorf("ifInOctets: expected 123456789, got %d", v)
	}
	// 7. ifInUcastPkts = 1000
	if v := binary.BigEndian.Uint32(buf[base+32:]); v != 1000 {
		t.Errorf("ifInUcastPkts: expected 1000, got %d", v)
	}
	// 8. ifInMulticastPkts = 50
	if v := binary.BigEndian.Uint32(buf[base+36:]); v != 50 {
		t.Errorf("ifInMulticastPkts: expected 50, got %d", v)
	}
	// 9. ifInBroadcastPkts = 10
	if v := binary.BigEndian.Uint32(buf[base+40:]); v != 10 {
		t.Errorf("ifInBroadcastPkts: expected 10, got %d", v)
	}
	// 10. ifInDiscards = 2
	if v := binary.BigEndian.Uint32(buf[base+44:]); v != 2 {
		t.Errorf("ifInDiscards: expected 2, got %d", v)
	}
	// 11. ifInErrors = 1
	if v := binary.BigEndian.Uint32(buf[base+48:]); v != 1 {
		t.Errorf("ifInErrors: expected 1, got %d", v)
	}
	// 12. ifInUnknownProtos = 0
	if v := binary.BigEndian.Uint32(buf[base+52:]); v != 0 {
		t.Errorf("ifInUnknownProtos: expected 0, got %d", v)
	}
	// 13. ifOutOctets = 987654321
	if v := binary.BigEndian.Uint64(buf[base+56:]); v != 987654321 {
		t.Errorf("ifOutOctets: expected 987654321, got %d", v)
	}
	// 14. ifOutUcastPkts = 2000
	if v := binary.BigEndian.Uint32(buf[base+64:]); v != 2000 {
		t.Errorf("ifOutUcastPkts: expected 2000, got %d", v)
	}
	// 15. ifOutMulticastPkts = 30
	if v := binary.BigEndian.Uint32(buf[base+68:]); v != 30 {
		t.Errorf("ifOutMulticastPkts: expected 30, got %d", v)
	}
	// 16. ifOutBroadcastPkts = 5
	if v := binary.BigEndian.Uint32(buf[base+72:]); v != 5 {
		t.Errorf("ifOutBroadcastPkts: expected 5, got %d", v)
	}
	// 17. ifOutDiscards = 0
	if v := binary.BigEndian.Uint32(buf[base+76:]); v != 0 {
		t.Errorf("ifOutDiscards: expected 0, got %d", v)
	}
	// 18. ifOutErrors = 0
	if v := binary.BigEndian.Uint32(buf[base+80:]); v != 0 {
		t.Errorf("ifOutErrors: expected 0, got %d", v)
	}
	// 19. ifPromiscuousMode = 0
	if v := binary.BigEndian.Uint32(buf[base+84:]); v != 0 {
		t.Errorf("ifPromiscuousMode: expected 0, got %d", v)
	}
}

func TestSFlowIfCountersSize(t *testing.T) {
	// Verify the constant matches: 16 uint32 fields (64) + 3 uint64 fields (24) = 88
	expected := 16*4 + 3*8
	if flowexport.IfCountersSize != expected {
		t.Errorf("flowexport.IfCountersSize: expected %d, got %d", expected, flowexport.IfCountersSize)
	}
}

func TestSFlowCounterSample(t *testing.T) {
	buf := make([]byte, 256)
	c := testCounters()

	off := WriteCounterSample(buf, 0, 7, 1, c)

	expectedOff := CounterSampleSize()
	if off != expectedOff {
		t.Fatalf("expected offset %d, got %d", expectedOff, off)
	}

	// data_format = 0x00000002 (counters_sample)
	if v := binary.BigEndian.Uint32(buf[0:]); v != DataFormatCountersSample {
		t.Errorf("data_format: expected 0x%08x, got 0x%08x", DataFormatCountersSample, v)
	}

	// sample_length: should be total - 8 (data_format + sample_length itself)
	sampleLen := binary.BigEndian.Uint32(buf[4:])
	expectedLen := uint32(off - 8) // everything after the sample_length field
	if sampleLen != expectedLen {
		t.Errorf("sample_length: expected %d, got %d", expectedLen, sampleLen)
	}

	// sequence_number = 1
	if v := binary.BigEndian.Uint32(buf[8:]); v != 1 {
		t.Errorf("sequence: expected 1, got %d", v)
	}

	// source_id: type=0 in high 8 bits, ifIndex=7 in low 24 bits = 0x00000007
	if v := binary.BigEndian.Uint32(buf[12:]); v != 7 {
		t.Errorf("source_id: expected 7, got 0x%08x", v)
	}

	// num_records = 1
	if v := binary.BigEndian.Uint32(buf[16:]); v != 1 {
		t.Errorf("num_records: expected 1, got %d", v)
	}

	// if_counters record starts at offset 20
	// Verify record_data_format = 0x00000001
	if v := binary.BigEndian.Uint32(buf[20:]); v != DataFormatIfCounters {
		t.Errorf("if_counters data_format: expected 0x%08x, got 0x%08x", DataFormatIfCounters, v)
	}
	// Verify record_length = 88
	if v := binary.BigEndian.Uint32(buf[24:]); v != flowexport.IfCountersSize {
		t.Errorf("if_counters record_length: expected %d, got %d", flowexport.IfCountersSize, v)
	}
}

func TestSFlowCounterSampleSourceIDEncoding(t *testing.T) {
	buf := make([]byte, 256)
	c := testCounters()

	// Test with large ifIndex that exercises the 24-bit mask
	var largeIndex uint32 = 0x00ABCDEF
	c.IfIndex = largeIndex

	WriteCounterSample(buf, 0, largeIndex, 1, c)

	// source_id should have type=0 in high byte, index=0xABCDEF in low 24 bits
	sourceID := binary.BigEndian.Uint32(buf[12:])
	expected := uint32(0x00ABCDEF)
	if sourceID != expected {
		t.Errorf("source_id: expected 0x%08x, got 0x%08x", expected, sourceID)
	}
}

func TestSFlowCounterSampleSourceIDOverflow(t *testing.T) {
	buf := make([]byte, 256)
	c := testCounters()

	// ifIndex larger than 24 bits should be masked
	c.IfIndex = 0x01FFFFFF

	WriteCounterSample(buf, 0, 0x01FFFFFF, 1, c)

	sourceID := binary.BigEndian.Uint32(buf[12:])
	expected := uint32(0x00FFFFFF) // masked to 24 bits
	if sourceID != expected {
		t.Errorf("source_id for overflow: expected 0x%08x, got 0x%08x", expected, sourceID)
	}
}

func TestSFlowCounterSampleTotalSize(t *testing.T) {
	// Verify CounterSampleSize() returns the correct total
	// counterSampleHeader(20) + ifCountersRecordHeader(8) + ifCounters(88) = 116
	expected := 116
	if CounterSampleSize() != expected {
		t.Errorf("CounterSampleSize: expected %d, got %d", expected, CounterSampleSize())
	}
}
