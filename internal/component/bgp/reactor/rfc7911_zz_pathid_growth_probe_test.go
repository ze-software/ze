// Design: docs/architecture/bgp/structural-forwarding.md -- Path Identifier lifetime.
// Related: forward_path_id.go -- fwdReleaseWithdrawnPathIDs.
package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/source"
)

// TestPathIDKeyFollowsWhatTheSourceFramed checks the consumer lifetime, not the
// table's choice of keys. Two prefixes may share an outgoing identifier; what
// matters is that retiring either prefix cannot renumber the surviving path.
// RFC requirement: RFC7911-2-2 positive -- each re-advertised path keeps its
// locally assigned identity through replacement and withdrawal, including when
// its source uses the same received identifier for another prefix.
// MUTATION: Key framed paths by source and received identifier alone, then free
// that entry on one prefix's withdrawal; the surviving prefix changes identity.
func TestPathIDKeyFollowsWhatTheSourceFramed(t *testing.T) {
	destCtx, destCtxID := registerForwardBodyTestContext(t, true, true)
	for index, mode := range []string{"same-context", "cross-context", "unframed-source"} {
		t.Run(mode, func(t *testing.T) {
			src := source.SourceID(4301 + index)
			t.Cleanup(func() { fwdPathIDs.releaseSource(src) })
			srcCtxID := destCtxID
			framed := mode != "unframed-source"
			if mode == "cross-context" {
				ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{
					family.IPv4Unicast: true, family.IPv6Unicast: true,
				})
				var err error
				srcCtxID, err = bgpctx.Registry.Register(ctx)
				require.NoError(t, err)
			}
			if !framed {
				_, srcCtxID = registerForwardBodyTestContext(t, true, false)
			}
			if mode != "same-context" {
				require.NotEqual(t, srcCtxID, destCtxID, "fixture must exercise conversion")
			}
			peer := forwardBodyTestPeer(destCtx, destCtxID)
			emit := func(prefix netip.Prefix, withdraw bool) uint32 {
				t.Helper()
				native := nlri.NewINET(family.IPv4Unicast, prefix, 0).Bytes()
				section := native
				if framed {
					section = append(binary.BigEndian.AppendUint32(nil, 0), native...)
				}
				body := fwdShapeBody(nil, forwardBodyBaseAttrs(t, 65001), section)
				if withdraw {
					body = fwdShapeBody(section, nil, nil)
				}
				wire := fwdPathIDWire(body, srcCtxID, src)
				result, ok := buildFwdBody(wire, message.MaxMsgLen, destCtxID, peer,
					netip.MustParseAddr("192.0.2.10"), &fwdParseCache{})
				require.True(t, ok)
				defer returnReadBuffer(result.transcodeBuf)
				var update *message.Update
				if mode == "same-context" {
					require.Len(t, result.rawBodies, 1)
					var err error
					update, err = message.UnpackUpdate(result.rawBodies[0])
					require.NoError(t, err)
				} else {
					require.Empty(t, result.rawBodies)
					require.Len(t, result.updates, 1)
					update = result.updates[0]
				}
				out := update.NLRI
				if withdraw {
					require.Empty(t, update.NLRI)
					out = update.WithdrawnRoutes
				} else {
					require.Empty(t, update.WithdrawnRoutes)
				}
				iter := nlri.NewNLRIIterator(out, true)
				got, id, found := iter.Next()
				require.True(t, found)
				require.Equal(t, native, got, "the identifier must accompany the exact prefix")
				require.Zero(t, iter.Remaining(), "one complete native prefix must be emitted")
				if withdraw {
					// Model cache eviction only after the recipient has its withdrawal.
					fwdReleaseWithdrawnPathIDs(&ReceivedUpdate{WireUpdate: wire})
				}
				return id
			}

			firstPrefix := netip.MustParsePrefix("10.1.0.0/24")
			secondPrefix := netip.MustParsePrefix("10.2.0.0/24")
			first, second := emit(firstPrefix, false), emit(secondPrefix, false)
			require.Equal(t, first, emit(firstPrefix, false), "replacement keeps the first path")
			require.Equal(t, first, emit(firstPrefix, true), "withdrawal removes the first path")
			require.Equal(t, second, emit(secondPrefix, false), "retiring a sibling must not renumber the survivor")
			require.Equal(t, second, emit(secondPrefix, true), "the survivor remains withdrawable")
		})
	}
}
