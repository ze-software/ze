// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- sFlow protocol encoder adapter

package sflow

import (
	"encoding/binary"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// CounterEncoder implements flowexport.ProtocolEncoder for sFlow v5 counter samples.
type CounterEncoder struct {
	AgentAddr  netip.Addr
	SubAgentID uint32
	StartTime  time.Time

	seqNums map[uint32]uint32

	// History is pruned on each complete snapshot, so interface churn cannot
	// retain counters or sequence numbers for sources that have disappeared.
	last map[uint32]counterHistory
}

type counterHistory struct {
	counters flowexport.InterfaceCounters
	present  bool
}

// NewCounterEncoder creates an sFlow counter encoder.
func NewCounterEncoder(agentAddr netip.Addr, subAgentID uint32, startTime time.Time) *CounterEncoder {
	return &CounterEncoder{
		AgentAddr:  agentAddr,
		SubAgentID: subAgentID,
		StartTime:  startTime,
		seqNums:    make(map[uint32]uint32),
		last:       make(map[uint32]counterHistory),
	}
}

// resetSequencesOnDiscontinuity restarts source sequences when the raw
// interface counter generation changes. Sources without metadata fall back
// to decreases visible between snapshots; supported Linux sources supply
// full-width continuity metadata before sFlow truncates counters to 32 bits.
func (e *CounterEncoder) resetSequencesOnDiscontinuity(ifaces []flowexport.InterfaceCounters) {
	for index, history := range e.last {
		history.present = false
		e.last[index] = history
	}
	for i := range ifaces {
		cur := &ifaces[i]
		prev, seen := e.last[cur.IfIndex]
		discontinuous := cur.CounterGeneration != prev.counters.CounterGeneration
		if cur.CounterGeneration == 0 {
			discontinuous = discontinuous || countersWentBackwards(&prev.counters, cur)
		}
		if seen && discontinuous {
			e.seqNums[cur.IfIndex] = 0
		}
		e.last[cur.IfIndex] = counterHistory{counters: *cur, present: true}
	}
	for index, history := range e.last {
		if !history.present {
			delete(e.last, index)
			delete(e.seqNums, index)
		}
	}
}

// countersWentBackwards reports whether any cumulative counter of cur is
// below the same counter of prev. A field marked CounterUnavailable never
// moves, so it never reports a reset. A 32-bit wrap also appears as a
// decrease because the exported snapshot contains the truncated value.
func countersWentBackwards(prev, cur *flowexport.InterfaceCounters) bool {
	if cur.IfInOctets < prev.IfInOctets || cur.IfOutOctets < prev.IfOutOctets {
		return true
	}
	pairs := [...][2]uint32{
		{prev.IfInUcastPkts, cur.IfInUcastPkts},
		{prev.IfInMulticastPkts, cur.IfInMulticastPkts},
		{prev.IfInBroadcastPkts, cur.IfInBroadcastPkts},
		{prev.IfInDiscards, cur.IfInDiscards},
		{prev.IfInErrors, cur.IfInErrors},
		{prev.IfInUnknownProtos, cur.IfInUnknownProtos},
		{prev.IfOutUcastPkts, cur.IfOutUcastPkts},
		{prev.IfOutMulticastPkts, cur.IfOutMulticastPkts},
		{prev.IfOutBroadcastPkts, cur.IfOutBroadcastPkts},
		{prev.IfOutDiscards, cur.IfOutDiscards},
		{prev.IfOutErrors, cur.IfOutErrors},
	}
	for _, p := range pairs {
		if p[1] < p[0] {
			return true
		}
	}
	return false
}

// Encode writes sFlow v5 counter datagrams and sends them.
func (e *CounterEncoder) Encode(snap flowexport.CounterSnapshot, sender *flowexport.Sender) (int, error) {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)

	// sFlow v5: uptime = milliseconds since agent start, recomputed each cycle
	uptime := uint32(snap.Time.Sub(e.StartTime).Milliseconds())

	e.resetSequencesOnDiscontinuity(snap.Interfaces)
	datagrams, nextSeq := writeCounterDatagrams(
		(*buf)[:sender.MaxDatagram()], e.AgentAddr, e.SubAgentID,
		sender.Sequence(), uptime,
		snap.Interfaces, e.seqNums,
	)
	sender.AdvanceSequence(nextSeq - sender.Sequence())

	sent := 0
	for _, dg := range datagrams {
		if err := sender.Send(dg); err != nil {
			return sent, err
		}
		sent += int(binary.BigEndian.Uint32(dg[HeaderSize(e.AgentAddr)-4:]))
	}
	return sent, nil
}

// EncodeTemplate is a no-op for sFlow (no template concept).
func (e *CounterEncoder) EncodeTemplate(_ *flowexport.Sender) error {
	return nil
}
