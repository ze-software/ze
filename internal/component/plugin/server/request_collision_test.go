// Design: docs/architecture/api/process-protocol.md -- live correlation identities.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRuntimeDuplicateLiveIDEndsConnection holds an original selected or
// ordinary handler while a selected, ordinary or cancellation frame reuses its
// ID. The duplicate must end the stream, cancel the original, and join its
// final obligation; an error reply under that ID would release the wrong call.
func TestRuntimeDuplicateLiveIDEndsConnection(t *testing.T) {
	for _, original := range []string{"selected", "ordinary"} {
		for _, duplicate := range []string{"selected", "ordinary", "cancel"} {
			t.Run(original+"/"+duplicate, func(t *testing.T) {
				work, stop := context.WithTimeout(t.Context(), 5*time.Second)
				defer stop()
				started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
				host := &Server{ctx: work, reactor: &mockReactor{}}
				proc := nativeOwnerProcess(t, host)
				if err := host.UpdateDeliveryGraphWithEndpoints(graphNS, []DeliveryPeer{{Addr: "192.0.2.7", Name: "p7", Path: "/" + graphNS + "/entry/p7"}}, []BusParticipantInput{{
					Path:         plugin.EndpointPath(proc.Name()),
					EndpointName: proc.Name(),
					Sends:        []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}, {Message: busMsgBGPUpdate}},
				}}); err != nil {
					t.Fatal(err)
				}
				method := rpc.MethodSelectRoutes
				if original == "selected" {
					host.reactor = &selectedCancellationReactor{mockReactor: &mockReactor{}, parent: work,
						started: started, canceled: canceled, release: release}
				} else {
					const ordinaryMethod = "ze-plugin-engine:ordinary-held-collision-test"
					method = ordinaryMethod
					engineOperationMu.Lock()
					previous := engineOpTable[ordinaryMethod]
					engineOpTable[ordinaryMethod] = &engineOp{method: ordinaryMethod, handle: func(_ *Server, ctx context.Context, _ *process.Process, _ json.RawMessage) (any, error) {
						close(started)
						<-ctx.Done()
						close(canceled)
						select {
						case <-release:
						case <-work.Done():
						}
						return nil, ctx.Err()
					}}
					engineOperationMu.Unlock()
					t.Cleanup(func() {
						engineOperationMu.Lock()
						if previous == nil {
							delete(engineOpTable, ordinaryMethod)
						} else {
							engineOpTable[ordinaryMethod] = previous
						}
						engineOperationMu.Unlock()
					})
				}
				client, engine := net.Pipe()
				conn := plugipc.NewPluginConn(engine, engine)
				serverDone := make(chan struct{})
				go func() { host.serveRuntimeRequests(proc, conn); close(serverDone) }()
				t.Cleanup(func() {
					if err := client.Close(); err != nil {
						t.Error(err)
					}
					if err := engine.Close(); err != nil {
						t.Error(err)
					}
					select {
					case <-serverDone:
					case <-work.Done():
					}
				})
				input := selectedServerInput()
				input.DeadlineMS = time.Now().Add(rpc.CancellableRequestTimeout).UnixMilli()
				encoded, err := json.Marshal(input)
				if err != nil {
					t.Fatal(err)
				}
				if err := writeCollisionFrame(work, client, 7, method, encoded); err != nil {
					t.Fatal(err)
				}
				select {
				case <-started:
				case <-work.Done():
					t.Fatal("original handler did not start")
				}
				switch duplicate {
				case "selected":
					method = rpc.MethodSelectRoutes
				case "ordinary":
					method = "ze-plugin-engine:unknown-collision-test"
				case "cancel":
					method = rpc.MethodCancelRequest
					encoded = json.RawMessage(`{"request-id":7,"method":"ze-plugin-engine:select-routes"}`)
				}
				if err := writeCollisionFrame(work, client, 7, method, encoded); err != nil {
					t.Fatal(err)
				}
				reply, err := rpc.NewConn(client, client).ReadRequest(work)
				if err == nil {
					t.Fatalf("duplicate replied under occupied ID before writer join: %+v", reply)
				}
				if errors.Is(work.Err(), context.DeadlineExceeded) {
					t.Fatal("collision did not end the stream")
				}
				select {
				case <-canceled:
				case <-work.Done():
					t.Fatal("collision did not cancel the original handler")
				}
				select {
				case <-serverDone:
					t.Fatal("dispatcher returned before held final work joined")
				default:
				}
				close(release)
				select {
				case <-serverDone:
				case <-work.Done():
					t.Fatal("collision cleanup did not join")
				}
			})
		}
	}
}

func writeCollisionFrame(ctx context.Context, conn net.Conn, id uint64, method string, input json.RawMessage) error {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		return errors.New("collision fixture requires a bounded context")
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	frame := append(rpc.AppendRequest(nil, id, method, input), '\n')
	n, err := conn.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

// TestRuntimeDuplicateCancelIDEndsConnection pins the cancellation control's
// own live identity while it waits for a held target. A collision must close
// the stream before canceling that control, rather than replying under its ID.
func TestRuntimeDuplicateCancelIDEndsConnection(t *testing.T) {
	work, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	release := make(chan struct{})
	reactor := &selectedCancellationReactor{mockReactor: &mockReactor{}, parent: work,
		started: make(chan struct{}), canceled: make(chan struct{}), release: release}
	host := &Server{ctx: work, reactor: reactor}
	proc := nativeOwnerProcess(t, host)
	if err := host.UpdateDeliveryGraphWithEndpoints(graphNS, []DeliveryPeer{{Addr: "192.0.2.7", Name: "p7", Path: "/" + graphNS + "/entry/p7"}}, []BusParticipantInput{{
		Path:         plugin.EndpointPath(proc.Name()),
		EndpointName: proc.Name(),
		Sends:        []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}, {Message: busMsgBGPUpdate}},
	}}); err != nil {
		t.Fatal(err)
	}
	client, engine := net.Pipe()
	conn := plugipc.NewPluginConn(engine, engine)
	serverDone := make(chan struct{})
	go func() { host.serveRuntimeRequests(proc, conn); close(serverDone) }()
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-serverDone:
		case <-work.Done():
		}
	})
	input := selectedServerInput()
	input.DeadlineMS = time.Now().Add(rpc.CancellableRequestTimeout).UnixMilli()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCollisionFrame(work, client, 7, rpc.MethodSelectRoutes, encoded); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reactor.started:
	case <-work.Done():
		t.Fatal(work.Err())
	}
	control := json.RawMessage(`{"request-id":7,"method":"ze-plugin-engine:select-routes"}`)
	if err := writeCollisionFrame(work, client, 8, rpc.MethodCancelRequest, control); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reactor.canceled:
	case <-work.Done():
		t.Fatal(work.Err())
	}
	if err := writeCollisionFrame(work, client, 8, "ze-plugin-engine:unknown-collision-test", nil); err != nil {
		t.Fatal(err)
	}
	reply, err := rpc.NewConn(client, client).ReadRequest(work)
	if err == nil {
		t.Fatalf("collision replied under a waiting cancellation ID: %+v", reply)
	}
	if work.Err() != nil {
		t.Fatal("waiting control collision did not close the stream")
	}
	select {
	case <-serverDone:
		t.Fatal("dispatcher returned before the canceled target joined")
	default:
	}
	close(release)
	select {
	case <-serverDone:
	case <-work.Done():
		t.Fatal("target did not join after fatal collision")
	}
}
