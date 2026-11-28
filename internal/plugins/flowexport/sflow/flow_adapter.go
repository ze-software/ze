// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- sFlow flow-sample encoder adapter
// Related: flow.go -- WriteFlowSample / WriteSampledHeader record writers
// Related: register.go -- registers newSFlowFlowEncoder as the flow-sample factory

package sflow

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func newSFlowFlowEncoder(cfg flowexport.CollectorConfig, startTime time.Time) flowexport.FlowSampleEncoder {
	return NewFlowEncoder(agentAddress(cfg), cfg.SubAgentID, startTime)
}

// sourceState holds the per-source (ifIndex) flow-sample sequence and pool
// estimate so they are created and reset together (sFlow v5 Section 5).
type sourceState struct {
	seq  uint32
	pool uint32
}

// FlowEncoder implements flowexport.FlowSampleEncoder, emitting one sFlow v5
// datagram per sampled packet with a single flow_sample containing a
// sampled_header record.
type FlowEncoder struct {
	AgentAddr  netip.Addr
	SubAgentID uint32
	StartTime  time.Time

	sources map[uint32]sourceState // per-source (ifIndex) sample sequence and pool
}

// NewFlowEncoder creates an sFlow flow-sample encoder.
func NewFlowEncoder(agentAddr netip.Addr, subAgentID uint32, startTime time.Time) *FlowEncoder {
	return &FlowEncoder{
		AgentAddr:  agentAddr,
		SubAgentID: subAgentID,
		StartTime:  startTime,
		sources:    make(map[uint32]sourceState),
	}
}

// ethernetFCSOctets is the frame check sequence the NIC removes before the
// kernel sees the frame, so psample's original size is 4 octets short of
// what was on the wire. sFlow v5, struct sampled_header: frame_length for a
// layer 2 header_protocol is "total number of octets of data received on the
// network (excluding framing bits but including FCS octets)", and "Any
// octets added to the frame_length to compensate for encapsulations removed
// by the underlying hardware must also be added to the stripped count.".
const ethernetFCSOctets = 4

// EncodeFlowSample assembles and sends one sFlow v5 flow_sample datagram.
// The captured header is truncated if needed so the datagram stays within
// the collector's max-datagram-size (sFlow permits captured length <
// frame_length).
func (e *FlowEncoder) EncodeFlowSample(sample flowexport.FlowSample, sender *flowexport.Sender) error {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)
	b := *buf

	uptime := uint32(time.Since(e.StartTime).Milliseconds())

	off := WriteDatagramHeader(b, 0, e.AgentAddr, e.SubAgentID, sender.Sequence(), uptime, 1)

	// sFlow v5 Section 5 (flow_sample / flow_sample_expanded):
	//   "unsigned int sample_pool; /* Total number of packets that could have
	//    been sampled (i.e. packets skipped by sampling process + total number
	//    of samples) */"
	//   "Note: If the agent resets the sample_pool then it must also reset the
	//    sequence_number."
	//   Counter conventions: "Unknown counter. Use the maximum counter value to
	//    indicate that the counter is not available. ... An available counter
	//    may temporarily have the max value just before it rolls to zero. This
	//    is permitted."
	//
	// Linux psample does not carry per-sample skipped-packet totals, so Ze
	// maintains a per-source accumulated rate estimate beside sequence_number.
	// Unsigned 32-bit addition wraps modulo 2^32 instead of saturating at
	// MaxUint32 (which collectors interpret as an unavailable counter), and
	// storing seq and pool together ensures they are created and reset together.
	state := e.sources[sample.IfIndex]
	state.seq++
	state.pool += sample.Rate
	e.sources[sample.IfIndex] = state

	hdr := sample.Header
	// Use expanded samples for every interface: Linux permits full-width
	// ifIndex, and sFlow forbids mixing compact and expanded samples.
	if maxHdr := sender.MaxDatagram() - off - flowSampleHeaderSize() - 24 - 4; maxHdr < 0 {
		hdr = nil
	} else if len(hdr) > maxHdr {
		hdr = hdr[:maxHdr]
	}

	fsOff, sampleLengthOff, numRecordsOff := writeFlowSample(
		b, off, state.seq, sample.IfIndex, sample.Rate, state.pool, 0, sample.IfIndex, sample.Output)
	off = writeSampledHeader(b, fsOff, sample.OrigSize+ethernetFCSOctets, ethernetFCSOctets, hdr)
	backfillFlowSample(b, sampleLengthOff, numRecordsOff, off, 1)

	sender.AdvanceSequence(1)
	return sender.Send(b[:off])
}
