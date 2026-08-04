// RFC: rfc/short/rfc7606.md -- Section 5.4 typed NLRI, Section 4 structural errors
// Overview: session_validation_nlritype.go -- the ingress filter these tests drive
//
// Two ways a peer could reach past the Section 5.4 filter with bytes it chooses, both
// closed here. One abandons the validator's attribute walk after the MP attribute has
// already been read. The other is the ordinary ADD-PATH encoding, where a wrong answer
// would not bypass the filter but would tear the session down instead.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned closes a one-octet bypass of
// the Section 5.4 MUST.
//
// VALIDATES: RFC 7606 Section 5.4 on an UPDATE whose attribute section ALSO carries a
// Section 4 structural error AFTER the MP attribute. ValidateUpdateRFC7606AddPath
// (message/rfc7606.go) abandons its walk at such an error and returns treat-as-withdraw.
// It must still report where the MP NLRI was, because it had already read it.
//
// PREVENTS: the exact shape the split-error bypass had. Put MP_REACH first so the walk
// records it, then append one attribute whose declared length overruns the section. The
// walk used to return a zero MPNLRILocation, Session.typedNLRIEdit found !loc.Present and
// filtered nothing, and message.mpUnreachAttrList (message/rfc7606_withdraw.go) then
// rescanned the attributes with its own iterator, converted the untouched MP_REACH into an
// MP_UNREACH carrying the same unrecognized NLRI, and processMessage dispatched it to every
// peer that negotiated the family. One octet, and the MUST was gone.
//
// The malformed attribute is deliberately NOT one validateAttribute rejects: a bad ORIGIN
// goes through recordError and continue, so the walk runs to completion and the location
// survives anyway. Only a Section 4 framing error abandons the walk, which is why
// TestRFC7606Section54FiltersTreatAsWithdrawSynthesis did not catch this.
//
// RFC requirement: RFC7606-5.4-1 positive -- an unrecognized NLRI type is discarded even when a later attribute's framing abandons the Section 4 walk, so it never rides out inside the synthesized withdrawal.
func TestRFC7606Section54FiltersWhenTheAttributeWalkIsAbandoned(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	keep := evpnWireNLRI(2, 0xaa)
	drop := evpnWireNLRI(99, 0xbb)
	nlri := append(append([]byte{}, keep...), drop...)

	// MP_REACH first, so the walk reads and records it before the error below.
	attrs := mpReachAttrs(evpnFam, nlri)
	// RFC 7606 Section 4: "attribute length ... exceeds the amount of data" -- one attribute
	// header declaring 64 octets of value with none following. Structural, so the walk
	// cannot continue and returns from inside the loop.
	attrs = append(attrs, 0x40, 0x02, 0x40)

	body := makeUpdateBody(nil, attrs, nil)
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
		"an attribute whose length overruns the section is treat-as-withdraw, the path under test")

	bodies := message.SynthesizeWithdrawFamilies(wu.Payload(), acceptEveryFamily)
	require.Len(t, bodies, 1, "the surviving route must still be withdrawn")
	assert.NotContains(t, string(bodies[0]), string(drop),
		"the unrecognized route type must not ride out inside the synthesized withdrawal")
	assert.Contains(t, string(bodies[0]), string(keep),
		"the implemented route must still be withdrawn, so this is not a vacuous absence")
}

// TestRFC7606Section54ReadsTypedNLRIUnderAddPath proves the filter reads the route type at
// the right offset when RFC 7911 is negotiated for a typed family.
//
// VALIDATES: RFC 7911 Section 5 -- each NLRI in an ADD-PATH family is prefixed with a
// 4-octet Path Identifier -- composed with the Section 5.4 discard. splitTypeLength
// (nlrisplit/typelen.go) counts the identifier before the family's fixed header, and every
// recognizer skips it the same way.
//
// PREVENTS: the failure mode the discard newly made expensive. Before this work an
// ADD-PATH verdict that was wrong for l2vpn/evpn cost nothing on this path, because nothing
// walked the NLRI. Now a wrong verdict makes the walk read the path identifier as a route
// type and length, overrun the attribute, and session-reset the peer. Every existing tagged
// test drives addPath=false, so nothing held the composed path down.
//
// RFC requirement: RFC7911-5-3 positive -- a typed-family UPDATE whose NLRIs carry 4-octet Path Identifiers is carved on those boundaries, so the implemented route survives with its identifier intact and the session is not reset.
// RFC requirement: RFC7606-5.4-1 positive -- the Section 5.4 discard reads the route type past the RFC 7911 Path Identifier rather than at a fixed offset.
func TestRFC7606Section54ReadsTypedNLRIUnderAddPath(t *testing.T) {
	registerEVPNRecognizer(t)
	s := nlriTypeTestSession()

	ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{evpnFam: true})
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	s.recvCtxID = ctxID
	require.True(t, ctx.AddPathFor(evpnFam), "the fixture must actually negotiate ADD-PATH")

	pathID := []byte{0x00, 0x00, 0x00, 0x07}
	keep := append(append([]byte{}, pathID...), evpnWireNLRI(2, 0xaa)...)
	drop := append(append([]byte{}, pathID...), evpnWireNLRI(99, 0xbb)...)
	nlri := append(append([]byte{}, keep...), drop...)

	body := makeUpdateBody(nil, mpReachAttrs(evpnFam, nlri), nil)
	wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionNone, action,
		"a well-formed ADD-PATH UPDATE must not be treated as malformed or session-reset")

	got, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found, "the implemented route survives, so the attribute must remain")
	assert.Equal(t, keep, got,
		"the unrecognized type goes and the surviving route keeps its Path Identifier")
}
