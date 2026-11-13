// Design: docs/architecture/forked-route-install.md -- routing-view consumer lifecycle.
// Related: register.go -- the real SDK runner and its resource cleanup.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A normal daemon stop cancels the consumer and closes its transport. The
// already-stopped lease is cleaned by its daemon generation hook; the runner
// must not turn that normal stop into a failed plugin because Close cannot RPC.
func TestRouteViewPluginNormalShutdownRemainsSuccessful(t *testing.T) {
	h := newRouteViewPluginHarness(t, nil)
	h.start(t)
	h.await(t, h.reading, "synchronized consumer idle read")
	h.cancel()
	if err := h.engine.Close(); err != nil {
		t.Fatal(err)
	}
	if err := h.result(t); err != nil {
		t.Fatalf("normal stopped transport became a failed plugin: %v", err)
	}
}

// Closing the SDK ends its callback loop cleanly. A failed routing-view worker
// still owes a failed runner result carrying its actual cause, so supervision
// cannot read a disconnected, non-forwarding consumer as healthy.
func TestRouteViewPluginRetainsFatalWorkerFailure(t *testing.T) {
	cause := errors.New("fixture-authority-view-lost")
	h := newRouteViewPluginHarness(t, cause)
	h.start(t)
	h.await(t, h.reading, "worker read of configured authority")
	err := h.result(t)
	if err == nil {
		t.Fatal("fatal routing-view failure became a successful plugin exit")
	}
	if !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("runner lost the worker cause %q: %v", cause, err)
	}
}

// This harness drives the production runner through all five SDK stages over
// a real net.Pipe/MuxConn. Its authority answers are a controlled socket mock,
// not proof of daemon registration, TLS placement, selection or kernel output.
// Each helper owns one joined lifecycle goroutine and one joined request pump.
type routeViewPluginHarness struct {
	ctx          context.Context
	run          context.Context
	cancel       context.CancelFunc
	engine       *rpc.MuxConn
	declared     chan struct{}
	capabilities chan struct{}
	reading      chan struct{}
	ended        chan struct{}
	pumpDone     chan struct{}
	registration rpc.DeclareRegistrationInput
	readFailure  error
	initial      bool
	err          error
}

func newRouteViewPluginHarness(t *testing.T, readFailure error) *routeViewPluginHarness {
	t.Helper()
	ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	run, cancel := context.WithCancel(ctx)
	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn(configRootRIB, pluginEnd)
	h := &routeViewPluginHarness{
		ctx: ctx, run: run, cancel: cancel,
		engine:   rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)),
		declared: make(chan struct{}), capabilities: make(chan struct{}),
		reading: make(chan struct{}), ended: make(chan struct{}), pumpDone: make(chan struct{}),
		readFailure: readFailure, initial: true,
	}
	go h.pump()
	go func() {
		defer close(h.ended)
		h.err = runRouteViewPlugin(run, p)
	}()
	t.Cleanup(func() {
		cleanup, finish := context.WithTimeout(context.WithoutCancel(h.ctx), 5*time.Second)
		defer finish()
		cancel()
		if err := h.engine.Close(); err != nil {
			t.Error(err)
		}
		h.awaitContext(t, cleanup, h.ended, "runner cleanup")
		h.awaitContext(t, cleanup, h.pumpDone, "engine request pump cleanup")
		stop()
	})
	return h
}

func (h *routeViewPluginHarness) start(t *testing.T) {
	t.Helper()
	h.await(t, h.declared, "Stage 1 registration")
	if h.registration.ApplyBudget != int(routeViewConfigurationApplyBudget/time.Second) {
		t.Fatalf("declared apply budget = %d", h.registration.ApplyBudget)
	}
	if len(h.registration.Commands) != len(commandDecls()) {
		t.Fatalf("declared command count = %d", len(h.registration.Commands))
	}
	// Empty sections are intentional: startup must still Open and acknowledge
	// a complete default declaration before answering configure.
	if _, err := h.engine.CallRPC(h.ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{}); err != nil {
		t.Fatalf("Stage 2 configure: %v", err)
	}
	h.await(t, h.capabilities, "Stage 3 capabilities")
	if _, err := h.engine.CallRPC(h.ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
		t.Fatalf("Stage 4 registry: %v", err)
	}
}

func (h *routeViewPluginHarness) await(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	h.awaitContext(t, h.ctx, done, what)
}

func (h *routeViewPluginHarness) awaitContext(t *testing.T, ctx context.Context, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatalf("waiting for %s: %v", what, ctx.Err())
	}
}

func (h *routeViewPluginHarness) result(t *testing.T) error {
	t.Helper()
	h.await(t, h.ended, "plugin result")
	return h.err
}

func (h *routeViewPluginHarness) pump() {
	defer close(h.pumpDone)
	for {
		select {
		case <-h.ctx.Done():
			return
		case request, open := <-h.engine.Requests():
			if !open {
				return
			}
			answer, err := h.answer(request)
			if err != nil {
				if sendErr := h.engine.SendError(h.ctx, request.ID, err.Error()); sendErr != nil {
					return
				}
				continue
			}
			if err := h.engine.SendResult(h.ctx, request.ID, answer); err != nil {
				return
			}
			if request.Method == rpc.MethodDeclareRegistration {
				close(h.declared)
			}
			if request.Method == "ze-plugin-engine:declare-capabilities" {
				close(h.capabilities)
			}
		}
	}
}

func (h *routeViewPluginHarness) answer(request *rpc.Request) (any, error) {
	switch request.Method {
	case rpc.MethodDeclareRegistration:
		if err := json.Unmarshal(request.Params, &h.registration); err != nil {
			return nil, err
		}
		return rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion, PluginName: "forwarding-west"}, nil
	case rpc.MethodMetricsPublish:
		var input rpc.MetricsPublishInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		if err := rpc.ValidateMetricsPublish(&input); err != nil {
			return nil, err
		}
		return rpc.MetricsPublishOutput{}, nil
	case "ze-plugin-engine:declare-capabilities", "ze-plugin-engine:ready":
		return nil, nil
	case rpc.MethodRouteViewOpen:
		return rpc.RouteViewOpenOutput{Session: "runner-owner", Epoch: "daemon", Snapshot: "initial"}, nil
	case rpc.MethodRouteViewDistances:
		var input rpc.RouteViewDistancesInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}, nil
	case rpc.MethodRouteViewRead:
		if h.readFailure != nil {
			close(h.reading)
			return nil, h.readFailure
		}
		if h.initial {
			h.initial = false
			return rpc.RouteViewReadOutput{Session: "runner-owner", Epoch: "daemon", Snapshot: "initial", Cursor: 1, Phase: rpc.RouteViewPhaseSnapshot, Complete: true}, nil
		}
		close(h.reading)
		<-h.run.Done()
		return nil, h.run.Err()
	case rpc.MethodRouteViewClose:
		return rpc.RouteViewCloseOutput{Session: "runner-owner"}, nil
	default:
		return nil, fmt.Errorf("unexpected runner method %q", request.Method)
	}
}
