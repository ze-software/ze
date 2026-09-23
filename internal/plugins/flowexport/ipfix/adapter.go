// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- IPFIX protocol encoder adapter

package ipfix

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// CounterEncoder implements flowexport.ProtocolEncoder for IPFIX counter records.
type CounterEncoder struct {
	ObservationDomainID uint32

	templateBytes []byte

	// templateExportTime retains the last successfully sent Template's time.
	// Template refreshes and Data messages use it as a floor after clock
	// rollback; snapshot record timestamps stay unchanged.
	templateExportTime uint32
}

// NewCounterEncoder creates an IPFIX counter encoder.
func NewCounterEncoder(observationDomainID uint32) *CounterEncoder {
	return &CounterEncoder{
		ObservationDomainID: observationDomainID,
		templateBytes:       BuildCounterTemplate(),
	}
}

// Encode writes IPFIX message(s) with counter data and sends them. Interface
// records are chunked so each datagram stays within the collector's
// max-datagram-size. The sequence counts records successfully sent over UDP.
// Failed chunks are discarded, and the first error is returned after all
// chunks have been attempted.
func (e *CounterEncoder) Encode(snap flowexport.CounterSnapshot, sender *flowexport.Sender) (int, error) {
	if len(snap.Interfaces) == 0 {
		return 0, nil
	}

	// The records keep the snapshot time; only the message header is clamped.
	snapTime := uint32(snap.Time.Unix())
	// RFC 7011 Section 8.2: "An Exporting Process MUST NOT export a Data Set
	// described by a new Template in an IPFIX Message with an Export Time
	// before the Export Time of the IPFIX Message containing that Template."
	exportTime := max(snapTime, e.templateExportTime)
	maxPer := maxCounterRecordsPerDatagram(sender.MaxDatagram())

	total := 0
	var sendErr error
	for start := 0; start < len(snap.Interfaces); start += maxPer {
		end := min(start+maxPer, len(snap.Interfaces))

		buf := flowexport.GetBuf()
		n, dataRecords := WriteMessage(
			*buf, exportTime, sender.Sequence(), e.ObservationDomainID,
			nil, false,
			snap.Interfaces[start:end],
			snapTime, snapTime,
		)
		err := sender.Send((*buf)[:n])
		flowexport.PutBuf(buf)
		if err != nil {
			if sendErr == nil {
				sendErr = err
			}
			continue
		}
		// RFC 7011 Section 10.3.2: "In the case of UDP, the IPFIX Sequence
		// Number contains the total number of IPFIX Data Records sent for
		// the Transport Session prior to the receipt of this IPFIX Message,
		// modulo 2^32."
		sender.AdvanceSequence(dataRecords)
		total += int(dataRecords)
	}
	return total, sendErr
}

// maxCounterRecordsPerDatagram is how many counter records fit one datagram
// of maxDatagram octets after the message header and Data Set header. At
// least one.
func maxCounterRecordsPerDatagram(maxDatagram int) int {
	recSize := CounterRecordSize()
	if recSize <= 0 {
		return 1
	}
	n := (maxDatagram - MessageHeaderSize - 4) / recSize
	if n < 1 {
		return 1
	}
	return n
}

// EncodeTemplate sends the IPFIX template Set.
func (e *CounterEncoder) EncodeTemplate(sender *flowexport.Sender) error {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)

	// RFC 7011 Section 8.2: "an Exporting Process MUST sequence all Template
	// management actions (i.e., Template Records defining new Templates and
	// Template Withdrawals withdrawing them) using the Export Time field in
	// the IPFIX Message Header."
	exportTime := max(uint32(time.Now().Unix()), e.templateExportTime)

	n, _ := WriteMessage(
		*buf, exportTime, sender.Sequence(), e.ObservationDomainID,
		e.templateBytes, true,
		nil, 0, 0,
	)

	if err := sender.Send((*buf)[:n]); err != nil {
		return err
	}
	e.templateExportTime = exportTime
	return nil
}
