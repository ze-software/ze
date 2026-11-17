// Design: docs/architecture/api/process-protocol.md — settlement panic terminates its connection.
// Related: sdk_event_settlement.go — the worker aborts after reporting a handler panic.
package sdk

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestEventSettlementPanicEndsLoop observes the actual SDK loop's panic error
// while its parent context is still live. Cleanup cancellation cannot stand in
// for the connection failure on either transport.
func TestEventSettlementPanicEndsLoop(t *testing.T) {
	eachSourcePolicyTransport(t, func(t *testing.T, bridged bool) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		h := newSourcePolicyHarness(t, bridged)
		h.p.OnEventSettlement(func(context.Context) error { panic("settlement terminal probe") })
		terminal := make(chan error, 1)
		if bridged {
			go func() { terminal <- h.p.bridgeEventLoop(ctx) }()
		} else {
			go func() { terminal <- h.p.Run(ctx, &Registration{}) }()
		}
		joined := false
		// This test owns the loop and MUST cancel and join it on every failure.
		t.Cleanup(func() {
			cancel()
			if !joined {
				<-terminal
			}
		})
		if !bridged {
			completeStartup(t, ctx, h.engine)
		}

		if err := eventSettlementProbeCall(ctx, h); err == nil {
			t.Fatal("panicking handler acknowledged settlement")
		}
		select {
		case err := <-terminal:
			joined = true
			if contextErr := ctx.Err(); contextErr != nil {
				t.Fatalf("loop ended only after its parent context: %v", contextErr)
			}
			if err == nil {
				t.Fatal("loop ended without the handler panic")
			}
			if !strings.Contains(err.Error(), "event settlement handler panic") {
				t.Fatalf("loop ended with %v, want the settlement handler panic", err)
			}
			if !strings.Contains(err.Error(), "settlement terminal probe") {
				t.Fatalf("loop lost the handler's panic cause: %v", err)
			}
		case <-ctx.Done():
			t.Fatal("loop did not end before its parent context after the handler panic")
		}
	})
}
