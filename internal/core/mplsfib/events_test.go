// Design: docs/architecture/mpls/mpls-kernel.md -- targeted replay of label entries to one writer
package mplsfib

import (
	"errors"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/replay"
)

// syncBus delivers every Emit to the in-process subscribers synchronously, as
// the daemon's bus delivers to engine subscribers, so Apply's acknowledgement
// callback runs before Emit returns.
type syncBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func (b *syncBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	handlers := append([]func(any){}, b.subs[namespace+"/"+eventType]...)
	b.mu.Unlock()
	for _, handler := range handlers {
		handler(payload)
	}
	return 0, nil
}

func (b *syncBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs == nil {
		b.subs = make(map[string][]func(any))
	}
	b.subs[namespace+"/"+eventType] = append(b.subs[namespace+"/"+eventType], handler)
	return func() {}
}

// testWriter is a label writer as fib-kernel and fib-vpp are: it consumes and
// acknowledges only the batches that address the token it holds.
type testWriter struct {
	held     uint64
	consumed int
	refusal  error
}

func (w *testWriter) handle(batch *EntryBatch) {
	if !batch.Addresses(w.held) {
		return
	}
	w.consumed++
	batch.Acknowledge(w.refusal)
}

// TestApplyReplayTargetsOneWriter proves the replay answer reaches only the
// writer that asked, while an incremental batch reaches every writer.
//
// VALIDATES: AC-25 -- a batch carrying a token is consumed and acknowledged
// only by the writer holding it; a writer holding another token (or none, as
// fib-kernel) neither consumes nor acknowledges it.
// METHOD: two writers on one synchronous bus; Apply, then ApplyReplay with
// each writer's token, then a token nobody holds.
func TestApplyReplayTargetsOneWriter(t *testing.T) {
	bus := &syncBus{}
	kernel := &testWriter{}
	vpp := &testWriter{held: NewReplayToken()}
	t.Cleanup(EntryChange.Subscribe(bus, kernel.handle))
	t.Cleanup(EntryChange.Subscribe(bus, vpp.handle))
	entries := []Entry{{Action: ActionAdd, Op: OpPop, InLabel: 18000, Source: 2}}

	if err := Apply(bus, entries); err != nil {
		t.Fatalf("incremental Apply: %v", err)
	}
	if kernel.consumed != 1 || vpp.consumed != 1 {
		t.Fatalf("incremental batch consumed kernel=%d vpp=%d, want 1 and 1", kernel.consumed, vpp.consumed)
	}

	if err := ApplyReplay(bus, vpp.held, entries); err != nil {
		t.Fatalf("ApplyReplay to the asking writer: %v", err)
	}
	if kernel.consumed != 1 {
		t.Fatalf("kernel writer consumed a replay it did not ask for (consumed %d)", kernel.consumed)
	}
	if vpp.consumed != 2 {
		t.Fatalf("asking writer consumed %d batches, want 2", vpp.consumed)
	}

	vpp.refusal = errors.New("vpp refused")
	if err := ApplyReplay(bus, vpp.held, entries); !errors.Is(err, vpp.refusal) {
		t.Fatalf("ApplyReplay error = %v, want the asking writer's refusal", err)
	}

	stale := NewReplayToken()
	if err := ApplyReplay(bus, stale, entries); !errors.Is(err, errNoOwnerAcknowledgement) {
		t.Fatalf("ApplyReplay to a token no writer holds = %v, want errNoOwnerAcknowledgement", err)
	}
	if err := ApplyReplay(bus, 0, entries); err == nil {
		t.Fatal("ApplyReplay accepted the incremental token 0")
	}
}

// TestEntryBatchAddresses pins the three token cases of the shared replay
// vocabulary on the label batch.
func TestEntryBatchAddresses(t *testing.T) {
	held := NewReplayToken()
	cases := []struct {
		name     string
		replayID uint64
		token    uint64
		want     bool
	}{
		{"incremental reaches a writer with no request", 0, 0, true},
		{"incremental reaches a writer with a request", 0, held, true},
		{"broadcast reaches every writer", replay.Broadcast, 0, true},
		{"targeted reaches its writer", held, held, true},
		{"targeted skips a writer with no request", held, 0, false},
		{"targeted skips a writer with another request", held, held + 1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			batch := &EntryBatch{ReplayID: c.replayID}
			if got := batch.Addresses(c.token); got != c.want {
				t.Fatalf("Addresses(%d) on ReplayID %d = %v, want %v", c.token, c.replayID, got, c.want)
			}
		})
	}
}

// TestReplayTokensAreDistinct proves the process-wide counter never hands
// out the incremental token, the broadcast token, or a token twice, also
// under concurrent writers.
//
// VALIDATES: AC-25 -- a fresh targeted token per writer open.
func TestReplayTokensAreDistinct(t *testing.T) {
	const writers, perWriter = 8, 64
	tokens := make(chan uint64, writers*perWriter)
	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			for range perWriter {
				tokens <- NewReplayToken()
			}
		})
	}
	wg.Wait()
	close(tokens)

	seen := make(map[uint64]bool, writers*perWriter)
	for token := range tokens {
		if !replay.IsReplay(token) {
			t.Fatal("NewReplayToken returned the incremental token 0")
		}
		if token == replay.Broadcast {
			t.Fatal("NewReplayToken returned replay.Broadcast")
		}
		if seen[token] {
			t.Fatalf("NewReplayToken returned %d twice", token)
		}
		seen[token] = true
	}
	if len(seen) != writers*perWriter {
		t.Fatalf("got %d distinct tokens, want %d", len(seen), writers*perWriter)
	}
}

// TestReplayRequestHandle pins the wire identity of the request event.
func TestReplayRequestHandle(t *testing.T) {
	if ReplayRequest.Namespace() != Namespace || ReplayRequest.EventType() != "replay-request" {
		t.Fatalf("ReplayRequest is (%s, %s), want (%s, replay-request)",
			ReplayRequest.Namespace(), ReplayRequest.EventType(), Namespace)
	}
}
