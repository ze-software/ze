// Design: docs/architecture/plugin/rib-storage-design.md -- RS optional read reconciliation.
package rs

import (
	"context"
	"encoding/json"
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

// TestRSReplayRetriesWhenSourceDepartsMidPage verifies that a destination's
// peer-up replay does not abort and wedge its session-ready fence when one
// source peer disconnects mid-page. Once the departed source's DOWN clears its
// live state, retrying the page skips the dead routes, sends EOR, and signals
// session ready.
//
// VALIDATES: a joined departed-source relay error retries boundedly while the
// destination stays up, instead of abandoning signalSessionReady forever.
// PREVENTS: a peer disconnecting during another peer's replay from wedging the
// replaying peer in forwardOrderHold for the rest of its session.
func TestRSReplayRetriesWhenSourceDepartsMidPage(t *testing.T) {
	rs, source, _, _ := retainedRSTestOwner(t)
	rs.peers[source].Up = true
	rs.peers[source].adoptionOpen = true
	rs.receiveGate = "request bgp adj-rib-in eligibility"
	target := rsFixtureKey("192.0.2.9", 19, 109)
	rs.peers[target] = &PeerState{
		Address:            target.address.String(),
		Up:                 true,
		RSClient:           true,
		StateSeen:          true,
		DestinationSession: 209,
		InitialReplay:      17,
		ForwardFrom:        42,
		ReplayGen:          1,
		Families:           map[family.Family]bool{family.IPv4Unicast: true},
	}

	relayCalls := 0
	var delivered []string
	rs.plugin = coldRSProtocol(t, true, func(_ context.Context, method string, value any) (any, error) {
		switch method {
		case rpc.MethodSourceLease:
			var input rpc.SourceLeaseInput
			switch v := value.(type) {
			case rpc.SourceLeaseInput:
				input = v
			case *rpc.SourceLeaseInput:
				input = *v
			default:
				return nil, errors.New("wrong lease shape")
			}
			return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, nil
		case rpc.MethodDispatchCommandArgs:
			var in rpc.DispatchCommandArgsInput
			switch v := value.(type) {
			case rpc.DispatchCommandArgsInput:
				in = v
			case *rpc.DispatchCommandArgsInput:
				in = *v
			default:
				return nil, errors.New("wrong command args shape")
			}
			if in.Command == rs.receiveGate {
				rs.mu.RLock()
				up := rs.peers[source].Up
				rs.mu.RUnlock()
				if up {
					return &rpc.DispatchCommandOutput{Status: statusDone, Data: []byte(`{"answers":[true]}`)}, nil
				}
				return &rpc.DispatchCommandOutput{Status: statusDone, Data: []byte(`{"answers":[false]}`)}, nil
			}
			page, marshalErr := json.Marshal(struct {
				Routes        []rpc.StoredRoute `json:"routes"`
				LastIndex     uint64            `json:"last-index"`
				More          bool              `json:"more"`
				IngestedMsgID uint64            `json:"ingested-msg-id"`
			}{
				Routes: []rpc.StoredRoute{{
					SourcePeer:         "192.0.2.1",
					SourceOwner:        11,
					SourceConnectionID: 101,
					SourceReceipt:      201,
					SourcePolicyEpoch:  7,
					MsgID:              42,
					Family:             "ipv4/unicast",
					AttrHex:            "40010100",
					NextHopHex:         "01010101",
					NLRIHex:            "180a0000",
					NLRIFraming:        rpc.NLRIFramingPrefixOnly,
				}},
				IngestedMsgID: 42,
			})
			if marshalErr != nil {
				return nil, marshalErr
			}
			return &rpc.DispatchCommandOutput{
				Status: statusDone,
				Data:   page,
			}, nil
		case rpc.MethodRelayStoredRoute:
			var input rpc.RelayStoredRouteInput
			switch v := value.(type) {
			case rpc.RelayStoredRouteInput:
				input = v
			case *rpc.RelayStoredRouteInput:
				input = *v
			default:
				return nil, errors.New("wrong relay shape")
			}
			relayCalls++
			// Simulate the source disconnecting mid-page on the first attempt:
			// the reactor returns a joined error, and bgp-rs receives DOWN before
			// the retry.
			rs.mu.Lock()
			rs.peers[source].Up = false
			rs.peers[source].adoptionOpen = false
			rs.mu.Unlock()
			rs.withdrawalMu.Lock()
			delete(rs.withdrawals, source.address.String())
			rs.withdrawalMu.Unlock()
			return &rpc.CachedWriteOutput{
				Completed:  true,
				DeadlineMS: input.DeadlineMS,
				Failure:    "original cold relay source ended without retained membership; use its existing retained lease before replay",
			}, nil
		case rpc.MethodUpdateRoute:
			delivered = append(delivered, "eor")
			return &rpc.UpdateRouteOutput{}, nil
		case rpc.MethodDispatchCommand:
			input, err := rsFixtureCommandInput(value)
			if err != nil {
				return nil, err
			}
			delivered = append(delivered, "ready")
			return rsFixtureBoundCommand(input), nil
		default:
			return nil, errors.New("unexpected operation: " + method)
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, replayTargetKey{}, rsFixtureTarget(target.address.String(), 19, 209))
	rs.replayForPeer(ctx, target, 1, 42, 17, make(chan struct{}))

	if relayCalls != 1 {
		t.Fatalf("expected 1 failed relay call before departed source was filtered on retry, got %d", relayCalls)
	}
	if !slices.Equal(delivered, []string{"eor", "ready"}) {
		t.Fatalf("destination replay did not finish after source departed mid-page: delivered=%v", delivered)
	}
	if rs.peers[target].replayFailure != nil {
		t.Fatalf("unexpected lingering replayFailure after retry succeeded: %v", rs.peers[target].replayFailure)
	}
}
