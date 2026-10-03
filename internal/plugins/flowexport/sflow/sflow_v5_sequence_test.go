// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- sFlow sample sequence numbers
// Related: counters_sflow_v5_test.go -- pollOnce, the ifInOctets reset case
//
// VALIDATES: a counters_sample sequence_number restarts when ANY cumulative
// counter of its source goes backwards, not only ifInOctets, and a
// flow_sample sequence_number counts the samples of its own source_id only.
// PREVENTS: a discontinuity detector that watches one counter, and one flow
// sequence shared across data sources.

package sflow

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// risingCounters returns source 3 with every cumulative counter set to base.
func risingCounters(base uint32) flowexport.InterfaceCounters {
	return flowexport.InterfaceCounters{
		IfIndex: 3, IfInOctets: uint64(base), IfOutOctets: uint64(base),
		IfInUcastPkts: base, IfInMulticastPkts: base, IfInBroadcastPkts: base,
		IfInDiscards: base, IfInErrors: base, IfInUnknownProtos: base,
		IfOutUcastPkts: base, IfOutMulticastPkts: base, IfOutBroadcastPkts: base,
		IfOutDiscards: base, IfOutErrors: base,
	}
}

// RFC requirement: SFLOW-V5-x-29 positive -- for each of the thirteen cumulative
// if_counters of source 3, that one counter going backwards while every other
// counter keeps rising restarts the counters_sample sequence_number at 1.
func TestSFlowV5SequenceResetOnAnyCounterGoingBack(t *testing.T) {
	for name, rewind := range map[string]func(*flowexport.InterfaceCounters){
		"ifInOctets":         func(c *flowexport.InterfaceCounters) { c.IfInOctets = 500 },
		"ifOutOctets":        func(c *flowexport.InterfaceCounters) { c.IfOutOctets = 500 },
		"ifInUcastPkts":      func(c *flowexport.InterfaceCounters) { c.IfInUcastPkts = 500 },
		"ifInMulticastPkts":  func(c *flowexport.InterfaceCounters) { c.IfInMulticastPkts = 500 },
		"ifInBroadcastPkts":  func(c *flowexport.InterfaceCounters) { c.IfInBroadcastPkts = 500 },
		"ifInDiscards":       func(c *flowexport.InterfaceCounters) { c.IfInDiscards = 500 },
		"ifInErrors":         func(c *flowexport.InterfaceCounters) { c.IfInErrors = 500 },
		"ifInUnknownProtos":  func(c *flowexport.InterfaceCounters) { c.IfInUnknownProtos = 500 },
		"ifOutUcastPkts":     func(c *flowexport.InterfaceCounters) { c.IfOutUcastPkts = 500 },
		"ifOutMulticastPkts": func(c *flowexport.InterfaceCounters) { c.IfOutMulticastPkts = 500 },
		"ifOutBroadcastPkts": func(c *flowexport.InterfaceCounters) { c.IfOutBroadcastPkts = 500 },
		"ifOutDiscards":      func(c *flowexport.InterfaceCounters) { c.IfOutDiscards = 500 },
		"ifOutErrors":        func(c *flowexport.InterfaceCounters) { c.IfOutErrors = 500 },
	} {
		t.Run(name, func(t *testing.T) {
			pc, s := newLoopbackFlowTarget(t)
			defer func() { _ = pc.Close() }()
			defer func() { _ = s.Close() }()
			enc := NewCounterEncoder(testAgent, 1, time.Unix(1716000000, 0))
			start := time.Unix(1716000100, 0)

			pollOnce(t, enc, s, pc, start, risingCounters(1000))
			if seq := pollOnce(t, enc, s, pc, start.Add(20*time.Second), risingCounters(2000)); seq != 2 {
				t.Fatalf("second poll sequence = %d, want 2", seq)
			}
			third := risingCounters(3000)
			rewind(&third)
			if seq := pollOnce(t, enc, s, pc, start.Add(40*time.Second), third); seq != 1 {
				t.Fatalf("sequence after %s went backwards = %d, want reset to 1", name, seq)
			}
		})
	}
}

// RFC requirement: SFLOW-V5-x-28 positive -- the flow_sample sequence_number is
// incremented with each flow sample of its own source_id: samples interleaved
// from sources 3 and 4 carry 1, 2, 3 for source 3 and 1, 2 for source 4.
func TestSFlowV5FlowSequencePerSourceID(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewFlowEncoder(netip.MustParseAddr("192.0.2.1"), 0, time.Now())

	for i, step := range []struct{ source, want uint32 }{{3, 1}, {4, 1}, {3, 2}, {3, 3}, {4, 2}} {
		sample := flowexport.FlowSample{IfIndex: step.source, Rate: 100, OrigSize: 64, Header: make([]byte, 20)}
		if err := enc.EncodeFlowSample(sample, s); err != nil {
			t.Fatal(err)
		}
		dg := recvFlowDatagram(t, pc)
		if got := binary.BigEndian.Uint32(dg[HeaderSizeIPv4+16:]); got != step.source {
			t.Fatalf("sample %d: source_id_index = %d, want %d", i, got, step.source)
		}
		if got := binary.BigEndian.Uint32(dg[HeaderSizeIPv4+8:]); got != step.want {
			t.Errorf("sample %d: source %d sequence_number = %d, want %d", i, step.source, got, step.want)
		}
	}
}
