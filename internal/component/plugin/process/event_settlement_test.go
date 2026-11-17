// Design: docs/architecture/api/process-protocol.md — applied event settlement.
// Related: event_settlement.go — process-owned settlement composition.

package process

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// settleEventsAppliedProbe uses the production composition after main observed
// the original strict FIFO returning before deferred output had settled.
func settleEventsAppliedProbe(ctx context.Context, p *Process) error {
	return p.SettleEventsApplied(ctx)
}

// TestSettleEventsAppliedWaitsForDeferredWork compares the actual Process FIFO
// receipt with held deferred completion on both callback transports. A later
// event must still apply while the composed settlement waits.
func TestSettleEventsAppliedWaitsForDeferredWork(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(fmt.Sprintf("typed=%t", typed), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 6*time.Second)
			defer cancel()
			proc := NewProcess(plugin.PluginConfig{Name: t.Name(), Encoder: "json"})
			proc.SetRegistration(&plugin.PluginRegistration{EventSettlement: true})
			applied := make(chan string, 2)
			entered := make(chan struct{})
			release := make(chan struct{})
			var stopTransport func()
			if typed {
				proc.bridge = rpc.NewDirectBridge()
				proc.bridge.SetDeliverEvents(func(events []string) error {
					for _, event := range events {
						applied <- event
					}
					return nil
				})
				engineEnd, pluginEnd := net.Pipe()
				t.Cleanup(func() { pluginEnd.Close() }) //nolint:errcheck // Test-only unused startup end.
				connection := ipc.NewPluginConn(engineEnd, engineEnd)
				connection.SetBridge(proc.bridge)
				proc.SetConn(connection)
				proc.bridge.SetReady()
				done := make(chan struct{})
				go func() {
					defer close(done)
					select {
					case callback := <-proc.bridge.CallbackCh():
						close(entered)
						select {
						case <-release:
							var input map[string]any
							if err := json.Unmarshal(callback.Params, &input); err != nil {
								callback.Result <- rpc.BridgeCallbackResult{Err: err}
								return
							}
							input["status"] = "ok"
							body, err := json.Marshal(input)
							callback.Result <- rpc.BridgeCallbackResult{Data: body, Err: err}
						case <-ctx.Done():
							callback.Result <- rpc.BridgeCallbackResult{Err: ctx.Err()}
						}
					case <-ctx.Done():
					}
				}()
				stopTransport = func() { <-done }
			} else {
				engineEnd, pluginEnd := net.Pipe()
				engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
				pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
				proc.SetConn(ipc.NewMuxPluginConn(engineMux))
				settlement := make(chan *rpc.Request, 1)
				deliveryDone, settlementDone := make(chan struct{}), make(chan struct{})
				go func() {
					defer close(deliveryDone)
					for request := range pluginMux.Requests() {
						if request.Method == "ze-plugin-callback:event-settlement" {
							settlement <- request
							continue
						}
						var input struct {
							Events []string `json:"events"`
						}
						if err := json.Unmarshal(request.Params, &input); err != nil {
							t.Errorf("decode delivery: %v", err)
							return
						}
						for _, event := range input.Events {
							applied <- event
						}
						if err := pluginMux.SendOK(ctx, request.ID); err != nil {
							return
						}
					}
				}()
				go func() {
					defer close(settlementDone)
					select {
					case request := <-settlement:
						close(entered)
						select {
						case <-release:
							var input map[string]any
							if err := json.Unmarshal(request.Params, &input); err != nil {
								t.Errorf("decode settlement: %v", err)
								return
							}
							input["status"] = "ok"
							if err := pluginMux.SendResult(ctx, request.ID, input); err != nil {
								t.Errorf("answer settlement: %v", err)
							}
						case <-ctx.Done():
						}
					case <-ctx.Done():
					}
				}()
				stopTransport = func() {
					pluginMux.Close() //nolint:errcheck // Test-only transport teardown.
					<-deliveryDone
					<-settlementDone
				}
			}
			proc.StartDelivery(ctx)
			defer func() {
				cancel()
				proc.Stop()
				stopTransport()
				wait, waitCancel := context.WithTimeout(context.Background(), time.Second)
				defer waitCancel()
				if err := proc.Wait(wait); err != nil {
					t.Errorf("join delivery: %v", err)
				}
			}()
			if !proc.Deliver(EventDelivery{Output: "prior"}) {
				t.Fatal("prior event refused")
			}
			if err := proc.DrainEventsApplied(ctx); err != nil {
				t.Fatal(err)
			}
			if got := <-applied; got != "prior" {
				t.Fatalf("prior event = %q", got)
			}
			result := make(chan error, 1)
			go func() { result <- settleEventsAppliedProbe(ctx, proc) }()
			select {
			case <-entered:
			case err := <-result:
				t.Fatalf("application receipt preceded deferred settlement: %v", err)
			case <-ctx.Done():
				t.Fatal("settlement callback never started")
			}
			select {
			case err := <-result:
				t.Fatalf("held settlement completed early: %v", err)
			default:
			}
			if !proc.Deliver(EventDelivery{Output: "later"}) {
				t.Fatal("later event refused while settlement waited")
			}
			select {
			case got := <-applied:
				if got != "later" {
					t.Fatalf("later event = %q", got)
				}
			case <-ctx.Done():
				t.Fatal("settlement blocked delivery")
			}
			close(release)
			if err := <-result; err != nil {
				t.Fatalf("released settlement: %v", err)
			}
		})
	}
}
