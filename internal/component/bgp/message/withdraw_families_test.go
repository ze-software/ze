package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC 7606 Section 2 treat-as-withdraw for an UPDATE carrying two DIFFERENT MP families.
// The RIB reads only the FIRST MP_UNREACH_NLRI of an UPDATE (a single first-match lookup),
// and RFC 7606 Section 3.g makes more than one a session-reset shape, so a single body
// cannot carry both families -- the second would be silently dropped and its routes would
// stay stale. SynthesizeWithdrawFamilies splits the two families across two withdraw-only
// UPDATEs so both reach the RIB. SynthesizeWithdrawFamilies also filters families by a
// negotiation predicate (D-5).
//
// These pin Ze's family-split and negotiation-filter mechanics; the RFC 7606 Section 2
// treat-as-withdraw obligation itself (RFC7606-2-1) is enrolled by the reactor tests, so it
// is not re-tagged here.

// mpUnreachFamilyAt reads the AFI/SAFI (RFC 4760 Section 4: AFI(2) SAFI(1)) of the first
// MP_UNREACH_NLRI in a synthesized withdraw-only body -- exactly the family the RIB's
// first-match accessor would withdraw. It also asserts the body carries EXACTLY one
// MP_UNREACH, which is what makes the first-match read unambiguous.
func mpUnreachFamilyAt(t *testing.T, body []byte) [3]byte {
	t.Helper()
	_, attrs, _ := splitBody(t, body)
	it := newAttrIter(attrs)
	code, _, value, ok := it.Next()
	require.True(t, ok, "each synthesized body must carry an MP_UNREACH")
	require.Equal(t, uint8(15), uint8(code), "attribute must be MP_UNREACH_NLRI (15)")
	require.GreaterOrEqual(t, len(value), 3, "MP_UNREACH value must hold AFI/SAFI")
	_, _, _, more := it.Next()
	require.False(t, more,
		"exactly one MP_UNREACH per body: the RIB reads only the first, so a second family "+
			"here would never be withdrawn")
	return [3]byte{value[0], value[1], value[2]}
}

// VALIDATES: a treat-as-withdraw UPDATE carrying MP_REACH (family Y) and MP_UNREACH (family
// X, X != Y) withdraws BOTH families, each in its own UPDATE so the RIB's first-match
// MP_UNREACH read recovers each.
// PREVENTS: two families merged into one body, where the RIB reads only the first and the
// second family's routes stay installed and stale (the AC-8 defect).
func TestSynthesizeWithdrawTwoFamilies(t *testing.T) {
	// MP_REACH IPv6 unicast (AFI=2, SAFI=1): NHLen=16, Reserved, NLRI 2001:db8::/32.
	mpReach := []byte{0x00, 0x02, 0x01, 0x10}
	mpReach = append(mpReach, make([]byte, 16)...)
	mpReach = append(mpReach, 0x00, 32, 0x20, 0x01, 0x0d, 0xb8)
	// MP_UNREACH IPv4 multicast (AFI=1, SAFI=2): withdraw 10.0.0.0/8.
	mpUnreach := []byte{0x00, 0x01, 0x02, 0x08, 0x0a}

	attrs := append([]byte{0x80, 0x0E, byte(len(mpReach))}, mpReach...)
	attrs = append(attrs, append([]byte{0x80, 0x0F, byte(len(mpUnreach))}, mpUnreach...)...)
	body := buildBody(nil, attrs, nil)

	bodies := SynthesizeWithdrawFamilies(body, nil)
	require.Len(t, bodies, 2,
		"two different MP families must ride two UPDATEs (RFC 7606 3.g: one MP_UNREACH per UPDATE)")

	got := map[[3]byte]bool{}
	for _, b := range bodies {
		got[mpUnreachFamilyAt(t, b)] = true
	}
	assert.True(t, got[[3]byte{0x00, 0x02, 0x01}], "IPv6 unicast (from MP_REACH) must be withdrawn")
	assert.True(t, got[[3]byte{0x00, 0x01, 0x02}], "IPv4 multicast (from MP_UNREACH) must be withdrawn")
}

// VALIDATES: an existing IPv4 Withdrawn/NLRI field rides the primary body while a second MP
// family rides its own — the legacy field is not lost when the split happens.
// PREVENTS: the two-family split dropping the legacy IPv4 routes.
func TestSynthesizeWithdrawFamiliesPrimaryKeepsLegacyField(t *testing.T) {
	// MP_REACH IPv6 unicast + MP_UNREACH IPv4 multicast, plus an announced IPv4 NLRI that
	// must be withdrawn on the primary body.
	mpReach := []byte{0x00, 0x02, 0x01, 0x10}
	mpReach = append(mpReach, make([]byte, 16)...)
	mpReach = append(mpReach, 0x00, 32, 0x20, 0x01, 0x0d, 0xb8)
	mpUnreach := []byte{0x00, 0x01, 0x02, 0x08, 0x0a}
	attrs := append([]byte{0x80, 0x0E, byte(len(mpReach))}, mpReach...)
	attrs = append(attrs, append([]byte{0x80, 0x0F, byte(len(mpUnreach))}, mpUnreach...)...)

	body := buildBody(nil, attrs, []byte{24, 192, 0, 2}) // announce 192.0.2.0/24

	bodies := SynthesizeWithdrawFamilies(body, nil)
	require.Len(t, bodies, 2)

	// The primary body (bodies[0]) carries the legacy IPv4 field: the announced prefix is now
	// withdrawn.
	w, _, nlri := splitBody(t, bodies[0])
	assert.Equal(t, []byte{24, 192, 0, 2}, w, "announced IPv4 prefix must be withdrawn on the primary body")
	assert.Empty(t, nlri, "nothing may remain announced")
}

// VALIDATES: D-5 — a family the predicate rejects (not negotiated) is skipped, and when it
// is the only content there is nothing to withdraw (empty result).
// PREVENTS: a synthesized withdrawal for a non-negotiated family reaching the strict-mode
// family check and manufacturing a teardown the pre-synthesis drop never had (AC-9 at the
// synthesis boundary).
func TestSynthesizeWithdrawFamiliesSkipsRejectedFamily(t *testing.T) {
	// MP_REACH IPv6 unicast only.
	mpReach := []byte{0x00, 0x02, 0x01, 0x10}
	mpReach = append(mpReach, make([]byte, 16)...)
	mpReach = append(mpReach, 0x00, 32, 0x20, 0x01, 0x0d, 0xb8)
	attrs := append([]byte{0x80, 0x0E, byte(len(mpReach))}, mpReach...)
	body := buildBody(nil, attrs, nil)

	// Predicate admits IPv4 only, so the IPv6 family is rejected.
	acceptIPv4Only := func(afi uint16, _ uint8) bool { return afi == 1 }

	bodies := SynthesizeWithdrawFamilies(body, acceptIPv4Only)
	assert.Empty(t, bodies,
		"a non-negotiated MP family is skipped, leaving nothing to withdraw -> the caller drops the UPDATE")

	// The same body with an accepting predicate DOES withdraw the family: the skip is caused
	// by the predicate, not by the body being unusable.
	require.Len(t, SynthesizeWithdrawFamilies(body, func(uint16, uint8) bool { return true }), 1,
		"an accepted family must still be withdrawn (isolates the predicate as the cause)")
}
