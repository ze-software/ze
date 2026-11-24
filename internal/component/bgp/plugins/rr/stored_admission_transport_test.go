// Design: docs/architecture/plugin/rib-storage-design.md -- cold replay admission.
// Related: stored_admission.go -- original progress and final local recheck.
package rr

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/test/pluginengine"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rrAdmissionInput is an input route the real registration accepts. Typed is
// valid only on DirectBridge; text and its complete JSON envelope use the real
// formatter and the SDK callback loop on either transport.
type rrAdmissionInput uint8

const (
	rrAdmissionUnspecified rrAdmissionInput = iota
	rrAdmissionTyped
	rrAdmissionText
	rrAdmissionEnvelope
)

func (input rrAdmissionInput) deliverState(t *testing.T, ctx context.Context, engine *pluginengine.Engine, peer plugin.PeerInfo, reason string) {
	t.Helper()
	if input == rrAdmissionTyped {
		require.NoError(t, engine.DeliverStructured([]any{&rpc.StructuredEvent{EventType: rpc.EventKindState,
			PeerAddress: peer.Address.String(), PeerOwner: peer.PeerOwner, ConnectionID: peer.ConnectionID,
			SessionGeneration: peer.SessionGeneration, DestinationSession: peer.DestinationSession, InitialReplay: peer.InitialReplay,
			IsIBGP: peer.IsIBGP, RSClient: peer.RSClient, RouteReflectorClient: peer.RouteReflectorClient,
			State: rpc.SessionStateUp, Reason: reason}}))
		return
	}
	require.NoError(t, engine.Deliver(ctx, string(format.AppendStateChange(nil, &peer, rpc.SessionStateUp, reason, nil, plugin.EncodingText))))
}

func (input rrAdmissionInput) deliverUpdate(t *testing.T, ctx context.Context, engine *pluginengine.Engine, peer plugin.PeerInfo, receipt, id uint64, withdraw bool, last byte) {
	t.Helper()
	raw := []byte{24, 10, 0, last}
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1}
	var body []byte
	if withdraw {
		body = append([]byte{0, byte(len(raw))}, raw...)
		body = append(body, 0, 0)
	} else {
		body = append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)
		body = append(body, raw...)
	}
	encoding, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	update := wireu.NewWireUpdate(body, encoding)
	wireAttrs, err := update.Attrs()
	require.NoError(t, err)
	message := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionReceived, RawBytes: body,
		WireUpdate: update, AttrsWire: wireAttrs, MessageID: id, SourceOwner: peer.PeerOwner, SourceReceipt: receipt, SourcePolicyEpoch: 5}
	if input == rrAdmissionTyped {
		require.NoError(t, engine.DeliverStructured([]any{&rpc.StructuredEvent{EventType: rpc.EventKindUpdate,
			PeerAddress: peer.Address.String(), PeerOwner: peer.PeerOwner, ConnectionID: peer.ConnectionID,
			SourceReceipt: receipt, RawMessage: &message}}))
		return
	}
	text := string(format.AppendMessage(nil, &peer, message, bgptypes.ContentConfig{Encoding: plugin.EncodingText, Format: plugin.FormatParsed}))
	if input == rrAdmissionEnvelope {
		payload, err := json.Marshal(text)
		require.NoError(t, err)
		metadata, err := json.Marshal(bgptypes.UpdateReceipt{Peer: peer.Address, PeerOwner: peer.PeerOwner, MessageID: id,
			SourcePolicyEpoch: 5, SourceReceipt: receipt, ReactorForwarded: false})
		require.NoError(t, err)
		envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: "bgp", Event: "update", Payload: payload, Metadata: metadata})
		require.NoError(t, err)
		text = string(envelope)
	}
	require.NoError(t, engine.Deliver(ctx, text))
}

// TestRRColdReplayRechecksWithdrawnRowAcrossRegisteredTransports starts the
// real five-stage RR registration. Its old page contains A, unseen B and C.
// The current-store query for B is held while the real original UPDATE path
// withdraws B. After the stale true answer escapes, the query must be repeated
// above that original source's applied MsgID90; pending is distinct from false.
// Useful A and unseen C reach the owner-bound relay once; B never does. This
// proves acknowledged role/SDK transports, not the reactor's final socket.
func TestRRColdReplayRechecksWithdrawnRowAcrossRegisteredTransports(t *testing.T) {
	for _, transport := range pluginengine.Transports() {
		inputs := []rrAdmissionInput{rrAdmissionText, rrAdmissionEnvelope}
		if transport == pluginengine.TransportBridge {
			inputs = append(inputs, rrAdmissionTyped)
		}
		for _, input := range inputs {
			name := map[rrAdmissionInput]string{rrAdmissionTyped: "typed", rrAdmissionText: "formatter-text", rrAdmissionEnvelope: "full-envelope"}[input]
			t.Run(transport.String()+"/"+name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				engine := pluginengine.Start(t, transport, runRouteReflectorCallbacks)
				source := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: rpc.RemoteASOf(65001), PeerOwner: 701,
					ConnectionID: 41, SessionGeneration: 1041, DestinationSession: 141, IsIBGP: true, RouteReflectorClient: true}
				target := plugin.PeerInfo{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: rpc.RemoteASOf(65002), PeerOwner: 702,
					ConnectionID: 42, SessionGeneration: 1042, DestinationSession: 142, InitialReplay: 142, IsIBGP: true, RouteReflectorClient: true}
				receipt := engine.IssueSource(source.Address.String(), source.PeerOwner)
				route := func(id uint64, hex string) rpc.StoredRoute {
					return rpc.StoredRoute{SourcePeer: source.Address.String(), SourceOwner: source.PeerOwner,
						SourceConnectionID: source.ConnectionID, SourceReceipt: receipt, MsgID: id, SourcePolicyEpoch: 5, InitialUpdate: true, AttrHex: "40010100400200400304c0000201", NextHopHex: "c0000201", Family: "ipv4/unicast", NLRIHex: hex, NLRIFraming: rpc.NLRIFramingPrefixOnly}
				}
				rows := []rpc.StoredRoute{route(80, "180a0000"), route(81, "180a0001"), route(82, "180a0002")}
				entered := make(chan struct{})
				release := make(chan struct{})
				unblock := sync.OnceFunc(func() { close(release) })
				t.Cleanup(unblock)
				ready := make(chan struct{}, 1)
				var mu sync.Mutex
				var relays []rpc.RelayStoredRouteInput
				var queries []ribevents.EligibilityQuery
				progress := uint64(79)
				held := false
				pending := 0
				engine.SetRPCHandler(func(call context.Context, method string, params json.RawMessage) (any, error) {
					switch method {
					case rpc.MethodDispatchCommandArgs:
						var command rpc.DispatchCommandArgsInput
						if err := json.Unmarshal(params, &command); err != nil {
							return nil, err
						}
						if command.Command == ribevents.ReplayRoutesCommand {
							page, err := json.Marshal(ribevents.ReplayPage{Routes: rows})
							if err != nil {
								return nil, err
							}
							return &rpc.DispatchCommandOutput{Status: rpc.StatusDone, Data: page}, nil
						}
						if command.Command != "request bgp adj-rib-in eligibility" {
							return nil, errors.New("unexpected store command " + command.Command)
						}
						if len(command.Args) != 1 {
							return nil, errors.New("eligibility request did not carry one JSON argument")
						}
						var request ribevents.EligibilityRequest
						if err := json.Unmarshal([]byte(command.Args[0]), &request); err != nil {
							return nil, err
						}
						if len(request.Queries) != 1 {
							return nil, errors.New("eligibility request did not name one current route")
						}
						query := request.Queries[0]
						mu.Lock()
						queries = append(queries, query)
						first := false
						if query.MsgID == 81 {
							first = !held
						}
						if first {
							held = true
						}
						floor := progress
						if query.AfterMsgID > floor {
							pending++
							progress = query.AfterMsgID
						}
						mu.Unlock()
						if first {
							close(entered)
							select {
							case <-release:
							case <-call.Done():
								return nil, call.Err()
							}
						}
						answer := ribevents.EligibilityAnswer{Answers: []bool{query.MsgID != 81 || floor < 90}}
						if query.AfterMsgID > floor {
							answer = ribevents.EligibilityAnswer{Pending: true}
						}
						body, err := json.Marshal(answer)
						if err != nil {
							return nil, err
						}
						return &rpc.DispatchCommandOutput{Status: rpc.StatusDone, Data: body}, nil
					case rpc.MethodForwardCached:
						var batch rpc.ForwardCachedInput
						if err := json.Unmarshal(params, &batch); err != nil {
							return nil, err
						}
						// Arrival proves the real RR receive path committed local inventory
						// before it submitted this ordinary withdrawal's cached output.
						if slices.Contains(batch.IDs, uint64(90)) {
							mu.Lock()
							progress = 89
							mu.Unlock()
						}
						return rpc.CompletedCachedWrite(batch.DeadlineMS, nil), nil
					case rpc.MethodRelayStoredRoute:
						var relay rpc.RelayStoredRouteInput
						if err := json.Unmarshal(params, &relay); err != nil {
							return nil, err
						}
						mu.Lock()
						relays = append(relays, relay)
						mu.Unlock()
						return rpc.CompletedCachedWrite(relay.DeadlineMS, nil), nil
					case rpc.MethodDispatchCommand:
						var input rpc.DispatchCommandInput
						if err := json.Unmarshal(params, &input); err != nil {
							return nil, err
						}
						ready <- struct{}{}
						return &rpc.DispatchCommandOutput{Status: rpc.StatusDone, DestinationOwner: input.DestinationOwner, DestinationSession: input.DestinationSession, DeadlineMS: input.DeadlineMS}, nil
					case rpc.MethodReleaseCached:
						return struct{}{}, nil
					default:
						return nil, pluginengine.ErrUnhandledRPC
					}
				})
				gate, err := json.Marshal(ribevents.ValidationGateState{Enabled: false, Command: "request bgp adj-rib-in eligibility"})
				require.NoError(t, err)
				announcement, err := json.Marshal(rpc.EventEnvelope{Namespace: ribevents.Namespace, Event: ribevents.EventValidationGate, Payload: gate})
				require.NoError(t, err)
				require.NoError(t, engine.Deliver(ctx, string(announcement)))
				input.deliverState(t, ctx, engine, source, rpc.ReasonAttached)
				input.deliverUpdate(t, ctx, engine, source, receipt, 80, false, 0)
				input.deliverState(t, ctx, engine, target, "")
				select {
				case <-entered:
				case <-ctx.Done():
					t.Fatal("actual RR replay never entered the held original B query")
				}
				input.deliverUpdate(t, ctx, engine, source, receipt, 90, true, 1)
				unblock()
				select {
				case <-ready:
				case <-ctx.Done():
					t.Fatal("useful current A/C replay did not complete")
				}
				require.NoError(t, engine.SettleEvents(ctx))
				mu.Lock()
				defer mu.Unlock()
				require.Len(t, relays, 1, "one replay page owns a complete admitted target request")
				require.Equal(t, target.Address.String(), relays[0].Destination)
				require.Equal(t, target.PeerOwner, relays[0].DestinationOwner)
				require.Equal(t, target.DestinationSession, relays[0].DestinationSession)
				require.Equal(t, []rpc.StoredRoute{rows[0], rows[2]}, relays[0].Routes, "withdrawn unseen B cannot borrow retained A's Session receipt; unseen current C must survive")
				require.Positive(t, pending, "the exact original source's delayed projection is retried as pending, not false")
				var bQueries []ribevents.EligibilityQuery
				for _, query := range queries {
					require.Equal(t, source.PeerOwner, query.Route.PeerOwner)
					require.Equal(t, source.ConnectionID, query.Route.ConnectionID)
					if query.MsgID == 81 {
						bQueries = append(bQueries, query)
					}
				}
				require.Len(t, bQueries, 3, "one escaped true answer, one pending catch-up, one current false answer")
				require.Equal(t, uint64(80), bQueries[0].AfterMsgID)
				require.Equal(t, uint64(90), bQueries[1].AfterMsgID)
				require.Equal(t, uint64(90), bQueries[2].AfterMsgID)
			})
		}
	}
}
