// Design: docs/guide/route-reflection.md -- a late destination's replay precedes its live forwards
// Related: server_handlers.go -- handleStateUp and endReplay, the replay gate
// Related: server_forward.go -- flushBatch, the live rail the engine fences per destination
// Related: ../../reactor/forward_replay_fence_test.go -- the engine's replay fence

package rs

import (
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/family"
)

// replayOrderFixture is a route server with a source, a destination whose
// peer-up replay is held open, and a third peer already up. Every send the
// plugin makes is appended to one ordered log as "<peer> <what>".
type replayOrderFixture struct {
	rs      *routeServer
	mu      sync.Mutex
	log     []string
	release chan struct{}
	// releaseOnce closes release once, from endReplay or from cleanup.
	releaseOnce         sync.Once
	source, dest, other string
}

func newReplayOrderFixture(t *testing.T) *replayOrderFixture {
	t.Helper()
	f := &replayOrderFixture{
		rs:      newTestRouteServer(t),
		release: make(chan struct{}),
		source:  "10.0.0.1", dest: "10.0.0.2", other: "10.0.0.3",
	}
	t.Cleanup(func() { f.releaseOnce.Do(func() { close(f.release) }) })

	entered := make(chan struct{})
	var enterOnce sync.Once
	f.rs.dispatchCommandHook = func(cmd string, args []string, _ string) (string, json.RawMessage, error) {
		if cmd != cmdAdjRIBInReplay {
			return statusDone, nil, nil
		}
		if len(args) >= 2 && args[1] == "0" {
			enterOnce.Do(func() { close(entered) })
			<-f.release
			f.record(args[0] + " replay")
			return statusDone, json.RawMessage(`{"last-index":1,"replayed":1}`), nil
		}
		return statusDone, json.RawMessage(`{"last-index":1,"replayed":0}`), nil
	}
	f.rs.updateRouteHook = func(peer, cmd string) {
		switch {
		case strings.HasSuffix(cmd, " eor"):
			f.record(peer + " eor")
		case strings.Contains(cmd, " del "):
			f.record(peer + " withdraw")
		}
	}
	f.rs.forwardCachedHook = func(_ []uint64, destinations []string) {
		sorted := slices.Clone(destinations)
		slices.Sort(sorted)
		for _, d := range sorted {
			f.record(d + " live")
		}
	}

	fams := map[family.Family]bool{family.IPv4Unicast: true}
	f.rs.mu.Lock()
	f.rs.peers[f.source] = &PeerState{Address: f.source, Up: true, StateSeen: true, Families: fams}
	f.rs.peers[f.other] = &PeerState{Address: f.other, Up: true, StateSeen: true, Families: fams}
	f.rs.peers[f.dest] = &PeerState{Address: f.dest, Families: fams}
	// Message 1 was taken delivery of before dest came up: it is the replay's.
	f.rs.seenMsgID = 1
	f.rs.mu.Unlock()

	f.rs.handleState(&Event{PeerAddr: f.dest, State: "up"})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("peer-up replay never started")
	}
	return f
}

func (f *replayOrderFixture) record(event string) {
	f.mu.Lock()
	f.log = append(f.log, event)
	f.mu.Unlock()
}

// endReplay lets the held replay finish and waits until its gate is cleared.
func (f *replayOrderFixture) endReplay(t *testing.T) {
	t.Helper()
	f.releaseOnce.Do(func() { close(f.release) })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f.rs.mu.RLock()
		p := f.rs.peers[f.dest]
		cleared := p != nil && p.replayDone == nil
		f.rs.mu.RUnlock()
		if cleared {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the replay gate was never cleared")
}

// TestReplayDoesNotDelayOtherDestinations proves that a live forward goes out
// while a destination's peer-up replay is still running, to every target
// including the replaying one, whose order the engine keeps.
//
// VALIDATES: flushBatch never waits for a replay. The replaying destination
// stays a target; the engine holds that destination's copy behind its replay
// fence (reactor TestLiveForwardWaitsForPeerUpReplay), and every other
// destination receives the forward at once.
// PREVENTS: the head-of-line block of a plugin-wide wait, where one peer's
// replay (up to its two-minute lifetime) stalled live forwarding to every
// peer of the route server.
func TestReplayDoesNotDelayOtherDestinations(t *testing.T) {
	f := newReplayOrderFixture(t)
	fams := map[family.Family]bool{family.IPv4Unicast: true}

	key := workerKey{sourcePeer: f.source}
	liveDone := make(chan struct{})
	go func() {
		defer close(liveDone)
		f.rs.batchForwardUpdate(key, f.source, 2, fams)
		f.rs.flushWorkerBatch(key)
	}()

	select {
	case <-liveDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("a live forward waited for another destination's replay: %v", snapshot(&f.mu, &f.log))
	}
	got := snapshot(&f.mu, &f.log)
	want := []string{f.dest + " live", f.other + " live"}
	if !slices.Equal(got, want) {
		t.Fatalf("sends during the replay %v, want %v", got, want)
	}
	f.endReplay(t)
}

// snapshot copies the delivered log under its lock.
func snapshot(mu *sync.Mutex, delivered *[]string) []string {
	mu.Lock()
	defer mu.Unlock()
	return slices.Clone(*delivered)
}
