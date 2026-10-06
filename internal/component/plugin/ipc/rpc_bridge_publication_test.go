// Design: docs/architecture/api/process-protocol.md -- negotiated bridge publication.
package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPluginConnBridgeActivationLiveness separates bridge publication from SDK
// activation, and checks that shutdown cannot revive the startup transport.
func TestPluginConnBridgeActivationLiveness(t *testing.T) {
	t.Parallel()
	for _, muxed := range []bool{false, true} {
		name := "direct"
		if muxed {
			name = "muxed"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var conn *PluginConn
			if muxed {
				conn, _ = newMuxEnginePluginConn(t)
			} else {
				conn, _ = newTestPluginConn(t)
			}
			bridge := rpc.NewDirectBridge()
			defer bridge.CloseCallbacks()
			conn.SetBridge(bridge)
			if !conn.HasBridge() {
				t.Fatal("negotiated bridge is not published")
			}
			if err := conn.Err(); err != nil {
				t.Fatalf("published but not activated bridge rejected live IPC: %v", err)
			}
			bridge.SetReady()
			if err := conn.Err(); err != nil {
				t.Fatalf("activated bridge: %v", err)
			}
			bridge.CloseCallbacks()
			if err := conn.Err(); !errors.Is(err, rpc.ErrBridgeClosed) {
				t.Fatalf("stopped bridge with live IPC: %v, want ErrBridgeClosed", err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if _, err := conn.CallRPC(ctx, "publication", nil); !errors.Is(err, rpc.ErrBridgeClosed) {
				t.Fatalf("callback after shutdown: %v, want ErrBridgeClosed", err)
			}
			if _, err := conn.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{}); !errors.Is(err, rpc.ErrBridgeClosed) {
				t.Fatalf("command after shutdown: %v, want ErrBridgeClosed", err)
			}
		})
	}
}

// TestPluginConnBridgePublication overlaps the one startup publication with
// liveness, generic RPC and typed command readers. Both real transports answer
// until readers finish; after publication every callback must use the bridge.
func TestPluginConnBridgePublication(t *testing.T) {
	t.Parallel()
	conn, pluginConn := newMuxEnginePluginConn(t)
	bridge := rpc.NewDirectBridge()
	defer bridge.CloseCallbacks()
	bridge.SetExecuteCommand(func(string, string, []string, string) (*rpc.ExecuteCommandOutput, error) {
		return &rpc.ExecuteCommandOutput{Status: rpc.StatusDone, Data: json.RawMessage(`"bridge"`)}, nil
	})
	bridge.SetReady()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var services sync.WaitGroup
	services.Go(func() {
		for {
			req, err := pluginConn.ReadRequest(ctx)
			if err != nil {
				return
			}
			if req.Method == methodExecuteCommand {
				err = rpc.WriteDocumentAnswer(pluginConn.AnswerWriter(ctx), req.ID, rpc.AnswerTail{}, json.RawMessage(`"ipc"`))
			} else {
				err = pluginConn.SendResult(ctx, req.ID, json.RawMessage(`"ipc"`))
			}
			if err != nil {
				t.Errorf("startup IPC response: %v", err)
				return
			}
		}
	})
	services.Go(func() {
		for {
			select {
			case req := <-bridge.CallbackCh():
				req.Result <- rpc.BridgeCallbackResult{Data: json.RawMessage(`"bridge"`)}
			case req := <-bridge.ExecuteCommandRequests():
				out, err := bridge.RunExecuteCommand(req)
				req.Result <- rpc.ExecuteCommandResult{Output: out, Err: err}
			case <-ctx.Done():
				return
			}
		}
	})
	defer func() {
		cancel()
		services.Wait()
	}()
	if result, err := conn.CallRPC(ctx, "publication", nil); err != nil || string(result) != `"ipc"` {
		t.Fatalf("startup IPC control: result=%s, err=%v", result, err)
	}
	start := make(chan struct{})
	var readers sync.WaitGroup
	readers.Go(func() {
		<-start
		for range 256 {
			conn.HasBridge()
			if err := conn.Err(); err != nil {
				t.Errorf("concurrent liveness: %v", err)
				return
			}
		}
	})
	readers.Go(func() {
		<-start
		for range 64 {
			result, err := conn.CallRPC(ctx, "publication", nil)
			if err != nil {
				t.Errorf("concurrent callback: %v", err)
				return
			}
			if string(result) != `"ipc"` && string(result) != `"bridge"` {
				t.Errorf("concurrent callback result: %s", result)
				return
			}
		}
	})
	readers.Go(func() {
		<-start
		for range 64 {
			out, err := conn.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{Command: "publication"})
			if err != nil {
				t.Errorf("concurrent command: %v", err)
				return
			}
			if out.Status != rpc.StatusDone {
				t.Errorf("concurrent command status: %s", out.Status)
				return
			}
			if string(out.Data) != `"ipc"` && string(out.Data) != `"bridge"` {
				t.Errorf("concurrent command result: %s", out.Data)
				return
			}
		}
	})
	close(start)
	conn.SetBridge(bridge)
	readers.Wait()
	if result, err := conn.CallRPC(ctx, "publication", nil); err != nil || string(result) != `"bridge"` {
		t.Fatalf("published callback: result=%s, err=%v", result, err)
	}
	out, err := conn.SendExecuteCommand(ctx, &rpc.ExecuteCommandInput{Command: "publication"})
	if err != nil {
		t.Fatalf("published typed command: %v", err)
	}
	if string(out.Data) != `"bridge"` {
		t.Fatalf("published typed command used IPC: %s", out.Data)
	}
}

// TestPluginConnBridgeCutoverLiveness observes liveness while the SDK activates
// the published bridge and closes IPC. No observation may invent a failure.
func TestPluginConnBridgeCutoverLiveness(t *testing.T) {
	t.Parallel()
	conn, pluginConn := newMuxEnginePluginConn(t)
	bridge := rpc.NewDirectBridge()
	defer bridge.CloseCallbacks()
	conn.SetBridge(bridge)
	start := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		<-start
		for range 4096 {
			if err := conn.Err(); err != nil {
				done <- fmt.Errorf("during activation: %w", err)
				return
			}
		}
		done <- nil
	}()
	close(start)
	bridge.SetReady()
	if err := pluginConn.Close(); err != nil {
		t.Fatalf("close startup IPC: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := conn.Err(); err != nil {
		t.Fatalf("activated bridge after IPC close: %v", err)
	}
}

// TestPluginConnUnactivatedBridgeKeepsIPCFailure proves publication alone
// cannot hide a dead startup transport before the SDK has activated its bridge.
func TestPluginConnUnactivatedBridgeKeepsIPCFailure(t *testing.T) {
	t.Parallel()
	conn, _ := newMuxEnginePluginConn(t)
	bridge := rpc.NewDirectBridge()
	defer bridge.CloseCallbacks()
	conn.SetBridge(bridge)
	if err := conn.Close(); err != nil {
		t.Fatalf("close startup IPC: %v", err)
	}
	select {
	case <-conn.mux.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("startup IPC reader did not terminate")
	}
	if err := conn.Err(); !errors.Is(err, rpc.ErrMuxConnClosed) {
		t.Fatalf("unactivated bridge with closed IPC: %v, want ErrMuxConnClosed", err)
	}
}
