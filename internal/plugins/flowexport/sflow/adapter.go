// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- sFlow protocol encoder adapter

package sflow

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// CounterEncoder implements flowexport.ProtocolEncoder for sFlow v5 counter samples.
type CounterEncoder struct {
	AgentAddr  netip.Addr
	SubAgentID uint32
	StartTime  time.Time

	datagramSeq uint32
	seqNums     map[uint32]uint32
}

// NewCounterEncoder creates an sFlow counter encoder.
func NewCounterEncoder(agentAddr netip.Addr, subAgentID uint32, startTime time.Time) *CounterEncoder {
	return &CounterEncoder{
		AgentAddr:  agentAddr,
		SubAgentID: subAgentID,
		StartTime:  startTime,
		seqNums:    make(map[uint32]uint32),
	}
}

// Encode writes sFlow v5 counter datagrams and sends them.
func (e *CounterEncoder) Encode(snap flowexport.CounterSnapshot, sender *flowexport.Sender) (int, error) {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)

	// sFlow v5: uptime = milliseconds since agent start, recomputed each cycle
	uptime := uint32(snap.Time.Sub(e.StartTime).Milliseconds())

	datagrams, nextSeq := WriteCounterDatagrams(
		*buf, e.AgentAddr, e.SubAgentID,
		e.datagramSeq, uptime,
		snap.Interfaces, e.seqNums,
	)
	e.datagramSeq = nextSeq

	for _, dg := range datagrams {
		if err := sender.Send(dg); err != nil {
			return 0, err
		}
	}
	return len(snap.Interfaces), nil
}

// EncodeTemplate is a no-op for sFlow (no template concept).
func (e *CounterEncoder) EncodeTemplate(_ *flowexport.Sender) error {
	return nil
}
