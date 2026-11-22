// Design: docs/architecture/api/process-protocol.md -- bounded event frames.
// Related: rib_eligibility.go -- emitValidationFrames gives each frame its own deadline.
package adj_rib_in

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestValidationEmitDeadEngineFailsWithinOneFrame proves the dead-connection
// bound survives the per-frame deadline.
// VALIDATES: an engine that receives a mid-table frame and never answers fails
// the hand-off at that frame's own deadline, keeping the acknowledged prefix.
// PREVENTS: a receive store blocked forever on a dead engine connection.
func TestValidationEmitDeadEngineFailsWithinOneFrame(t *testing.T) {
	const frameTimeout = 3 * time.Second
	changes := deadlineValidationChanges()
	release := make(chan struct{})
	var calls atomic.Int64
	r := deadlineValidationManager(t, func(rpc.EmitEventInput) {
		if calls.Add(1) > 1 {
			<-release // The engine holds the frame and never answers it.
		}
	})
	// Registered after the engine's own cleanup, so it runs first and frees it.
	t.Cleanup(func() { close(release) })
	start := time.Now()
	err := r.emitValidationFrames(frameTimeout, changes)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an unanswered frame ended with %v, want its own deadline", err)
	}
	failure, ok := errors.AsType[*validationEmissionError](err)
	if !ok {
		t.Fatalf("failure %v does not report its acknowledged prefix", err)
	}
	if failure.acknowledged == 0 || failure.acknowledged >= len(changes) || calls.Load() != 2 {
		t.Fatalf("acknowledged %d/%d in %d frames, want the first frame only", failure.acknowledged, len(changes), calls.Load())
	}
	if elapsed < frameTimeout {
		t.Fatalf("hand-off failed after %v, before the %v frame deadline", elapsed, frameTimeout)
	}
}

// TestValidationEmitLiveEngineOutlastsOneFrameDeadline proves the deadline is
// per frame, never per hand-off.
// VALIDATES: an engine that acknowledges every frame, each inside the frame
// deadline but together beyond it, receives the whole table.
// PREVENTS: one deadline over the whole table cutting off a live engine that
// acknowledges a large table more slowly than one frame's bound.
func TestValidationEmitLiveEngineOutlastsOneFrameDeadline(t *testing.T) {
	const frameTimeout = 4 * time.Second
	const ackDelay = frameTimeout * 11 / 20
	changes := deadlineValidationChanges()
	var calls atomic.Int64
	r := deadlineValidationManager(t, func(rpc.EmitEventInput) {
		if calls.Add(1) <= 2 {
			// sleep(timeout-under-test): two acknowledgements of 0.55 of the
			// frame deadline each add up past one deadline while each frame
			// still answers inside its own.
			time.Sleep(ackDelay)
		}
	})
	start := time.Now()
	err := r.emitValidationFrames(frameTimeout, changes)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("a live engine acknowledging every frame was cut off: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("fixture: the table crossed %d frames, want several", calls.Load())
	}
	if elapsed <= frameTimeout {
		t.Fatalf("fixture: the hand-off took %v, inside one %v frame deadline, so it cannot tell a per-frame deadline from a per-hand-off one", elapsed, frameTimeout)
	}
}

// deadlineValidationChanges is a validation table of three frames, cut by
// ribevents.FrameEntriesMax as TestValidationFrameCarriesAtMostFrameEntriesMax
// proves.
func deadlineValidationChanges() []ribevents.ValidationRoute {
	changes := make([]ribevents.ValidationRoute, 3*ribevents.FrameEntriesMax)
	for i := range changes {
		changes[i] = ribevents.ValidationRoute{ConnectionID: 41,
			Peer: netip.MustParseAddr("2001:db8::1"), PeerOwner: 1, Family: family.IPv6Unicast,
			Prefix: netip.MustParsePrefix("2001:db8:1234:5678::/64"), PathID: uint32(i)}
	}
	return changes
}

// deadlineValidationManager has one owned engine worker over a real RPC
// connection. It calls consume for each emit-event request before answering
// it; cleanup closes both ends and joins that worker.
func deadlineValidationManager(t *testing.T, consume func(rpc.EmitEventInput)) *AdjRIBInManager {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	plugin := sdk.NewWithConn("validation-deadline-test", pluginEnd)
	engine := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	ctx, cancel := context.WithCancel(t.Context())
	var worker sync.WaitGroup
	worker.Go(func() {
		for request := range engine.Requests() {
			var input rpc.EmitEventInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				t.Errorf("emit-event request does not decode: %v", err)
			}
			consume(input)
			if err := engine.SendResult(ctx, request.ID, &rpc.EmitEventOutput{Delivered: 1}); err != nil {
				return
			}
		}
	})
	t.Cleanup(func() { cancel(); _ = plugin.Close(); _ = engine.Close(); worker.Wait() })
	r := newTestManager(t)
	r.plugin = plugin
	return r
}
