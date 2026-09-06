package bridgerun

import (
	"testing"
	"time"
)

// VALIDATES: the restart limit is ExaBGP's -- respawnMax forks inside one
// 64-second window, counting the first fork, with the count reset when the
// window moves.
// PREVENTS: a script that dies at once being forked forever.

// TestRespawnLimiterAllowsFiveForksPerWindow pins the count ExaBGP allows:
// respawn_number is 5, and the sixth fork inside one bucket is refused
// (src/exabgp/reactor/api/processes.py, Processes._start).
func TestRespawnLimiterAllowsFiveForksPerWindow(t *testing.T) {
	var l respawnLimiter
	now := time.Unix(1_700_000_000, 0)

	for fork := 1; fork <= respawnMax; fork++ {
		if !l.count(now) {
			t.Fatalf("fork %d refused, want allowed", fork)
		}
	}
	if l.count(now) {
		t.Errorf("fork %d allowed, want refused", respawnMax+1)
	}
}

// TestRespawnLimiterResetsOnTheNextWindow verifies a script that died five
// times an hour ago still starts today.
func TestRespawnLimiterResetsOnTheNextWindow(t *testing.T) {
	var l respawnLimiter
	now := time.Unix(1_700_000_000, 0)

	for fork := 1; fork <= respawnMax+1; fork++ {
		l.count(now)
	}
	if !l.count(now.Add(respawnWindow)) {
		t.Errorf("the first fork of the next window was refused")
	}
}

// TestRespawnLimiterHoldsInsideOneWindow verifies the window is the clock
// bucket ExaBGP uses rather than a timer started at the first fork: two forks
// one second apart share a window.
func TestRespawnLimiterHoldsInsideOneWindow(t *testing.T) {
	var l respawnLimiter
	// 1_700_000_000 starts a 64-second bucket, so one second later is inside it.
	now := time.Unix(1_700_000_000, 0)

	for fork := 1; fork <= respawnMax; fork++ {
		l.count(now)
	}
	if l.count(now.Add(time.Second)) {
		t.Errorf("a fork one second later was allowed, want the same window to refuse it")
	}
}
