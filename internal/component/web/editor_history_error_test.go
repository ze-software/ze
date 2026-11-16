// Design: docs/architecture/web-interface.md -- browser recall failure ownership.
package web

import (
	"errors"
	"strings"
	"testing"
)

// TestHistorySnapshotFailureKeepsEveryCause proves browser recall normalizes
// a joined storage failure without selecting only its first actionable cause.
func TestHistorySnapshotFailureKeepsEveryCause(t *testing.T) {
	read := errors.New("saved configuration history could not be read")
	write := errors.New("saved operational history could not be written")
	refusal := historySnapshotFailure(errors.Join(read, write))
	if !errors.Is(refusal, read) {
		t.Fatalf("recall refusal lost the read cause: %v", refusal)
	}
	if !errors.Is(refusal, write) {
		t.Fatalf("recall refusal lost the write cause: %v", refusal)
	}
	for _, evidence := range []string{read.Error(), write.Error()} {
		if !strings.Contains(refusal.Error(), evidence) {
			t.Fatalf("recall refusal lost diagnostic %q: %v", evidence, refusal)
		}
	}
	if !strings.HasSuffix(refusal.Error(), "fix: Reconnect after the configuration store is available to restore command recall") {
		t.Fatalf("recall refusal has no final recovery action: %v", refusal)
	}
}
