// Design: docs/architecture/plugin/rib-storage-design.md -- cold replay source authority.
package rs

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// coldRSProtocol uses the real SDK and either the native engine slot or full
// framed request/response. The test owns and joins the one peer lifecycle.
func coldRSProtocol(t *testing.T, native bool, handle func(context.Context, string, any) (any, error)) *sdk.Plugin {
	t.Helper()
	client, host := net.Pipe()
	bridge := rpc.NewDirectBridge()
	var plugin *sdk.Plugin
	if native {
		plugin = sdk.NewWithConn("rs-cold-owner", rpc.NewBridgedConn(client, bridge))
	} else {
		plugin = sdk.NewWithConn("rs-cold-owner", client)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	joined := make(chan struct{})
	if native {
		bridge.SetDispatchNative(func(ctx context.Context, method string, value any) (any, bool, error) {
			output, err := handle(ctx, method, value)
			return output, true, err
		})
		bridge.SetDispatchCommandArgs(func(ctx context.Context, command string, args []string, peer string) (*rpc.DispatchCommandOutput, error) {
			value, err := handle(ctx, rpc.MethodDispatchCommandArgs, rpc.DispatchCommandArgsInput{Command: command, Args: args, Peer: peer})
			if err != nil {
				return nil, err
			}
			output, ok := value.(*rpc.DispatchCommandOutput)
			if !ok {
				return nil, errors.New("eligibility producer returned wrong command shape")
			}
			return output, nil
		})
		bridge.SetRelayStoredRoute(func(ctx context.Context, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
			value, err := handle(ctx, rpc.MethodRelayStoredRoute, input)
			if err != nil {
				return nil, err
			}
			output, ok := value.(*rpc.CachedWriteOutput)
			if !ok {
				return nil, errors.New("cold relay producer returned wrong completion shape")
			}
			return output, nil
		})
		bridge.SetDispatchRPC(func(ctx context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
			if method != rpc.MethodUpdateRoute {
				return nil, errors.New("cold protocol has no raw evaluator for " + method)
			}
			var input rpc.UpdateRouteInput
			if err := json.Unmarshal(raw, &input); err != nil {
				return nil, err
			}
			output, err := handle(ctx, method, input)
			if err != nil {
				return nil, err
			}
			return json.Marshal(output)
		})
		bridge.SetReady()
		close(joined)
	} else {
		go func() {
			defer close(joined)
			wire := rpc.NewConn(host, host)
			for {
				request, err := wire.ReadRequest(ctx)
				if err != nil {
					return
				}
				var input any
				switch request.Method {
				case rpc.MethodSourceLease:
					input = &rpc.SourceLeaseInput{}
				case rpc.MethodDispatchCommandArgs:
					input = &rpc.DispatchCommandArgsInput{}
				case rpc.MethodRelayStoredRoute:
					input = &rpc.RelayStoredRouteInput{}
				case rpc.MethodUpdateRoute:
					input = &rpc.UpdateRouteInput{}
				case rpc.MethodDispatchCommand:
					input = &rpc.DispatchCommandInput{}
				default:
					t.Errorf("unexpected actual SDK operation %s", request.Method)
					return
				}
				if err := json.Unmarshal(request.Params, input); err != nil {
					t.Error(err)
					return
				}
				output, err := handle(ctx, request.Method, input)
				if err != nil {
					if err := wire.SendError(ctx, request.ID, err.Error()); err != nil {
						return
					}
					continue
				}
				if request.Method == rpc.MethodDispatchCommandArgs {
					result, ok := output.(*rpc.DispatchCommandOutput)
					if !ok {
						t.Errorf("wrong actual command answer %T", output)
						return
					}
					if err := rpc.WriteDocumentAnswer(host, request.ID, rpc.AnswerTail{}, result.Data); err != nil {
						return
					}
				} else if err := wire.SendResult(ctx, request.ID, output); err != nil {
					return
				}
			}
		}()
	}
	t.Cleanup(func() {
		cancel()
		if err := plugin.Close(); err != nil {
			t.Error(err)
		}
		if err := host.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-joined:
		case <-time.After(time.Second):
			t.Error("owned protocol lifecycle did not join")
		}
	})
	return plugin
}

// Cold removed B must be rejected without throwing away unseen useful C.
func TestRSColdReplayRejectsWithdrawnPathAndKeepsUnseenSiblingBothTransports(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "native"}[native], func(t *testing.T) {
			rs, key, a, b := retainedRSTestOwner(t)
			rs.peers[key].Up = true
			rs.peers[key].adoptionOpen = true
			rs.peers[key].processedMsgID = 400
			for k := range rs.withdrawals[key.address.String()] {
				if k.prefix.String() == "10.0.2.0/24" {
					delete(rs.withdrawals[key.address.String()], k)
				}
			}
			c := a
			c.NLRIHex = "180a0003"
			c.MsgID = 303
			var delivered []rpc.StoredRoute
			var queried []ribevents.EligibilityQuery
			rs.receiveGate = "request bgp adj-rib-in eligibility"
			rs.plugin = coldRSProtocol(t, native, func(_ context.Context, method string, value any) (any, error) {
				switch method {
				case rpc.MethodDispatchCommandArgs:
					var in rpc.DispatchCommandArgsInput
					switch v := value.(type) {
					case rpc.DispatchCommandArgsInput:
						in = v
					case *rpc.DispatchCommandArgsInput:
						in = *v
					default:
						return nil, errors.New("wrong eligibility input shape")
					}
					var request ribevents.EligibilityRequest
					if err := json.Unmarshal([]byte(in.Args[0]), &request); err != nil {
						return nil, err
					}
					query := request.Queries[0]
					queried = append(queried, query)
					if query.AfterMsgID != 400 {
						t.Errorf("current original source floor lost: %#v", query)
					}
					answer, err := json.Marshal(ribevents.EligibilityAnswer{Answers: []bool{query.Route.Prefix.String() == "10.0.3.0/24"}})
					return &rpc.DispatchCommandOutput{Status: statusDone, Data: answer}, err
				case rpc.MethodRelayStoredRoute:
					var in rpc.RelayStoredRouteInput
					switch v := value.(type) {
					case rpc.RelayStoredRouteInput:
						in = v
					case *rpc.RelayStoredRouteInput:
						in = *v
					default:
						return nil, errors.New("wrong relay input shape")
					}
					delivered = append(delivered, in.Routes...)
					return &rpc.CachedWriteOutput{Completed: true, DeadlineMS: in.DeadlineMS}, nil
				default:
					return nil, errors.New("unexpected engine operation")
				}
			})
			target := rpc.ForwardTarget{Destination: "192.0.2.9", DestinationOwner: 19, DestinationSession: 209}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := rs.relayToTarget(ctx, target, []rpc.StoredRoute{a, b, c}); err != nil {
				t.Fatal(err)
			}
			if len(delivered) != 2 || delivered[0].MsgID != a.MsgID || delivered[1].MsgID != c.MsgID {
				t.Fatalf("cold stale B survived or useful unseen C was lost: %#v", delivered)
			}
			if len(queried) != 2 {
				t.Fatalf("cold candidates did not ask actual current receive owner: %#v", queried)
			}
			rs.withdrawalMu.Lock()
			keptA := rs.currentStoredRouteLocked(a)
			removedB := rs.currentStoredRouteLocked(b)
			keptC := rs.currentStoredRouteLocked(c)
			rs.withdrawalMu.Unlock()
			if !keptA || removedB || !keptC {
				t.Fatalf("current membership differs from useful replay A=%v B=%v C=%v", keptA, removedB, keptC)
			}
		})
	}
}

// The actual source worker withdraws B while its earlier cold eligibility RPC
// is held. That old positive answer cannot re-adopt B. Pending source progress
// is retried within the same finite replay, and unseen current C remains useful.
func TestRSColdReplayRechecksOriginalProgressAcrossHeldQueryBothTransports(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "native"}[native], func(t *testing.T) {
			rs, key, a, b := retainedRSTestOwner(t)
			actor := rs.peers[key]
			actor.Up = true
			actor.adoptionOpen = true
			actor.processedMsgID = 400
			for k := range rs.withdrawals[key.address.String()] {
				if k.prefix.String() == "10.0.2.0/24" {
					delete(rs.withdrawals[key.address.String()], k)
				}
			}
			c := a
			c.NLRIHex = "180a0003"
			c.MsgID = 303
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			queryCount := 0
			var delivered []rpc.StoredRoute
			rs.receiveGate = "request bgp adj-rib-in eligibility"
			rs.plugin = coldRSProtocol(t, native, func(_ context.Context, method string, value any) (any, error) {
				switch method {
				case rpc.MethodDispatchCommandArgs:
					var input rpc.DispatchCommandArgsInput
					switch v := value.(type) {
					case rpc.DispatchCommandArgsInput:
						input = v
					case *rpc.DispatchCommandArgsInput:
						input = *v
					default:
						return nil, errors.New("wrong current-query input shape")
					}
					var request ribevents.EligibilityRequest
					if err := json.Unmarshal([]byte(input.Args[0]), &request); err != nil {
						return nil, err
					}
					queryCount++
					query := request.Queries[0]
					var answer ribevents.EligibilityAnswer
					if queryCount == 1 {
						answer.Pending = true
					} else {
						if queryCount == 2 {
							close(entered)
							<-release
						}
						verdict := query.Route.Prefix.String() == "10.0.3.0/24"
						if queryCount == 2 {
							verdict = true
						}
						if queryCount > 2 && query.AfterMsgID != 401 {
							t.Errorf("new original-source withdrawal progress was not queried: %#v", query)
						}
						answer.Answers = []bool{verdict}
					}
					payload, err := json.Marshal(answer)
					return &rpc.DispatchCommandOutput{Status: statusDone, Data: payload}, err
				case rpc.MethodRelayStoredRoute:
					var input rpc.RelayStoredRouteInput
					switch v := value.(type) {
					case rpc.RelayStoredRouteInput:
						input = v
					case *rpc.RelayStoredRouteInput:
						input = *v
					default:
						return nil, errors.New("wrong bound relay shape")
					}
					delivered = append(delivered, input.Routes...)
					return &rpc.CachedWriteOutput{Completed: true, DeadlineMS: input.DeadlineMS}, nil
				default:
					return nil, errors.New("unexpected actual SDK operation")
				}
			})
			rs.releaseCachedHook = func([]uint64) {}
			rs.workers = newWorkerPool(rs.processForward, poolConfig{chanSize: 2, idleTimeout: time.Hour, onDrained: rs.flushWorkerBatch, onClose: rs.dropParkedValidations})
			defer rs.workers.Stop()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			target := rpc.ForwardTarget{Destination: "192.0.2.9", DestinationOwner: 19, DestinationSession: 209}
			relayDone := make(chan error, 1)
			go func() { relayDone <- rs.relayToTarget(ctx, target, []rpc.StoredRoute{a, b, c}) }()
			relayJoined := false
			defer func() {
				once.Do(func() { close(release) })
				cancel()
				if !relayJoined {
					select {
					case <-relayDone:
					case <-time.After(time.Second):
						t.Error("owned cold replay did not join")
					}
				}
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("actual old-page query was not held", ctx.Err())
			}
			rs.dispatchText("peer 192.0.2.1 remote as 65001 received update 401 peer-owner 11 connection-id 101 source-receipt 201 source-policy-epoch 7 nlri ipv4/unicast del prefix 10.0.2.0/24")
			if err := rs.workers.settleOwner(ctx, key.owner); err != nil {
				t.Fatal(err)
			}
			rs.withdrawalMu.Lock()
			revision, processed := actor.inventoryRevision, actor.processedMsgID
			rs.withdrawalMu.Unlock()
			if revision != 1 || processed != 401 {
				t.Fatalf("actual source withdrawal did not advance its own applied inventory: revision=%d processed=%d", revision, processed)
			}
			once.Do(func() { close(release) })
			select {
			case err := <-relayDone:
				relayJoined = true
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("current useful replay did not finish", ctx.Err())
			}
			if queryCount < 5 {
				t.Fatalf("Pending or changed revision was treated as a completed current lookup: %d queries", queryCount)
			}
			if len(delivered) != 2 || delivered[0].MsgID != a.MsgID || delivered[1].MsgID != c.MsgID {
				t.Fatalf("old query resurrected withdrawn B or lost useful unseen C: %#v", delivered)
			}
		})
	}
}

// A failed Release cannot be assumed to leave active authority. The next SDK
// admission must retry that original release before reacquiring, and a DOWN
// actor must never use the same retry path to acquire a new lease.
func TestRSFailedReleaseDoesNotAuthorizeFreshAdmissionBothTransports(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "native"}[native], func(t *testing.T) {
			rs, key, _, _ := retainedRSTestOwner(t)
			rs.peers[key].Up = true
			rs.withdrawals = make(map[string]map[withdrawalKey]withdrawalEntry)
			var mu sync.Mutex
			var calls []rpc.SourceLeaseAction
			releases := 0
			rs.plugin = coldRSProtocol(t, native, func(_ context.Context, method string, value any) (any, error) {
				if method != rpc.MethodSourceLease {
					return nil, errors.New("unexpected source operation")
				}
				var in rpc.SourceLeaseInput
				switch v := value.(type) {
				case rpc.SourceLeaseInput:
					in = v
				case *rpc.SourceLeaseInput:
					in = *v
				default:
					return nil, errors.New("wrong lease input shape")
				}
				mu.Lock()
				defer mu.Unlock()
				calls = append(calls, in.Action)
				if in.SourceReceipt != 201 || in.PeerOwner != key.owner {
					t.Errorf("original release identity changed: %#v", in)
				}
				if in.Action == rpc.SourceLeaseRelease {
					releases++
					if releases == 1 {
						return nil, errors.New("original writer join refused")
					}
				}
				return &rpc.SourceLeaseOutput{Action: in.Action, SourceReceipt: in.SourceReceipt, PeerOwner: in.PeerOwner}, nil
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := rs.releaseEmptySource(ctx, key); err == nil {
				t.Fatal("failed original writer join reported success")
			}
			if rs.peers[key].sourceLease || !rs.peers[key].sourceReleaseOwed {
				t.Fatal("failed Release left active source authority instead of original join debt")
			}
			if err := rs.acquireSource(key, 201); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			got := append([]rpc.SourceLeaseAction(nil), calls...)
			mu.Unlock()
			if len(got) != 3 || got[0] != rpc.SourceLeaseRelease || got[1] != rpc.SourceLeaseRelease || got[2] != rpc.SourceLeaseAcquire {
				t.Fatalf("fresh admission borrowed revoked lease instead of release-then-acquire: %v", got)
			}
			if !rs.peers[key].sourceLease || rs.peers[key].sourceReleaseOwed {
				t.Fatal("joined release did not permit useful fresh original admission")
			}
			if err := rs.releaseEmptySource(ctx, key); err != nil {
				t.Fatal(err)
			}
			rs.peers[key].Up = false
			rs.peers[key].StateSeen = true
			if err := rs.acquireSource(key, 201); err == nil {
				t.Fatal("DOWN acquired fresh source authority")
			}
			mu.Lock()
			count := len(calls)
			mu.Unlock()
			if count != 4 {
				t.Fatalf("DOWN admission sent another lease RPC: %v", calls)
			}
		})
	}
}

// The optional receive store owns no FlowSpec rule. A cold native rule must
// come from the mandatory selecting RIB's current original path, through the
// same finite query on both transports, with useful current C beside stale B.
func TestRSColdFlowSpecUsesMandatoryCurrentOriginalPathBothTransports(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "native"}[native], func(t *testing.T) {
			for _, kind := range []string{"absent", "wrong-generation", "wrong-receipt", "wrong-epoch"} {
				t.Run(kind, func(t *testing.T) {
					rs, key, a, b := retainedRSTestOwner(t)
					actor := rs.peers[key]
					actor.Up = true
					actor.adoptionOpen = true
					actor.processedMsgID = 400
					for k := range rs.withdrawals[key.address.String()] {
						if k.prefix.String() == "10.0.2.0/24" {
							delete(rs.withdrawals[key.address.String()], k)
						}
					}
					b.Family = (family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}).String()
					b.NLRIHex = "0501180a0000"
					c := b
					c.NLRIHex = "0501180a0001"
					c.MsgID = 303
					rs.flowSpecGate = "request bgp rib flowspec-gate"
					calls := 0
					var delivered []rpc.StoredRoute
					rs.plugin = coldRSProtocol(t, native, func(_ context.Context, method string, value any) (any, error) {
						switch method {
						case rpc.MethodDispatchCommandArgs:
							var input rpc.DispatchCommandArgsInput
							switch v := value.(type) {
							case rpc.DispatchCommandArgsInput:
								input = v
							case *rpc.DispatchCommandArgsInput:
								input = *v
							default:
								return nil, errors.New("wrong mandatory-query input shape")
							}
							if input.Command != rs.flowSpecGate {
								return nil, errors.New("cold FlowSpec incorrectly asked the optional receive store")
							}
							var request ribevents.FlowSpecRequest
							if err := json.Unmarshal([]byte(input.Args[0]), &request); err != nil {
								return nil, err
							}
							query := request.Queries[0]
							if query.Operation != ribevents.FlowSpecPathOf || query.AfterMsgID != 400 || query.Route.PeerOwner != key.owner || query.Route.ConnectionID != key.connectionID {
								t.Errorf("mandatory query lost original source/floor: %#v", query)
							}
							calls++
							answer := ribevents.FlowSpecAnswer{Pending: calls == 1}
							if !answer.Pending {
								path := ribevents.FlowSpecPath{MsgID: query.MsgID, SourceReceipt: 201, SourcePolicyEpoch: 7}
								verdict := true
								if query.Route.NLRI == string([]byte{5, 1, 24, 10, 0, 0}) {
									switch kind {
									case "absent":
										verdict = false
									case "wrong-generation":
										path.MsgID--
									case "wrong-receipt":
										path.SourceReceipt++
									case "wrong-epoch":
										path.SourcePolicyEpoch++
									}
								}
								answer.Results = []ribevents.FlowSpecResult{{Verdict: verdict, Path: path}}
							}
							payload, err := json.Marshal(answer)
							return &rpc.DispatchCommandOutput{Status: statusDone, Data: payload}, err
						case rpc.MethodRelayStoredRoute:
							var input rpc.RelayStoredRouteInput
							switch v := value.(type) {
							case rpc.RelayStoredRouteInput:
								input = v
							case *rpc.RelayStoredRouteInput:
								input = *v
							default:
								return nil, errors.New("wrong mandatory-route relay shape")
							}
							delivered = append(delivered, input.Routes...)
							return &rpc.CachedWriteOutput{Completed: true, DeadlineMS: input.DeadlineMS}, nil
						default:
							return nil, errors.New("unexpected cold FlowSpec operation")
						}
					})
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					target := rpc.ForwardTarget{Destination: "192.0.2.9", DestinationOwner: 19, DestinationSession: 209}
					if err := rs.relayToTarget(ctx, target, []rpc.StoredRoute{a, b, c}); err != nil {
						t.Fatal(err)
					}
					if calls != 3 {
						t.Fatalf("mandatory original source Pending was not retried: %d calls", calls)
					}
					if len(delivered) != 2 || delivered[0].MsgID != a.MsgID || delivered[1].MsgID != c.MsgID {
						t.Fatalf("stale native B survived or useful mandatory-RIB C was lost: %#v", delivered)
					}
				})
			}
		})
	}
}

// A stale-only cold page must release the lease it just acquired, while a
// current same-address sibling keeps its own useful route and owned lease.
func TestRSColdEmptyPageReleasesOnlyItsAcquiredSourceBothTransports(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "native"}[native], func(t *testing.T) {
			rs, key, a, _ := retainedRSTestOwner(t)
			actor := rs.peers[key]
			actor.Up = true
			actor.adoptionOpen = true
			actor.sourceLease = false
			actor.sourceReceipt = 0
			sibling, err := newWorkerKey(key.address.String(), 12, 102)
			if err != nil {
				t.Fatal(err)
			}
			rs.peers[sibling] = &PeerState{Address: key.address.String(), Up: true, sourceLease: true, sourceReceipt: 202}
			useful := withdrawalKey{owner: sibling.owner, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("10.0.9.0/24")}
			rs.withdrawals[key.address.String()] = map[withdrawalKey]withdrawalEntry{useful: {owner: sibling.owner, connectionID: sibling.connectionID, sourceReceipt: 202, msgID: 309}}
			rs.receiveGate = "request bgp adj-rib-in eligibility"
			var calls []rpc.SourceLeaseAction
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
						return nil, errors.New("wrong empty-page lease shape")
					}
					if input.PeerOwner != key.owner || input.SourceReceipt != 201 {
						return nil, errors.New("empty stale page borrowed sibling original authority")
					}
					calls = append(calls, input.Action)
					return &rpc.SourceLeaseOutput{Action: input.Action, PeerOwner: input.PeerOwner, SourceReceipt: input.SourceReceipt}, nil
				case rpc.MethodDispatchCommandArgs:
					payload, err := json.Marshal(ribevents.EligibilityAnswer{Answers: []bool{false}})
					return &rpc.DispatchCommandOutput{Status: statusDone, Data: payload}, err
				default:
					return nil, errors.New("stale-only page attempted a destination write")
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			target := rpc.ForwardTarget{Destination: "192.0.2.9", DestinationOwner: 19, DestinationSession: 209}
			if err := rs.relayToTarget(ctx, target, []rpc.StoredRoute{a}); err != nil {
				t.Fatal(err)
			}
			if len(calls) != 2 || calls[0] != rpc.SourceLeaseAcquire || calls[1] != rpc.SourceLeaseRelease {
				t.Fatalf("stale-only cold admission kept its newly acquired zero-row lease: %v", calls)
			}
			if actor.sourceLease || actor.sourceReleaseOwed || actor.pendingAdoptions != 0 {
				t.Fatal("completed empty page kept active/owed admission authority")
			}
			if !rs.peers[sibling].sourceLease {
				t.Fatal("empty original page released the useful same-address sibling lease")
			}
			if len(rs.withdrawals[key.address.String()]) != 1 {
				t.Fatal("empty original page changed sibling current membership")
			}
			if _, present := rs.withdrawals[key.address.String()][useful]; !present {
				t.Fatal("empty original page erased useful sibling route")
			}
		})
	}
}
