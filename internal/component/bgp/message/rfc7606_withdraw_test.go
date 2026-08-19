package message

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// RFC 7606 Section 2 treat-as-withdraw: the UPDATE "MUST be handled as though all of the
// routes contained in [it] ... had been withdrawn". Skipping dispatch is NOT that: a
// prefix already in the Adj-RIB-In would stay installed and stale. SynthesizeWithdraw
// rewrites the UPDATE so the routes it announced become withdrawals.

// VALIDATES: RFC7606-2-1 — announced IPv4 NLRI is moved into Withdrawn Routes.
// PREVENTS: a malformed re-announcement leaving the previous route installed and stale.
//
// RFC requirement: RFC7606-2-1 negative — a malformed UPDATE's routes are withdrawn.
func TestSynthesizeWithdrawMovesNLRIToWithdrawn(t *testing.T) {
	// withdrawn=empty, attrs=[ORIGIN,AS_PATH,NEXT_HOP], nlri=10.0.0.0/24
	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0A, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
	}
	body := buildBody(nil, pathAttrs, []byte{24, 10, 0, 0})

	out, changed := SynthesizeWithdraw(body)
	require.True(t, changed)

	w, attrs, nlri := splitBody(t, out)
	assert.Equal(t, []byte{24, 10, 0, 0}, w, "the announced prefix must now be withdrawn")
	assert.Empty(t, nlri, "nothing may remain announced")
	assert.Empty(t, attrs, "a withdrawal carries no path attributes")
}

// VALIDATES: RFC7606-2-1 — an already-withdrawn prefix is preserved alongside the new one.
// PREVENTS: dropping the UPDATE's original withdrawals while synthesizing.
func TestSynthesizeWithdrawPreservesExistingWithdrawn(t *testing.T) {
	body := buildBody([]byte{24, 192, 168, 1}, []byte{0x40, 0x01, 0x01, 0x00}, []byte{24, 10, 0, 0})

	out, changed := SynthesizeWithdraw(body)
	require.True(t, changed)

	w, _, nlri := splitBody(t, out)
	assert.Equal(t, []byte{24, 192, 168, 1, 24, 10, 0, 0}, w,
		"original withdrawal must survive, with the announced prefix appended")
	assert.Empty(t, nlri)
}

// VALIDATES: RFC7606-2-1 — MP_REACH_NLRI becomes MP_UNREACH_NLRI for the same AFI/SAFI.
// PREVENTS: multiprotocol routes staying installed when only IPv4 NLRI is withdrawn.
//
// RFC requirement: RFC7606-2-1 negative — multiprotocol routes are withdrawn too.
func TestSynthesizeWithdrawConvertsMPReachToMPUnreach(t *testing.T) {
	// MP_REACH: AFI=2 SAFI=1 NHLen=16 NH[16] Reserved=0 NLRI(64 bits of 2001:db8::/32)
	mpReach := []byte{0x00, 0x02, 0x01, 0x10}
	mpReach = append(mpReach, make([]byte, 16)...)              // next hop
	mpReach = append(mpReach, 0x00, 32, 0x20, 0x01, 0x0d, 0xb8) // reserved + NLRI
	attrs := append([]byte{0x80, 0x0E, byte(len(mpReach))}, mpReach...)

	out, changed := SynthesizeWithdraw(buildBody(nil, attrs, nil))
	require.True(t, changed)

	_, newAttrs, _ := splitBody(t, out)
	it := newAttrIter(newAttrs)
	code, _, value, ok := it.Next()
	require.True(t, ok, "an MP_UNREACH must be present")
	assert.Equal(t, uint8(15), uint8(code), "MP_REACH (14) must become MP_UNREACH (15)")
	// MP_UNREACH value = AFI(2) SAFI(1) NLRI — no next hop, no reserved byte.
	assert.Equal(t, []byte{0x00, 0x02, 0x01, 32, 0x20, 0x01, 0x0d, 0xb8}, value)
	_, _, _, more := it.Next()
	assert.False(t, more, "no other attribute may survive")
}

// VALIDATES: RFC7606-2-1 — an existing MP_UNREACH is preserved unchanged.
func TestSynthesizeWithdrawKeepsMPUnreach(t *testing.T) {
	mpUnreach := []byte{0x00, 0x02, 0x01, 32, 0x20, 0x01, 0x0d, 0xb8}
	attrs := append([]byte{0x80, 0x0F, byte(len(mpUnreach))}, mpUnreach...)

	out, _ := SynthesizeWithdraw(buildBody(nil, attrs, nil))

	_, newAttrs, _ := splitBody(t, out)
	it := newAttrIter(newAttrs)
	code, _, value, ok := it.Next()
	require.True(t, ok)
	assert.Equal(t, uint8(15), uint8(code))
	assert.Equal(t, mpUnreach, value)
}

// VALIDATES: RFC7606-2-1 — an End-of-RIB style body with nothing to withdraw is untouched.
// PREVENTS: manufacturing a bogus withdrawal from a message that announced nothing.
func TestSynthesizeWithdrawNoRoutesIsNoOp(t *testing.T) {
	body := buildBody(nil, nil, nil)
	out, changed := SynthesizeWithdraw(body)
	assert.False(t, changed, "nothing was announced, so nothing can be withdrawn")
	assert.Equal(t, body, out)
}

// VALIDATES: RFC7606-2-1 — a structurally impossible body is refused, not mangled.
// PREVENTS: synthesizing from lengths that cannot be trusted (§3(b) session-resets those).
func TestSynthesizeWithdrawRefusesMalformedStructure(t *testing.T) {
	for _, body := range [][]byte{
		{},
		{0x00},
		{0x00, 0xFF, 0x00, 0x00}, // withdrawn length overruns
	} {
		_, changed := SynthesizeWithdraw(body)
		assert.False(t, changed, "must refuse rather than guess: %v", body)
	}
}

// --------------------------------------------------------------------------
// Boundary tests — MP_UNREACH attribute length encoding (RFC 4271 Section 4.3:
// Extended Length is required once the value exceeds 255 octets).
// --------------------------------------------------------------------------

// VALIDATES: 255-octet value uses the 1-octet length form; 256 switches to Extended.
// PREVENTS: emitting a length field that cannot represent the value it precedes.
func TestSynthesizeWithdrawMPUnreachLengthBoundary(t *testing.T) {
	cases := []struct {
		name      string
		nlriLen   int
		wantFlags byte
		wantHdr   int // flags + code + length octets
	}{
		{"last valid 1-octet length (255)", 255 - 3, 0x80, 3},
		{"first extended length (256)", 256 - 3, 0x90, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// MP_UNREACH value = AFI(2) SAFI(1) + NLRI
			value := append([]byte{0x00, 0x02, 0x01}, make([]byte, tc.nlriLen)...)
			attrs := appendMPUnreach(nil, value)
			require.Equal(t, tc.wantFlags, attrs[0], "flags must reflect the length form")
			require.Equal(t, byte(15), attrs[1])
			assert.Len(t, attrs, tc.wantHdr+len(value))

			// It must still parse back to exactly the value we put in.
			it := newAttrIter(attrs)
			_, _, got, ok := it.Next()
			require.True(t, ok)
			assert.Equal(t, value, got)
		})
	}
}

// VALIDATES: MP_REACH with a next-hop length that overruns the value is refused.
// PREVENTS: reading past the attribute while converting.
func TestMPReachToUnreachBoundary(t *testing.T) {
	// AFI(2) SAFI(1) NHLen=200 but only a few bytes follow -> nlriStart > len(value).
	_, ok := mpReachToUnreach([]byte{0x00, 0x02, 0x01, 200, 0x00})
	assert.False(t, ok, "must refuse a next-hop length that overruns the attribute")

	// Shortest legal: AFI(2) SAFI(1) NHLen=0 Reserved(1), no NLRI.
	out, ok := mpReachToUnreach([]byte{0x00, 0x02, 0x01, 0x00, 0x00})
	require.True(t, ok)
	assert.Equal(t, []byte{0x00, 0x02, 0x01}, out)

	// One octet short of legal.
	_, ok = mpReachToUnreach([]byte{0x00, 0x02, 0x01})
	assert.False(t, ok)
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

func newAttrIter(attrs []byte) attribute.AttrIterator {
	return attribute.NewAttrIterator(attrs)
}

// buildBody assembles an UPDATE body from its three sections.
func buildBody(withdrawn, pathAttrs, nlri []byte) []byte {
	body := make([]byte, 2+len(withdrawn)+2+len(pathAttrs)+len(nlri))
	binary.BigEndian.PutUint16(body[0:2], uint16(len(withdrawn)))
	n := 2 + copy(body[2:], withdrawn)
	binary.BigEndian.PutUint16(body[n:n+2], uint16(len(pathAttrs)))
	n += 2
	n += copy(body[n:], pathAttrs)
	copy(body[n:], nlri)
	return body
}

// splitBody is the inverse of buildBody, so a test asserts against parsed sections rather
// than hand-counted offsets.
func splitBody(t *testing.T, body []byte) (withdrawn, pathAttrs, nlri []byte) {
	t.Helper()
	require.GreaterOrEqual(t, len(body), 4)
	wLen := int(binary.BigEndian.Uint16(body[0:2]))
	require.LessOrEqual(t, 2+wLen+2, len(body))
	withdrawn = body[2 : 2+wLen]
	aLen := int(binary.BigEndian.Uint16(body[2+wLen : 2+wLen+2]))
	start := 2 + wLen + 2
	require.LessOrEqual(t, start+aLen, len(body))
	pathAttrs = body[start : start+aLen]
	nlri = body[start+aLen:]
	return withdrawn, pathAttrs, nlri
}

// =============================================================================
// RFC 7606 Section 5.3 — the MP attribute criteria that had no test.
//
// §5.3 lists four ways an MP_REACH_NLRI/MP_UNREACH_NLRI is "incorrect". §3(j) then
// mandates SESSION RESET (or AFI/SAFI disable) for an MP attribute that cannot be parsed,
// because treat-as-withdraw is only available once "the entire ... MP_REACH_NLRI and
// MP_UNREACH_NLRI attributes [have been] successfully parsed".
//
// These assert what the RFC requires. If Ze does something weaker, that is a divergence to
// report, not an assertion to soften.
// =============================================================================

// VALIDATES: an MP_REACH whose last NLRI overruns the attribute is incorrect (§5.3),
// which §3(j) escalates to session reset.
// PREVENTS: trusting NLRI boundaries that were never successfully parsed.
//
// Isolation: AFI=2/SAFI=1 with a valid 16-octet next hop, so §7.11 passes and ORIGIN +
// AS_PATH are present, so §3.d passes. The ONLY defect is the overrunning NLRI. The paired
// positive (TestRFC7606MPReachWellFormedAccepted) uses the same shape with an NLRI that
// fits, so this test fails on the overrun and not on a neighboring rule.
//
// RFC requirement: RFC7606-5.3-4 negative — the last NLRI in the attribute overruns it.
func TestRFC7606MPReachNLRIOverrunsAttribute(t *testing.T) {
	origin := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	aspath := []byte{0x40, 0x02, 0x00}       // AS_PATH = empty
	// MP_REACH: AFI=2 SAFI=1 NHLen=16 NextHop[16] Reserved=0, then an IPv6 NLRI claiming
	// /128 (16 octets) with only 2 octets present -- the last NLRI overruns the attribute.
	value := []byte{0x00, 0x02, 0x01, 0x10}
	value = append(value, make([]byte, 16)...)
	value = append(value, 0x00, 128, 0x20, 0x01)
	attrs := append(append(append([]byte{}, origin...), aspath...),
		append([]byte{0x80, 0x0E, byte(len(value))}, value...)...)

	result := ValidateUpdateRFC7606(attrs, false, false, false)
	require.NotNil(t, result)
	require.Equal(t, RFC7606ActionSessionReset, result.Action,
		"an unparseable MP NLRI cannot be treated as withdraw: §3(j) requires session reset")
	require.Contains(t, result.Description, "overrun",
		"must fail on the NLRI overrun, not a neighboring rule")
}

// VALIDATES: an MP_REACH whose flags contradict RFC 4760 is incorrect (§5.3), which
// §3(j) escalates to session reset — STRONGER than the generic §3.c flag-conflict
// treat-as-withdraw.
// PREVENTS: an MP attribute flag error being downgraded to treat-as-withdraw, which would
// trust NLRI boundaries read from an attribute whose framing is already in doubt.
//
// Isolation: everything except the flags is well-formed — valid 16-octet next hop, a /32
// NLRI that fits, ORIGIN + AS_PATH present. Only the 0x40 flag byte (well-known/transitive
// instead of the Optional non-transitive RFC 4760 mandates) differs from the paired
// positive (TestRFC7606MPReachWellFormedAccepted), so flipping it back to 0x80 makes the
// UPDATE pass — proving this fails on the flags and nothing else.
//
// RFC requirement: RFC7606-5.3-5 negative — MP attribute flags inconsistent with RFC 4760.
func TestRFC7606MPReachFlagsInconsistentWithRFC4760(t *testing.T) {
	origin := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	aspath := []byte{0x40, 0x02, 0x00}       // AS_PATH = empty
	// AFI=2 SAFI=1 NHLen=16 NextHop[16] Reserved=0, then a well-formed /32 NLRI. The only
	// defect is the attribute flags below.
	value := []byte{0x00, 0x02, 0x01, 0x10}
	value = append(value, make([]byte, 16)...)
	value = append(value, 0x00, 32, 0x20, 0x01, 0x0d, 0xb8)
	// RFC 4760: MP_REACH_NLRI is Optional (0x80). Well-known/transitive (0x40) contradicts it.
	attrs := append(append(append([]byte{}, origin...), aspath...),
		append([]byte{0x40, 0x0E, byte(len(value))}, value...)...)

	result := ValidateUpdateRFC7606(attrs, false, false, false)
	require.NotNil(t, result)
	require.Equal(t, RFC7606ActionSessionReset, result.Action,
		"§5.3 makes inconsistent MP flags 'incorrect', and §3(j) escalates that to session "+
			"reset — the generic §3.c treat-as-withdraw is too weak here")
	require.Contains(t, result.Description, "RFC 4760",
		"must fail on the flag inconsistency, not a neighboring rule")
}

// VALIDATES: a well-formed MP_REACH is accepted — the §5.3 criteria reject only what is
// actually incorrect. It is the shared conforming case for all three MP NLRI criteria: a
// length consistent with the AFI/SAFI (5.3-3), a last NLRI that fits (5.3-4), and RFC
// 4760-consistent flags (5.3-5).
// PREVENTS: a validator that rejects everything, which a negative-only test cannot detect.
//
// RFC requirement: RFC7606-5.3-3 positive — an NLRI length consistent with the AFI/SAFI is accepted.
// RFC requirement: RFC7606-5.3-4 positive — NLRI that fits the attribute is accepted.
// RFC requirement: RFC7606-5.3-5 positive — RFC 4760-consistent flags are accepted.
func TestRFC7606MPReachWellFormedAccepted(t *testing.T) {
	// Optional flag (0x80) per RFC 4760; AFI=2 SAFI=1 NHLen=16 NH[16] Reserved=0,
	// then a /32 prefix with exactly its 4 octets present.
	value := []byte{0x00, 0x02, 0x01, 0x10}
	value = append(value, make([]byte, 16)...)
	value = append(value, 0x00, 32, 0x20, 0x01, 0x0d, 0xb8)

	// The well-known mandatory attributes must be present or §3.d fires first and the
	// MP_REACH is never reached — the buffer has to isolate the rule under test.
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
	}
	attrs = append(attrs, 0x80, 0x0E, byte(len(value)))
	attrs = append(attrs, value...)

	result := ValidateUpdateRFC7606(attrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a well-formed MP_REACH must be accepted: %s", result.Description)
}
