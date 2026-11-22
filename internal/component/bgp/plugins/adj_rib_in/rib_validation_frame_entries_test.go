// Design: docs/architecture/api/process-protocol.md -- bounded event frames.
// Related: rib_eligibility.go -- emitValidationFrames cuts a frame at ribevents.FrameEntriesMax.
package adj_rib_in

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestValidationFrameCarriesAtMostFrameEntriesMax proves a validation frame is
// bounded by its route count as well as its bytes.
// VALIDATES: a table far below the byte budget but above
// ribevents.FrameEntriesMax routes leaves in frames of at most that many
// routes, every route once and in order.
// PREVENTS: one byte-bounded frame of about 100k routes whose subscriber work
// outlasts the per-frame emit deadline.
func TestValidationFrameCarriesAtMostFrameEntriesMax(t *testing.T) {
	changes := deadlineValidationChanges()[:2*ribevents.FrameEntriesMax+1]
	var mu sync.Mutex
	var frames [][]ribevents.ValidationRoute
	r := deadlineValidationManager(t, func(input rpc.EmitEventInput) {
		var frame []ribevents.ValidationRoute
		if err := json.Unmarshal([]byte(input.Event), &frame); err != nil {
			t.Errorf("validation frame does not decode: %v", err)
		}
		mu.Lock()
		frames = append(frames, frame)
		mu.Unlock()
	})
	if err := r.emitValidationFrames(validationEmitTimeout, changes); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(frames) != 3 {
		t.Fatalf("%d routes left in %d frames, want 3 of at most %d", len(changes), len(frames), ribevents.FrameEntriesMax)
	}
	next := 0
	for index, frame := range frames {
		if len(frame) == 0 || len(frame) > ribevents.FrameEntriesMax {
			t.Fatalf("frame %d carries %d routes, want 1 to %d", index, len(frame), ribevents.FrameEntriesMax)
		}
		for _, route := range frame {
			if next >= len(changes) || route != changes[next] {
				t.Fatalf("route %d lost, duplicated or reordered", next)
			}
			next++
		}
	}
	if next != len(changes) {
		t.Fatalf("delivered %d/%d routes", next, len(changes))
	}
}
