package engine

import (
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/clock"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/auth"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/transport"
)

const (
	addrA = "203.0.113.1"
	addrB = "203.0.113.2"
)

func reqFor(peer, local string) api.SessionRequest {
	return api.SessionRequest{
		Peer:                  netip.MustParseAddr(peer),
		Local:                 netip.MustParseAddr(local),
		Interface:             "loop",
		Mode:                  api.SingleHop,
		DesiredMinTxInterval:  10_000, // 10ms
		RequiredMinRxInterval: 10_000, // 10ms
		DetectMult:            3,
	}
}

// VALIDATES: two Loops connected via paired Loopback transports run the
// complete three-way handshake and reach Up on both sides within a
// reasonable wall-clock window.
// PREVENTS: regression where the engine fails to tick, fails to transmit,
// fails to dispatch first-packet by key, or deadlocks under concurrent
// EnsureSession.
func TestLoopbackHandshake(t *testing.T) {
	lbA, lbB := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))

	loopA := NewLoop(lbA, clock.RealClock{})
	loopB := NewLoop(lbB, clock.RealClock{})

	if err := loopA.Start(); err != nil {
		t.Fatalf("loopA.Start: %v", err)
	}
	defer func() {
		if err := loopA.Stop(); err != nil {
			t.Errorf("loopA.Stop: %v", err)
		}
	}()
	if err := loopB.Start(); err != nil {
		t.Fatalf("loopB.Start: %v", err)
	}
	defer func() {
		if err := loopB.Stop(); err != nil {
			t.Errorf("loopB.Stop: %v", err)
		}
	}()

	hA, err := loopA.EnsureSession(reqFor(addrB, addrA))
	if err != nil {
		t.Fatalf("loopA.EnsureSession: %v", err)
	}
	hB, err := loopB.EnsureSession(reqFor(addrA, addrB))
	if err != nil {
		t.Fatalf("loopB.EnsureSession: %v", err)
	}

	subA := hA.Subscribe()
	subB := hB.Subscribe()
	defer hA.Unsubscribe(subA)
	defer hB.Unsubscribe(subB)

	// Wait up to 5 seconds for both sides to reach Up. Slow-start uses
	// 1 second intervals so the handshake typically completes in ~2 s.
	deadline := time.Now().Add(5 * time.Second)
	var upA, upB bool
	for !upA || !upB {
		if time.Now().After(deadline) {
			t.Fatalf("handshake did not reach Up in time (upA=%v upB=%v)", upA, upB)
		}
		select {
		case change, ok := <-subA:
			if !ok {
				t.Fatalf("subA closed prematurely")
			}
			if change.State == packet.StateUp {
				upA = true
			}
		case change, ok := <-subB:
			if !ok {
				t.Fatalf("subB closed prematurely")
			}
			if change.State == packet.StateUp {
				upB = true
			}
		case <-time.After(time.Until(deadline) + 10*time.Millisecond):
			t.Fatalf("no state change received (upA=%v upB=%v)", upA, upB)
		}
	}
}

// VALIDATES: once both sides reach Up through the full express loop,
// the Poll Sequence initiated on the Up transition terminates within a
// few ticks (the peer's Final reply clears PollOutstanding). After the
// sequence terminates the operating TX interval is the configured fast
// value, not the slow-start floor.
// PREVENTS: regression where the Poll Sequence never terminates and
// both sides remain stuck on slow-start intervals after reaching Up.
func TestLoopbackPollFinalTerminates(t *testing.T) {
	lbA, lbB := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	loopA := NewLoop(lbA, clock.RealClock{})
	loopB := NewLoop(lbB, clock.RealClock{})
	if err := loopA.Start(); err != nil {
		t.Fatalf("loopA.Start: %v", err)
	}
	defer func() { _ = loopA.Stop() }()
	if err := loopB.Start(); err != nil {
		t.Fatalf("loopB.Start: %v", err)
	}
	defer func() { _ = loopB.Stop() }()

	if _, err := loopA.EnsureSession(reqFor(addrB, addrA)); err != nil {
		t.Fatalf("loopA.EnsureSession: %v", err)
	}
	if _, err := loopB.EnsureSession(reqFor(addrA, addrB)); err != nil {
		t.Fatalf("loopB.EnsureSession: %v", err)
	}

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		loopA.mu.Lock()
		var aPoll bool
		var aTx uint32
		for _, e := range loopA.sessions {
			if e.machine.State() == packet.StateUp {
				aPoll = e.machine.PollOutstanding()
				aTx = e.machine.DesiredMinTxIntervalUs()
			}
		}
		loopA.mu.Unlock()
		if aTx > 0 && !aPoll {
			if aTx != 10_000 {
				t.Fatalf("TX interval after poll completion: got %d want 10000", aTx)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Poll Sequence did not terminate within 6s")
}

// VALIDATES: EnsureSession is idempotent and refcounts a shared session.
// PREVENTS: regression where a second client creates a duplicate session
// instead of sharing one.
func TestEnsureSessionRefcount(t *testing.T) {
	lbA, lbB := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	defer func() { _ = lbA.Stop() }()
	defer func() { _ = lbB.Stop() }()

	loop := NewLoop(lbA, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("loop.Start: %v", err)
	}
	defer func() {
		if err := loop.Stop(); err != nil {
			t.Errorf("loop.Stop: %v", err)
		}
	}()

	req := reqFor(addrB, addrA)

	h1, err := loop.EnsureSession(req)
	if err != nil {
		t.Fatalf("first EnsureSession: %v", err)
	}
	h2, err := loop.EnsureSession(req)
	if err != nil {
		t.Fatalf("second EnsureSession: %v", err)
	}
	if h1.Key() != h2.Key() {
		t.Fatalf("handles carry different keys: %+v vs %+v", h1.Key(), h2.Key())
	}

	loop.mu.Lock()
	entry := loop.sessions[h1.Key()]
	loop.mu.Unlock()
	if entry == nil {
		t.Fatal("session not in map after EnsureSession")
	}
	if got := entry.machine.Refcount(); got != 2 {
		t.Fatalf("refcount after 2x EnsureSession: got %d want 2", got)
	}

	if err := loop.ReleaseSession(h1); err != nil {
		t.Fatalf("first ReleaseSession: %v", err)
	}
	loop.mu.Lock()
	entry2 := loop.sessions[h1.Key()]
	loop.mu.Unlock()
	if entry2 == nil {
		t.Fatal("session torn down before refcount reached zero")
	}
	if got := entry2.machine.Refcount(); got != 1 {
		t.Fatalf("refcount after one Release: got %d want 1", got)
	}

	if err := loop.ReleaseSession(h2); err != nil {
		t.Fatalf("second ReleaseSession: %v", err)
	}
	loop.mu.Lock()
	_, stillPresent := loop.sessions[h1.Key()]
	loop.mu.Unlock()
	if stillPresent {
		t.Fatal("session still present after final Release")
	}
}

// VALIDATES: Loop.Stop closes the auth persister of every pinned
// session so the Meticulous Keyed TX sequence reaches disk before the
// process exits, even when ReleaseSession was never called.
// PREVENTS: regression of the bfd-auth-meticulous-persist flake where
// the runtime teardown path skipped CloseAuth on still-pinned sessions
// and the persister's 500 ms ticker was the only flush mechanism.
func TestLoopStopFlushesPinnedPersister(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("k-persist-test")

	lbA, lbB := transport.Pair(api.SingleHop, netip.MustParseAddr(addrA), netip.MustParseAddr(addrB))
	defer func() { _ = lbB.Stop() }()

	loop := NewLoop(lbA, clock.RealClock{})
	if err := loop.Start(); err != nil {
		t.Fatalf("loop.Start: %v", err)
	}

	req := reqFor(addrB, addrA)
	req.Auth = &api.AuthSettings{
		Type:       packet.AuthTypeMeticulousKeyedSHA1,
		KeyID:      1,
		Secret:     secret,
		Meticulous: true,
	}
	req.PersistDir = dir

	if _, err := loop.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}

	// Wait for the express-loop to tick a handful of times so
	// AdvanceAuthSeq has stored at least one sequence. The persister's
	// 500 ms ticker cannot have fired yet -- if the .seq file exists
	// after Stop, Stop's CloseAuth path is the only thing that could
	// have written it.
	deadline := time.Now().Add(250 * time.Millisecond)
	var txFired bool
	for time.Now().Before(deadline) {
		loop.mu.Lock()
		for _, entry := range loop.sessions {
			if entry.txPackets > 0 {
				txFired = true
			}
		}
		loop.mu.Unlock()
		if txFired {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !txFired {
		t.Fatal("no TX packet within 250ms; express-loop never advanced auth")
	}

	if err := loop.Stop(); err != nil {
		t.Fatalf("loop.Stop: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read persist dir: %v", err)
	}
	var seqFile string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".seq" {
			seqFile = filepath.Join(dir, e.Name())
			break
		}
	}
	if seqFile == "" {
		t.Fatalf("no .seq file present in %s after Stop; CloseAuth did not flush", dir)
	}

	raw, err := os.ReadFile(seqFile) //nolint:gosec // test-owned tempdir
	if err != nil {
		t.Fatalf("read %s: %v", seqFile, err)
	}
	n, err := strconv.ParseUint(string(raw), 10, 32)
	if err != nil {
		t.Fatalf("parse %s contents %q: %v", seqFile, raw, err)
	}
	if n == 0 {
		t.Fatalf("persisted sequence is zero; expected > 0 after express-loop TX")
	}

	// A fresh persister on the same directory + key must see the
	// stored sequence as its starting floor.
	keyStr := netip.MustParseAddr(addrB).String() + "--" + api.SingleHop.String()
	p, err := auth.NewSeqPersister(dir, keyStr)
	if err != nil {
		t.Fatalf("reopen NewSeqPersister: %v", err)
	}
	defer func() { _ = p.Close() }()
	if got := p.Start(); uint64(got) != n {
		t.Fatalf("reopened Start() = %d, want %d", got, n)
	}
}
