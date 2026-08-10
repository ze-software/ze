// VALIDATES: terminateScaffoldPeers (spec-fixit-rib-graph-ci-never-terminates) --
//            a sink/echo/inject peer is signaled at teardown so the drain barrier
//            reaps it at once, while a check-mode peer is left running to exit on
//            its own.
// PREVENTS: (1) the drain barrier burning its whole peerDrainGrace on a peer whose
//            only exit is a signal (a flat 10s on every --mode sink test);
//            (2) signaling a check peer, which would truncate the capture the
//            per-peer verdict reads (peer_contract.go failedCheckPeers).
//
// test-relax: TestTerminateScaffoldPeersSkipsWaitedPeer is gone because its
// subject is gone: terminateScaffoldPeers no longer waits a peer, so it needs no
// `waited` guard and there is no second Wait to skip. The test could not fail
// either way -- a second Wait returns "Wait was already called" and Signal on a
// reaped process returns ErrProcessDone, so neither arm hangs. The two tests
// below replace its coverage of the loop: each now fails when the term it checks
// is dropped from the predicate.

package runner

import (
	"testing"
	"time"
)

// stillRunningProbe is how long a peer that must NOT have been signaled is given
// to prove it is still there. A signaled `sleep` exits in milliseconds, so this
// only has to outlast that.
const stillRunningProbe = 200 * time.Millisecond

// mkScaffoldPeer builds a peerOutput around a real long-lived child process, as
// runOrchestrated does for every ze-peer it launches.
func mkScaffoldPeer(t *testing.T, checkMode bool) peerOutput {
	t.Helper()
	return peerOutput{
		stdout:    newSyncWriter(),
		stderr:    &lockedBuilder{},
		proc:      startSleeper(t),
		checkMode: checkMode,
	}
}

// TestTerminateScaffoldPeersReapsSinkPeer proves the whole gain: the sink peer is
// signaled, so the drain barrier that follows reaps it immediately instead of
// spending peerDrainGrace on a process that would never exit by itself.
//
// It fails if the loop body stops running: an unsignaled `sleep` outlives the
// real grace, so drainPeers returns only when the deadline in withinDeadline
// fires.
func TestTerminateScaffoldPeersReapsSinkPeer(t *testing.T) {
	peers := []peerOutput{mkScaffoldPeer(t, false)}

	start := time.Now()
	withinDeadline(t, "terminateScaffoldPeers(sink) then drainPeers", func() {
		terminateScaffoldPeers(peers)
		drainPeers(peers, peerDrainGrace)
	})
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("teardown took %s on a signaled peer, want an immediate return", elapsed)
	}
	if !peers[0].waited {
		t.Fatal("sink peer was not reaped by the drain barrier: waited is still false")
	}
	if peers[0].proc.ProcessState == nil {
		t.Fatal("sink peer was not reaped: ProcessState is nil after teardown")
	}
}

// TestTerminateScaffoldPeersLeavesCheckPeer proves the verdict path is untouched:
// a check-mode peer reports its own result, so teardown must not signal it.
//
// The assertion is that the peer is STILL RUNNING afterwards, which is what
// discriminates: drop checkMode from the predicate and the sleeper is signaled,
// exits in milliseconds, and the wait below completes.
func TestTerminateScaffoldPeersLeavesCheckPeer(t *testing.T) {
	peers := []peerOutput{mkScaffoldPeer(t, true)}

	terminateScaffoldPeers(peers)

	exited := make(chan struct{})
	go func() {
		_ = peers[0].proc.Wait() //nolint:errcheck // the test reads liveness, not status
		close(exited)
	}()
	select {
	case <-exited:
		t.Fatal("check peer exited at teardown: it must be left to exit on its own")
	case <-time.After(stillRunningProbe):
	}
}
