// VALIDATES: AC-9 of spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff --
// waitForPADO paces its retries on the discovery read's own blocking
// duration (SO_RCVTIMEO on the real socket) rather than on a
// runtime.Gosched() spin, and it still checks the stop signal between two
// read attempts rather than after several of them. The discovery deadline
// that ends the wait (AC-9's second half) is untouched by this change: the
// `time.NewTimer(discoveryTimeout)` construction in waitForPADO is not
// part of the edit this test exercises, so it is not re-proven with a
// second, ten-second-long test here.
// PREVENTS: a read call that returns without blocking (a misconfigured
// socket, or a future edit that retries internally before rechecking
// stopCh) turning this loop into the busy loop this spec removes from the
// other four subscriber receiver loops.

package pppoeclient

import (
	"errors"
	"testing"
	"time"
)

var errFakeNoDiscoveryFrame = errors.New("pppoeclient test: no frame available")

// TestWaitForPADOBlocksRatherThanSpins drives waitForPADO against a fake
// readDiscoveryFrame that sleeps for readDelay on every call, the same
// shape SO_RCVTIMEO gives the real socket when no frame arrives, and closes
// stopCh after stopAfter. The loop must return within about one blocking
// read call of the stop signal, not several: a read that retried
// internally before rechecking stopCh, or a stray yield-based spin loop
// layered on top of it, would push the return well past readDelay.
func TestWaitForPADOBlocksRatherThanSpins(t *testing.T) {
	const readDelay = 25 * time.Millisecond
	// Well under readDelay, so stop always fires while the very first read
	// call is still in flight: the loop can notice it only once that one
	// call returns, whether or not a later read is layered on top of it.
	const stopAfter = 5 * time.Millisecond

	orig := readDiscoveryFrame
	readDiscoveryFrame = func(_ int, _ []byte) (int, int, error) {
		time.Sleep(readDelay)
		return 0, 0, errFakeNoDiscoveryFrame
	}
	t.Cleanup(func() { readDiscoveryFrame = orig })

	stopCh := make(chan struct{})
	go func() {
		time.Sleep(stopAfter)
		close(stopCh)
	}()

	start := time.Now()
	pkt, err := waitForPADO(0, 0, [4]byte{}, "", stopCh)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("waitForPADO returned pkt %+v with no error after stop fired", pkt)
	}

	// One in-flight read call is allowed to be running when stop fires, so
	// the loop notices it only once that call returns. More than one call's
	// worth of slack means the loop is not checking stop between reads.
	slack := elapsed - stopAfter
	const maxSlack = readDelay + 40*time.Millisecond // scheduler jitter margin
	if slack < 0 || slack > maxSlack {
		t.Errorf("waitForPADO took %v to return after stop fired at %v (%v of slack, want at most %v): "+
			"the wait is not pacing on the blocking read alone", elapsed, stopAfter, slack, maxSlack)
	}
}
