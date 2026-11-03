// Design: docs/architecture/testing/runner-architecture.md -- cooperative editor ownership.

package testing

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestEditorOwnedRunCancellation checks the cooperative API reports its deadline
// even when initialization consumes the remaining budget.
func TestEditorOwnedRunCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	result := RunETContentIn(ctx, "option=mode:value=command\nwait=ms:60000\n", t.TempDir())
	if result.Passed || !strings.Contains(result.Error, "deadline exceeded") {
		t.Fatalf("expected deadline failure, got %+v", result)
	}
}

// TestEditorOwnedRunClosesModels uses an expectation to retain the constructed
// model and cancel the driver, then checks closure after the API returns.
func TestEditorOwnedRunClosesModels(t *testing.T) {
	const observer = "owned-close-proof"
	previous, existed := validExpectationTypes[observer]
	t.Cleanup(func() {
		if existed {
			validExpectationTypes[observer] = previous
		} else {
			delete(validExpectationTypes, observer)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var model *headlessModel
	validExpectationTypes[observer] = func(_ Expectation, state State) error {
		model = state.(*headlessModel)
		cancel()
		return nil
	}
	result := RunETContentIn(ctx, "option=mode:value=command\nexpect=owned-close-proof\nwait=ms:60000\n", t.TempDir())
	if result.Passed || !strings.Contains(result.Error, "context canceled") {
		t.Fatalf("expected cancellation, got %+v", result)
	}
	if model == nil {
		t.Fatal("authored observer never ran")
	}
	select {
	case <-model.stopped:
	default:
		t.Fatal("result published before model closure")
	}
	if len(model.pending) != 0 {
		t.Fatal("result retained pending timer work")
	}
}
