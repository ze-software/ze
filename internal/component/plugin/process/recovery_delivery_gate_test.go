// Design: docs/architecture/api/process-protocol.md -- FIFO delivery and drain ownership.
// Related: recovery_delivery_export_test.go -- test-only recovery gate.
package process

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRecoveryDeliveryGateRetainsCancelledBarrier proves the test seam itself
// cannot lose a queued Result, turn a canceled drain into a lost barrier, or
// strand the original deliveryLoop when Stop closes the replacement queue.
func TestRecoveryDeliveryGateRetainsCancelledBarrier(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	proc := NewProcess(plugin.PluginConfig{Name: "recovery-delivery-gate"})
	var delivered []string
	proc.bridge = rpc.NewDirectBridge()
	proc.bridge.SetDeliverEvents(func(batch []string) error {
		delivered = append(delivered, batch...)
		return nil
	})
	proc.bridge.SetReady()
	proc.StartDelivery(ctx)
	gate := GateRecoveryDelivery(t, ctx, proc, func(item EventDelivery) bool { return item.Output == "held" })
	t.Cleanup(func() {
		gate.Release()
		proc.Stop()
		join, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := gate.Wait(join); err != nil {
			t.Error(err)
		}
		if err := proc.Wait(join); err != nil {
			t.Error(err)
		}
	})
	first, second := make(chan EventResult, 1), make(chan EventResult, 1)
	if !proc.Deliver(EventDelivery{Output: "held", Result: first}) {
		t.Fatal("held event was not admitted")
	}
	select {
	case <-gate.Held:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if !proc.Deliver(EventDelivery{Output: "following", Result: second}) {
		t.Fatal("following event was not admitted")
	}
	drain, cancelDrain := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- proc.DrainEvents(drain) }()
	select {
	case <-gate.Barriers:
	case <-ctx.Done():
		cancelDrain()
		t.Fatal(ctx.Err())
	}
	cancelDrain()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled drain returned %v", err)
		}
	case <-ctx.Done():
		t.Fatal("canceled drain did not join", ctx.Err())
	}
	gate.Release()
	// A fresh barrier must still follow the canceled one and both Results.
	if err := proc.DrainEvents(ctx); err != nil {
		t.Fatal(err)
	}
	// Stop closes only the replacement queue; the proxy owns the original.
	proc.Stop()
	if err := gate.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if err := proc.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(delivered, []string{"held", "following"}) {
		t.Fatalf("delivery changed order or leaked a barrier: %q", delivered)
	}
	for _, answer := range []<-chan EventResult{first, second} {
		select {
		case got := <-answer:
			if got.Err != nil {
				t.Error(got.Err)
			}
			if len(answer) != 0 {
				t.Error("admitted event received duplicate Results")
			}
		default:
			t.Error("admitted event lost its Result")
		}
	}
}
