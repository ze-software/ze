// Design: rfc/short/rfc7011.md -- IPFIX message encoding

package ipfix

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// MessageHeaderSize is the fixed IPFIX message header size.
// RFC 7011 Section 3.1: version (2) + length (2) + export time (4) +
// sequence number (4) + observation domain ID (4) = 16 octets.
const MessageHeaderSize = 16

// Version is the IPFIX protocol version number.
// RFC 7011 Section 3.1: version MUST be 0x000a.
const Version = 0x000a

// WriteMessageHeader writes a 16-byte IPFIX message header at off.
// Returns the number of bytes written (always 16).
func WriteMessageHeader(buf []byte, off int, length uint16, exportTime, seqNum, observationDomainID uint32) int {
	binary.BigEndian.PutUint16(buf[off:], Version)
	binary.BigEndian.PutUint16(buf[off+2:], length)
	binary.BigEndian.PutUint32(buf[off+4:], exportTime)
	binary.BigEndian.PutUint32(buf[off+8:], seqNum)
	binary.BigEndian.PutUint32(buf[off+12:], observationDomainID)
	return MessageHeaderSize
}

// WriteMessage encodes a complete IPFIX message containing an optional
// Template Set and a Data Set with interface counter records.
// Uses skip-and-backfill for the message header length field.
// Returns (total bytes written, data record count). The caller adds
// the returned data record count to the sequence number for the next
// message.
// RFC 7011 Section 3.1: sequence number is the cumulative number of
// IPFIX Data Records sent by this exporting process.
func WriteMessage(buf []byte, exportTime, seqNum, observationDomainID uint32, templateBytes []byte, needTemplate bool, ifaces []flowexport.InterfaceCounters, startTime, endTime uint32) (int, uint32) {
	off := 0

	// Write header, skip length for backfill.
	lengthPos := off + 2
	binary.BigEndian.PutUint16(buf[off:], Version)
	off += 2
	off += 2 // skip length
	binary.BigEndian.PutUint32(buf[off:], exportTime)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], seqNum)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], observationDomainID)
	off += 4

	// RFC 7011 Section 3.4.1: template must be sent before data that
	// references it, or in the same message preceding the data Set.
	if needTemplate && len(templateBytes) > 0 {
		copy(buf[off:], templateBytes)
		off += len(templateBytes)
	}

	var dataRecords uint32
	if len(ifaces) > 0 {
		n, count := WriteDataSet(buf, off, CounterTemplateID, ifaces, startTime, endTime)
		off += n
		dataRecords = count
	}

	// Backfill message length.
	// RFC 7011 Section 3.1: Length = total message length including header.
	binary.BigEndian.PutUint16(buf[lengthPos:], uint16(off))

	return off, dataRecords
}
