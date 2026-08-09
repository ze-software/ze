// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- IPFIX protocol encoder adapter

package ipfix

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// CounterEncoder implements flowexport.ProtocolEncoder for IPFIX counter records.
type CounterEncoder struct {
	ObservationDomainID uint32

	seqNum        uint32
	templateBytes []byte
}

// NewCounterEncoder creates an IPFIX counter encoder.
func NewCounterEncoder(observationDomainID uint32) *CounterEncoder {
	return &CounterEncoder{
		ObservationDomainID: observationDomainID,
		templateBytes:       BuildCounterTemplate(),
	}
}

// Encode writes IPFIX message(s) with counter data and sends them. Interface
// records are chunked so each datagram stays within MaxDatagramSize; a device
// with more interfaces than fit one datagram produces several, with the
// sequence number advancing by the data-record count per RFC 7011.
func (e *CounterEncoder) Encode(snap flowexport.CounterSnapshot, sender *flowexport.Sender) (int, error) {
	if len(snap.Interfaces) == 0 {
		return 0, nil
	}

	exportTime := uint32(snap.Time.Unix())
	maxPer := maxCounterRecordsPerDatagram()

	total := 0
	for start := 0; start < len(snap.Interfaces); start += maxPer {
		end := min(start+maxPer, len(snap.Interfaces))

		buf := flowexport.GetBuf()
		n, dataRecords := WriteMessage(
			*buf, exportTime, e.seqNum, e.ObservationDomainID,
			nil, false,
			snap.Interfaces[start:end],
			exportTime, exportTime,
		)
		err := sender.Send((*buf)[:n])
		flowexport.PutBuf(buf)
		if err != nil {
			return total, err
		}
		// RFC 7011: the sequence number counts Data Records actually sent;
		// advance only after a successful send so a send failure does not open
		// a phantom gap at the collector.
		e.seqNum += dataRecords
		total += int(dataRecords)
	}
	return total, nil
}

// maxCounterRecordsPerDatagram is how many counter records fit one datagram
// after the message header and Data Set header. At least one.
func maxCounterRecordsPerDatagram() int {
	recSize := CounterRecordSize()
	if recSize <= 0 {
		return 1
	}
	n := (flowexport.MaxDatagramSize - MessageHeaderSize - 4) / recSize
	if n < 1 {
		return 1
	}
	return n
}

// EncodeTemplate sends the IPFIX template Set.
func (e *CounterEncoder) EncodeTemplate(sender *flowexport.Sender) error {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)

	exportTime := uint32(time.Now().Unix())

	n, _ := WriteMessage(
		*buf, exportTime, e.seqNum, e.ObservationDomainID,
		e.templateBytes, true,
		nil, 0, 0,
	)

	return sender.Send((*buf)[:n])
}
