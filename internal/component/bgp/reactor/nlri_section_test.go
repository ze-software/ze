// Design: docs/architecture/core-design.md -- NLRI sections of an UPDATE as the policy step reads them
// Related: filter_library.go -- libraryNLRIOps, the library chain's NLRI rewrite

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/core/family"
)

// legacyBody is an UPDATE whose legacy NLRI carries 10.0.0.0/24, 10.0.1.0/24
// and 192.168.1.0/24, rendered "nlri add 10.0.0.0/24 10.0.1.0/24 192.168.1.0/24".
const legacyBody = "0000 0014 40010100 40020602010000FDE9 40030401010101 180a0000 180a0001 18c0a801"

// malformedMPReachBody carries an MP_REACH_NLRI of four octets, which RFC 7606
// Section 5.3 calls incorrect ("the length of the MP_REACH_NLRI attribute is
// less than 5"): wireu's MPReach answers an error for it.
const malformedMPReachBody = "0000 000b 40010100 800E04 00020100"

// malformedMPUnreachBody carries an MP_UNREACH_NLRI of two octets, below the
// three RFC 7606 Section 5.3 requires.
const malformedMPUnreachBody = "0000 0009 40010100 800F02 0002"

// nextHopOverrunMPReachBody carries an ipv6/unicast MP_REACH_NLRI of eight
// octets whose Length of Next Hop Network Address declares 16 octets: the next
// hop and the Reserved octet run past the attribute (RFC 4760 Section 3).
const nextHopOverrunMPReachBody = "0000 000f 40010100 800E08 00020110 20010db8"

// TestNLRISectionOfMalformedMPIsError proves an MP attribute that does not
// parse is reported as an error, never as an absent section.
//
// VALIDATES: nlriSectionOf answers MPReach's and MPUnreach's error for the
// MP sections, with no family named, and still answers a nil error with
// present false for an UPDATE that carries no MP attribute.
// PREVENTS: a malformed MP_REACH_NLRI reading as "no MP section", which let a
// family strip or a dropped mp block pass with the attribute still in place.
func TestNLRISectionOfMalformedMPIsError(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		section nlriSection
	}{
		{"mp_reach shorter than 5", malformedMPReachBody, sectionMPReach},
		{"mp_unreach shorter than 3", malformedMPUnreachBody, sectionMPUnreach},
		{"mp_reach next hop past the attribute", nextHopOverrunMPReachBody, sectionMPReach},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := nlriSectionOf(testWireUpdate(nlriHex(t, tt.body)), nil, tt.section)
			require.Error(t, err, "a malformed MP attribute must not read as an absent section")
			assert.False(t, source.present)
			assert.Empty(t, source.familyName(), "an MP attribute that did not parse names no family")
		})
	}

	// An UPDATE with no MP attribute: the section is absent, with no error.
	for _, section := range []nlriSection{sectionMPReach, sectionMPUnreach} {
		source, err := nlriSectionOf(testWireUpdate(nlriHex(t, legacyBody)), nil, section)
		require.NoError(t, err)
		assert.False(t, source.present)
	}
}

// TestLibraryNLRIOpsRefusesStripOverMalformedMP proves a family strip fails
// closed over an MP attribute that does not parse.
//
// VALIDATES: libraryNLRIOps refuses with refuseMalformed when the folded sets
// strip a family and the UPDATE's MP_REACH_NLRI or MP_UNREACH_NLRI does not
// parse, since the wire cannot say which family the attribute carries.
// PREVENTS: the strip passing over an "absent" section while the attribute,
// possibly of the stripped family, stays in the UPDATE.
func TestLibraryNLRIOpsRefusesStripOverMalformedMP(t *testing.T) {
	for _, hexBody := range []string{malformedMPReachBody, malformedMPUnreachBody} {
		var folded policychain.Sets
		require.NoError(t, folded.StripFamily(family.IPv6Unicast))
		var mods filterapi.ModAccumulator
		_, refusal, ok := libraryNLRIOps(newTestScratch(t), &folded, testWireUpdate(nlriHex(t, hexBody)), &mods)
		assert.False(t, ok, "a strip over a malformed MP attribute must drop the route")
		assert.Equal(t, refuseMalformed, refusal.reason)
	}
}
