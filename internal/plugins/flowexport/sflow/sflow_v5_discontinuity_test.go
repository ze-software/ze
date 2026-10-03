// sFlow v5 conformance tests for the counter sequence reset that an
// ifCounterDiscontinuityTime change forces. CounterGeneration is the value
// the interface source changes on each counter discontinuity, which is the
// role ifCounterDiscontinuityTime plays for an ifIndex-based source_id.

package sflow

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// RFC requirement: SFLOW-V5-x-30 positive -- when the CounterGeneration of ifIndex source 3 changes while every counter keeps growing, the counters_sample of that poll carries sequence_number 1 again.
func TestSFlowV5SequenceResetOnDiscontinuityTime(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewCounterEncoder(testAgent, 1, time.Unix(1716000000, 0))
	start := time.Unix(1716000100, 0)

	if seq := pollOnce(t, enc, s, pc, start, flowexport.InterfaceCounters{IfIndex: 3, CounterGeneration: 7, IfInOctets: 1000}); seq != 1 {
		t.Fatalf("first poll sequence = %d, want 1", seq)
	}
	if seq := pollOnce(t, enc, s, pc, start.Add(20*time.Second), flowexport.InterfaceCounters{IfIndex: 3, CounterGeneration: 7, IfInOctets: 2000}); seq != 2 {
		t.Fatalf("second poll sequence = %d, want 2", seq)
	}
	if seq := pollOnce(t, enc, s, pc, start.Add(40*time.Second), flowexport.InterfaceCounters{IfIndex: 3, CounterGeneration: 8, IfInOctets: 3000}); seq != 1 {
		t.Fatalf("sequence after the discontinuity generation changed = %d, want reset to 1", seq)
	}
}

// RFC requirement: SFLOW-V5-x-30 negative -- while the CounterGeneration of source 3 stays the same, the sequence_number is never reset: three polls carry 1, 2, 3 even when a counter reads lower, because the unchanged generation says no discontinuity happened.
func TestSFlowV5SequenceKeptWhileDiscontinuityTimeSteady(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewCounterEncoder(testAgent, 1, time.Unix(1716000000, 0))
	start := time.Unix(1716000100, 0)

	octets := []uint64{1000, 2000, 1500} // a 32-bit wrap reads lower, not a discontinuity
	for i, want := range []uint32{1, 2, 3} {
		seq := pollOnce(t, enc, s, pc, start.Add(time.Duration(i)*20*time.Second),
			flowexport.InterfaceCounters{IfIndex: 3, CounterGeneration: 7, IfInOctets: octets[i]})
		if seq != want {
			t.Fatalf("poll %d: sequence = %d, want %d", i+1, seq, want)
		}
	}
}
