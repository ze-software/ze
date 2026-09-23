// sFlow v5 conformance tests for the per-source counter sequence number and
// the sampled_header frame accounting.

package sflow

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// counterSampleSeq reads the sequence_number of an expanded counters_sample.
func counterSampleSeq(t *testing.T, data []byte) uint32 {
	t.Helper()
	if len(data) < 4 {
		t.Fatalf("counters_sample too short: %d bytes", len(data))
	}
	return binary.BigEndian.Uint32(data[0:])
}

// pollOnce encodes one counter poll of the given interfaces and returns the
// sequence number of the first counters_sample on the wire.
func pollOnce(t *testing.T, enc *CounterEncoder, s *flowexport.Sender, pc net.PacketConn, at time.Time, ifaces ...flowexport.InterfaceCounters) uint32 {
	t.Helper()
	if _, err := enc.Encode(flowexport.CounterSnapshot{Time: at, Interfaces: ifaces}, s); err != nil {
		t.Fatal(err)
	}
	dg := decodeDatagram(t, recvFlowDatagram(t, pc))
	if len(dg.samples) == 0 {
		t.Fatal("datagram carries no sample")
	}
	return counterSampleSeq(t, dg.samples[0].data)
}

// RFC requirement: SFLOW-V5-x-29 positive -- when ifInOctets of source 3 goes backwards between two polls, the counters_sample of the third poll carries sequence_number 1 again.
func TestSFlowV5SequenceResetWithCounters(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewCounterEncoder(testAgent, 1, time.Unix(1716000000, 0))
	start := time.Unix(1716000100, 0)

	if seq := pollOnce(t, enc, s, pc, start, flowexport.InterfaceCounters{IfIndex: 3, IfInOctets: 1000}); seq != 1 {
		t.Fatalf("first poll sequence = %d, want 1", seq)
	}
	if seq := pollOnce(t, enc, s, pc, start.Add(20*time.Second), flowexport.InterfaceCounters{IfIndex: 3, IfInOctets: 2000}); seq != 2 {
		t.Fatalf("second poll sequence = %d, want 2", seq)
	}
	if seq := pollOnce(t, enc, s, pc, start.Add(40*time.Second), flowexport.InterfaceCounters{IfIndex: 3, IfInOctets: 500}); seq != 1 {
		t.Fatalf("sequence after the counters went backwards = %d, want reset to 1", seq)
	}
}

// RFC requirement: SFLOW-V5-x-29 negative -- a sequence_number reset never appears while every counter of the source keeps growing: three polls with rising ifInOctets carry 1, 2, 3, and a second source's reset leaves the first source's sequence untouched.
func TestSFlowV5SequenceNeverResetWithoutDiscontinuity(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewCounterEncoder(testAgent, 1, time.Unix(1716000000, 0))
	start := time.Unix(1716000100, 0)

	octets := []uint64{1000, 2000, 3000}
	other := []uint64{100, 50, 200} // source 4 resets on the second poll
	for i, want := range []uint32{1, 2, 3} {
		seq := pollOnce(t, enc, s, pc, start.Add(time.Duration(i)*20*time.Second),
			flowexport.InterfaceCounters{IfIndex: 3, IfInOctets: octets[i]},
			flowexport.InterfaceCounters{IfIndex: 4, IfInOctets: other[i]},
		)
		if seq != want {
			t.Fatalf("poll %d: source 3 sequence = %d, want %d", i+1, seq, want)
		}
	}
}

// sampledHeaderFrame reads frame_length and stripped off the sampled_header
// record of a flow_sample.
func sampledHeaderFrame(t *testing.T, data []byte) (frameLength, stripped uint32) {
	t.Helper()
	rec := data[44:]
	if len(rec) < 24 {
		t.Fatalf("flow record too short: %d bytes", len(rec))
	}
	return binary.BigEndian.Uint32(rec[12:]), binary.BigEndian.Uint32(rec[16:])
}

// RFC requirement: SFLOW-V5-x-34 positive -- for an Ethernet frame psample reported as 60 octets, the sampled_header carries frame_length 64, the 4 FCS octets the NIC removed added back, and stripped 4, the same 4 octets.
func TestSFlowV5FrameLengthAndStrippedIncludeFCS(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	dg := encodeOne(t, enc, s, pc, flowexport.FlowSample{IfIndex: 3, Rate: 64, OrigSize: 60, Header: []byte{1, 2, 3, 4}})
	frameLength, stripped := sampledHeaderFrame(t, dg.samples[0].data)
	if frameLength != 64 {
		t.Errorf("frame_length = %d, want 64 (60 on the skb plus the 4-octet FCS)", frameLength)
	}
	if stripped != 4 {
		t.Errorf("stripped = %d, want 4 (the FCS added to frame_length)", stripped)
	}
}

// RFC requirement: SFLOW-V5-x-34 negative -- octets added to frame_length never go unaccounted in stripped: over frames of 64, 1500 and 9000 skb octets, frame_length minus stripped is always the skb length psample reported.
func TestSFlowV5FrameCompensationNeverUnstripped(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	for _, size := range []uint32{64, 1500, 9000} {
		dg := encodeOne(t, enc, s, pc, flowexport.FlowSample{IfIndex: 3, Rate: 64, OrigSize: size, Header: []byte{1, 2, 3, 4}})
		frameLength, stripped := sampledHeaderFrame(t, dg.samples[0].data)
		if frameLength-stripped != size {
			t.Errorf("skb %d: frame_length %d - stripped %d = %d, want %d", size, frameLength, stripped, frameLength-stripped, size)
		}
		if frameLength <= size {
			t.Errorf("skb %d: frame_length %d adds no FCS octets", size, frameLength)
		}
	}
}

// TestSFlowCounterSourceReappears starts a new sample sequence after a
// source disappears from a complete snapshot, even if its new counters grow.
func TestSFlowCounterSourceReappears(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	start := time.Now()
	enc := NewCounterEncoder(testAgent, 1, start)
	for i := range 2 {
		if seq := pollOnce(t, enc, s, pc, start, flowexport.InterfaceCounters{IfIndex: 3, IfInOctets: uint64(i + 1)}); seq != uint32(i+1) {
			t.Fatalf("poll %d sequence = %d", i, seq)
		}
	}
	if _, err := enc.Encode(flowexport.CounterSnapshot{Time: start}, s); err != nil {
		t.Fatal(err)
	}
	if seq := pollOnce(t, enc, s, pc, start, flowexport.InterfaceCounters{IfIndex: 3, IfInOctets: 100}); seq != 1 {
		t.Fatalf("reappearing source sequence = %d, want 1", seq)
	}
}

// TestSFlowCounterGenerationRestartsSamples covers a replacement whose
// counters already exceed the previous source's before the next poll.
func TestSFlowCounterGenerationRestartsSamples(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	start := time.Now()
	enc := NewCounterEncoder(testAgent, 1, start)
	cur := flowexport.InterfaceCounters{IfIndex: 3, CounterGeneration: 7, IfInOctets: 100, IfInUcastPkts: ^uint32(0)}
	if seq := pollOnce(t, enc, s, pc, start, cur); seq != 1 {
		t.Fatalf("first sample sequence = %d, want 1", seq)
	}
	// A normal 32-bit wire-counter wrap does not change raw continuity.
	cur.IfInOctets++
	cur.IfInUcastPkts = 0
	if seq := pollOnce(t, enc, s, pc, start, cur); seq != 2 {
		t.Fatalf("counter-wrap sample sequence = %d, want 2", seq)
	}
	cur.CounterGeneration++
	cur.IfInOctets = 1000
	cur.IfInUcastPkts = 200
	if seq := pollOnce(t, enc, s, pc, start, cur); seq != 1 {
		t.Fatalf("replacement sample sequence = %d, want 1", seq)
	}
}

// TestSFlowUnavailableCountersOnWire checks the collector-visible encoding
// of unavailable counters while retaining a legitimate zero octet count.
func TestSFlowUnavailableCountersOnWire(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	start := time.Now()
	enc := NewCounterEncoder(testAgent, 1, start)
	snap := flowexport.CounterSnapshot{Time: start, Interfaces: []flowexport.InterfaceCounters{{
		IfIndex:            3,
		IfInBroadcastPkts:  flowexport.CounterUnavailable,
		IfInUnknownProtos:  flowexport.CounterUnavailable,
		IfOutMulticastPkts: flowexport.CounterUnavailable,
		IfOutBroadcastPkts: flowexport.CounterUnavailable,
	}}}
	if _, err := enc.Encode(snap, s); err != nil {
		t.Fatal(err)
	}
	dg := decodeDatagram(t, recvFlowDatagram(t, pc))
	if len(dg.samples) != 1 || len(dg.samples[0].data) != 112 {
		t.Fatal("expected one complete counters_sample_expanded")
	}
	fields := dg.samples[0].data[24:]
	for _, offset := range []int{40, 52, 68, 72} {
		if got := binary.BigEndian.Uint32(fields[offset:]); got != flowexport.CounterUnavailable {
			t.Errorf("counter at offset %d = %#x, want unavailable", offset, got)
		}
	}
	if got := binary.BigEndian.Uint64(fields[24:]); got != 0 {
		t.Fatalf("available ifInOctets = %d, want 0", got)
	}
}

// RFC requirement: SFLOW-V5-x-9 positive -- counter and flow datagrams of
// one collector share a wrapping 32-bit sequence, while each sample type
// retains its source sequence across the interleaved datagrams.
func TestSFlowMixedDatagramSequenceWraps(t *testing.T) {
	pc, sender := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = sender.Close() }()
	sender.AdvanceSequence(^uint32(0) - 1)
	start := time.Now()
	counters := NewCounterEncoder(testAgent, 1, start)
	flows := NewFlowEncoder(testAgent, 1, start)
	for i := range 4 {
		if i%2 == 0 {
			if _, err := counters.Encode(flowexport.CounterSnapshot{
				Time:       start,
				Interfaces: []flowexport.InterfaceCounters{{IfIndex: 3}},
			}, sender); err != nil {
				t.Fatal(err)
			}
		} else if err := flows.EncodeFlowSample(flowexport.FlowSample{
			IfIndex: 3, Rate: 10, OrigSize: 64, Header: []byte{1, 2, 3, 4},
		}, sender); err != nil {
			t.Fatal(err)
		}
		wire := recvFlowDatagram(t, pc)
		wantDatagram := (^uint32(0) - 1) + uint32(i)
		if got := binary.BigEndian.Uint32(wire[16:]); got != wantDatagram {
			t.Fatalf("datagram %d sequence = %d, want %d", i, got, wantDatagram)
		}
		dg := decodeDatagram(t, wire)
		wantSample := uint32(i/2 + 1)
		if got := counterSampleSeq(t, dg.samples[0].data); got != wantSample {
			t.Fatalf("sample %d sequence = %d, want %d", i, got, wantSample)
		}
	}
}

// RFC requirement: SFLOW-V5-x-9 negative -- a counter generation reset
// restarts only the source sample sequence, never the datagram sequence.
func TestSFlowCounterResetDoesNotResetDatagrams(t *testing.T) {
	pc, sender := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = sender.Close() }()
	start := time.Now()
	enc := NewCounterEncoder(testAgent, 1, start)
	for i, generation := range []uint64{1, 1, 2} {
		if _, err := enc.Encode(flowexport.CounterSnapshot{
			Time:       start,
			Interfaces: []flowexport.InterfaceCounters{{IfIndex: 3, CounterGeneration: generation}},
		}, sender); err != nil {
			t.Fatal(err)
		}
		wire := recvFlowDatagram(t, pc)
		if got := binary.BigEndian.Uint32(wire[16:]); got != uint32(i) {
			t.Fatalf("datagram %d sequence = %d", i, got)
		}
		dg := decodeDatagram(t, wire)
		want := []uint32{1, 2, 1}[i]
		if got := counterSampleSeq(t, dg.samples[0].data); got != want {
			t.Fatalf("sample %d sequence = %d, want %d", i, got, want)
		}
	}
}
