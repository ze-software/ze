// Design: docs/architecture/api/process-protocol.md -- registered engine calls during startup.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestConfigureRefusesRuntimeEngineOperation proves an unsupported nested call
// receives a refusal while configure is still pending, rather than timing out.
func TestConfigureRefusesRuntimeEngineOperation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	conn, remote := newDriverPipe(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: "configure-rpc-test"})
	proc.SetRegistration(&plugin.PluginRegistration{})
	proc.SetConn(conn)
	s := &Server{config: &ServerConfig{}, registry: plugin.NewPluginRegistry(), ctx: ctx}
	peerDone := make(chan error, 1)
	go func() {
		select {
		case request := <-remote.Requests():
			if request.Method != "ze-plugin-callback:configure" {
				peerDone <- fmt.Errorf("expected configure, got %s", request.Method)
				return
			}
			_, err := remote.CallRPC(ctx, rpc.MethodRouteMetrics, nil)
			if _, refused := errors.AsType[*rpc.RPCCallError](err); !refused {
				peerDone <- fmt.Errorf("runtime call must be refused before configure completes, got %w", err)
				return
			}
			peerDone <- remote.SendOK(ctx, request.ID)
		case <-ctx.Done():
			peerDone <- ctx.Err()
		}
	}()
	if err := s.deliverConfigRPC(ctx, proc); err != nil {
		t.Errorf("configure after nested refusal: %v", err)
	}
	if err := <-peerDone; err != nil {
		t.Fatal(err)
	}
}

// TestConfigureServesOwnerEngineOperation proves a configured process can make
// a nested owner call before acknowledging configuration, with its real identity
// and deadline, while runtime-only, unknown, and codec names stay refused.
func TestConfigureServesOwnerEngineOperation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	conn, remote := newDriverPipe(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: "configured-alias"})
	proc.SetRegistration(&plugin.PluginRegistration{Name: "owner-registration"})
	proc.SetConn(conn)
	s := &Server{config: &ServerConfig{}, registry: plugin.NewPluginRegistry(), ctx: ctx}
	const method = "ze-plugin:configure-owner-test"
	registerTestEngineOperation(t, EngineOperation{
		Method: method, Startup: EngineOperationConfigureSafe,
		Handler: func(_ *Server, callCtx context.Context, authenticated *process.Process, params json.RawMessage) (any, error) {
			if authenticated != proc {
				return nil, errors.New("handler received a different process generation")
			}
			if _, bounded := callCtx.Deadline(); !bounded {
				return nil, errors.New("handler received no startup deadline")
			}
			return map[string]string{"owner": authenticated.Registration().Name}, nil
		},
	})
	registerTestEngineOperation(t, EngineOperation{
		Method: "ze-plugin:runtime-owner-test", Startup: EngineOperationRuntimeOnly,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
			return nil, errors.New("runtime handler must remain behind ready")
		},
	})
	registerTestEngineOperation(t, EngineOperation{
		Method: "ze-plugin:panic-owner-test", Startup: EngineOperationConfigureSafe,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
			panic("test owner handler defect")
		},
	})
	peerDone := make(chan error, 1)
	go func() {
		select {
		case request := <-remote.Requests():
			result, err := remote.CallRPC(ctx, method, map[string]string{"owner": "forged-owner"})
			if err != nil {
				peerDone <- err
				return
			}
			var reply map[string]string
			if err := json.Unmarshal(result, &reply); err != nil {
				peerDone <- err
				return
			}
			if reply["owner"] != "owner-registration" {
				peerDone <- fmt.Errorf("owner reply: %s", result)
				return
			}
			for _, denied := range []string{"ze-plugin:runtime-owner-test", "ze-plugin:unknown-test", rpc.MethodStateGet, "ze-plugin:panic-owner-test"} {
				_, err := remote.CallRPC(ctx, denied, nil)
				if _, refused := errors.AsType[*rpc.RPCCallError](err); !refused {
					peerDone <- fmt.Errorf("%s must be refused during configure: %w", denied, err)
					return
				}
			}
			// Recovery consumes the panicking request and keeps serving the
			// connection, rather than replaying it as Stage 3 or aborting startup.
			if _, err := remote.CallRPC(ctx, method, nil); err != nil {
				peerDone <- fmt.Errorf("owner call after contained panic: %w", err)
				return
			}
			peerDone <- remote.SendOK(ctx, request.ID)
		case <-ctx.Done():
			peerDone <- ctx.Err()
		}
	}()
	if err := s.deliverConfigRPC(ctx, proc); err != nil {
		t.Errorf("configure with nested owner call: %v", err)
	}
	if err := <-peerDone; err != nil {
		t.Fatal(err)
	}
}

// TestConfigureOwnerCallCancellation proves startup cancellation reaches the
// admitted handler and the configure lifecycle returns after its worker joins.
func TestConfigureOwnerCallCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	conn, remote := newDriverPipe(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: "cancel-owner"})
	proc.SetRegistration(&plugin.PluginRegistration{})
	proc.SetConn(conn)
	s := &Server{config: &ServerConfig{}, registry: plugin.NewPluginRegistry(), ctx: ctx}
	entered := make(chan struct{})
	registerTestEngineOperation(t, EngineOperation{
		Method: "ze-plugin:cancel-owner-test", Startup: EngineOperationConfigureSafe,
		Handler: func(_ *Server, callCtx context.Context, _ *process.Process, _ json.RawMessage) (any, error) {
			close(entered)
			<-callCtx.Done()
			return nil, callCtx.Err()
		},
	})
	peerDone := make(chan error, 1)
	go func() {
		select {
		case <-remote.Requests():
			_, err := remote.CallRPC(ctx, "ze-plugin:cancel-owner-test", nil)
			peerDone <- err
		case <-ctx.Done():
			peerDone <- ctx.Err()
		}
	}()
	go func() {
		select {
		case <-entered:
			cancel()
		case <-ctx.Done():
		}
	}()
	if err := s.deliverConfigRPC(ctx, proc); !errors.Is(err, context.Canceled) {
		t.Errorf("configure cancellation: %v", err)
	}
	if err := <-peerDone; !errors.Is(err, context.Canceled) {
		t.Errorf("nested call cancellation: %v", err)
	}
}

// TestConfigureFailsWhenNestedReplyCannotEncode keeps the peer waiting for a
// nested reply, proving a local encoding failure ends configure immediately.
func TestConfigureFailsWhenNestedReplyCannotEncode(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	conn, remote := newDriverPipe(t)
	proc := process.NewProcess(plugin.PluginConfig{Name: "encode-owner"})
	proc.SetRegistration(&plugin.PluginRegistration{})
	proc.SetConn(conn)
	s := &Server{config: &ServerConfig{}, registry: plugin.NewPluginRegistry(), ctx: ctx}
	registerTestEngineOperation(t, EngineOperation{
		Method: "ze-plugin:encode-owner-test", Startup: EngineOperationConfigureSafe,
		Handler: func(*Server, context.Context, *process.Process, json.RawMessage) (any, error) {
			return func() {}, nil
		},
	})
	peerDone := make(chan error, 1)
	go func() {
		select {
		case <-remote.Requests():
			_, err := remote.CallRPC(ctx, "ze-plugin:encode-owner-test", nil)
			peerDone <- err
		case <-ctx.Done():
			peerDone <- ctx.Err()
		}
	}()
	err := conn.SendConfigureServing(ctx, nil, nil, func(request *rpc.Request) (bool, error) {
		return s.serveConfigureEngineOp(ctx, proc, conn, request)
	})
	if _, unsupported := errors.AsType[*json.UnsupportedTypeError](err); !unsupported {
		t.Errorf("configure must return the nested result encoding failure, got %v", err)
	}
	if ctx.Err() != nil {
		t.Errorf("configure waited for its deadline instead of returning the encoding failure: %v", ctx.Err())
	}
	cancel()
	if err := <-peerDone; !errors.Is(err, context.Canceled) {
		t.Errorf("waiting nested peer after configure failure: %v", err)
	}
}

// registerTestEngineOperation isolates its declaration from other tests and
// restores the registry seal without changing any installed runtime operation.
func registerTestEngineOperation(t *testing.T, declaration EngineOperation) {
	t.Helper()
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() {
		engineOperationMu.Lock()
		delete(engineOpTable, declaration.Method)
		engineOperationMu.Unlock()
		registry.Restore(snapshot)
	})
	if err := RegisterEngineOp(declaration); err != nil {
		t.Fatalf("RegisterEngineOp: %v", err)
	}
}
