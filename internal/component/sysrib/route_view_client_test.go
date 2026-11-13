// Design: docs/architecture/rib/unified-locrib.md -- system-RIB routing-view consumer.

package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The external consumer must acquire its lease and await the complete distance
// declaration at the daemon. Publishing to this process's local distance seam
// cannot satisfy that requirement, even when the SDK socket returns success.
func TestRouteViewRuntimeOpensAuthoritativeSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	var methods []string
	var applied rpc.RouteViewDistancesInput
	p := routeViewRuntimePeer(t, ctx, func(request *rpc.Request) (any, error) {
		methods = append(methods, request.Method)
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			return rpc.RouteViewOpenOutput{Session: "runtime-owner", Epoch: "daemon", Snapshot: "initial"}, nil
		case rpc.MethodRouteViewDistances:
			if err := json.Unmarshal(request.Params, &applied); err != nil {
				return nil, err
			}
			return rpc.RouteViewDistancesOutput{Session: applied.Session, Operation: applied.Operation}, nil
		case rpc.MethodRouteViewClose:
			return rpc.RouteViewCloseOutput{Session: "runtime-owner"}, nil
		default:
			return nil, fmt.Errorf("unexpected runtime method %q", request.Method)
		}
	})
	// A real SDK call is the positive control for the fixture's socket path.
	if _, err := p.RouteViewOpen(ctx, &rpc.RouteViewOpenInput{}); err != nil {
		t.Fatal(err)
	}
	declared, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	distance.Set(func(protocol string) (uint8, bool) { return 99, protocol == "static" })
	t.Cleanup(func() { distance.Set(nil) })
	client, err := openRouteViewClient(ctx, p, newSysRIB(), declared, map[string]bool{"static": true})
	if err != nil {
		t.Fatalf("open configured routing-view consumer: %v", err)
	}
	defer func() {
		if err := client.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	if !slices.Equal(methods, []string{rpc.MethodRouteViewOpen, rpc.MethodRouteViewOpen, rpc.MethodRouteViewDistances}) {
		t.Fatalf("startup methods = %v", methods)
	}
	if applied.Session != "runtime-owner" || applied.Operation == 0 || len(applied.Distances) != len(declared) {
		t.Fatalf("distance declaration did not reach authority: %+v", applied)
	}
	seen := make(map[string]bool, len(declared))
	for _, row := range applied.Distances {
		want, exists := declared[row.Protocol]
		if !exists || seen[row.Protocol] || int(row.Distance) != want {
			t.Fatalf("incomplete, duplicated or changed distance row: %+v", row)
		}
		seen[row.Protocol] = true
	}
	if got, declared := distance.Of("static"); !declared || got != 99 {
		t.Fatalf("external consumer changed its disconnected local declaration to %d, declared %v", got, declared)
	}
}

// OnStarted precedes the SDK callback loop. Start must return before the
// daemon admits or answers the worker's first selected-view read.
func TestRouteViewRuntimeStartsWithoutWaitingForView(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reading := make(chan struct{})
	release := make(chan struct{})
	initial := true
	p := routeViewRuntimePeer(t, ctx, func(request *rpc.Request) (any, error) {
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			return rpc.RouteViewOpenOutput{Session: "runtime-owner", Epoch: "daemon", Snapshot: "initial"}, nil
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, err
			}
			return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}, nil
		case rpc.MethodRouteViewRead:
			if initial {
				initial = false
				close(reading)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Snapshot: "initial", Cursor: 1, Phase: rpc.RouteViewPhaseSnapshot, Complete: true}, nil
			}
			return rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 1, Phase: rpc.RouteViewPhaseDelta, Complete: true}, nil
		case rpc.MethodRouteViewClose:
			return rpc.RouteViewCloseOutput{Session: "runtime-owner"}, nil
		default:
			return nil, fmt.Errorf("unexpected runtime method %q for an empty snapshot", request.Method)
		}
	})
	declared, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	client, err := openRouteViewClient(ctx, p, newSysRIB(), declared, map[string]bool{"static": true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		close(release)
		if err := client.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("start routing-view worker: %v", err)
	}
	select {
	case <-reading:
	case <-ctx.Done():
		t.Fatal("Start returned without launching the selected-view reader")
	}
}

// routeViewRuntimePeer runs one bounded socket responder. Cleanup closes both
// connections and joins it, including when a constructor never sends a call.
func routeViewRuntimePeer(t *testing.T, ctx context.Context, answer func(*rpc.Request) (any, error)) *sdk.Plugin {
	t.Helper()
	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn("rib", pluginEnd)
	engine := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case request, open := <-engine.Requests():
				if !open {
					return
				}
				result, err := answer(request)
				if err != nil {
					if sendErr := engine.SendError(ctx, request.ID, err.Error()); sendErr != nil {
						return
					}
					continue
				}
				if err := engine.SendResult(ctx, request.ID, result); err != nil {
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		_ = p.Close()
		_ = engine.Close()
		<-done
	})
	return p
}
