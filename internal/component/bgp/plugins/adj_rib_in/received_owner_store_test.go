// Design: docs/architecture/plugin/rib-storage-design.md -- auxiliary original-source isolation.
package adj_rib_in

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"strconv"
	"testing"
	"time"
)

// withdraw runs the actual wire UPDATE producer on the same original source
// facts as receive, including real format.AppendMessage for the JSON rail.
func (p *adjOwnerProducer) withdraw(owner, connection, msgID uint64, octet byte) {
	p.t.Helper()
	body := []byte{0, 4, 24, 10, 77, octet, 0, 0}
	update := wireu.NewWireUpdate(body, p.context)
	attrs, err := update.Attrs()
	require.NoError(p.t, err)
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: msgID, WireUpdate: update, AttrsWire: attrs, RawBytes: body}
	if p.json {
		info := pluginapi.PeerInfo{Address: p.address, AddressStr: p.address.String(), PeerOwner: owner, ConnectionID: connection, PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000}
		encoded := format.AppendMessage(nil, &info, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
		event, err := bgp.ParseEvent(encoded)
		require.NoError(p.t, err)
		p.r.handleReceived(event)
		return
	}
	p.r.handleReceivedStructured(&rpc.StructuredEvent{EventType: rpc.EventKindUpdate, PeerAddress: p.address.String(),
		PeerOwner: owner, ConnectionID: connection, RawMessage: &msg})
}

// Original same-address owners keep the SAME prefix simultaneously; a later
// physical connection replaces only its owner's value. Old UPDATE/withdrawal
// traffic cannot erase the useful successor or independent sibling. Replay
// preserves both useful original message generations through its real walk.
func TestAdjStoreSamePrefixOwnersAndReconnectBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(1701, 41, true)
			p.state(1702, 42, true)
			old := p.receive(1701, 41, 71, 0)
			sibling := p.receive(1702, 42, 72, 0)
			require.True(t, p.r.routeEligible(old, 71))
			require.True(t, p.r.routeEligible(sibling, 72))
			p.state(1701, 43, true)
			current := p.receive(1701, 43, 73, 0)
			p.receive(1701, 41, 74, 0)
			p.withdraw(1701, 41, 75, 0)
			require.False(t, p.r.routePresent(old))
			require.True(t, p.r.routeEligible(current, 73))
			require.True(t, p.r.routeEligible(sibling, 72))
			routes, _ := p.r.buildReplayRoutes(netip.MustParseAddr("192.0.2.189"), 0, unboundedReplay())
			require.Len(t, routes, 2)
			generations := map[uint64]string{}
			for _, route := range routes {
				generations[route.MsgID] = route.NLRIHex
			}
			require.Equal(t, map[uint64]string{72: "180a4d00", 73: "180a4d00"}, generations)
			p.withdraw(1701, 43, 76, 0)
			require.False(t, p.r.routePresent(current))
			require.True(t, p.r.routeEligible(sibling, 72), "matching withdrawal preserves useful sibling")
		})
	}
}

// A decision arriving ahead of its UPDATE belongs to its original owner and
// physical connection. Untagged and retired callbacks cannot promote an owned
// pending route; source DOWN releases just its own early/pending population.
func TestAdjPendingAndEarlyKeepOriginalSourceBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.r.validationEnabled = true
			p.state(1701, 41, true)
			p.state(1702, 42, true)
			decision := rpc.ValidationDecision{PeerAddr: p.address.String(), Family: "ipv4/unicast", Prefix: "10.77.0.0/24", Accept: true, ValState: ValidationValid,
				PeerOwner: 1702, ConnectionID: 42, MsgID: 72}
			result, err := p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.Equal(t, 1, result.Early)
			pending := p.receive(1701, 41, 71, 0)
			sibling := p.receive(1702, 42, 72, 0)
			require.False(t, p.r.routeEligible(pending, 71))
			require.True(t, p.r.routePresent(pending))
			require.True(t, p.r.routeEligible(sibling, 72))
			decision.PeerOwner = 0
			decision.ConnectionID = 0
			decision.MsgID = 0
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.False(t, p.r.routeEligible(pending, 71), "observation decision is not wildcard authority")
			p.state(1701, 43, true)
			current := p.receive(1701, 43, 73, 0)
			decision.PeerOwner = 1701
			decision.ConnectionID = 41
			decision.MsgID = 99
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.False(t, p.r.routeEligible(current, 73), "old connection cannot promote successor even with newer callback ID")
			for key := range p.r.earlyDecisions {
				require.NotEqual(t, uint64(1701), key.Peer.owner, "discard old physical callbacks without new debt")
			}
			decision.ConnectionID = 43
			decision.MsgID = 73
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.True(t, p.r.routeEligible(current, 73))
			p.state(1701, 43, false)
			require.False(t, p.r.routePresent(current))
			require.True(t, p.r.routeEligible(sibling, 72))
			for key := range p.r.earlyDecisions {
				require.NotEqual(t, uint64(1701), key.Peer.owner)
			}
		})
	}
}

// Exact validation replay cannot borrow the same prefix from a sibling or a
// successor physical connection. The useful sibling remains queryable on its
// own captured source, and its actual stored bytes/message remain unchanged.
func TestAdjReplayPathKeepsOriginalSourceBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(1701, 41, true)
			p.state(1702, 42, true)
			gone := p.receive(1701, 41, 71, 0)
			sibling := p.receive(1702, 42, 72, 0)
			p.withdraw(1701, 41, 73, 0)
			require.False(t, p.r.routePresent(gone))
			require.True(t, p.r.routeEligible(sibling, 72))
			args := []string{"192.0.2.189", p.address.String(), "ipv4/unicast", "10.77.0.0/24", "0", "1701", "41"}
			status, data, err := p.r.replayPathCommand(args)
			require.NoError(t, err)
			require.Equal(t, statusDone, status)
			page, ok := data.(ribevents.ReplayPage)
			require.True(t, ok)
			require.Len(t, page.Routes, 1)
			require.True(t, page.Routes[0].Withdraw)
			args[5] = "1702"
			args[6] = "42"
			_, data, err = p.r.replayPathCommand(args)
			require.NoError(t, err)
			page = data.(ribevents.ReplayPage)
			require.Len(t, page.Routes, 1)
			require.False(t, page.Routes[0].Withdraw)
			require.Equal(t, uint64(72), page.Routes[0].MsgID)
			require.Equal(t, "180a4d00", page.Routes[0].NLRIHex)
			args[6] = "43"
			_, data, err = p.r.replayPathCommand(args)
			require.NoError(t, err)
			require.True(t, data.(ribevents.ReplayPage).Routes[0].Withdraw)
			args[5] = "0"
			_, _, err = p.r.replayPathCommand(args)
			require.Error(t, err)
		})
	}
}

// Original uint64 source identities survive the actual shared JSON DTO codec
// losslessly, above JavaScript's integer width; failed decode leaves no accepted
// decision and the useful exact owned route remains gated by its source.
func TestValidationDecisionCarriesOriginalSourceWidth(t *testing.T) {
	original := rpc.ValidationDecision{PeerAddr: "192.0.2.188", Family: "ipv4/unicast", Prefix: "10.77.0.0/24",
		PeerOwner: ^uint64(0), ConnectionID: ^uint64(0) - 1, MsgID: 71, Accept: true, ValState: ValidationValid}
	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"peer-owner":"18446744073709551615"`)
	require.Contains(t, string(encoded), `"connection-id":"18446744073709551614"`)
	var decoded rpc.ValidationDecision
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, original, decoded)
	require.Error(t, json.Unmarshal([]byte(`{"peer-owner":"-1","connection-id":"42"}`), &decoded))
}

// deliverBody crosses each actual received-event producer with exact original
// owner/connection. It deliberately reuses the established per-family writer.
func (p *adjOwnerProducer) deliverBody(owner, connection, msgID uint64, body []byte) {
	p.t.Helper()
	update := wireu.NewWireUpdate(body, p.context)
	attrs, err := update.Attrs()
	require.NoError(p.t, err)
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: msgID, RawBytes: body, WireUpdate: update, AttrsWire: attrs, SourcePolicyEpoch: 5, SourceOwner: owner, SourceReceipt: p.receipt}
	if p.json {
		info := pluginapi.PeerInfo{Address: p.address, AddressStr: p.address.String(), PeerOwner: owner, ConnectionID: connection, PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000}
		line := format.AppendMessage(nil, &info, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
		event, err := bgp.ParseEvent(line)
		require.NoError(p.t, err)
		p.r.handleReceived(event)
		return
	}
	p.r.handleReceivedStructured(&rpc.StructuredEvent{EventType: rpc.EventKindUpdate, PeerAddress: p.address.String(), PeerOwner: owner, ConnectionID: connection,
		PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000, RawMessage: &msg})
}

// The SAME native VPN/ADD-PATH-zero path belongs independently to two actual
// source owners. The family's real JSON and structured splitters preserve RD,
// framing, zero identifier and useful sibling bytes after exact withdrawal.
func TestAdjNativeOwnerIsolationKeepsPathIDZeroBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			contextID, err := bgpctx.Registry.Register(bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{nlriKeyVPN: true}))
			require.NoError(t, err)
			p.context = contextID
			p.state(1701, 41, true)
			p.state(1702, 42, true)
			nextHop := []byte{0, 0, 0, 0, 0, 0, 0, 0, 192, 0, 2, 1}
			announce := mpAnnounce(nlriKeyVPN, nextHop, pathID(0, nlriKeyVPNWire))
			p.deliverBody(1701, 41, 71, announce)
			p.deliverBody(1702, 42, 72, announce)
			first := ribevents.ValidationRoute{Peer: p.address, PeerOwner: 1701, ConnectionID: 41, Family: nlriKeyVPN, PathID: 0, NLRI: string(nlriKeyVPNWire)}
			sibling := first
			sibling.PeerOwner = 1702
			sibling.ConnectionID = 42
			require.True(t, p.r.routeEligible(first, 71))
			require.True(t, p.r.routeEligible(sibling, 72))
			p.deliverBody(1701, 41, 73, mpWithdraw(nlriKeyVPN, pathID(0, nlriKeyVPNWithdrawal)))
			require.False(t, p.r.routePresent(first))
			require.True(t, p.r.routeEligible(sibling, 72))
			routes, _ := p.r.buildReplayRoutes(netip.MustParseAddr("192.0.2.189"), 0, unboundedReplay())
			require.Len(t, routes, 1)
			require.Equal(t, uint32(0), routes[0].PathID)
			require.Equal(t, rpc.NLRIFramingPrefixOnly, routes[0].NLRIFraming)
			require.Equal(t, uint64(72), routes[0].MsgID)
			require.Equal(t, "700000110000fde8000000010a0100", routes[0].NLRIHex)
		})
	}
}

// Actual SDK execute-command carries original source tokens through both bridge
// and framed RPC transports into the nine-field batch parser and live store.
// Its useful same-prefix sibling remains pending until its OWN decision arrives.
func TestAdjOwnerBatchCommandCrossesSDKBothTransports(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		t.Run(transport.String(), func(t *testing.T) {
			previous := rpc.GetBatchValidator()
			t.Cleanup(func() { rpc.RegisterBatchValidator(previous); ribevents.RegisterValidationLookup(nil, nil) })
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			engine := pluginengine.Start(t, transport, runAdjRIBInPlugin)
			p := newAdjOwnerProducer(t, true)
			state := func(owner, connection uint64) string {
				encoded, err := json.Marshal(&bgp.Event{Type: "state", State: "up", Reason: rpc.ReasonAttached, PeerOwner: owner, ConnectionID: connection,
					Peer: mustMarshal(t, bgp.PeerInfoJSON{Remote: bgp.PeerRemoteInfo{Address: p.address.String(), AS: 65000}})})
				require.NoError(t, err)
				return string(encoded)
			}
			require.NoError(t, engine.Deliver(ctx, state(1701, 41)))
			require.NoError(t, engine.Deliver(ctx, state(1702, 42)))
			_, err := engine.Execute(ctx, "request bgp adj-rib-in enable-validation", nil)
			require.NoError(t, err)
			body := rfc4271Announce(1, 24, 10, 77, 0)
			for _, source := range []struct{ owner, connection, message uint64 }{{1701, 41, 71}, {1702, 42, 72}} {
				update := wireu.NewWireUpdate(body, p.context)
				attrs, err := update.Attrs()
				require.NoError(t, err)
				info := pluginapi.PeerInfo{Address: p.address, AddressStr: p.address.String(), PeerOwner: source.owner, ConnectionID: source.connection, PeerAS: rpc.RemoteASOf(65000), LocalAS: 65000}
				msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: source.message, RawBytes: body, WireUpdate: update, AttrsWire: attrs}
				line := format.AppendMessage(nil, &info, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull})
				require.NoError(t, engine.Deliver(ctx, string(line)))
			}
			decide := func(owner, connection, message uint64) {
				args := []string{"a", p.address.String(), "ipv4/unicast", "10.77.0.0/24", "0", "1", strconv.FormatUint(message, 10), strconv.FormatUint(owner, 10), strconv.FormatUint(connection, 10)}
				_, err := engine.Execute(ctx, "request bgp adj-rib-in batch-validate", args)
				require.NoError(t, err)
			}
			eligible := func(owner, connection, message uint64) bool {
				query := ribevents.EligibilityRequest{Queries: []ribevents.EligibilityQuery{{Route: ribevents.ValidationRoute{Peer: p.address, PeerOwner: owner, ConnectionID: connection,
					Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.77.0.0/24")}, MsgID: message}}}
				encoded, err := json.Marshal(query)
				require.NoError(t, err)
				result, err := engine.Execute(ctx, eligibilityCommand, []string{string(encoded)})
				require.NoError(t, err)
				var answer ribevents.EligibilityAnswer
				require.NoError(t, json.Unmarshal(result, &answer))
				require.Len(t, answer.Answers, 1)
				return answer.Answers[0]
			}
			decide(1701, 41, 71)
			require.True(t, eligible(1701, 41, 71))
			require.False(t, eligible(1702, 42, 72))
			decide(1702, 42, 72)
			require.True(t, eligible(1701, 41, 71))
			require.True(t, eligible(1702, 42, 72))
		})
	}
}

// Address-only state remains useful observation while conveying no established
// source-policy authority. Its DOWN cannot lower an actual same-address source.
func TestAdjObservedStateKeepsBookkeepingWithoutAuthorityBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(0, 0, true)
			require.True(t, p.r.peerUp[p.address])
			require.Empty(t, p.r.sourceLive)
			require.Error(t, p.policy(1701, rpc.SourcePolicyPrepare, "unowned", 0))
			p.receive(0, 0, 70, 0)
			p.state(1701, 41, true)
			useful := p.receive(1701, 41, 71, 0)
			require.True(t, p.r.routeEligible(useful, 71))
			p.state(0, 0, false)
			require.True(t, p.r.peerUp[p.address], "unowned DOWN cannot lower actual sibling state")
			require.True(t, p.r.routeEligible(useful, 71))
			require.NotContains(t, p.r.ribIn, receivedPeer{address: p.address})
			p.state(1701, 41, false)
			require.False(t, p.r.peerUp[p.address])
			require.False(t, p.r.routePresent(useful))
		})
	}
}

// The stored replay codec preserves full-width original source facts. It proves
// data carriage only: scalar values here are not issued capabilities or leases.
func TestStoredRouteOriginalFieldsUseLosslessJSON(t *testing.T) {
	original := rpc.StoredRoute{SourcePeer: "192.0.2.188", SourceOwner: ^uint64(0), SourceReceipt: ^uint64(0) - 1, SourceConnectionID: ^uint64(0) - 2,
		Family: "ipv4/unicast", NLRIHex: "180a4d00", NLRIFraming: rpc.NLRIFramingPrefixOnly}
	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"source-owner":"18446744073709551615"`)
	require.Contains(t, string(encoded), `"source-receipt":"18446744073709551614"`)
	require.Contains(t, string(encoded), `"source-connection-id":"18446744073709551613"`)
	var decoded rpc.StoredRoute
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, original, decoded)
}

// Real typed/full-JSON receive retains a passive receipt scalar unchanged while
// pending, then installed, then projected into snapshot and replay. This is a
// transport/storage question, not a claim of issuing or acquiring a source lease.
func TestAdjCapturedReceiptSurvivesPendingAndReplayBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.receipt = ^uint64(0) - 1
			p.state(1701, 41, true)
			p.r.validationEnabled = true
			query := p.receive(1701, 41, 71, 0)
			key := pendingKey(receivedPeer{address: p.address, owner: 1701}, routeKeyFromStrings(family.IPv4Unicast, "10.77.0.0/24", 0))
			require.Equal(t, p.receipt, p.r.pending[key].route.SourceReceipt)
			_, err := p.r.handleBatchValidateTyped([]rpc.ValidationDecision{{PeerAddr: p.address.String(), PeerOwner: 1701, ConnectionID: 41,
				Family: "ipv4/unicast", Prefix: "10.77.0.0/24", MsgID: 71, Accept: true, ValState: ValidationValid}})
			require.NoError(t, err)
			require.True(t, p.r.routeEligible(query, 71))
			routes, _ := p.r.buildReplayRoutes(netip.MustParseAddr("192.0.2.189"), 0, unboundedReplay())
			require.Len(t, routes, 1)
			require.Equal(t, p.receipt, routes[0].SourceReceipt)
			require.Equal(t, uint64(1701), routes[0].SourceOwner)
			require.Equal(t, uint64(41), routes[0].SourceConnectionID)
			p.r.mu.Lock()
			snapshot := p.r.suspendForRevalidation()
			p.r.unlockValidation()
			require.Len(t, snapshot, 1)
			require.Equal(t, strconv.FormatUint(p.receipt, 10), snapshot[0]["source-receipt"])
			require.Equal(t, "1701", snapshot[0]["peer-owner"])
			require.Equal(t, "41", snapshot[0]["connection-id"])
			p.state(1701, 43, true)
			p.receipt = 0
			p.receive(1701, 43, 73, 0)
			require.Zero(t, p.r.pending[key].route.SourceReceipt, "unrecorded replacement cannot inherit old receipt")
		})
	}
}
