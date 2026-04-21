package bmp

import (
	"net"
	"testing"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

func TestPeerHeaderFromEvent(t *testing.T) {
	// VALIDATES: sender builds correct PeerHeader from structured event
	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		LocalAS:     65000,
	}

	ph := peerHeaderFromEvent(se)
	if ph.PeerAS != 65001 {
		t.Errorf("PeerAS = %d, want 65001", ph.PeerAS)
	}
	if ph.PeerType != PeerTypeGlobal {
		t.Errorf("PeerType = %d, want %d", ph.PeerType, PeerTypeGlobal)
	}
	if ph.Flags&PeerFlagV != 0 {
		t.Error("IPv4 address should not have V flag")
	}
	// Verify IPv4-mapped address.
	if ph.Address[10] != 0xff || ph.Address[11] != 0xff {
		t.Errorf("IPv4-mapped prefix missing: got %x", ph.Address[10:12])
	}
	if ph.Address[12] != 10 || ph.Address[15] != 1 {
		t.Errorf("IPv4 address wrong: got %v", ph.Address[12:16])
	}
}

func TestPeerHeaderFromEventIPv6(t *testing.T) {
	se := &rpc.StructuredEvent{
		PeerAddress: "2001:db8::1",
		PeerAS:      65002,
	}

	ph := peerHeaderFromEvent(se)
	if ph.Flags&PeerFlagV == 0 {
		t.Error("IPv6 address should have V flag")
	}
	if ph.PeerAS != 65002 {
		t.Errorf("PeerAS = %d, want 65002", ph.PeerAS)
	}
}

func TestPeerHeaderFromEventAdjRIBOut(t *testing.T) {
	// VALIDATES: RFC 8671 -- sent direction sets O flag (Adj-RIB-Out) and L flag (post-policy)
	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		Direction:   rpc.DirectionSent,
	}

	ph := peerHeaderFromEvent(se)
	if ph.Flags&PeerFlagO == 0 {
		t.Error("sent direction should set O flag (Adj-RIB-Out)")
	}
	if ph.Flags&PeerFlagL == 0 {
		t.Error("sent direction should set L flag (post-policy)")
	}
}

func TestPeerHeaderFromEventAdjRIBIn(t *testing.T) {
	// VALIDATES: received direction does NOT set O or L flags (pre-policy Adj-RIB-In)
	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		Direction:   rpc.DirectionReceived,
	}

	ph := peerHeaderFromEvent(se)
	if ph.Flags&PeerFlagO != 0 {
		t.Error("received direction should NOT set O flag")
	}
	if ph.Flags&PeerFlagL != 0 {
		t.Error("received direction should NOT set L flag")
	}
}

func TestPeerDownReasonMapping(t *testing.T) {
	tests := []struct {
		reason string
		want   uint8
	}{
		{"notification", PeerDownLocalNotify},
		{"tcp-failure", PeerDownLocalNoNotify},
		{"timer-expired", PeerDownLocalNoNotify},
		{"remote-notification", PeerDownRemoteNotify},
		{"remote-close", PeerDownRemoteNoData},
		{"config-changed", PeerDownDeconfigured},
		{"deconfigured", PeerDownDeconfigured},
		{"unknown", PeerDownLocalNoNotify},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got := peerDownReasonFromString(tt.reason)
			if got != tt.want {
				t.Errorf("peerDownReasonFromString(%q) = %d, want %d", tt.reason, got, tt.want)
			}
		})
	}
}

func TestHandleSenderStatePeerUp(t *testing.T) {
	// VALIDATES: AC-26 -- peer up event triggers BMP Peer Up to collectors

	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}

	se := &rpc.StructuredEvent{
		PeerAddress:  "10.0.0.1",
		PeerAS:       65001,
		LocalAS:      65000,
		LocalAddress: "10.0.0.100",
		EventType:    "state",
		State:        "up",
	}

	result := asyncRead(server)

	bp.handleStructuredEvent(se)

	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	pu, ok := r.msg.(*PeerUp)
	if !ok {
		t.Fatalf("expected *PeerUp, got %T", r.msg)
	}
	if pu.Peer.PeerAS != 65001 {
		t.Errorf("PeerAS = %d, want 65001", pu.Peer.PeerAS)
	}
}

func TestHandleSenderStatePeerDown(t *testing.T) {
	// VALIDATES: AC-27 -- peer down event triggers BMP Peer Down to collectors

	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   "state",
		State:       "down",
		Reason:      "notification",
	}

	result := asyncRead(server)

	bp.handleStructuredEvent(se)

	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	pd, ok := r.msg.(*PeerDown)
	if !ok {
		t.Fatalf("expected *PeerDown, got %T", r.msg)
	}
	if pd.Reason != PeerDownLocalNotify {
		t.Errorf("reason = %d, want %d", pd.Reason, PeerDownLocalNotify)
	}
}

func TestHandleSenderNoSenders(t *testing.T) {
	// VALIDATES: no panic when no senders configured
	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   "state",
		State:       "up",
	}

	// Should not panic.
	bp.handleStructuredEvent(se)
}

func TestParseIPIntoIPv4(t *testing.T) {
	var addr [16]byte
	parseIPInto("192.168.1.1", &addr)

	// Should be IPv4-mapped: ::ffff:192.168.1.1
	if addr[10] != 0xff || addr[11] != 0xff {
		t.Errorf("missing IPv4-mapped prefix: %x", addr[10:12])
	}
	if addr[12] != 192 || addr[13] != 168 || addr[14] != 1 || addr[15] != 1 {
		t.Errorf("wrong IP: %v", addr[12:16])
	}
}

func TestParseIPIntoIPv6(t *testing.T) {
	var addr [16]byte
	parseIPInto("2001:db8::1", &addr)

	if addr[0] != 0x20 || addr[1] != 0x01 {
		t.Errorf("wrong IPv6 prefix: %x", addr[0:2])
	}
	if addr[15] != 1 {
		t.Errorf("wrong IPv6 last byte: %d", addr[15])
	}
}

func TestParseIPIntoInvalid(t *testing.T) {
	var addr [16]byte
	parseIPInto("not-an-ip", &addr)

	// Should be all zeros.
	for i, b := range addr {
		if b != 0 {
			t.Errorf("addr[%d] = %d, want 0 for invalid input", i, b)
		}
	}
}
