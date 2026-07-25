package report

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/mrt"
)

func testMRTLogConfig() MRTLogConfig {
	return MRTLogConfig{
		LocalAS:   65000,
		LocalAddr: netip.MustParseAddr("127.0.0.1"),
		Peers: []MRTPeer{
			{ASN: 64512, Addr: netip.MustParseAddr("10.255.0.1")},
			{ASN: 64513, Addr: netip.MustParseAddr("10.255.0.2")},
			{ASN: 65000, Addr: netip.MustParseAddr("10.255.0.3")},
		},
	}
}

func TestMRTLogStateChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")

	m := NewMRTLog(path, testMRTLogConfig())

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	// Establish peer 0, then disconnect it
	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(5 * time.Second)})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var records []stateRecord
	handler := &mrt.Handler{
		OnStateChange: func(h mrt.Header, _ uint32, s *mrt.StateChangeRecord) error {
			records = append(records, stateRecord{header: h, sc: s})
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	// AC-1: Established event -> OldState=Idle, NewState=Established
	r := records[0]
	if r.header.Type != mrt.TypeBGP4MP || r.header.Subtype != mrt.BGP4MPStateChangeAS4 {
		t.Errorf("record 0: type=%d subtype=%d, want %d/%d", r.header.Type, r.header.Subtype, mrt.TypeBGP4MP, mrt.BGP4MPStateChangeAS4)
	}
	if r.sc.OldState != mrt.FSMIdle || r.sc.NewState != mrt.FSMEstablished {
		t.Errorf("record 0: old=%d new=%d, want %d/%d", r.sc.OldState, r.sc.NewState, mrt.FSMIdle, mrt.FSMEstablished)
	}
	if r.sc.PeerAS != 64512 || r.sc.LocalAS != 65000 {
		t.Errorf("record 0: peerAS=%d localAS=%d, want 64512/65000", r.sc.PeerAS, r.sc.LocalAS)
	}
	if r.header.Timestamp != uint32(now.Unix()) {
		t.Errorf("record 0: timestamp=%d, want %d", r.header.Timestamp, now.Unix())
	}

	// AC-2: Disconnected event -> OldState=Established, NewState=Idle
	r = records[1]
	if r.sc.OldState != mrt.FSMEstablished || r.sc.NewState != mrt.FSMIdle {
		t.Errorf("record 1: old=%d new=%d, want %d/%d", r.sc.OldState, r.sc.NewState, mrt.FSMEstablished, mrt.FSMIdle)
	}
	if r.sc.PeerAS != 64512 {
		t.Errorf("record 1: peerAS=%d, want 64512", r.sc.PeerAS)
	}
	if r.header.Timestamp != uint32(now.Add(5*time.Second).Unix()) {
		t.Errorf("record 1: timestamp=%d, want %d", r.header.Timestamp, now.Add(5*time.Second).Unix())
	}
}

func TestMRTLogStateChangeIPVerification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")

	m := NewMRTLog(path, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var records []stateRecord
	handler := &mrt.Handler{
		OnStateChange: func(h mrt.Header, _ uint32, s *mrt.StateChangeRecord) error {
			records = append(records, stateRecord{header: h, sc: s})
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	// Verify peer IP and local IP in the record
	wantPeerIP := []byte{10, 255, 0, 2}
	wantLocalIP := []byte{127, 0, 0, 1}
	if !bytes.Equal(r.sc.PeerIP, wantPeerIP) {
		t.Errorf("PeerIP=%v, want %v", r.sc.PeerIP, wantPeerIP)
	}
	if !bytes.Equal(r.sc.LocalIP, wantLocalIP) {
		t.Errorf("LocalIP=%v, want %v", r.sc.LocalIP, wantLocalIP)
	}
	if r.sc.AFI != mrt.AFIIPv4 {
		t.Errorf("AFI=%d, want %d", r.sc.AFI, mrt.AFIIPv4)
	}
}

func TestMRTLogDisconnectBeforeEstablished(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")

	m := NewMRTLog(path, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	// Disconnect without prior Established — should produce no state change record
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 1, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 2, Time: now})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return // File not created is acceptable
	}
	if info.Size() != 0 {
		t.Errorf("file size=%d, want 0 (no record for disconnect before established)", info.Size())
	}
}

func TestMRTLogDisconnectReconnectCycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")

	m := NewMRTLog(path, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	// Simulate: establish, disconnect, re-establish, disconnect again
	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(1 * time.Second)})
	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now.Add(2 * time.Second)})
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(3 * time.Second)})
	// Extra disconnect after already disconnected — should be skipped
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(4 * time.Second)})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var records []stateRecord
	handler := &mrt.Handler{
		OnStateChange: func(h mrt.Header, _ uint32, s *mrt.StateChangeRecord) error {
			records = append(records, stateRecord{header: h, sc: s})
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Expect 4 records: establish, disconnect, establish, disconnect
	// The 5th event (duplicate disconnect) should be skipped
	if len(records) != 4 {
		t.Fatalf("got %d records, want 4", len(records))
	}

	// Verify alternating pattern
	for i, r := range records {
		if i%2 == 0 {
			if r.sc.OldState != mrt.FSMIdle || r.sc.NewState != mrt.FSMEstablished {
				t.Errorf("record %d: old=%d new=%d, want Idle->Established", i, r.sc.OldState, r.sc.NewState)
			}
		} else {
			if r.sc.OldState != mrt.FSMEstablished || r.sc.NewState != mrt.FSMIdle {
				t.Errorf("record %d: old=%d new=%d, want Established->Idle", i, r.sc.OldState, r.sc.NewState)
			}
		}
	}
}

func TestMRTLogMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")

	m := NewMRTLog(path, testMRTLogConfig())

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	bgpMsg := makeFakeBGPUpdate()

	// AC-3: Route sent with BGPMessage
	m.ProcessEvent(peer.Event{
		Type:       peer.EventRouteSent,
		PeerIndex:  0,
		Time:       now,
		BGPMessage: bgpMsg,
	})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var records []msgRecord
	handler := &mrt.Handler{
		OnMessage: func(h mrt.Header, _ uint32, msg *mrt.MessageRecord) error {
			records = append(records, msgRecord{header: h, msg: msg})
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	r := records[0]
	if r.header.Type != mrt.TypeBGP4MP || r.header.Subtype != mrt.BGP4MPMessageAS4 {
		t.Errorf("type=%d subtype=%d, want %d/%d", r.header.Type, r.header.Subtype, mrt.TypeBGP4MP, mrt.BGP4MPMessageAS4)
	}
	if r.msg.PeerAS != 64512 || r.msg.LocalAS != 65000 {
		t.Errorf("peerAS=%d localAS=%d, want 64512/65000", r.msg.PeerAS, r.msg.LocalAS)
	}

	// Verify the BGP message bytes are preserved exactly
	if !bytes.Equal(r.msg.BGPMessage, bgpMsg) {
		t.Fatalf("BGPMessage mismatch: got %d bytes, want %d bytes", len(r.msg.BGPMessage), len(bgpMsg))
	}
}

func TestMRTLogReceivedMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")

	m := NewMRTLog(path, testMRTLogConfig())

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	bgpMsg := makeFakeBGPUpdate()

	// AC-4: Route received with BGPMessage
	m.ProcessEvent(peer.Event{
		Type:       peer.EventRouteReceived,
		PeerIndex:  2,
		Time:       now,
		BGPMessage: bgpMsg,
	})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var count int
	handler := &mrt.Handler{
		OnMessage: func(_ mrt.Header, _ uint32, msg *mrt.MessageRecord) error {
			count++
			if msg.PeerAS != 65000 {
				t.Errorf("peerAS=%d, want 65000 (iBGP peer)", msg.PeerAS)
			}
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if count != 1 {
		t.Fatalf("got %d message records, want 1", count)
	}
}

func TestMRTLogNilMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")

	m := NewMRTLog(path, testMRTLogConfig())

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	// AC-5: EventRouteSent with nil BGPMessage — no record written
	m.ProcessEvent(peer.Event{
		Type:      peer.EventRouteSent,
		PeerIndex: 0,
		Time:      now,
	})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		// File not created is also acceptable (no records to write).
		return
	}
	if info.Size() != 0 {
		t.Errorf("file size=%d, want 0 (no record for nil BGPMessage)", info.Size())
	}
}

func TestMRTLogRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.mrt")

	cfg := testMRTLogConfig()
	m := NewMRTLog(path, cfg)

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	bgpMsg := makeFakeBGPUpdate()

	// Write a mix of state changes and messages
	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 1, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventRouteSent, PeerIndex: 0, Time: now.Add(time.Second), BGPMessage: bgpMsg})
	m.ProcessEvent(peer.Event{Type: peer.EventRouteReceived, PeerIndex: 1, Time: now.Add(2 * time.Second), BGPMessage: bgpMsg})
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 0, Time: now.Add(3 * time.Second)})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// AC-7: Read back with mrt.ReadFile — all records decode without error
	var stateChanges int
	var messages int
	handler := &mrt.Handler{
		OnStateChange: func(h mrt.Header, _ uint32, _ *mrt.StateChangeRecord) error {
			if h.Type != mrt.TypeBGP4MP {
				t.Errorf("state change type=%d, want %d", h.Type, mrt.TypeBGP4MP)
			}
			if h.Subtype != mrt.BGP4MPStateChangeAS4 {
				t.Errorf("state change subtype=%d, want %d", h.Subtype, mrt.BGP4MPStateChangeAS4)
			}
			stateChanges++
			return nil
		},
		OnMessage: func(h mrt.Header, _ uint32, _ *mrt.MessageRecord) error {
			if h.Type != mrt.TypeBGP4MP {
				t.Errorf("message type=%d, want %d", h.Type, mrt.TypeBGP4MP)
			}
			if h.Subtype != mrt.BGP4MPMessageAS4 {
				t.Errorf("message subtype=%d, want %d", h.Subtype, mrt.BGP4MPMessageAS4)
			}
			messages++
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// 2 Established + 1 Disconnected = 3 state changes
	if stateChanges != 3 {
		t.Errorf("state changes=%d, want 3", stateChanges)
	}
	if messages != 2 {
		t.Errorf("messages=%d, want 2", messages)
	}
}

func TestMRTLogLargeMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.mrt")

	m := NewMRTLog(path, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	// Build a near-max BGP message (4096 bytes of attributes)
	bgpMsg := makeBGPUpdateWithBody(4096)

	m.ProcessEvent(peer.Event{
		Type:       peer.EventRouteSent,
		PeerIndex:  0,
		Time:       now,
		BGPMessage: bgpMsg,
	})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var got []byte
	handler := &mrt.Handler{
		OnMessage: func(_ mrt.Header, _ uint32, msg *mrt.MessageRecord) error {
			got = msg.BGPMessage
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !bytes.Equal(got, bgpMsg) {
		t.Fatalf("large message not preserved: got %d bytes, want %d", len(got), len(bgpMsg))
	}
}

func TestMRTLogMultiplePeers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.mrt")

	m := NewMRTLog(path, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	bgpMsg := makeFakeBGPUpdate()

	// All three peers send messages
	for i := range 3 {
		m.ProcessEvent(peer.Event{
			Type:       peer.EventRouteSent,
			PeerIndex:  i,
			Time:       now.Add(time.Duration(i) * time.Second),
			BGPMessage: bgpMsg,
		})
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	wantASNs := []uint32{64512, 64513, 65000}
	var idx int
	handler := &mrt.Handler{
		OnMessage: func(_ mrt.Header, _ uint32, msg *mrt.MessageRecord) error {
			if msg.PeerAS != wantASNs[idx] {
				t.Errorf("message %d: peerAS=%d, want %d", idx, msg.PeerAS, wantASNs[idx])
			}
			idx++
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if idx != 3 {
		t.Fatalf("got %d messages, want 3", idx)
	}
}

func TestMRTLogOutOfBoundsPeerIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oob.mrt")

	m := NewMRTLog(path, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	bgpMsg := makeFakeBGPUpdate()

	// PeerIndex beyond config range — should not panic
	m.ProcessEvent(peer.Event{
		Type:       peer.EventRouteSent,
		PeerIndex:  99,
		Time:       now,
		BGPMessage: bgpMsg,
	})
	// Negative peer index
	m.ProcessEvent(peer.Event{
		Type:       peer.EventRouteSent,
		PeerIndex:  -1,
		Time:       now,
		BGPMessage: bgpMsg,
	})
	// Established with out-of-bounds peer — should not panic
	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 99, Time: now})
	// Disconnected with out-of-bounds peer — should not panic
	m.ProcessEvent(peer.Event{Type: peer.EventDisconnected, PeerIndex: 99, Time: now})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify records are produced for message events (with zeroed peer fields)
	var count int
	handler := &mrt.Handler{
		OnMessage: func(_ mrt.Header, _ uint32, msg *mrt.MessageRecord) error {
			if msg.PeerAS != 0 {
				t.Errorf("out-of-bounds peer: PeerAS=%d, want 0", msg.PeerAS)
			}
			count++
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if count != 2 {
		t.Errorf("got %d messages, want 2", count)
	}
}

func TestMRTLogIgnoresIrrelevantEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "irrelevant.mrt")

	m := NewMRTLog(path, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	// These event types should produce no MRT records
	m.ProcessEvent(peer.Event{Type: peer.EventEORSent, PeerIndex: 0, Time: now, Count: 100})
	m.ProcessEvent(peer.Event{Type: peer.EventError, PeerIndex: 0, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventChaosExecuted, PeerIndex: 0, Time: now, ChaosAction: "disconnect"})
	m.ProcessEvent(peer.Event{Type: peer.EventReconnecting, PeerIndex: 0, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventRouteAction, PeerIndex: 0, Time: now})
	m.ProcessEvent(peer.Event{Type: peer.EventDroppedEvents, PeerIndex: 0, Time: now, Count: 5})
	m.ProcessEvent(peer.Event{Type: peer.EventRouteWithdrawn, PeerIndex: 0, Time: now})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return // No file = no records = correct
	}
	if info.Size() != 0 {
		t.Errorf("file size=%d, want 0 (no MRT records for irrelevant events)", info.Size())
	}
}

func TestMRTLogStrftimePattern(t *testing.T) {
	dir := t.TempDir()
	// AC-6: strftime pattern in filename
	pattern := filepath.Join(dir, "chaos-%Y%m%d.mrt")

	m := NewMRTLog(pattern, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	m.ProcessEvent(peer.Event{Type: peer.EventEstablished, PeerIndex: 0, Time: now})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The file should exist with today's date expanded
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
	name := entries[0].Name()
	// Should start with "chaos-" and end with ".mrt", with 8 digits in between
	if len(name) != len("chaos-20260115.mrt") {
		t.Errorf("filename=%q, expected chaos-YYYYMMDD.mrt format", name)
	}
}

func TestMRTLogWithdrawalSentWithMessage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mrt")

	m := NewMRTLog(path, testMRTLogConfig())
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	bgpMsg := makeFakeBGPUpdate()

	// EventWithdrawalSent with BGPMessage should produce a MESSAGE record
	m.ProcessEvent(peer.Event{
		Type:       peer.EventWithdrawalSent,
		PeerIndex:  0,
		Time:       now,
		BGPMessage: bgpMsg,
	})

	if err := m.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var count int
	handler := &mrt.Handler{
		OnMessage: func(_ mrt.Header, _ uint32, _ *mrt.MessageRecord) error {
			count++
			return nil
		},
	}
	if err := mrt.ReadFile(path, handler); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if count != 1 {
		t.Fatalf("got %d messages, want 1 (WithdrawalSent with BGPMessage)", count)
	}
}

type stateRecord struct {
	header mrt.Header
	sc     *mrt.StateChangeRecord
}

type msgRecord struct {
	header mrt.Header
	msg    *mrt.MessageRecord
}

// makeFakeBGPUpdate builds a minimal valid BGP UPDATE message (header + empty update).
func makeFakeBGPUpdate() []byte {
	// BGP header: 16-byte marker + 2-byte length + 1-byte type
	// UPDATE body: 2-byte withdrawn len (0) + 2-byte attr len (0) = 4 bytes
	msgLen := 19 + 4
	msg := make([]byte, msgLen)
	// Marker: all 0xFF
	for i := range 16 {
		msg[i] = 0xFF
	}
	msg[16] = 0
	msg[17] = byte(msgLen)
	msg[18] = 2 // UPDATE
	// Withdrawn routes length = 0, path attribute length = 0
	return msg
}

// makeBGPUpdateWithBody builds a BGP UPDATE with a body of the given size.
func makeBGPUpdateWithBody(bodySize int) []byte {
	msgLen := 19 + 4 + bodySize // header + wd_len(2) + attr_len(2) + attributes
	msg := make([]byte, msgLen)
	for i := range 16 {
		msg[i] = 0xFF
	}
	msg[16] = byte(msgLen >> 8)
	msg[17] = byte(msgLen)
	msg[18] = 2 // UPDATE
	// Withdrawn routes length = 0
	msg[19] = 0
	msg[20] = 0
	// Path attribute length = bodySize
	msg[21] = byte(bodySize >> 8)
	msg[22] = byte(bodySize)
	// Fill attributes with pattern
	for i := range bodySize {
		msg[23+i] = byte(i & 0xFF)
	}
	return msg
}
