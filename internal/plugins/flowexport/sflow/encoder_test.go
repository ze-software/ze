package sflow

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// RFC requirement: SFLOW-V5-x-1 positive -- the datagram's first field decodes big-endian to 5, the compile-time Version constant WriteDatagramHeader writes (encoder.go:14,40).
// RFC requirement: SFLOW-V5-x-2 positive -- the agent-address bytes decode to the stable operator IP handed to WriteDatagramHeader unchanged (encoder.go:43-57), so a reboot re-emits the same identifying address.
func TestSFlowDatagramHeaderIPv4(t *testing.T) {
	buf := make([]byte, flowexport.MaxDatagramSize)
	agent := netip.MustParseAddr("10.0.0.1")

	off := WriteDatagramHeader(buf, 0, agent, 0, 42, 123456, 3)

	if off != HeaderSizeIPv4 {
		t.Fatalf("expected offset %d, got %d", HeaderSizeIPv4, off)
	}

	// version = 5
	if v := binary.BigEndian.Uint32(buf[0:]); v != 5 {
		t.Errorf("version: expected 5, got %d", v)
	}
	// address_type = 1 (IPv4)
	if v := binary.BigEndian.Uint32(buf[4:]); v != AddressTypeIPv4 {
		t.Errorf("address_type: expected %d, got %d", AddressTypeIPv4, v)
	}
	// agent address = 10.0.0.1
	if buf[8] != 10 || buf[9] != 0 || buf[10] != 0 || buf[11] != 1 {
		t.Errorf("agent address: expected 10.0.0.1, got %d.%d.%d.%d", buf[8], buf[9], buf[10], buf[11])
	}
	// sub_agent_id = 0
	if v := binary.BigEndian.Uint32(buf[12:]); v != 0 {
		t.Errorf("sub_agent_id: expected 0, got %d", v)
	}
	// sequence_number = 42
	if v := binary.BigEndian.Uint32(buf[16:]); v != 42 {
		t.Errorf("sequence: expected 42, got %d", v)
	}
	// uptime = 123456
	if v := binary.BigEndian.Uint32(buf[20:]); v != 123456 {
		t.Errorf("uptime: expected 123456, got %d", v)
	}
	// num_samples = 3
	if v := binary.BigEndian.Uint32(buf[24:]); v != 3 {
		t.Errorf("num_samples: expected 3, got %d", v)
	}
}

// RFC requirement: SFLOW-V5-x-3 positive -- the agent address and the sub_agent_id (7) both decode to the distinct values written into the header (encoder.go:43-59); together they name one sampling entity, so neither alone is relied upon.
func TestSFlowDatagramHeaderIPv6(t *testing.T) {
	buf := make([]byte, flowexport.MaxDatagramSize)
	agent := netip.MustParseAddr("2001:db8::1")

	off := WriteDatagramHeader(buf, 0, agent, 7, 1, 5000, 2)

	if off != HeaderSizeIPv6 {
		t.Fatalf("expected offset %d, got %d", HeaderSizeIPv6, off)
	}

	// version = 5
	if v := binary.BigEndian.Uint32(buf[0:]); v != 5 {
		t.Errorf("version: expected 5, got %d", v)
	}
	// address_type = 2 (IPv6)
	if v := binary.BigEndian.Uint32(buf[4:]); v != AddressTypeIPv6 {
		t.Errorf("address_type: expected %d, got %d", AddressTypeIPv6, v)
	}
	// agent address = 2001:db8::1
	expected := netip.MustParseAddr("2001:db8::1").As16()
	for i := range 16 {
		if buf[8+i] != expected[i] {
			t.Errorf("agent address byte %d: expected %02x, got %02x", i, expected[i], buf[8+i])
		}
	}
	// sub_agent_id = 7
	if v := binary.BigEndian.Uint32(buf[24:]); v != 7 {
		t.Errorf("sub_agent_id: expected 7, got %d", v)
	}
	// sequence_number = 1
	if v := binary.BigEndian.Uint32(buf[28:]); v != 1 {
		t.Errorf("sequence: expected 1, got %d", v)
	}
	// num_samples = 2
	if v := binary.BigEndian.Uint32(buf[36:]); v != 2 {
		t.Errorf("num_samples: expected 2, got %d", v)
	}
}

// RFC requirement: SFLOW-V5-x-4 positive -- one sub-agent keeps its own sequence space: the per-agent datagram sequence advances 1,2,3 (encoder.go:98,110) while each per-source (ifIndex) sample sequence is tracked independently in seqNums (encoder.go:132-134).
// RFC requirement: SFLOW-V5-x-5 positive -- a counter batch too large for the caller's 1400-byte payload bound spills into another datagram, and each emitted datagram stays within that configured bound.
func TestSFlowMultiInterface(t *testing.T) {
	buf := make([]byte, flowexport.MaxDatagramSize)
	agent := netip.MustParseAddr("10.0.0.1")
	seqNums := make(map[uint32]uint32)

	// CounterSampleSize() = 24 + 8 + 88 = 120 bytes per interface.
	// Header = 28 bytes
	// Available = 1400 - 28 = 1372 bytes
	// 1372 / 120 rounds down to 11 interfaces per datagram.
	ifaces := make([]flowexport.InterfaceCounters, 15)
	for i := range ifaces {
		ifaces[i] = flowexport.InterfaceCounters{
			IfIndex:     uint32(i + 1),
			IfType:      6,
			IfSpeed:     1000000000,
			IfDirection: 1,
			IfStatus:    3,
			IfInOctets:  uint64(1000 * (i + 1)),
			IfOutOctets: uint64(2000 * (i + 1)),
			Name:        "eth0",
		}
	}

	datagrams, nextSeq := writeCounterDatagrams(buf, agent, 0, 1, 5000, ifaces, seqNums)

	// 15 interfaces: 11 fit in first datagram, 4 in second
	if len(datagrams) != 2 {
		t.Fatalf("expected 2 datagrams, got %d", len(datagrams))
	}

	// SFLOW-V5-x-5: splitting preserves the configured payload bound.
	for i, dg := range datagrams {
		if len(dg) > flowexport.MaxDatagramSize {
			t.Errorf("datagram %d size %d exceeds MaxDatagramSize %d", i, len(dg), flowexport.MaxDatagramSize)
		}
	}

	// First datagram: 11 samples
	dg1 := datagrams[0]
	numSamples1 := binary.BigEndian.Uint32(dg1[24:])
	if numSamples1 != 11 {
		t.Errorf("datagram 1 num_samples: expected 11, got %d", numSamples1)
	}

	// Second datagram: 4 samples
	dg2 := datagrams[1]
	numSamples2 := binary.BigEndian.Uint32(dg2[24:])
	if numSamples2 != 4 {
		t.Errorf("datagram 2 num_samples: expected 4, got %d", numSamples2)
	}

	// Sequence numbers: datagram 1 = seq 1, datagram 2 = seq 2
	seq1 := binary.BigEndian.Uint32(dg1[16:])
	seq2 := binary.BigEndian.Uint32(dg2[16:])
	if seq1 != 1 {
		t.Errorf("datagram 1 sequence: expected 1, got %d", seq1)
	}
	if seq2 != 2 {
		t.Errorf("datagram 2 sequence: expected 2, got %d", seq2)
	}

	// Next datagram sequence should be 3
	if nextSeq != 3 {
		t.Errorf("next datagram seq: expected 3, got %d", nextSeq)
	}

	// Per-source sequence numbers should all be 1 (one sample per source)
	for i := uint32(1); i <= 15; i++ {
		if seqNums[i] != 1 {
			t.Errorf("source %d seq: expected 1, got %d", i, seqNums[i])
		}
	}

	// First datagram size: header(28) + 11 * sample(120) = 1348.
	expectedSize1 := HeaderSizeIPv4 + 11*counterSampleSize()
	if len(dg1) != expectedSize1 {
		t.Errorf("datagram 1 size: expected %d, got %d", expectedSize1, len(dg1))
	}
}

func TestSFlowEmptyInterfaces(t *testing.T) {
	buf := make([]byte, flowexport.MaxDatagramSize)
	agent := netip.MustParseAddr("10.0.0.1")
	seqNums := make(map[uint32]uint32)

	datagrams, nextSeq := writeCounterDatagrams(buf, agent, 0, 1, 5000, nil, seqNums)

	if len(datagrams) != 0 {
		t.Errorf("expected 0 datagrams for empty interfaces, got %d", len(datagrams))
	}
	if nextSeq != 1 {
		t.Errorf("next seq should be unchanged at 1, got %d", nextSeq)
	}
}

func TestSFlowSingleInterface(t *testing.T) {
	buf := make([]byte, flowexport.MaxDatagramSize)
	agent := netip.MustParseAddr("10.0.0.1")
	seqNums := make(map[uint32]uint32)

	ifaces := []flowexport.InterfaceCounters{{
		IfIndex:     42,
		IfType:      6,
		IfSpeed:     10000000000,
		IfDirection: 1,
		IfStatus:    3,
		IfInOctets:  999,
		IfOutOctets: 888,
		Name:        "eth0",
	}}

	datagrams, nextSeq := writeCounterDatagrams(buf, agent, 0, 1, 5000, ifaces, seqNums)

	if len(datagrams) != 1 {
		t.Fatalf("expected 1 datagram, got %d", len(datagrams))
	}
	if nextSeq != 2 {
		t.Errorf("next seq: expected 2, got %d", nextSeq)
	}

	dg := datagrams[0]
	expectedSize := HeaderSizeIPv4 + counterSampleSize()
	if len(dg) != expectedSize {
		t.Errorf("datagram size: expected %d, got %d", expectedSize, len(dg))
	}

	// Verify num_samples = 1
	if v := binary.BigEndian.Uint32(dg[24:]); v != 1 {
		t.Errorf("num_samples: expected 1, got %d", v)
	}
}

// TestSFlowCounterExpandedChunkBoundary decodes samples across a split that
// compact-size accounting would miss, and checks an exactly full datagram.
func TestSFlowCounterExpandedChunkBoundary(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{
		{IfIndex: 0x00FFFFFF},
		{IfIndex: 0x01000000},
		{IfIndex: 0xFFFFFFFF},
	}
	for _, limit := range []int{267, 268} {
		buf := make([]byte, limit)
		datagrams, _ := writeCounterDatagrams(buf, testAgent, 0, 1, 0, ifaces, make(map[uint32]uint32))
		wantDatagrams := 2
		if limit == 267 {
			wantDatagrams = 3
		}
		if len(datagrams) != wantDatagrams {
			t.Fatalf("limit %d: datagrams = %d, want %d", limit, len(datagrams), wantDatagrams)
		}
		source := 0
		for _, dg := range datagrams {
			if len(dg) > limit {
				t.Fatalf("datagram length = %d, limit %d", len(dg), limit)
			}
			off := 28
			count := binary.BigEndian.Uint32(dg[24:])
			for range count {
				if len(dg)-off < 24 {
					t.Fatalf("truncated sample at %d", off)
				}
				format := binary.BigEndian.Uint32(dg[off:])
				length := int(binary.BigEndian.Uint32(dg[off+4:]))
				if format != 4 {
					t.Fatalf("counter format = %d, want 4", format)
				}
				if got := binary.BigEndian.Uint32(dg[off+12:]); got != 0 {
					t.Fatalf("source type = %d, want 0", got)
				}
				sourceOff := off + 16
				if source >= len(ifaces) {
					t.Fatal("unexpected extra counter sample")
				}
				if got := binary.BigEndian.Uint32(dg[sourceOff:]); got != ifaces[source].IfIndex {
					t.Errorf("source %d = %#x, want %#x", source, got, ifaces[source].IfIndex)
				}
				source++
				off += 8 + length
			}
			if off != len(dg) {
				t.Fatalf("decoded end = %d, datagram length %d", off, len(dg))
			}
		}
		if source != len(ifaces) {
			t.Errorf("decoded %d sources, want %d", source, len(ifaces))
		}
	}
}
