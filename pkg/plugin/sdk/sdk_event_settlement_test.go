// Design: docs/architecture/api/process-protocol.md — applied event settlement.
// Related: sdk_event_settlement.go — off-loop settlement worker.

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const eventSettlementProbeMethod = "ze-plugin-callback:event-settlement"

// registerEventSettlementProbe registers the actual public SDK handler after
// main observed the ordinary-loop baseline block its required later event.
func registerEventSettlementProbe(p *Plugin, handler func(context.Context) error) {
	p.OnEventSettlement(handler)
}

func eventSettlementProbeCall(ctx context.Context, h *sourcePolicyHarness) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("settlement probe requires a deadline")
	}
	input := map[string]any{"deadline-ms": deadline.UnixMilli()}
	var output json.RawMessage
	var err error
	if h.bridged {
		var body json.RawMessage
		body, err = json.Marshal(input)
		if err != nil {
			return err
		}
		output, err = h.bridge.SendCallback(ctx, eventSettlementProbeMethod, body)
	} else {
		output, err = h.engine.mux.CallRPC(ctx, eventSettlementProbeMethod, input)
	}
	if err != nil {
		return err
	}
	var result rpc.EventSettlementOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return err
	}
	if err := result.Validate(rpc.EventSettlementInput{DeadlineMS: deadline.UnixMilli()}); err != nil {
		return err
	}
	if result.Status != rpc.StatusOK {
		return errors.New(result.Error)
	}
	return nil
}

// TestEventSettlementAllowsNestedEngineRPCAndEvents holds settlement on an
// engine answer that itself requires a later delivered event. Both actual SDK
// loops must apply that event while settlement waits and acknowledge only after
// the engine returns, proving the handler does not run on the callback loop.
func TestEventSettlementAllowsNestedEngineRPCAndEvents(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
		defer cancel()
		h := newSourcePolicyHarness(t, bridged)
		prior := make(chan struct{})
		later := make(chan struct{})
		var priorOnce, laterOnce sync.Once
		h.p.OnEvent(func(event string) error {
			switch event {
			case "prior":
				priorOnce.Do(func() { close(prior) })
			case "later":
				laterOnce.Do(func() { close(later) })
			}
			return nil
		})
		engineEntered := make(chan struct{})
		h.engineRPC = func(ctx context.Context) error {
			close(engineEntered)
			select {
			case <-later:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		registerEventSettlementProbe(h.p, func(ctx context.Context) error {
			select {
			case <-prior:
			default:
				return errors.New("settlement preceded the earlier event")
			}
			return h.p.callEngine(ctx, "settlement-probe", map[string]any{})
		})
		h.start(t, ctx)
		if err := h.event(ctx, "prior"); err != nil {
			t.Fatal(err)
		}
		settlementCtx, settlementCancel := context.WithTimeout(ctx, 3*time.Second)
		defer settlementCancel()
		result := make(chan error, 1)
		go func() { result <- eventSettlementProbeCall(settlementCtx, h) }()
		select {
		case <-engineEntered:
		case err := <-result:
			t.Fatalf("settlement ended before its nested engine request: %v", err)
		case <-ctx.Done():
			t.Fatal("settlement never reached its nested engine request")
		}
		select {
		case err := <-result:
			t.Fatalf("settlement acknowledged before downstream work: %v", err)
		default:
		}
		eventCtx, eventCancel := context.WithTimeout(ctx, time.Second)
		defer eventCancel()
		if err := h.event(eventCtx, "later"); err != nil {
			t.Fatalf("settlement blocked the event needed by its nested request: %v", err)
		}
		select {
		case err := <-result:
			if err != nil {
				t.Fatalf("settlement failed after the event and engine answer: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("settlement receipt never arrived")
		}
	})
}
