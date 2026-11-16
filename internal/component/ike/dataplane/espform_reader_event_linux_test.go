//go:build linux

// Design: docs/guide/ipsec.md -- Both ESP wire forms
// Related: espform_open_linux.go -- reader failure reporting and recovery

package dataplane

import (
	"errors"
	"net/netip"
	"testing"
	"time"
)

// espFormReaderBeforeRevive installs one reader without scheduling its revive worker.
// The test MUST run read with one wg hold, and MUST run revive with one revivers hold.
// This controls worker scheduling alone: read, Watch and retireLocked still produce
// every lifecycle transition, including the ordering of err, dead and stop.
func espFormReaderBeforeRevive(r *espFormReceiver, paths espFormPaths) *espFormReaderLife {
	life := &espFormReaderLife{stop: make(chan struct{}), dead: make(chan struct{}), opened: time.Now()}
	r.conn, r.tun, r.life = paths.conn, paths.tun, life
	return life
}

// TestESPFormReceiverReportsFailureRetiredBeforeRevive proves a recorded read failure
// is reported exactly once even when Watch has already replaced its reader.
//
// METHOD: run the actual read producer before scheduling revive, then let Watch see
// dead and retire the old life. Both channels are ready when revive starts. Each of
// 64 independent lives must report once; the broken two-ready select has probability
// 2^-64 of choosing dead every time. There is no timing or scheduler-speed oracle.
// The replacement must serve a watched SA and no stale worker may reopen its paths.
// MUTATION: return unconditionally when revive selects stop: a recorded failure loses
// its event on the first iteration that selects stop instead of dead.
func TestESPFormReceiverReportsFailureRetiredBeforeRevive(t *testing.T) {
	const spi = 0x5E0F0001
	peer, local := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	for attempt := range 64 {
		first, replacement := newIdleESPFormPaths(), newIdleESPFormPaths()
		opens := 0
		r := espFormTestReceiver(t, func() (espFormPaths, error) {
			opens++
			if opens != 1 {
				return espFormPaths{}, errors.New("test: a retired reader attempted another open")
			}
			return replacement, nil
		})
		events := make(chan ESPFormReaderStopped, 2)
		r.readerStopped = func(stopped ESPFormReaderStopped) { events <- stopped }
		life := espFormReaderBeforeRevive(r, first)
		r.reg.watch(spi, natTarget(peer, local))
		failedSocket, ok := first.conn.(*espFormScriptedSocket)
		if !ok {
			t.Fatalf("reader fixture socket = %T, want *espFormScriptedSocket", first.conn)
		}
		close(failedSocket.fail)
		r.wg.Add(1)
		r.read(first.conn, first.tun, life)
		select {
		case <-life.dead:
		default:
			t.Fatal("the actual read producer did not record its socket failure")
		}
		if err := r.Watch(spi+1, natTarget(peer, local)); err != nil {
			t.Fatalf("Watch replaces the failed reader: %v", err)
		}
		select {
		case <-life.stop:
		default:
			t.Fatal("Watch did not retire the failed reader")
		}
		// As at startReaderLocked, the caller MUST add the hold before revive; here
		// its delayed start makes Watch's otherwise scheduler-dependent lead exact.
		r.revivers.Add(1)
		r.revive(life)
		requireESPFormServed(t, replacement, spi, local)
		if err := r.Close(); err != nil {
			t.Fatalf("join the replacement reader and all revive workers: %v", err)
		}
		if opens != 1 {
			t.Fatalf("iteration %d: opened %d replacement paths, want exactly one", attempt, opens)
		}
		if len(events) != 1 {
			t.Fatalf("iteration %d: recorded failure produced %d events after retirement, want exactly one", attempt, len(events))
		}
		stopped := <-events
		if !errors.Is(stopped.Err, errESPFormSocketFailed) {
			t.Errorf("reported %v, want the actual socket failure", stopped.Err)
		}
		if stopped.At.IsZero() {
			t.Error("the failure event has no time")
		}
	}
}

// TestESPFormReceiverIntentionalRetirementHasNoFailureEvent proves closing a wanted
// reader intentionally stays silent even when its read and revive run after retirement.
// METHOD: retire through the real transition, close the scripted socket, then run read
// and revive in order. read must leave dead open and revive must neither report nor open.
func TestESPFormReceiverIntentionalRetirementHasNoFailureEvent(t *testing.T) {
	opens := 0
	r := espFormTestReceiver(t, func() (espFormPaths, error) {
		opens++
		return espFormPaths{}, errors.New("test: intentional retirement reopened paths")
	})
	events := make(chan ESPFormReaderStopped, 2)
	r.readerStopped = func(stopped ESPFormReaderStopped) { events <- stopped }
	paths := newIdleESPFormPaths()
	life := espFormReaderBeforeRevive(r, paths)
	r.mu.Lock()
	r.retireLocked()
	r.mu.Unlock()
	if err := paths.conn.Close(); err != nil {
		t.Fatalf("close the intentionally retired socket: %v", err)
	}
	r.wg.Add(1)
	r.read(paths.conn, paths.tun, life)
	select {
	case <-life.dead:
		t.Fatal("an intentional socket close was recorded as a reader failure")
	default:
	}
	r.revivers.Add(1)
	r.revive(life)
	if len(events) != 0 {
		t.Errorf("intentional retirement produced %d failure events", len(events))
	}
	if opens != 0 {
		t.Errorf("intentional retirement attempted %d opens", opens)
	}
}

// TestESPFormReceiverRetiredLifeCannotReopen proves the lock-protected recovery guard
// refuses stale work independently of revive's stop-channel cancellation.
// METHOD: retire the real current life, then call reviveLocked and observe no open,
// retry or backoff change. This pins the guard needed after reporting a retired death.
func TestESPFormReceiverRetiredLifeCannotReopen(t *testing.T) {
	opens := 0
	r := espFormTestReceiver(t, func() (espFormPaths, error) {
		opens++
		return espFormPaths{}, errors.New("test: stale recovery opened paths")
	})
	life := espFormReaderBeforeRevive(r, newIdleESPFormPaths())
	r.reopenWait = time.Second
	r.mu.Lock()
	r.retireLocked()
	retry, err := r.reviveLocked(life)
	r.mu.Unlock()
	if err != nil {
		t.Errorf("stale recovery returned %v, want no error", err)
	}
	if retry {
		t.Error("stale recovery requested another attempt")
	}
	if opens != 0 {
		t.Errorf("stale recovery attempted %d opens", opens)
	}
	if r.reopenWait != time.Second {
		t.Errorf("stale recovery changed backoff to %s", r.reopenWait)
	}
}

// TestESPFormReceiverMatureRetiredLifeKeepsBackoff proves reporting an old reader's
// failure cannot reset the replacement's recovery backoff.
// METHOD: age the old life past the reset threshold, run read, then let Watch replace
// it before revive runs. The recorded failure must be reported, the replacement must
// serve its SA, and its backoff must survive the delayed old worker unchanged.
// MUTATION: omit revive's life identity guard before reopenWaitLocked: the mature old
// life resets the receiver's backoff to zero after reporting its failure.
func TestESPFormReceiverMatureRetiredLifeKeepsBackoff(t *testing.T) {
	const spi = 0x5E0F0001
	peer, local := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("203.0.113.9")
	first, replacement := newIdleESPFormPaths(), newIdleESPFormPaths()
	opens := 0
	r := espFormTestReceiver(t, func() (espFormPaths, error) {
		opens++
		if opens != 1 {
			return espFormPaths{}, errors.New("test: a mature retired reader attempted another open")
		}
		return replacement, nil
	})
	events := make(chan ESPFormReaderStopped, 2)
	r.readerStopped = func(stopped ESPFormReaderStopped) { events <- stopped }
	life := espFormReaderBeforeRevive(r, first)
	life.opened = time.Now().Add(-2 * espFormReopenResetAfter)
	r.reg.watch(spi, natTarget(peer, local))
	failedSocket, ok := first.conn.(*espFormScriptedSocket)
	if !ok {
		t.Fatalf("reader fixture socket = %T, want *espFormScriptedSocket", first.conn)
	}
	close(failedSocket.fail)
	r.wg.Add(1)
	r.read(first.conn, first.tun, life)
	if err := r.Watch(spi+1, natTarget(peer, local)); err != nil {
		t.Fatalf("Watch replaces the mature failed reader: %v", err)
	}
	r.mu.Lock()
	r.reopenWait = espFormReopenWaitMax
	r.mu.Unlock()
	// The caller MUST hold the revive count before starting the delayed old worker.
	r.revivers.Add(1)
	r.revive(life)
	requireESPFormServed(t, replacement, spi, local)
	if err := r.Close(); err != nil {
		t.Fatalf("join the replacement reader and all revive workers: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("mature retired failure produced %d events, want exactly one", len(events))
	}
	if stopped := <-events; !errors.Is(stopped.Err, errESPFormSocketFailed) {
		t.Errorf("reported %v, want the mature reader's actual socket failure", stopped.Err)
	}
	if opens != 1 {
		t.Errorf("opened %d replacement paths, want exactly one", opens)
	}
	if r.reopenWait != espFormReopenWaitMax {
		t.Errorf("mature retired reader changed replacement backoff to %s, want %s", r.reopenWait, espFormReopenWaitMax)
	}
}
