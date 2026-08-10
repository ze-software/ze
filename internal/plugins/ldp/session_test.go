package ldp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: AC-4 -- Address messages accumulate the peer's interface addresses
// (deduped) and Address Withdraw removes them.
func TestSessionPeerAddresses(t *testing.T) {
	s := &Session{stopCh: make(chan struct{})}
	a := netip.MustParseAddr("10.0.0.1")
	b := netip.MustParseAddr("10.0.0.2")

	s.addPeerAddresses([]netip.Addr{a, b, a}) // duplicate a
	if got := s.peerAddresses(); len(got) != 2 {
		t.Fatalf("PeerAddresses len = %d, want 2 (deduped)", len(got))
	}

	s.removePeerAddresses([]netip.Addr{a})
	got := s.peerAddresses()
	if len(got) != 1 || got[0] != b {
		t.Errorf("after remove = %v, want [%s]", got, b)
	}
}

func TestSessionStateString(t *testing.T) {
	tests := []struct {
		state SessionState
		want  string
	}{
		{StateNonExistent, "non-existent"},
		{StateInitialized, "initialized"},
		{StateOpenReceived, "open-received"},
		{StateOpenSent, "open-sent"},
		{StateOperational, "operational"},
		{SessionState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("SessionState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestSessionHandleInit(t *testing.T) {
	lib := newLIB()
	s := &Session{
		state:         StateOpenSent,
		keepaliveTime: DefaultKeepaliveTime,
		maxPDU:        DefaultMaxPDULength,
		lib:           lib,
		stopCh:        make(chan struct{}),
	}

	peerLSRID := [4]byte{10, 0, 0, 2}
	msg := initMessage{
		MessageID:       1,
		ProtocolVersion: ldpVersion,
		KeepaliveTime:   30,
		MaxPDULength:    4096,
	}

	if !s.handleInit(msg, peerLSRID) {
		t.Error("handleInit should report operational transition from open-sent")
	}

	if s.state != StateOperational {
		t.Errorf("state = %s, want operational", s.state)
	}
	if s.peerLSRID != peerLSRID {
		t.Errorf("peerLSRID mismatch")
	}
	if s.keepaliveTime.Seconds() != 30 {
		t.Errorf("keepaliveTime = %v, want 30s", s.keepaliveTime)
	}
}

func TestSessionHandleInitFromInitialized(t *testing.T) {
	lib := newLIB()
	s := &Session{
		state:         StateInitialized,
		keepaliveTime: DefaultKeepaliveTime,
		maxPDU:        DefaultMaxPDULength,
		lib:           lib,
		stopCh:        make(chan struct{}),
	}

	msg := initMessage{
		MessageID:       1,
		ProtocolVersion: ldpVersion,
		KeepaliveTime:   45,
		MaxPDULength:    2048,
	}

	if s.handleInit(msg, [4]byte{10, 0, 0, 3}) {
		t.Error("handleInit from initialized should not report operational transition")
	}

	if s.state != StateOpenReceived {
		t.Errorf("state = %s, want open-received", s.state)
	}
	if s.maxPDU != 2048 {
		t.Errorf("maxPDU = %d, want 2048", s.maxPDU)
	}
}

// VALIDATES: AC-3 -- when an Initialization message drives the session to
// operational, processMessages fires onOperational exactly once so the engine
// advertises its local label mappings.
func TestSessionProcessMessagesFiresOperational(t *testing.T) {
	s := &Session{
		state:         StateOpenSent,
		keepaliveTime: DefaultKeepaliveTime,
		maxPDU:        DefaultMaxPDULength,
		lib:           newLIB(),
		log:           slogutil.DiscardLogger(),
		stopCh:        make(chan struct{}),
	}

	var buf [256]byte
	n := EncodeInit(buf[:], initMessage{
		MessageID:       1,
		ProtocolVersion: ldpVersion,
		KeepaliveTime:   30,
		MaxPDULength:    4096,
	})

	fired := 0
	if err := s.processMessages(buf[:n], [4]byte{10, 0, 0, 2}, nil, nil, func() { fired++ }); err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if fired != 1 {
		t.Errorf("onOperational fired %d times, want 1", fired)
	}
	if s.State() != StateOperational {
		t.Errorf("state = %s, want operational", s.State())
	}
}

// VALIDATES: the keepalive sender reads the negotiated interval (not the
// pre-negotiation default) under lock, so it paces correctly after Init exchange.
func TestSessionCurrentKeepalive(t *testing.T) {
	s := &Session{
		state:         StateOpenSent,
		keepaliveTime: DefaultKeepaliveTime,
		maxPDU:        DefaultMaxPDULength,
		lib:           newLIB(),
		stopCh:        make(chan struct{}),
	}
	if s.currentKeepalive() != DefaultKeepaliveTime {
		t.Fatalf("initial keepalive = %v, want %v", s.currentKeepalive(), DefaultKeepaliveTime)
	}

	s.handleInit(initMessage{KeepaliveTime: 20}, [4]byte{10, 0, 0, 2})

	if s.currentKeepalive().Seconds() != 20 {
		t.Errorf("keepalive after negotiation = %v, want 20s", s.currentKeepalive())
	}
}

func TestSessionKeepaliveNegotiation(t *testing.T) {
	lib := newLIB()
	s := &Session{
		state:         StateOpenSent,
		keepaliveTime: 60 * 1e9,
		maxPDU:        DefaultMaxPDULength,
		lib:           lib,
		stopCh:        make(chan struct{}),
	}

	msg := initMessage{
		KeepaliveTime: 20,
	}
	s.handleInit(msg, [4]byte{10, 0, 0, 4})

	if s.keepaliveTime.Seconds() != 20 {
		t.Errorf("keepaliveTime = %v, want 20s (should pick lower)", s.keepaliveTime)
	}
}
