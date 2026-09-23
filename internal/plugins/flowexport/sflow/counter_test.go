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

	off := writeIfCounters(buf, 0, c)

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

	off := writeCounterSample(buf, 0, 7, 1, c)

	expectedOff := counterSampleSize()
	if off != expectedOff {
		t.Fatalf("expected offset %d, got %d", expectedOff, off)
	}

	// data_format = 4 (counters_sample_expanded).
	if v := binary.BigEndian.Uint32(buf[0:]); v != DataFormatCountersSampleExpanded {
		t.Errorf("data_format: expected 0x%08x, got 0x%08x", DataFormatCountersSampleExpanded, v)
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

	if v := binary.BigEndian.Uint32(buf[12:]); v != 0 {
		t.Errorf("source type: expected 0, got %d", v)
	}
	// The source index follows its separate type field.
	if v := binary.BigEndian.Uint32(buf[16:]); v != 7 {
		t.Errorf("source_id: expected 7, got 0x%08x", v)
	}

	// num_records = 1
	if v := binary.BigEndian.Uint32(buf[20:]); v != 1 {
		t.Errorf("num_records: expected 1, got %d", v)
	}

	// if_counters record starts at offset 24.
	// Verify record_data_format = 0x00000001.
	if v := binary.BigEndian.Uint32(buf[24:]); v != DataFormatIfCounters {
		t.Errorf("if_counters data_format: expected 0x%08x, got 0x%08x", DataFormatIfCounters, v)
	}
	// Verify record_length = 88
	if v := binary.BigEndian.Uint32(buf[28:]); v != flowexport.IfCountersSize {
		t.Errorf("if_counters record_length: expected %d, got %d", flowexport.IfCountersSize, v)
	}
}

func TestSFlowCounterSampleSourceIDEncoding(t *testing.T) {
	buf := make([]byte, 256)
	c := testCounters()

	// Exercise an index that also fits the old compact format.
	var largeIndex uint32 = 0x00ABCDEF
	c.IfIndex = largeIndex

	writeCounterSample(buf, 0, largeIndex, 1, c)

	// The full index follows a separate source type.
	sourceID := binary.BigEndian.Uint32(buf[16:])
	expected := uint32(0x00ABCDEF)
	if sourceID != expected {
		t.Errorf("source_id: expected 0x%08x, got 0x%08x", expected, sourceID)
	}
}

// TestSFlowCounterSampleSourceIDOverflow decodes indices across the old compact
// boundary and verifies that the embedded if_counters identifies the same source.
// RFC requirement: SFLOW-V5-x-12 negative -- source indexes at and above 2^24 never alias their low 24 bits: expanded source_id_index and the embedded if_counters.ifIndex retain the original value.
func TestSFlowCounterSampleSourceIDOverflow(t *testing.T) {
	for _, ifIndex := range []uint32{0x00FFFFFF, 0x01000000, 0xFFFFFFFF} {
		c := testCounters()
		c.IfIndex = ifIndex
		buf := make([]byte, counterSampleSize())
		for i := range buf {
			buf[i] = 0xFF
		}
		off := writeCounterSample(buf, 0, ifIndex, 9, c)
		if off != len(buf) {
			t.Fatalf("ifIndex %#x: encoded %d bytes, sized %d", ifIndex, off, len(buf))
		}
		const format = 4
		const recordOff = 24
		const sourceOff = 16
		if got := binary.BigEndian.Uint32(buf[12:]); got != 0 {
			t.Fatalf("ifIndex %#x: source type = %d, want 0", ifIndex, got)
		}
		if got := binary.BigEndian.Uint32(buf); got != format {
			t.Fatalf("ifIndex %#x: sample format = %d, want %d", ifIndex, got, format)
		}
		if got := binary.BigEndian.Uint32(buf[sourceOff:]); got != ifIndex {
			t.Errorf("source index = %#x, want %#x", got, ifIndex)
		}
		if got := binary.BigEndian.Uint32(buf[4:]); got != uint32(off-8) {
			t.Errorf("sample length = %d, want %d", got, off-8)
		}
		if got := binary.BigEndian.Uint32(buf[8:]); got != 9 {
			t.Errorf("sequence = %d, want 9", got)
		}
		if got := binary.BigEndian.Uint32(buf[recordOff-4:]); got != 1 {
			t.Errorf("record count = %d, want 1", got)
		}
		if got := binary.BigEndian.Uint32(buf[recordOff:]); got != 1 {
			t.Errorf("counter format = %d, want 1", got)
		}
		if got := binary.BigEndian.Uint32(buf[recordOff+4:]); got != 88 {
			t.Errorf("counter length = %d, want 88", got)
		}
		if got := binary.BigEndian.Uint32(buf[recordOff+8:]); got != ifIndex {
			t.Errorf("counter ifIndex = %#x, want %#x", got, ifIndex)
		}
	}
}

func TestSFlowCounterSampleTotalSize(t *testing.T) {
	// Expanded counter sample: header(24) + record header(8) + counters(88).
	expected := 120
	if counterSampleSize() != expected {
		t.Errorf("CounterSampleSize: expected %d, got %d", expected, counterSampleSize())
	}
}
