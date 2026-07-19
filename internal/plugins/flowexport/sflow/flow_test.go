package sflow

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// RFC requirement: SFLOW-V5-x-10 positive -- the flow_sample carries the actual 1-in-N sampling_rate the agent used: the rate handed to WriteFlowSample (2048) decodes back at the sampling_rate offset (flow.go:51-52).
func TestSFlowFlowSample(t *testing.T) {
	buf := make([]byte, 256)

	off, slOff, nrOff := WriteFlowSample(buf, 0,
		42,   // seqNum
		100,  // sourceID (ifIndex)
		2048, // rate
		5000, // pool
		3,    // drops
		100,  // input ifIndex
		200,  // output ifIndex
	)
	BackfillFlowSample(buf, slOff, nrOff, off, 0)

	// data_format = flow_sample (enterprise 0, format 1)
	if got := binary.BigEndian.Uint32(buf[0:]); got != 0x00000001 {
		t.Fatalf("data_format = %#x, want 0x00000001", got)
	}

	// sequence_number at offset 8
	if got := binary.BigEndian.Uint32(buf[8:]); got != 42 {
		t.Fatalf("sequence_number = %d, want 42", got)
	}

	// source_id at offset 12 (masked to 24 bits)
	if got := binary.BigEndian.Uint32(buf[12:]); got != 100 {
		t.Fatalf("source_id = %d, want 100", got)
	}

	// sampling_rate at offset 16
	if got := binary.BigEndian.Uint32(buf[16:]); got != 2048 {
		t.Fatalf("sampling_rate = %d, want 2048", got)
	}

	// sample_pool at offset 20
	if got := binary.BigEndian.Uint32(buf[20:]); got != 5000 {
		t.Fatalf("sample_pool = %d, want 5000", got)
	}

	// drops at offset 24
	if got := binary.BigEndian.Uint32(buf[24:]); got != 3 {
		t.Fatalf("drops = %d, want 3", got)
	}

	// input at offset 28
	if got := binary.BigEndian.Uint32(buf[28:]); got != 100 {
		t.Fatalf("input = %d, want 100", got)
	}

	// output at offset 32
	if got := binary.BigEndian.Uint32(buf[32:]); got != 200 {
		t.Fatalf("output = %d, want 200", got)
	}

	// Header size: 4 (data_format) + 4 (sample_length) + 8*4 fields = 40 bytes
	if off != 40 {
		t.Fatalf("header offset = %d, want 40", off)
	}
}

// RFC requirement: SFLOW-V5-x-8 positive -- the sampled_header's variable-length opaque header is XDR-framed: a 4-byte count prefix precedes the bytes (asserted == 14) and the 14-byte payload is zero-padded up to the next 4-byte boundary (asserted pad bytes == 0) (flow.go:116-128).
func TestSFlowSampledHeader(t *testing.T) {
	buf := make([]byte, 256)
	// Simulate a 14-byte Ethernet header (dst MAC + src MAC + ethertype)
	ethHdr := []byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, // dst MAC
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, // src MAC
		0x08, 0x00, // ethertype IPv4
	}

	off := WriteSampledHeader(buf, 0, HeaderProtocolEthernet, 128, 0, ethHdr)

	// data_format = sampled_header (enterprise 0, format 1)
	if got := binary.BigEndian.Uint32(buf[0:]); got != 0x00000001 {
		t.Fatalf("data_format = %#x, want 0x00000001", got)
	}

	// header_protocol at offset 8
	if got := binary.BigEndian.Uint32(buf[8:]); got != 1 {
		t.Fatalf("header_protocol = %d, want 1 (Ethernet)", got)
	}

	// frame_length at offset 12
	if got := binary.BigEndian.Uint32(buf[12:]); got != 128 {
		t.Fatalf("frame_length = %d, want 128", got)
	}

	// stripped at offset 16
	if got := binary.BigEndian.Uint32(buf[16:]); got != 0 {
		t.Fatalf("stripped = %d, want 0", got)
	}

	// header length at offset 20 (XDR opaque count)
	if got := binary.BigEndian.Uint32(buf[20:]); got != 14 {
		t.Fatalf("header length = %d, want 14", got)
	}

	// header data starts at offset 24
	for i, b := range ethHdr {
		if buf[24+i] != b {
			t.Fatalf("header byte %d = %02x, want %02x", i, buf[24+i], b)
		}
	}

	// XDR padding: 14 bytes data + 2 bytes padding = 16 (4-byte aligned)
	// Total: 4 (data_format) + 4 (record_length) + 4 (protocol) + 4 (frame_length)
	//      + 4 (stripped) + 4 (hdr count) + 14 (data) + 2 (pad) = 40
	if off != 40 {
		t.Fatalf("total offset = %d, want 40", off)
	}

	// Verify padding bytes are zero
	if buf[38] != 0 || buf[39] != 0 {
		t.Fatalf("padding bytes not zero: %02x %02x", buf[38], buf[39])
	}
}

func TestSFlowSampledHeaderAligned(t *testing.T) {
	buf := make([]byte, 256)
	// 16-byte header: already 4-byte aligned, no padding needed
	hdr := make([]byte, 16)
	for i := range hdr {
		hdr[i] = byte(i)
	}

	off := WriteSampledHeader(buf, 0, HeaderProtocolEthernet, 64, 4, hdr)

	// Total: 4 + 4 + 4 + 4 + 4 + 4 + 16 + 0(no pad) = 40
	if off != 40 {
		t.Fatalf("total offset = %d, want 40", off)
	}
}

func TestSFlowExtendedGateway(t *testing.T) {
	buf := make([]byte, 512)
	nextHop := netip.MustParseAddr("10.0.0.1")
	asPath := []uint32{65001, 65002, 65003}
	communities := []uint32{0xFFFF0001, 0xFFFF0002}

	off := WriteExtendedGateway(buf, 0, nextHop, 65000, 65001, 65001, asPath, communities, 100)

	// data_format = extended_gateway (enterprise 0, format 1003)
	if got := binary.BigEndian.Uint32(buf[0:]); got != 0x000003EB {
		t.Fatalf("data_format = %#x, want 0x000003EB", got)
	}

	// Skip record_length (offset 4), check nexthop
	// address_type = 1 (IPv4) at offset 8
	if got := binary.BigEndian.Uint32(buf[8:]); got != 1 {
		t.Fatalf("nexthop address_type = %d, want 1", got)
	}

	// nexthop IPv4 at offset 12
	if buf[12] != 10 || buf[13] != 0 || buf[14] != 0 || buf[15] != 1 {
		t.Fatalf("nexthop = %d.%d.%d.%d, want 10.0.0.1", buf[12], buf[13], buf[14], buf[15])
	}

	// agent AS at offset 16
	if got := binary.BigEndian.Uint32(buf[16:]); got != 65000 {
		t.Fatalf("agent AS = %d, want 65000", got)
	}

	// src_as at offset 20
	if got := binary.BigEndian.Uint32(buf[20:]); got != 65001 {
		t.Fatalf("src_as = %d, want 65001", got)
	}

	// src_peer_as at offset 24
	if got := binary.BigEndian.Uint32(buf[24:]); got != 65001 {
		t.Fatalf("src_peer_as = %d, want 65001", got)
	}

	// dst_as_path: num_segments=1 at offset 28
	if got := binary.BigEndian.Uint32(buf[28:]); got != 1 {
		t.Fatalf("num_segments = %d, want 1", got)
	}

	// segment type = 2 (AS_SEQUENCE) at offset 32
	if got := binary.BigEndian.Uint32(buf[32:]); got != 2 {
		t.Fatalf("segment type = %d, want 2", got)
	}

	// segment length = 3 at offset 36
	if got := binary.BigEndian.Uint32(buf[36:]); got != 3 {
		t.Fatalf("segment length = %d, want 3", got)
	}

	// AS path values at offsets 40, 44, 48
	for i, expected := range asPath {
		if got := binary.BigEndian.Uint32(buf[40+i*4:]); got != expected {
			t.Fatalf("AS path[%d] = %d, want %d", i, got, expected)
		}
	}

	// communities count at offset 52
	if got := binary.BigEndian.Uint32(buf[52:]); got != 2 {
		t.Fatalf("communities count = %d, want 2", got)
	}

	// community values at offsets 56, 60
	if got := binary.BigEndian.Uint32(buf[56:]); got != 0xFFFF0001 {
		t.Fatalf("community[0] = %#x, want 0xFFFF0001", got)
	}
	if got := binary.BigEndian.Uint32(buf[60:]); got != 0xFFFF0002 {
		t.Fatalf("community[1] = %#x, want 0xFFFF0002", got)
	}

	// localpref at offset 64
	if got := binary.BigEndian.Uint32(buf[64:]); got != 100 {
		t.Fatalf("localpref = %d, want 100", got)
	}

	// Total = 68 bytes
	if off != 68 {
		t.Fatalf("total offset = %d, want 68", off)
	}
}

func TestSFlowExtendedGatewayEmptyASPath(t *testing.T) {
	buf := make([]byte, 256)
	nextHop := netip.MustParseAddr("10.0.0.1")

	off := WriteExtendedGateway(buf, 0, nextHop, 65000, 0, 0, nil, nil, 200)

	// num_segments = 0 at offset 28
	if got := binary.BigEndian.Uint32(buf[28:]); got != 0 {
		t.Fatalf("num_segments = %d, want 0", got)
	}

	// communities count = 0 at offset 32
	if got := binary.BigEndian.Uint32(buf[32:]); got != 0 {
		t.Fatalf("communities count = %d, want 0", got)
	}

	// localpref at offset 36
	if got := binary.BigEndian.Uint32(buf[36:]); got != 200 {
		t.Fatalf("localpref = %d, want 200", got)
	}

	// Total: 4+4 + 4+4(nexthop) + 4+4+4(as/src_as/src_peer_as)
	//       + 4(num_segments=0) + 4(communities count=0) + 4(localpref) = 40
	if off != 40 {
		t.Fatalf("total offset = %d, want 40", off)
	}
}

func TestSFlowExtendedGatewayIPv6(t *testing.T) {
	buf := make([]byte, 512)
	nextHop := netip.MustParseAddr("2001:db8::1")

	off := WriteExtendedGateway(buf, 0, nextHop, 65000, 0, 0, nil, nil, 100)

	// address_type = 2 (IPv6) at offset 8
	if got := binary.BigEndian.Uint32(buf[8:]); got != 2 {
		t.Fatalf("nexthop address_type = %d, want 2 (IPv6)", got)
	}

	// IPv6 is 12 bytes larger than IPv4, so total shifts by 12
	// Total: 4+4 + 4+16(nexthop IPv6) + 4+4+4 + 4 + 4 + 4 = 52
	if off != 52 {
		t.Fatalf("total offset = %d, want 52", off)
	}
}
