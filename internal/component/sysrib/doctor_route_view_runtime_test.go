// Design: docs/architecture/doctor-and-health-checks.md -- live routing-view transitions.
package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewDoctorRuntimeResetAndRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	opened, release := make(chan struct{}), make(chan struct{})
	var opens atomic.Int32
	feed := runtimeFeedOverride(t, ctx, func(request *rpc.Request) (any, bool, error) {
		switch request.Method {
		case rpc.MethodRouteViewOpen:
			if opens.Add(1) == 1 {
				return nil, false, nil
			}
			close(opened)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, true, ctx.Err()
			}
			return rpc.RouteViewOpenOutput{Session: "replacement", Epoch: "daemon", Snapshot: "replacement-snapshot"}, true, nil
		case rpc.MethodRouteViewClose:
			var input rpc.RouteViewCloseInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewCloseOutput{Session: input.Session}, true, nil
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				return nil, true, err
			}
			return rpc.RouteViewDistancesOutput{Session: input.Session, Operation: input.Operation}, true, nil
		}
		return nil, false, nil
	})
	feed.pages <- runtimeSnapshot(1, true)
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if client.health == nil {
		t.Fatal("consumer has no runtime health monitor")
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	awaitRouteViewDoctor(t, ctx, client.health, "")
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "daemon", Cursor: 1, Phase: rpc.RouteViewPhaseReset}
	select {
	case <-opened:
	case <-ctx.Done():
		t.Fatal("reset never reopened its lease")
	}
	awaitRouteViewDoctor(t, ctx, client.health, "resynchronizing")
	feed.pages <- rpc.RouteViewReadOutput{Session: "replacement", Epoch: "daemon", Snapshot: "replacement-snapshot", Cursor: 1, Phase: rpc.RouteViewPhaseSnapshot, Complete: true}
	close(release)
	awaitRouteViewDoctor(t, ctx, client.health, "")
}

func TestRouteViewDoctorRuntimeExternalSynchronizationFailureRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	redistevents.RegisterProtocol("static")
	feed := newRouteViewRuntimeFeed(t, ctx)
	client := openRuntimeFeedClient(t, ctx, feed)
	defer closeRuntimeFeedClient(t, ctx, client)
	if client.health == nil {
		t.Fatal("successful consumer construction has no runtime health monitor")
	}
	awaitRouteViewDoctor(t, ctx, client.health, "initializing")
	feed.pages <- runtimeSnapshot(1, false, runtimeSelected(netip.MustParsePrefix("198.18.240.0/24"), rpc.RouteViewPath{Protocol: "static", Distance: 10, RouteType: uint8(routetype.Blackhole)}))
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	awaitRouteViewPending(t, ctx, client.health, 1)
	awaitRouteViewDoctor(t, ctx, client.health, "initializing")
	feed.pages <- runtimeSnapshot(2, true)
	awaitRouteViewDoctor(t, ctx, client.health, "")
	feed.pages <- rpc.RouteViewReadOutput{Session: "runtime-owner", Epoch: "wrong-epoch", Cursor: 2, Phase: rpc.RouteViewPhaseDelta, Complete: true}
	select {
	case <-client.worker.done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	awaitRouteViewDoctor(t, ctx, client.health, "epoch")
	// Recovery is a fresh consumer, rather than turning the failed session green.
	replacement := newRouteViewRuntimeFeed(t, ctx)
	replacement.pages <- runtimeSnapshot(1, true)
	recovered := openRuntimeFeedClient(t, ctx, replacement)
	defer closeRuntimeFeedClient(t, ctx, recovered)
	if recovered.health == nil {
		t.Fatal("replacement consumer has no runtime health monitor")
	}
	if err := recovered.Start(ctx); err != nil {
		t.Fatal(err)
	}
	awaitRouteViewDoctor(t, ctx, recovered.health, "")
}

func TestRouteViewDoctorRuntimeInternalSynchronization(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	client := routeViewNativeRuntimeClient(t, ctx, func(_ context.Context, method string, payload any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, ok := payload.(routeViewPublication)
		if !ok {
			return nil, true, fmt.Errorf("native publication = %T", payload)
		}
		return rpc.RouteViewPublishOutput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation}, true, nil
	})
	defer closeRuntimeFeedClient(t, ctx, client)
	if client.health == nil {
		t.Fatal("internal consumer construction has no runtime health monitor")
	}
	awaitRouteViewDoctor(t, ctx, client.health, "initializing")
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	awaitRouteViewDoctor(t, ctx, client.health, "")
}

func awaitRouteViewDoctor(t *testing.T, ctx context.Context, health *routeViewHealth, want string) {
	t.Helper()
	for {
		findings, err := health.findings(routeViewDoctorName)
		if err != nil {
			t.Fatal(err)
		}
		if want == "" && len(findings) == 0 {
			return
		}
		if want != "" && len(findings) == 1 && strings.Contains(findings[0].Message, want) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("doctor = %v; want %q: %v", findings, want, ctx.Err())
		default:
			runtime.Gosched()
		}
	}
}

func awaitRouteViewPending(t *testing.T, ctx context.Context, health *routeViewHealth, want int) {
	t.Helper()
	for {
		health.mu.Lock()
		pending := health.pendingCount
		health.mu.Unlock()
		if pending == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("pending = %d, want %d: %v", pending, want, ctx.Err())
		default:
			runtime.Gosched()
		}
	}
}
