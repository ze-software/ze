package bmp

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
)

// Design: the per-type message layouts of RFC 7854 Sections 4.3, 4.5, 4.7,
// 4.8 and 4.10, proved on the octets a collector receives (positive) and on
// the receiver refusing a message that breaks the layout (negative).
// Related: rfc7854_wire_header_test.go (common and per-peer header).

// rawResult holds one BMP message read whole off a pipe, undecoded.
type rawResult struct {
	msg []byte
	err error
}

// asyncReadRaw reads one complete BMP message from the pipe in a goroutine and
// hands back its octets, so a test can read the layout the collector receives.
func asyncReadRaw(conn net.Conn) <-chan rawResult {
	ch := make(chan rawResult, 1)
	go func() {
		header := make([]byte, CommonHeaderSize)
		if _, err := io.ReadFull(conn, header); err != nil {
			ch <- rawResult{nil, err}
			return
		}
		msg := make([]byte, binary.BigEndian.Uint32(header[1:5]))
		copy(msg, header)
		_, err := io.ReadFull(conn, msg[CommonHeaderSize:])
		ch <- rawResult{msg, err}
	}()
	return ch
}

// walkTLVs reads the TLVs in msg[off:] by hand: 2-octet type, 2-octet length,
// then the value. It fails the test when the octets do not end exactly on a
// TLV boundary, so the caller learns the region is a whole set of TLVs.
func walkTLVs(t *testing.T, msg []byte, off int) []TLV {
	t.Helper()
	var tlvs []TLV
	for off < len(msg) {
		if len(msg)-off < TLVHeaderSize {
			t.Fatalf("%d octets left at %d, shorter than a TLV header", len(msg)-off, off)
		}
		length := int(binary.BigEndian.Uint16(msg[off+2 : off+4]))
		if off+TLVHeaderSize+length > len(msg) {
			t.Fatalf("TLV at %d says %d value octets, only %d remain", off, length, len(msg)-off-TLVHeaderSize)
		}
		tlvs = append(tlvs, TLV{
			Type:   binary.BigEndian.Uint16(msg[off : off+2]),
			Length: uint16(length),
			Value:  msg[off+TLVHeaderSize : off+TLVHeaderSize+length],
		})
		off += TLVHeaderSize + length
	}
	return tlvs
}

// setMessageLength rewrites the common header's Message Length after a test
// cut or grew a message, so the receiver judges the body and not the header.
func setMessageLength(msg []byte) {
	binary.BigEndian.PutUint32(msg[1:5], uint32(len(msg)))
}

// RFC requirement: RFC7854-4.3-2 positive -- the Initiation the sender transmits
// is the 6-octet common header followed directly by at least two Information
// TLVs that run exactly to the end of the message.
//
// VALIDATES: the Initiation layout on the octets sendInitiation writes. Method:
// read the message raw off a pipe and walk the TLVs from octet 6 by hand.
func TestRFC7854InitiationIsTheCommonHeaderThenTwoOrMoreTLVs(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	ss := &senderSession{name: "test", stopCh: make(chan struct{})}
	result := asyncReadRaw(server)
	if err := ss.sendInitiation(client); err != nil {
		t.Fatalf("sendInitiation: %v", err)
	}
	res := <-result
	if res.err != nil {
		t.Fatalf("read initiation: %v", res.err)
	}
	if res.msg[5] != MsgInitiation {
		t.Fatalf("Message Type = %d, want Initiation %d", res.msg[5], MsgInitiation)
	}
	tlvs := walkTLVs(t, res.msg, CommonHeaderSize)
	if len(tlvs) < 2 {
		t.Errorf("Initiation carries %d Information TLVs after the common header, want two or more", len(tlvs))
	}
}

// RFC requirement: RFC7854-4.3-2 negative -- an Initiation carrying one
// Information TLV, fewer than the two the layout requires, is refused by the
// receiver.
//
// VALIDATES: the receiver does not accept an Initiation short of TLVs. Method:
// write an Initiation with the sysDescr TLV alone and decode it.
func TestRFC7854InitiationWithFewerThanTwoTLVsIsRefused(t *testing.T) {
	buf := make([]byte, 256)
	n := writeInitiation(buf, 0, &Initiation{TLVs: []TLV{makeStringTLV(InitTLVSysDescr, "ze")}})
	if _, err := DecodeMsg(buf[:n]); err == nil {
		t.Error("an Initiation with one Information TLV was accepted")
	}
}

// RFC requirement: RFC7854-4.5-4 positive -- the Termination the sender transmits
// is the 6-octet common header followed directly by one or more TLVs that run
// exactly to the end of the message, one of them the Reason.
//
// VALIDATES: the Termination layout on the octets sendTermination writes.
// Method: read the message raw off a pipe and walk the TLVs from octet 6.
func TestRFC7854TerminationIsTheCommonHeaderThenTLVs(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	result := asyncReadRaw(server)
	ss.sendTermination(client)
	res := <-result
	if res.err != nil {
		t.Fatalf("read termination: %v", res.err)
	}
	if res.msg[5] != MsgTermination {
		t.Fatalf("Message Type = %d, want Termination %d", res.msg[5], MsgTermination)
	}
	tlvs := walkTLVs(t, res.msg, CommonHeaderSize)
	if len(tlvs) == 0 {
		t.Fatal("Termination carries no TLV after the common header")
	}
	reasons := 0
	for _, tlv := range tlvs {
		if tlv.Type == TermTLVReason {
			reasons++
		}
	}
	if reasons != 1 {
		t.Errorf("Termination TLVs %v carry %d Reason TLVs, want 1", tlvs, reasons)
	}
}

// RFC requirement: RFC7854-4.5-4 negative -- a Termination that is the common
// header alone, with no TLV after it, is refused by the receiver.
//
// VALIDATES: the receiver does not accept a Termination with no TLV. Method:
// write a Termination with an empty TLV list and decode it.
func TestRFC7854TerminationWithNoTLVIsRefused(t *testing.T) {
	buf := make([]byte, 64)
	n := writeTermination(buf, 0, &Termination{})
	if n != CommonHeaderSize {
		t.Fatalf("empty Termination is %d octets, want the %d-octet common header", n, CommonHeaderSize)
	}
	if _, err := DecodeMsg(buf[:n]); err == nil {
		t.Error("a Termination with no TLV was accepted")
	}
}

// mirroringTLVs is a Route Mirroring TLV set: a Messages Lost Information TLV,
// then a BGP Message TLV, which RFC 7854 Section 4.7 places last.
func mirroringTLVs() []TLV {
	update := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x17, 0x02, 0x00, 0x00, 0x00, 0x00}
	lost := []byte{0x00, byte(MirrorInfoMessagesLost)}
	return []TLV{
		{Type: MirrorTLVInformation, Length: uint16(len(lost)), Value: lost},
		{Type: MirrorTLVBGPMsg, Length: uint16(len(update)), Value: update},
	}
}

// RFC requirement: RFC7854-4.7-3 positive -- a transmitted Route Mirroring is the
// common header, the per-peer header, then the set of TLVs it was given, in
// order, running exactly to the end of the message.
//
// VALIDATES: the Route Mirroring layout on the writer's octets. Method: decode
// the per-peer header at octet 6 and walk the TLVs from octet 48 by hand.
func TestRFC7854RouteMirroringIsThePerPeerHeaderThenTLVs(t *testing.T) {
	peer := testPeerHeader()
	want := mirroringTLVs()
	buf := make([]byte, 512)
	msg := buf[:writeRouteMirroring(buf, 0, &routeMirroring{Peer: peer, TLVs: want})]

	got, _, err := decodePeerHeader(msg, CommonHeaderSize)
	if err != nil {
		t.Fatalf("no per-peer header after the common header: %v", err)
	}
	if got != peer {
		t.Errorf("per-peer header = %+v, want %+v", got, peer)
	}
	tlvs := walkTLVs(t, msg, CommonHeaderSize+PeerHeaderSize)
	if len(tlvs) != len(want) {
		t.Fatalf("Route Mirroring carries %d TLVs after the per-peer header, want %d", len(tlvs), len(want))
	}
	for i := range want {
		if tlvs[i].Type != want[i].Type {
			t.Errorf("TLV %d type = %d, want %d", i, tlvs[i].Type, want[i].Type)
		}
		if !bytes.Equal(tlvs[i].Value, want[i].Value) {
			t.Errorf("TLV %d value = %x, want %x", i, tlvs[i].Value, want[i].Value)
		}
	}
}

// RFC requirement: RFC7854-4.7-3 negative -- a Route Mirroring whose octets after
// the per-peer header do not form a set of TLVs (a 3-octet tail, shorter than
// a TLV header) is refused by the receiver.
//
// VALIDATES: the receiver does not accept a Route Mirroring body that is not
// TLVs. Method: write a valid message, append three octets, fix the length.
func TestRFC7854RouteMirroringWhoseBodyIsNotTLVsIsRefused(t *testing.T) {
	buf := make([]byte, 512)
	n := writeRouteMirroring(buf, 0, &routeMirroring{Peer: testPeerHeader(), TLVs: mirroringTLVs()})
	valid := buf[:n]
	if _, err := DecodeMsg(valid); err != nil {
		t.Fatalf("the valid Route Mirroring was refused: %v", err)
	}
	msg := append(append([]byte(nil), valid...), 0x00, 0x01, 0x00)
	setMessageLength(msg)
	if _, err := DecodeMsg(msg); err == nil {
		t.Error("a Route Mirroring with a 3-octet tail that is no TLV was accepted")
	}
}

// statsCounters is three counters, two 4-octet and one 8-octet gauge.
func statsCounters() []StatEntry {
	return []StatEntry{
		{Type: 0, Value: []byte{0, 0, 0, 7}},
		{Type: 1, Value: []byte{0, 0, 0, 9}},
		{Type: 7, Value: []byte{0, 0, 0, 0, 0, 0, 0, 3}},
	}
}

// RFC requirement: RFC7854-4.8-3 positive -- a transmitted Stats Report carries,
// right after the per-peer header, a 4-octet count equal to the number of
// counters, and exactly that many counter TLVs follow to the end of the message.
//
// VALIDATES: the Stats Report layout on the writer's octets. Method: read the
// count at octet 48 raw and walk the TLVs after it by hand.
func TestRFC7854StatsReportCountNamesTheCountersThatFollow(t *testing.T) {
	want := statsCounters()
	buf := make([]byte, 512)
	msg := buf[:writeStatisticsReport(buf, 0, &statisticsReport{Peer: testPeerHeader(), Stats: want})]

	off := CommonHeaderSize + PeerHeaderSize
	if len(msg) < off+4 {
		t.Fatalf("Stats Report is %d octets, no room for the count", len(msg))
	}
	if got := binary.BigEndian.Uint32(msg[off : off+4]); got != uint32(len(want)) {
		t.Errorf("Stats Count = %d, want %d", got, len(want))
	}
	tlvs := walkTLVs(t, msg, off+4)
	if len(tlvs) != len(want) {
		t.Fatalf("%d counter TLVs follow the count, want %d", len(tlvs), len(want))
	}
	for i := range want {
		if tlvs[i].Type != want[i].Type {
			t.Errorf("counter %d type = %d, want %d", i, tlvs[i].Type, want[i].Type)
		}
		if !bytes.Equal(tlvs[i].Value, want[i].Value) {
			t.Errorf("counter %d value = %x, want %x", i, tlvs[i].Value, want[i].Value)
		}
	}
}

// RFC requirement: RFC7854-4.8-3 negative -- a Stats Report whose count does not
// match the counter TLVs that follow is refused by the receiver, whether the
// count claims one counter more or one counter fewer than the message carries.
//
// VALIDATES: the receiver holds the count to the counters. Method: write a valid
// report, rewrite the count at octet 48, decode.
func TestRFC7854StatsReportCountDisagreeingWithItsCountersIsRefused(t *testing.T) {
	counters := statsCounters()
	buf := make([]byte, 512)
	n := writeStatisticsReport(buf, 0, &statisticsReport{Peer: testPeerHeader(), Stats: counters})
	valid := buf[:n]
	if _, err := DecodeMsg(valid); err != nil {
		t.Fatalf("the valid Stats Report was refused: %v", err)
	}
	off := CommonHeaderSize + PeerHeaderSize
	for _, count := range []uint32{uint32(len(counters)) + 1, uint32(len(counters)) - 1} {
		msg := append([]byte(nil), valid...)
		binary.BigEndian.PutUint32(msg[off:off+4], count)
		if _, err := DecodeMsg(msg); err == nil {
			t.Errorf("a Stats Report counting %d counters over %d TLVs was accepted", count, len(counters))
		}
	}
}

// RFC requirement: RFC7854-4.10-1 positive -- a transmitted Peer Up is the common
// header, the per-peer header, then Local Address (16 octets), Local Port,
// Remote Port, the Sent OPEN and the Received OPEN, at those offsets and in that
// order, running exactly to the end of the message.
//
// VALIDATES: the Peer Up layout on the writer's octets. Method: read each field
// raw at its offset from octet 48.
func TestRFC7854PeerUpIsThePerPeerHeaderThenTheFixedFieldsAndBothOPENs(t *testing.T) {
	peer := testPeerHeader()
	pu := PeerUp{Peer: peer, LocalPort: 179, RemotePort: 50000, SentOpenMsg: makeBGPOpen(65001, 0x01020304), ReceivedOpenMsg: makeBGPOpen(65002, 0x05060708)}
	pu.LocalAddress[15] = 9
	buf := make([]byte, 512)
	msg := buf[:writePeerUp(buf, 0, &pu)]

	got, _, err := decodePeerHeader(msg, CommonHeaderSize)
	if err != nil {
		t.Fatalf("no per-peer header after the common header: %v", err)
	}
	if got != peer {
		t.Errorf("per-peer header = %+v, want %+v", got, peer)
	}
	off := CommonHeaderSize + PeerHeaderSize
	want := len(pu.SentOpenMsg) + len(pu.ReceivedOpenMsg)
	if len(msg) != off+16+2+2+want {
		t.Fatalf("Peer Up is %d octets, want %d", len(msg), off+16+2+2+want)
	}
	if !bytes.Equal(msg[off:off+16], pu.LocalAddress[:]) {
		t.Errorf("Local Address = %x, want %x", msg[off:off+16], pu.LocalAddress)
	}
	if got := binary.BigEndian.Uint16(msg[off+16 : off+18]); got != pu.LocalPort {
		t.Errorf("Local Port = %d, want %d", got, pu.LocalPort)
	}
	if got := binary.BigEndian.Uint16(msg[off+18 : off+20]); got != pu.RemotePort {
		t.Errorf("Remote Port = %d, want %d", got, pu.RemotePort)
	}
	sent := msg[off+20 : off+20+len(pu.SentOpenMsg)]
	if !bytes.Equal(sent, pu.SentOpenMsg) {
		t.Errorf("Sent OPEN = %x, want %x", sent, pu.SentOpenMsg)
	}
	received := msg[off+20+len(pu.SentOpenMsg):]
	if !bytes.Equal(received, pu.ReceivedOpenMsg) {
		t.Errorf("Received OPEN = %x, want %x", received, pu.ReceivedOpenMsg)
	}
}

// RFC requirement: RFC7854-4.10-1 negative -- a Peer Up whose data after the
// per-peer header stops inside the fixed fields (Local Address, Local Port,
// Remote Port) is refused by the receiver.
//
// VALIDATES: the receiver does not accept a Peer Up short of its layout.
// Method: write a valid Peer Up, cut it 10 octets into the fixed fields, fix the
// length, decode.
func TestRFC7854PeerUpShorterThanItsFixedFieldsIsRefused(t *testing.T) {
	pu := PeerUp{Peer: testPeerHeader(), LocalPort: 179, RemotePort: 50000, SentOpenMsg: makeBGPOpen(65001, 0x01020304), ReceivedOpenMsg: makeBGPOpen(65002, 0x05060708)}
	buf := make([]byte, 512)
	n := writePeerUp(buf, 0, &pu)
	if _, err := DecodeMsg(buf[:n]); err != nil {
		t.Fatalf("the valid Peer Up was refused: %v", err)
	}
	msg := append([]byte(nil), buf[:CommonHeaderSize+PeerHeaderSize+10]...)
	setMessageLength(msg)
	if _, err := DecodeMsg(msg); err == nil {
		t.Error("a Peer Up cut inside its fixed fields was accepted")
	}
}
