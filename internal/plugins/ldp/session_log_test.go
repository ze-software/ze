// Design: docs/architecture/ldp/mpls-ldp.md -- LDP session FSM
// Related: iface_backend_test.go -- recordingHandler, the slog handler this test reads.
package ldp

import (
	"log/slog"
	"net/netip"
	"testing"
)

// VALIDATES: a session that reaches OPERATIONAL logs `ldp: session operational`
// at INFO with the peer's address, so an operator running the LDP log at info
// (docs/guide/mpls.md) sees when each session came up. Method: drive
// processMessages from OPENSENT with the peer's Initialization and KeepAlive in
// one PDU, as TestSessionProcessMessagesFiresOperational does, into a session
// whose logger keeps every record, then read the record.
// PREVENTS: the session-up line logged below the level operators read, which
// leaves a log that shows sessions ending but never coming up.
//
// MUTATION: with the call in processMessages back to s.log.Debug, the record's
// level is DEBUG and the test fails on "level = DEBUG, want INFO".
func TestSessionOperationalLoggedAtInfo(t *testing.T) {
	handler := &recordingHandler{}
	peer := netip.MustParseAddr("10.0.0.2")
	s := &Session{
		state:         StateOpenSent,
		peerAddr:      peer,
		peerLSRID:     [4]byte{10, 0, 0, 2},
		keepaliveTime: DefaultKeepaliveTime,
		maxPDU:        DefaultMaxPDULength,
		lib:           newLIB(),
		log:           slog.New(handler),
		stopCh:        make(chan struct{}),
	}

	var buf [256]byte
	n := EncodeInit(buf[:], initMessage{
		MessageID:       1,
		ProtocolVersion: ldpVersion,
		KeepaliveTime:   30,
		MaxPDULength:    4096,
	})
	if err := s.processMessages(withPeerKeepAlive(buf[:n]), [4]byte{10, 0, 0, 2}, 0, nil, nil, func() {}); err != nil {
		t.Fatalf("processMessages: %v", err)
	}
	if s.State() != StateOperational {
		t.Fatalf("state = %s, want operational", s.State())
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	found := 0
	for i := range handler.records {
		record := handler.records[i]
		if record.Message != "ldp: session operational" {
			continue
		}
		found++
		if record.Level != slog.LevelInfo {
			t.Errorf("level = %s, want INFO", record.Level)
		}
		logged := ""
		record.Attrs(func(a slog.Attr) bool {
			if a.Key != "peer" {
				return true
			}
			logged = a.Value.String()
			return false
		})
		if logged != peer.String() {
			t.Errorf("peer = %q, want %q", logged, peer)
		}
	}
	if found != 1 {
		t.Fatalf("logged %q %d times, want once", "ldp: session operational", found)
	}
}
