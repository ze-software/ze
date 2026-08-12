package filter_community

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// extractLargeCommunities reads every GA:LD1:LD2 triple from the
// LARGE_COMMUNITY attribute of a payload. Returns nil when absent.
func extractLargeCommunities(payload []byte) [][3]uint32 {
	_, _, dataStart, dataEnd, found := findAttribute(payload, attribute.AttrLargeCommunity)
	if !found {
		return nil
	}
	var out [][3]uint32
	for i := dataStart; i+12 <= dataEnd; i += 12 {
		out = append(out, [3]uint32{
			binary.BigEndian.Uint32(payload[i : i+4]),
			binary.BigEndian.Uint32(payload[i+4 : i+8]),
			binary.BigEndian.Uint32(payload[i+8 : i+12]),
		})
	}
	return out
}

const scrubLocalAS uint32 = 65000

// TestScrubKeepList verifies RFC 7454 Section 11 carve-out: an own-Global-
// Administrator value whose function is in the keep-list survives, and one
// whose function is not is removed.
//
// The keep-list is the whole design and the reason this is not a denylist.
// Section 11's first bullet is one obligation with a carve-out: scrub
// inbound communities carrying your own number in the high-order bits, AND
// allow only those the customers and peers use as a signaling mechanism. A
// denylist fails open for every function a neighbor invents, which is what
// the bullet exists to prevent.
//
// VALIDATES: AC-6, AC-7 (spec-bcp194-1-communities)
// PREVENTS: an unrecognized own-GA function surviving ingress, which lets a
// neighbor forge any signal the local AS's policy keys on.
func TestScrubKeepList(t *testing.T) {
	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{scrubLocalAS, 64, 64511}, // kept function
		[3]uint32{scrubLocalAS, 99, 64511}, // not in the keep-list
	)...))

	got := scrubOwnGACommunities(payload, scrubLocalAS, map[uint32]bool{64: true}, 0)
	require.NotNil(t, got, "a value outside the keep-list must be removed")

	assert.Equal(t, [][3]uint32{{scrubLocalAS, 64, 64511}}, extractLargeCommunities(got))
}

// TestScrubEmptyKeepListRemovesEveryOwnGAValue pins the fail-closed
// default: with no keep-list configured, every own-GA value is scrubbed. An
// empty keep-list must not read as "keep everything".
//
// VALIDATES: AC-7
// PREVENTS: the zero-value trap of ai/rules/evidence.md, where the absent
// configuration selects the permissive branch.
func TestScrubEmptyKeepListRemovesEveryOwnGAValue(t *testing.T) {
	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{scrubLocalAS, 64, 64511},
		[3]uint32{scrubLocalAS, 99, 64511},
	)...))

	got := scrubOwnGACommunities(payload, scrubLocalAS, nil, 0)
	require.NotNil(t, got)
	assert.Empty(t, extractLargeCommunities(got),
		"an empty keep-list keeps nothing that carries our own Global Administrator")
}

// TestScrubIgnoresForeignGA verifies RFC 7454 Section 11's second bullet:
// "Networks administrators SHOULD NOT remove other communities applied on
// received routes" (RFC7454-11-2). Only our own Global Administrator is in
// scope.
//
// VALIDATES: AC-6 (survival half), and Section 11 bullet-two obligation
// PREVENTS: the scrub deleting a customer's signaling to its own upstream,
// which is the failure the bullet names.
func TestScrubIgnoresForeignGA(t *testing.T) {
	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{64511, 99, 1}, // another AS's Global Administrator, unknown function
		[3]uint32{64512, 3, 2},  // another AS's relation tag
	)...))

	assert.Nil(t, scrubOwnGACommunities(payload, scrubLocalAS, nil, 3),
		"no value carries our Global Administrator, so nothing may be rewritten")
}

// TestWellKnownCommunitiesSurviveScrub verifies that BLACKHOLE, NO_EXPORT
// and NO_ADVERTISE all survive. RFC 1997 reserves 0xFFFF0000-0xFFFFFFFF, so
// no assignable AS can be the Global Administrator of one. RFC 7454 Section
// 11's fourth bullet (RFC7454-11-4) protects NO_EXPORT by name.
//
// VALIDATES: AC-20 (spec-bcp194-1-communities)
// PREVENTS: an operator's blackhole request being silently deleted at ingress,
// which turns a DDoS mitigation into a no-op.
func TestWellKnownCommunitiesSurviveScrub(t *testing.T) {
	wellKnown := []uint32{
		uint32(attribute.CommunityBlackhole),
		uint32(attribute.CommunityNoExport),
		uint32(attribute.CommunityNoAdvertise),
	}
	payload := buildPayload(append(buildOriginAttr(), buildCommunityAttr(wellKnown...)...))

	assert.Nil(t, scrubOwnGACommunities(payload, scrubLocalAS, nil, 0),
		"a well-known community carries no assignable Global Administrator")
	assert.Equal(t, wellKnown, extractCommunities(payload), "payload untouched")
}

// TestWellKnownSurviveScrubEvenAtReservedLocalAS drives the same three
// values with the local AS set to 65535, the reserved value whose sixteen
// bits equal the well-known prefix. The arithmetic alone would then match,
// so this pins that the protection is structural rather than incidental.
//
// VALIDATES: AC-20
// PREVENTS: a misconfigured or reserved local AS turning Section 11 scrub
// into a well-known community shredder.
func TestWellKnownSurviveScrubEvenAtReservedLocalAS(t *testing.T) {
	wellKnown := []uint32{
		uint32(attribute.CommunityBlackhole),
		uint32(attribute.CommunityNoExport),
		uint32(attribute.CommunityNoAdvertise),
	}
	payload := buildPayload(append(buildOriginAttr(), buildCommunityAttr(wellKnown...)...))

	assert.Nil(t, scrubOwnGACommunities(payload, 65535, nil, 0),
		"the reserved 0xFFFF prefix is never our Global Administrator, whatever the local AS says")
}

// TestScrubStandardCommunityOwnGA verifies the scrub reaches RFC 1997
// standard communities too: Section 11 speaks of "your number in the
// high-order bits", which is the standard community's high half as much as
// the large community's Global Administrator.
//
// VALIDATES: AC-7
// PREVENTS: a neighbor forging <ourAS>:<value> and having it survive because
// only the large-community form was scrubbed.
func TestScrubStandardCommunityOwnGA(t *testing.T) {
	kept := scrubLocalAS<<16 | 64
	forged := scrubLocalAS<<16 | 99
	foreign := uint32(64511)<<16 | 99
	payload := buildPayload(append(buildOriginAttr(), buildCommunityAttr(kept, forged, foreign)...))

	got := scrubOwnGACommunities(payload, scrubLocalAS, map[uint32]bool{64: true}, 0)
	require.NotNil(t, got)
	assert.Equal(t, []uint32{kept, foreign}, extractCommunities(got))
}

// TestScrubNeverKeepsTheRelationFunction verifies that the relation
// function is removed even when an operator has listed it in the keep-list.
// A kept relation function would let a neighbor state its own relation to
// us and have the value stored, which is the forgery AC-5 exists to
// prevent.
//
// VALIDATES: AC-5 (spec-bcp194-1-communities)
// PREVENTS: a config typo re-opening the forgery the de-forge pass closes.
func TestScrubNeverKeepsTheRelationFunction(t *testing.T) {
	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{scrubLocalAS, 3, 4}, // forged "you are my provider"
	)...))

	got := scrubOwnGACommunities(payload, scrubLocalAS, map[uint32]bool{3: true}, 3)
	require.NotNil(t, got, "the relation function is never kept, whatever the keep-list says")
	assert.Empty(t, extractLargeCommunities(got))
}

// TestScrubNoCommunityAttributes verifies the no-op path returns nil rather
// than a copy, so an UPDATE carrying no communities stays byte-identical on
// the forwarding rail.
//
// PREVENTS: an unnecessary rebuild that defeats the fan-out dedup.
func TestScrubNoCommunityAttributes(t *testing.T) {
	assert.Nil(t, scrubOwnGACommunities(buildPayload(buildOriginAttr()), scrubLocalAS, nil, 0))
}

// TestScrubGlobalAdministratorBoundaries drives the local AS across the
// edges of its range. Spec Boundary Tests rows "Global Administrator for
// the scrub" (0-4294967295) and "Route-server ASN for standard-community
// match" (0-65535 after truncation).
//
// The 65535/65536 pair is the load-bearing one: 65535 is the last value a
// standard community's high half can hold AND the reserved well-known
// prefix. 65536 is the first local AS with no standard-community form at
// all.
//
// PREVENTS: an off-by-one that lets the scrub read a four-octet AS's low sixteen
// bits as a standard-community Global Administrator, which would delete
// another AS's values, and a local AS of 0 matching the RS blacklist
// convention 0:X.
func TestScrubGlobalAdministratorBoundaries(t *testing.T) {
	tests := []struct {
		name          string
		localAS       uint32
		standardHigh  uint32
		wantStandard  bool
		wantLargeGone bool
	}{
		{"local AS 1, the first assignable value", 1, 1, true, true},
		{"local AS 65534, last before the reserved value", 65534, 65534, true, true},
		// The two widths diverge here, and only here. 0xFFFF is RFC 1997
		// well-known PREFIX, so no standard community carrying it is ever ours.
		// A large community has no such range: RFC 8092 Section 6 states the
		// attribute is not malformed when the Global Administrator holds a
		// reserved ASN. So GA 65535 is an ordinary own-GA match.
		{"local AS 65535 is never a standard-community Global Administrator", 65535, 65535, false, true},
		{"local AS 65536 has no standard-community form", 65536, 0, false, true},
		{"local AS 4294967295 has no standard-community form", 4294967295, 0, false, true},
		{"local AS 0 matches nothing: 0:X is the RS blacklist convention", 0, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			std := tt.standardHigh<<16 | 99
			attrs := append(buildOriginAttr(), buildCommunityAttr(std)...)
			attrs = append(attrs, buildLargeCommunityAttr([3]uint32{tt.localAS, 99, 1})...)
			payload := buildPayload(attrs)

			got := scrubOwnGACommunities(payload, tt.localAS, nil, 0)
			if !tt.wantStandard && !tt.wantLargeGone {
				assert.Nil(t, got, "nothing may be rewritten")
				return
			}
			require.NotNil(t, got)
			if tt.wantStandard {
				assert.Empty(t, extractCommunities(got), "own-GA standard value removed")
			} else {
				assert.Equal(t, []uint32{std}, extractCommunities(got), "standard value untouched")
			}
			if tt.wantLargeGone {
				assert.Empty(t, extractLargeCommunities(got), "own-GA large value removed")
			}
		})
	}
}

// TestScrubKeepListFunctionBoundaries drives the keep-list at the edges of
// the four-octet function field. Spec Boundary Tests row "Function number
// leaf" (0-4294967295).
//
// PREVENTS: a truncation to sixteen bits somewhere in the match, which would make
// function 65536 collide with function 0 and keep a value the operator did
// not list.
func TestScrubKeepListFunctionBoundaries(t *testing.T) {
	const maxFn = uint32(4294967295)
	payload := buildPayload(append(buildOriginAttr(), buildLargeCommunityAttr(
		[3]uint32{scrubLocalAS, 0, 1},
		[3]uint32{scrubLocalAS, 65536, 1},
		[3]uint32{scrubLocalAS, maxFn, 1},
	)...))

	got := scrubOwnGACommunities(payload, scrubLocalAS, map[uint32]bool{0: true, maxFn: true}, 0)
	require.NotNil(t, got, "function 65536 is not in the keep-list and must be removed")
	assert.Equal(t, [][3]uint32{{scrubLocalAS, 0, 1}, {scrubLocalAS, maxFn, 1}},
		extractLargeCommunities(got))
}

// TestStripSetStillExact verifies that adding the scrub leaves the
// operator's named-set `strip` a whole-value exact match. The scrub has its
// own container precisely so `strip` does not change meaning for a config
// that already uses it.
//
// VALIDATES: AC-18 (spec-bcp194-1-communities)
// PREVENTS: `strip` silently matching more values than it did before this change.
func TestStripSetStillExact(t *testing.T) {
	sameGA := scrubLocalAS<<16 | 100
	named := scrubLocalAS<<16 | 200
	payload := buildPayload(append(buildOriginAttr(), buildCommunityAttr(sameGA, named)...))

	namedWire := make([]byte, 4)
	binary.BigEndian.PutUint32(namedWire, named)
	defs := communityDefs{"drop-me": &communityDef{
		typ:        communityTypeStandard,
		wireValues: [][]byte{namedWire},
	}}
	got := applyIngressFilter(payload, defs, filterConfig{ingressStrip: []string{"drop-me"}}, scrubLocalAS, 64511)
	require.NotNil(t, got)

	assert.Equal(t, []uint32{sameGA}, extractCommunities(got),
		"only the exact named value is removed; the sibling sharing its ASN survives")
}
