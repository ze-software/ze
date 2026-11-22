// Design: docs/architecture/plugin/rib-storage-design.md -- original RPKI actors.
// Related: rpki.go, rpki_reload.go -- actual input and revalidation producers.
package rpki

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const rpkiOwnerAddress = "192.0.2.50"

// rpkiOwnerBody is a real IPv4 UPDATE from [100,200,300] for 10.0.0.0/24.
func rpkiOwnerBody() []byte {
	return []byte{0, 0, 0, 28,
		0x40, 1, 1, 0, 0x40, 2, 14, 2, 3,
		0, 0, 0, 100, 0, 0, 0, 200, 0, 0, 1, 44,
		0x40, 3, 4, 192, 0, 2, 50, 24, 10, 0, 0}
}

// rpkiOwnerFeed drives the actual typed SDK handler or the public full-JSON
// decoder and handler. It retains no current-address authority in a helper.
func rpkiOwnerFeed(t *testing.T, rp *rPKIPlugin, rail, name string, source rpkiSource, msgID uint64) {
	t.Helper()
	rpkiOwnerFeedRoute(t, rp, rail, name, source, msgID, rpkiOwnerBody(), "10.0.0.0/24")
}

// rpkiOwnerFeedRoute supplies actual attributes and NLRI to both input rails.
func rpkiOwnerFeedRoute(t *testing.T, rp *rPKIPlugin, rail, name string, source rpkiSource, msgID uint64, body []byte, prefix string) {
	t.Helper()
	wire := wireu.NewWireUpdate(body, bgpctx.APIContextID)
	attrs, err := wire.Attrs()
	require.NoError(t, err)
	if rail == "typed" {
		rp.handleStructuredEvent(&rpc.StructuredEvent{
			EventType: rpc.EventKindUpdate, PeerAddress: rpkiOwnerAddress,
			PeerName: name, PeerGroup: "ix", PeerOwner: source.owner, ConnectionID: source.connection,
			PeerAS: rpc.RemoteASOf(100), LocalAS: 65000, MessageID: msgID,
			RawMessage: &bgptypes.RawMessage{MessageID: msgID, RawBytes: body, WireUpdate: wire, AttrsWire: attrs},
		})
		return
	}
	primary := fmt.Sprintf(`{"type":"bgp","bgp":{
        "peer-owner":%d,"connection-id":"%d",
        "peer":{"name":%q,"group":"ix","remote":{"address":"192.0.2.50","as":100},"local":{"as":65000}},
        "message":{"type":"update","id":%d,"direction":"received"},
        "raw":{"attributes":%q},
        "update":{"nlri":{"ipv4/unicast":[{"action":"add","next-hop":"192.0.2.50","nlri":[%q]}]}}}}`,
		source.owner, source.connection, name, msgID, hex.EncodeToString(attrs.Packed()), prefix)
	event, err := bgp.ParseEvent([]byte(primary))
	require.NoError(t, err)
	require.Equal(t, source.owner, event.PeerOwner)
	require.Equal(t, source.connection, event.ConnectionID)
	rp.handleEvent(event)
}

func rpkiOwnerDown(t *testing.T, rp *rPKIPlugin, rail string, source rpkiSource, reason string) {
	t.Helper()
	if rail == "typed" {
		rp.handleStructuredEvent(&rpc.StructuredEvent{EventType: rpc.EventKindState,
			State: rpc.SessionStateDown, PeerAddress: rpkiOwnerAddress,
			PeerOwner: source.owner, ConnectionID: source.connection, Reason: reason})
		return
	}
	raw := fmt.Sprintf(`{"type":"state","peer":{"remote":{"address":"192.0.2.50","as":100}},
        "peer-owner":%d,"connection-id":"%d","state":"down","reason":%q}`,
		source.owner, source.connection, reason)
	event, err := bgp.ParseEvent([]byte(raw))
	require.NoError(t, err)
	rp.handleEvent(event)
}

func rpkiOwnerPlugin(t *testing.T) (*rPKIPlugin, *rpc.DirectBridge) {
	t.Helper()
	rp, bridge := aspaRolePlugin(t, "provider")
	rp.cache.Add(makeVRP("10.0.0.0/24", 24, 300))
	rp.aspaCache.Set(200, []uint32{100})
	rp.aspaCache.Set(300, []uint32{200})
	return rp, bridge
}

// rpkiOwnerDecisionTransport reads the actual SDK BatchValidate call, including
// its newline request and width-framed answer on the socket placement. The
// connection worker MUST stop and join in cleanup; it serves a bounded request
// population. Every new socket MUST be closed before that join on failure too.
func rpkiOwnerDecisionTransport(t *testing.T, rp *rPKIPlugin, bridge *rpc.DirectBridge, transport string, requests int) <-chan []rpc.ValidationDecision {
	t.Helper()
	captured := make(chan []rpc.ValidationDecision, 8)
	if transport == "typed" {
		bridge.SetBatchValidate(func(decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
			captured <- slices.Clone(decisions)
			return &rpc.BatchValidateResult{Accepted: len(decisions)}, nil
		})
		return captured
	}
	client, engine := net.Pipe()
	plugin := sdk.NewWithConn("rpki-owner", client)
	rp.plugin = plugin
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	done := make(chan error, 1)
	go func() {
		connection := rpc.NewConn(engine, engine)
		for range requests {
			request, err := connection.ReadRequest(ctx)
			if err != nil {
				done <- err
				return
			}
			var answer any
			switch request.Method {
			case rpc.MethodBatchValidate:
				var input rpc.BatchValidateInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					done <- err
					return
				}
				captured <- input.Decisions
				answer = &rpc.BatchValidateResult{Accepted: len(input.Decisions)}
			case rpc.MethodEmitEvent:
				answer = &rpc.EmitEventOutput{Delivered: 1}
			default:
				done <- fmt.Errorf("unexpected RPKI producer request %s", request.Method)
				return
			}
			if err := connection.SendResult(ctx, request.ID, answer); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	t.Cleanup(func() {
		cancel()
		if err := plugin.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil {
			if !t.Failed() {
				t.Error(err)
			}
		}
	})
	return captured
}

func rpkiOwnerNext(t *testing.T, captured <-chan []rpc.ValidationDecision) []rpc.ValidationDecision {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case decisions := <-captured:
		return decisions
	case <-ctx.Done():
		t.Fatal("no actual RPKI decision RPC")
	}
	return nil
}

func rpkiOwnerDecisionSources(t *testing.T, decisions []rpc.ValidationDecision, want map[rpkiSource]uint64) {
	t.Helper()
	require.Len(t, decisions, len(want))
	seen := make(map[rpkiSource]uint64, len(decisions))
	for _, decision := range decisions {
		require.Equal(t, rpkiOwnerAddress, decision.PeerAddr)
		require.Equal(t, "10.0.0.0/24", decision.Prefix)
		require.Equal(t, uint32(0), decision.PathID, "native zero is an actual identifier")
		seen[rpkiSource{owner: decision.PeerOwner, connection: decision.ConnectionID}] = decision.MsgID
	}
	require.Equal(t, want, seen)
}

// TestRPKIOriginalActorsReachBothDecisionRails preserves same-address siblings
// through actual input, both trackers, queue and typed/framed SDK decisions.
func TestRPKIOriginalActorsReachBothDecisionRails(t *testing.T) {
	for _, rail := range []string{"typed", "full-json"} {
		for _, transport := range []string{"typed", "framed"} {
			t.Run(rail+"/"+transport, func(t *testing.T) {
				rp, bridge := rpkiOwnerPlugin(t)
				left := rpkiSource{owner: ^uint64(0) - 1, connection: ^uint64(0) - 2}
				right := rpkiSource{owner: 702, connection: 42}
				rpkiOwnerFeed(t, rp, rail, "left", left, 71)
				rpkiOwnerFeed(t, rp, rail, "right", right, 72)
				require.Equal(t, 2, rp.originTracker.count())
				require.Equal(t, 2, rp.aspaTracker.count())
				requests := drainRequests(rp.validateCh)
				captured := rpkiOwnerDecisionTransport(t, rp, bridge, transport, 1)
				rp.dispatchBatch(requests)
				decisions := rpkiOwnerNext(t, captured)
				rpkiOwnerDecisionSources(t, decisions, map[rpkiSource]uint64{left: 71, right: 72})
				for _, decision := range decisions {
					require.True(t, decision.Accept)
					require.Equal(t, ValidationValid, decision.ValState)
				}
			})
		}
	}
}

// TestRPKICacheCallbacksKeepOriginalActors checks both actual cache callbacks,
// retained named policy and latest-message guard without another source UPDATE.
func TestRPKICacheCallbacksKeepOriginalActors(t *testing.T) {
	for _, rail := range []string{"typed", "full-json"} {
		t.Run(rail, func(t *testing.T) {
			rp, _ := rpkiOwnerPlugin(t)
			left := rpkiSource{owner: 701, connection: 41}
			right := rpkiSource{owner: 702, connection: 41}
			rpkiOwnerFeed(t, rp, rail, "left", left, 71)
			rpkiOwnerFeed(t, rp, rail, "right", right, 72)
			drainRequests(rp.validateCh)
			policies := map[configjson.PeerConfigKey]peerActionSet{
				{ID: "left"}:  {OriginInvalid: resolvedAction{Action: ASPAPolicyReject}, ASPAInvalid: resolvedAction{Action: ASPAPolicyReject}},
				{ID: "right"}: {OriginInvalid: resolvedAction{Action: ASPAPolicyAccept}, ASPAInvalid: resolvedAction{Action: ASPAPolicyAccept}},
			}
			rp.perPeerActions.Store(&policies)
			rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 999)})
			rp.handleROAChange()
			decisions := rp.buildDecisions(drainRequests(rp.validateCh))
			rpkiOwnerDecisionSources(t, decisions, map[rpkiSource]uint64{left: 71, right: 72})
			for _, decision := range decisions {
				require.Equal(t, decision.PeerOwner == right.owner, decision.Accept)
				require.Equal(t, ValidationInvalid, decision.ValState)
			}
			rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 300)})
			rp.handleROAChange()
			drainRequests(rp.validateCh)
			rp.aspaCache.Set(300, []uint32{999})
			rp.handleASPAChange([]uint32{300})
			decisions = rp.buildDecisions(drainRequests(rp.validateCh))
			rpkiOwnerDecisionSources(t, decisions, map[rpkiSource]uint64{left: 71, right: 72})
			for _, decision := range decisions {
				require.Equal(t, decision.PeerOwner == right.owner, decision.Accept)
			}
			key := routeKey{source: left, peerAddr: rpkiOwnerAddress, family: "ipv4/unicast", prefix: "10.0.0.0/24"}
			rp.originTracker.Track(key, originRoute{msgID: 70, state: ValidationInvalid})
			require.Equal(t, uint64(71), rp.originTracker.routes[key].msgID, "old generation cannot replace original tracker state")
		})
	}
}

// TestRPKIDownKeepsSiblingAndSuccessor uses actual state handlers. Old DOWN
// cannot clear fresh same-owner physical42 or a same-address independent peer;
// terminal removal reaches only its original owner, and observations remain useful.
func TestRPKIDownKeepsSiblingAndSuccessor(t *testing.T) {
	for _, rail := range []string{"typed", "full-json"} {
		t.Run(rail, func(t *testing.T) {
			rp, _ := rpkiOwnerPlugin(t)
			old := rpkiSource{owner: 701, connection: 41}
			fresh := rpkiSource{owner: 701, connection: 42}
			sibling := rpkiSource{owner: 702, connection: 41}
			observed := rpkiSource{}
			for i, source := range []rpkiSource{old, fresh, sibling, observed} {
				rpkiOwnerFeed(t, rp, rail, "source", source, uint64(71+i))
			}
			requests := drainRequests(rp.validateCh)
			require.Len(t, requests, 4)
			require.Equal(t, 4, rp.originTracker.count())
			require.Equal(t, 4, rp.aspaTracker.count())
			rpkiOwnerDown(t, rp, rail, old, "")
			require.Equal(t, 3, rp.originTracker.count())
			require.Equal(t, 3, rp.aspaTracker.count())
			// A queued old decision retains its original actor, even after UP42;
			// only the receiving owner may decide whether that original still exists.
			oldDecision := rp.buildDecisions(requests[:1])
			rpkiOwnerDecisionSources(t, oldDecision, map[rpkiSource]uint64{old: 71})
			rpkiOwnerDown(t, rp, rail, old, "")
			require.Equal(t, 3, rp.originTracker.count())
			rpkiOwnerDown(t, rp, rail, rpkiSource{owner: 701}, rpc.ReasonPeerRemoved)
			require.Equal(t, 2, rp.originTracker.count())
			require.Equal(t, 2, rp.aspaTracker.count())
			rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 999)})
			rp.handleROAChange()
			decisions := rp.buildDecisions(drainRequests(rp.validateCh))
			rpkiOwnerDecisionSources(t, decisions, map[rpkiSource]uint64{sibling: 73, observed: 74})
			rpkiOwnerDown(t, rp, rail, observed, "")
			require.Equal(t, 1, rp.originTracker.count())
			require.Equal(t, 1, rp.aspaTracker.count())
			rpkiOwnerDown(t, rp, rail, sibling, "")
			require.Zero(t, rp.originTracker.count())
			require.Zero(t, rp.aspaTracker.count())
		})
	}
}

// TestRPKIConfigSnapshotKeepsOriginalActors exercises the real retained snapshot
// decoder, tracker reconstruction and typed/framed RPC producer. This is the
// RPKI side of config revalidation, not an Adj store, GR or daemon proof.
func TestRPKIConfigSnapshotKeepsOriginalActors(t *testing.T) {
	for _, transport := range []string{"typed", "framed"} {
		t.Run(transport, func(t *testing.T) {
			rp, bridge := rpkiOwnerPlugin(t)
			left := rpkiSource{owner: ^uint64(0) - 1, connection: ^uint64(0) - 2}
			right := rpkiSource{owner: 702, connection: 42}
			wire := wireu.NewWireUpdate(rpkiOwnerBody(), bgpctx.APIContextID)
			attrs, err := wire.Attrs()
			require.NoError(t, err)
			rows := make([]map[string]any, 0, 2)
			for i, source := range []rpkiSource{left, right} {
				rows = append(rows, map[string]any{"peer": rpkiOwnerAddress, "peer-name": "source", "peer-group": "ix",
					"peer-owner": fmt.Sprint(source.owner), "connection-id": fmt.Sprint(source.connection),
					"peer-as": 100, "local-as": 65000, "family": "ipv4/unicast", "prefix": "10.0.0.0/24",
					"path-id": 0, "msg-id": 71 + i, "attr-hex": hex.EncodeToString(attrs.Packed())})
			}
			snapshot, err := json.Marshal(map[string]any{"routes": rows})
			require.NoError(t, err)
			originalPlugin := rp.plugin
			captured := rpkiOwnerDecisionTransport(t, rp, bridge, transport, 3)
			require.NoError(t, rp.revalidateConfigSnapshot(t.Context(), snapshot))
			rpkiOwnerDecisionSources(t, rpkiOwnerNext(t, captured), map[rpkiSource]uint64{left: 71, right: 72})
			require.Equal(t, 2, rp.originTracker.count())
			require.Equal(t, 2, rp.aspaTracker.count())
			rp.cache.Replace([]VRP{makeVRP("10.0.0.0/24", 24, 999)})
			// Emit through the existing original bridge again: the bounded
			// socket recorder above already served its complete three requests.
			rp.plugin = originalPlugin
			rp.handleROAChange()
			rpkiOwnerDecisionSources(t, rp.buildDecisions(drainRequests(rp.validateCh)), map[rpkiSource]uint64{left: 71, right: 72})
		})
	}
}

// TestRPKINamedBlackholeKeepsOriginalSession requires the actual input and
// queued decision to use that source's agreement, with the same address,
// attributes and length-only invalid origin on the nonagreeing sibling.
func TestRPKINamedBlackholeKeepsOriginalSession(t *testing.T) {
	for _, rail := range []string{"typed", "full-json"} {
		t.Run(rail, func(t *testing.T) {
			rp, _ := rpkiOwnerPlugin(t)
			policies := map[configjson.PeerConfigKey]peerActionSet{
				{ID: "left"}: {
					OriginInvalid:        resolvedAction{Action: ASPAPolicyReject},
					ASPAUnknown:          resolvedAction{Action: ASPAPolicyAccept},
					ASPAMode:             aspaDownstream,
					BlackholeExempt:      true,
					BlackholeCommunities: []attribute.Community{attribute.Community(65001<<16 | 666)},
				},
				{ID: "right"}: {
					OriginInvalid:        resolvedAction{Action: ASPAPolicyReject},
					ASPAUnknown:          resolvedAction{Action: ASPAPolicyAccept},
					ASPAMode:             aspaDownstream,
					BlackholeExempt:      true,
					BlackholeCommunities: []attribute.Community{attribute.Community(65002<<16 | 666)},
				},
			}
			rp.perPeerActions.Store(&policies)
			wire := wireu.NewWireUpdate(rpkiOwnerBody(), bgpctx.APIContextID)
			attrs, err := wire.Attrs()
			require.NoError(t, err)
			body := []byte{0, 0, 0, 35}
			body = append(body, attrs.Packed()...)
			body = append(body, 0xc0, 8, 4, 0xfd, 0xe9, 2, 0x9a, 32, 10, 0, 0, 1)
			rpkiOwnerFeedRoute(t, rp, rail, "left", rpkiSource{owner: 701, connection: 41}, 71, body, "10.0.0.1/32")
			rpkiOwnerFeedRoute(t, rp, rail, "right", rpkiSource{owner: 702, connection: 41}, 72, body, "10.0.0.1/32")
			requests := drainRequests(rp.validateCh)
			require.Len(t, requests, 2)
			for _, request := range requests {
				require.Equal(t, request.peerName == "left", request.blackhole)
			}
			decisions := rp.buildDecisions(requests)
			require.Len(t, decisions, 2)
			for _, decision := range decisions {
				require.Equal(t, "10.0.0.1/32", decision.Prefix)
				require.Equal(t, ValidationInvalid, decision.ValState)
				require.Equal(t, decision.PeerOwner == 701, decision.Accept)
			}
		})
	}
}
