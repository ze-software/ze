// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- NetFlow v9 protocol encoder adapter

package netflow9

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// CounterEncoder implements flowexport.ProtocolEncoder for NetFlow v9 counter records.
type CounterEncoder struct {
	SourceID  uint32
	StartTime time.Time

	seqNum        uint32
	templateBytes []byte
}

// NewCounterEncoder creates a NetFlow v9 counter encoder.
func NewCounterEncoder(sourceID uint32, startTime time.Time) *CounterEncoder {
	return &CounterEncoder{
		SourceID:      sourceID,
		StartTime:     startTime,
		templateBytes: BuildCounterTemplate(),
	}
}

// Encode writes NetFlow v9 export packet(s) with counter data and sends them.
// Interface records are chunked so each datagram stays within MaxDatagramSize;
// a device with more interfaces than fit one datagram produces several, with
// the export-packet sequence number advancing per datagram.
func (e *CounterEncoder) Encode(snap flowexport.CounterSnapshot, sender *flowexport.Sender) (int, error) {
	if len(snap.Interfaces) == 0 {
		return 0, nil
	}

	sysUpTime := uint32(snap.Time.Sub(e.StartTime).Milliseconds())
	unixSecs := uint32(snap.Time.Unix())
	maxPer := maxCounterRecordsPerDatagram()

	total := 0
	for start := 0; start < len(snap.Interfaces); start += maxPer {
		end := min(start+maxPer, len(snap.Interfaces))
		chunk := snap.Interfaces[start:end]

		buf := flowexport.GetBuf()
		n := writeExportPacket(
			*buf, sysUpTime, unixSecs,
			e.seqNum, e.SourceID,
			nil, false,
			chunk,
		)
		err := sender.Send((*buf)[:n])
		flowexport.PutBuf(buf)
		if err != nil {
			return total, err
		}
		// RFC 3954: the sequence number counts export packets actually sent;
		// advance only after a successful send so a send failure does not open
		// a phantom gap at the collector.
		e.seqNum++
		total += len(chunk)
	}
	return total, nil
}

// maxCounterRecordsPerDatagram is how many counter records fit one datagram
// after the export-packet header and Data FlowSet header. At least one.
func maxCounterRecordsPerDatagram() int {
	recSize := CounterRecordSize()
	if recSize <= 0 {
		return 1
	}
	n := (flowexport.MaxDatagramSize - HeaderSize - FlowSetHeaderSize) / recSize
	if n < 1 {
		return 1
	}
	return n
}

// EncodeTemplate sends the template FlowSet.
func (e *CounterEncoder) EncodeTemplate(sender *flowexport.Sender) error {
	buf := flowexport.GetBuf()
	defer flowexport.PutBuf(buf)

	sysUpTime := uint32(time.Since(e.StartTime).Milliseconds())
	unixSecs := uint32(time.Now().Unix())

	n := writeExportPacket(
		*buf, sysUpTime, unixSecs,
		e.seqNum, e.SourceID,
		e.templateBytes, true,
		nil,
	)
	if err := sender.Send((*buf)[:n]); err != nil {
		return err
	}
	e.seqNum++
	return nil
}
