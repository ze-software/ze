package bmp

import (
	"encoding/json"
	"testing"
)

func marshalAny(v any) []byte {
	if raw, ok := v.(json.RawMessage); ok {
		return []byte(raw)
	}
	b, _ := json.Marshal(v)
	return b
}

func TestStateAddRemoveRouter(t *testing.T) {
	// VALIDATES: AC-13 -- router identification captured and queryable
	s := newBMPState()

	s.addRouter("10.0.0.1:12345")
	s.setRouterInfo("10.0.0.1:12345", "router1", "ze test", nil)

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
	if err := json.Unmarshal(marshalAny(data), &result); err != nil {
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
	if err := json.Unmarshal(marshalAny(data), &result); err != nil {
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
	s.peerUp("10.0.0.1:12345", peer, nil, nil)

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
	if err := json.Unmarshal(marshalAny(data), &result); err != nil {
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
	if err := json.Unmarshal(marshalAny(data), &result); err != nil {
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
	s.peerUp("10.0.0.1:12345", testPeerHeader(), nil, nil)

	s.removeRouter("10.0.0.1:12345")

	_, data, _ := s.peersCommand()
	var result struct {
		Peers []monitoredPeer `json:"peers"`
	}
	if err := json.Unmarshal(marshalAny(data), &result); err != nil {
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
	if err := json.Unmarshal(marshalAny(data), &result); err != nil {
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

// Loc-RIB peers share a zero address, so a delayed Peer Down must target the
// original BGP ID without replacing or withdrawing the new instance's report.
func TestStateLocRIBIdentityChangeKeepsReportsSeparate(t *testing.T) {
	bp := &BMPPlugin{state: newBMPState()}
	const remote = "192.0.2.8:12345"
	first := PeerHeader{PeerType: PeerTypeLocRIB, PeerAS: 65000, PeerBGPID: 0xc0000201}
	second := first
	second.PeerBGPID = 0xc0000202
	for _, peer := range []PeerHeader{first, second} {
		open := fabricateLocRIBOpen(localIdentity{asn: peer.PeerAS, routerID: peer.PeerBGPID})
		bp.processPeerUp(remote, &PeerUp{Peer: peer, SentOpenMsg: open, ReceivedOpenMsg: open})
	}
	bp.processPeerDown(remote, &PeerDown{Peer: first, Reason: PeerDownTLVData})
	_, data, err := bp.handleCommand("show bmp peers")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Peers []monitoredPeer `json:"peers"`
	}
	if err := json.Unmarshal(marshalAny(data), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Peers) != 2 {
		t.Fatalf("reported %d Loc-RIB identities, want both old and new", len(result.Peers))
	}
	for _, peer := range result.Peers {
		switch peer.PeerBGPID {
		case "192.0.2.1":
			if peer.IsUp {
				t.Fatal("old Loc-RIB remains up after its Peer Down")
			}
		case "192.0.2.2":
			if !peer.IsUp {
				t.Fatal("old Loc-RIB Peer Down withdrew the new identity")
			}
		default:
			t.Fatalf("unexpected Loc-RIB identity %q", peer.PeerBGPID)
		}
	}
}
