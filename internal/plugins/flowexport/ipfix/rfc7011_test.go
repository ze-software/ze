// RFC 7011 conformance tests for the IPFIX exporter encoding path.
// Each test is bound to a Compliance Checklist requirement in
// rfc/short/rfc7011.md via an `RFC requirement:` tag scanned by
// scripts/dev/rfc_requirements.py.
//
// VALIDATES: the IPFIX exporter's on-wire encoding meets the RFC 7011 MUST-level
// message, Set, padding, Template ID, field-specifier, and reduced-size rules.
// PREVENTS: regressions such as a wrong version, a header-only zero-Set message,
// non-zero Set padding, a sub-256 Template ID, an accidental Enterprise Number,
// or reduced-size encoding of an address or timestamp IE.

package ipfix

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// countSets walks the Sets following the 16-octet message header and returns how
// many Set headers it finds, validating that every Set Length stays within the
// message bound. RFC 7011 Section 3: a Message is the header plus one or more Sets.
func countSets(t *testing.T, msg []byte) int {
	t.Helper()
	if len(msg) < MessageHeaderSize {
		t.Fatalf("message shorter than header: %d octets", len(msg))
	}
	total := int(binary.BigEndian.Uint16(msg[2:]))
	if total != len(msg) {
		t.Fatalf("header length %d != actual %d", total, len(msg))
	}
	off := MessageHeaderSize
	sets := 0
	for off+4 <= total {
		setLen := int(binary.BigEndian.Uint16(msg[off+2:]))
		if setLen < 4 || off+setLen > total {
			t.Fatalf("Set at offset %d has bad length %d (message total %d)", off, setLen, total)
		}
		sets++
		off += setLen
	}
	if off != total {
		t.Fatalf("%d trailing octets after the last Set", total-off)
	}
	return sets
}

// TestRFC7011MessageHasAtLeastOneSet verifies an emitted IPFIX Message carries at
// least one Set after the header.
func TestRFC7011MessageHasAtLeastOneSet(t *testing.T) {
	// RFC requirement: RFC7011-3-1 positive -- a counter message the exporter builds carries a Template Set followed by a Data Set (encoder.go:56-66), so the walk after the 16-octet header finds >= 1 Set
	ifaces := []flowexport.InterfaceCounters{{IfIndex: 1, IfInOctets: 10, IfOutOctets: 20}}
	var buf [1400]byte
	n, _ := WriteMessage(buf[:], 1716000000, 0, 1, BuildCounterTemplate(), true, ifaces, 1000, 1020)
	if sets := countSets(t, buf[:n]); sets < 1 {
		t.Fatalf("emitted message has %d Sets, want >= 1", sets)
	}
}

// TestRFC7011NoEmptyMessageEmitted verifies the exporter never puts a Set-less
// (header-only) message on the wire: an empty snapshot yields no datagram.
func TestRFC7011NoEmptyMessageEmitted(t *testing.T) {
	// RFC requirement: RFC7011-3-1 negative -- Encode with zero interfaces returns early (adapter.go:32-34) and never calls Send, so no header-only, zero-Set message reaches the wire
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc.Close() }()
	addr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	s, err := flowexport.NewSender("127.0.0.1", addr.Port, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	enc := NewCounterEncoder(7)
	n, err := enc.Encode(flowexport.CounterSnapshot{Time: time.Unix(1716000000, 0)}, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("records exported = %d, want 0 for an empty snapshot", n)
	}
	datagrams, _, _ := s.Stats()
	if datagrams != 0 {
		t.Fatalf("datagrams sent = %d, want 0 (no zero-Set message on the wire)", datagrams)
	}
}

// TestRFC7011VersionIsIPFIX verifies the message header carries version 0x000a.
func TestRFC7011VersionIsIPFIX(t *testing.T) {
	// RFC requirement: RFC7011-3.1-1 positive -- WriteMessageHeader emits the compile-time constant Version (encoder.go:18,23), so the first two header octets are 0x000a
	var buf [16]byte
	WriteMessageHeader(buf[:], 0, 16, 1716000000, 0, 1)
	if v := binary.BigEndian.Uint16(buf[0:]); v != 0x000a {
		t.Fatalf("version = 0x%04x, want 0x000a", v)
	}
	if Version != 0x000a {
		t.Fatalf("Version constant = 0x%04x, want 0x000a", Version)
	}
}

// flowWithPadding builds a one-record IPv4 flow set. The 53-octet record plus the
// 4-octet Set header total 57, so the Set is padded up to 60: 3 padding octets.
func flowWithPadding(t *testing.T, dirty bool) (buf []byte, dataLen, n int) {
	t.Helper()
	flows := []FlowRecord{{
		SrcAddr: netip.MustParseAddr("192.0.2.1"),
		DstAddr: netip.MustParseAddr("192.0.2.2"),
	}}
	buf = make([]byte, 256)
	if dirty {
		for i := range buf {
			buf[i] = 0xFF
		}
	}
	written, _ := WriteFlowDataSet(buf, 0, flows, FlowTemplateID)
	dataLen = 4 + FlowRecordSize()
	if written <= dataLen {
		t.Fatalf("no padding was added: written=%d, dataLen=%d", written, dataLen)
	}
	return buf, dataLen, written
}

// TestRFC7011PaddingIsZero verifies Set padding octets are zero.
func TestRFC7011PaddingIsZero(t *testing.T) {
	// RFC requirement: RFC7011-3.3.1-1 positive -- the 3 octets of 4-byte-alignment padding after a 53-octet flow record are written as zero (flow_data.go:66-73)
	buf, dataLen, n := flowWithPadding(t, false)
	for i := dataLen; i < n; i++ {
		if buf[i] != 0 {
			t.Fatalf("padding octet %d = 0x%02x, want 0x00", i, buf[i])
		}
	}
}

// TestRFC7011PaddingZeroedOverGarbage verifies padding is actively zeroed rather
// than left as whatever the buffer held.
func TestRFC7011PaddingZeroedOverGarbage(t *testing.T) {
	// RFC requirement: RFC7011-3.3.1-1 negative -- with the buffer pre-filled 0xFF, the encoder overwrites the padding region with zero instead of leaving the non-zero octets in place (flow_data.go:69-71)
	buf, dataLen, n := flowWithPadding(t, true)
	for i := dataLen; i < n; i++ {
		if buf[i] != 0 {
			t.Fatalf("dirty padding octet %d = 0x%02x, want it zeroed", i, buf[i])
		}
	}
}

// TestRFC7011PaddingShorterThanRecord verifies the padding never reaches a full
// record in length.
func TestRFC7011PaddingShorterThanRecord(t *testing.T) {
	// RFC requirement: RFC7011-3.3.1-2 positive -- 4-byte-alignment padding is at most 3 octets, far below the 53-octet minimum record, so the padLen < recSize guard (flow_data.go:68) holds
	_, dataLen, n := flowWithPadding(t, false)
	pad := n - dataLen
	if pad <= 0 || pad >= FlowRecordSize() {
		t.Fatalf("padding = %d octets, must be > 0 and < the record size %d", pad, FlowRecordSize())
	}
	if pad > 3 {
		t.Fatalf("padding = %d octets, must be <= 3 for 4-byte alignment", pad)
	}
}

// TestRFC7011TemplateIDAbove255 verifies every emitted Template ID is >= 256.
func TestRFC7011TemplateIDAbove255(t *testing.T) {
	// RFC requirement: RFC7011-3.4.1-1 positive -- the counter (256) and per-flow (257/258) template builders each write a Template ID in the 256-65535 range (template.go:10, flow_template.go:11,16)
	for _, tc := range []struct {
		name string
		tmpl []byte
		want uint16
	}{
		{"counter", BuildCounterTemplate(), CounterTemplateID},
		{"flow4", BuildFlowTemplate(), FlowTemplateID},
		{"flow6", BuildFlowTemplate6(), FlowTemplateID6},
	} {
		// Template ID follows the 4-octet Set header.
		id := binary.BigEndian.Uint16(tc.tmpl[4:])
		if id != tc.want {
			t.Errorf("%s: Template ID = %d, want %d", tc.name, id, tc.want)
		}
		if id < 256 {
			t.Errorf("%s: Template ID = %d, MUST be >= 256", tc.name, id)
		}
	}
}

// TestRFC7011NoEnterpriseNumberWhenEClear verifies E=0 field specifiers carry no
// Enterprise Number.
func TestRFC7011NoEnterpriseNumberWhenEClear(t *testing.T) {
	// RFC requirement: RFC7011-3.2-1 positive -- every field specifier is the 4-octet E=0 form with bit 15 clear (template.go:69-74, flow_template.go:115-119), so no 4-octet Enterprise Number is ever appended
	for _, tc := range []struct {
		name       string
		tmpl       []byte
		fieldCount int
	}{
		{"counter", BuildCounterTemplate(), 6},
		{"flow4", BuildFlowTemplate(), FlowFieldCount()},
		{"flow6", BuildFlowTemplate6(), FlowFieldCount6()},
	} {
		// Set header (4) + template record header (4) + N four-octet specifiers.
		// Any 8-octet specifier would mean an Enterprise Number was appended.
		wantLen := 4 + 4 + tc.fieldCount*4
		if len(tc.tmpl) != wantLen {
			t.Errorf("%s: template length = %d, want %d (an 8-octet specifier would carry an Enterprise Number)", tc.name, len(tc.tmpl), wantLen)
		}
		base := 8
		for i := range tc.fieldCount {
			ie := binary.BigEndian.Uint16(tc.tmpl[base+i*4:])
			if ie&0x8000 != 0 {
				t.Errorf("%s: field %d has the E bit set (0x%04x); the exporter emits only E=0 IANA IEs", tc.name, i, ie)
			}
		}
	}
}

// TestRFC7011NoReducedSizeForAddressOrTimestamp verifies address and timestamp IEs
// keep their full octet width.
func TestRFC7011NoReducedSizeForAddressOrTimestamp(t *testing.T) {
	// RFC requirement: RFC7011-6.2-1 positive -- the templates declare full-width field lengths for every address (IPv4=4, IPv6=16) and timestamp (dateTimeSeconds=4, dateTimeMilliseconds=8) IE (template.go:20-27, flow_template.go:20-48), so reduced-size encoding is never applied to a prohibited type
	// full[IE] is the type's full octet width; a shorter Field Length would be reduced-size.
	full := map[uint16]uint16{
		IESourceIPv4Address:      4,
		IEDestinationIPv4Address: 4,
		IESourceIPv6Address:      16,
		IEDestinationIPv6Address: 16,
		IEFlowStartSeconds:       4,
		IEFlowEndSeconds:         4,
		IEFlowStartMilliseconds:  8,
		IEFlowEndMilliseconds:    8,
	}
	check := func(name string, tmpl []byte, fieldCount int) {
		base := 8
		for i := range fieldCount {
			ie := binary.BigEndian.Uint16(tmpl[base+i*4:])
			length := binary.BigEndian.Uint16(tmpl[base+i*4+2:])
			if want, ok := full[ie]; ok && length != want {
				t.Errorf("%s: IE %d encoded at %d octets, want full width %d (reduced-size is forbidden for addresses/timestamps)", name, ie, length, want)
			}
		}
	}
	check("counter", BuildCounterTemplate(), 6)
	check("flow4", BuildFlowTemplate(), FlowFieldCount())
	check("flow6", BuildFlowTemplate6(), FlowFieldCount6())
}
