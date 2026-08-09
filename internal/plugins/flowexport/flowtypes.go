// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- flow-record value types and encoder interfaces
// Related: encoder_registry.go -- flow encoder factory registration
// Related: exporter.go -- ExportFlowSample / ExportFlows dispatch

package flowexport

import "net/netip"

// FlowSample is a packet sampled by the kernel (tc sample + psample),
// ready for sFlow flow_sample encoding. Value type, no shared pointers
// across the iface/kernel boundary: Header is an owned copy.
type FlowSample struct {
	IfIndex  uint32
	Rate     uint32 // 1-in-N sampling rate
	OrigSize uint32 // original frame length on the wire
	Output   uint32 // egress ifIndex, 0 if unknown
	Header   []byte // first trunc-size bytes of the sampled frame
}

// ConntrackFlow is a per-flow record sourced from a conntrack entry with
// optional BGP enrichment, ready for NetFlow v9 / IPFIX flow encoding.
// Counters are deltas since the previous export. Timestamps are absolute
// Unix milliseconds; each protocol encoder converts to its own time base.
// Value type, safe to copy.
type ConntrackFlow struct {
	SrcAddr  netip.Addr
	DstAddr  netip.Addr
	SrcPort  uint16
	DstPort  uint16
	Protocol uint8
	Bytes    uint64
	Packets  uint64
	FirstMs  uint64 // Unix ms of the first packet of this flow
	LastMs   uint64 // Unix ms of the most recent packet of this flow
	SrcAS    uint32 // BGP origin AS of SrcAddr (0 if unknown)
	DstAS    uint32 // BGP origin AS of DstAddr (0 if unknown)
	NextHop  netip.Addr
	TCPState uint8 // conntrack TCP state (nf_conntrack_tcp), 0 for non-TCP; feeds SYN-flood classification
}

// FlowSampleEncoder encodes a single sFlow flow_sample datagram and sends
// it. Implemented by the sflow package; registered via
// RegisterFlowSampleEncoderFactory.
type FlowSampleEncoder interface {
	EncodeFlowSample(sample FlowSample, sender *Sender) error
}

// FlowRecordEncoder encodes per-flow records (NetFlow v9, IPFIX) into one
// or more datagrams and sends them. Implemented by the netflow9 and ipfix
// packages; registered via RegisterFlowRecordEncoderFactory.
type FlowRecordEncoder interface {
	// EncodeFlows writes per-flow data records and sends them. Returns the
	// number of data records exported (for IPFIX sequence counting).
	EncodeFlows(flows []ConntrackFlow, sender *Sender) (dataRecords int, err error)

	// EncodeFlowTemplate sends the per-flow template (NetFlow v9, IPFIX).
	EncodeFlowTemplate(sender *Sender) error
}
