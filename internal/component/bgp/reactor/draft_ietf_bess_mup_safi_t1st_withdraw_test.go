package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlritype"
	"github.com/ze-software/ze/internal/core/family"
)

// These tests drive the recognizer the mup plugin's init() registers (mup_test.go links
// the plugin into this binary), not a stand-in: the body verdict under test is the
// plugin's ValidateNLRI.

// keepIf turns a stand-in recognizer's yes-or-no type ruling into its verdict.
func keepIf(implemented bool) nlritype.Verdict {
	if implemented {
		return nlritype.Keep
	}
	return nlritype.Discard
}

var mupIPv6Fam = family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMUP}

// t1stBody is an IPv4 Type 1 Session Transformed route body (draft-ietf-bess-mup-safi
// Section 3.1.3): RD 0:0, prefix 10.0.0.last/32, TEID 1, QFI 9, endpoint 192.0.2.1/32,
// then tail (Source Address Length and TLVs). The Source Address Length is a
// mandatory field, so a well-formed route passes at least 0x00 as tail.
func t1stBody(last byte, tail ...byte) []byte {
	body := make([]byte, 8)
	body = append(body, 32, 10, 0, 0, last, 0, 0, 0, 1, 9, 32, 192, 0, 2, 1)
	return append(body, tail...)
}

// mpUnreachNLRIOf slices the NLRI bytes out of the MP_UNREACH attribute of an UPDATE body.
func mpUnreachNLRIOf(t *testing.T, body []byte) ([]byte, bool) {
	t.Helper()
	attrs := rfc8669PathAttrs(t, body)
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPUnreachNLRI)
	if !found {
		return nil, false
	}
	start, ok := message.MPNLRIStart(uint8(attribute.AttrMPUnreachNLRI), value)
	require.True(t, ok)
	return value[start:], true
}

// holdT1STGeneration has s receive k0, a well-formed generation of a route, so a later
// malformed NLRI of the same key is withdrawn by it.
func holdT1STGeneration(t *testing.T, s *Session, fam family.Family, k0 []byte) {
	t.Helper()
	wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(fam, k0), nil))
	kept, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found)
	require.Equal(t, k0, kept, "the earlier generation is well formed and kept")
}

// requireDecodable fails unless every NLRI of section is one the family's registered
// recognizer keeps: the bytes a receiver decodes when Ze relays the section.
func requireDecodable(t *testing.T, fam family.Family, section []byte, addPath bool) {
	t.Helper()
	recognize := nlritype.Get(fam)
	require.NotNil(t, recognize, "the mup plugin registers the recognizer")
	nlris, err := nlrisplit.Split(fam, section, addPath)
	require.NoError(t, err)
	for _, one := range nlris {
		require.Equal(t, nlritype.Keep, recognize(one, addPath), "Ze relays an NLRI its own recognizer refuses: %x", one)
	}
}

// applyRFC7606Body classifies and applies RFC 7606 to body the way the receive path does,
// returning the validation so a test can read the extra withdraw body.
func applyRFC7606Body(t *testing.T, s *Session, body []byte) (*wireu.WireUpdate, message.RFC7606Action, rfc7606Validation) {
	t.Helper()
	validation := s.classifyRFC7606(body)
	wu, action, err := s.applyRFC7606(wireu.NewWireUpdate(body, 0), &validation)
	require.NoError(t, err, "a malformed T1ST is never a session reset")
	return wu, action, validation
}

// TestMUPMalformedT1STTreatAsWithdraw covers the three malformed shapes
// draft-ietf-bess-mup-safi Section 3.1.3.1 answers with treat-as-withdraw, and the
// well-formed sibling with an unknown TLV that must survive unchanged.
//
// VALIDATES: after a well-formed generation K0 of K's route, the malformed route K
// leaves MP_REACH_NLRI and the route is withdrawn in an MP_UNREACH_NLRI of the same
// UPDATE, by K0's bytes, which the family's recognizer keeps; L, and L2 with TLV 0xF0,
// are kept byte for byte; no session reset.
// PREVENTS: a malformed T1ST being installed and relayed (the defect D3), and its
// withdrawal relayed by the malformed bytes (GoBGP v4.10.0 resets the session on
// them; v3.31.0 panicked decoding them).
func TestMUPMalformedT1STTreatAsWithdraw(t *testing.T) {
	l := mupWireNLRI(3, t1stBody(1, 0x00)...)
	l2 := mupWireNLRI(3, t1stBody(3, 0x00, 0xf0, 0x02, 0xaa, 0xbb)...)
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)
	full := t1stBody(2)
	cases := []struct {
		name string
		k    []byte
	}{
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6 positive -- a T1ST whose Length ends inside the Endpoint Address leaves MP_REACH_NLRI and is withdrawn in MP_UNREACH_NLRI, and its well-formed siblings are kept byte for byte
		{"mandatory fields exceed the Length", mupWireNLRI(3, full[:len(full)-2]...)},
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7 positive -- a T1ST whose TLV area is exactly one octet leaves MP_REACH_NLRI and is withdrawn in MP_UNREACH_NLRI, and its well-formed siblings are kept byte for byte
		{"one octet of TLV area", mupWireNLRI(3, t1stBody(2, 0x00, 0xf0)...)},
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10 positive -- a T1ST whose TLV Length overruns the NLRI leaves MP_REACH_NLRI and is withdrawn in MP_UNREACH_NLRI, and its well-formed siblings are kept byte for byte
		{"TLV overruns the NLRI", mupWireNLRI(3, t1stBody(2, 0x00, 0xf0, 0x05, 0xaa)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := nlriTypeTestSession()
			holdT1STGeneration(t, s, mupFam, k0)
			nlri := append(append(append([]byte{}, l...), tc.k...), l2...)
			wu, action, validation := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, nlri), nil))
			assert.Equal(t, message.RFC7606ActionNone, action)
			assert.Nil(t, validation.typedWithdrawBodies, "no MP_UNREACH was present, so the withdrawal joins the UPDATE")

			kept, found := mpReachNLRIOf(t, wu.Payload())
			require.True(t, found)
			assert.Equal(t, append(append([]byte{}, l...), l2...), kept, "only the malformed route leaves the announcement")
			withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
			require.True(t, found, "the malformed route is withdrawn, not dropped")
			assert.Equal(t, k0, withdrawn, "the route is withdrawn by the generation held for its key")
			requireDecodable(t, mupFam, withdrawn, false)
		})
	}

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6 negative -- an UPDATE of well-formed T1ST routes, one carrying an unknown TLV, is returned unchanged with no withdrawal
	t.Run("well-formed routes untouched", func(t *testing.T) {
		s := nlriTypeTestSession()
		body := makeUpdateBody(nil, mpReachAttrs(mupFam, append(append([]byte{}, l...), l2...)), nil)
		wu, _, _ := applyRFC7606Body(t, s, body)
		assert.Equal(t, body, wu.Payload())
	})
}

// TestMUPMalformedT1STWithdrawEdges covers AC-10 (i) to (iii).
//
// VALIDATES: (i) beside an MP_UNREACH of another family the withdrawal rides its own
// withdraw-only UPDATE and the existing MP_UNREACH is untouched; (ii) when K was the only
// route the UPDATE becomes withdraw-only, never an End-of-RIB; (iii) under ADD-PATH the
// path identifier stays on the withdrawal.
// PREVENTS: a second MP_UNREACH_NLRI in one UPDATE (RFC 7606 Section 3(g) session reset at
// the peer), a forged End-of-RIB, and a withdrawal that names another path.
func TestMUPMalformedT1STWithdrawEdges(t *testing.T) {
	l := mupWireNLRI(3, t1stBody(1, 0x00)...)
	k := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0)...)
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)

	t.Run("another family already withdrawn", func(t *testing.T) {
		s := nlriTypeTestSession()
		holdT1STGeneration(t, s, mupFam, k0)
		other := []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0}
		attrs := append(mpReachAttrs(mupFam, append(append([]byte{}, l...), k...)), mpUnreachAttrs(family.IPv6Unicast, other)...)
		wu, _, validation := applyRFC7606Body(t, s, makeUpdateBody(nil, attrs, nil))
		kept, _ := mpReachNLRIOf(t, wu.Payload())
		assert.Equal(t, l, kept)
		unreach, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, other, unreach, "the other family's withdrawal survives unchanged")
		require.Len(t, validation.typedWithdrawBodies, 1, "K rides its own withdraw-only UPDATE")
		assert.Equal(t, message.BuildMPUnreachBody(uint16(mupFam.AFI), uint8(mupFam.SAFI), k0), validation.typedWithdrawBodies[0])
	})

	t.Run("the only route", func(t *testing.T) {
		s := nlriTypeTestSession()
		holdT1STGeneration(t, s, mupFam, k0)
		wu, action, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, k), nil))
		assert.Equal(t, message.RFC7606ActionNone, action, "a withdrawal is not an emptied UPDATE")
		_, found := mpReachNLRIOf(t, wu.Payload())
		assert.False(t, found)
		withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, k0, withdrawn)
		assert.False(t, updateCarriesNoRoutes(wu.Payload()), "never an End-of-RIB")
	})

	t.Run("ADD-PATH keeps the path identifier", func(t *testing.T) {
		s := nlriTypeTestSession()
		ctx := bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{mupFam: true})
		ctxID, err := bgpctx.Registry.Register(ctx)
		require.NoError(t, err)
		s.recvCtxID = ctxID
		pathID := []byte{0, 0, 0, 7}
		lp := append(append([]byte{}, pathID...), l...)
		kp := append(append([]byte{}, pathID...), k...)
		k0p := append(append([]byte{}, pathID...), k0...)
		// The same route under another path identifier is a different path.
		other := append([]byte{0, 0, 0, 8}, k0...)
		holdT1STGeneration(t, s, mupFam, append(append([]byte{}, k0p...), other...))
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, append(append([]byte{}, lp...), kp...)), nil))
		kept, _ := mpReachNLRIOf(t, wu.Payload())
		assert.Equal(t, lp, kept)
		withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, k0p, withdrawn, "the path held under identifier 7, not the one under 8")
	})
}

// TestMUPUnkeyableMalformedT1STDiscardedNotRelayed covers AC-10a at ingress and on the
// forward rail: an NLRI whose route key cannot form is discarded, never withdrawn.
//
// VALIDATES: K leaves MP_REACH_NLRI, no MP_UNREACH_NLRI is added, the sibling L is kept,
// no session reset; a destination already holding L receives L again and the forward
// batch completes.
// PREVENTS: a withdrawal that names no route reaching the RIB or a writer, and a K in a
// forwarded body abandoning the destination's batch.
func TestMUPUnkeyableMalformedT1STDiscardedNotRelayed(t *testing.T) {
	v6Body := func(prefixBits byte) []byte {
		body := append(make([]byte, 8), prefixBits)
		body = append(body, make([]byte, 17)...) // 129 bits need 17 octets
		return append(body, 0, 0, 0, 1, 9, 32, 192, 0, 2, 1)
	}
	cases := []struct {
		name string
		fam  family.Family
		l, k []byte
	}{
		{"RD overruns the Length", mupFam, mupWireNLRI(3, t1stBody(1, 0x00)...), mupWireNLRI(3, 0, 0, 0, 0, 0)},
		{"IPv6 Prefix Length 129", mupIPv6Fam, mupWireNLRI(1, append(append(make([]byte, 8), 64), make([]byte, 8)...)...), mupWireNLRI(3, v6Body(129)...)},
		{"ISD octet past the prefix", mupFam, mupWireNLRI(1, append(make([]byte, 8), 24, 10, 0, 0)...), mupWireNLRI(1, append(make([]byte, 8), 24, 10, 0, 1, 0xff)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := nlriTypeTestSession()
			nlri := append(append([]byte{}, tc.l...), tc.k...)
			wu, action, validation := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(tc.fam, nlri), nil))
			assert.Equal(t, message.RFC7606ActionNone, action)
			kept, found := mpReachNLRIOf(t, wu.Payload())
			require.True(t, found)
			assert.Equal(t, tc.l, kept)
			_, found = mpUnreachNLRIOf(t, wu.Payload())
			assert.False(t, found, "no withdrawal can name an unkeyable route")
			assert.Nil(t, validation.typedWithdrawBodies)

			// Forward rail: a destination that already holds L receives the edited
			// UPDATE. A K left in the body would fail fwdPathKeyFor inside
			// writeUpdateBody and abandon the rest of the batch, so the later marker
			// proves the batch completed; ownershipRailRead fails on any unkeyable
			// NLRI written to the destination, so no K reaches it.
			f := ownershipRailNew(t, nil)
			f.destination.negotiated.Store(&NegotiatedCapabilities{
				families: map[family.Family]bool{family.IPv4Unicast: true, tc.fam: true},
			})
			f.destination.refreshForwardFacts()
			f.send(t, "pool", f.source, f.ctxID, ownershipRailBody(tc.fam, false, tc.l, 1))
			f.awaitNative(t, tc.fam, 1, tc.l)
			// The edited body carries no MED, so its L arrives with fingerprint 0.
			f.send(t, "pool", f.source, f.ctxID, wu.Payload())
			f.awaitNative(t, tc.fam, 0, tc.l)
			f.marker(t, "pool", 3)
		})
	}
}

// TestMUPMalformedT1STWithdrawnByHeldGeneration covers the held-generation rule on every
// withdrawal a body-judged family relays.
//
// VALIDATES: a malformed announced NLRI whose key holds no generation is removed and
// nothing is relayed for it; a malformed NLRI in a received MP_UNREACH_NLRI is replaced
// by the held generation, or removed when none is held; an unkeyable withdrawal is
// removed; a received withdrawal and a whole-UPDATE treat-as-withdraw forget the held
// generation; a replacement longer than a one-octet attribute length gains the Extended
// Length bit; every withdrawal Ze relays passes the family's own recognizer.
// PREVENTS: Ze relaying an NLRI its own ValidateNLRI refuses (GoBGP v4.10.0's
// MUPType1SessionTransformedRoute.DecodeFromBytes refuses one and the session resets;
// v3.31.0 panicked there), and a withdrawal relayed for a
// route this session never held.
func TestMUPMalformedT1STWithdrawnByHeldGeneration(t *testing.T) {
	l := mupWireNLRI(3, t1stBody(1, 0x00)...)
	k := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0)...)
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)

	t.Run("no generation held, nothing relayed", func(t *testing.T) {
		s := nlriTypeTestSession()
		wu, action, validation := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, append(append([]byte{}, l...), k...)), nil))
		assert.Equal(t, message.RFC7606ActionNone, action)
		kept, found := mpReachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, l, kept)
		_, found = mpUnreachNLRIOf(t, wu.Payload())
		assert.False(t, found, "the Adj-RIB-In held no route for K, so there is nothing to withdraw")
		assert.Nil(t, validation.typedWithdrawBodies)
	})

	t.Run("received malformed withdrawal replaced by the held generation", func(t *testing.T) {
		s := nlriTypeTestSession()
		holdT1STGeneration(t, s, mupFam, k0)
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpUnreachAttrs(mupFam, k), nil))
		withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, k0, withdrawn)
		requireDecodable(t, mupFam, withdrawn, false)
	})

	t.Run("received malformed withdrawal with no generation held", func(t *testing.T) {
		s := nlriTypeTestSession()
		wu, action, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpUnreachAttrs(mupFam, k), nil))
		assert.Equal(t, message.RFC7606ActionTreatAsWithdraw, action, "an UPDATE left with no route is not relayed")
		_, found := mpUnreachNLRIOf(t, wu.Payload())
		assert.False(t, found)
	})

	t.Run("received unkeyable withdrawal removed, well-formed sibling kept", func(t *testing.T) {
		s := nlriTypeTestSession()
		unkeyable := mupWireNLRI(3, 0, 0, 0, 0, 0)
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpUnreachAttrs(mupFam, append(append([]byte{}, unkeyable...), l...)), nil))
		withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, l, withdrawn, "a well-formed withdrawal is relayed as received")
	})

	t.Run("received withdrawal forgets the generation", func(t *testing.T) {
		s := nlriTypeTestSession()
		holdT1STGeneration(t, s, mupFam, k0)
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpUnreachAttrs(mupFam, k0), nil))
		withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, k0, withdrawn)
		wu, _, _ = applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, append(append([]byte{}, l...), k...)), nil))
		_, found = mpUnreachNLRIOf(t, wu.Payload())
		assert.False(t, found, "K left the Adj-RIB-In with the first withdrawal")
	})

	t.Run("whole-UPDATE treat-as-withdraw forgets the generation", func(t *testing.T) {
		s := nlriTypeTestSession()
		holdT1STGeneration(t, s, mupFam, k0)
		pathAttrs := mpReachAttrs(mupFam, k0)
		attrs := nlritype.NewUpdateAttrs(pathAttrs)
		loc := message.MPNLRILocation{Present: true, AFI: uint16(mupFam.AFI), SAFI: uint8(mupFam.SAFI)}
		_, ok := s.typedNLRIEdit(nil, uint8(attribute.AttrMPReachNLRI), loc, pathAttrs, &attrs, nil, true)
		require.True(t, ok)
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, append(append([]byte{}, l...), k...)), nil))
		_, found := mpUnreachNLRIOf(t, wu.Payload())
		assert.False(t, found, "a route withdrawn with its UPDATE is no longer held")
	})

	t.Run("longer replacement gains the Extended Length bit", func(t *testing.T) {
		s := nlriTypeTestSession()
		tlv := append([]byte{0x00, 0xf0, 200}, make([]byte, 200)...)
		long2 := mupWireNLRI(3, t1stBody(2, tlv...)...)
		long4 := mupWireNLRI(3, t1stBody(4, tlv...)...)
		holdT1STGeneration(t, s, mupFam, long2)
		holdT1STGeneration(t, s, mupFam, long4)
		short4 := mupWireNLRI(3, t1stBody(4, 0x00, 0xf0)...)
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpUnreachAttrs(mupFam, append(append([]byte{}, k...), short4...)), nil))
		attrsOut := rfc8669PathAttrs(t, wu.Payload())
		_, flags, _, found := attribute.AttrFind(attrsOut, attribute.AttrMPUnreachNLRI)
		require.True(t, found)
		assert.True(t, flags.IsExtLength(), "a value over 255 octets needs a two-octet length")
		withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, append(append([]byte{}, long2...), long4...), withdrawn)
		requireDecodable(t, mupFam, withdrawn, false)
	})
}

// TestMUPHeldGenerationWithdrawalsFitTheMessageLimit is the size half of the
// held-generation rule: a held generation can be longer than the malformed NLRI it
// replaces, so the replacements must never grow a message past the limit.
//
// VALIDATES: 150 malformed T1STs in one received MP_UNREACH_NLRI, each holding a
// 230-octet generation, leave as withdraw-only bodies no larger than the session's
// 4096-octet UPDATE limit, carrying every held generation once and in order, each
// one the recognizer keeps.
// PREVENTS: an UPDATE whose attribute or message length overflows its field
// (106411 octets declaring 40871 in review round 1 of the closure).
func TestMUPHeldGenerationWithdrawalsFitTheMessageLimit(t *testing.T) {
	s := nlriTypeTestSession()
	tlv := append([]byte{0x00, 0xf0, 200}, make([]byte, 200)...)
	var held, malformed []byte
	for i := range 150 {
		last := byte(10 + i)
		long := mupWireNLRI(3, t1stBody(last, tlv...)...)
		holdT1STGeneration(t, s, mupFam, long)
		held = append(held, long...)
		malformed = append(malformed, mupWireNLRI(3, t1stBody(last, 0x00, 0xf0)...)...)
	}
	body := makeUpdateBody(nil, mpUnreachAttrsExtLen(mupFam, malformed), nil)
	wu, _, validation := applyRFC7606Body(t, s, body)

	limit := int(message.MaxMessageLength(msgtype.TypeUPDATE, false)) - message.HeaderLen
	assert.LessOrEqual(t, len(wu.Payload()), limit)
	var relayed []byte
	if inUpdate, found := mpUnreachNLRIOf(t, wu.Payload()); found {
		relayed = append(relayed, inUpdate...)
	}
	require.NotEmpty(t, validation.typedWithdrawBodies, "34500 octets of withdrawals cannot fit one 4096-octet UPDATE")
	for _, extra := range validation.typedWithdrawBodies {
		assert.LessOrEqual(t, len(extra), limit)
		nlri, found := mpUnreachNLRIOf(t, extra)
		require.True(t, found)
		relayed = append(relayed, nlri...)
	}
	assert.Equal(t, held, relayed, "every held generation once, in order")
	requireDecodable(t, mupFam, relayed, false)
}

// TestMUPHeldGenerationRefreshAllocatesNothing pins the steady path: a peer
// re-announcing the route it already announced costs the hold no allocation.
//
// VALIDATES: typedNLRIRelayEdit over an MP_REACH_NLRI that repeats the held bytes
// allocates nothing.
// PREVENTS: a closure or a copy per MUP UPDATE on the receive goroutine.
func TestMUPHeldGenerationRefreshAllocatesNothing(t *testing.T) {
	s := nlriTypeTestSession()
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)
	holdT1STGeneration(t, s, mupFam, k0)
	recognize := nlritype.Get(mupFam)
	require.NotNil(t, recognize)
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := s.typedNLRIRelayEdit(mupFam, recognize, k0, nil, false, true, false); err != nil {
			t.Fatal(err)
		}
	})
	assert.Zero(t, allocs)
}

// TestMUPHeldGenerationOutlivesAGracefulRestart covers the hold's lifetime, which
// is the lifetime of the routes it mirrors (RFC 4724 Section 4.2).
//
// VALIDATES: a generation held on one session withdraws a malformed NLRI on the
// next when that session negotiated Graceful Restart; a session without it starts
// empty; an End-of-RIB drops the generations the new session did not announce
// again and keeps those it did.
// PREVENTS: a stale route that a malformed NLRI must withdraw surviving until the
// stale purge, and a hold that grows across sessions.
func TestMUPHeldGenerationOutlivesAGracefulRestart(t *testing.T) {
	k := mupWireNLRI(3, t1stBody(2, 0x00, 0xf0)...)
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)
	l := mupWireNLRI(3, t1stBody(1, 0x00)...)
	withdrawnBy := func(s *Session) []byte {
		wu, _, _ := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, append(append([]byte{}, l...), k...)), nil))
		nlri, _ := mpUnreachNLRIOf(t, wu.Payload())
		return nlri
	}
	session := func(held *typedNLRIHeldRoutes, gracefulRestart bool) *Session {
		s := nlriTypeTestSession()
		s.typedNLRIHeld = held
		held.beginSession(gracefulRestart)
		return s
	}

	t.Run("retained across a graceful restart", func(t *testing.T) {
		var held typedNLRIHeldRoutes
		holdT1STGeneration(t, session(&held, true), mupFam, k0)
		assert.Equal(t, k0, withdrawnBy(session(&held, true)))
	})
	t.Run("dropped by a session without graceful restart", func(t *testing.T) {
		var held typedNLRIHeldRoutes
		holdT1STGeneration(t, session(&held, true), mupFam, k0)
		assert.Nil(t, withdrawnBy(session(&held, false)))
	})
	t.Run("End-of-RIB drops what the new session did not refresh", func(t *testing.T) {
		var held typedNLRIHeldRoutes
		holdT1STGeneration(t, session(&held, true), mupFam, k0)
		next := session(&held, true)
		holdT1STGeneration(t, next, mupFam, l)
		held.endOfRIB(mupFam)
		assert.Nil(t, withdrawnBy(next), "K0 was stale and the End-of-RIB removed it")
		_, kept := held.routes[mustHeldKey(t, next, l)]
		assert.True(t, kept, "L was announced again and stays")
	})
}

// mustHeldKey is the hold's key for one NLRI.
func mustHeldKey(t *testing.T, s *Session, one []byte) fwdPathKey {
	t.Helper()
	var key fwdPathKey
	require.True(t, s.typedNLRIKey(&key, mupFam, one, false, false))
	return key
}

// mpUnreachAttrsExtLen is mpUnreachAttrs with a two-octet Extended Length, for an
// NLRI section longer than 252 octets.
func mpUnreachAttrsExtLen(fam family.Family, nlri []byte) []byte {
	value := []byte{byte(uint16(fam.AFI) >> 8), byte(uint16(fam.AFI)), byte(fam.SAFI)}
	value = append(value, nlri...)
	return append([]byte{0x90, byte(attribute.AttrMPUnreachNLRI), byte(len(value) >> 8), byte(len(value))}, value...)
}
