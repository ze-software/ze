// Design: docs/architecture/testing/runner-architecture.md -- peer drain ownership.

package runner

import (
	"errors"
	"os/exec"
	"testing"
	"time"
)

// TestPeerDrainCompletedOutcomes checks absent and previously joined peers beside
// an unconsumed exit. Draining must preserve the child's own status and mark only
// the newly joined peer; no timeout or signal supplies that status.
func TestPeerDrainCompletedOutcomes(t *testing.T) {
	p := startOwnedTestProcess(t, exec.CommandContext(t.Context(), "sh", "-c", "exit 29"))
	<-p.done
	peers := []peerOutput{{}, {proc: p, waited: true}, {proc: p}}
	drainPeers(peers, time.Hour)
	if peers[0].waited {
		t.Error("absent process was marked waited")
	}
	if !peers[1].waited {
		t.Error("previously joined peer lost its verdict marker")
	}
	if !peers[2].waited {
		t.Error("completed peer was not joined")
	}
	var exit *exec.ExitError
	if !errors.As(p.Wait(), &exit) {
		t.Fatalf("exit = %v, want native exit 29", p.Wait())
	}
	if exit.ExitCode() != 29 {
		t.Fatalf("exit = %d, want native exit 29", exit.ExitCode())
	}
}
