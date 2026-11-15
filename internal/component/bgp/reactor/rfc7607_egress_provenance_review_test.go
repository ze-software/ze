// Design: docs/architecture/bgp/structural-forwarding.md -- final-writer provenance
// Related: session_rfc7607.go -- post-policy AS-zero replacement
// Related: forward_provenance.go -- section identity bound before writer admission
// RFC: rfc/short/rfc7607.md -- Section 2 forbids propagating AS zero
// RFC naming: untagged -- source-ownership regression controls await their own native fault proof before gaining conformance tags.

package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC7607ExportASZeroWithdrawsHeldForwardedPath drives the actual export
// chain and both relay dispatchers, rather than installing a manufactured
// sentForward manifest. The clean announcement must reach the socket first.
// VALIDATES: a later policy replacement carrying AS zero withdraws that same
// advertised path, including its destination-generated ADD-PATH identifier,
// and leaves the established writer able to advertise an independent route.
// PREVENTS: reach-to-withdraw replacement looking up stale announcement-only
// provenance, or bypassing ownership to avoid that lookup error.
func TestRFC7607ExportASZeroWithdrawsHeldForwardedPath(t *testing.T) {
	for _, addPath := range []bool{false, true} {
		framing := "ordinary"
		if addPath {
			framing = "add-path"
		}
		for _, native := range []bool{false, true} {
			t.Run(framing+"/"+nextHopAutoRailName(native), func(t *testing.T) {
				f := rfc7607EgressProvenanceFixture(t, addPath)
				session := f.destination.currentSession()
				prefix := []byte{24, 203, 0, 113}
				path := append([]byte(nil), prefix...)
				if addPath {
					// RFC 7911 Section 3: the received path ID remains seven
					// across the clean and policy-replaced announcements.
					path = append([]byte{0, 0, 0, 7}, prefix...)
				}
				tc := rfc7607EgressCase{code: attribute.AttrASPath, asn4: true, real: 65011}
				clean := makeUpdateBody(nil, rfc7607EgressAttrs(tc, tc.real, f.source.Settings().Address), path)
				rfc4271ProofRelay(t, f, clean, native)
				before := aigpSocketBodies(t, f.conn)
				require.Len(t, before, 1, "clean route must really be advertised")
				advertised, err := message.UnpackUpdate(before[0])
				require.NoError(t, err)
				require.Empty(t, advertised.WithdrawnRoutes)
				wantWithdrawal := append([]byte(nil), advertised.NLRI...)
				if addPath {
					require.Len(t, wantWithdrawal, 4+len(prefix))
					require.Equal(t, prefix, wantWithdrawal[4:])
				} else {
					require.Equal(t, prefix, wantWithdrawal)
				}

				// A raw export answer is a supported plugin policy surface.
				// The received route stays clean: AS zero is introduced only
				// after the receive validators, before provenance is staged.
				bad := makeUpdateBody(nil, rfc7607EgressAttrs(tc, 0, f.source.Settings().Address), path)
				transport := rawAnswerTransport(bad)
				f.r.filterTransportSeam = transport
				f.r.orderedEgressSteps = buildOrderedEgressSteps()
				f.destination.settings.ExportPolicy = answerPolicy(t, routepolicy.FinalAccept, "as-zero-review")
				f.destination.refreshForwardFacts()
				f.conn.reset()
				rfc4271ProofRelay(t, f, clean, native)
				require.Equal(t, 1, transport.calls(), "the configured export replacement must actually run")
				after := aigpSocketBodies(t, f.conn)
				require.Len(t, after, 1, "held route must be withdrawn through staged relay provenance")
				withdrawal, err := message.UnpackUpdate(after[0])
				require.NoError(t, err)
				require.Empty(t, withdrawal.NLRI)
				require.Empty(t, withdrawal.PathAttributes)
				require.Equal(t, wantWithdrawal, withdrawal.WithdrawnRoutes, "withdraw the actual destination path, not its ingress identifier")
				require.Same(t, session, f.destination.currentSession())
				require.False(t, session.tearingDown.Load(), "route refusal must retain the session")
				session.writeMu.Lock()
				writeErr := session.writeFailed
				session.writeMu.Unlock()
				require.NoError(t, writeErr)
				session.mu.Lock()
				connection := session.conn
				session.mu.Unlock()
				require.Same(t, f.conn, connection, "route refusal must retain the connection")

				// Restore acceptance and prove the same socket still transmits.
				f.destination.settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
				f.destination.refreshForwardFacts()
				marker := append([]byte(nil), path...)
				marker[len(marker)-1] = 114
				if addPath {
					binary.BigEndian.PutUint32(marker, 8)
				}
				f.conn.reset()
				rfc4271ProofRelay(t, f, makeUpdateBody(nil, rfc7607EgressAttrs(tc, tc.real, f.source.Settings().Address), marker), native)
				live := aigpSocketBodies(t, f.conn)
				require.Len(t, live, 1)
				update, err := message.UnpackUpdate(live[0])
				require.NoError(t, err)
				require.Empty(t, update.WithdrawnRoutes)
				if addPath {
					require.Len(t, update.NLRI, len(marker))
					require.Equal(t, marker[4:], update.NLRI[4:])
				} else {
					require.Equal(t, marker, update.NLRI)
				}
			})
		}
	}
}

func rfc7607EgressProvenanceFixture(t *testing.T, addPath bool) *aigpReplayFixture {
	t.Helper()
	f := rfc7607EgressFixture(t, true)
	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: addPath})
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	f.ctxID = ctxID
	for _, peer := range []*Peer{f.source, f.destination} {
		caps := []capability.Capability{capIPv4(), &capability.ASN4{ASN: peer.settings.LocalAS}}
		if addPath {
			caps = append(caps, capAddPathIPv4Both())
		}
		negotiated := capability.Negotiate(caps, caps, capability.PeerIdentity{
			LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
			Internal: peer.settings.IsIBGP(),
		})
		peer.session.negotiated = negotiated
		peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
		peer.sendCtx.Store(ctx)
		peer.sendCtxID, peer.recvCtxID = ctxID, ctxID
		peer.session.sendCtxID, peer.session.recvCtxID = ctxID, ctxID
		peer.refreshForwardFacts()
	}
	return f
}
