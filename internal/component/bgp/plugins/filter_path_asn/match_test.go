// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
package filter_path_asn

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// updateFor renders the filter text of a route whose AS_PATH carries segments,
// through the producer of that format so a test and the wire cannot drift.
func updateFor(segments ...attribute.ASPathSegment) string {
	path := &attribute.ASPath{Segments: segments}
	buf := attribute.Origin(0).AppendText(nil)
	buf = append(buf, ' ')
	buf = path.AppendText(buf)
	return string(buf)
}

// sequence is one AS_SEQUENCE, the segment nearly every path carries.
func sequence(asns ...uint32) attribute.ASPathSegment {
	return attribute.ASPathSegment{Type: attribute.ASSequence, ASNs: asns}
}

// asPathOf renders one AS_SEQUENCE and reads it back the way the plugin does,
// so a matcher test sees exactly the string filtertext.ASPath produces.
func asPathOf(asns ...uint32) string {
	return filtertext.ASPath(updateFor(sequence(asns...)))
}

// listing maps one ASN to the primitive set the named position leaf expands to,
// reading the vocabulary rather than restating it.
func listing(asn uint32, key string) map[uint32]positionSet {
	return map[uint32]positionSet{asn: positionsByKey[key]}
}

// from is the sender a matcher test judges the leading run against.
func from(asn uint32) senderASN { return senderASN{asn: asn, known: true} }

// configureFrom hands one policy body to the plugin's configure, which is the
// only route a list reaches the hot path by.
func configureFrom(t *testing.T, listBody string) {
	t.Helper()
	require.NoError(t, configure([]sdk.ConfigSection{{Root: "bgp", Data: subtreeJSON(t, listBody)}}))
}

// TestPositionMatrixEveryKeyEveryIndex drives AC-3 through AC-12 as one table:
// every position leaf against every place an ASN can sit, judged against a peer
// that IS the listed ASN and a peer that is not.
//
// VALIDATES: AC-3, AC-4, AC-5, AC-6, AC-7, AC-8, AC-9, AC-10, AC-11, AC-12.
// PREVENTS: a matcher that is right about one key and wrong about the rest,
// which every single-key test would pass.
func TestPositionMatrixEveryKeyEveryIndex(t *testing.T) {
	cases := []struct {
		name   string
		path   []uint32
		sender senderASN
		listed uint32
		key    string
		reject bool
		at     position
	}{{
		name:   "AC-3_leading_run_is_the_sender_so_indirect_excludes_it",
		path:   []uint32{3356, 65001},
		sender: from(3356),
		listed: 3356,
		key:    "indirect",
	}, {
		name:   "AC-4_prepends_collapse_into_one_direct",
		path:   []uint32{3356, 3356, 3356, 65001},
		sender: from(3356),
		listed: 3356,
		key:    "indirect",
	}, {
		name:   "AC-5_only_the_leading_run_is_exempt",
		path:   []uint32{3356, 174, 65001},
		sender: from(3356),
		listed: 174,
		key:    "indirect",
		reject: true,
		at:     positionTransit,
	}, {
		name:   "AC-6_index_zero_is_not_direct",
		path:   []uint32{3356, 65001},
		sender: from(65001),
		listed: 3356,
		key:    "indirect",
		reject: true,
		at:     positionTransit,
	}, {
		name:   "AC-7_the_senders_own_ASN_later_in_the_path_is_not_exempt",
		path:   []uint32{3356, 65001, 3356},
		sender: from(3356),
		listed: 3356,
		key:    "indirect",
		reject: true,
		at:     positionOrigin,
	}, {
		name:   "AC-8_transit_alone_does_not_reach_the_origin",
		path:   []uint32{65001, 3356},
		sender: from(65001),
		listed: 3356,
		key:    "transit",
	}, {
		name:   "AC-9_via_reaches_the_origin",
		path:   []uint32{65001, 3356},
		sender: from(65001),
		listed: 3356,
		key:    "indirect",
		reject: true,
		at:     positionOrigin,
	}, {
		name:   "AC-10_all_reaches_the_origin",
		path:   []uint32{65001, 3356},
		sender: from(65001),
		listed: 3356,
		key:    "anywhere",
		reject: true,
		at:     positionOrigin,
	}, {
		name:   "AC-11_a_lone_ASN_that_is_not_the_sender_is_the_origin",
		path:   []uint32{3356},
		sender: from(65001),
		listed: 3356,
		key:    "origin",
		reject: true,
		at:     positionOrigin,
	}, {
		name:   "AC-12_a_lone_ASN_that_is_the_sender_is_direct",
		path:   []uint32{3356},
		sender: from(3356),
		listed: 3356,
		key:    "indirect",
	}, {
		name:   "direct_key_reaches_the_leading_run",
		path:   []uint32{3356, 65001},
		sender: from(3356),
		listed: 3356,
		key:    "direct",
		reject: true,
		at:     positionDirect,
	}, {
		name:   "direct_key_does_not_reach_the_origin",
		path:   []uint32{65001, 3356},
		sender: from(65001),
		listed: 3356,
		key:    "direct",
	}, {
		name:   "transit_key_reaches_the_middle",
		path:   []uint32{65001, 174, 65002},
		sender: from(65001),
		listed: 174,
		key:    "transit",
		reject: true,
		at:     positionTransit,
	}, {
		name:   "origin_key_does_not_reach_the_middle",
		path:   []uint32{65001, 174, 65002},
		sender: from(65001),
		listed: 174,
		key:    "origin",
	}, {
		name:   "anywhere_reaches_direct",
		path:   []uint32{65001, 174, 65002},
		sender: from(65001),
		listed: 65001,
		key:    "anywhere",
		reject: true,
		at:     positionDirect,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found, ok := matchPositions(asPathOf(tc.path...), tc.sender, listing(tc.listed, tc.key), nil)

			require.Equal(t, tc.reject, ok)
			if !tc.reject {
				return
			}
			assert.Equal(t, tc.listed, found.asn, "the reject must name the offending ASN")
			assert.Equal(t, tc.at, found.at, "the reject must name the position it matched at")
		})
	}
}

// TestDirectIsDefinedByTheSenderNotTheIndex drives AC-6 through the whole
// path, from an operator's config to the decision.
//
// A path [3356 65001] carries 3356 at index zero. Learned from AS3356 it is the
// direct peer and legitimate; learned from AS65001 it is that peer handing us
// its upstream, which is the route-server case RFC 7454 Section 9 names and the
// exact leak this filter exists to catch. One config, one path, two senders,
// two answers.
//
// VALIDATES: AC-6, and R-11's mitigation.
// PREVENTS: direct implemented as index zero, which passes every test whose
// peer is not itself on the list and accepts the leak.
func TestDirectIsDefinedByTheSenderNotTheIndex(t *testing.T) {
	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
        }`)

	update := updateFor(sequence(3356, 65001))

	sent := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 3356, Update: update,
	})
	assert.Equal(t, sdk.FilterAccept, sent.Action,
		"AS3356 sent this path itself, so its leading run is the neighbor and via excludes it")

	leaked := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.2", PeerAS: 65001, Update: update,
	})
	assert.Equal(t, sdk.FilterReject, leaked.Action,
		"AS65001 reached us through AS3356, so 3356 is transit however early it sits in the path")
}

// TestPrependRunCollapsesToOneNeighbor covers AC-4.
//
// VALIDATES: a run of prepends by the sending peer is one direct position,
// however long, and the first token that is not the sender's ends the run.
// PREVENTS: an exemption written as "index zero", which would reject a peer that
// prepends itself twice.
func TestPrependRunCollapsesToOneNeighbor(t *testing.T) {
	positions := listing(3356, "indirect")

	_, ok := matchPositions(asPathOf(3356, 3356, 3356, 65001), from(3356), positions, nil)
	assert.False(t, ok, "three prepends by AS3356 are one direct position")

	found, ok := matchPositions(asPathOf(3356, 3356, 65001, 3356, 65002), from(3356), positions, nil)
	require.True(t, ok, "the run ends at the first token that is not the sender's ASN")
	assert.Equal(t, uint32(3356), found.asn)
	assert.Equal(t, positionTransit, found.at)
}

// TestExportHasNoNeighborSoViaCoversTheWholePath covers AC-13 and AC-14.
//
// On export FilterUpdateInput.PeerAS is the DESTINATION peer, not the ASN the
// route was learned from, so no token is direct and indirect reaches index
// zero. The asymmetry is read off the seam once, in senderOf, and never written
// into the list.
//
// VALIDATES: AC-13, AC-14.
// PREVENTS: the export half exempting the destination's own ASN, which would
// let the one path an operator most wants suppressed through.
func TestExportHasNoNeighborSoViaCoversTheWholePath(t *testing.T) {
	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
        }`)

	// AC-13: 3356 sits at index zero and the destination peer IS AS3356. Nothing
	// makes it direct, because nothing on export says who sent the route.
	leaked := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "export", Peer: "10.0.0.1", PeerAS: 3356,
		Update: updateFor(sequence(3356, 65001)),
	})
	assert.Equal(t, sdk.FilterReject, leaked.Action,
		"a path through AS3356 must not be advertised, whatever the destination's own ASN is")

	// AC-14: the same destination, a path that carries no listed ASN.
	clean := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "export", Peer: "10.0.0.1", PeerAS: 3356,
		Update: updateFor(sequence(65001, 65002)),
	})
	assert.Equal(t, sdk.FilterAccept, clean.Action,
		"the destination's own ASN is never consulted, so a clean path is advertised")
}

// TestScanUnbracketedSingleASN covers AC-20.
//
// (*attribute.ASPath).AppendText writes one ASN bare and several in brackets, so
// a reader that assumes brackets loses the single-ASN case, which is the
// direct-peer case this filter most needs to read correctly.
//
// VALIDATES: AC-20, and AC-11 and AC-12 as the two decisions it resolves to.
// PREVENTS: a single-ASN path scanning as no tokens, which accepts everything a
// direct peer originates.
func TestScanUnbracketedSingleASN(t *testing.T) {
	update := updateFor(sequence(3356))
	require.Contains(t, update, "as-path 3356", "the producer writes a lone ASN unbracketed")
	require.Equal(t, "3356", filtertext.ASPath(update))

	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
        }`)

	stranger := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001, Update: update,
	})
	assert.Equal(t, sdk.FilterReject, stranger.Action, "a lone ASN that is not the sender is the origin")

	itself := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 3356, Update: update,
	})
	assert.Equal(t, sdk.FilterAccept, itself.Action, "a lone ASN that is the sender is direct")
}

// TestScanAbsentASPath covers AC-19.
//
// A path with no ASNs emits no as-path token at all, so the subject is the empty
// string. It carries no token, so no position can match it, and a locally
// originated route is accepted in both directions with no case written for it.
//
// VALIDATES: AC-19.
// PREVENTS: an empty subject reading as one empty token, which would reject
// every locally originated route the moment a list named AS0.
func TestScanAbsentASPath(t *testing.T) {
	update := updateFor()
	require.NotContains(t, update, "as-path", "an empty path emits no keyword")
	require.Empty(t, filtertext.ASPath(update))

	configureFrom(t, `        reject-asn NO-TRANSIT {
            anywhere [ 0 3356 ]
        }`)

	for _, direction := range []string{"import", "export"} {
		out := handleFilterUpdate(&sdk.FilterUpdateInput{
			Filter: "NO-TRANSIT", Direction: direction, Peer: "10.0.0.1", PeerAS: 65001, Update: update,
		})
		assert.Equal(t, sdk.FilterAccept, out.Action, "a route with no AS_PATH is accepted on %s", direction)
	}
}

// TestScanFlattenedASSet covers AC-21.
//
// AppendText writes AS_SEQUENCE, AS_SET, AS_CONFED_SEQUENCE and AS_CONFED_SET
// into one space-separated list with no marker, so an ASN inside an AS_SET is
// scanned like any other and its position is whatever its index in the
// flattened list gives it.
//
// VALIDATES: AC-21.
// PREVENTS: an AS_SET hiding a listed ASN, which passes a leak.
func TestScanFlattenedASSet(t *testing.T) {
	asPath := filtertext.ASPath(updateFor(
		sequence(65001),
		attribute.ASPathSegment{Type: attribute.ASSet, ASNs: []uint32{174, 3356}},
		sequence(65002),
	))
	require.Equal(t, "65001 174 3356 65002", asPath, "every segment type flattens into one list")

	found, ok := matchPositions(asPath, from(65001), listing(3356, "indirect"), nil)
	require.True(t, ok, "an AS_SET member is scanned like any other token")
	assert.Equal(t, uint32(3356), found.asn)
	assert.Equal(t, positionTransit, found.at, "its position is its index in the flattened list")
}

// TestPathLengthBoundaries walks the token count of a scanned path.
//
// One AS_PATH segment carries at most 255 ASNs, because RFC 4271 Section 4.3
// gives its count one octet, and an UPDATE can carry several segments. The
// interesting counts are therefore zero, one, a full segment, and two full
// segments, and the ASN under test sits at the last index of each so the origin
// is judged from a count the scan worked out rather than assumed.
//
// VALIDATES: the Boundary Tests row for ASNs per scanned path.
// PREVENTS: an off-by-one in the origin test, which only shows at a length no
// hand-written case happens to use.
func TestPathLengthBoundaries(t *testing.T) {
	const segmentMax = 255

	for _, tokens := range []int{1, segmentMax, 2 * segmentMax} {
		t.Run(strconv.Itoa(tokens), func(t *testing.T) {
			asns := make([]uint32, tokens)
			for i := range asns {
				asns[i] = uint32(65000 + i)
			}
			asns[tokens-1] = 3356

			var segments []attribute.ASPathSegment
			for start := 0; start < tokens; start += segmentMax {
				segments = append(segments, sequence(asns[start:min(start+segmentMax, tokens)]...))
			}
			asPath := filtertext.ASPath(updateFor(segments...))

			found, ok := matchPositions(asPath, from(65000), listing(3356, "origin"), nil)
			require.True(t, ok, "the last token of a %d-token path is the origin", tokens)
			assert.Equal(t, uint32(3356), found.asn)
			assert.Equal(t, positionOrigin, found.at)
		})
	}

	_, ok := matchPositions("", from(65000), listing(3356, "anywhere"), nil)
	assert.False(t, ok, "a path of no tokens has no position for any ASN to occupy")
}

// TestScanAllocatesNothing covers AC-51.
//
// A list with no regex block runs for every UPDATE on every session it is
// attached to, so it is a wire path and Ze targets zero allocation on one. The
// scan therefore walks the subject in place and parses each decimal off a slice
// of it: no strings.Fields, no strings.Split, no per-token buffer.
//
// VALIDATES: AC-51.
// PREVENTS: a tokenizer that allocates one slice for each UPDATE, which is
// invisible in every correctness test and is paid millions of times.
func TestScanAllocatesNothing(t *testing.T) {
	asPath := asPathOf(65001, 65001, 174, 65002, 3356, 65003)
	sender := from(65001)
	positions := map[uint32]positionSet{
		3356:  positionsByKey["indirect"],
		65010: positionsByKey["anywhere"],
	}

	hits := testing.AllocsPerRun(100, func() {
		if _, ok := matchPositions(asPath, sender, positions, nil); !ok {
			t.Error("the path carries 3356 at transit, so this run must reject")
		}
	})
	assert.Zero(t, hits, "a rejecting scan must allocate nothing")

	misses := testing.AllocsPerRun(100, func() {
		if _, ok := matchPositions(asPath, sender, listing(65500, "anywhere"), nil); ok {
			t.Error("the path carries no listed ASN, so this run must accept")
		}
	})
	assert.Zero(t, misses, "an accepting scan must allocate nothing")
}

// TestRegexRejectsOnShape covers AC-43 and AC-44.
//
// A pattern asks a shape question the position vocabulary cannot: not "is 3356
// in the path" but "is 3356 immediately followed by 174".
//
// VALIDATES: AC-43, AC-44.
// PREVENTS: a pattern matched against something other than the whole path, for
// example one token at a time, which would answer the position question again
// and never the shape one.
func TestRegexRejectsOnShape(t *testing.T) {
	configureFrom(t, `        reject-asn SHAPES {
            regex [ "^3356 174 " ]
        }`)

	adjacent := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "SHAPES", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
		Update: updateFor(sequence(3356, 174, 65001)),
	})
	assert.Equal(t, sdk.FilterReject, adjacent.Action, "the shape holds, so the route is rejected")

	apart := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "SHAPES", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
		Update: updateFor(sequence(3356, 65001, 174)),
	})
	assert.Equal(t, sdk.FilterAccept, apart.Action,
		"both ASNs are present and the shape does not hold, so the route is accepted")
}

// TestRegexAndPositionsUnion covers AC-45 and AC-42.
//
// The keys of one list are unioned with no ordering between them: a route
// matching either arm is rejected. A route matching both is still one reject,
// because the list is a set and not an ordered chain.
//
// VALIDATES: AC-42, AC-45.
// PREVENTS: a regex leaf being read as an alternative to the position leaves
// rather than as another member of the same set.
func TestRegexAndPositionsUnion(t *testing.T) {
	configureFrom(t, `        reject-asn BOTH {
            indirect [ 174 ]
            regex [ "^3356 " ]
        }`)

	cases := []struct {
		name string
		path []uint32
		want sdk.FilterAction
	}{
		{name: "position_arm_alone", path: []uint32{65001, 174, 65002}, want: sdk.FilterReject},
		{name: "pattern_arm_alone", path: []uint32{3356, 65002}, want: sdk.FilterReject},
		{name: "both_arms", path: []uint32{3356, 174, 65002}, want: sdk.FilterReject},
		{name: "neither_arm", path: []uint32{65001, 65002}, want: sdk.FilterAccept},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := handleFilterUpdate(&sdk.FilterUpdateInput{
				Filter: "BOTH", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
				Update: updateFor(sequence(tc.path...)),
			})
			assert.Equal(t, tc.want, out.Action)
		})
	}
}

// TestRegexSubjectIsTheFlattenedString covers AC-49.
//
// VALIDATES: a pattern is matched against the same space-separated string every
// other reader of the format sees, so an AS_SET member is reachable and no
// bracket appears in the subject.
// PREVENTS: a pattern matched against the raw as-path FIELD, whose multi-ASN
// form carries brackets, so every anchored pattern an operator writes would
// silently stop matching at two ASNs.
func TestRegexSubjectIsTheFlattenedString(t *testing.T) {
	configureFrom(t, `        reject-asn SHAPES {
            regex [ "^65001 174 3356 " ]
        }`)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "SHAPES", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
		Update: updateFor(
			sequence(65001),
			attribute.ASPathSegment{Type: attribute.ASSet, ASNs: []uint32{174, 3356}},
			sequence(65002),
		),
	})

	assert.Equal(t, sdk.FilterReject, out.Action,
		"the subject is the flattened path with no brackets, so an anchored pattern reaches an AS_SET member")
}

// nthListing maps one ASN to the collapsed positions an `nth` keyword rejects it
// at, in the shape the hot path reads.
func nthListing(asn uint32, indexes ...uint8) map[nthKey]struct{} {
	listed := make(map[nthKey]struct{}, len(indexes))
	for _, index := range indexes {
		listed[nthKey{index: index, asn: asn}] = struct{}{}
	}
	return listed
}

// TestNthCountsCollapsedPositionsFromUs drives the `nth` keyword across the
// places an ASN can sit, against a peer that is the listed ASN and one that is
// not.
//
// VALIDATES: goal 6's `nth <n>` row. The count is 1-based, starts at the ASN we
// are talking to, and reads the same whichever of direct, transit and origin the
// token happens to occupy.
// PREVENTS: an index counted from the origin end, or counted zero-based, either
// of which shifts every rule an operator writes by one position.
func TestNthCountsCollapsedPositionsFromUs(t *testing.T) {
	cases := []struct {
		name   string
		path   []uint32
		sender senderASN
		listed uint32
		index  uint8
		reject bool
	}{{
		name:   "nth_1_is_the_first_ASN_of_the_path",
		path:   []uint32{3356, 65001},
		sender: from(3356),
		listed: 3356,
		index:  1,
		reject: true,
	}, {
		name:   "nth_1_is_positional_where_direct_is_relational",
		path:   []uint32{3356, 65001},
		sender: from(65001),
		listed: 3356,
		index:  1,
		reject: true,
	}, {
		name:   "nth_2_reaches_a_transit_token",
		path:   []uint32{65001, 3491, 65002},
		sender: from(65001),
		listed: 3491,
		index:  2,
		reject: true,
	}, {
		name:   "nth_2_reaches_an_origin_token",
		path:   []uint32{65001, 3491},
		sender: from(65001),
		listed: 3491,
		index:  2,
		reject: true,
	}, {
		name:   "nth_2_does_not_reach_position_three",
		path:   []uint32{65001, 65002, 3491},
		sender: from(65001),
		listed: 3491,
		index:  2,
	}, {
		name:   "nth_1_does_not_reach_the_origin",
		path:   []uint32{65001, 3491},
		sender: from(65001),
		listed: 3491,
		index:  1,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			found, ok := matchPositions(asPathOf(tc.path...), tc.sender, nil, nthListing(tc.listed, tc.index))

			require.Equal(t, tc.reject, ok)
			if !tc.reject {
				return
			}
			assert.Equal(t, tc.listed, found.asn, "the reject must name the offending ASN")
			assert.Equal(t, positionNth, found.at, "an nth reject must say which keyword caught it")
			assert.Equal(t, tc.index, found.index, "the reject must name the position it matched at")
		})
	}
}

// TestNthCollapsesARunAnywhereInThePath holds the collapse rule the owner
// confirmed on 2026-09-03: a run of consecutive identical ASNs counts ONCE,
// wherever in the path it sits.
//
// VALIDATES: the collapse rule, in the leading run and in the middle.
// PREVENTS: a peer choosing which of your rules fires by prepending. Counting
// tokens instead of runs would let AS3491 move itself off `nth 2` by sending
// [65001 65001 3491], which hands your policy to the peer.
func TestNthCollapsesARunAnywhereInThePath(t *testing.T) {
	t.Run("a leading run is one position", func(t *testing.T) {
		found, ok := matchPositions(asPathOf(3356, 3356, 3356, 3491, 65002), from(3356),
			nil, nthListing(3491, 2))

		require.True(t, ok, "three prepends of AS3356 are one position, so AS3491 is at nth 2")
		assert.Equal(t, uint8(2), found.index)
	})

	t.Run("a run in the middle is one position", func(t *testing.T) {
		found, ok := matchPositions(asPathOf(65001, 65002, 65002, 65002, 3491), from(65001),
			nil, nthListing(3491, 3))

		require.True(t, ok, "three copies of AS65002 are one position, so AS3491 is at nth 3")
		assert.Equal(t, uint8(3), found.index)
	})

	t.Run("a peer cannot move a rule by prepending", func(t *testing.T) {
		listed := nthListing(3491, 2)

		_, plain := matchPositions(asPathOf(65001, 3491, 65002), from(65001), nil, listed)
		_, padded := matchPositions(asPathOf(65001, 65001, 3491, 65002), from(65001), nil, listed)

		assert.True(t, plain, "AS3491 is at nth 2 of the plain path")
		assert.True(t, padded, "and the prepend must not move it")
	})
}

// TestNthCutsAcrossThePartition proves a token holds a SET of properties rather
// than one label: the same ASN is caught by the keyword naming its partition
// position AND by the `nth` keyword naming its collapsed index.
//
// VALIDATES: goal 6's statement that nth cuts across direct/transit/origin.
// PREVENTS: a matcher that decides one label per token and then tests only that
// label, which would drop every nth rule whose token is not where the matcher
// looked.
func TestNthCutsAcrossThePartition(t *testing.T) {
	path := asPathOf(65001, 3491, 65002)

	_, byPosition := matchPositions(path, from(65001), listing(3491, "transit"), nil)
	assert.True(t, byPosition, "AS3491 sits at transit")

	_, byIndex := matchPositions(path, from(65001), nil, nthListing(3491, 2))
	assert.True(t, byIndex, "and the same token sits at nth 2")

	_, neither := matchPositions(path, from(65001), listing(3491, "origin"), nthListing(3491, 3))
	assert.False(t, neither, "and it is at neither the origin nor nth 3")
}

// TestNthBeyondTheIndexBoundDoesNotMatch walks the far end of the nth range.
//
// VALIDATES: a collapsed position past 255, the largest index the YANG allows,
// is a non-match rather than a wrap to a small index.
// PREVENTS: uint8 truncation, where position 256 would read as 0 and position
// 257 as 1, so a long path would fire a rule written for the peer itself.
func TestNthBeyondTheIndexBoundDoesNotMatch(t *testing.T) {
	path := make([]uint32, 0, nthIndexMax+2)
	for i := range nthIndexMax + 1 {
		path = append(path, uint32(65000+i))
	}
	path = append(path, 3491)

	listed := nthListing(3491, 1)
	_, ok := matchPositions(asPathOf(path...), from(65000), nil, listed)

	assert.False(t, ok, "AS3491 sits at collapsed position 257, which no nth rule can name")
}

// TestNthAllocatesNothing holds AC-51 across the keyword this phase added.
//
// VALIDATES: the nth lookup is one map read on a comparable struct key, so it
// allocates nothing for each UPDATE.
// PREVENTS: a nested map, a slice built for each token, or a key that escapes to
// the heap, each of which would put garbage on the wire path.
func TestNthAllocatesNothing(t *testing.T) {
	path := asPathOf(65001, 3491, 65002)
	listed := nthListing(3491, 2)

	assert.Zero(t, testing.AllocsPerRun(100, func() {
		matchPositions(path, from(65001), nil, listed) //nolint:errcheck // measuring allocation
	}), "the nth lookup must allocate nothing on a hit")

	miss := nthListing(3491, 3)
	assert.Zero(t, testing.AllocsPerRun(100, func() {
		matchPositions(path, from(65001), nil, miss) //nolint:errcheck // measuring allocation
	}), "the nth lookup must allocate nothing on a miss")
}
