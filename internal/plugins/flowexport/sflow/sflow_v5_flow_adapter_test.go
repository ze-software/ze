package sflow

import (
	"context"
	"encoding/binary"
	"math"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// newLoopbackFlowTarget opens a loopback UDP listener and a Sender aimed at it,
// so a test can read back the exact datagram EncodeFlowSample transmitted.
func newLoopbackFlowTarget(t *testing.T) (net.PacketConn, *flowexport.Sender) {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "", flowexport.DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	return pc, s
}

// recvFlowDatagram reads one datagram from pc with a short deadline.
func recvFlowDatagram(t *testing.T, pc net.PacketConn) []byte {
	t.Helper()
	_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 2048)
	nr, _, err := pc.ReadFrom(buf)
	if err != nil {
		t.Fatalf("read datagram: %v", err)
	}
	return buf[:nr]
}

const (
	// sampleSeqOffset is the flow_sample_expanded sequence_number field in an
	// IPv4-agent datagram: 28-byte header + 8-byte sample tag/length.
	sampleSeqOffset = HeaderSizeIPv4 + 8
	// samplePoolOffset is the flow_sample_expanded sample_pool field in an
	// IPv4-agent datagram: 28-byte header + 8-byte sample tag/length + 16.
	samplePoolOffset = HeaderSizeIPv4 + 24
)

// RFC requirement: SFLOW-V5-x-11 positive -- EncodeFlowSample estimates sample_pool as a per-source accumulated sum of each sample's Rate in unsigned 32-bit modular arithmetic (flow_adapter.go::FlowEncoder.EncodeFlowSample): below 2^32 it grows by Rate per exported sample, and across 2^32 it wraps modulo 2^32 instead of saturating at MaxUint32 (which sFlow v5 Section 5 reserves as the "not available" counter sentinel while permitting a natural rollover to zero). Decoded on-wire sample_pool values confirm both constant-rate growth and modular wrap.
func TestSFlowFlowSamplePoolTracksTotal(t *testing.T) {
	agent := netip.MustParseAddr("10.0.0.1")

	t.Run("grows by rate per sample", func(t *testing.T) {
		pc, s := newLoopbackFlowTarget(t)
		defer func() { _ = pc.Close() }()
		defer func() { _ = s.Close() }()

		enc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))
		const rate = 1024

		for seq := uint32(1); seq <= 3; seq++ {
			if err := enc.EncodeFlowSample(flowexport.FlowSample{
				IfIndex: 7, Rate: rate, OrigSize: 128, Header: []byte{1, 2, 3, 4},
			}, s); err != nil {
				t.Fatal(err)
			}
			dg := recvFlowDatagram(t, pc)
			pool := binary.BigEndian.Uint32(dg[samplePoolOffset:])
			if want := seq * rate; pool != want {
				t.Errorf("sample %d: sample_pool = %d, want %d (accumulated rate)", seq, pool, want)
			}
		}
	})

	t.Run("wraps modulo 2^32", func(t *testing.T) {
		pc, s := newLoopbackFlowTarget(t)
		defer func() { _ = pc.Close() }()
		defer func() { _ = s.Close() }()

		enc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))
		const halfSpace uint32 = 1 << 31
		wantPools := []uint32{halfSpace, 0, halfSpace}

		var prev uint32
		for i, want := range wantPools {
			if err := enc.EncodeFlowSample(flowexport.FlowSample{
				IfIndex: 7, Rate: halfSpace, OrigSize: 128, Header: []byte{1, 2, 3, 4},
			}, s); err != nil {
				t.Fatal(err)
			}
			dg := recvFlowDatagram(t, pc)
			pool := binary.BigEndian.Uint32(dg[samplePoolOffset:])
			if pool == math.MaxUint32 {
				t.Fatalf("sample %d: sample_pool saturated at MaxUint32 (%d)", i+1, uint32(math.MaxUint32))
			}
			if pool != want {
				t.Errorf("sample %d: sample_pool = %d, want %d", i+1, pool, want)
			}
			if gotDelta := pool - prev; gotDelta != halfSpace {
				t.Errorf("sample %d: modular delta = %d, want %d", i+1, gotDelta, halfSpace)
			}
			prev = pool
		}

		// Boundary check near MaxUint32: (MaxUint32 - 10) + 20 wraps cleanly to 9.
		nearEnc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))
		boundaryRates := []uint32{math.MaxUint32 - 10, 20}
		boundaryWant := []uint32{math.MaxUint32 - 10, 9}
		prev = 0
		for i, rate := range boundaryRates {
			if err := nearEnc.EncodeFlowSample(flowexport.FlowSample{
				IfIndex: 7, Rate: rate, OrigSize: 128, Header: []byte{1, 2, 3, 4},
			}, s); err != nil {
				t.Fatal(err)
			}
			dg := recvFlowDatagram(t, pc)
			pool := binary.BigEndian.Uint32(dg[samplePoolOffset:])
			if pool != boundaryWant[i] {
				t.Errorf("boundary sample %d: sample_pool = %d, want %d", i+1, pool, boundaryWant[i])
			}
			if gotDelta := pool - prev; gotDelta != rate {
				t.Errorf("boundary sample %d: modular delta = %d, want %d", i+1, gotDelta, rate)
			}
			prev = pool
		}
	})
}

// TestSFlowSamplePoolPerSource verifies independent per-source accumulation,
// rate changes without retroactive rewriting, and coupled encoder reset.
//
// VALIDATES: per-source (ifIndex) sample_pool accumulation advances by each
// sample's Rate independently across interfaces and resets alongside
// sequence_number on a fresh FlowEncoder.
// PREVENTS: cross-source pool contamination, seq*currentRate retroactive
// misstatement on rate change, or uncoupled sequence/pool reset.
// RFC requirement: SFLOW-V5-x-11 positive -- EncodeFlowSample maintains independent per-source (ifIndex) accumulated sample_pool estimates that advance by each sample's Rate across rate changes and reset alongside sequence_number on a fresh FlowEncoder (flow_adapter.go::FlowEncoder.EncodeFlowSample)
func TestSFlowSamplePoolPerSource(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	agent := netip.MustParseAddr("10.0.0.1")
	enc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))

	steps := []struct {
		ifIndex  uint32
		rate     uint32
		wantSeq  uint32
		wantPool uint32
	}{
		{ifIndex: 7, rate: 100, wantSeq: 1, wantPool: 100},
		{ifIndex: 9, rate: 512, wantSeq: 1, wantPool: 512},
		// Rate change on ifIndex 7: accumulated pool is 100 + 250 = 350, not 2*250 = 500.
		{ifIndex: 7, rate: 250, wantSeq: 2, wantPool: 350},
		{ifIndex: 9, rate: 512, wantSeq: 2, wantPool: 1024},
		{ifIndex: 7, rate: 250, wantSeq: 3, wantPool: 600},
	}

	for i, step := range steps {
		if err := enc.EncodeFlowSample(flowexport.FlowSample{
			IfIndex: step.ifIndex, Rate: step.rate, OrigSize: 128, Header: []byte{1, 2, 3, 4},
		}, s); err != nil {
			t.Fatal(err)
		}
		dg := recvFlowDatagram(t, pc)
		gotSeq := binary.BigEndian.Uint32(dg[sampleSeqOffset:])
		gotPool := binary.BigEndian.Uint32(dg[samplePoolOffset:])
		if gotSeq != step.wantSeq {
			t.Errorf("step %d (ifIndex %d): sequence_number = %d, want %d", i+1, step.ifIndex, gotSeq, step.wantSeq)
		}
		if gotPool != step.wantPool {
			t.Errorf("step %d (ifIndex %d): sample_pool = %d, want %d", i+1, step.ifIndex, gotPool, step.wantPool)
		}
	}

	// A fresh FlowEncoder resets sequence_number and sample_pool together.
	resetEnc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))
	if err := resetEnc.EncodeFlowSample(flowexport.FlowSample{
		IfIndex: 7, Rate: 250, OrigSize: 128, Header: []byte{1, 2, 3, 4},
	}, s); err != nil {
		t.Fatal(err)
	}
	dg := recvFlowDatagram(t, pc)
	if gotSeq := binary.BigEndian.Uint32(dg[sampleSeqOffset:]); gotSeq != 1 {
		t.Errorf("fresh encoder: sequence_number = %d, want 1", gotSeq)
	}
	if gotPool := binary.BigEndian.Uint32(dg[samplePoolOffset:]); gotPool != 250 {
		t.Errorf("fresh encoder: sample_pool = %d, want 250", gotPool)
	}
}

// TestSFlowExpandedFlowDatagramBound reads the transmitted datagram and checks
// that expanded metadata and a truncated capture fit both agent address headers.
func TestSFlowExpandedFlowDatagramBound(t *testing.T) {
	for _, agent := range []netip.Addr{
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("2001:db8::1"),
	} {
		t.Run(agent.String(), func(t *testing.T) {
			pc, sender := newLoopbackFlowTarget(t)
			defer func() { _ = pc.Close() }()
			defer func() { _ = sender.Close() }()
			capture := make([]byte, 1500)
			for i := range capture {
				capture[i] = byte(i)
			}
			enc := NewFlowEncoder(agent, 1, time.Unix(1716000000, 0))
			sample := flowexport.FlowSample{
				IfIndex: 0x01000000, Rate: 64, OrigSize: 1500,
				Output: 0x40000102, Header: capture,
			}
			if err := enc.EncodeFlowSample(sample, sender); err != nil {
				t.Fatal(err)
			}
			dg := recvFlowDatagram(t, pc)
			if len(dg) > sender.MaxDatagram() {
				t.Fatalf("datagram length = %d, limit %d", len(dg), sender.MaxDatagram())
			}
			headerSize := 28
			if agent.Is6() {
				headerSize = 40
			}
			record := dg[headerSize:]
			if got := binary.BigEndian.Uint32(record); got != 3 {
				t.Fatalf("sample format = %d, want 3", got)
			}
			if got := binary.BigEndian.Uint32(record[4:]); got != uint32(len(record)-8) {
				t.Errorf("sample length = %d, want %d", got, len(record)-8)
			}
			for _, offset := range []int{12, 32, 40} {
				if got := binary.BigEndian.Uint32(record[offset:]); got != 0 {
					t.Errorf("source/interface format at %d = %d, want 0", offset, got)
				}
			}
			if got := binary.BigEndian.Uint32(record[16:]); got != sample.IfIndex {
				t.Errorf("source index = %#x, want %#x", got, sample.IfIndex)
			}
			if got := binary.BigEndian.Uint32(record[36:]); got != sample.IfIndex {
				t.Errorf("input index = %#x, want %#x", got, sample.IfIndex)
			}
			if got := binary.BigEndian.Uint32(record[44:]); got != sample.Output {
				t.Errorf("output index = %#x, want %#x", got, sample.Output)
			}
			if got := binary.BigEndian.Uint32(record[48:]); got != 1 {
				t.Fatalf("flow record count = %d, want 1", got)
			}
			header := record[52:]
			if got := binary.BigEndian.Uint32(header); got != 1 {
				t.Fatalf("flow record format = %d, want sampled_header 1", got)
			}
			captured := int(binary.BigEndian.Uint32(header[20:]))
			wantCaptured := sender.MaxDatagram() - headerSize - 52 - 24 - 4
			if captured != wantCaptured {
				t.Fatalf("captured bytes = %d, want %d", captured, wantCaptured)
			}
			padded := (captured + 3) &^ 3
			if len(header) != 24+padded {
				t.Fatalf("sampled header size = %d, want %d", len(header), 24+padded)
			}
			for i := range captured {
				if header[24+i] != capture[i] {
					t.Fatalf("capture byte %d = %d, want %d", i, header[24+i], capture[i])
				}
			}
		})
	}
}
