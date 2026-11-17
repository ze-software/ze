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
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestReceivedPartialSurvivesSelectedCapture feeds a valid structured UPDATE
// through received storage, election, owned capture and selected RPC projection.
// RFC 4271 Section 5: "If a path with a recognized, transitive optional attribute
// is accepted and passed along to other BGP peers and the Partial bit in the
// Attribute Flags octet is set to 1 by some previous AS, it MUST NOT be set back
// to 0 by the current AS." A complete same-value control must remain complete.
// The manager is passive: its zero source receipt grants no Session authority.
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
			r, source := advertiseCaptureRIB(t)
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
			partialSelectedReceive(t, r, source, ctxID, attrs, prefix)
			held, found := r.bgpPeers[source].LookupRetained(family.IPv4Unicast, prefix)
			require.True(t, found, "the valid received candidate must remain in Adj-RIB-In")
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
			assert.Zero(t, first[0].route.SourceReceipt, "passive storage invents no source grant")
			assert.Equal(t, attrs, first[0].route.Attributes)
			partialSelectedProjection(t, &first[0], tc.flags, []byte{0, 1, 0, 1})
			saved := bytes.Clone(first[0].route.Attributes)

			// Existing attachment replaces the pooled value, not its received
			// Partial fact. The previous capture must outlive that replacement.
			attached := false
			modified := r.bgpPeers[source].ModifyFamilyEntry(family.IPv4Unicast, prefix,
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
			partialSelectedReceive(t, r, source, ctxID, replacement, prefix)
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

func partialSelectedReceive(t *testing.T, r *RIBManager, source netip.Addr,
	ctxID bgpctx.ContextID, attrs, prefix []byte) {
	t.Helper()
	update := message.Update{PathAttributes: attrs, NLRI: prefix}
	buffer := make([]byte, message.MaxMsgLen)
	n := update.WriteTo(buffer, 0, nil)
	require.Greater(t, n, message.HeaderLen)
	feedReceived(r, source, ctxID, buffer[message.HeaderLen:n])
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
	assert.Zero(t, projected.SourceReceipt, "a projection is not a fabricated Session grant")
	attrs, err := hex.DecodeString(projected.Route.AttrHex)
	require.NoError(t, err)
	partialSelectedExpect(t, attrs, flags, values)
}
