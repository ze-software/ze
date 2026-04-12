package bmp

import (
	"encoding/json"
	"testing"
)

func TestStateAddRemoveRouter(t *testing.T) {
	// VALIDATES: AC-13 -- router identification captured and queryable
	s := newBMPState()

	s.addRouter("10.0.0.1:12345")
	s.setRouterInfo("10.0.0.1:12345", "router1", "ze test")

	status, data, err := s.sessionsCommand()
	if err != nil {
		t.Fatalf("sessionsCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want %q", status, statusDone)
	}

	var result struct {
		Sessions []monitoredRouter `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Sessions) != 1 {
		t.Fatalf("sessions count = %d, want 1", len(result.Sessions))
	}
	if result.Sessions[0].SysName != "router1" {
		t.Errorf("sysName = %q, want %q", result.Sessions[0].SysName, "router1")
	}

	// Remove and verify empty.
	s.removeRouter("10.0.0.1:12345")
	_, data, _ = s.sessionsCommand()
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Sessions) != 0 {
		t.Errorf("sessions count = %d, want 0 after remove", len(result.Sessions))
	}
}

func TestStatePeerUpDown(t *testing.T) {
	// VALIDATES: AC-14 -- peer appears in query after Peer Up
	// VALIDATES: AC-16 -- peer marked down after Peer Down
	s := newBMPState()
	s.addRouter("10.0.0.1:12345")

	peer := testPeerHeader()
	s.peerUp("10.0.0.1:12345", peer)

	status, data, err := s.peersCommand()
	if err != nil {
		t.Fatalf("peersCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want %q", status, statusDone)
	}

	var result struct {
		Peers []monitoredPeer `json:"peers"`
	}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Peers) != 1 {
		t.Fatalf("peers count = %d, want 1", len(result.Peers))
	}
	if result.Peers[0].PeerAS != 65001 {
		t.Errorf("peer AS = %d, want 65001", result.Peers[0].PeerAS)
	}
	if !result.Peers[0].IsUp {
		t.Error("peer should be up")
	}

	// Peer down.
	s.peerDown("10.0.0.1:12345", peer, PeerDownDeconfigured)
	_, data, _ = s.peersCommand()
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Peers[0].IsUp {
		t.Error("peer should be down")
	}
	if result.Peers[0].Reason != PeerDownDeconfigured {
		t.Errorf("reason = %d, want %d", result.Peers[0].Reason, PeerDownDeconfigured)
	}
}

func TestStateRemoveRouterClearsPeers(t *testing.T) {
	// VALIDATES: AC-16 -- removing router drops its peers
	s := newBMPState()
	s.addRouter("10.0.0.1:12345")
	s.peerUp("10.0.0.1:12345", testPeerHeader())

	s.removeRouter("10.0.0.1:12345")

	_, data, _ := s.peersCommand()
	var result struct {
		Peers []monitoredPeer `json:"peers"`
	}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Peers) != 0 {
		t.Errorf("peers count = %d, want 0 after router remove", len(result.Peers))
	}
}

func TestStateCollectorsCommand(t *testing.T) {
	// VALIDATES: AC-30 -- collector status queryable
	s := newBMPState()

	ss := &senderSession{
		name:    "col1",
		address: "10.0.0.2",
		port:    11019,
		stopCh:  make(chan struct{}),
	}

	status, data, err := s.collectorsCommand([]*senderSession{ss})
	if err != nil {
		t.Fatalf("collectorsCommand: %v", err)
	}
	if status != statusDone {
		t.Errorf("status = %q, want %q", status, statusDone)
	}

	var result struct {
		Collectors []collectorStatus `json:"collectors"`
	}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Collectors) != 1 {
		t.Fatalf("collectors count = %d, want 1", len(result.Collectors))
	}
	if result.Collectors[0].Name != "col1" {
		t.Errorf("name = %q, want %q", result.Collectors[0].Name, "col1")
	}
	if result.Collectors[0].Connected {
		t.Error("should not be connected (conn is nil)")
	}
}

func TestHandleCommandUnknown(t *testing.T) {
	bp := &BMPPlugin{
		state:  newBMPState(),
		stopCh: make(chan struct{}),
	}
	status, _, err := bp.handleCommand("bmp unknown")
	if status != statusError {
		t.Errorf("status = %q, want %q", status, statusError)
	}
	if err == nil {
		t.Error("expected error for unknown command")
	}
}
