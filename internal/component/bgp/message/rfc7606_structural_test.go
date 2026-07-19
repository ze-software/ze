// RFC: rfc/short/rfc7606.md — revised UPDATE error handling
// Overview: rfc7606.go — RFC 7606 UPDATE validation and action selection
//
// Structural RFC 7606 requirements that rfc7606_test.go leaves uncovered: the §5.3
// syntax boundaries, the §4 length and zero-length rules, the §2 action-strength
// ordering, the §2 attribute-discard prohibition, and the §3.g first-occurrence rule.
//
// Every case here asserts an EXACT action (require.Equal), never a severity floor.
// Severity orders None < AttributeDiscard < TreatAsWithdraw < SessionReset, so a floor
// of TreatAsWithdraw is also satisfied by SessionReset -- the over-reaction RFC 7606
// exists to eliminate. A floor cannot fail when the implementation stops complying, so
// it is not evidence. Each buffer therefore isolates ONE error and pins the attribute
// code where the API exposes it.

package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Well-formed attributes reused as filler so that the case under test is the only error
// in the buffer. Keeping these aligned (declared length == bytes present) is what stops a
// structural cascade from confounding the assertion.
var (
	sOrigin  = []byte{0x40, 0x01, 0x01, 0x00}                   // ORIGIN = IGP
	sASPath  = []byte{0x40, 0x02, 0x00}                         // AS_PATH (empty, valid per §4)
	sNextHop = []byte{0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01} // NEXT_HOP = 192.0.2.1
)

// sJoin concatenates path-attribute fragments into one blob.
func sJoin(parts ...[]byte) []byte {
	var buf []byte
	for _, p := range parts {
		buf = append(buf, p...)
	}
	return buf
}

// =============================================================================
// RFC 7606 Section 5.3 — NLRI / Withdrawn Routes syntax boundaries
// =============================================================================

// TestRFC7606NLRIMaxPrefixLengthAccepted pins the upper boundary of the §5.3 length rule.
//
// VALIDATES: an IPv4 NLRI whose largest prefix length is exactly 32 is syntactically correct.
// PREVENTS: an off-by-one that rejects /32 and session-resets every host route.
//
// §5.3 makes the field incorrect when a length is "greater than 32". 32 itself is the
// largest conforming value, so this is the exact case the rule must NOT catch. It pairs
// TestRFC7606NLRIPrefixLengthTooLongIPv4, which drives length 33 through the same call.
//
// RFC requirement: RFC7606-5.3-1 positive — prefix length 32 is not greater than 32, so the field is syntactically correct.
func TestRFC7606NLRIMaxPrefixLengthAccepted(t *testing.T) {
	// 0.0.0.0/0 (the shortest legal NLRI) then 10.0.0.1/32 (the longest).
	nlri := []byte{
		0,               // 0.0.0.0/0 — no prefix octets
		32, 10, 0, 0, 1, // 10.0.0.1/32 — the family maximum
	}

	require.Nil(t, ValidateNLRISyntax(nlri, false),
		"prefix length 32 is the family maximum, not a violation of the 'greater than 32' rule")
}

// TestRFC7606NLRILastPrefixExactlyFitsAccepted pins the boundary of the §5.3 overrun rule.
//
// VALIDATES: an NLRI field whose last prefix consumes exactly the remaining octets is correct.
// PREVENTS: an off-by-one that reads the exact-fit case as an overrun and session-resets it.
//
// §5.3 makes the field incorrect when "the length of the last NLRI found exceeds the
// unconsumed data remaining in the field". Exactly-equal does not exceed, so this is the
// conforming edge. It pairs TestEnforceRFC7606_InvalidWithdrawnNLRI (the negative).
//
// RFC requirement: RFC7606-5.3-2 positive — the last NLRI consumes exactly the data remaining, so nothing overruns.
func TestRFC7606NLRILastPrefixExactlyFitsAccepted(t *testing.T) {
	// /24 needs ceil(24/8) = 3 octets and exactly 3 remain after the length octet.
	nlri := []byte{24, 192, 0, 2}

	require.Nil(t, ValidateNLRISyntax(nlri, false),
		"the last NLRI consumes the field exactly; equal is not 'exceeds'")
}

// TestRFC7606MPNLRILengthConsistentWithAFIAccepted pins the conforming side of §5.3's
// AFI/SAFI consistency rule.
//
// VALIDATES: NLRI lengths that ARE consistent with the family are not flagged.
// PREVENTS: rejecting a legitimate /128 host route.
//
// Driven through ValidateNLRISyntax with isIPv6=true. This is now the same code the
// production path runs on MP attribute NLRI (validateMPReachAttr -> validateMPNLRISyntax
// -> ValidateNLRISyntax), so this is a focused unit test of that shared helper; the
// end-to-end §5.3-3 pair drives it through ValidateUpdateRFC7606
// (TestRFC7606MPReachWellFormedAccepted positive, TestRFC7606NLRIPrefixLengthTooLongIPv6
// negative).
//
// RFC requirement: RFC7606-5.3-3 positive — lengths 64 and 128 are consistent with IPv6, so the field is correct.
func TestRFC7606MPNLRILengthConsistentWithAFIAccepted(t *testing.T) {
	nlri := []byte{
		64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 2001:db8::/64
		128, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, // 2001:db8::1/128 — the family maximum
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	}

	require.Nil(t, ValidateNLRISyntax(nlri, true),
		"128 is the IPv6 maximum, so it is consistent with the AFI, not inconsistent with it")
}

// TestRFC7606MPReachNLRIConsistentWithAFISAFIAccepted covers the conforming side of §5.3's
// "NLRI parsed within the attribute" rules.
//
// VALIDATES: an MP_REACH whose NLRI parses cleanly within the attribute is accepted.
// PREVENTS: rejecting a well-formed multiprotocol announcement.
//
// Untagged: this is an IPv4 well-formed guard. validateMPReachAttr (rfc7606.go) now parses
// the NLRI carried inside the attribute (validateMPNLRISyntax -> ValidateNLRISyntax), so a
// last NLRI that overruns, or an IPv4 length greater than 32, is a session reset per §3(j).
// The §5.3-3 and §5.3-4 coverage tags live on the IPv6 production-path pair in
// rfc7606_withdraw_test.go (TestRFC7606MPReachWellFormedAccepted positive,
// TestRFC7606MPReachNLRIOverrunsAttribute / TestRFC7606NLRIPrefixLengthTooLongIPv6
// negatives). This case remains as a guard that the new NLRI check does not reject a
// well-formed IPv4 MP_REACH, whose /8 fits the attribute exactly.
//
// RFC requirement: RFC8654-3-1 positive -- extended-message peers require RFC 7606 UPDATE error
// handling; ze runs every UPDATE through ValidateUpdateRFC7606, and a well-formed UPDATE is accepted
// with action None (rfc7606.go ValidateUpdateRFC7606).
func TestRFC7606MPReachNLRIConsistentWithAFISAFIAccepted(t *testing.T) {
	// MP_REACH: AFI=1 SAFI=1 NHLen=4 NH=192.0.2.1 Reserved=0, then 10.0.0.0/8.
	// The /8 needs 1 prefix octet and exactly 1 remains, so the NLRI ends flush with the
	// attribute boundary.
	mp := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x01,                   // SAFI = 1 (unicast)
		0x04,                   // next-hop length 4
		0xc0, 0x00, 0x02, 0x01, // 192.0.2.1
		0x00,       // Reserved
		0x08, 0x0a, // NLRI 10.0.0.0/8 — consistent with IPv4, exact fit
	}
	pathAttrs := sJoin(sOrigin, sASPath, append([]byte{0x80, 0x0e, byte(len(mp))}, mp...))

	result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"a well-formed MP_REACH must not be flagged: %s", result.Description)
}

// TestRFC7606MPMinimumLengthsAccepted pins the exact lower boundaries of §5.3's
// minimum-length rule.
//
// VALIDATES: MP_REACH at exactly 5 and MP_UNREACH at exactly 3 are accepted.
// PREVENTS: an off-by-one turning the smallest legal MP attribute into a session reset.
//
// §5.3 makes the attribute incorrect when "the MP_UNREACH_NLRI attribute is less than 3"
// or "the MP_REACH_NLRI attribute is less than 5". 3 and 5 are the smallest conforming
// values, so these are the exact cases the rule must NOT catch. They pair
// test/plugin/rfc7606-reset.ci, which drives an MP_REACH shorter than 5.
//
// RFC requirement: RFC7606-5.3-6 positive — MP_REACH length 5 and MP_UNREACH length 3 are the minimums, not below them.
func TestRFC7606MPMinimumLengthsAccepted(t *testing.T) {
	t.Run("MP_REACH length exactly 5", func(t *testing.T) {
		// The shortest MP_REACH RFC 4760 Section 3 can encode: AFI(2) SAFI(1)
		// NextHopLen(1)=0 Reserved(1), no next hop and no NLRI. AFI=1/SAFI=133
		// (FlowSpec) is used because RFC 8955 Section 4.3 gives it no next hop, so a
		// zero-length one is well-formed rather than a §7.11 error -- which keeps the
		// §5.3 length rule the only thing this case exercises.
		mp := []byte{0x00, 0x01, 0x85, 0x00, 0x00}
		pathAttrs := sJoin(sOrigin, sASPath, append([]byte{0x80, 0x0e, byte(len(mp))}, mp...))

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action,
			"length 5 is the minimum, not below it: %s", result.Description)
	})

	t.Run("MP_UNREACH length exactly 3", func(t *testing.T) {
		// AFI(2) SAFI(1) and no NLRI: the shortest MP_UNREACH RFC 4760 Section 4 allows.
		mp := []byte{0x00, 0x01, 0x01}
		pathAttrs := sJoin(sOrigin, sASPath, append([]byte{0x80, 0x0f, byte(len(mp))}, mp...))

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action,
			"length 3 is the minimum, not below it: %s", result.Description)
	})
}

// TestRFC7606MPAttributeFlagsPerRFC4760Accepted covers the conforming side of §5.3's
// attribute-flags rule.
//
// VALIDATES: MP_REACH and MP_UNREACH flagged Optional non-transitive (0x80) are accepted.
// PREVENTS: rejecting the flags RFC 4760 actually mandates.
//
// Untagged guard: validateAttributeFlags (rfc7606.go) now special-cases codes 14 and 15
// per RFC 4760 (Optional set, Transitive clear); flags 0xC0 or 0x40 are inconsistent and
// §3(j) escalates them to session reset, strictly stronger than the generic §3.c
// treat-as-withdraw. The §5.3-5 coverage tags live on the production-path pair in
// rfc7606_withdraw_test.go (TestRFC7606MPReachWellFormedAccepted positive,
// TestRFC7606MPReachFlagsInconsistentWithRFC4760 negative). This case remains as a guard
// that the new flag check accepts the flags RFC 4760 actually mandates (0x80).
func TestRFC7606MPAttributeFlagsPerRFC4760Accepted(t *testing.T) {
	mpReach := []byte{0x00, 0x01, 0x01, 0x04, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x08, 0x0a}
	mpUnreach := []byte{0x00, 0x01, 0x01, 0x08, 0x0a}

	t.Run("MP_REACH optional non-transitive", func(t *testing.T) {
		pathAttrs := sJoin(sOrigin, sASPath,
			append([]byte{0x80, 0x0e, byte(len(mpReach))}, mpReach...))

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action,
			"0x80 is exactly what RFC 4760 specifies: %s", result.Description)
	})

	t.Run("MP_UNREACH optional non-transitive", func(t *testing.T) {
		pathAttrs := sJoin(sOrigin, sASPath,
			append([]byte{0x80, 0x0f, byte(len(mpUnreach))}, mpUnreach...))

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionNone, result.Action,
			"0x80 is exactly what RFC 4760 specifies: %s", result.Description)
	})
}

// =============================================================================
// RFC 7606 Section 4 — attribute length conflicts
// =============================================================================

// TestRFC7606AttributeLengthConflictTreatAsWithdraw pins §4's length-conflict action.
//
// VALIDATES: an attribute whose declared length exceeds the data remaining in the
// Total Attribute Length is EXACTLY treat-as-withdraw.
// PREVENTS: escalating a recoverable length conflict to a session reset, which is the
// over-reaction §4 exists to replace.
//
// Isolation: ORIGIN, AS_PATH and NEXT_HOP are all well-formed and aligned, so the only
// error is the COMMUNITY length conflict, and AttrCode pins it to COMMUNITY. The declared
// length is a multiple of 4, so §7.8's length rule cannot be what fires. The existing
// extlen/COMMUNITY_inflate_to_8 case covers the same shape with a severity floor, which
// cannot fail on escalation; this asserts the action exactly.
//
// RFC requirement: RFC7606-4-1 negative — an attribute length exceeding the data remaining is treat-as-withdraw, not a session reset.
// RFC requirement: RFC8654-3-1 negative -- the same RFC 7606 machinery treats a malformed UPDATE
// attribute as withdraw rather than resetting the session, the revised error handling 8654 requires
// of extended-message peers (rfc7606.go ValidateUpdateRFC7606).
func TestRFC7606AttributeLengthConflictTreatAsWithdraw(t *testing.T) {
	pathAttrs := sJoin(sOrigin, sASPath, sNextHop,
		// COMMUNITY declares 8 octets but only 4 follow: §4's "attribute length ...
		// exceeds the amount of data" conflict. 8 is a legal COMMUNITY length, so §7.8
		// cannot be the rule under test.
		[]byte{0xc0, 0x08, 0x08, 0xfd, 0xe8, 0x00, 0x64},
	)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action,
		"§4 names treat-as-withdraw for a length conflict, not session reset: %s", result.Description)
	require.Equal(t, uint8(8), result.AttrCode, "the conflict must be attributed to COMMUNITY")
	require.Contains(t, result.Description, "Section 4")
}

// TestRFC7606ZeroLengthAttributeMalformed pins §4's zero-length rule.
//
// VALIDATES: a zero-length attribute that is neither AS_PATH nor ATOMIC_AGGREGATE is
// EXACTLY treat-as-withdraw.
// PREVENTS: silently accepting an attribute that carries no value.
//
// Isolation: a zero-length attribute consumes no value octets, so appending it last
// cannot shift any boundary or cascade into a structural error. Nothing else in the
// buffer is malformed, and AttrCode pins the result to the attribute under test. This is
// what the existing EBGP/COMMUNITY_deflate_to_0 case cannot claim: it deflates a real
// 4-octet COMMUNITY, leaving 4 stray octets to be misparsed as a phantom attribute.
//
// Note: Ze has no single general §4 zero-length check; each per-attribute validator
// rejects zero as part of its own length rule (validateCommunityAttr rfc7606.go:495,
// validateMEDAttr rfc7606.go:429). The observable action is the one §4 mandates, which is
// what this pins.
//
// RFC requirement: RFC7606-4-2 negative — a zero-length attribute other than AS_PATH or ATOMIC_AGGREGATE is malformed.
func TestRFC7606ZeroLengthAttributeMalformed(t *testing.T) {
	cases := []struct {
		name     string
		attr     []byte
		wantCode uint8
	}{
		// COMMUNITY is optional transitive (0xC0); §7.8 gives it a non-zero length.
		{"COMMUNITY", []byte{0xc0, 0x08, 0x00}, 8},
		// MULTI_EXIT_DISC is optional non-transitive (0x80); §7.4 fixes its length at 4.
		{"MULTI_EXIT_DISC", []byte{0x80, 0x04, 0x00}, 4},
		// LARGE_COMMUNITY (RFC 8092), to show the rule is not COMMUNITY-specific.
		{"LARGE_COMMUNITY", []byte{0xc0, 0x20, 0x00}, 32},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pathAttrs := sJoin(sOrigin, sASPath, sNextHop, tc.attr)

			result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
			require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action,
				"a zero length on %s is a syntax error: %s", tc.name, result.Description)
			require.Equal(t, tc.wantCode, result.AttrCode,
				"the error must be attributed to %s, not to a cascade", tc.name)
		})
	}
}

// =============================================================================
// RFC 7606 Section 2 — strength ordering across multiple errors
// =============================================================================

// TestRFC7606StrongestActionWins pins §2's ordering when errors disagree.
//
// VALIDATES: session reset beats treat-as-withdraw, which beats attribute discard.
// PREVENTS: an UPDATE with a serious error being handled by the weakest approach found.
//
// AFI/SAFI disable does not appear as a distinct rung: Ze has no such action
// (RFC7606Action is none < attribute-discard < treat-as-withdraw < session-reset,
// rfc7606.go:21-30). §7.11 requires one of session reset or AFI/SAFI disable, and §2
// (captured as RFC7606-2-4 [MAY]) leaves the choice to the implementation; Ze always
// takes session reset, which is the stronger of the two.
//
// RFC requirement: RFC7606-2-6 negative — where errors dictate different approaches, the strongest one is used.
func TestRFC7606StrongestActionWins(t *testing.T) {
	t.Run("treat-as-withdraw beats attribute discard", func(t *testing.T) {
		pathAttrs := sJoin(sOrigin, sASPath, sNextHop,
			// ATOMIC_AGGREGATE length 1 (§7.6: attribute discard) — the weaker error.
			[]byte{0x40, 0x06, 0x01, 0x00},
			// COMMUNITY length 5 (§7.8: treat-as-withdraw) — the stronger error.
			[]byte{0xc0, 0x08, 0x05, 0x00, 0x01, 0x00, 0x02, 0x03},
		)

		result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
		require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action,
			"the stronger of {attribute discard, treat-as-withdraw} must win: %s", result.Description)
		require.Equal(t, uint8(8), result.AttrCode,
			"the reported attribute must be the one that produced the strongest error")
	})

	t.Run("session reset beats treat-as-withdraw", func(t *testing.T) {
		// MP_REACH for IPv6 with a 5-octet next hop: §7.11 makes the NLRI unlocatable,
		// so §3(j) requires session reset.
		mp := []byte{
			0x00, 0x02, // AFI = 2 (IPv6)
			0x01,                         // SAFI = 1 (unicast)
			0x05,                         // next-hop length 5 — legal for no IPv6 SAFI
			0x01, 0x02, 0x03, 0x04, 0x05, // the 5 octets
			0x00, // Reserved
		}
		// The COMMUNITY error is placed FIRST so that treat-as-withdraw is already the
		// recorded strongest action by the time the session-reset error is found. Without
		// that ordering the case would prove nothing about which action wins.
		pathAttrs := sJoin(sOrigin, sASPath,
			[]byte{0xc0, 0x08, 0x05, 0x00, 0x01, 0x00, 0x02, 0x03}, // COMMUNITY → treat-as-withdraw
			append([]byte{0x80, 0x0e, byte(len(mp))}, mp...),       // MP_REACH → session reset
		)

		result := ValidateUpdateRFC7606(pathAttrs, false, false, false)
		require.Equal(t, RFC7606ActionSessionReset, result.Action,
			"the stronger of {treat-as-withdraw, session reset} must win: %s", result.Description)
		require.Equal(t, uint8(14), result.AttrCode,
			"the reported attribute must be the one that produced the strongest error")
	})
}

// TestRFC7606EqualStrengthErrorsDoNotEscalate pins the conforming side of §2's ordering.
//
// VALIDATES: two attribute-discard errors yield EXACTLY attribute discard.
// PREVENTS: the strength selection over-reacting — reaching for a stronger action than
// any error present actually calls for, which is the whole failure mode RFC 7606 targets.
//
// This is the positive that a floor assertion can never provide: "at least attribute
// discard" is satisfied by a session reset, so only require.Equal can fail when the
// selection starts escalating. Both errors are aligned, so no structural error confounds
// the result, and both DiscardEntries are checked so neither error is silently dropped.
//
// RFC requirement: RFC7606-2-6 positive — the strongest of two equal approaches is that same approach, with no escalation.
func TestRFC7606EqualStrengthErrorsDoNotEscalate(t *testing.T) {
	pathAttrs := sJoin(sOrigin, sASPath, sNextHop,
		// ATOMIC_AGGREGATE length 1 (§7.6: attribute discard).
		[]byte{0x40, 0x06, 0x01, 0x00},
		// AGGREGATOR length 8 with asn4=false, which expects 6 (§7.7: attribute discard).
		[]byte{0xc0, 0x07, 0x08, 0x00, 0x00, 0xfd, 0xe9, 0xc0, 0x00, 0x02, 0x01},
	)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionAttributeDiscard, result.Action,
		"neither error calls for more than attribute discard, so nothing may escalate: %s",
		result.Description)

	codes := make(map[uint8]bool, len(result.DiscardEntries))
	for _, e := range result.DiscardEntries {
		codes[e.Code] = true
	}
	require.True(t, codes[6], "ATOMIC_AGGREGATE must be among the discarded attributes")
	require.True(t, codes[7], "AGGREGATOR must be among the discarded attributes")
}

// =============================================================================
// RFC 7606 Section 2 — attribute discard is forbidden for route-selection attributes
// =============================================================================

// TestRFC7606RouteSelectionAttributesNeverDiscarded pins §2's MUST NOT.
//
// VALIDATES: for EVERY attribute that feeds route selection or installation, a
// malformation yields something other than attribute discard.
// PREVENTS: a malformed ORIGIN, AS_PATH, NEXT_HOP, MED or LOCAL_PREF being silently
// dropped while the route is still installed — a route selected on attributes the peer
// never actually sent.
//
// Enumeration is the only way to make this MUST NOT falsifiable. A single example would
// show one attribute is not discarded; the prohibition is over the whole class, so the
// test walks the class. §2 names the criterion ("attributes that affect route selection
// or installation") rather than a list, and §3.e supplies the members RFC 7606 considers:
// ORIGIN, AS_PATH, NEXT_HOP, MULTI_EXIT_DISC and LOCAL_PREF.
//
// LOCAL_PREF is driven on an iBGP session deliberately. §7.5 mandates attribute discard
// for a LOCAL_PREF arriving from an EXTERNAL neighbor, but that is an out-of-context
// attribute, not a malformed one. Only iBGP isolates the malformation this rule is about.
//
// The assertion is NotEqual rather than an exact action because the prohibition itself is
// negative: §2 forbids one action without naming which of the remaining three applies.
// Asserting treat-as-withdraw here would be testing §3.e (already covered), not this rule.
//
// RFC requirement: RFC7606-2-3 negative — no route-selection attribute's malformation is handled by attribute discard.
func TestRFC7606RouteSelectionAttributesNeverDiscarded(t *testing.T) {
	cases := []struct {
		name      string
		pathAttrs []byte
		isIBGP    bool
		code      uint8
	}{
		{
			// ORIGIN length 2 (§7.1 requires 1).
			"ORIGIN",
			sJoin([]byte{0x40, 0x01, 0x02, 0x00, 0x00}, sASPath, sNextHop),
			false, 1,
		},
		{
			// AS_PATH with segment type 5 (§7.2 recognizes only 1-4).
			"AS_PATH",
			sJoin(sOrigin, []byte{0x40, 0x02, 0x04, 0x05, 0x01, 0xfd, 0xe9}, sNextHop),
			false, 2,
		},
		{
			// NEXT_HOP length 8 (§7.3 requires 4); padded so the boundary still aligns.
			"NEXT_HOP",
			sJoin(sOrigin, sASPath,
				[]byte{0x40, 0x03, 0x08, 0xc0, 0x00, 0x02, 0x01, 0xff, 0xff, 0xff, 0xff}),
			false, 3,
		},
		{
			// MULTI_EXIT_DISC length 8 (§7.4 requires 4); padded so the boundary aligns.
			"MULTI_EXIT_DISC",
			sJoin(sOrigin, sASPath, sNextHop,
				[]byte{0x80, 0x04, 0x08, 0x00, 0x00, 0x00, 0x64, 0xff, 0xff, 0xff, 0xff}),
			false, 4,
		},
		{
			// LOCAL_PREF length 3 (§7.5 requires 4) on an iBGP session, so the eBGP
			// discard rule of §7.5 cannot be what fires.
			"LOCAL_PREF",
			sJoin(sOrigin, sASPath, sNextHop,
				[]byte{0x40, 0x05, 0x03, 0x00, 0x00, 0x64}),
			true, 5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidateUpdateRFC7606(tc.pathAttrs, true, tc.isIBGP, false)

			require.NotEqual(t, RFC7606ActionAttributeDiscard, result.Action,
				"%s affects route selection, so §2 forbids handling its malformation by "+
					"attribute discard: %s", tc.name, result.Description)
			require.NotEqual(t, RFC7606ActionNone, result.Action,
				"the malformation must be detected at all, or the rule above is vacuous")
			require.Equal(t, tc.code, result.AttrCode,
				"the error must come from %s, not from a cascade elsewhere in the buffer", tc.name)
			require.Empty(t, result.DiscardEntries,
				"%s must not appear on the discard list under any action", tc.name)
		})
	}
}

// TestRFC7606NonRouteSelectionAttributesAreDiscarded pins the conforming use of the
// mechanism §2 restricts.
//
// VALIDATES: attributes that do NOT affect route selection ARE handled by attribute discard.
// PREVENTS: the prohibition being satisfied trivially by never discarding anything.
//
// Without this, TestRFC7606RouteSelectionAttributesNeverDiscarded would pass on an
// implementation that had deleted attribute discard entirely. §3.f names the two
// attributes RFC 7606 considers safe to discard: ATOMIC_AGGREGATE and AGGREGATOR.
//
// RFC requirement: RFC7606-2-3 positive — attribute discard is used for attributes that do not affect route selection.
func TestRFC7606NonRouteSelectionAttributesAreDiscarded(t *testing.T) {
	cases := []struct {
		name string
		attr []byte
		code uint8
	}{
		// ATOMIC_AGGREGATE length 1 (§7.6 requires 0).
		{"ATOMIC_AGGREGATE", []byte{0x40, 0x06, 0x01, 0x00}, 6},
		// AGGREGATOR length 8 with asn4=false, which expects 6 (§7.7).
		{"AGGREGATOR", []byte{0xc0, 0x07, 0x08, 0x00, 0x00, 0xfd, 0xe9, 0xc0, 0x00, 0x02, 0x01}, 7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pathAttrs := sJoin(sOrigin, sASPath, sNextHop, tc.attr)

			result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
			require.Equal(t, RFC7606ActionAttributeDiscard, result.Action,
				"%s does not affect route selection, so discard is permitted: %s",
				tc.name, result.Description)
			require.Equal(t, tc.code, result.AttrCode)
			require.Len(t, result.DiscardEntries, 1,
				"%s must be recorded for removal from the wire", tc.name)
			require.Equal(t, tc.code, result.DiscardEntries[0].Code)
		})
	}
}

// =============================================================================
// RFC 7606 Section 3.g — duplicate non-MP attributes: keep the FIRST
// =============================================================================

// TestRFC7606DuplicateAttributeFirstOccurrenceWins pins WHICH copy of a duplicated
// attribute survives.
//
// VALIDATES: the FIRST occurrence is the one that is used; later ones are discarded.
// PREVENTS: a last-wins implementation, where a peer could override an attribute it
// already sent by appending a second copy.
//
// The pair below is what makes the requirement falsifiable, and it is the reason
// TestRFC7606DuplicateOrigin and TestRFC7606DuplicateMED deliberately carry no tag: both
// of their copies are VALID, so a last-wins implementation passes them identically.
// Making exactly one copy malformed is what separates the two designs, because the
// validator only ever looks at the copy it keeps:
//
//	valid first + malformed duplicate  → none               (last-wins would report an error)
//	malformed first + valid duplicate  → treat-as-withdraw  (last-wins would report none)
//
// Producer: rfc7606.go:245 skips an already-seen code before validateAttribute runs, so
// the surviving copy is the one that gets validated. Both ORIGINs are length 1 and
// aligned, so no structural cascade can supply the action instead.
//
// RFC requirement: RFC7606-3.g-2 positive — a duplicate is discarded and the valid first occurrence stands.
func TestRFC7606DuplicateAttributeFirstOccurrenceWins(t *testing.T) {
	pathAttrs := sJoin(
		[]byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN = IGP (0) — FIRST, valid
		sASPath, sNextHop,
		[]byte{0x40, 0x01, 0x01, 0x03}, // ORIGIN = 3 — duplicate, undefined value per §7.1
	)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action,
		"the first ORIGIN is valid and the duplicate must be discarded unexamined; "+
			"reporting an error here would mean the LAST occurrence was kept: %s",
		result.Description)
}

// TestRFC7606DuplicateAttributeFirstOccurrenceIsValidated is the converse: the first copy
// is the one whose errors count.
//
// VALIDATES: a malformed FIRST occurrence is reported even when a valid duplicate follows.
// PREVENTS: a peer masking a malformed attribute by appending a well-formed copy of it.
//
// RFC requirement: RFC7606-3.g-2 negative — a later duplicate cannot repair or replace the first occurrence.
func TestRFC7606DuplicateAttributeFirstOccurrenceIsValidated(t *testing.T) {
	pathAttrs := sJoin(
		[]byte{0x40, 0x01, 0x01, 0x03}, // ORIGIN = 3 — FIRST, undefined value per §7.1
		sASPath, sNextHop,
		[]byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN = IGP (0) — duplicate, valid
	)

	result := ValidateUpdateRFC7606(pathAttrs, true, false, false)
	require.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action,
		"the malformed FIRST ORIGIN must be what is validated; reporting none here would "+
			"mean the valid LAST occurrence had replaced it: %s", result.Description)
	require.Equal(t, uint8(1), result.AttrCode)
	require.Contains(t, result.Description, "undefined value",
		"the error must be the first copy's undefined value, not a length or cascade error")
}
