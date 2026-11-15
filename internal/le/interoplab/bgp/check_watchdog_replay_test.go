// Design: docs/architecture/testing/interop.md -- replay the registered checker over a real lab recording
// Related: check_watchdog_withdraw_value.go -- causal completion fence

package bgp

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestWatchdogWithdrawValueReplaysTheLabRecording runs the registered checker
// over the passing FRR recording. Replay matches every call in order, including
// credited readiness, quiesce, fresh sentinel receipt and held-pool commands.
// VALIDATES: the whole checker reaches the recorded passing verdict without
// skipping or inventing a lab call. Predicate polarity stays in its unit tests.
// PREVENTS: a checker change judged only through its isolated predicates.
func TestWatchdogWithdrawValueReplaysTheLabRecording(t *testing.T) {
	checkout, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join("testdata", scenarioWatchdogWithdrawValueFRR)
	recordings, err := filepath.Glob(filepath.Join(directory, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recordings) != 1 {
		t.Fatalf("%s holds %d recordings, want exactly one passing lab recording", directory, len(recordings))
	}
	result, err := interoplab.ReplayScenario(t.Context(), checkout, recordings[0])
	if err != nil {
		t.Fatalf("replay of %s: %v", recordings[0], err)
	}
	if !result.Lab.Passed {
		t.Fatalf("the committed recording is of a failed lab run: %s", result.Lab.Error)
	}
	if result.Replay != result.Lab {
		t.Errorf("replayed verdict %#v differs from the recorded lab verdict %#v", result.Replay, result.Lab)
	}
}
