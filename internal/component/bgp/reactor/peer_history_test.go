package reactor

import (
	"testing"
	"time"
)

func TestFSMHistoryAppendAndSnapshot(t *testing.T) {
	h := newFSMHistory()
	now := time.Now()

	h.append(FSMTransition{Timestamp: now, From: "IDLE", To: "CONNECT"})
	h.append(FSMTransition{Timestamp: now.Add(time.Second), From: "CONNECT", To: "OPENSENT"})
	h.append(FSMTransition{Timestamp: now.Add(2 * time.Second), From: "OPENSENT", To: "ESTABLISHED"})

	snap := h.snapshot()
	if len(snap) != 3 {
		t.Fatalf("count = %d, want 3", len(snap))
	}
	if snap[0].To != "ESTABLISHED" {
		t.Errorf("newest.To = %s, want ESTABLISHED", snap[0].To)
	}
	if snap[2].To != "CONNECT" {
		t.Errorf("oldest.To = %s, want CONNECT", snap[2].To)
	}
}

func TestFSMHistoryOverflow(t *testing.T) {
	h := newFSMHistory()
	for i := range peerHistoryCapacity + 5 {
		h.append(FSMTransition{From: "A", To: string(rune('a' + i%26))})
	}
	snap := h.snapshot()
	if len(snap) != peerHistoryCapacity {
		t.Fatalf("count = %d, want %d", len(snap), peerHistoryCapacity)
	}
}

func TestFSMHistoryEmpty(t *testing.T) {
	h := newFSMHistory()
	snap := h.snapshot()
	if snap == nil {
		t.Fatal("snapshot should be non-nil empty slice")
	}
	if len(snap) != 0 {
		t.Fatalf("count = %d, want 0", len(snap))
	}
}

func TestPeerFSMHistory(t *testing.T) {
	p := &Peer{history: newFSMHistory()}
	p.history.append(FSMTransition{From: "IDLE", To: "CONNECT"})
	p.history.append(FSMTransition{From: "CONNECT", To: "ESTABLISHED"})

	snap := p.FSMHistory()
	if len(snap) != 2 {
		t.Fatalf("count = %d, want 2", len(snap))
	}
	if snap[0].To != "ESTABLISHED" {
		t.Errorf("newest = %s, want ESTABLISHED", snap[0].To)
	}
}

func TestPeerFSMHistoryNil(t *testing.T) {
	p := &Peer{}
	snap := p.FSMHistory()
	if len(snap) != 0 {
		t.Fatalf("count = %d, want 0", len(snap))
	}
}
