package bmp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// makeBGPOpen builds a minimal BGP OPEN message (29 bytes).
// 16-byte marker + 2-byte length + 1-byte type(1) + 1-byte version(4) +
// 2-byte AS + 2-byte hold(90) + 4-byte ID + 1-byte opt-len(0).
func makeBGPOpen(asn uint16, routerID uint32) []byte {
	buf := make([]byte, 29)
	// 16-byte marker (all 0xFF).
	for i := range 16 {
		buf[i] = 0xFF
	}
	binary.BigEndian.PutUint16(buf[16:18], 29) // length
	buf[18] = 1                                // type = OPEN
	buf[19] = 4                                // BGP version
	binary.BigEndian.PutUint16(buf[20:22], asn)
	binary.BigEndian.PutUint16(buf[22:24], 90) // hold time
	binary.BigEndian.PutUint32(buf[24:28], routerID)
	buf[28] = 0 // opt params length
	return buf
}

func testPeerHeader() PeerHeader {
	p := PeerHeader{
		PeerType:      PeerTypeGlobal,
		Flags:         PeerFlagV,
		Distinguisher: 100,
		PeerAS:        65001,
		PeerBGPID:     0x01020304,
		TimestampSec:  1700000000,
		TimestampUsec: 123456,
	}
	p.Address[10] = 0xff
	p.Address[11] = 0xff
	p.Address[12] = 10
	p.Address[13] = 0
	p.Address[14] = 0
	p.Address[15] = 1
	return p
}

func TestBMPInitiationRoundTrip(t *testing.T) {
	// VALIDATES: AC-7, AC-9 -- Initiation encode then decode
	init := &Initiation{
		TLVs: []TLV{
			MakeStringTLV(InitTLVSysName, "router1"),
			MakeStringTLV(InitTLVSysDescr, "ze test"),
		},
	}
	buf := make([]byte, 512)
	n := WriteInitiation(buf, 0, init)

	msg, err := DecodeMsg(buf[:n])
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	decoded, ok := msg.(*Initiation)
	if !ok {
		t.Fatalf("expected *Initiation, got %T", msg)
	}
	if len(decoded.TLVs) != 2 {
		t.Fatalf("got %d TLVs, want 2", len(decoded.TLVs))
	}
	if string(decoded.TLVs[0].Value) != "router1" {
		t.Errorf("sysName = %q, want %q", string(decoded.TLVs[0].Value), "router1")
	}
	if string(decoded.TLVs[1].Value) != "ze test" {
		t.Errorf("sysDescr = %q, want %q", string(decoded.TLVs[1].Value), "ze test")
	}
}

func TestBMPTerminationRoundTrip(t *testing.T) {
	// VALIDATES: AC-9 -- Termination encode then decode
	term := &Termination{
		TLVs: []TLV{
			MakeStringTLV(TermTLVString, "goodbye"),
		},
	}
	buf := make([]byte, 512)
	n := WriteTermination(buf, 0, term)

	msg, err := DecodeMsg(buf[:n])
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	decoded, ok := msg.(*Termination)
	if !ok {
		t.Fatalf("expected *Termination, got %T", msg)
	}
	if len(decoded.TLVs) != 1 {
		t.Fatalf("got %d TLVs, want 1", len(decoded.TLVs))
	}
	if string(decoded.TLVs[0].Value) != "goodbye" {
		t.Errorf("value = %q, want %q", string(decoded.TLVs[0].Value), "goodbye")
	}
}

func TestBMPPeerUpRoundTrip(t *testing.T) {
	// VALIDATES: AC-9 -- Peer Up encode then decode with OPEN messages
	sentOpen := makeBGPOpen(65001, 0x01020304)
	recvOpen := makeBGPOpen(65002, 0x05060708)

	pu := &PeerUp{
		Peer:            testPeerHeader(),
		LocalPort:       179,
		RemotePort:      54321,
		SentOpenMsg:     sentOpen,
		ReceivedOpenMsg: recvOpen,
	}
	pu.LocalAddress[12] = 192
	pu.LocalAddress[13] = 168
	pu.LocalAddress[14] = 1
	pu.LocalAddress[15] = 1

	buf := make([]byte, 1024)
	n := WritePeerUp(buf, 0, pu)

	msg, err := DecodeMsg(buf[:n])
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	decoded, ok := msg.(*PeerUp)
	if !ok {
		t.Fatalf("expected *PeerUp, got %T", msg)
	}
	if decoded.LocalPort != 179 {
		t.Errorf("local port = %d, want 179", decoded.LocalPort)
	}
	if decoded.RemotePort != 54321 {
		t.Errorf("remote port = %d, want 54321", decoded.RemotePort)
	}
	if !bytes.Equal(decoded.SentOpenMsg, sentOpen) {
		t.Error("sent OPEN mismatch")
	}
	if !bytes.Equal(decoded.ReceivedOpenMsg, recvOpen) {
		t.Error("received OPEN mismatch")
	}
	if decoded.Peer.PeerAS != 65001 {
		t.Errorf("peer AS = %d, want 65001", decoded.Peer.PeerAS)
	}
}

func TestBMPPeerDownRoundTrip(t *testing.T) {
	// VALIDATES: AC-9 -- Peer Down encode then decode for all 5 reasons
	tests := []struct {
		name   string
		reason uint8
		data   []byte
	}{
		{"local notify", PeerDownLocalNotify, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0, 21, 3, 6, 4}},
		{"local no-notify", PeerDownLocalNoNotify, []byte{0, 7}},
		{"remote notify", PeerDownRemoteNotify, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0, 21, 3, 2, 0}},
		{"remote no-data", PeerDownRemoteNoData, nil},
		{"deconfigured", PeerDownDeconfigured, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd := &PeerDown{
				Peer:   testPeerHeader(),
				Reason: tt.reason,
				Data:   tt.data,
			}
			buf := make([]byte, 512)
			n := WritePeerDown(buf, 0, pd)

			msg, err := DecodeMsg(buf[:n])
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}
			decoded, ok := msg.(*PeerDown)
			if !ok {
				t.Fatalf("expected *PeerDown, got %T", msg)
			}
			if decoded.Reason != tt.reason {
				t.Errorf("reason = %d, want %d", decoded.Reason, tt.reason)
			}
			if !bytes.Equal(decoded.Data, tt.data) {
				t.Errorf("data mismatch: got %v, want %v", decoded.Data, tt.data)
			}
		})
	}
}

func TestBMPRouteMonitoringRoundTrip(t *testing.T) {
	// VALIDATES: AC-9 -- Route Monitoring encode then decode
	bgpUpdate := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}
	rm := &RouteMonitoring{
		Peer:      testPeerHeader(),
		BGPUpdate: bgpUpdate,
	}
	buf := make([]byte, 512)
	n := WriteRouteMonitoring(buf, 0, rm)

	msg, err := DecodeMsg(buf[:n])
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	decoded, ok := msg.(*RouteMonitoring)
	if !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", msg)
	}
	if !bytes.Equal(decoded.BGPUpdate, bgpUpdate) {
		t.Errorf("BGP update mismatch: got %x, want %x", decoded.BGPUpdate, bgpUpdate)
	}
	if decoded.Peer.PeerAS != 65001 {
		t.Errorf("peer AS = %d, want 65001", decoded.Peer.PeerAS)
	}
}

func TestBMPStatisticsReportRoundTrip(t *testing.T) {
	// VALIDATES: AC-8, AC-9 -- Statistics Report with counter TLVs
	val1 := make([]byte, 8)
	binary.BigEndian.PutUint64(val1, 42)
	val2 := make([]byte, 8)
	binary.BigEndian.PutUint64(val2, 1000)

	sr := &StatisticsReport{
		Peer: testPeerHeader(),
		Stats: []StatEntry{
			{Type: StatPrefixesRejected, Value: val1},
			{Type: StatRoutesAdjRIBIn, Value: val2},
		},
	}
	buf := make([]byte, 512)
	n := WriteStatisticsReport(buf, 0, sr)

	msg, err := DecodeMsg(buf[:n])
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	decoded, ok := msg.(*StatisticsReport)
	if !ok {
		t.Fatalf("expected *StatisticsReport, got %T", msg)
	}
	if len(decoded.Stats) != 2 {
		t.Fatalf("got %d stats, want 2", len(decoded.Stats))
	}
	if decoded.Stats[0].Type != StatPrefixesRejected {
		t.Errorf("stat[0] type = %d, want %d", decoded.Stats[0].Type, StatPrefixesRejected)
	}
	if binary.BigEndian.Uint64(decoded.Stats[0].Value) != 42 {
		t.Errorf("stat[0] value = %d, want 42", binary.BigEndian.Uint64(decoded.Stats[0].Value))
	}
	if decoded.Stats[1].Type != StatRoutesAdjRIBIn {
		t.Errorf("stat[1] type = %d, want %d", decoded.Stats[1].Type, StatRoutesAdjRIBIn)
	}
}

func TestBMPRouteMirroringRoundTrip(t *testing.T) {
	// VALIDATES: AC-9 -- Route Mirroring encode then decode
	bgpPDU := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	rm := &RouteMirroring{
		Peer: testPeerHeader(),
		TLVs: []TLV{
			{Type: MirrorTLVBGPMsg, Length: uint16(len(bgpPDU)), Value: bgpPDU},
		},
	}
	buf := make([]byte, 512)
	n := WriteRouteMirroring(buf, 0, rm)

	msg, err := DecodeMsg(buf[:n])
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	decoded, ok := msg.(*RouteMirroring)
	if !ok {
		t.Fatalf("expected *RouteMirroring, got %T", msg)
	}
	if len(decoded.TLVs) != 1 {
		t.Fatalf("got %d TLVs, want 1", len(decoded.TLVs))
	}
	if !bytes.Equal(decoded.TLVs[0].Value, bgpPDU) {
		t.Errorf("BGP PDU mismatch")
	}
}

func TestBMPDecodeBadVersion(t *testing.T) {
	// VALIDATES: AC-5 -- version != 3 returns error
	buf := []byte{2, 0, 0, 0, 6, MsgInitiation}
	_, err := DecodeMsg(buf)
	if err == nil {
		t.Fatal("expected error for bad version")
	}
}

func TestBMPDecodeBadType(t *testing.T) {
	// VALIDATES: AC-6 -- unknown type returns error
	buf := []byte{3, 0, 0, 0, 6, 7} // type 7 does not exist
	_, err := DecodeMsg(buf)
	if err == nil {
		t.Fatal("expected error for bad message type")
	}
}

func TestBMPDecodeTruncated(t *testing.T) {
	// VALIDATES: AC-6 -- length exceeds buffer returns error
	buf := []byte{3, 0, 0, 0, 100, MsgInitiation} // claims 100 bytes, only 6
	_, err := DecodeMsg(buf)
	if err == nil {
		t.Fatal("expected error for truncated message")
	}
}
