// Design: docs/architecture/plugin/rib-storage-design.md -- joined selected replay.
package rib

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestSelectedRefreshJoinsOnlyCurrentConfiguredOutput(t *testing.T) {
	family.RegisterTestFamilies()
	for _, enhanced := range []bool{false, true} {
		t.Run(fmt.Sprintf("enhanced=%t", enhanced), func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			r := newTestRIBManager(t)
			t.Cleanup(r.shutdownReceivedSources)
			peer := netip.MustParseAddr("192.0.2.251")
			const session uint64 = 2251
			const owner uint64 = 1251
			const receipt uint64 = 3251
			r.peerUp[owner] = true
			r.peerMeta[owner] = &peerMetadata{Address: peer, Session: session,
				refreshOwner: owner, refreshSession: session, refreshOrdinary: true}
			encoding, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			record := func(last byte, peerOwner, sentSession, message uint64, configured, local bool) {
				t.Helper()
				body := []byte{0, 0, 0, 14, 0x40, 1, 1, 0, 0x40, 2, 0,
					0x40, 3, 4, 192, 0, 2, 1, 24, 198, 51, last}
				wire := wireu.NewWireUpdate(body, encoding)
				attrs, attrErr := wire.Attrs()
				require.NoError(t, attrErr)
				var meta map[string]any
				if configured {
					meta = map[string]any{"config-static": true}
				}
				r.handleSentStructured(&rpc.StructuredEvent{PeerAddress: peer.String(), PeerOwner: peerOwner,
					DestinationSession: sentSession, MessageID: message, Meta: meta,
					RawMessage: &bgptypes.RawMessage{WireUpdate: wire, AttrsWire: attrs,
						SourceLocal: local, SourcePolicyEpoch: 99}})
			}
			record(100, owner, session, receipt, true, true)
			record(101, owner, session-1, receipt+1, true, true)
			record(102, owner+1, session, receipt+2, true, true)
			record(103, owner, session, receipt+3, false, true)
			// A concurrent learned write can inherit the configured marker.
			record(104, owner, session, receipt+4, true, false)

			d, err := newSelectedDispatch(ctx, func(_ context.Context, in rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
				return selectedDispatchAnswer(in, rpc.SelectedWritten), nil
			}, nil, nil)
			require.NoError(t, err)
			r.selectedDispatch = d
			t.Cleanup(func() { require.NoError(t, d.stop(ctx)) })
			markers := make(chan rpc.SelectRefreshInput, 8)
			writes := make(chan rpc.UpdateRouteInput, 8)
			failedAttempt, retry := make(chan struct{}), make(chan struct{})
			var releaseFailure, releaseRetry sync.Once
			t.Cleanup(func() {
				releaseFailure.Do(func() { close(failedAttempt) })
				releaseRetry.Do(func() { close(retry) })
			})
			attempt := 0
			send := func(work context.Context, in rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
				attempt++
				writes <- in
				switch attempt {
				case 1:
					select {
					case <-work.Done():
						return nil, work.Err()
					case <-failedAttempt:
						return nil, errors.New("configured replay final writer failed")
					}
				case 2:
					select {
					case <-work.Done():
						return nil, work.Err()
					case <-retry:
					}
				}
				return &rpc.UpdateRouteOutput{Announced: 1}, nil
			}
			require.NoError(t, d.setRefreshWriters(func(_ context.Context, in rpc.SelectRefreshInput) (*rpc.SelectRefreshOutput, error) {
				markers <- in
				out := selectedRefreshReceipt(in)
				if !enhanced {
					out.Outcome = rpc.SelectedRefreshNotNegotiated
				}
				return out, nil
			}, func(work context.Context, in rpc.SelectRefreshInput, state *selectedConfiguredRefresh) error {
				return r.replayConfiguredSent(work, in, state, send)
			}))
			require.NoError(t, d.destinationOwnerUp(peer, owner, session))
			require.NoError(t, d.barrier(ctx, peer))

			checkMarker := func(action rpc.SelectedRefreshAction, fam family.Family) {
				t.Helper()
				in := selectedDispatchAwait(t, ctx, markers)
				require.Equal(t, action, in.Action)
				require.Equal(t, fam.String(), in.Family)
				require.Equal(t, peer.String(), in.Destination)
				require.Equal(t, owner, in.PeerOwner)
				require.Equal(t, session, in.DestinationSession)
			}
			checkWrite := func() {
				t.Helper()
				in := selectedDispatchAwait(t, ctx, writes)
				require.Equal(t, peer.String(), in.PeerSelector)
				require.Equal(t, owner, in.DestinationOwner)
				require.Equal(t, session, in.DestinationSession)
				require.Zero(t, in.SourcePolicyEpoch, "configured replay grants no learned source policy authority")
				require.Equal(t, "3251", in.Meta[bgptypes.SentOwnerMessageMeta])
				require.Equal(t, true, in.Meta[metaKeyReplay])
				groups := consumeRefreshCommand(t, in.Command)
				require.Equal(t, []refreshRouteIdentity{{family: family.IPv4Unicast,
					prefix: netip.MustParsePrefix("198.51.100.0/24")}}, consumedRefreshRoutes(t, groups))
			}
			v4, v6 := family.IPv4Unicast, family.IPv6Unicast
			require.NoError(t, d.requestRefresh(peer, owner, session, v4))
			checkMarker(rpc.SelectedRefreshBegin, v4)
			checkWrite()
			waitSelectedRefreshBarrier(t, ctx, d, peer)
			releaseFailure.Do(func() { close(failedAttempt) })
			checkWrite()
			waitSelectedRefreshBarrier(t, ctx, d, peer)
			require.Empty(t, markers, "End must wait through failure and the held retry")
			releaseRetry.Do(func() { close(retry) })
			require.NoError(t, d.barrier(ctx, peer))
			checkMarker(rpc.SelectedRefreshEnd, v4)

			require.NoError(t, d.requestRefresh(peer, owner, session, v6))
			require.NoError(t, d.barrier(ctx, peer))
			checkMarker(rpc.SelectedRefreshBegin, v6)
			checkMarker(rpc.SelectedRefreshEnd, v6)
			require.Empty(t, writes, "IPv6 refresh must never borrow IPv4 SENT output")

			require.NoError(t, d.requestRefresh(peer, owner, session, v4))
			require.NoError(t, d.barrier(ctx, peer))
			checkMarker(rpc.SelectedRefreshBegin, v4)
			checkWrite()
			checkMarker(rpc.SelectedRefreshEnd, v4)
			require.Empty(t, writes, "only the current configured route can be replayed")
			request, err := parseOwnerExcludedResend([]string{"owner", "901"})
			require.NoError(t, err)
			out, err := request.resend(ctx, r)
			require.NoError(t, err)
			require.Equal(t, map[string]any{"refreshed-destinations": 1, "refused-destinations": 0}, out)
			checkMarker(rpc.SelectedRefreshBegin, v4)
			checkWrite()
			checkMarker(rpc.SelectedRefreshEnd, v4)
			require.Empty(t, markers)

			// An unconfirmed writer cannot be retried or forgotten when the
			// same refresh phase is revisited. Its transport may have sent it.
			input := rpc.SelectRefreshInput{Destination: peer.String(), PeerOwner: owner,
				DestinationSession: session, Family: v4.String()}
			var unconfirmed selectedConfiguredRefresh
			unsettled := &rpc.CancellationUnsettledError{}
			calls := 0
			uncertainSend := func(context.Context, rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
				calls++
				return nil, unsettled
			}
			require.ErrorIs(t, r.replayConfiguredSent(ctx, input, &unconfirmed, uncertainSend), unsettled)
			require.ErrorIs(t, r.replayConfiguredSent(ctx, input, &unconfirmed, uncertainSend), unsettled)
			require.Equal(t, 1, calls)
		})
	}
}
