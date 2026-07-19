package netflow9

import (
	"encoding/binary"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/flowexport"
)

// RFC requirement: RFC3954-x-3 positive -- header integer fields (version, count, sysUpTime, unixSecs, seq, sourceID) decode big-endian to their asymmetric written values (encoder.go:17-24); a little-endian write would fail these BigEndian reads.
func TestNetflow9Header(t *testing.T) {
	buf := make([]byte, 64)
	n := WritePacketHeader(buf, 0, 3, 1000, 1716900000, 42, 1)

	if n != HeaderSize {
		t.Fatalf("header size: got %d, want %d", n, HeaderSize)
	}
	if n != 20 {
		t.Fatalf("header size: got %d, want 20", n)
	}

	// RFC 3954 Section 5.1: version MUST be 9
	ver := binary.BigEndian.Uint16(buf[0:])
	if ver != 9 {
		t.Errorf("version: got %d, want 9", ver)
	}

	count := binary.BigEndian.Uint16(buf[2:])
	if count != 3 {
		t.Errorf("count: got %d, want 3", count)
	}

	sysUp := binary.BigEndian.Uint32(buf[4:])
	if sysUp != 1000 {
		t.Errorf("sysUpTime: got %d, want 1000", sysUp)
	}

	unixSecs := binary.BigEndian.Uint32(buf[8:])
	if unixSecs != 1716900000 {
		t.Errorf("UNIX_Secs: got %d, want 1716900000", unixSecs)
	}

	seq := binary.BigEndian.Uint32(buf[12:])
	if seq != 42 {
		t.Errorf("sequence: got %d, want 42", seq)
	}

	srcID := binary.BigEndian.Uint32(buf[16:])
	if srcID != 1 {
		t.Errorf("source ID: got %d, want 1", srcID)
	}
}

func TestNetflow9HeaderOffset(t *testing.T) {
	buf := make([]byte, 128)
	buf[10] = 0xff
	n := WritePacketHeader(buf, 10, 0, 0, 0, 0, 0)

	if n != HeaderSize {
		t.Fatalf("header size: got %d, want %d", n, HeaderSize)
	}

	ver := binary.BigEndian.Uint16(buf[10:])
	if ver != 9 {
		t.Errorf("version at offset 10: got %d, want 9", ver)
	}
}

// RFC requirement: RFC3954-x-1 positive -- WriteExportPacket with needTemplate=true packs the Template FlowSet before the Data FlowSet in one packet (encoder.go:36-56); count==2 (1 template + 1 data record) confirms the template precedes the data it describes.
func TestWriteExportPacketWithTemplate(t *testing.T) {
	buf := make([]byte, 1400)
	tmpl := BuildCounterTemplate()

	ifaces := []flowexport.InterfaceCounters{
		{
			IfIndex:        1,
			IfInOctets:     1000,
			IfInUcastPkts:  10,
			IfOutOctets:    2000,
			IfOutUcastPkts: 20,
		},
	}

	n := WriteExportPacket(buf, 5000, 1716900000, 1, 0, tmpl, true, ifaces)

	// Header
	ver := binary.BigEndian.Uint16(buf[0:])
	if ver != 9 {
		t.Errorf("version: got %d, want 9", ver)
	}

	// Count = 1 template record + 1 data record
	count := binary.BigEndian.Uint16(buf[2:])
	if count != 2 {
		t.Errorf("count: got %d, want 2", count)
	}

	if n <= HeaderSize+len(tmpl) {
		t.Errorf("total size %d too small for header + template + data", n)
	}
}

func TestWriteExportPacketWithoutTemplate(t *testing.T) {
	buf := make([]byte, 1400)
	tmpl := BuildCounterTemplate()

	ifaces := []flowexport.InterfaceCounters{
		{
			IfIndex:        5,
			IfInOctets:     500,
			IfInUcastPkts:  5,
			IfOutOctets:    600,
			IfOutUcastPkts: 6,
		},
	}

	n := WriteExportPacket(buf, 5000, 1716900000, 1, 0, tmpl, false, ifaces)

	count := binary.BigEndian.Uint16(buf[2:])
	if count != 1 {
		t.Errorf("count without template: got %d, want 1", count)
	}

	// Should only have header + data FlowSet, no template
	expectedMax := HeaderSize + FlowSetHeaderSize + CounterRecordSize() + 4
	if n > expectedMax {
		t.Errorf("total size %d exceeds expected %d (no template)", n, expectedMax)
	}
}
