package topology

import (
	"errors"
	"testing"
)

// VALIDATES: the WAN SSH clause passes only on the bound ending the web
// server's connect, the drop the converted WAN input term makes; a completed
// handshake, a refused connect and an exec that never ran each fail.
// PREVENTS: the unanswered connect the clause exists to see (exit 124, which
// Exec returns as an error) read as a lab failure, and a reset read as a drop.
// Method: call the verdicts with each outcome.
func TestWANConnectVerdict(t *testing.T) {
	if err := wanConnectVerdict(nil); err == nil {
		t.Error("a completed handshake passed")
	}
	notRun := errors.New("docker exec ze-topo-web: no such container")
	if err := wanConnectVerdict(notRun); !errors.Is(err, notRun) {
		t.Errorf("an exec that never ran = %v, want it wrapped", err)
	}
	if err := wanExitVerdict(bareConnectUnanswered, notRun); err != nil {
		t.Errorf("an unanswered connect failed: %v", err)
	}
	if err := wanExitVerdict(1, notRun); err == nil {
		t.Error("a refused connect passed as a drop")
	}
}
