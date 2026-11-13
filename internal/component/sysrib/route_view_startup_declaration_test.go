// Design: docs/architecture/forked-route-install.md -- startup distance acknowledgment.

package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The fixture runs the actual RunEngine entry point and all five SDK stages.
// Its socket peer uses a private real authority and one exact process owner;
// it does not prove server startup admission or TLS placement.
type routeViewStartupDeclaration struct {
	ctx          context.Context
	engine       *rpc.MuxConn
	authority    routeViewAuthority
	rib          *locrib.RIB
	owner        *process.Process
	declared     chan struct{}
	capabilities chan struct{}
	ready        chan struct{}
	pumpDone     chan struct{}
	runDone      chan int
}

func startRouteViewDeclaration(t *testing.T) *routeViewStartupDeclaration {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	pluginEnd, engineEnd := net.Pipe()
	fixture := &routeViewStartupDeclaration{ctx: ctx, engine: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), rib: locrib.NewRIB(), owner: routeViewOwner(), declared: make(chan struct{}), capabilities: make(chan struct{}), ready: make(chan struct{}), pumpDone: make(chan struct{}), runDone: make(chan int, 1)}
	go fixture.pump()
	go func() { fixture.runDone <- runSysRIBPlugin(pluginEnd) }()
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer stop()
		if err := fixture.engine.Close(); err != nil {
			t.Error(err)
		}
		cancel()
		select {
		case <-fixture.runDone:
		case <-cleanup.Done():
			t.Error("runSysRIBPlugin did not join after engine close")
		}
		select {
		case <-fixture.pumpDone:
		case <-cleanup.Done():
			t.Error("startup declaration request pump did not join")
		}
		if err := fixture.authority.cleanup(fixture.owner); err != nil {
			t.Error(err)
		}
	})
	fixture.await(t, fixture.declared, "Stage 1 registration")
	// No rib section must still invoke bootstrap with the complete defaults.
	if _, err := fixture.engine.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{}); err != nil {
		t.Fatalf("Stage 2 configure acknowledgment: %v", err)
	}
	fixture.await(t, fixture.capabilities, "Stage 3 capabilities")
	if _, err := fixture.engine.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
		t.Fatalf("Stage 4 registry: %v", err)
	}
	fixture.await(t, fixture.ready, "Stage 5 ready")
	return fixture
}

func (fixture *routeViewStartupDeclaration) await(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-fixture.ctx.Done():
		t.Fatalf("waiting for %s: %v", what, fixture.ctx.Err())
	}
}

func (fixture *routeViewStartupDeclaration) pump() {
	defer close(fixture.pumpDone)
	for {
		select {
		case <-fixture.ctx.Done():
			return
		case request, open := <-fixture.engine.Requests():
			if !open {
				return
			}
			answer, err := fixture.answer(request)
			if err != nil {
				if sendErr := fixture.engine.SendError(fixture.ctx, request.ID, err.Error()); sendErr != nil {
					return
				}
				continue
			}
			if err := fixture.engine.SendResult(fixture.ctx, request.ID, answer); err != nil {
				return
			}
			switch request.Method {
			case rpc.MethodDeclareRegistration:
				close(fixture.declared)
			case "ze-plugin-engine:declare-capabilities":
				close(fixture.capabilities)
			case "ze-plugin-engine:ready":
				close(fixture.ready)
			}
		}
	}
}

func (fixture *routeViewStartupDeclaration) answer(request *rpc.Request) (any, error) {
	switch request.Method {
	case rpc.MethodDeclareRegistration:
		return rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}, nil
	case "ze-plugin-engine:declare-capabilities", "ze-plugin-engine:ready":
		return nil, nil
	case rpc.MethodRouteViewOpen:
		var input rpc.RouteViewOpenInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return fixture.authority.open(fixture.ctx, fixture.owner, fixture.rib, input)
	case rpc.MethodRouteViewDistances:
		var input rpc.RouteViewDistancesInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return fixture.authority.applyDistances(fixture.ctx, fixture.owner, input)
	case rpc.MethodRouteViewRead:
		var input rpc.RouteViewReadInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return fixture.authority.read(fixture.ctx, fixture.owner, input)
	case rpc.MethodRouteViewClose:
		var input rpc.RouteViewCloseInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return nil, err
		}
		return fixture.authority.close(fixture.owner, input.Session)
	default:
		return nil, fmt.Errorf("unexpected startup declaration method %q", request.Method)
	}
}
