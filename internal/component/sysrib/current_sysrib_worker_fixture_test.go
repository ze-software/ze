// Design: docs/architecture/rib/unified-locrib.md -- owned current SDK worker fixtures.

package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// startCurrentSysRIB waits for the actual current worker to synchronize before
// returning. Its owner MUST let testing cleanup close the client and SDK.
func startCurrentSysRIB(t *testing.T, ctx context.Context, rib *locrib.RIB, state *sysRIB, bus *testEventBus) *routeViewClient {
	t.Helper()
	client := openCurrentSysRIB(t, ctx, rib, state, bus)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitForCurrentSysRIB(t, client)
	return client
}

func waitForCurrentSysRIB(t *testing.T, client *routeViewClient) {
	t.Helper()
	waitFor(t, 5*time.Second, func() bool {
		client.health.mu.Lock()
		defer client.health.mu.Unlock()
		_, ready := client.health.state.(routeViewHealthy)
		return ready
	})
}

// openCurrentSysRIB opens the actual native SDK lease against a private real
// authority. Its owner MUST let testing cleanup close the client before the
// SDK connection and exact authority generation, including if Start fails.
func openCurrentSysRIB(t *testing.T, ctx context.Context, rib *locrib.RIB, state *sysRIB, bus *testEventBus) *routeViewClient {
	t.Helper()
	if locrib.Default() == nil {
		t.Fatal("current native worker fixture has no initialized daemon Loc-RIB")
	}
	var authority routeViewAuthority
	owner := routeViewOwner()
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn("rib", rpc.NewBridgedConn(pluginEnd, bridge))
	var client *routeViewClient
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if client != nil {
			if err := client.Close(cleanup); err != nil {
				t.Error(err)
			}
			if err := client.Failure(); err != nil {
				t.Error(err)
			}
		}
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := engineEnd.Close(); err != nil {
			t.Error(err)
		}
		if err := authority.cleanup(owner); err != nil {
			t.Error(err)
		}
		distance.Set(nil)
	})
	bridge.SetDispatchRPC(func(call context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		var output any
		var err error
		switch method {
		case rpc.MethodRouteViewOpen:
			var input rpc.RouteViewOpenInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			output, err = authority.open(call, owner, rib, input)
		case rpc.MethodRouteViewDistances:
			var input rpc.RouteViewDistancesInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			output, err = authority.applyDistances(call, owner, input)
		case rpc.MethodRouteViewClose:
			var input rpc.RouteViewCloseInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			output, err = authority.close(owner, input.Session)
		default:
			return nil, fmt.Errorf("current worker fixture received unexpected RPC %q", method)
		}
		if err != nil {
			return nil, err
		}
		return json.Marshal(output)
	})
	bridge.SetDispatchNative(func(call context.Context, method string, payload any) (any, bool, error) {
		if method != rpc.MethodRouteViewPublish {
			return nil, false, nil
		}
		input, ok := payload.(routeViewPublication)
		if !ok {
			return nil, true, fmt.Errorf("current worker native publication received %T", payload)
		}
		output, err := authority.publish(call, owner, input, bus)
		return output, true, err
	})
	bridge.SetReady()
	distances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	permit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	client, err = openRouteViewClient(ctx, p, state, distances, permit)
	if err != nil {
		t.Fatal(err)
	}
	// Match the successfully opened private authority with its actual reader.
	// The production constructor uses the daemon singleton for this dependency;
	// the fixture owns a private real RIB to isolate the original test topology.
	client.reader = routeViewClientLocal{rib: rib}
	return client
}
