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

	templateBytes  []byte
	templateBytes6 []byte

	// templateExportTime retains the last successfully sent Template's time.
	// Both families share the floor so a wall-clock rollback cannot put
	// their subsequent Template or Data messages before that Template.
	templateExportTime uint32
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
// messages, bounded by the collector's max-datagram-size. The returned count
// sums records successfully sent for both families.
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

	// Attempt both families and report successful records even if one fails.
	total := 0
	var sendErr error
	if len(recs4) > 0 {
		n, err := e.sendDataMessage(sender, recs4, false)
		if err != nil {
			sendErr = err
		}
		total += n
	}
	if len(recs6) > 0 {
		n, err := e.sendDataMessage(sender, recs6, true)
		if err != nil && sendErr == nil {
			sendErr = err
		}
		total += n
	}
	return total, sendErr
}

// sendDataMessage encodes and sends the IPFIX messages for a single address
// family. v6 selects the IPv6 template/Data Set (258); otherwise IPv4 (257).
// Records are chunked so each datagram stays within the collector's
// max-datagram-size. Failed chunks are discarded without advancing the UDP
// sequence number. The first error is returned after all chunks are attempted.
func (e *FlowEncoder) sendDataMessage(sender *flowexport.Sender, recs []FlowRecord, v6 bool) (int, error) {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)
	b := *buf

	// RFC 7011 Section 8.2: "An Exporting Process MUST NOT export a Data Set
	// described by a new Template in an IPFIX Message with an Export Time
	// before the Export Time of the IPFIX Message containing that Template."
	exportTime := max(uint32(time.Now().Unix()), e.templateExportTime)
	maxPer := maxFlowRecordsPerDatagram(v6, sender.MaxDatagram())

	total := 0
	var sendErr error
	for start := 0; start < len(recs); start += maxPer {
		end := min(start+maxPer, len(recs))

		off := MessageHeaderSize
		var n int
		var count uint32
		if v6 {
			n, count = writeFlowDataSet6(b, off, recs[start:end], FlowTemplateID6)
		} else {
			n, count = WriteFlowDataSet(b, off, recs[start:end], FlowTemplateID)
		}
		off += n
		WriteMessageHeader(b, 0, uint16(off), exportTime, sender.Sequence(), e.ObservationDomainID)

		if err := sender.Send(b[:off]); err != nil {
			if sendErr == nil {
				sendErr = err
			}
			continue
		}
		// RFC 7011 Section 10.3.2: "In the case of UDP, the IPFIX Sequence
		// Number contains the total number of IPFIX Data Records sent for
		// the Transport Session prior to the receipt of this IPFIX Message,
		// modulo 2^32."
		sender.AdvanceSequence(count)
		total += int(count)
	}
	return total, sendErr
}

// maxFlowRecordsPerDatagram is how many flow records of one family fit one
// datagram of maxDatagram octets after the message header and Data Set
// header. At least one.
func maxFlowRecordsPerDatagram(v6 bool, maxDatagram int) int {
	recSize := FlowRecordSize()
	if v6 {
		recSize = FlowRecordSize6()
	}
	if recSize <= 0 {
		return 1
	}
	// The Data Set writer pads to four octets. Round the budget down before
	// counting records so padding cannot exceed an odd configured bound.
	n := ((maxDatagram &^ 3) - MessageHeaderSize - 4) / recSize
	if n < 1 {
		return 1
	}
	return n
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

	// RFC 7011 Section 8.2: "an Exporting Process MUST sequence all Template
	// management actions (i.e., Template Records defining new Templates and
	// Template Withdrawals withdrawing them) using the Export Time field in
	// the IPFIX Message Header."
	exportTime := max(uint32(time.Now().Unix()), e.templateExportTime)

	off := MessageHeaderSize
	off += copy(b[off:], tmpl)
	WriteMessageHeader(b, 0, uint16(off), exportTime, sender.Sequence(), e.ObservationDomainID)

	if err := sender.Send(b[:off]); err != nil {
		return err
	}
	e.templateExportTime = exportTime
	return nil
}
