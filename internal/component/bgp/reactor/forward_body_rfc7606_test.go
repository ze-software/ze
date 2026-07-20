package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 7606 Section 5.1, second bullet: "An UPDATE message MUST NOT contain more than one of
// the following: non-empty Withdrawn Routes field, non-empty Network Layer Reachability
// Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."
//
// Ze is the sender of the bytes it relays, even when another speaker composed them, so the
// obligation applies to forwarding. Both splitters already emitted one field per message; the
// two branches that emitted an UPDATE WHOLE because it fit did not, and those are what these
// tests pin. The receive side is untouched: Section 5.1 also requires an implementation to be
// "prepared to receive these fields in any position or combination".

// fwdShapeBody builds an UPDATE body from explicit sections, unlike buildRawUpdateBody, which
// hardcodes an empty Withdrawn Routes field.
func fwdShapeBody(withdrawn, attrs, nlriBytes []byte) []byte {
	body := make([]byte, 0, 4+len(withdrawn)+len(attrs)+len(nlriBytes))
	body = binary.BigEndian.AppendUint16(body, uint16(len(withdrawn)))
	body = append(body, withdrawn...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	body = append(body, nlriBytes...)
	return body
}

// fwdShapeFieldCount counts the NLRI-bearing fields of a forwarded raw body.
func fwdShapeFieldCount(t *testing.T, body []byte) int {
	t.Helper()
	u, err := message.UnpackUpdate(body)
	require.NoError(t, err)
	return message.NLRIBearingFieldCount(u.WithdrawnRoutes, u.PathAttributes, u.NLRI)
}

// VALIDATES: AC-1 -- a same-context UPDATE that FITS but mixes Withdrawn Routes with NLRI is
// split before being relayed, and nothing is lost.
// PREVENTS: forward_body.go's zero-copy branch passing a received mixed shape straight
// through. Removing the MixesNLRIFields condition there leaves a single body here, carrying
// two NLRI-bearing fields.
//
// RFC requirement: RFC7606-5.1-2 positive -- a relayed UPDATE mixing NLRI-bearing fields is
// split so that each emitted message carries at most one of them.
func TestForwardSplitsMixedShapeSameContextThatFits(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(ctx, ctxID)

	withdrawn := forwardBodyNLRIs(3, false)
	announced := forwardBodyNLRIs(4, false)
	body := fwdShapeBody(withdrawn, forwardBodyBaseAttrs(t, 65000), announced)
	require.LessOrEqual(t, message.HeaderLen+len(body), message.MaxMsgLen,
		"guard: the UPDATE must FIT, so only its shape can force a split")
	require.Equal(t, 2, fwdShapeFieldCount(t, body), "guard: the fixture must start non-compliant")

	result, ok := buildFwdBody(wireu.NewWireUpdate(body, ctxID), message.MaxMsgLen, ctxID, peer,
		netip.MustParseAddr("192.0.2.10"), &fwdParseCache{})
	require.True(t, ok)
	require.Empty(t, result.updates, "same-context forwarding must stay on the raw path")
	require.Greater(t, len(result.rawBodies), 1, "a mixed UPDATE must be split even when it fits")

	var gotWithdrawn, gotNLRI []byte
	for i, raw := range result.rawBodies {
		assert.LessOrEqualf(t, fwdShapeFieldCount(t, raw), 1,
			"forwarded body %d carries more than one NLRI-bearing field", i)
		u, err := message.UnpackUpdate(raw)
		require.NoError(t, err)
		gotWithdrawn = append(gotWithdrawn, u.WithdrawnRoutes...)
		gotNLRI = append(gotNLRI, u.NLRI...)
	}
	assert.Equal(t, withdrawn, gotWithdrawn, "withdrawn routes must survive the split")
	assert.Equal(t, announced, gotNLRI, "announced routes must survive the split")
}

// VALIDATES: AC-6 -- the split relay emits withdrawals before announcements.
// PREVENTS: a peer briefly re-learning a route ze meant to withdraw. Splitting makes the
// ordering observable where a single message had no ordering to get wrong.
func TestForwardSplitMixedShapeWithdrawalsFirst(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(ctx, ctxID)

	body := fwdShapeBody(forwardBodyNLRIs(3, false), forwardBodyBaseAttrs(t, 65000), forwardBodyNLRIs(4, false))
	result, ok := buildFwdBody(wireu.NewWireUpdate(body, ctxID), message.MaxMsgLen, ctxID, peer,
		netip.MustParseAddr("192.0.2.11"), &fwdParseCache{})
	require.True(t, ok)

	seenAnnounce := false
	for i, raw := range result.rawBodies {
		u, err := message.UnpackUpdate(raw)
		require.NoError(t, err)
		if len(u.NLRI) > 0 {
			seenAnnounce = true
			continue
		}
		if len(u.WithdrawnRoutes) > 0 {
			assert.Falsef(t, seenAnnounce, "body %d withdraws after an announcement was sent", i)
		}
	}
	require.True(t, seenAnnounce, "guard: the fixture announces something")
}

// VALIDATES: AC-4 -- a compliant same-context UPDATE that fits is still forwarded by handing
// on the received bytes themselves.
// PREVENTS: paying for RFC 7606 shape compliance on every forward. The assertion is pointer
// identity of the backing array, not equality: a copy would compare equal while having lost
// the zero-copy property that forward_body.go:48-50 exists to protect.
//
// RFC requirement: RFC7606-5.1-2 negative -- an UPDATE with a single NLRI-bearing field
// already satisfies the restriction, so it is relayed unchanged rather than split.
func TestForwardCompliantShapeKeepsZeroCopy(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(ctx, ctxID)

	body := fwdShapeBody(nil, forwardBodyBaseAttrs(t, 65000), forwardBodyNLRIs(4, false))
	require.Equal(t, 1, fwdShapeFieldCount(t, body))

	wu := wireu.NewWireUpdate(body, ctxID)
	result, ok := buildFwdBody(wu, message.MaxMsgLen, ctxID, peer,
		netip.MustParseAddr("192.0.2.12"), &fwdParseCache{})
	require.True(t, ok)
	require.Len(t, result.rawBodies, 1)
	require.Same(t, &body[0], &result.rawBodies[0][0],
		"a compliant UPDATE must be forwarded as the received bytes, not a copy")
}

// VALIDATES: AC-5 -- End-of-RIB is forwarded untouched.
// PREVENTS: the shape branch rewriting an EoR marker, which would break RFC 4724 graceful
// restart for the receiving peer.
func TestForwardEndOfRIBUnaffected(t *testing.T) {
	ctx, ctxID := registerForwardBodyTestContext(t, true, false)
	peer := forwardBodyTestPeer(ctx, ctxID)

	body := fwdShapeBody(nil, nil, nil)
	wu := wireu.NewWireUpdate(body, ctxID)
	result, ok := buildFwdBody(wu, message.MaxMsgLen, ctxID, peer,
		netip.MustParseAddr("192.0.2.13"), &fwdParseCache{})
	require.True(t, ok)
	require.Len(t, result.rawBodies, 1)
	assert.Same(t, &body[0], &result.rawBodies[0][0], "an EoR must be forwarded verbatim")
}

// VALIDATES: AC-2 -- the context-mismatch branch also splits a mixed shape that fits, after
// re-encoding for the destination.
// PREVENTS: the second whole-emit branch reproducing the mixed shape. This one is ze's own
// composition (fwdUpdateForDestination rebuilt the sections), so it is the clearer violation
// of the two.
//
// RFC requirement: RFC7606-5.1-2 positive -- an UPDATE re-encoded for a destination context
// is emitted as one NLRI-bearing field per message.
func TestForwardSplitsMixedShapeAcrossContextsThatFits(t *testing.T) {
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	withdrawn := forwardBodyNLRIs(3, false)
	announced := forwardBodyNLRIs(4, false)
	// An ASN4 mismatch forces the re-encode branch while leaving NLRI framing alone.
	body := fwdShapeBody(withdrawn, forwardBodyBaseAttrs(t, 200000), announced)
	require.LessOrEqual(t, message.HeaderLen+len(body), message.MaxMsgLen, "guard: must FIT")

	result, ok := buildFwdBody(wireu.NewWireUpdate(body, srcCtxID), message.MaxMsgLen, destCtxID, peer,
		netip.MustParseAddr("192.0.2.14"), &fwdParseCache{})
	require.True(t, ok)
	require.Empty(t, result.rawBodies, "mismatched contexts must not reuse the source bytes")
	require.Greater(t, len(result.updates), 1, "a mixed re-encoded UPDATE must be split")

	var gotWithdrawn, gotNLRI []byte
	for i, u := range result.updates {
		assert.LessOrEqualf(t,
			message.NLRIBearingFieldCount(u.WithdrawnRoutes, u.PathAttributes, u.NLRI), 1,
			"emitted UPDATE %d carries more than one NLRI-bearing field", i)
		gotWithdrawn = append(gotWithdrawn, u.WithdrawnRoutes...)
		gotNLRI = append(gotNLRI, u.NLRI...)
	}
	assert.Equal(t, withdrawn, gotWithdrawn)
	assert.Equal(t, announced, gotNLRI)
}

// VALIDATES: a compliant UPDATE crossing contexts and fitting is still emitted as one whole
// UPDATE, without going through the splitter.
// PREVENTS: the shape check pushing the ordinary re-encode path into per-section copies.
func TestForwardCompliantShapeAcrossContextsNotSplit(t *testing.T) {
	_, srcCtxID := registerForwardBodyTestContext(t, true, false)
	destCtx, destCtxID := registerForwardBodyTestContext(t, false, false)
	peer := forwardBodyTestPeer(destCtx, destCtxID)

	body := fwdShapeBody(nil, forwardBodyBaseAttrs(t, 200000), forwardBodyNLRIs(4, false))
	result, ok := buildFwdBody(wireu.NewWireUpdate(body, srcCtxID), message.MaxMsgLen, destCtxID, peer,
		netip.MustParseAddr("192.0.2.15"), &fwdParseCache{})
	require.True(t, ok)
	require.Len(t, result.updates, 1, "a compliant UPDATE must not be split")
}
