// Design: docs/architecture/api/process-protocol.md -- bounded event frames.
// Related: rib_event_sdk.go -- sdkEventChunkEnd cuts a frame at ribevents.FrameEntriesMax.
package rib

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSDKEventBusFrameCarriesAtMostFrameEntriesMax proves a forked RIB's frame
// is bounded by its entry count as well as its bytes.
// VALIDATES: a best-change family far below the byte budget but above
// ribevents.FrameEntriesMax entries leaves in frames of at most that many
// entries, every entry once and in order, with its batch metadata.
// PREVENTS: one byte-bounded frame whose subscriber work outlasts the
// per-frame emit deadline.
func TestSDKEventBusFrameCarriesAtMostFrameEntriesMax(t *testing.T) {
	batch := &ribevents.BestChangeBatch{Protocol: "bgp", Family: family.IPv4Unicast,
		Changes: make([]ribevents.BestChangeEntry, 2*ribevents.FrameEntriesMax+1)}
	for i := range batch.Changes {
		batch.Changes[i] = ribevents.BestChangeEntry{Action: routeaction.Add, Prefix: burstPrefix(i), Metric: uint32(i)}
	}
	seen := 0
	frames := 0
	bus := burstSDKBus(t, func(input rpc.EmitEventInput) error {
		var got ribevents.BestChangeBatch
		if err := json.Unmarshal([]byte(input.Event), &got); err != nil {
			return err
		}
		if got.Protocol != batch.Protocol || got.Family != batch.Family {
			return fmt.Errorf("frame %d changed its batch metadata", frames)
		}
		if len(got.Changes) == 0 || len(got.Changes) > ribevents.FrameEntriesMax {
			return fmt.Errorf("frame %d carries %d entries, want 1 to %d", frames, len(got.Changes), ribevents.FrameEntriesMax)
		}
		for i := range got.Changes {
			if seen >= len(batch.Changes) || got.Changes[i].Prefix != burstPrefix(seen) || got.Changes[i].Metric != uint32(seen) {
				return fmt.Errorf("entry %d lost, duplicated or reordered", seen)
			}
			seen++
		}
		frames++
		return nil
	})
	if _, err := bus.Emit(ribevents.Namespace, ribevents.EventBestChange, batch); err != nil {
		t.Fatal(err)
	}
	if frames != 3 || seen != len(batch.Changes) {
		t.Fatalf("delivered %d/%d entries in %d frames, want all in 3", seen, len(batch.Changes), frames)
	}
}
