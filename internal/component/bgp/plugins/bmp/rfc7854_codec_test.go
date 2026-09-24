// Design: docs/guide/bmp.md -- mandatory BMP TLVs and hostile framing boundaries.
// RFC: rfc/short/rfc7854.md

package bmp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
)

// RFC requirement: RFC7854-x-18 negative -- a sysName alone cannot replace the mandatory sysDescr.
// The live receiver must close before consuming the following valid Termination.
// MUTATION: removing decodeInitiation's sysDescr presence check keeps the session open.
func TestRFC7854InitiationMissingSysDescrEndsSession(t *testing.T) {
	var buf [128]byte
	n := writeInitiation(buf[:], 0, &Initiation{TLVs: []TLV{
		makeStringTLV(InitTLVSysName, "router"),
	}})
	if err := receiverSurvives(t, buf[:n]); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("message after missing sysDescr = %v, want closed pipe", err)
	}
}

// Both identity TLVs are required independently; a description cannot replace a name.
func TestBMPInitiationMissingSysNameEndsSession(t *testing.T) {
	var buf [128]byte
	n := writeInitiation(buf[:], 0, &Initiation{TLVs: []TLV{
		makeStringTLV(InitTLVSysDescr, "router software"),
	}})
	if err := receiverSurvives(t, buf[:n]); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("message after missing sysName = %v, want closed pipe", err)
	}
}

// RFC requirement: RFC7854-x-18 positive -- both identity TLVs are accepted even with empty values.
// Presence, not a made-up minimum string length, is the Section 4.3 requirement.
// MUTATION: rejecting empty sysDescr values closes this otherwise valid session.
func TestRFC7854InitiationEmptyIdentityValuesAccepted(t *testing.T) {
	var buf [128]byte
	n := writeInitiation(buf[:], 0, &Initiation{TLVs: []TLV{
		makeStringTLV(InitTLVSysName, ""),
		makeStringTLV(InitTLVSysDescr, ""),
	}})
	if err := receiverSurvives(t, buf[:n]); err != nil {
		t.Fatalf("message after valid identity = %v, want accepted", err)
	}
}

// RFC requirement: RFC7854-4.5-3 positive -- the live sender's shutdown carries a two-byte administrative Reason.
// MUTATION: removing the Reason from writeTerminationLocked makes its wire output undecodable.
func TestRFC7854SenderTerminationCarriesReason(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "collector")
	defer closeLog(client, "sender")
	ss := &senderSession{name: "test", conn: client, stopCh: make(chan struct{})}
	result := asyncRead(server)
	ss.stop()
	got := <-result
	if got.err != nil {
		t.Fatalf("read termination: %v", got.err)
	}
	term, ok := got.msg.(*Termination)
	if !ok {
		t.Fatalf("shutdown message = %T, want *Termination", got.msg)
	}
	reasons := 0
	for _, tlv := range term.TLVs {
		if tlv.Type != TermTLVReason {
			continue
		}
		reasons++
		if !bytes.Equal(tlv.Value, []byte{0, 0}) {
			t.Errorf("shutdown reason = %x, want administrative close 0000", tlv.Value)
		}
	}
	if reasons != 1 {
		t.Fatalf("shutdown carries %d Reasons, want 1", reasons)
	}
}

// RFC requirement: RFC7854-4.5-3 negative -- optional strings and malformed Reason TLVs do not form a valid Termination.
// DecodeMsg's error distinguishes malformed termination from the normal close;
// the receiver check also proves these bytes cannot leave a live session behind.
// MUTATION: dropping the required-Reason or width check accepts one of these messages.
func TestRFC7854TerminationRequiresTwoByteReason(t *testing.T) {
	for _, tt := range []struct {
		name string
		tlvs []TLV
	}{
		{"absent", []TLV{makeStringTLV(TermTLVString, "done")}},
		{"empty", []TLV{{Type: TermTLVReason}}},
		{"short", []TLV{{Type: TermTLVReason, Length: 1, Value: []byte{0}}}},
		{"long", []TLV{{Type: TermTLVReason, Length: 3, Value: []byte{0, 0, 0}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var buf [128]byte
			n := writeTermination(buf[:], 0, &Termination{TLVs: tt.tlvs})
			if _, err := DecodeMsg(buf[:n]); err == nil {
				t.Fatal("malformed Termination decoded successfully")
			}
			if err := receiverSurvives(t, buf[:n]); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("message after malformed Termination = %v, want closed pipe", err)
			}
		})
	}
}

// mirrorBytes supplies the actual receiver with a complete per-peer message.
func mirrorBytes(tlvs ...TLV) []byte {
	buf := make([]byte, 256)
	n := writeRouteMirroring(buf, 0, &routeMirroring{Peer: testPeerHeader(), TLVs: tlvs})
	return buf[:n]
}

// RFC requirement: RFC7854-4.7-1 positive -- Information precedes the last BGP Message TLV.
// RFC requirement: RFC7854-4.7-2 positive -- an Errored PDU has its BGP Message TLV and survives receipt.
// The PDU is deliberately malformed: a monitor must still accept the error report.
// MUTATION: rejecting all Information code 0 messages closes this valid report.
func TestRFC7854ErroredMirrorWithPDUAccepted(t *testing.T) {
	buf := mirrorBytes(
		TLV{Type: MirrorTLVInformation, Length: 2, Value: []byte{0, 0}},
		TLV{Type: MirrorTLVBGPMsg, Length: 1, Value: []byte{0xff}},
	)
	if err := receiverSurvives(t, buf); err != nil {
		t.Fatalf("message after errored PDU with BGP TLV = %v, want accepted", err)
	}
}

// RFC requirement: RFC7854-4.7-1 negative -- a BGP Message before Information is refused, not silently reordered.
// MUTATION: dropping decodeRouteMirroring's last-TLV check lets the next message through.
func TestRFC7854MirrorBGPMessageNotLastEndsSession(t *testing.T) {
	buf := mirrorBytes(
		TLV{Type: MirrorTLVBGPMsg, Length: 1, Value: []byte{0xff}},
		TLV{Type: MirrorTLVInformation, Length: 2, Value: []byte{0, 1}},
	)
	if err := receiverSurvives(t, buf); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("message after misplaced BGP TLV = %v, want closed pipe", err)
	}
}

// RFC requirement: RFC7854-4.7-2 negative -- Errored PDU without a BGP Message TLV cannot be accepted.
// MUTATION: removing the errored-PDU dependency lets the following Termination through.
func TestRFC7854ErroredMirrorWithoutPDUEndsSession(t *testing.T) {
	buf := mirrorBytes(TLV{Type: MirrorTLVInformation, Length: 2, Value: []byte{0, 0}})
	if err := receiverSurvives(t, buf); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("message after errored PDU without BGP TLV = %v, want closed pipe", err)
	}
}

// A lost-message report has no PDU to supply, and unknown Information codes are not code 0.
func TestBMPMirrorInformationWithoutPDUs(t *testing.T) {
	for _, code := range []uint16{MirrorInfoMessagesLost, 60000} {
		var value [2]byte
		binary.BigEndian.PutUint16(value[:], code)
		buf := mirrorBytes(TLV{Type: MirrorTLVInformation, Length: 2, Value: value[:]})
		if err := receiverSurvives(t, buf); err != nil {
			t.Fatalf("message after Information code %d = %v, want accepted", code, err)
		}
	}
}

// Information codes have a two-byte width, even when no BGP Message TLV follows.
func TestBMPMirrorMalformedInformationEndsSession(t *testing.T) {
	buf := mirrorBytes(TLV{Type: MirrorTLVInformation, Length: 1, Value: []byte{1}})
	if err := receiverSurvives(t, buf); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("message after short Information code = %v, want closed pipe", err)
	}
}

// RFC requirement: RFC7854-4.8-2 negative -- an empty report is refused before it reaches a connected collector.
// The next valid report is the collector's first message, not an empty predecessor.
// MUTATION: removing senderSession.writeStatisticsReport's empty guard returns nil here.
func TestRFC7854SenderRefusesEmptyStatistics(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "collector")
	defer closeLog(client, "sender")
	ss := newTestSession(t, "test", client)
	result := asyncRead(server)
	if err := ss.writeStatisticsReport(testPeerHeader(), nil); err == nil {
		t.Fatal("empty Statistics Report accepted for transmission")
	}
	stats := []StatEntry{{Type: StatDuplicateUpdates, Value: []byte{0, 0, 0, 7}}}
	if err := ss.writeStatisticsReport(testPeerHeader(), stats); err != nil {
		t.Fatalf("send valid Statistics Report after refusal: %v", err)
	}
	got := <-result
	if got.err != nil {
		t.Fatalf("read first report: %v", got.err)
	}
	report, ok := got.msg.(*statisticsReport)
	if !ok {
		t.Fatalf("message = %T, want *statisticsReport", got.msg)
	}
	if len(report.Stats) != 1 {
		t.Fatalf("first report has %d statistics, want 1", len(report.Stats))
	}
	if report.Stats[0].Type != StatDuplicateUpdates {
		t.Fatalf("first statistic type = %d, want duplicate updates", report.Stats[0].Type)
	}
	if !bytes.Equal(report.Stats[0].Value, stats[0].Value) {
		t.Fatalf("first statistic value = %x, want 00000007", report.Stats[0].Value)
	}
}

// The declared count bounds the complete statistics body, not just allocation.
func TestBMPStatisticsCountMustMatchBody(t *testing.T) {
	if err := receiverSurvives(t, statsReportBytes(nil)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("message after empty statistics = %v, want closed pipe", err)
	}
	for _, tt := range []struct {
		name  string
		count uint32
	}{
		{"empty", 0},
		{"missing", 2},
		{"hostile", ^uint32(0)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf := statsReportBytes([]StatEntry{{Type: StatDuplicateUpdates, Value: []byte{0, 0, 0, 7}}})
			binary.BigEndian.PutUint32(buf[CommonHeaderSize+PeerHeaderSize:], tt.count)
			if err := receiverSurvives(t, buf); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("message after invalid stats count = %v, want closed pipe", err)
			}
		})
	}
	buf := statsReportBytes([]StatEntry{
		{Type: StatDuplicateUpdates, Value: []byte{0, 0, 0, 7}},
		{Type: StatDuplicatePrefix, Value: []byte{0, 0, 0, 8}},
	})
	binary.BigEndian.PutUint32(buf[CommonHeaderSize+PeerHeaderSize:], 1)
	if err := receiverSurvives(t, buf); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("message after trailing statistics = %v, want closed pipe", err)
	}
}

// DecodeMsg accepts exactly one complete frame; hostile length fields must never panic.
func TestBMPDecodeExactFrameBoundaries(t *testing.T) {
	for _, length := range []uint32{0, 1, CommonHeaderSize - 1, CommonHeaderSize + 1, ^uint32(0)} {
		var buf [CommonHeaderSize + PeerHeaderSize]byte
		WriteCommonHeader(buf[:], 0, CommonHeader{Version: Version, Length: length, Type: MsgRouteMonitoring})
		if _, err := DecodeMsg(buf[:]); err == nil {
			t.Errorf("accepted declared length %d in %d-byte buffer", length, len(buf))
		}
	}
	var buf [32]byte
	n := writeTermination(buf[:], 0, &Termination{TLVs: []TLV{
		{Type: TermTLVReason, Length: 2, Value: []byte{0, 1}},
	}})
	if _, err := DecodeMsg(buf[:n]); err != nil {
		t.Fatalf("exact frame: %v", err)
	}
	if _, err := DecodeMsg(buf[:n+1]); err == nil {
		t.Fatal("accepted trailing byte outside the declared frame")
	}
}

// TLV slices stop at the caller's boundary even if the backing buffer contains more bytes.
func TestBMPTLVDecodeBounds(t *testing.T) {
	buf := []byte{0, 2, 0, 2, 'o', 'k'}
	for _, tt := range []struct {
		off int
		end int
	}{
		{-1, len(buf)},
		{0, -1},
		{3, 2},
		{0, len(buf) + 1},
		{0, len(buf) - 1},
		{len(buf) + 1, len(buf) + 1},
	} {
		if _, err := DecodeTLVs(buf, tt.off, tt.end); err == nil {
			t.Errorf("accepted TLV range [%d:%d]", tt.off, tt.end)
		}
	}
	for _, off := range []int{-1, len(buf) + 1} {
		if _, _, err := DecodeTLV(buf, off); err == nil {
			t.Errorf("accepted TLV offset %d", off)
		}
	}
	tlvs, err := DecodeTLVs(buf, 0, len(buf))
	if err != nil {
		t.Fatalf("exact TLV range: %v", err)
	}
	if len(tlvs) != 1 {
		t.Fatalf("decoded %d TLVs, want 1", len(tlvs))
	}
	if string(tlvs[0].Value) != "ok" {
		t.Fatalf("decoded value = %q, want ok", tlvs[0].Value)
	}
}

// Arbitrary complete frames must never panic, and successful decodes consume
// exactly the frame's declared length rather than borrowing trailing bytes.
func FuzzBMPDecodeMsg(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{Version, 0, 0, 0, 0, MsgInitiation})
	f.Add([]byte{Version, 0xff, 0xff, 0xff, 0xff, MsgStatisticsReport})
	var buf [32]byte
	n := writeInitiation(buf[:], 0, &Initiation{TLVs: []TLV{
		makeStringTLV(InitTLVSysName, ""),
		makeStringTLV(InitTLVSysDescr, ""),
	}})
	f.Add(buf[:n])
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := DecodeMsg(data)
		if err != nil {
			return
		}
		if len(data) < CommonHeaderSize {
			t.Fatal("accepted a frame shorter than the common header")
		}
		if uint64(binary.BigEndian.Uint32(data[1:5])) != uint64(len(data)) {
			t.Fatal("accepted a frame with a mismatched length")
		}
	})
}
