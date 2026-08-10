// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- IPFIX flow-record encoder adapter
// Related: flow_data.go -- WriteFlowDataSet / FlowRecord
// Related: flow_template.go -- BuildFlowTemplate / FlowTemplateID (v4 + v6)
// Related: register.go -- registers newIPFIXFlowEncoder as the flow-record factory

package ipfix

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func newIPFIXFlowEncoder(cfg flowexport.CollectorConfig, _ time.Time) flowexport.FlowRecordEncoder {
	return NewFlowEncoder(cfg.ObservationDomain)
}

// FlowEncoder implements flowexport.FlowRecordEncoder for IPFIX per-flow
// records sourced from conntrack.
type FlowEncoder struct {
	ObservationDomainID uint32

	seqNum         uint32
	templateBytes  []byte
	templateBytes6 []byte
}

// NewFlowEncoder creates an IPFIX flow-record encoder.
func NewFlowEncoder(observationDomainID uint32) *FlowEncoder {
	return &FlowEncoder{
		ObservationDomainID: observationDomainID,
		templateBytes:       BuildFlowTemplate(),
		templateBytes6:      BuildFlowTemplate6(),
	}
}

// EncodeFlows writes IPFIX messages with per-flow data records. IPv4 and IPv6
// flows use distinct templates (257 / 258), so each family is sent in its own
// message. The returned count sums both families.
func (e *FlowEncoder) EncodeFlows(flows []flowexport.ConntrackFlow, sender *flowexport.Sender) (int, error) {
	if len(flows) == 0 {
		return 0, nil
	}

	recs4 := make([]FlowRecord, 0, len(flows))
	recs6 := make([]FlowRecord, 0, len(flows))
	for i := range flows {
		f := &flows[i]
		rec := FlowRecord{
			SrcAddr:     f.SrcAddr,
			DstAddr:     f.DstAddr,
			SrcPort:     f.SrcPort,
			DstPort:     f.DstPort,
			Protocol:    f.Protocol,
			Bytes:       f.Bytes,
			Packets:     f.Packets,
			SrcAS:       f.SrcAS,
			DstAS:       f.DstAS,
			StartTimeMs: f.FirstMs,
			EndTimeMs:   f.LastMs,
		}
		if f.SrcAddr.Is4() && f.DstAddr.Is4() {
			recs4 = append(recs4, rec)
		} else {
			recs6 = append(recs6, rec)
		}
	}

	total := 0
	if len(recs4) > 0 {
		n, err := e.sendDataMessage(sender, recs4, false)
		if err != nil {
			return total, err
		}
		total += n
	}
	if len(recs6) > 0 {
		n, err := e.sendDataMessage(sender, recs6, true)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// sendDataMessage encodes and sends one IPFIX message for a single address
// family. v6 selects the IPv6 template/Data Set (258); otherwise IPv4 (257).
func (e *FlowEncoder) sendDataMessage(sender *flowexport.Sender, recs []FlowRecord, v6 bool) (int, error) {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)
	b := *buf

	exportTime := uint32(time.Now().Unix())

	off := MessageHeaderSize
	var n int
	var count uint32
	if v6 {
		n, count = writeFlowDataSet6(b, off, recs, FlowTemplateID6)
	} else {
		n, count = WriteFlowDataSet(b, off, recs, FlowTemplateID)
	}
	off += n
	WriteMessageHeader(b, 0, uint16(off), exportTime, e.seqNum, e.ObservationDomainID)
	e.seqNum += count

	if err := sender.Send(b[:off]); err != nil {
		return 0, err
	}
	return int(count), nil
}

// EncodeFlowTemplate sends both the IPv4 and IPv6 per-flow IPFIX Template
// Sets, each in its own message.
func (e *FlowEncoder) EncodeFlowTemplate(sender *flowexport.Sender) error {
	if err := e.sendTemplateMessage(sender, e.templateBytes); err != nil {
		return err
	}
	return e.sendTemplateMessage(sender, e.templateBytes6)
}

// sendTemplateMessage sends one pre-encoded Template Set as an IPFIX message.
func (e *FlowEncoder) sendTemplateMessage(sender *flowexport.Sender, tmpl []byte) error {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)
	b := *buf

	exportTime := uint32(time.Now().Unix())

	off := MessageHeaderSize
	off += copy(b[off:], tmpl)
	WriteMessageHeader(b, 0, uint16(off), exportTime, e.seqNum, e.ObservationDomainID)

	return sender.Send(b[:off])
}
