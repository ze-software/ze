// Design: docs/architecture/plugin/rib-storage-design.md -- captured auxiliary source output debt.
package rib

import (
	"errors"
	"net/netip"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// With no ordinary selected worker, the real registered purge/release and
// safety expiry must join their captured auxiliary SDK writer before releasing
// the original source lease. Same-address and same-owner successor routes stay
// useful. A missing source/output join makes this whole question go red.
// MUTATION: omit holdPhysicalSentOutput in removePhysicalSource; the held or
// unknown auxiliary output then ceases to protect its original source receipt.
func TestGRSourceAuxiliaryOutputOwnsOriginalLease(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, operation := range []string{"purge-stale", "release-routes", "expiry"} {
			for _, mode := range []string{"joined", "not-dispatched", "unknown"} {
				t.Run(strconv.FormatBool(native)+"/"+operation+"/"+mode, func(t *testing.T) {
					ctx := selectedDispatchContext(t)
					entered, unblock := make(chan rpc.UpdateRouteInput, 1), make(chan struct{})
					var once sync.Once
					var mu sync.Mutex
					attempts := 0
					writer := func(input rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
						mu.Lock()
						attempts++
						mu.Unlock()
						entered <- input
						<-unblock
						if mode == "unknown" {
							return nil, errors.New("original auxiliary writer completion is unknown")
						}
						return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
					}
					r, calls := sourceLeaseTestRIBWithOutput(t, native, true, writer)
					if mode == "not-dispatched" {
						var refusal sync.Once
						r.updateRefuseHook = func(rpc.UpdateRouteInput) error {
							var refused error
							refusal.Do(func() {
								refused = rpc.CachedWriteNotDispatched(errors.New("local auxiliary request was not dispatched"))
							})
							return refused
						}
					}
					source := netip.MustParseAddr("192.0.2.251")
					grSourceCommandRoute(t, r, source, 1270, 41, 8270, 70, nil)
					grSourceCommandRoute(t, r, source, 1271, 51, 8271, 71, nil)
					grSourceCommandRun(t, r, "retain-routes", 1270, 41, "on-down", "families", "ipv4/unicast")
					grSourceCommandRun(t, r, "mark-stale", 1270, 41, "restart-time", "0")
					grSourceCommandRoute(t, r, source, 1270, 42, 8272, 72, nil)
					grAuxSent(t, r, !native, 2270, 91, 1270, 41, 9270, 170, 70)
					grAuxSent(t, r, !native, 2271, 92, 1271, 51, 9271, 171, 71)
					grAuxSent(t, r, !native, 2270, 91, 1270, 42, 9272, 172, 72)
					// Current metadata deliberately names another session. The
					// old SENT projection, not this lookup, owns withdrawal.
					r.peerMeta[2270].Session = 93
					state := r.grState[1270]
					done := make(chan error, 1)
					joined := false
					t.Cleanup(func() {
						once.Do(func() { close(unblock) })
						if !joined {
							selectedDispatchAwait(t, ctx, done)
						}
						// Unknown debt is intentionally retained until owner
						// shutdown; fixture teardown MUST NOT manufacture join.
						r.sourceLeases.stop()
						r.sourceLeases = nil
						for _, destinations := range r.ribOut {
							for _, routes := range destinations {
								for _, entry := range routes {
									entry.release()
								}
							}
						}
						r.ribOut = nil
					})
					go func() {
						if operation == "expiry" {
							r.autoExpireStale(1270, state)
							done <- nil
							return
						}
						_, _, err := r.handleCommand("request bgp rib source "+operation, "*", []string{"owner", "1270", "connection", "41"})
						done <- err
					}()
					captured := selectedDispatchAwait(t, ctx, entered)
					require.Equal(t, uint64(2270), captured.DestinationOwner)
					require.Equal(t, uint64(91), captured.DestinationSession)
					require.Positive(t, captured.DeadlineMS)
					require.Equal(t, "170", captured.Meta[bgptypes.SentOwnerMessageMeta])
					require.Contains(t, captured.Command, "70.0.0.0/8")
					require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8270), "held auxiliary final output still owns the source lease")
					once.Do(func() { close(unblock) })
					result := selectedDispatchAwait(t, ctx, done)
					joined = true
					if mode == "unknown" {
						if operation != "expiry" {
							require.Error(t, result)
						}
						require.Error(t, r.sourceLeases.settle(ctx), "a second call cannot convert an unknown original completion into proof")
						require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8270))
						mu.Lock()
						observed := attempts
						mu.Unlock()
						require.Equal(t, 1, observed, "unknown original write must not be blindly retried")
					} else {
						if mode == "not-dispatched" {
							if operation != "expiry" {
								require.Error(t, result, "the first local refusal remains reported")
							}
						} else {
							require.NoError(t, result)
						}
						require.NoError(t, r.sourceLeases.settle(ctx))
						require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 8270))
					}
					sameAddress := grSourceCommandRead(t, r, 1271, 71)
					require.Equal(t, uint64(51), sameAddress.SourceConnectionID())
					successor := grSourceCommandRead(t, r, 1270, 72)
					require.Equal(t, uint64(42), successor.SourceConnectionID())
					require.Contains(t, r.ribOut[2271][family.IPv4Unicast], ribOutKey{Prefix: netip.MustParsePrefix("71.0.0.0/8")})
					require.Contains(t, r.ribOut[2270][family.IPv4Unicast], ribOutKey{Prefix: netip.MustParsePrefix("72.0.0.0/8")})
					require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8271))
					require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8272))
				})
			}
		}
	}
}

// grAuxSent publishes a real SENT event through the native or full-JSON
// constructor. No current destination metadata supplies its captured session.
func grAuxSent(t *testing.T, r *RIBManager, jsonRail bool, destination, session, source, connection, receipt, message uint64, prefix byte) {
	t.Helper()
	address := netip.MustParseAddr("192.0.2.252")
	attrs := unicastAttrs([4]byte{192, 0, 2, 1}, 30, 100)
	body := append([]byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}, attrs...)
	body = append(body, 8, prefix)
	encoding, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	update := wireu.NewWireUpdate(body, encoding)
	aw, err := update.Attrs()
	require.NoError(t, err)
	msg := bgptypes.RawMessage{Type: msgtype.TypeUPDATE, Direction: rpc.DirectionSent, MessageID: message,
		SourceOwner: source, SourceConnectionID: connection, SourceReceipt: receipt - 1000, SourceMessageID: receipt,
		RawBytes: body, WireUpdate: update, AttrsWire: aw}
	r.peerMeta[destination] = &peerMetadata{Address: address, Session: session}
	if jsonRail {
		peer := pluginapi.PeerInfo{PeerOwner: destination, DestinationSession: session, Address: address}
		event, err := parseEvent(format.AppendSentMessage(nil, &peer, msg, bgptypes.ContentConfig{Encoding: pluginapi.EncodingJSON, Format: pluginapi.FormatFull}))
		require.NoError(t, err)
		r.handleSent(event)
		return
	}
	r.handleSentStructured(&rpc.StructuredEvent{PeerOwner: destination, DestinationSession: session, PeerAddress: address.String(), MessageID: message, RawMessage: &msg})
}
