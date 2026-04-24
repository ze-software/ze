package l2tpauthradius

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/radius"
)

func sendCoAPacket(t *testing.T, addr string, code uint8, secret []byte, attrs []radius.Attr) *radius.Packet {
	t.Helper()

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
