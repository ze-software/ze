// Design: docs/architecture/plugin/rib-storage-design.md -- actual registered RS cleanup barrier.
package rs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The real RS startup/event/settlement callbacks must keep an UNKNOWN original
// writer completion as an actionable failed barrier, with its source lease
// unreleased. Exact positive writer receipts are the useful control: the same
// cleanup joins, releases its original lease, and fresh C still forwards.
// This is registered SDK evidence, not an authenticated daemon final writer.
func TestRSRegisteredCleanupRefusesUnknownEnrollmentAndKeepsUsefulCurrentWork(t *testing.T) {
	for _, transport := range []pluginengine.Transport{pluginengine.TransportWire, pluginengine.TransportBridge} {
		for _, unknown := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unknown=%t", transport.String(), unknown), func(t *testing.T) {
				engine := pluginengine.Start(t, transport, RunRouteServer)
				receipt := engine.IssueSource("192.0.2.1", 1)
				engine.SetRPCHandler(func(_ context.Context, method string, raw json.RawMessage) (any, error) {
					if method != rpc.MethodForwardCached {
						return nil, pluginengine.ErrUnhandledRPC
					}
					var input rpc.ForwardCachedInput
					if err := json.Unmarshal(raw, &input); err != nil {
						return nil, err
					}
					if len(input.Targets) != 1 || input.Targets[0].DestinationOwner != 2 || input.Targets[0].DestinationSession != 202 {
						return nil, errors.New("cached output lost original target tuple")
					}
					if unknown {
						return nil, errors.New("original remote enrollment has no joined writer receipt")
					}
					return &rpc.CachedWriteOutput{Completed: true, DeadlineMS: input.DeadlineMS}, nil
				})
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				for _, event := range []string{
					"peer 192.0.2.1 remote as 65001 state up reason attached peer-owner 1 connection-id 101 destination-session 201 session-generation 7 rs-client true",
					"peer 192.0.2.2 remote as 65002 state up reason attached peer-owner 2 connection-id 102 destination-session 202 session-generation 8 rs-client true",
					rsFixtureUpdate(rsFixtureKey("192.0.2.1", 1, 101), receipt, 7, 1, "nlri ipv4/unicast add prefix 10.0.1.0/24"),
				} {
					if err := engine.Deliver(ctx, event); err != nil {
						t.Fatal(err)
					}
				}
				payload, err := retention.EncodeCleanup(1, 101, "192.0.2.1", []family.Family{family.IPv4Unicast}, false)
				if err != nil {
					t.Fatal(err)
				}
				envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: retention.Namespace, Event: retention.CleanupEventType, Payload: json.RawMessage(payload)})
				if err != nil {
					t.Fatal(err)
				}
				if err := engine.Deliver(ctx, string(envelope)); err != nil {
					t.Fatal(err)
				}
				err = engine.SettleEvents(ctx)
				if unknown {
					if err == nil {
						t.Fatal("registered cleanup acknowledged an unresolved original remote enrollment")
					}
					if !strings.Contains(err.Error(), "192.0.2.2 owner 2 session 202") {
						t.Fatalf("failed barrier lost its original target scope: %v", err)
					}
					if repeat := engine.SettleEvents(ctx); repeat == nil {
						t.Fatal("a repeated barrier erased unresolved original output debt")
					}
					if err := engine.Deliver(ctx, rsFixtureUpdate(rsFixtureKey("192.0.2.1", 1, 101), receipt, 8, 2, "nlri ipv4/unicast add prefix 10.0.3.0/24")); err != nil {
						t.Fatal(err)
					}
					if repeat := engine.SettleEvents(ctx); repeat == nil {
						t.Fatal("fresh input erased unresolved original output debt")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if err := engine.Deliver(ctx, rsFixtureUpdate(rsFixtureKey("192.0.2.1", 1, 101), receipt, 8, 2, "nlri ipv4/unicast add prefix 10.0.3.0/24")); err != nil {
						t.Fatal(err)
					}
					if err := engine.SettleEvents(ctx); err != nil {
						t.Fatal(err)
					}
				}
				forwards, releases := 0, 0
				for _, call := range engine.RPCs() {
					if call.Method == rpc.MethodForwardCached {
						forwards++
					}
					if call.Method == rpc.MethodSourceLease {
						var input rpc.SourceLeaseInput
						if err := json.Unmarshal(call.Params, &input); err != nil {
							t.Fatal(err)
						}
						if input.Action == rpc.SourceLeaseRelease {
							releases++
						}
					}
				}
				if unknown {
					if forwards != 1 || releases != 0 {
						t.Fatalf("unresolved original enrollment was retried or released: forwards=%d releases=%d", forwards, releases)
					}
				} else {
					if forwards != 2 || releases != 1 {
						t.Fatalf("healthy cleanup lost its release or useful fresh C: forwards=%d releases=%d", forwards, releases)
					}
				}
			})
		}
	}
}
