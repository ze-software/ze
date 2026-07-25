// Design: plan/learned/819-flow-export-2-flow-records.md -- sFlow flow-sample encoder adapter
// Related: flow.go -- WriteFlowSample / WriteSampledHeader record writers
// Related: register.go -- registers newSFlowFlowEncoder as the flow-sample factory

package sflow

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func newSFlowFlowEncoder(cfg flowexport.CollectorConfig, startTime time.Time) flowexport.FlowSampleEncoder {
	agentAddr := netip.IPv4Unspecified()
	if cfg.AgentAddress != "" {
		if parsed, err := netip.ParseAddr(cfg.AgentAddress); err == nil {
			agentAddr = parsed
		}
	}
	return NewFlowEncoder(agentAddr, cfg.SubAgentID, startTime)
}

// FlowEncoder implements flowexport.FlowSampleEncoder, emitting one sFlow v5
// datagram per sampled packet with a single flow_sample containing a
// sampled_header record.
type FlowEncoder struct {
	AgentAddr  netip.Addr
	SubAgentID uint32
	StartTime  time.Time

	datagramSeq uint32
	seqNums     map[uint32]uint32 // per-source (ifIndex) sample sequence
}

// NewFlowEncoder creates an sFlow flow-sample encoder.
func NewFlowEncoder(agentAddr netip.Addr, subAgentID uint32, startTime time.Time) *FlowEncoder {
	return &FlowEncoder{
		AgentAddr:  agentAddr,
		SubAgentID: subAgentID,
		StartTime:  startTime,
		seqNums:    make(map[uint32]uint32),
	}
}

// flowSampleOverhead is the worst-case byte cost of the datagram header
// contributions other than the captured packet header: flow_sample header
// (40) + sampled_header fixed fields (24) + XDR padding slack (4).
const flowSampleOverhead = 40 + 24 + 4

// EncodeFlowSample assembles and sends one sFlow v5 flow_sample datagram.
// The captured header is truncated if needed so the datagram stays within
// MaxDatagramSize (sFlow permits captured length < frame_length).
func (e *FlowEncoder) EncodeFlowSample(sample flowexport.FlowSample, sender *flowexport.Sender) error {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)
	b := *buf

	uptime := uint32(time.Since(e.StartTime).Milliseconds())

	off := WriteDatagramHeader(b, 0, e.AgentAddr, e.SubAgentID, e.datagramSeq, uptime, 1)

	seq := e.seqNums[sample.IfIndex] + 1
	e.seqNums[sample.IfIndex] = seq

	hdr := sample.Header
	if maxHdr := flowexport.MaxDatagramSize - off - flowSampleOverhead; maxHdr < 0 {
		hdr = nil
	} else if len(hdr) > maxHdr {
		hdr = hdr[:maxHdr]
	}

	// sample_pool ~= cumulative samples * rate (packets that could be sampled).
	// Compute in uint64 and saturate so a high rate * large seq does not wrap
	// the uint32 field and report a misleadingly small pool to the collector.
	pool := uint32(min(uint64(seq)*uint64(sample.Rate), uint64(^uint32(0))))

	fsOff, sampleLengthOff, numRecordsOff := WriteFlowSample(
		b, off, seq, sample.IfIndex, sample.Rate, pool, 0, sample.IfIndex, sample.Output)
	off = WriteSampledHeader(b, fsOff, HeaderProtocolEthernet, sample.OrigSize, 0, hdr)
	BackfillFlowSample(b, sampleLengthOff, numRecordsOff, off, 1)

	e.datagramSeq++
	return sender.Send(b[:off])
}
