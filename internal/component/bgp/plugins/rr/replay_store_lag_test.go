// Design: docs/architecture/plugin/rib-storage-design.md -- original source catch-up before replay.
// Related: replay_sources.go -- readiness requires committed physical ingress progress.
package rr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/adj_rib_in"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: a peer-up replay waits for the receive store's original source
// to apply the UPDATE, then joins exactly one relay before reporting ready.
// PREVENTS: the initial replay declaring success while neither rail delivered
// a pre-UP announcement. Both roles and the store run their registered SDK callbacks.
func TestPeerUpReplayWaitsForOriginalReceiveStore(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		for _, delayed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/lag=%t", transport, delayed), func(t *testing.T) {
				testPeerUpReplayStoreLag(t, transport, delayed)
			})
		}
	}
}

func testPeerUpReplayStoreLag(t *testing.T, transport pluginengine.Transport, delayed bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	inventory := registry.Lookup("bgp-adj-rib-in")
	require.NotNil(t, inventory)
	store := pluginengine.Start(t, transport, inventory.RunEngine)
	reflector := pluginengine.Start(t, transport, runRouteReflectorCallbacks)
	source := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001),
		PeerOwner: 701, ConnectionID: 41, SessionGeneration: 1041, DestinationSession: 141,
		IsIBGP: true, RouteReflectorClient: true}
	target := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: rpc.RemoteASOf(65002),
		PeerOwner: 702, ConnectionID: 42, SessionGeneration: 1042, DestinationSession: 142,
		InitialReplay: 142, IsIBGP: true, RouteReflectorClient: true}
	receipt := reflector.IssueSource(source.Address.String(), source.PeerOwner)
	ready := make(chan int, 1)
	sourceProcessed := make(chan struct{}, 1)
	probePending := make(chan struct{})
	releaseProbe := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseProbe) }) }
	t.Cleanup(release)
	var observed []rpc.RelayStoredRouteInput
	var mu sync.Mutex
	covered := false
	reflector.SetRPCHandler(func(call context.Context, method string, params json.RawMessage) (any, error) {
		switch method {
		case rpc.MethodDispatchCommandArgs:
			var input rpc.DispatchCommandArgsInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			if input.Command != ribevents.ReplayRoutesCommand && input.Command != "request bgp adj-rib-in eligibility" {
				return nil, pluginengine.ErrUnhandledRPC
			}
			data, err := store.Execute(call, input.Command, input.Args)
			if err != nil {
				return nil, err
			}
			if len(input.Args) == 4 && input.Args[2] == ribevents.ReplaySourceProgressArgument {
				var progress ribevents.ReplaySourceProgress
				if err := json.Unmarshal(data, &progress); err != nil {
					return nil, err
				}
				if progress.Covered == nil {
					return nil, fmt.Errorf("store omitted original source progress")
				}
				mu.Lock()
				covered = *progress.Covered
				mu.Unlock()
				if !*progress.Covered {
					close(probePending)
					select {
					case <-releaseProbe:
					case <-call.Done():
						return nil, call.Err()
					}
				}
			} else if input.Command == ribevents.ReplayRoutesCommand {
				mu.Lock()
				checked := covered
				mu.Unlock()
				if !checked {
					return nil, fmt.Errorf("read replay page before original source was covered")
				}
			}
			return &rpc.DispatchCommandOutput{Status: rpc.StatusDone, Data: data}, nil
		case rpc.MethodDispatchCommand:
			var input rpc.DispatchCommandInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			if input.DestinationSession == target.DestinationSession {
				mu.Lock()
				ready <- len(observed)
				mu.Unlock()
			}
			return &rpc.DispatchCommandOutput{Status: rpc.StatusDone, DestinationOwner: input.DestinationOwner,
				DestinationSession: input.DestinationSession, DeadlineMS: input.DeadlineMS}, nil
		case rpc.MethodReleaseCached:
			sourceProcessed <- struct{}{}
			return struct{}{}, nil
		case rpc.MethodRelayStoredRoute:
			var input rpc.RelayStoredRouteInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			mu.Lock()
			observed = append(observed, input)
			mu.Unlock()
			return rpc.CompletedCachedWrite(input.DeadlineMS, nil), nil
		}
		return nil, pluginengine.ErrUnhandledRPC
	})
	gate, err := json.Marshal(ribevents.ValidationGateState{Command: "request bgp adj-rib-in eligibility"})
	require.NoError(t, err)
	event, err := json.Marshal(rpc.EventEnvelope{Namespace: ribevents.Namespace, Event: ribevents.EventValidationGate, Payload: gate})
	require.NoError(t, err)
	require.NoError(t, reflector.Deliver(ctx, string(event)))
	storeState := `{"type":"state","peer":{"address":"192.0.2.1","remote":{"address":"192.0.2.1","as":65001}},"state":"up","peer-owner":701,"connection-id":"41"}`
	require.NoError(t, store.Deliver(ctx, storeState))
	storeUpdate := fmt.Sprintf(`{"type":"bgp","bgp":{"message":{"type":"update","id":80,"direction":"received"},"peer":{"address":"192.0.2.1","remote":{"address":"192.0.2.1","as":65001}},"connection-id":"41","peer-owner":701,"source-policy-epoch":5,"source-receipt":%d,"raw":{"attributes":"40010100400200400304c0000201","nlri":{"ipv4/unicast":"180a0000"}},"update":{"attr":{"origin":"igp"},"ipv4/unicast":[{"next-hop":"192.0.2.1","action":"add","nlri":["10.0.0.0/24"]}]}}}`, receipt)
	rrAdmissionText.deliverState(t, ctx, reflector, source, rpc.ReasonAttached)
	rrAdmissionText.deliverUpdate(t, ctx, reflector, source, receipt, 80, false, 0)
	select {
	case <-sourceProcessed:
	case <-ctx.Done():
		t.Fatal("RR did not finish processing the original UPDATE before peer-up")
	}
	if !delayed {
		require.NoError(t, store.Deliver(ctx, storeUpdate))
	}
	rrAdmissionText.deliverState(t, ctx, reflector, target, "")
	if delayed {
		select {
		case <-probePending:
		case <-ready:
			t.Fatal("RR reported ready before the store received the original UPDATE")
		case <-ctx.Done():
			t.Fatal("RR did not ask the lagging receive store for original source progress")
		}
		require.NoError(t, store.Deliver(ctx, storeUpdate))
		release()
	}
	select {
	case n := <-ready:
		require.Equal(t, 1, n, "initial readiness must follow the joined original relay")
	case <-ctx.Done():
		t.Fatal("RR did not report initial readiness")
	}
	require.NoError(t, reflector.SettleEvents(ctx))
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, observed, 1)
	require.Equal(t, target.PeerOwner, observed[0].DestinationOwner)
	require.Equal(t, target.DestinationSession, observed[0].DestinationSession)
	require.Len(t, observed[0].Routes, 1)
	require.Equal(t, receipt, observed[0].Routes[0].SourceReceipt)
	require.Equal(t, uint64(80), observed[0].Routes[0].MsgID)
}
