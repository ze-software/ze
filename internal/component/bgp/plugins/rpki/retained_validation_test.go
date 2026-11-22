// Design: docs/architecture/plugin/rib-storage-design.md -- GR RPKI cache revalidation.
package rpki

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func retainedRPKIUp(t *testing.T, rp *rPKIPlugin, rail string, source rpkiSource) {
	t.Helper()
	if rail == "typed" {
		rp.handleStructuredEvent(&rpc.StructuredEvent{EventType: rpc.EventKindState, State: rpc.SessionStateUp,
			PeerAddress: rpkiOwnerAddress, PeerOwner: source.owner, ConnectionID: source.connection})
		return
	}
	body := fmt.Sprintf(`{"type":"state","peer":{"remote":{"address":"192.0.2.50","as":100}},"peer-owner":%d,"connection-id":"%d","state":"up"}`, source.owner, source.connection)
	event, err := bgp.ParseEvent([]byte(body))
	require.NoError(t, err)
	rp.handleEvent(event)
}

func retainedRPKIEvent(t *testing.T, rp *rPKIPlugin, event, payload string) {
	t.Helper()
	body, err := json.Marshal(rpc.EventEnvelope{Namespace: retention.Namespace, Event: event, Payload: json.RawMessage(payload)})
	require.NoError(t, err)
	matched, err := rp.handleRetainedValidationEvent(string(body))
	require.True(t, matched)
	require.NoError(t, err)
}

// The actual original inputs stay in both trackers after retained DOWN; actual
// cache callbacks still publish changed validation with original message IDs.
// NO_LLGR cleanup then removes only that route, and a late all-family cleanup
// leaves the useful fresh physical source and independent same-address peer.
func TestRPKIRetainedCacheChangeAndCleanupBothRails(t *testing.T) {
	for _, rail := range []string{"typed", "full-json"} {
		t.Run(rail, func(t *testing.T) {
			rp, _ := rpkiOwnerPlugin(t)
			old := rpkiSource{owner: 1701, connection: 41}
			fresh := rpkiSource{owner: 1701, connection: 43}
			sibling := rpkiSource{owner: 1702, connection: 42}
			retainedRPKIUp(t, rp, rail, old)
			retainedRPKIUp(t, rp, rail, sibling)
			body := rpkiOwnerBody()
			tail := append([]byte(nil), body[32:]...)
			body = append(body[:32:32], 0xc0, 8, 4, 0xff, 0xff, 0, 7)
			body = append(body, tail...)
			body[3] += 7
			rpkiOwnerFeedRoute(t, rp, rail, "old", old, 71, body, "10.0.0.0/24")
			plain := rpkiOwnerBody()
			plain[len(plain)-1] = 1
			rpkiOwnerFeedRoute(t, rp, rail, "old", old, 72, plain, "10.0.1.0/24")
			rpkiOwnerFeed(t, rp, rail, "sibling", sibling, 73)
			drainRequests(rp.validateCh)
			payload, err := retention.Encode(old.owner, old.connection, rpkiOwnerAddress, []family.Family{family.IPv4Unicast})
			require.NoError(t, err)
			retainedRPKIEvent(t, rp, retention.EventType, payload)
			rpkiOwnerDown(t, rp, rail, old, rpc.ReasonTCPFailure)
			require.Equal(t, 3, rp.originTracker.count())
			require.Equal(t, 3, rp.aspaTracker.count())
			rpkiOwnerDown(t, rp, rail, old, rpc.ReasonTCPFailure)
			require.Equal(t, 3, rp.originTracker.count(), "duplicate DOWN cannot clear retained inputs")
			require.Equal(t, 3, rp.aspaTracker.count())
			rp.cache.Replace([]VRP{makeVRP("10.0.0.0/16", 24, 999)})
			rp.handleROAChange()
			decisions := rp.buildDecisions(drainRequests(rp.validateCh))
			require.Len(t, decisions, 3)
			for _, decision := range decisions {
				require.Equal(t, ValidationInvalid, decision.ValState)
				require.NotZero(t, decision.ConnectionID)
			}
			payload, err = retention.EncodeCleanup(old.owner, old.connection, rpkiOwnerAddress, []family.Family{family.IPv4Unicast}, true)
			require.NoError(t, err)
			retainedRPKIEvent(t, rp, retention.CleanupEventType, payload)
			require.Equal(t, 2, rp.originTracker.count())
			require.Equal(t, 2, rp.aspaTracker.count())
			retainedRPKIUp(t, rp, rail, fresh)
			rpkiOwnerFeedRoute(t, rp, rail, "fresh", fresh, 74, plain, "10.0.1.0/24")
			drainRequests(rp.validateCh)
			payload, err = retention.EncodeCleanup(old.owner, old.connection, rpkiOwnerAddress, nil, false)
			require.NoError(t, err)
			retainedRPKIEvent(t, rp, retention.CleanupEventType, payload)
			require.Equal(t, 2, rp.originTracker.count())
			require.Equal(t, 2, rp.aspaTracker.count())
			rp.cache.Replace([]VRP{makeVRP("10.0.0.0/16", 24, 300)})
			rp.handleROAChange()
			decisions = rp.buildDecisions(drainRequests(rp.validateCh))
			require.Len(t, decisions, 2)
			type wantRoute struct {
				prefix string
				msgID  uint64
			}
			got := make(map[rpkiSource]wantRoute, len(decisions))
			for _, decision := range decisions {
				require.Equal(t, rpkiOwnerAddress, decision.PeerAddr)
				require.Equal(t, uint32(0), decision.PathID, "native zero is an actual identifier")
				require.Equal(t, ValidationValid, decision.ValState)
				got[rpkiSource{owner: decision.PeerOwner, connection: decision.ConnectionID}] = wantRoute{
					prefix: decision.Prefix,
					msgID:  decision.MsgID,
				}
			}
			require.Equal(t, map[rpkiSource]wantRoute{
				fresh:   {prefix: "10.0.1.0/24", msgID: 74},
				sibling: {prefix: "10.0.0.0/24", msgID: 73},
			}, got)
		})
	}
}

// The existing revalidation snapshot carries the retained lifetime across
// validator disable/re-enable and policy rollback; it is never inferred from
// whichever source happens to be UP when the snapshot is read.
func TestRPKIRetainedSnapshotKeepsLifetimeAndNoLLGR(t *testing.T) {
	rp, bridge := rpkiOwnerPlugin(t)
	captured := rpkiOwnerDecisionTransport(t, rp, bridge, "typed", 1)
	body := rpkiOwnerBody()
	attrs := append([]byte(nil), body[4:32]...)
	attrs = append(attrs, 0xc0, 8, 4, 0xff, 0xff, 0, 7)
	row := configRouteSnapshot{Retained: true, PeerOwner: 1701, ConnectionID: 41, Peer: rpkiOwnerAddress,
		PeerName: "old", PeerGroup: "ix", PeerAS: 100, LocalAS: 65000, Family: "ipv4/unicast", Prefix: "10.0.0.0/24", Attributes: hex.EncodeToString(attrs), MsgID: 71}
	snapshot, err := json.Marshal(struct {
		Routes []configRouteSnapshot `json:"routes"`
	}{Routes: []configRouteSnapshot{row}})
	require.NoError(t, err)
	require.NoError(t, rp.revalidateConfigSnapshot(context.Background(), snapshot))
	decisions := <-captured
	rpkiOwnerDecisionSources(t, decisions, map[rpkiSource]uint64{{owner: 1701, connection: 41}: 71})
	rpkiOwnerDown(t, rp, "typed", rpkiSource{owner: 1701, connection: 41}, rpc.ReasonTCPFailure)
	require.Equal(t, 1, rp.originTracker.count())
	require.Equal(t, 1, rp.aspaTracker.count())
	cleanup, err := retention.EncodeCleanup(1701, 41, rpkiOwnerAddress, []family.Family{family.IPv4Unicast}, true)
	require.NoError(t, err)
	retainedRPKIEvent(t, rp, retention.CleanupEventType, cleanup)
	require.Zero(t, rp.originTracker.count())
	require.Zero(t, rp.aspaTracker.count())
}
