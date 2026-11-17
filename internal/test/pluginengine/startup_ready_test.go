// Design: docs/contributing/testing.md -- plugin transport fixture startup
// Related: pluginengine.go -- Start must return an initialized SDK callback loop

package pluginengine

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Goal: Start joins the SDK's initialization, even after bridge activation.
// Method: hold the real OnStarted callback, let every runnable goroutine reach
// a durable block, and require Start to remain pending until that hold ends.
// MUTATION: return after the Stage 5 acknowledgement without its runtime receipt.
func TestStartJoinsRuntimeInitialization(t *testing.T) {
	for _, transport := range Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				entered := make(chan struct{})
				release := make(chan struct{})
				returned := make(chan *Engine, 1)
				var bridge *rpc.DirectBridge
				go func() {
					returned <- Start(t, transport, func(conn net.Conn) int {
						if carrier, ok := conn.(rpc.Bridger); ok {
							bridge = carrier.Bridge()
						}
						p := sdk.NewWithConn("startup-join-probe", conn)
						defer p.Close() //nolint:errcheck // the fixture also closes both connection ends
						p.OnStarted(func(ctx context.Context) error {
							close(entered)
							select {
							case <-release:
								return nil
							case <-ctx.Done():
								return ctx.Err()
							}
						})
						if err := p.Run(ctx, &sdk.Registration{}); err != nil {
							return 1
						}
						return 0
					})
				}()
				<-entered
				synctest.Wait()
				if transport == TransportBridge {
					if bridge == nil {
						t.Error("the SDK initializer did not receive its actual bridge")
					} else if !bridge.Activated() {
						t.Error("the blocked initializer did not reach bridge activation")
					}
				}
				var engine *Engine
				select {
				case engine = <-returned:
					t.Error("Start returned before the SDK initializer completed")
				default:
				}
				// Release on both the expected and mutant paths, then join Start;
				// its owned cleanup closes and joins the real SDK runner afterward.
				close(release)
				if engine == nil {
					engine = <-returned
				}
				if engine == nil {
					t.Fatal("completed startup returned no engine")
				}
			})
		})
	}
}

// Goal: a returned engine runs its first command without another readiness call.
// Method: the real initializer emits an ordinary engine RPC, then the very next
// test operation executes one real SDK command and checks its initialized reply.
// This is the positive control; the blocked initializer above is the causal RED.
func TestStartFirstCommandUsesInitializedCallback(t *testing.T) {
	for _, transport := range Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				initialized := make(chan struct{})
				calls := 0
				engine := Start(t, transport, func(conn net.Conn) int {
					p := sdk.NewWithConn("startup-command-probe", conn)
					defer p.Close() //nolint:errcheck // the fixture also closes both connection ends
					p.OnStarted(func(ctx context.Context) error {
						if _, err := p.EmitEvent(ctx, "startup-probe", "initialized", "", "", "ready"); err != nil {
							return err
						}
						close(initialized)
						return nil
					})
					p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
						select {
						case <-initialized:
						default:
							return rpc.StatusError, nil, errors.New("command ran before initialization")
						}
						calls++
						if serial != "1" {
							return rpc.StatusError, nil, errors.New("first command lost its serial")
						}
						if peer != "" {
							return rpc.StatusError, nil, errors.New("first command invented a peer")
						}
						if command != "show startup probe" {
							return rpc.StatusError, nil, errors.New("first command changed its route")
						}
						if len(args) != 1 {
							return rpc.StatusError, nil, errors.New("first command changed its argument count")
						}
						if args[0] != "first" {
							return rpc.StatusError, nil, errors.New("first command changed its argument")
						}
						return rpc.StatusDone, "initialized first command", nil
					})
					if err := p.Run(ctx, &sdk.Registration{
						Commands: []sdk.CommandDecl{{Name: "show startup probe"}},
					}); err != nil {
						return 1
					}
					return 0
				})
				data, err := engine.Execute(ctx, "show startup probe", []string{"first"})
				if err != nil {
					t.Fatal(err)
				}
				var reply string
				if err := json.Unmarshal(data, &reply); err != nil {
					t.Fatal(err)
				}
				if reply != "initialized first command" {
					t.Fatalf("first command answered %q", reply)
				}
				if calls != 1 {
					t.Fatalf("first command invoked %d callbacks, want one", calls)
				}
				requests := engine.RPCs()
				if len(requests) != 1 {
					t.Fatalf("initializer made %d engine requests, want one", len(requests))
				}
				if requests[0].Method != rpc.MethodEmitEvent {
					t.Fatalf("initializer reached %q, want the real emit-event operation", requests[0].Method)
				}
				var emitted rpc.EmitEventInput
				if err := json.Unmarshal(requests[0].Params, &emitted); err != nil {
					t.Fatal(err)
				}
				if emitted.Namespace != "startup-probe" {
					t.Fatalf("initializer namespace=%q", emitted.Namespace)
				}
				if emitted.EventType != "initialized" {
					t.Fatalf("initializer event type=%q", emitted.EventType)
				}
				if emitted.Event != "ready" {
					t.Fatalf("initializer event=%q", emitted.Event)
				}
			})
		})
	}
}
