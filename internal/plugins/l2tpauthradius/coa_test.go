package l2tpauthradius

import (
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	"codeberg.org/thomas-mangin/ze/internal/component/radius"
)

func sendCoAPacket(t *testing.T, addr string, code uint8, secret []byte, attrs []radius.Attr) *radius.Packet {
	t.Helper()
	return sendRawCoAPacket(t, addr, buildCoAPacket(t, code, secret, attrs, time.Now()))
}

func buildCoAPacket(t *testing.T, code uint8, secret []byte, attrs []radius.Attr, ts time.Time) []byte {
	t.Helper()
	if !ts.IsZero() {
		tsAttr := make([]byte, 4)
		binary.BigEndian.PutUint32(tsAttr, uint32(ts.Unix()))
		attrs = append(attrs, radius.Attr{Type: radius.AttrEventTimestamp, Value: tsAttr})
	}
	pkt := &radius.Packet{
		Code:       code,
		Identifier: 1,
		Attrs:      attrs,
	}

	buf := make([]byte, radius.MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}

	auth := radius.AccountingRequestAuth(buf, n, secret)
	copy(buf[4:4+radius.AuthenticatorLen], auth[:])
	return append([]byte(nil), buf[:n]...)
}

func sendRawCoAPacket(t *testing.T, addr string, wire []byte) *radius.Packet {
	t.Helper()
	conn, err := net.DialUDP("udp4", nil, mustResolveUDP(t, addr))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn close: %v", err)
		}
	}()

	if _, err := conn.Write(wire); err != nil {
		t.Fatal(err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	respBuf := make([]byte, radius.MaxPacketLen)
	rn, err := conn.Read(respBuf)
	if err != nil {
		t.Fatal("no response:", err)
	}

	resp, err := radius.Decode(respBuf[:rn])
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

type fakeL2TPService struct {
	snap      l2tp.Snapshot
	teardowns atomic.Int32
}

func (f *fakeL2TPService) Snapshot() l2tp.Snapshot { return f.snap }
func (f *fakeL2TPService) LookupTunnel(localTID uint16) (l2tp.TunnelSnapshot, bool) {
	for i := range f.snap.Tunnels {
		if f.snap.Tunnels[i].LocalTID == localTID {
			return f.snap.Tunnels[i], true
		}
	}
	return l2tp.TunnelSnapshot{}, false
}
func (f *fakeL2TPService) LookupSession(localSID uint16) (l2tp.SessionSnapshot, bool) {
	for i := range f.snap.Tunnels {
		for _, session := range f.snap.Tunnels[i].Sessions {
			if session.LocalSID == localSID {
				return session, true
			}
		}
	}
	return l2tp.SessionSnapshot{}, false
}
func (f *fakeL2TPService) Listeners() []l2tp.ListenerSnapshot   { return nil }
func (f *fakeL2TPService) EffectiveConfig() l2tp.ConfigSnapshot { return l2tp.ConfigSnapshot{} }
func (f *fakeL2TPService) TeardownTunnel(uint16) error          { return nil }
func (f *fakeL2TPService) TeardownSession(uint16) error {
	f.teardowns.Add(1)
	return nil
}
func (f *fakeL2TPService) TeardownAllTunnels() int                                 { return 0 }
func (f *fakeL2TPService) TeardownAllSessions() int                                { return 0 }
func (f *fakeL2TPService) SessionEvents(uint16) []l2tp.ObserverEvent               { return nil }
func (f *fakeL2TPService) LoginSamples(string) []l2tp.CQMBucket                    { return nil }
func (f *fakeL2TPService) SessionSummaries() []l2tp.SessionSummary                 { return nil }
func (f *fakeL2TPService) LoginSummaries() []l2tp.LoginSummary                     { return nil }
func (f *fakeL2TPService) EchoState(string) *l2tp.LoginEchoState                   { return nil }
func (f *fakeL2TPService) ReliableStats(uint16) *l2tp.ReliableStats                { return nil }
func (f *fakeL2TPService) TunnelFSMHistory(uint16) []l2tp.FSMTransition            { return nil }
func (f *fakeL2TPService) SessionFSMHistory(uint16) []l2tp.FSMTransition           { return nil }
func (f *fakeL2TPService) CaptureSnapshot(int, uint16, string) []l2tp.CaptureEntry { return nil }
func (f *fakeL2TPService) RecordDisconnect(uint16, string, string, uint32)         {}

func mustResolveUDP(t *testing.T, addr string) *net.UDPAddr {
	t.Helper()
	a, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// VALIDATES: AC-4 -- invalid authenticator silently discarded.
func TestCoAListenerInvalidAuth(t *testing.T) {
	secret := []byte("test-coa-secret")
	cl, err := newCoAListener(0, nil, secret, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()
	addr := cl.conn.LocalAddr().String()

	pkt := &radius.Packet{
		Code:       radius.CodeCoARequest,
		Identifier: 1,
		Attrs: []radius.Attr{
			{Type: radius.AttrAcctSessionID, Value: radius.AttrString("1-100")},
		},
	}

	buf := make([]byte, radius.MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Leave authenticator as zeros (wrong).

	conn, err := net.DialUDP("udp4", nil, mustResolveUDP(t, addr))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Logf("conn close: %v", err)
		}
	}()

	if _, err := conn.Write(buf[:n]); err != nil {
		t.Fatal(err)
	}

	// Should get no response (silently discarded).
	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	respBuf := make([]byte, radius.MaxPacketLen)
	_, readErr := conn.Read(respBuf)
	if readErr == nil {
		t.Fatal("expected timeout (no response for invalid auth), got a response")
	}
}

// VALIDATES: AC-5 -- CoA for unknown session returns NAK with Error-Cause 503.
func TestCoAListenerUnknownSession(t *testing.T) {
	secret := []byte("test-coa-secret-2")
	cl, err := newCoAListener(0, nil, secret, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()
	addr := cl.conn.LocalAddr().String()

	resp := sendCoAPacket(t, addr, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("999-999")},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10mbit")},
	})

	if resp.Code != radius.CodeCoANAK {
		t.Errorf("code: got %d, want %d (CoA-NAK)", resp.Code, radius.CodeCoANAK)
	}
	errCause := resp.FindAttr(radius.AttrErrorCause)
	if len(errCause) != 4 {
		t.Fatalf("Error-Cause missing or wrong length: %v", errCause)
	}
	causeVal := binary.BigEndian.Uint32(errCause)
	if causeVal != radius.ErrorCauseSessionNotFound {
		t.Errorf("Error-Cause: got %d, want %d", causeVal, radius.ErrorCauseSessionNotFound)
	}
}

func TestCoAListenerMissingEventTimestamp(t *testing.T) {
	secret := []byte("test-coa-secret-missing-ts")
	cl, err := newCoAListener(0, nil, secret, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()
	addr := cl.conn.LocalAddr().String()

	wire := buildCoAPacket(t, radius.CodeCoARequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("999-999")},
	}, time.Time{})
	resp := sendRawCoAPacket(t, addr, wire)

	if resp.Code != radius.CodeCoANAK {
		t.Errorf("code: got %d, want %d (CoA-NAK)", resp.Code, radius.CodeCoANAK)
	}
	errCause := resp.FindAttr(radius.AttrErrorCause)
	if len(errCause) != 4 {
		t.Fatalf("Error-Cause missing or wrong length: %v", errCause)
	}
	if got := binary.BigEndian.Uint32(errCause); got != radius.ErrorCauseInvalidRequest {
		t.Errorf("Error-Cause: got %d, want %d", got, radius.ErrorCauseInvalidRequest)
	}
}

func TestDisconnectReplayReturnsCachedResponse(t *testing.T) {
	secret := []byte("test-dm-replay-secret")
	fake := &fakeL2TPService{snap: l2tp.Snapshot{
		Tunnels: []l2tp.TunnelSnapshot{{
			LocalTID: 10,
			Sessions: []l2tp.SessionSnapshot{{
				LocalSID:       20,
				TunnelLocalTID: 10,
				Username:       "alice",
			}},
		}},
	}}
	l2tp.PublishService(fake)
	defer l2tp.PublishService(nil)

	cl, err := newCoAListener(0, nil, secret, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()
	addr := cl.conn.LocalAddr().String()

	wire := buildCoAPacket(t, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")},
	}, time.Now())
	resp := sendRawCoAPacket(t, addr, wire)
	if resp.Code != radius.CodeDisconnectACK {
		t.Fatalf("first code: got %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}
	resp = sendRawCoAPacket(t, addr, wire)
	if resp.Code != radius.CodeDisconnectACK {
		t.Fatalf("replay code: got %d, want %d (Disconnect-ACK)", resp.Code, radius.CodeDisconnectACK)
	}
	if got := fake.teardowns.Load(); got != 1 {
		t.Fatalf("teardowns after replay: got %d, want 1", got)
	}
}

// VALIDATES: AC-7 -- DM for unknown session returns Disconnect-NAK.
func TestDisconnectListenerUnknownSession(t *testing.T) {
	secret := []byte("test-dm-secret")
	cl, err := newCoAListener(0, nil, secret, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cl.Close(); err != nil {
			t.Logf("close: %v", err)
		}
	}()
	addr := cl.conn.LocalAddr().String()

	resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString("999-999")},
	})

	if resp.Code != radius.CodeDisconnectNAK {
		t.Errorf("code: got %d, want %d (Disconnect-NAK)", resp.Code, radius.CodeDisconnectNAK)
	}
}

func TestExtractRate(t *testing.T) {
	tests := []struct {
		filterID string
		want     uint64
	}{
		{"10mbit", 10_000_000},
		{"100kbit", 100_000},
		{"1gbit", 1_000_000_000},
		{"5mbps", 40_000_000},
	}
	for _, tt := range tests {
		pkt := &radius.Packet{
			Attrs: []radius.Attr{
				{Type: radius.AttrFilterID, Value: radius.AttrString(tt.filterID)},
			},
		}
		got := extractRate(pkt)
		if got != tt.want {
			t.Errorf("extractRate(Filter-Id=%q) = %d, want %d", tt.filterID, got, tt.want)
		}
	}
}

func TestExtractRate_Invalid(t *testing.T) {
	pkt := &radius.Packet{
		Attrs: []radius.Attr{
			{Type: radius.AttrFilterID, Value: radius.AttrString("notarate")},
		},
	}
	if got := extractRate(pkt); got != 0 {
		t.Errorf("extractRate with invalid filter-id: got %d, want 0", got)
	}
}
