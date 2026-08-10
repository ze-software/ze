// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- NetFlow v9 flow-record encoder adapter
// Related: flow_data.go -- WriteFlowDataFlowSet / FlowRecord
// Related: flow_template.go -- BuildFlowTemplate / FlowTemplateID (v4 + v6)
// Related: register.go -- registers newNetflow9FlowEncoder as the flow-record factory

package netflow9

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func newNetflow9FlowEncoder(cfg flowexport.CollectorConfig, startTime time.Time) flowexport.FlowRecordEncoder {
	return NewFlowEncoder(cfg.ObservationDomain, startTime)
}

// FlowEncoder implements flowexport.FlowRecordEncoder for NetFlow v9 per-flow
// records sourced from conntrack.
type FlowEncoder struct {
	SourceID  uint32
	StartTime time.Time

	seqNum         uint32
	templateBytes  []byte
	templateBytes6 []byte
}

// NewFlowEncoder creates a NetFlow v9 flow-record encoder.
func NewFlowEncoder(sourceID uint32, startTime time.Time) *FlowEncoder {
	return &FlowEncoder{
		SourceID:       sourceID,
		StartTime:      startTime,
		templateBytes:  BuildFlowTemplate(),
		templateBytes6: BuildFlowTemplate6(),
	}
}

// EncodeFlows writes NetFlow v9 export packets with per-flow data records.
// IPv4 and IPv6 flows use distinct templates (257 / 258), so each family is
// sent in its own datagram. The returned count sums both families.
func (e *FlowEncoder) EncodeFlows(flows []flowexport.ConntrackFlow, sender *flowexport.Sender) (int, error) {
	if len(flows) == 0 {
		return 0, nil
	}

	now := time.Now()
	sysUpTime := uint32(now.Sub(e.StartTime).Milliseconds())
	unixSecs := uint32(now.Unix())
	startMs := uint64(e.StartTime.UnixMilli())

	recs4 := make([]FlowRecord, 0, len(flows))
	recs6 := make([]FlowRecord, 0, len(flows))
	for i := range flows {
		f := &flows[i]
		rec := FlowRecord{
			SrcAddr:       f.SrcAddr,
			DstAddr:       f.DstAddr,
			SrcPort:       f.SrcPort,
			DstPort:       f.DstPort,
			Protocol:      f.Protocol,
			Bytes:         f.Bytes,
			Packets:       uint32(f.Packets),
			SrcAS:         f.SrcAS,
			DstAS:         f.DstAS,
			FirstSwitched: relUpTime(f.FirstMs, startMs),
			LastSwitched:  relUpTime(f.LastMs, startMs),
		}
		if f.SrcAddr.Is4() && f.DstAddr.Is4() {
			recs4 = append(recs4, rec)
		} else {
			recs6 = append(recs6, rec)
		}
	}

	total := 0
	if len(recs4) > 0 {
		n, err := e.sendDataPacket(sender, recs4, false, sysUpTime, unixSecs)
		if err != nil {
			return total, err
		}
		total += n
	}
	if len(recs6) > 0 {
		n, err := e.sendDataPacket(sender, recs6, true, sysUpTime, unixSecs)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// sendDataPacket encodes and sends one export packet for a single address
// family. v6 selects the IPv6 template/data FlowSet (258); otherwise IPv4 (257).
func (e *FlowEncoder) sendDataPacket(sender *flowexport.Sender, recs []FlowRecord, v6 bool, sysUpTime, unixSecs uint32) (int, error) {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)
	b := *buf

	off := HeaderSize
	var n int
	var count uint16
	if v6 {
		n, count = writeFlowDataFlowSet6(b, off, recs)
	} else {
		n, count = writeFlowDataFlowSet(b, off, recs)
	}
	off += n
	writePacketHeader(b, 0, count, sysUpTime, unixSecs, e.seqNum, e.SourceID)
	// RFC 3954 Section 5.1: the sequence number counts EXPORT PACKETS, not
	// records or flows. Advance by one per datagram (matches the counter
	// encoder and the template path), so collectors do not see false loss.
	e.seqNum++

	if err := sender.Send(b[:off]); err != nil {
		return 0, err
	}
	return int(count), nil
}

// EncodeFlowTemplate sends both the IPv4 and IPv6 per-flow template FlowSets,
// each in its own datagram (seq increments per datagram).
func (e *FlowEncoder) EncodeFlowTemplate(sender *flowexport.Sender) error {
	now := time.Now()
	sysUpTime := uint32(now.Sub(e.StartTime).Milliseconds())
	unixSecs := uint32(now.Unix())

	if err := e.sendTemplatePacket(sender, e.templateBytes, sysUpTime, unixSecs); err != nil {
		return err
	}
	return e.sendTemplatePacket(sender, e.templateBytes6, sysUpTime, unixSecs)
}

// sendTemplatePacket sends one pre-encoded template FlowSet as a datagram.
func (e *FlowEncoder) sendTemplatePacket(sender *flowexport.Sender, tmpl []byte, sysUpTime, unixSecs uint32) error {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)
	b := *buf

	off := HeaderSize
	off += copy(b[off:], tmpl)
	writePacketHeader(b, 0, 1, sysUpTime, unixSecs, e.seqNum, e.SourceID)
	e.seqNum++

	return sender.Send(b[:off])
}

// relUpTime converts an absolute Unix-ms timestamp to a sysUpTime-relative
// millisecond value (NetFlow v9 FIRST_SWITCHED / LAST_SWITCHED). Flows that
// began before the exporter started clamp to 0.
func relUpTime(ms, startMs uint64) uint32 {
	if ms <= startMs {
		return 0
	}
	return uint32(ms - startMs)
}
