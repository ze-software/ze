// Design: docs/architecture/plugin/rib-storage-design.md -- retained selected attributes.
// Related: rib_structured.go, rib_advertise_capture.go, rib_selected_dispatch.go.
// RFC: rfc/short/rfc4271.md -- Section 5, received optional-transitive Partial preservation.

package rib

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReceivedPartialSurvivesSelectedCapture feeds a valid structured UPDATE
// through received storage, election, owned capture and selected RPC projection.
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS." A complete same-value control must remain complete.
// The positive path acquires its original issued Session through the SDK.
// A separate passive twin keeps the same useful bytes without selected authority.
// The callback observes the selected transport, not a socket or foreign peer.
func TestReceivedPartialSurvivesSelectedCapture(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags byte
	}{
		{"partial", 0xe0},
		{"complete", 0xc0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := netip.MustParseAddr("192.0.2.71")
			ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			prefix := []byte{24, 198, 18, 93}
			nextHop := source.As4()
			attrs := []byte{
				0x40, 1, 1, 0, // ORIGIN = IGP
				0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, // canonical AS_SEQUENCE [65001]
				0x40, 3, 4, nextHop[0], nextHop[1], nextHop[2], nextHop[3],
				tc.flags, 8, 4, 0, 1, 0, 1, // ordinary community 1:1, not NO_EXPORT
			}
			t.Run("passive", func(t *testing.T) {
				partialSelectedPassive(t, source, ctxID, attrs, prefix, tc.flags)
			})
			r, observed := advertiseCaptureRIB(t)
			require.Equal(t, source, observed, "the issued original source keeps its observed address")
			partialSelectedReceive(t, r, advertiseCaptureOwner, 701, source, ctxID, attrs, prefix)
			held, found := r.bgpPeers[advertiseCaptureOwner].LookupRetained(family.IPv4Unicast, prefix)
			require.True(t, found, "the valid received candidate must remain in Adj-RIB-In")
			assert.Equal(t, advertiseCaptureOwner, held.SourcePeerOwner())
			assert.Equal(t, uint64(701), held.SourceReceipt)
			stored, err := held.ToWireBytes()
			held.Release()
			require.NoError(t, err)
			partialSelectedExpect(t, stored, tc.flags, []byte{0, 1, 0, 1})

			first, err := r.advertiseSnapshot()
			require.NoError(t, err)
			require.Len(t, first, 1, "the resolved received candidate must be elected")
			assert.Positive(t, first[0].revision)
			assert.Equal(t, netip.MustParsePrefix("198.18.93.0/24"), first[0].key.prefix)
			assert.Equal(t, source, first[0].route.Source)
			assert.Equal(t, prefix, first[0].route.NLRI)
			assert.Equal(t, source.AsSlice(), first[0].route.NextHop)
			assert.Equal(t, uint64(701), first[0].route.SourceReceipt, "the selected route keeps its issued source receipt")
			assert.Equal(t, attrs, first[0].route.Attributes)
			partialSelectedProjection(t, &first[0], tc.flags, []byte{0, 1, 0, 1})
			saved := bytes.Clone(first[0].route.Attributes)

			// Existing attachment replaces the pooled value, not its received
			// Partial fact. The previous capture must outlive that replacement.
			attached := false
			modified := r.bgpPeers[advertiseCaptureOwner].ModifyFamilyEntry(family.IPv4Unicast, prefix,
				func(entry *storage.RouteEntry) {
					attached = r.attachCommunity(entry, []byte{0, 2, 0, 2})
				})
			require.True(t, modified)
			require.True(t, attached)
			_, changed := r.checkBestPathChange(family.IPv4Unicast, prefix, false, nil)
			require.True(t, changed, "the appended value must produce a fresh selected capture")
			second, err := r.advertiseSnapshot()
			require.NoError(t, err)
			require.Len(t, second, 1)
			assert.Greater(t, second[0].revision, first[0].revision)
			assert.Equal(t, saved, first[0].route.Attributes, "the old capture owns its bytes")
			partialSelectedProjection(t, &second[0], tc.flags, []byte{0, 1, 0, 1, 0, 2, 0, 2})

			// A fresh complete receipt replaces the locally appended value and
			// any previous Partial flag. The complete twin stays complete, and
			// the old received and attached snapshots remain immutable.
			replacement := bytes.Clone(attrs)
			replacement[len(replacement)-7] = 0xc0
			partialSelectedReceive(t, r, advertiseCaptureOwner, 701, source, ctxID, replacement, prefix)
			third, err := r.advertiseSnapshot()
			require.NoError(t, err)
			require.Len(t, third, 1)
			assert.Greater(t, third[0].revision, second[0].revision)
			assert.Equal(t, saved, first[0].route.Attributes)
			partialSelectedExpect(t, second[0].route.Attributes, tc.flags,
				[]byte{0, 1, 0, 1, 0, 2, 0, 2})
			partialSelectedProjection(t, &third[0], 0xc0, []byte{0, 1, 0, 1})
		})
	}
}

func partialSelectedReceive(t *testing.T, r *RIBManager, owner, receipt uint64, source netip.Addr,
	ctxID bgpctx.ContextID, attrs, prefix []byte) {
	t.Helper()
	update := message.Update{PathAttributes: attrs, NLRI: prefix}
	buffer := make([]byte, message.MaxMsgLen)
	n := update.WriteTo(buffer, 0, nil)
	require.Greater(t, n, message.HeaderLen)
	body := buffer[message.HeaderLen:n]
	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetSourceOwner(owner)
	wu.SetSourceReceipt(receipt)
	aw, err := wu.Attrs()
	require.NoError(t, err)
	r.handleReceivedStructured(&rpc.StructuredEvent{ConnectionID: 41,
		EventType: rpc.EventKindUpdate, PeerOwner: owner, SourceReceipt: receipt,
		PeerAddress: source.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000,
		RawMessage: &bgptypes.RawMessage{Type: msgtype.TypeUPDATE,
			SourceOwner: owner, SourceReceipt: receipt, RawBytes: body, WireUpdate: wu, AttrsWire: aw},
	})
}

// partialSelectedPassive keeps useful received Partial bytes and forwarding
// while refusing to manufacture a learned selection from an unleased source.
func partialSelectedPassive(t *testing.T, source netip.Addr, ctxID bgpctx.ContextID, attrs, prefix []byte, flags byte) {
	t.Helper()
	r := newTestRIBManager(t)
	t.Cleanup(func() {
		for _, routes := range r.bgpPeers {
			routes.Release()
		}
	})
	loc := electionFixtureLocRIB(t, r, source)
	partialSelectedReceive(t, r, advertiseCaptureOwner, 0, source, ctxID, attrs, prefix)
	held, found := r.bgpPeers[advertiseCaptureOwner].LookupRetained(family.IPv4Unicast, prefix)
	require.True(t, found, "the passive twin still stores its useful received route")
	assert.Zero(t, held.SourceReceipt, "passive storage invents no source grant")
	assert.Zero(t, held.SourcePeerOwner())
	stored, err := held.ToWireBytes()
	held.Release()
	require.NoError(t, err)
	partialSelectedExpect(t, stored, flags, []byte{0, 1, 0, 1})
	best, present := loc.Best(family.IPv4Unicast, netip.MustParsePrefix("198.18.93.0/24"))
	require.True(t, present, "the passive twin remains useful for forwarding")
	require.Equal(t, source, best.NextHop)
	selected, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Empty(t, selected, "an unleased source has no ordinary selected authority")
	require.Nil(t, r.sourceLeases)
}

func partialSelectedExpect(t *testing.T, attrs []byte, expected byte, values []byte) {
	t.Helper()
	_, flags, value, found := attribute.AttrFind(attrs, attribute.AttrCommunity)
	require.True(t, found, "the useful selected route must carry its ordinary community")
	assert.Equal(t, expected, byte(flags), "the literal received Partial flag must survive")
	assert.Equal(t, values, value)
}

func partialSelectedProjection(t *testing.T, capture *advertiseRoute, flags byte, values []byte) {
	t.Helper()
	ctx := selectedDispatchContext(t)
	// One publication names one route. A full channel waits only for this
	// bounded context; stopping the dispatcher MUST join its worker.
	requests := make(chan rpc.SelectRoutesInput, 1)
	send := func(ctx context.Context, input rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
		select {
		case requests <- input:
			return selectedDispatchAnswer(input, rpc.SelectedWritten), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	dispatch, err := newSelectedDispatch(ctx, send, nil, nil)
	require.NoError(t, err)
	defer func() {
		// Cleanup MUST still join if the observation context has expired.
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, dispatch.stop(cleanup))
	}()
	target := netip.MustParseAddr("192.0.2.89")
	require.NoError(t, dispatch.publish(capture))
	require.NoError(t, dispatch.destinationUp(target, 89))
	input := selectedDispatchAwait(t, ctx, requests)
	require.NoError(t, dispatch.barrier(ctx, target))
	require.Len(t, input.Routes, 1)
	projected := input.Routes[0]
	require.Equal(t, rpc.SelectedRouteReceived, projected.Kind)
	require.NotNil(t, projected.Route)
	assert.Equal(t, capture.keyNLRI, projected.KeyNLRI)
	assert.Equal(t, hex.EncodeToString(capture.route.NLRI), projected.Route.NLRIHex)
	assert.Equal(t, capture.route.Source.String(), projected.Route.SourcePeer)
	assert.Equal(t, capture.route.SourceReceipt, projected.SourceReceipt, "a projection preserves its issued Session receipt")
	attrs, err := hex.DecodeString(projected.Route.AttrHex)
	require.NoError(t, err)
	partialSelectedExpect(t, attrs, flags, values)
}
