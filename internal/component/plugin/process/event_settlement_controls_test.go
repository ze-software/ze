// Design: docs/architecture/api/process-protocol.md -- captured event settlement lifetime.
// Related: event_settlement_test.go -- useful deferred-work and later-event progress.
package process

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// newEventSettlingProcess owns the test process and one callback-serving worker.
// Cleanup MUST stop and join both, including the original captured connection
// if a test replaces it deliberately.
func newEventSettlingProcess(t *testing.T, typed, support bool, settle func(*Process, rpc.EventSettlementInput) (rpc.EventSettlementOutput, error)) (*Process, *atomic.Int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	proc := NewProcess(plugin.PluginConfig{Name: t.Name(), Encoder: "json"})
	proc.SetRegistration(&plugin.PluginRegistration{EventSettlement: support})
	engineEnd, pluginEnd := net.Pipe()
	var connection *ipc.PluginConn
	var client *rpc.MuxConn
	if typed {
		connection = ipc.NewPluginConn(engineEnd, engineEnd)
		proc.bridge = rpc.NewDirectBridge()
		proc.bridge.SetDeliverEvents(func([]string) error { return nil })
		connection.SetBridge(proc.bridge)
		proc.bridge.SetReady()
	} else {
		connection = ipc.NewMuxPluginConn(rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)))
		client = rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	}
	proc.SetConn(connection)
	calls := &atomic.Int64{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		// One test-owned worker serves its process lifetime, never one per call.
		for {
			if typed {
				select {
				case callback, ok := <-proc.bridge.CallbackCh():
					if !ok {
						return
					}
					var input rpc.EventSettlementInput
					if err := json.Unmarshal(callback.Params, &input); err != nil {
						callback.Result <- rpc.BridgeCallbackResult{Err: err}
						continue
					}
					calls.Add(1)
					output, err := settle(proc, input)
					if err != nil {
						callback.Result <- rpc.BridgeCallbackResult{Err: err}
						continue
					}
					data, err := json.Marshal(output)
					callback.Result <- rpc.BridgeCallbackResult{Data: data, Err: err}
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case request, ok := <-client.Requests():
				if !ok {
					return
				}
				var input rpc.EventSettlementInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					t.Errorf("decode settlement: %v", err)
					return
				}
				calls.Add(1)
				output, callbackErr := settle(proc, input)
				var err error
				if callbackErr != nil {
					err = client.SendError(ctx, request.ID, callbackErr.Error())
				} else {
					err = client.SendResult(ctx, request.ID, output)
				}
				if err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	proc.StartDelivery(ctx)
	t.Cleanup(func() {
		cancel()
		proc.Stop()
		connection.Close() //nolint:errcheck // Fixture also closes an original connection replaced by a test.
		if client != nil {
			client.Close() //nolint:errcheck // Fixture transport teardown.
		} else {
			pluginEnd.Close() //nolint:errcheck // Unused typed startup endpoint.
		}
		<-done
		wait, waitCancel := context.WithTimeout(context.Background(), time.Second)
		defer waitCancel()
		if err := proc.Wait(wait); err != nil {
			t.Errorf("join delivery: %v", err)
		}
	})
	return proc, calls
}

// TestSettleEventsAppliedWithoutDeclaration retains pure successful FIFO
// semantics for a plugin that did not declare asynchronous settlement.
func TestSettleEventsAppliedWithoutDeclaration(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(fmt.Sprintf("typed=%t", typed), func(t *testing.T) {
			proc, calls := newEventSettlingProcess(t, typed, false, func(*Process, rpc.EventSettlementInput) (rpc.EventSettlementOutput, error) {
				return rpc.EventSettlementOutput{}, errors.New("undeclared callback must not run")
			})
			if err := proc.SettleEventsApplied(t.Context()); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 0 {
				t.Fatal("undeclared settlement callback ran")
			}
		})
	}
}

// TestSettleEventsAppliedFailureDoesNotPoisonApplication separates a transient
// downstream failure from the lifetime-sticky application error on both rails.
func TestSettleEventsAppliedFailureDoesNotPoisonApplication(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(fmt.Sprintf("typed=%t", typed), func(t *testing.T) {
			var attempts atomic.Int64
			proc, calls := newEventSettlingProcess(t, typed, true, func(_ *Process, input rpc.EventSettlementInput) (rpc.EventSettlementOutput, error) {
				output := rpc.EventSettlementOutput{DeadlineMS: input.DeadlineMS, Status: rpc.StatusOK}
				if attempts.Add(1) == 1 {
					output.Status, output.Error = rpc.StatusError, "writer blocked"
				}
				return output, nil
			})
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			if err := proc.SettleEventsApplied(ctx); err == nil {
				t.Fatal("failed downstream settlement was acknowledged")
			} else if !strings.Contains(err.Error(), "writer blocked") {
				t.Fatalf("failure lost its cause: %v", err)
			}
			if err := proc.DrainEventsApplied(ctx); err != nil {
				t.Fatalf("transient settlement poisoned strict application: %v", err)
			}
			if err := proc.SettleEventsApplied(ctx); err != nil {
				t.Fatalf("recovered downstream work did not settle: %v", err)
			}
			if calls.Load() != 2 {
				t.Fatalf("settlement callbacks = %d, want 2", calls.Load())
			}
		})
	}
}

// TestSettleEventsAppliedRefusesEndedOwner returns a valid receipt after each
// original lifetime ends; none may authorize the captured drain as complete.
func TestSettleEventsAppliedRefusesEndedOwner(t *testing.T) {
	for _, typed := range []bool{false, true} {
		for _, end := range []string{"connection", "replacement", "engine", "stop"} {
			t.Run(fmt.Sprintf("typed=%t/%s", typed, end), func(t *testing.T) {
				engineDone := make(chan struct{})
				replacementEnd, replacementPeer := net.Pipe()
				replacement := ipc.NewPluginConn(replacementEnd, replacementEnd)
				replacementBridge := rpc.NewDirectBridge()
				replacementBridge.SetReady()
				replacement.SetBridge(replacementBridge)
				t.Cleanup(func() {
					replacement.Close()     //nolint:errcheck // Test-only successor connection.
					replacementPeer.Close() //nolint:errcheck // Test-only unused startup endpoint.
					replacementBridge.CloseCallbacks()
				})
				proc, calls := newEventSettlingProcess(t, typed, true, func(proc *Process, input rpc.EventSettlementInput) (rpc.EventSettlementOutput, error) {
					switch end {
					case "connection":
						proc.SetConn(nil)
					case "replacement":
						proc.SetConn(replacement)
					case "engine":
						close(engineDone)
					case "stop":
						proc.Stop()
					}
					return rpc.EventSettlementOutput{DeadlineMS: input.DeadlineMS, Status: rpc.StatusOK}, nil
				})
				proc.eventMu.Lock()
				proc.engineDone = engineDone
				proc.eventMu.Unlock()
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				if err := proc.SettleEventsApplied(ctx); err == nil {
					t.Fatal("ended owner acknowledged settlement")
				}
				if calls.Load() != 1 {
					t.Fatalf("original callback count = %d, want 1", calls.Load())
				}
			})
		}
	}
}

// TestSettleEventsAppliedRequiresCallerDeadline refuses an unbounded deferred
// wait before invoking a plugin that has declared asynchronous settlement.
func TestSettleEventsAppliedRequiresCallerDeadline(t *testing.T) {
	proc, calls := newEventSettlingProcess(t, true, true, func(*Process, rpc.EventSettlementInput) (rpc.EventSettlementOutput, error) {
		return rpc.EventSettlementOutput{}, errors.New("unbounded callback must not run")
	})
	if err := proc.SettleEventsApplied(context.Background()); err == nil {
		t.Fatal("unbounded deferred settlement was accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("unbounded callback ran")
	}
}
