// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- sFlow flow samples
// Related: flow_adapter_test.go -- newLoopbackFlowTarget, recvFlowDatagram, samplePoolOffset
//
// VALIDATES: a flow_sample_expanded datagram the real FlowEncoder sends carries
// the data source's sampling rate, writes 0 for the unknown egress interface,
// and is XDR-encoded throughout: big-endian words, four-octet alignment, and
// zero padding after the sampled header bytes.
// PREVENTS: a hard-coded or stale sampling_rate, a sentinel other than 0 for
// an unknown integer, and a datagram whose opaque data breaks XDR alignment.

package sflow

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// Offsets of the flow_sample_expanded fields in an IPv4-agent datagram.
const (
	samplingRateOffset = HeaderSizeIPv4 + 20
	outputFormatOffset = HeaderSizeIPv4 + 40
	outputValueOffset  = HeaderSizeIPv4 + 44
	headerLengthOffset = HeaderSizeIPv4 + 72
	headerBytesOffset  = HeaderSizeIPv4 + 76
)

// RFC requirement: SFLOW-V5-x-10 positive -- the sampling_rate field of each
// flow_sample EncodeFlowSample sends is the sFlowPacketSamplingRate of the data
// source that took the sample: two sources sampling 1-in-74565 and 1-in-1000
// report those two rates, not a constant or the previous sample's rate.
func TestSFlowV5SamplingRateIsTheSourceRate(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewFlowEncoder(netip.MustParseAddr("192.0.2.1"), 0, time.Now())

	for _, sample := range []flowexport.FlowSample{
		{IfIndex: 3, Rate: 0x00012345, OrigSize: 64, Header: make([]byte, 20)},
		{IfIndex: 4, Rate: 1000, OrigSize: 64, Header: make([]byte, 20)},
	} {
		if err := enc.EncodeFlowSample(sample, s); err != nil {
			t.Fatal(err)
		}
		dg := recvFlowDatagram(t, pc)
		if got := binary.BigEndian.Uint32(dg[samplingRateOffset:]); got != sample.Rate {
			t.Errorf("source %d: sampling_rate = %d, want its rate %d", sample.IfIndex, got, sample.Rate)
		}
	}
}

// RFC requirement: SFLOW-V5-x-16 positive -- an unknown integer value is sent as
// 0: a sample whose egress interface the kernel did not report carries an
// output interface format 0 and value 0.
func TestSFlowV5UnknownEgressInterfaceIsZero(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewFlowEncoder(netip.MustParseAddr("192.0.2.1"), 0, time.Now())

	if err := enc.EncodeFlowSample(flowexport.FlowSample{IfIndex: 3, Rate: 100, OrigSize: 64, Header: make([]byte, 20)}, s); err != nil {
		t.Fatal(err)
	}
	dg := recvFlowDatagram(t, pc)
	if got := binary.BigEndian.Uint32(dg[outputFormatOffset:]); got != 0 {
		t.Errorf("unknown output format = %#x, want 0", got)
	}
	if got := binary.BigEndian.Uint32(dg[outputValueOffset:]); got != 0 {
		t.Errorf("unknown output value = %#x, want 0", got)
	}
}

// RFC requirement: SFLOW-V5-x-7 positive -- a flow datagram is XDR-encoded: the
// header words are big-endian (version 5, address type 1, the sub-agent id and
// sequence number most significant octet first), every length is a multiple of
// four, and the 13 sampled header bytes are followed by three zero octets of
// padding so the datagram ends on a four-octet boundary.
func TestSFlowV5FlowDatagramIsXDR(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewFlowEncoder(netip.MustParseAddr("192.0.2.1"), 0x01020304, time.Now())

	frame := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	if err := enc.EncodeFlowSample(flowexport.FlowSample{IfIndex: 3, Rate: 100, OrigSize: 64, Header: frame}, s); err != nil {
		t.Fatal(err)
	}
	dg := recvFlowDatagram(t, pc)

	wantHeader := []byte{0, 0, 0, 5, 0, 0, 0, 1, 192, 0, 2, 1, 0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(dg[:16], wantHeader) {
		t.Fatalf("datagram header = %x, want %x", dg[:16], wantHeader)
	}
	if len(dg)%4 != 0 {
		t.Fatalf("datagram length %d is not a multiple of four", len(dg))
	}
	if got := binary.BigEndian.Uint32(dg[headerLengthOffset:]); got != uint32(len(frame)) {
		t.Fatalf("header_length = %d, want %d", got, len(frame))
	}
	body := dg[headerBytesOffset:]
	if !bytes.Equal(body[:len(frame)], frame) {
		t.Fatalf("sampled header bytes = %x, want %x", body[:len(frame)], frame)
	}
	if pad := body[len(frame):]; !bytes.Equal(pad, []byte{0, 0, 0}) {
		t.Fatalf("padding after 13 header bytes = %x, want three zero octets", pad)
	}
	sampleLength := binary.BigEndian.Uint32(dg[HeaderSizeIPv4+4:])
	if sampleLength%4 != 0 || int(sampleLength) != len(dg)-HeaderSizeIPv4-8 {
		t.Fatalf("flow_sample length = %d, want %d and a multiple of four", sampleLength, len(dg)-HeaderSizeIPv4-8)
	}
}
