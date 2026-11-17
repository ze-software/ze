// Design: docs/architecture/api/process-protocol.md -- exact event settlement receipts.
// Related: event_settlement.go -- captured-connection callback sender.
package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// newEventSettlementSender owns one callback-serving worker for the fixture
// lifetime. Cleanup MUST cancel and join it before its transports are discarded.
func newEventSettlementSender(t *testing.T, bridged bool, reply func(rpc.EventSettlementInput) (json.RawMessage, error)) (*PluginConn, *atomic.Int64) {
	t.Helper()
	engine, client := newMuxEnginePluginConn(t)
	ctx, cancel := context.WithCancel(t.Context())
	calls := &atomic.Int64{}
	done := make(chan struct{})
	var bridge *rpc.DirectBridge
	if bridged {
		bridge = rpc.NewDirectBridge()
		engine.SetBridge(bridge)
		bridge.SetReady()
	}
	go func() {
		defer close(done)
		// One test-owned worker serves until its fixture context ends.
		for {
			if bridge != nil {
				select {
				case callback, ok := <-bridge.CallbackCh():
					if !ok {
						return
					}
					var input rpc.EventSettlementInput
					if err := json.Unmarshal(callback.Params, &input); err != nil {
						callback.Result <- rpc.BridgeCallbackResult{Err: err}
						continue
					}
					calls.Add(1)
					data, err := reply(input)
					callback.Result <- rpc.BridgeCallbackResult{Data: data, Err: err}
				case <-ctx.Done():
					return
				}
				continue
			}
			request, err := client.ReadRequest(ctx)
			if err != nil {
				return
			}
			var input rpc.EventSettlementInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			if request.Method != rpc.MethodEventSettlement {
				t.Errorf("callback method = %q", request.Method)
				return
			}
			calls.Add(1)
			data, replyErr := reply(input)
			if replyErr != nil {
				err = client.SendError(ctx, request.ID, replyErr.Error())
			} else {
				err = client.SendResult(ctx, request.ID, data)
			}
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		if bridge != nil {
			bridge.CloseCallbacks()
		}
		<-done
	})
	return engine, calls
}

// TestEventSettlementSenderRequiresExactReceipt drives real bridge and socket
// callbacks and admits only a complete matching successful receipt.
func TestEventSettlementSenderRequiresExactReceipt(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		for _, fault := range []string{"none", "missing-deadline", "null-deadline", "foreign-deadline", "missing-status", "null-status", "unknown-status", "failed", "failure-without-cause", "success-with-cause", "large-cause", "null-receipt", "invalid-json"} {
			t.Run(fmt.Sprintf("bridge=%t/%s", bridged, fault), func(t *testing.T) {
				connection, calls := newEventSettlementSender(t, bridged, func(input rpc.EventSettlementInput) (json.RawMessage, error) {
					output := map[string]any{"deadline-ms": input.DeadlineMS, "status": rpc.StatusOK}
					switch fault {
					case "none":
					case "missing-deadline":
						delete(output, "deadline-ms")
					case "null-deadline":
						output["deadline-ms"] = nil
					case "foreign-deadline":
						output["deadline-ms"] = input.DeadlineMS + 1
					case "missing-status":
						delete(output, "status")
					case "null-status":
						output["status"] = nil
					case "unknown-status":
						output["status"] = "queued"
					case "failed":
						output["status"], output["error"] = rpc.StatusError, "writer is still blocked"
					case "failure-without-cause":
						output["status"] = rpc.StatusError
					case "success-with-cause":
						output["error"] = "writer is still blocked"
					case "large-cause":
						output["status"], output["error"] = rpc.StatusError, strings.Repeat("x", rpc.EventSettlementErrorBytesMax+1)
					case "null-receipt":
						return json.RawMessage("null"), nil
					case "invalid-json":
						return json.RawMessage("{"), nil
					}
					return json.Marshal(output)
				})
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				err := connection.SendEventSettlement(ctx, rpc.EventSettlementInput{DeadlineMS: time.Now().Add(time.Second).UnixMilli()})
				if fault == "none" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatal("incomplete, foreign or failed receipt completed settlement")
				}
				if got := calls.Load(); got != 1 {
					t.Fatalf("callback count = %d, want 1", got)
				}
			})
		}
	}
}

// TestEventSettlementSenderUsesEarlierCallerDeadline verifies that the plugin
// sees the caller's earlier budget and must echo that exact effective bound.
func TestEventSettlementSenderUsesEarlierCallerDeadline(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("test context has no deadline")
			}
			connection, _ := newEventSettlementSender(t, bridged, func(input rpc.EventSettlementInput) (json.RawMessage, error) {
				if input.DeadlineMS != deadline.UnixMilli() {
					return nil, fmt.Errorf("received deadline %d, want %d", input.DeadlineMS, deadline.UnixMilli())
				}
				return json.Marshal(rpc.EventSettlementOutput{DeadlineMS: input.DeadlineMS, Status: rpc.StatusOK})
			})
			if err := connection.SendEventSettlement(ctx, rpc.EventSettlementInput{DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestEventSettlementSenderRefusesCanceledSuccess cancels the caller before
// returning a correct receipt, so late success cannot revive its request.
func TestEventSettlementSenderRefusesCanceledSuccess(t *testing.T) {
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			connection, _ := newEventSettlementSender(t, bridged, func(input rpc.EventSettlementInput) (json.RawMessage, error) {
				cancel()
				return json.Marshal(rpc.EventSettlementOutput{DeadlineMS: input.DeadlineMS, Status: rpc.StatusOK})
			})
			if err := connection.SendEventSettlement(ctx, rpc.EventSettlementInput{DeadlineMS: time.Now().Add(time.Second).UnixMilli()}); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled settlement = %v, want context.Canceled", err)
			}
		})
	}
}
