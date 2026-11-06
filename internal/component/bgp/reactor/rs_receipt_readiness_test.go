// Design: docs/architecture/core-design.md -- route-server forwarding receipt readiness.
package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestRSReceiptReplayFenceRequiresActualReports pins the admission state the
// native receipt fixture probes with control traffic. An EOR on the same
// established session is not readiness: each captured reporter must credit this
// session before the fast path owns a new UPDATE. An old receipt credits none.
func TestRSReceiptReplayFenceRequiresActualReports(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.resetAPISync([]string{"first", "second"})
	peer.takeInitialReplayReceipt()
	token := peer.session.initialReplay.Load()
	peer.mu.Lock()
	peer.replayFence = []string{"first", "second"}
	peer.initialUpdateOwed.Store(true)
	ready := peer.apiSyncReady
	peer.mu.Unlock()

	if _, err := peer.creditAPIReady("first", token+1); err == nil {
		t.Fatal("another session's report was accepted")
	}
	if !peer.replayOwns(token + 1) {
		t.Fatal("fast path owned an UPDATE before the real reports")
	}
	if lowered, err := peer.creditAPIReady("first", token); err != nil || lowered {
		t.Fatalf("first report lowered=%v error=%v, want incomplete fence", lowered, err)
	}
	select {
	case <-ready:
		t.Fatal("completion channel closed before every actual report")
	default:
	}
	if !peer.replayOwns(token + 1) {
		t.Fatal("one reporter released another reporter's fence")
	}
	if lowered, err := peer.creditAPIReady("second", token); err != nil || !lowered {
		t.Fatalf("last report lowered=%v error=%v, want completed fence", lowered, err)
	}
	select {
	case <-ready:
	default:
		t.Fatal("last report did not publish completion")
	}
	if peer.replayOwns(token + 1) {
		t.Fatal("completed replay kept the new UPDATE off the fast path")
	}
	if !peer.replayOwns(token - 1) {
		t.Fatal("completion let an UPDATE older than establishment bypass replay")
	}
}
