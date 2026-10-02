package mrt_test

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/mrt"
)

// rfc8050Update is one BGP UPDATE whose NLRI carries an RFC 7911 Path
// Identifier: marker, length 45, type 2, no withdrawn routes, 14 octets of
// attributes (ORIGIN IGP, empty AS_PATH, NEXT_HOP 192.0.2.1), then Path
// Identifier 1 and 10.0.0.0/24.
var rfc8050Update = []byte{
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0x00, 0x2d, 0x02,
	0x00, 0x00,
	0x00, 0x0e,
	0x40, 0x01, 0x01, 0x00,
	0x40, 0x02, 0x00,
	0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
	0x00, 0x00, 0x00, 0x01, 0x18, 0x0a, 0x00, 0x00,
}

// rfc8050BGP4MPRecord builds one whole MRT record: the common header with
// type BGP4MP and the given subtype, then the given BGP4MP fields, then
// rfc8050Update. The fields are literal octets, so the record does not depend
// on Ze's encoder.
func rfc8050BGP4MPRecord(subtype uint16, fields []byte) []byte {
	length := len(fields) + len(rfc8050Update)
	record := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x10,
		byte(subtype >> 8), byte(subtype),
		byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length),
	}
	record = append(record, fields...)
	return append(record, rfc8050Update...)
}

// rfc8050AS4Fields are the BGP4MP_MESSAGE_AS4 fields of RFC 6396 Section
// 4.4.3: Peer AS 65001, Local AS 65000, Interface Index 0, AFI 1, Peer IP
// 192.0.2.1, Local IP 192.0.2.2.
var rfc8050AS4Fields = []byte{
	0x00, 0x00, 0xfd, 0xe9,
	0x00, 0x00, 0xfd, 0xe8,
	0x00, 0x00,
	0x00, 0x01,
	0xc0, 0x00, 0x02, 0x01,
	0xc0, 0x00, 0x02, 0x02,
}

// rfc8050AS2Fields are the BGP4MP_MESSAGE fields of RFC 6396 Section 4.4.2,
// with the same values and two-octet AS numbers.
var rfc8050AS2Fields = []byte{
	0xfd, 0xe9,
	0xfd, 0xe8,
	0x00, 0x00,
	0x00, 0x01,
	0xc0, 0x00, 0x02, 0x01,
	0xc0, 0x00, 0x02, 0x02,
}

// readRFC8050Message reads one MRT record through ReadFrom, which dispatches
// on the subtype, and returns the one BGP4MP message it delivered.
func readRFC8050Message(t *testing.T, record []byte) (mrt.Header, *mrt.MessageRecord) {
	t.Helper()
	var header mrt.Header
	var messages []*mrt.MessageRecord
	handler := &mrt.Handler{
		OnMessage: func(h mrt.Header, _ uint32, m *mrt.MessageRecord) error {
			header = h
			messages = append(messages, m)
			return nil
		},
	}
	if err := mrt.ReadFrom(bytes.NewReader(record), handler); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("BGP4MP messages delivered = %d, want 1", len(messages))
	}
	return header, messages[0]
}

// TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields proves that the four
// add-path BGP4MP subtypes read exactly the fields of their base subtype and
// hand over the whole BGP message.
//
// Method: each add-path subtype (8 to 11) is fed a literal record whose fields
// are laid out as RFC 6396 Section 4.4 lays out its base subtype (1, 4, 6, 7),
// with an UPDATE whose NLRI carries a Path Identifier. The record goes through
// ReadFrom, so the subtype dispatch and the shared header decoder both run.
// Every field must hold the literal value, the BGP message field must equal the
// 45 octets of the UPDATE from its marker on, and the base subtype read over
// the same octets must give the same answer.
//
// RFC requirement: RFC8050-x-3 positive -- BGP4MP_MESSAGE_ADDPATH (8),
// BGP4MP_MESSAGE_AS4_ADDPATH (9), BGP4MP_MESSAGE_LOCAL_ADDPATH (10) and
// BGP4MP_MESSAGE_AS4_LOCAL_ADDPATH (11) each read Peer AS 65001, Local AS
// 65000, Interface Index 0, AFI 1, Peer IP 192.0.2.1 and Local IP 192.0.2.2
// from the base-subtype layout, equal to the base subtype's reading of the
// same octets, and the BGP message field holds the entire 45-octet UPDATE.
func TestRFC8050AddPathBGP4MPRecordsKeepTheBaseFields(t *testing.T) {
	cases := []struct {
		name    string
		addPath uint16
		base    uint16
		fields  []byte
	}{
		{"BGP4MP_MESSAGE_ADDPATH", mrt.BGP4MPMessageAP, mrt.BGP4MPMessage, rfc8050AS2Fields},
		{"BGP4MP_MESSAGE_AS4_ADDPATH", mrt.BGP4MPMessageAS4AP, mrt.BGP4MPMessageAS4, rfc8050AS4Fields},
		{"BGP4MP_MESSAGE_LOCAL_ADDPATH", mrt.BGP4MPMessageLocalAP, mrt.BGP4MPMessageLocal, rfc8050AS2Fields},
		{"BGP4MP_MESSAGE_AS4_LOCAL_ADDPATH", mrt.BGP4MPMessageAS4LocalAP, mrt.BGP4MPMessageAS4Local, rfc8050AS4Fields},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header, got := readRFC8050Message(t, rfc8050BGP4MPRecord(tc.addPath, tc.fields))
			if header.Subtype != tc.addPath {
				t.Fatalf("subtype = %d, want %d", header.Subtype, tc.addPath)
			}
			if got.PeerAS != 65001 {
				t.Errorf("Peer AS = %d, want 65001", got.PeerAS)
			}
			if got.LocalAS != 65000 {
				t.Errorf("Local AS = %d, want 65000", got.LocalAS)
			}
			if got.IfIndex != 0 {
				t.Errorf("Interface Index = %d, want 0", got.IfIndex)
			}
			if got.AFI != mrt.AFIIPv4 {
				t.Errorf("AFI = %d, want 1", got.AFI)
			}
			if !bytes.Equal(got.PeerIP, []byte{0xc0, 0x00, 0x02, 0x01}) {
				t.Errorf("Peer IP = % x, want c0 00 02 01", got.PeerIP)
			}
			if !bytes.Equal(got.LocalIP, []byte{0xc0, 0x00, 0x02, 0x02}) {
				t.Errorf("Local IP = % x, want c0 00 02 02", got.LocalIP)
			}
			if !bytes.Equal(got.BGPMessage, rfc8050Update) {
				t.Errorf("BGP message = % x, want the entire UPDATE % x", got.BGPMessage, rfc8050Update)
			}

			_, base := readRFC8050Message(t, rfc8050BGP4MPRecord(tc.base, tc.fields))
			if base.PeerAS != got.PeerAS || base.LocalAS != got.LocalAS || base.AFI != got.AFI {
				t.Errorf("base subtype %d read AS %d/%d AFI %d, add-path read AS %d/%d AFI %d",
					tc.base, base.PeerAS, base.LocalAS, base.AFI, got.PeerAS, got.LocalAS, got.AFI)
			}
			if !bytes.Equal(base.BGPMessage, got.BGPMessage) {
				t.Errorf("base subtype %d message % x differs from add-path % x", tc.base, base.BGPMessage, got.BGPMessage)
			}
		})
	}
}

// TestRFC8050AddPathRIBEntryFollowsFigure1 proves that an AFI/SAFI-specific
// add-path RIB entry carries a 32-bit, network-order Path Identifier between
// Originated Time and Attribute Length, on the write side and the read side.
//
// Method: the writer's output for one entry is compared with the literal
// Figure 1 octets, and the same literal octets are read back under each of
// the four AFI/SAFI-specific add-path subtypes. Path Identifier 0x01020304 is
// above 0xFFFF and has four distinct octets, so a 16-bit field or a reversed
// byte order changes the octets or the value read.
//
// RFC requirement: RFC8050-4.1-1 positive -- WriteRIBEntries with add-path
// writes Peer Index 00 03, Originated Time 5f 5e 10 00, Path Identifier
// 01 02 03 04, Attribute Length 00 04, then the attributes; DecodeRIBRecord
// reads Path Identifier 0x01020304 and the 4 attribute octets from those
// octets under RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH,
// RIB_IPV6_UNICAST_ADDPATH and RIB_IPV6_MULTICAST_ADDPATH.
// RFC requirement: RFC8050-x-2 positive -- the Path Identifier occupies the 4
// octets 01 02 03 04 in network byte order, and reads back as 0x01020304.
func TestRFC8050AddPathRIBEntryFollowsFigure1(t *testing.T) {
	attributes := []byte{0x40, 0x01, 0x01, 0x00}
	entryWire := []byte{
		0x00, 0x01,
		0x00, 0x03,
		0x5f, 0x5e, 0x10, 0x00,
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x04,
		0x40, 0x01, 0x01, 0x00,
	}

	buf := make([]byte, 64)
	entries := []mrt.RIBEntry{{PeerIndex: 3, OrigTime: 0x5f5e1000, PathID: 0x01020304, Attributes: attributes}}
	n := mrt.WriteRIBEntries(buf, 0, entries, true)
	if !bytes.Equal(buf[:n], entryWire) {
		t.Fatalf("written entry = % x, want Figure 1 octets % x", buf[:n], entryWire)
	}

	cases := []struct {
		name    string
		subtype uint16
		prefix  []byte
	}{
		{"RIB_IPV4_UNICAST_ADDPATH", mrt.TDV2RIBIPv4UnicastAP, []byte{0x18, 0x0a, 0x00, 0x00}},
		{"RIB_IPV4_MULTICAST_ADDPATH", mrt.TDV2RIBIPv4MulticastAP, []byte{0x18, 0xe8, 0x00, 0x00}},
		{"RIB_IPV6_UNICAST_ADDPATH", mrt.TDV2RIBIPv6UnicastAP, []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}},
		{"RIB_IPV6_MULTICAST_ADDPATH", mrt.TDV2RIBIPv6MulticastAP, []byte{0x08, 0xff}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := []byte{0x00, 0x00, 0x00, 0x07}
			record = append(record, tc.prefix...)
			record = append(record, entryWire...)

			rib, err := mrt.DecodeRIBRecord(tc.subtype, record)
			if err != nil {
				t.Fatalf("DecodeRIBRecord: %v", err)
			}
			if len(rib.Entries) != 1 {
				t.Fatalf("entries = %d, want 1", len(rib.Entries))
			}
			got := rib.Entries[0]
			if got.PeerIndex != 3 {
				t.Errorf("Peer Index = %d, want 3", got.PeerIndex)
			}
			if got.OrigTime != 0x5f5e1000 {
				t.Errorf("Originated Time = %#x, want 0x5f5e1000", got.OrigTime)
			}
			if got.PathID != 0x01020304 {
				t.Errorf("Path Identifier = %#x, want 0x01020304", got.PathID)
			}
			if !bytes.Equal(got.Attributes, attributes) {
				t.Errorf("attributes = % x, want % x", got.Attributes, attributes)
			}
		})
	}
}
