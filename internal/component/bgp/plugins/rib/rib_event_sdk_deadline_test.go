// Design: docs/architecture/api/process-protocol.md -- bounded event frames.
// Related: rib_event_sdk.go -- sdkEventBus.emitFrames gives each frame its own deadline.
package rib

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSDKEventBusDeadEngineFailsWithinOneFrame proves the dead-connection
// bound survives the per-frame deadline.
// VALIDATES: an engine that receives a mid-family frame and never answers
// fails the hand-off at that frame's own deadline, with the acknowledged
// prefix reported and no later frame sent.
// PREVENTS: a forked RIB blocked forever on a dead engine connection.
func TestSDKEventBusDeadEngineFailsWithinOneFrame(t *testing.T) {
	const frameTimeout = 3 * time.Second
	batch := deadlineBestChanges()
	release := make(chan struct{})
	var calls atomic.Int64
	bus := burstSDKBus(t, func(rpc.EmitEventInput) error {
		if calls.Add(1) > 1 {
			<-release // The engine holds the frame and never answers it.
		}
		return nil
	})
	// Registered after burstSDKBus, so it runs first and frees its worker.
	t.Cleanup(func() { close(release) })
	start := time.Now()
	delivered, err := bus.emitFrames(frameTimeout, ribevents.Namespace, ribevents.EventBestChange, batch)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an unanswered frame ended with %v, want its own deadline", err)
	}
	if delivered != 1 || calls.Load() != 2 {
		t.Fatalf("delivered %d in %d frames, want the first frame acknowledged and the second unanswered", delivered, calls.Load())
	}
	if elapsed < frameTimeout {
		t.Fatalf("hand-off failed after %v, before the %v frame deadline", elapsed, frameTimeout)
	}
}

// TestSDKEventBusLiveEngineOutlastsOneFrameDeadline proves the deadline is per
// frame, never per hand-off.
// VALIDATES: an engine that acknowledges every frame, each inside the frame
// deadline but together beyond it, receives the whole family.
// PREVENTS: one deadline over the whole family cutting off a live engine
// that acknowledges a large table more slowly than one frame's bound.
func TestSDKEventBusLiveEngineOutlastsOneFrameDeadline(t *testing.T) {
	const frameTimeout = 4 * time.Second
	const ackDelay = frameTimeout * 11 / 20
	batch := deadlineBestChanges()
	calls := 0
	bus := burstSDKBus(t, func(rpc.EmitEventInput) error {
		calls++
		if calls <= 2 {
			// sleep(timeout-under-test): two acknowledgements of 0.55 of the
			// frame deadline each add up past one deadline while each frame
			// still answers inside its own.
			time.Sleep(ackDelay)
		}
		return nil
	})
	start := time.Now()
	delivered, err := bus.emitFrames(frameTimeout, ribevents.Namespace, ribevents.EventBestChange, batch)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("a live engine acknowledging every frame was cut off: %v", err)
	}
	if calls < 2 || delivered != calls {
		t.Fatalf("delivered %d of %d frames, want every frame of a multi-frame family", delivered, calls)
	}
	if elapsed <= frameTimeout {
		t.Fatalf("fixture: the hand-off took %v, inside one %v frame deadline, so it cannot tell a per-frame deadline from a per-hand-off one", elapsed, frameTimeout)
	}
}

// deadlineBestChanges is a best-change family wider than one frame, as
// TestSDKEventBusWholeFamilyFrames/best-change proves it splits.
func deadlineBestChanges() *ribevents.BestChangeBatch {
	batch := &ribevents.BestChangeBatch{Protocol: "bgp", Family: family.IPv4Unicast,
		Changes: make([]ribevents.BestChangeEntry, 400)}
	attributes := bytes.Repeat([]byte{0xab}, 65535)
	for i := range batch.Changes {
		batch.Changes[i] = ribevents.BestChangeEntry{Action: routeaction.Add,
			Prefix: burstPrefix(i), NextHop: netip.MustParseAddr("192.0.2.1"), Attributes: attributes}
	}
	return batch
}
