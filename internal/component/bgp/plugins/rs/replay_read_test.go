// Design: docs/architecture/plugin/rib-storage-design.md -- RS optional read reconciliation.
package rs

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A refused optional read must preserve useful mandatory rule/EOR/ready output
// on both actual SDK transports. A later successful current read clears its
// original-UP obligation. Required relay failures are a separate output debt.
func TestRSOptionalReadRefusalCompletesMandatoryReplayAndReconciles(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "native"}[native], func(t *testing.T) {
			rs, source, _, _ := retainedRSTestOwner(t)
			rs.peers[source].Up = true
			rs.peers[source].adoptionOpen = true
			target := rsFixtureKey("192.0.2.9", 19, 109)
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			rule := ribevents.ValidationRoute{Peer: netip.MustParseAddr("192.0.2.1"), PeerOwner: 11, ConnectionID: 101, Family: fam, NLRI: string([]byte{5, 1, 24, 10, 0, 0})}
			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, 14, 11, 0, 1, 133, 0, 0, 5, 1, 24, 10, 0, 0}
			ribevents.RegisterFlowSpecLookup(func(candidate ribevents.ValidationRoute, id uint64) bool { return candidate == rule && id == 42 }, func(candidate ribevents.ValidationRoute) bool { return candidate == rule }, func(candidate ribevents.ValidationRoute) (ribevents.FlowSpecPath, bool) {
				return ribevents.FlowSpecPath{Attributes: attrs, MsgID: 42, SourceReceipt: 201, SourcePolicyEpoch: 7}, candidate == rule
			}, func() []ribevents.ValidationRoute { return []ribevents.ValidationRoute{rule} })
			t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
			rs.peers[target] = &PeerState{Address: target.address.String(), Up: true, RSClient: true, StateSeen: true, DestinationSession: 209, InitialReplay: 17, ForwardFrom: 42, ReplayGen: 1, Families: map[family.Family]bool{fam: true}}
			refuseRead := true
			var delivered []string
			rs.plugin = coldRSProtocol(t, native, func(_ context.Context, method string, value any) (any, error) {
				switch method {
				case rpc.MethodSourceLease:
					var input rpc.SourceLeaseInput
					switch v := value.(type) {
					case rpc.SourceLeaseInput:
						input = v
					case *rpc.SourceLeaseInput:
						input = *v
					default:
						return nil, errors.New("wrong original lease shape")
					}
					return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, nil
				case rpc.MethodDispatchCommandArgs:
					if refuseRead {
						return nil, errors.New("optional receive-store page unavailable")
					}
					return &rpc.DispatchCommandOutput{Status: statusDone, Data: []byte(`{"routes":[],"last-index":0,"more":false,"ingested-msg-id":42}`)}, nil
				case rpc.MethodRelayStoredRoute:
					var input rpc.RelayStoredRouteInput
					switch v := value.(type) {
					case rpc.RelayStoredRouteInput:
						input = v
					case *rpc.RelayStoredRouteInput:
						input = *v
					default:
						return nil, errors.New("wrong required relay shape")
					}
					if len(input.Routes) != 1 || input.Routes[0].MsgID != 42 {
						return nil, errors.New("mandatory useful rule changed")
					}
					delivered = append(delivered, "rule")
					return &rpc.CachedWriteOutput{Completed: true, DeadlineMS: input.DeadlineMS}, nil
				case rpc.MethodUpdateRoute:
					delivered = append(delivered, "eor")
					return &rpc.UpdateRouteOutput{}, nil
				case rpc.MethodDispatchCommand:
					input, err := rsFixtureCommandInput(value)
					if err != nil {
						return nil, err
					}
					if input.DestinationOwner != 19 || input.DestinationSession != 209 {
						return nil, errors.New("ready escaped original target UP")
					}
					delivered = append(delivered, "ready")
					return rsFixtureBoundCommand(input), nil
				default:
					return nil, errors.New("unexpected mandatory replay operation")
				}
			})
			rsFixtureCurrentFlowSpec(t, rs, source, 201, 7, 42, fam, "0501180a0000")
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, replayTargetKey{}, rsFixtureTarget(target.address.String(), 19, 209))
			rs.replayForPeer(ctx, target, 1, 42, 17, make(chan struct{}))
			if !slices.Equal(delivered, []string{"rule", "eor", "ready"}) {
				t.Fatalf("optional read blocked useful mandatory replay: %v", delivered)
			}
			if rs.peers[target].replayFailure == nil {
				t.Fatal("failed current read lost its original-UP reconciliation obligation")
			}
			refuseRead = false
			if err := rs.retryCurrentReplays(ctx); err != nil {
				t.Fatal(err)
			}
			if rs.peers[target].replayFailure != nil {
				t.Fatal("successful current read kept a lifetime-sticky read failure")
			}
			if !slices.Equal(delivered, []string{"rule", "eor", "ready", "rule", "eor", "ready"}) {
				t.Fatalf("current retry lost useful mandatory output: %v", delivered)
			}
		})
	}
}
