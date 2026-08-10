// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- NetFlow v9 export packet encoder

package netflow9

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// HeaderSize is the fixed size of a NetFlow v9 export packet header.
// RFC 3954 Section 5.1: 20 octets.
const HeaderSize = 20

// writePacketHeader writes a NetFlow v9 export packet header at off.
// RFC 3954 Section 5.1: version number MUST be 9.
func writePacketHeader(buf []byte, off int, count uint16, sysUpTime, unixSecs, seqNum, sourceID uint32) int {
	binary.BigEndian.PutUint16(buf[off:], 9)
	binary.BigEndian.PutUint16(buf[off+2:], count)
	binary.BigEndian.PutUint32(buf[off+4:], sysUpTime)
	binary.BigEndian.PutUint32(buf[off+8:], unixSecs)
	binary.BigEndian.PutUint32(buf[off+12:], seqNum)
	binary.BigEndian.PutUint32(buf[off+16:], sourceID)
	return HeaderSize
}

// writeExportPacket writes a complete NetFlow v9 export packet into buf
// starting at offset 0. It writes the header, optionally the template
// FlowSet, then the data FlowSet for the given interfaces.
// Returns the total bytes written.
//
// templateBytes must be pre-built via BuildCounterTemplate. When
// needTemplate is true, the template FlowSet is included before the
// data FlowSet.
// RFC 3954 Section 5.1: count = total records across all FlowSets.
func writeExportPacket(buf []byte, sysUpTime, unixSecs, seqNum, sourceID uint32, templateBytes []byte, needTemplate bool, ifaces []flowexport.InterfaceCounters) int {
	off := HeaderSize

	var totalRecords uint16

	if needTemplate && len(templateBytes) > 0 {
		n := copy(buf[off:], templateBytes)
		off += n
		totalRecords++
	}

	if len(ifaces) > 0 {
		n, dataRecords := writeDataFlowSet(buf, off, CounterTemplateID, ifaces)
		off += n
		totalRecords += dataRecords
	}

	writePacketHeader(buf, 0, totalRecords, sysUpTime, unixSecs, seqNum, sourceID)

	return off
}
