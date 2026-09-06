// Design: docs/architecture/diagnostics/packet-capture.md -- reading a pcap file back
// Overview: pcap.go -- the file format this reads

package pcap

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

// ErrBadMagic is returned when the first four bytes of a file are none of the
// four pcap magics. It names the file as not a classic pcap, which is what
// separates a wrong file from a corrupt one.
var ErrBadMagic = errors.New("pcap: not a pcap file")

// Record is one packet read from a pcap file.
//
// Data is valid only until the next call to Next, which reuses the reader's
// buffer. A caller that keeps the bytes MUST copy them.
type Record struct {
	Timestamp time.Time
	Data      []byte
	// OriginalLen is the length the packet had on the network. It is longer
	// than len(Data) when the capture truncated the packet.
	OriginalLen int
}

// Reader iterates the records of a classic pcap file. It reads either byte
// order and both timestamp resolutions, because a capture handed to Ze was
// written by somebody else's tcpdump.
//
// Not safe for concurrent use.
type Reader struct {
	source    io.Reader
	byteOrder binary.ByteOrder
	// nanoseconds says whether the second timestamp field counts nanoseconds
	// rather than microseconds, which the file magic decides.
	nanoseconds bool
	linkType    uint32
	snapLen     uint32
	buf         []byte
}

// NewReader reads the file header from source and returns a Reader positioned
// at the first record.
func NewReader(source io.Reader) (*Reader, error) {
	var hdr [FileHeaderLen]byte
	if _, err := io.ReadFull(source, hdr[:]); err != nil {
		return nil, fmt.Errorf("pcap: read file header: %w", err)
	}

	r := &Reader{source: source}
	switch {
	case binary.BigEndian.Uint32(hdr[0:4]) == magicMicro:
		r.byteOrder = binary.BigEndian
	case binary.LittleEndian.Uint32(hdr[0:4]) == magicMicro:
		r.byteOrder = binary.LittleEndian
	case binary.BigEndian.Uint32(hdr[0:4]) == magicNano:
		r.byteOrder, r.nanoseconds = binary.BigEndian, true
	case binary.LittleEndian.Uint32(hdr[0:4]) == magicNano:
		r.byteOrder, r.nanoseconds = binary.LittleEndian, true
	default:
		return nil, ErrBadMagic
	}

	r.snapLen = r.byteOrder.Uint32(hdr[16:20])
	r.linkType = r.byteOrder.Uint32(hdr[20:24])
	return r, nil
}

// LinkType returns the link type the file header declares. It decides how the
// first byte of every record is read.
func (r *Reader) LinkType() uint32 { return r.linkType }

// SnapLen returns the snapshot length the file header declares. It is a hint
// and never a bound: a record's own length field is what Next checks.
func (r *Reader) SnapLen() uint32 { return r.snapLen }

// Next reads the next record into rec and returns io.EOF at the end of the
// file. rec.Data points into the reader's buffer and is valid until the call
// after this one.
func (r *Reader) Next(rec *Record) error {
	var hdr [RecordHeaderLen]byte
	if _, err := io.ReadFull(r.source, hdr[:]); err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF
		}
		return fmt.Errorf("pcap: read record header: %w", err)
	}

	seconds := r.byteOrder.Uint32(hdr[0:4])
	fraction := r.byteOrder.Uint32(hdr[4:8])
	captured := r.byteOrder.Uint32(hdr[8:12])
	original := r.byteOrder.Uint32(hdr[12:16])

	// The captured length is attacker-controlled, so it is checked before it
	// sizes a buffer or a read.
	if captured > RecordBytesMax {
		return fmt.Errorf("pcap: record declares %d bytes, above the %d-byte maximum", captured, RecordBytesMax)
	}
	if original < captured {
		return fmt.Errorf("pcap: record declares %d original bytes, below its %d captured bytes", original, captured)
	}

	if uint32(len(r.buf)) < captured {
		r.buf = make([]byte, captured)
	}
	data := r.buf[:captured]
	if _, err := io.ReadFull(r.source, data); err != nil {
		return fmt.Errorf("pcap: read record of %d bytes: %w", captured, err)
	}

	nanoseconds := int64(fraction) * 1000
	if r.nanoseconds {
		nanoseconds = int64(fraction)
	}
	rec.Timestamp = time.Unix(int64(seconds), nanoseconds).UTC()
	rec.Data = data
	rec.OriginalLen = int(original)
	return nil
}
