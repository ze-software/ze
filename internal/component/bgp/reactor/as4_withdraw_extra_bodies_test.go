// Design: docs/architecture/edge-cases/as4.md — where the reconciliation runs
// Related: session_read.go — processValidatedMessage dispatch of the withdraw-only bodies
// Related: as4_withdraw_export_chain_test.go — the same contract for the primary body

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// twoOctetMultiFamilySession is a two-octet receive session (no ASN4) that
// negotiated IPv4 unicast, IPv6 unicast and IPv4 BGP-MUP, so one UPDATE can
// produce the withdraw-only bodies dispatched beside the primary one.
func twoOctetMultiFamilySession(t *testing.T) *Session {
	t.Helper()

	settings := collapseSettings()
	caps := []capability.Capability{
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: mupFam.AFI, SAFI: mupFam.SAFI},
	}
	s := NewSession(settings)
	s.nextHopScope.Store(&receiveNextHopScope{
		addresses: []netip.Prefix{
			netip.MustParsePrefix("10.9.0.2/24"),
			netip.MustParsePrefix("192.0.2.2/24"),
		},
		local:  netip.MustParseAddr("10.9.0.2"),
		remote: settings.Address,
		direct: true,
	})
	s.negotiated = capability.Negotiate(caps, caps, capability.PeerIdentity{LocalASN: settings.LocalAS, PeerASN: settings.PeerAS, Internal: settings.IsIBGP()})
	require.False(t, s.negotiated.ASN4, "the fixture must be a two-octet session")
	ctxID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedRecv(s.negotiated))
	require.NoError(t, err)
	s.setRecvCtxID(ctxID)
	collapseEstablish(t, s)
	return s
}

// receiveAllUpdates drives body through processMessage and answers every
// UPDATE the receive path dispatched, the primary and each withdraw-only body.
func receiveAllUpdates(t *testing.T, s *Session, body []byte) []*wireu.WireUpdate {
	t.Helper()

	var dispatched []*wireu.WireUpdate
	s.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if direction == rpc.DirectionReceived && wu != nil {
			dispatched = append(dispatched, wu)
		}
		return false
	}
	hdr := message.Header{Length: uint16(message.HeaderLen + len(body)), Type: msgtype.TypeUPDATE} //nolint:gosec // fixture bodies are small
	processErr, _ := s.processMessage(&hdr, body, BufHandle{ID: noPoolBufID, Buf: body})
	require.NoError(t, processErr)
	return dispatched
}

// requireFourOctetContexts fails unless every dispatched UPDATE carries a
// four-octet source context and names the families it withdraws.
func requireFourOctetContexts(t *testing.T, dispatched []*wireu.WireUpdate) map[family.Family]bool {
	t.Helper()

	withdrawn := map[family.Family]bool{}
	for i, wu := range dispatched {
		ctx := bgpctx.Registry.Get(wu.SourceCtxID())
		require.NotNil(t, ctx, "dispatched UPDATE %d has no registered context", i)
		assert.True(t, ctx.ASN4(), "dispatched UPDATE %d left ingest in the two-octet context", i)
		unreach, err := wu.MPUnreach()
		require.NoError(t, err)
		if unreach != nil {
			withdrawn[unreach.Family()] = true
		}
	}
	return withdrawn
}

// twoOctetPathAttrs is ORIGIN origin and the two-octet AS_PATH [source AS].
func twoOctetPathAttrs(origin byte) []byte {
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{origin})
	return append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath),
		collapseASPathValue(2, collapseSourceAS))...)
}

// mpReachOnly is the MP_REACH_NLRI attribute of mpReachAttrs, without the
// ORIGIN and empty AS_PATH that helper puts in front of it.
func mpReachOnly(fam family.Family, nlri []byte) []byte {
	return mpReachAttrs(fam, nlri)[extLenMPReachOffset:]
}

// TestTwoOctetWithdrawOnlyBodiesCarryTheFourOctetContext drives the two
// withdraw-only body producers of the receive path on a two-octet session.
//
// Method: the typed NLRI discard (RFC 7606 Section 5.4, here a malformed
// BGP-MUP T1ST route beside another family's MP_UNREACH) and the
// treat-as-withdraw split across MP families (RFC 7606 Section 3(g)) each
// dispatch a body beside the primary. A withdrawal holds no AS number, so each
// is in the four-octet form every UPDATE leaving ingest carries, and an export
// member reading as-path refuses a two-octet context.
func TestTwoOctetWithdrawOnlyBodiesCarryTheFourOctetContext(t *testing.T) {
	l := mupWireNLRI(3, t1stBody(1, 0x00)...)
	k := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0)...)
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)
	other := []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}

	t.Run("typed NLRI discard", func(t *testing.T) {
		s := twoOctetMultiFamilySession(t)
		held := append(twoOctetPathAttrs(0x00), mpReachOnly(mupFam, k0)...)
		receiveAllUpdates(t, s, makeUpdateBody(nil, held, nil))

		attrs := append(twoOctetPathAttrs(0x00), mpReachOnly(mupFam, append(append([]byte{}, l...), k...))...)
		attrs = append(attrs, mpUnreachAttrs(family.IPv6Unicast, other)...)
		dispatched := receiveAllUpdates(t, s, makeUpdateBody(nil, attrs, nil))
		require.Len(t, dispatched, 2, "K rides its own withdraw-only UPDATE beside the primary")
		withdrawn := requireFourOctetContexts(t, dispatched)
		assert.True(t, withdrawn[mupFam], "the malformed MUP route is withdrawn")
	})

	t.Run("treat-as-withdraw across families", func(t *testing.T) {
		s := twoOctetMultiFamilySession(t)
		// ORIGIN 7 is treat-as-withdraw (RFC 7606 Section 7.1).
		attrs := append(twoOctetPathAttrs(0x07), mpReachOnly(mupFam, l)...)
		attrs = append(attrs, mpUnreachAttrs(family.IPv6Unicast, other)...)
		dispatched := receiveAllUpdates(t, s, makeUpdateBody(nil, attrs, nil))
		require.Len(t, dispatched, 2, "each MP family rides its own withdraw-only UPDATE")
		withdrawn := requireFourOctetContexts(t, dispatched)
		assert.True(t, withdrawn[mupFam], "the MUP route is withdrawn")
		assert.True(t, withdrawn[family.IPv6Unicast], "the IPv6 unicast route is withdrawn")
	})
}
