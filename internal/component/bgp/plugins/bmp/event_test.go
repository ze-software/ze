package bmp

import (
	"bytes"
	"net"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
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
	// VALIDATES: AC-1 -- peer up with real cached OPENs from OPEN events

	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	sentOpen := makeBGPOpen(65000, 0x0A000064)
	recvOpen := makeBGPOpen(65001, 0x0A000001)

	bp := &BMPPlugin{
		state:     newBMPState(),
		openCache: make(map[string]*openPair),
		stopCh:    make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}

	// Pre-populate the OPEN cache (as OPEN events would).
	bp.openCache["10.0.0.1"] = &openPair{sent: sentOpen, received: recvOpen}

	se := &rpc.StructuredEvent{
		PeerAddress:  "10.0.0.1",
		PeerAS:       65001,
		LocalAS:      65000,
		LocalAddress: "10.0.0.100",
		EventType:    rpc.EventKindState,
		State:        rpc.SessionStateUp,
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
	if !bytes.Equal(pu.SentOpenMsg, sentOpen) {
		t.Error("sent OPEN should be the cached real OPEN, not synthetic")
	}
	if !bytes.Equal(pu.ReceivedOpenMsg, recvOpen) {
		t.Error("received OPEN should be the cached real OPEN, not synthetic")
	}
}

func TestBMPOpenCaching(t *testing.T) {
	// VALIDATES: AC-1 -- OPEN events are cached per peer and used in Peer Up

	bp := &BMPPlugin{
		state:     newBMPState(),
		openCache: make(map[string]*openPair),
		stopCh:    make(chan struct{}),
	}

	openBody := []byte{4, 0xFF, 0xE9, 0, 90, 0x0A, 0, 0, 1, 0}

	// Simulate sent OPEN event.
	seSent := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		EventType:   rpc.EventKindOpen,
		Direction:   rpc.DirectionSent,
		RawMessage:  &bgptypes.RawMessage{Type: message.TypeOPEN, RawBytes: openBody},
	}
	bp.cacheOpenPDU(seSent)

	// Simulate received OPEN event.
	seRecv := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		EventType:   rpc.EventKindOpen,
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{Type: message.TypeOPEN, RawBytes: openBody},
	}
	bp.cacheOpenPDU(seRecv)

	pair := bp.openCache["10.0.0.1"]
	if pair == nil {
		t.Fatal("OPEN cache should have entry for 10.0.0.1")
	}
	if pair.sent == nil {
		t.Error("sent OPEN should be cached")
	}
	if pair.received == nil {
		t.Error("received OPEN should be cached")
	}
	// Cached PDU should be full BGP message: 19-byte header + body.
	wantLen := message.HeaderLen + len(openBody)
	if len(pair.sent) != wantLen {
		t.Errorf("cached sent OPEN length = %d, want %d", len(pair.sent), wantLen)
	}
}

func TestBMPOpenCacheClearedOnPeerDown(t *testing.T) {
	// VALIDATES: AC-9 (partial) -- peer down clears OPEN cache

	bp := &BMPPlugin{
		state:     newBMPState(),
		openCache: make(map[string]*openPair),
		stopCh:    make(chan struct{}),
	}

	bp.openCache["10.0.0.1"] = &openPair{
		sent:     makeBGPOpen(65000, 0),
		received: makeBGPOpen(65001, 0),
	}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindState,
		State:       rpc.SessionStateDown,
		Reason:      "notification",
	}

	bp.handleStructuredEvent(se)

	if _, ok := bp.openCache["10.0.0.1"]; ok {
		t.Error("OPEN cache should be cleared after peer down")
	}
}

func TestBMPPeerUpSkippedOnCacheMiss(t *testing.T) {
	// VALIDATES: AC-3 edge case -- no cached OPENs -> Peer Up skipped (not crash)

	bp := &BMPPlugin{
		state:     newBMPState(),
		openCache: make(map[string]*openPair),
		stopCh:    make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   nil, // no connection, shouldn't reach write
			stopCh: make(chan struct{}),
		}},
	}

	se := &rpc.StructuredEvent{
		PeerAddress:  "10.0.0.1",
		PeerAS:       65001,
		LocalAS:      65000,
		LocalAddress: "10.0.0.100",
		EventType:    rpc.EventKindState,
		State:        rpc.SessionStateUp,
	}

	// Should not panic -- just logs warning and returns.
	bp.handleStructuredEvent(se)
}

func TestHandleSenderStatePeerDown(t *testing.T) {
	// VALIDATES: AC-27 -- peer down event triggers BMP Peer Down to collectors

	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		state:     newBMPState(),
		openCache: make(map[string]*openPair),
		stopCh:    make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindState,
		State:       rpc.SessionStateDown,
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
		EventType:   rpc.EventKindState,
		State:       rpc.SessionStateUp,
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

func TestBMPRouteMirroringSend(t *testing.T) {
	// VALIDATES: AC-4, AC-5, AC-6 -- Route Mirroring for UPDATE, NOTIFICATION, KEEPALIVE

	cases := []struct {
		name    string
		kind    rpc.EventKind
		msgType message.MessageType
		body    []byte
	}{
		{"UPDATE", rpc.EventKindUpdate, message.TypeUPDATE, []byte{0x00, 0x00, 0x00, 0x00, 0x00}},
		{"NOTIFICATION", rpc.EventKindNotification, message.TypeNOTIFICATION, []byte{0x06, 0x04}},
		{"KEEPALIVE", rpc.EventKindKeepalive, message.TypeKEEPALIVE, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, client := net.Pipe()
			defer closeLog(server, "server")
			defer closeLog(client, "client")

			bp := &BMPPlugin{
				state:          newBMPState(),
				openCache:      make(map[string]*openPair),
				dedupState:     make(map[string]map[uint64]struct{}),
				routeMirroring: true,
				stopCh:         make(chan struct{}),
				senders: []*senderSession{{
					name:   "test",
					conn:   client,
					stopCh: make(chan struct{}),
				}},
			}

			se := &rpc.StructuredEvent{
				PeerAddress: "10.0.0.1",
				PeerAS:      65001,
				EventType:   tc.kind,
				Direction:   rpc.DirectionReceived,
				RawMessage:  &bgptypes.RawMessage{Type: tc.msgType, RawBytes: tc.body},
			}

			// Run event handler in goroutine -- net.Pipe writes block
			// until each message is read.
			go bp.handleStructuredEvent(se)

			// For UPDATE, Route Monitoring arrives before Route Mirroring.
			if tc.kind == rpc.EventKindUpdate {
				rm, err := readBMPFromPipe(server)
				if err != nil {
					t.Fatalf("Route Monitoring read: %v", err)
				}
				if _, ok := rm.(*RouteMonitoring); !ok {
					t.Fatalf("expected Route Monitoring first, got %T", rm)
				}
			}

			msg, err := readBMPFromPipe(server)
			if err != nil {
				t.Fatalf("Route Mirroring read: %v", err)
			}
			rm, ok := msg.(*RouteMirroring)
			if !ok {
				t.Fatalf("expected *RouteMirroring, got %T", msg)
			}
			if len(rm.TLVs) != 1 {
				t.Fatalf("TLV count = %d, want 1", len(rm.TLVs))
			}
			if rm.TLVs[0].Type != MirrorTLVBGPMsg {
				t.Errorf("TLV type = %d, want %d (BGP Message)", rm.TLVs[0].Type, MirrorTLVBGPMsg)
			}
		})
	}
}

func TestBMPRouteMirroringConfig(t *testing.T) {
	// VALIDATES: AC-12 -- Route Mirroring disabled means no mirror messages

	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		state:          newBMPState(),
		openCache:      make(map[string]*openPair),
		dedupState:     make(map[string]map[uint64]struct{}),
		routeMirroring: false,
		stopCh:         make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindNotification,
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{Type: message.TypeNOTIFICATION, RawBytes: []byte{0x06, 0x04}},
	}

	bp.handleStructuredEvent(se)

	// With mirroring disabled, NOTIFICATION should produce no output.
	// Verify by sending an UPDATE (which always produces Route Monitoring)
	// and confirming that's the first thing read.
	updateSe := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindUpdate,
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{Type: message.TypeUPDATE, RawBytes: []byte{0x00, 0x00, 0x00, 0x00, 0x00}},
	}

	result := asyncRead(server)
	bp.handleStructuredEvent(updateSe)
	r := <-result
	if r.err != nil {
		t.Fatalf("read: %v", r.err)
	}
	if _, ok := r.msg.(*RouteMonitoring); !ok {
		t.Fatalf("expected Route Monitoring (no mirror), got %T", r.msg)
	}
}

func TestBMPRiboutDedupSameUpdate(t *testing.T) {
	// VALIDATES: AC-7 -- same UPDATE forwarded twice, second is suppressed

	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		state:      newBMPState(),
		openCache:  make(map[string]*openPair),
		dedupState: make(map[string]map[uint64]struct{}),
		stopCh:     make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}

	updateBody := []byte{0x00, 0x00, 0x00, 0x04, 0x40, 0x01, 0x01, 0x00, 0x18, 0x0A, 0x00, 0x01}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindUpdate,
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{Type: message.TypeUPDATE, RawBytes: updateBody},
	}

	// First UPDATE: should be forwarded.
	result := asyncRead(server)
	bp.handleStructuredEvent(se)
	r := <-result
	if r.err != nil {
		t.Fatalf("first UPDATE read: %v", r.err)
	}
	if _, ok := r.msg.(*RouteMonitoring); !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", r.msg)
	}

	// Second identical UPDATE: should be suppressed (no data on pipe).
	// Write a marker event after to prove the duplicate was skipped.
	bp.handleStructuredEvent(se)

	// Send a different UPDATE to flush -- if dedup works, only this one arrives.
	differentBody := []byte{0x00, 0x00, 0x00, 0x04, 0x40, 0x01, 0x01, 0x02, 0x18, 0x0A, 0x00, 0x02}
	se2 := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindUpdate,
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{Type: message.TypeUPDATE, RawBytes: differentBody},
	}
	result = asyncRead(server)
	bp.handleStructuredEvent(se2)
	r = <-result
	if r.err != nil {
		t.Fatalf("marker UPDATE read: %v", r.err)
	}
}

func TestBMPRiboutDedupDifferentAttrs(t *testing.T) {
	// VALIDATES: AC-8 -- different UPDATE for same prefix is forwarded

	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		state:      newBMPState(),
		openCache:  make(map[string]*openPair),
		dedupState: make(map[string]map[uint64]struct{}),
		stopCh:     make(chan struct{}),
		senders: []*senderSession{{
			name:   "test",
			conn:   client,
			stopCh: make(chan struct{}),
		}},
	}

	body1 := []byte{0x00, 0x00, 0x00, 0x04, 0x40, 0x01, 0x01, 0x00, 0x18, 0x0A, 0x00, 0x01}
	body2 := []byte{0x00, 0x00, 0x00, 0x04, 0x40, 0x01, 0x01, 0x02, 0x18, 0x0A, 0x00, 0x01}

	se1 := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindUpdate,
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{Type: message.TypeUPDATE, RawBytes: body1},
	}
	se2 := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindUpdate,
		Direction:   rpc.DirectionReceived,
		RawMessage:  &bgptypes.RawMessage{Type: message.TypeUPDATE, RawBytes: body2},
	}

	// First UPDATE.
	result := asyncRead(server)
	bp.handleStructuredEvent(se1)
	<-result

	// Second UPDATE with different attributes: should be forwarded.
	result = asyncRead(server)
	bp.handleStructuredEvent(se2)
	r := <-result
	if r.err != nil {
		t.Fatalf("second UPDATE read: %v", r.err)
	}
	if _, ok := r.msg.(*RouteMonitoring); !ok {
		t.Fatalf("expected *RouteMonitoring, got %T", r.msg)
	}
}

func TestBMPRiboutDedupPeerDown(t *testing.T) {
	// VALIDATES: AC-9 -- peer down clears dedup state

	bp := &BMPPlugin{
		state:      newBMPState(),
		openCache:  make(map[string]*openPair),
		dedupState: make(map[string]map[uint64]struct{}),
		stopCh:     make(chan struct{}),
	}

	// Pre-populate dedup state.
	bp.dedupState["10.0.0.1"] = map[uint64]struct{}{12345: {}}

	se := &rpc.StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   rpc.EventKindState,
		State:       rpc.SessionStateDown,
		Reason:      "notification",
	}

	bp.handleStructuredEvent(se)

	bp.mu.RLock()
	_, ok := bp.dedupState["10.0.0.1"]
	bp.mu.RUnlock()
	if ok {
		t.Error("dedup state should be cleared after peer down")
	}
}
