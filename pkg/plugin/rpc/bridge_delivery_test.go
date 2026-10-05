// Design: docs/architecture/api/process-protocol.md -- direct event delivery shutdown.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// TestDirectBridgeDeliveryShutdown keeps an admitted handler blocked while the
// shutdown fence rejects new text and structured batches, then joins its result.
func TestDirectBridgeDeliveryShutdown(t *testing.T) {
	for _, structured := range []bool{false, true} {
		name := "text"
		if structured {
			name = "structured"
		}
		t.Run(name, func(t *testing.T) {
			bridge := NewDirectBridge()
			entered := make(chan struct{})
			release := make(chan struct{})
			var completed atomic.Bool
			handle := func() error {
				close(entered)
				<-release
				completed.Store(true)
				return nil
			}
			bridge.SetDeliverEvents(func([]string) error { return handle() })
			bridge.SetDeliverStructured(func([]any) error { return handle() })
			bridge.SetReady()
			callDone := make(chan error, 1)
			go func() {
				if structured {
					callDone <- bridge.DeliverStructured(nil)
					return
				}
				callDone <- bridge.DeliverEvents(nil)
			}()
			<-entered
			bridge.CloseCallbacks()
			if err := bridge.DeliverEvents(nil); !errors.Is(err, ErrBridgeClosed) {
				t.Errorf("text delivery after shutdown = %v, want ErrBridgeClosed", err)
			}
			if err := bridge.DeliverStructured(nil); !errors.Is(err, ErrBridgeClosed) {
				t.Errorf("structured delivery after shutdown = %v, want ErrBridgeClosed", err)
			}
			waitDone := make(chan bool, 1)
			go func() {
				bridge.WaitDelivery()
				waitDone <- completed.Load()
			}()
			close(release)
			select {
			case err := <-callDone:
				if err != nil {
					t.Fatalf("admitted delivery = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("admitted delivery did not finish")
			}
			select {
			case finished := <-waitDone:
				if !finished {
					t.Fatal("delivery drain returned before the admitted handler finished")
				}
			case <-time.After(time.Second):
				t.Fatal("delivery drain did not finish")
			}
		})
	}
}

// TestDirectBridgeDeliveryReentrantStop proves a handler can call back into the
// engine and request shutdown without holding admission locks or joining itself.
func TestDirectBridgeDeliveryReentrantStop(t *testing.T) {
	bridge := NewDirectBridge()
	bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		bridge.CloseCallbacks()
		return json.RawMessage(`{"stopped":true}`), nil
	})
	bridge.SetDeliverStructured(func([]any) error {
		_, err := bridge.DispatchRPC(t.Context(), "stop", nil)
		return err
	})
	bridge.SetReady()
	done := make(chan error, 1)
	go func() { done <- bridge.DeliverStructured(nil) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("reentrant shutdown = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reentrant shutdown deadlocked")
	}
	bridge.WaitDelivery()
	bridge.WaitDispatch()
}

// TestDirectBridgeShutdownReleasesPendingCallbacks removes a request from its
// queue without answering it, then stops the callback loop. Both callback forms
// must release their waiting caller rather than pin an admitted event forever.
func TestDirectBridgeShutdownReleasesPendingCallbacks(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "generic"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			bridge := NewDirectBridge()
			bridge.SetExecuteCommand(func(string, string, []string, string) (*ExecuteCommandOutput, error) {
				t.Fatal("callback must not run after its loop stopped")
				return nil, ErrBridgeClosed
			})
			bridge.SetReady()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if typed {
					_, err := bridge.ExecuteCommand(ctx, "", "test", nil, "")
					done <- err
					return
				}
				_, err := bridge.SendCallback(ctx, "test", nil)
				done <- err
			}()
			if typed {
				<-bridge.ExecuteCommandRequests()
			} else {
				<-bridge.CallbackCh()
			}
			bridge.CloseCallbacks()
			select {
			case err := <-done:
				if !errors.Is(err, ErrBridgeClosed) {
					t.Fatalf("callback shutdown = %v, want ErrBridgeClosed", err)
				}
			case <-time.After(time.Second):
				t.Fatal("callback waiter remained blocked after its loop stopped")
			}
		})
	}
}

// TestDirectBridgeShutdownUnblocksFullCallbackQueue fills each callback queue,
// waits until a sender holds the send admission, and then closes the bridge
// without a reader. Shutdown must wake the sender before joining its admission.
func TestDirectBridgeShutdownUnblocksFullCallbackQueue(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "generic"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			bridge := NewDirectBridge()
			bridge.SetExecuteCommand(func(string, string, []string, string) (*ExecuteCommandOutput, error) {
				return &ExecuteCommandOutput{}, nil
			})
			bridge.SetReady()
			if typed {
				for range cap(bridge.executeCommandCh) {
					bridge.executeCommandCh <- ExecuteCommandRequest{}
				}
			} else {
				for range cap(bridge.callbackCh) {
					bridge.callbackCh <- BridgeCallback{}
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			sent := make(chan error, 1)
			go func() {
				if typed {
					_, err := bridge.ExecuteCommand(ctx, "", "test", nil, "")
					sent <- err
					return
				}
				_, err := bridge.SendCallback(ctx, "test", nil)
				sent <- err
			}()
			// Only the blocked sender can hold this lock; queue contents are
			// fixed. The context bounds the scheduling wait.
			for bridge.sendMu.TryLock() {
				bridge.sendMu.Unlock()
				if err := ctx.Err(); err != nil {
					t.Fatal("sender did not enter the full callback queue")
				}
				runtime.Gosched()
			}
			closed := make(chan struct{})
			go func() {
				bridge.CloseCallbacks()
				close(closed)
			}()
			select {
			case err := <-sent:
				if !errors.Is(err, ErrBridgeClosed) {
					t.Fatalf("full-queue sender = %v, want ErrBridgeClosed", err)
				}
			case <-ctx.Done():
				t.Fatal("shutdown did not unblock the full-queue sender")
			}
			select {
			case <-closed:
			case <-ctx.Done():
				t.Fatal("callback shutdown did not finish")
			}
		})
	}
}

// TestDirectBridgeShutdownPreservesCallbackResult places the completed reply in
// the sender's result slot before closure. Shutdown must not replace that reply.
func TestDirectBridgeShutdownPreservesCallbackResult(t *testing.T) {
	for _, typed := range []bool{false, true} {
		name := "generic"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			bridge := NewDirectBridge()
			bridge.SetExecuteCommand(func(string, string, []string, string) (*ExecuteCommandOutput, error) {
				t.Fatal("completed callback handler must not rerun")
				return nil, ErrBridgeClosed
			})
			bridge.SetReady()
			done := make(chan error, 1)
			replyErr := errors.New("specific callback refusal")
			go func() {
				var err error
				if typed {
					_, err = bridge.ExecuteCommand(t.Context(), "serial", "test", nil, "")
				} else {
					_, err = bridge.SendCallback(t.Context(), "test", nil)
				}
				done <- err
			}()
			if typed {
				request := <-bridge.ExecuteCommandRequests()
				request.Result <- ExecuteCommandResult{Err: replyErr}
			} else {
				request := <-bridge.CallbackCh()
				request.Result <- BridgeCallbackResult{Err: replyErr}
			}
			bridge.CloseCallbacks()
			select {
			case err := <-done:
				if !errors.Is(err, replyErr) {
					t.Fatalf("completed callback result = %v, want original refusal", err)
				}
			case <-time.After(time.Second):
				t.Fatal("completed callback did not return")
			}
		})
	}
}
