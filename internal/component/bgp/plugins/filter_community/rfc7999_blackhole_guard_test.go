package filter_community

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

const blackholeLocalAS uint32 = 65000

// blackholePayload builds an UPDATE body carrying the given standard
// communities.
func blackholePayload(values ...uint32) []byte {
	if len(values) == 0 {
		return buildPayload(buildOriginAttr())
	}
	return buildPayload(append(buildOriginAttr(), buildCommunityAttr(values...)...))
}

// TestRFC7999BlackholeFieldHasReader is the test the spec names for AC-19:
// it proves wireu.CommunityPolicy.RFC7999Blackhole is no longer write-only.
//
// The proof is behavioral rather than structural, and that is deliberate.
// The guard cannot fire unless something reads the field:
// blackholePropagationGuard calls wireu.ParseCommunityPolicy and branches
// on RFC7999Blackhole, so a green bar here means a production code path
// consumed it. Deleting the assignment in wireu.parseCommunityAttr turns
// this test red.
//
// It lives beside its READER rather than in wireu's own test file, which is
// where the spec's TDD table first placed it. A test in package wireu can
// only reach the field through the parser that writes it, so it can assert
// the field is SET. Only a test at the reader can assert the field is USED,
// which is the property "no longer write-only" names.
//
// VALIDATES: AC-19 (the community-add half)
// PREVENTS: the field going back to having only test callers, which is the
// failure this spec corrects.
// RFC requirement: RFC7999-3.2-1 positive -- the receiver adds NO_EXPORT to a
// BLACKHOLE-tagged route.
func TestRFC7999BlackholeFieldHasReader(t *testing.T) {
	payload := blackholePayload(uint32(attribute.CommunityBlackhole))

	got := blackholePropagationGuard(payload, blackholeLocalAS, blackholeGuardNoExport)
	require.NotNil(t, got, "a BLACKHOLE-tagged route must gain the propagation community")

	assert.Equal(t,
		[]uint32{uint32(attribute.CommunityBlackhole), uint32(attribute.CommunityNoExport)},
		extractCommunities(got))
}

// TestBlackholeGuardNoAdvertise covers the operator's second choice.
//
// RFC 7999 Section 3.2 (RFC7999-3.2-2): "The community to prevent
// propagation SHOULD be chosen according to the operator's routing policy."
// Both values are offered and neither is forced, which is why the leaf is
// an enumeration rather than a boolean.
//
// VALIDATES: AC-19
// RFC requirement: RFC7999-3.2-2 positive -- the operator's leaf selects
// NO_ADVERTISE instead, so neither community is forced.
func TestBlackholeGuardNoAdvertise(t *testing.T) {
	payload := blackholePayload(uint32(attribute.CommunityBlackhole))

	got := blackholePropagationGuard(payload, blackholeLocalAS, blackholeGuardNoAdvertise)
	require.NotNil(t, got)

	assert.Equal(t,
		[]uint32{uint32(attribute.CommunityBlackhole), uint32(attribute.CommunityNoAdvertise)},
		extractCommunities(got))
}

// TestBlackholeGuardDisabledChangesNothing pins AC-21: with the guard off,
// a BLACKHOLE-tagged route keeps the bytes it arrived with.
//
// RFC 7999 Section 3.1 makes ignoring the community a conformant choice, so
// the disabled state is a supported configuration and not a degraded one.
//
// VALIDATES: AC-21 (spec-bcp194-1-communities)
// PREVENTS: an opt-in feature rewriting a payload on a peer that never asked for
// it, which would also cost that UPDATE its place in the fan-out dedup.
func TestBlackholeGuardDisabledChangesNothing(t *testing.T) {
	payload := blackholePayload(uint32(attribute.CommunityBlackhole))

	for _, guard := range []string{"", blackholeGuardNone, "not-a-token"} {
		assert.Nil(t, blackholePropagationGuard(payload, blackholeLocalAS, guard),
			"guard %q must add nothing", guard)
	}
}

// TestBlackholeGuardIgnoresUntaggedRoute verifies the guard is keyed on the
// BLACKHOLE community and on nothing else.
//
// VALIDATES: AC-21
// PREVENTS: the guard stamping every route once enabled, which would stop the
// local AS advertising anything to its eBGP peers.
func TestBlackholeGuardIgnoresUntaggedRoute(t *testing.T) {
	assert.Nil(t, blackholePropagationGuard(blackholePayload(), blackholeLocalAS, blackholeGuardNoExport),
		"a route with no COMMUNITY attribute at all")

	near := []uint32{
		uint32(attribute.CommunityBlackhole) - 1, // 65535:665
		uint32(attribute.CommunityBlackhole) + 1, // 65535:667
		blackholeLocalAS<<16 | 666,               // our own AS with the same low half
	}
	for _, v := range near {
		assert.Nil(t, blackholePropagationGuard(blackholePayload(v), blackholeLocalAS, blackholeGuardNoExport),
			"community %#08x is not BLACKHOLE", v)
	}
}

// TestBlackholeGuardIsIdempotent verifies a route that already carries the
// chosen community is left byte-identical, so a replay or a second ingress
// pass cannot accumulate duplicates.
//
// RFC 1997 gives the COMMUNITIES attribute set semantics, so a duplicate
// value carries no extra meaning and only costs wire bytes.
//
// VALIDATES: AC-19
// PREVENTS: an unbounded attribute growing by four octets per pass.
func TestBlackholeGuardIsIdempotent(t *testing.T) {
	payload := blackholePayload(uint32(attribute.CommunityBlackhole), uint32(attribute.CommunityNoExport))

	assert.Nil(t, blackholePropagationGuard(payload, blackholeLocalAS, blackholeGuardNoExport),
		"NO_EXPORT is already present, so there is nothing to add")

	// The other token is still added: the two communities are not
	// interchangeable.
	got := blackholePropagationGuard(payload, blackholeLocalAS, blackholeGuardNoAdvertise)
	require.NotNil(t, got)
	assert.Len(t, extractCommunities(got), 3)
}

// TestBlackholeGuardSurvivesTheScrub runs the two ingress passes in the
// order applyIngressFilter runs them, over a route from a peer that has
// both the RFC 7454 Section 11 scrub and the guard enabled.
//
// VALIDATES: AC-20 together with AC-19
// PREVENTS: the scrub deleting the very community the guard just added, which
// would make the two features silently cancel each other on any peer with
// both on.
func TestBlackholeGuardSurvivesTheScrub(t *testing.T) {
	forged := blackholeLocalAS<<16 | 99
	payload := blackholePayload(uint32(attribute.CommunityBlackhole), forged)

	fc := filterConfig{
		scrubOwnGA:           new(true),
		blackholePropagation: new(blackholeGuardNoExport),
	}
	got := applyIngressFilter(payload, nil, fc, blackholeLocalAS, 64511)
	require.NotNil(t, got)

	assert.Equal(t,
		[]uint32{uint32(attribute.CommunityBlackhole), uint32(attribute.CommunityNoExport)},
		extractCommunities(got),
		"the forged own-GA value is scrubbed, BLACKHOLE survives, and NO_EXPORT is added")
}
