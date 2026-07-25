package ipfix

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

func TestIPFIXHeader(t *testing.T) {
	var buf [64]byte
	n := WriteMessageHeader(buf[:], 0, 42, 1716000000, 7, 99)
	if n != MessageHeaderSize {
		t.Fatalf("header size = %d, want %d", n, MessageHeaderSize)
	}
	if n != 16 {
		t.Fatalf("MessageHeaderSize = %d, want 16", n)
	}

	// RFC 7011 Section 3.1: version MUST be 0x000a.
	ver := binary.BigEndian.Uint16(buf[0:])
	if ver != 0x000a {
		t.Errorf("version = 0x%04x, want 0x000a", ver)
	}

	length := binary.BigEndian.Uint16(buf[2:])
	if length != 42 {
		t.Errorf("length = %d, want 42", length)
	}

	exportTime := binary.BigEndian.Uint32(buf[4:])
	if exportTime != 1716000000 {
		t.Errorf("export time = %d, want 1716000000", exportTime)
	}

	seqNum := binary.BigEndian.Uint32(buf[8:])
	if seqNum != 7 {
		t.Errorf("sequence number = %d, want 7", seqNum)
	}

	obsDomainID := binary.BigEndian.Uint32(buf[12:])
	if obsDomainID != 99 {
		t.Errorf("observation domain ID = %d, want 99", obsDomainID)
	}
}

func TestIPFIXSequenceCounting(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{
		{IfIndex: 1, IfInOctets: 100, IfOutOctets: 200},
		{IfIndex: 2, IfInOctets: 300, IfOutOctets: 400},
		{IfIndex: 3, IfInOctets: 500, IfOutOctets: 600},
	}
	tmpl := BuildCounterTemplate()

	var buf [1400]byte
	_, dataRecords := WriteMessage(buf[:], 1716000000, 0, 1, tmpl, true, ifaces, 1000, 1020)

	// RFC 7011 Section 3.1: sequence counts data records only.
	// Template records do NOT increment the sequence.
	if dataRecords != 3 {
		t.Errorf("data records = %d, want 3 (one per interface)", dataRecords)
	}

	// Second message: sequence should be 3 (from first batch).
	_, dataRecords2 := WriteMessage(buf[:], 1716000020, 3, 1, nil, false, ifaces, 1020, 1040)
	if dataRecords2 != 3 {
		t.Errorf("second batch data records = %d, want 3", dataRecords2)
	}

	// Verify the sequence field in the second message header.
	seq := binary.BigEndian.Uint32(buf[8:])
	if seq != 3 {
		t.Errorf("second message sequence = %d, want 3", seq)
	}
}

func TestIPFIXWriteMessageLength(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{
		{IfIndex: 1, IfInOctets: 100, IfOutOctets: 200},
	}
	tmpl := BuildCounterTemplate()

	var buf [1400]byte
	totalBytes, _ := WriteMessage(buf[:], 1716000000, 0, 1, tmpl, true, ifaces, 1000, 1020)

	// Verify length in header matches actual bytes written.
	headerLength := binary.BigEndian.Uint16(buf[2:])
	if int(headerLength) != totalBytes {
		t.Errorf("header length = %d, actual written = %d", headerLength, totalBytes)
	}
}

func TestIPFIXWriteMessageNoTemplate(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{
		{IfIndex: 1, IfInOctets: 100, IfOutOctets: 200},
	}

	var buf [1400]byte
	withTmpl, _ := WriteMessage(buf[:], 1716000000, 0, 1, BuildCounterTemplate(), true, ifaces, 1000, 1020)

	var buf2 [1400]byte
	withoutTmpl, _ := WriteMessage(buf2[:], 1716000000, 0, 1, nil, false, ifaces, 1000, 1020)

	tmplSize := len(BuildCounterTemplate())
	if withTmpl-withoutTmpl != tmplSize {
		t.Errorf("template size difference = %d, want %d", withTmpl-withoutTmpl, tmplSize)
	}
}
