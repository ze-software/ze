// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
package filter_path_asn

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/hub/yang"
)

// subtreeJSON parses one policy body and returns the BGP subtree in the form
// the plugin boundary delivers it: the subtree config.ExtractConfigSubtree cuts,
// marshaled to JSON.
//
// Every config test goes the long way on purpose. A parse driven from a
// hand-built map would pass while the delivered shape was something else, and
// the delivered shape is the thing four readers have already guessed wrong
// about (internal/core/configvalue).
func subtreeJSON(t *testing.T, listBody string) string {
	t.Helper()

	tree, err := config.ParseTreeWithYANG(policyOnly(listBody), nil)
	require.NoError(t, err, "the schema must accept the body; this test is about the plugin's own refusals")

	subtree := config.ExtractConfigSubtree(tree.ToMap(), "bgp")
	require.NotNil(t, subtree)
	data, err := json.Marshal(subtree)
	require.NoError(t, err)

	return string(data)
}

// parseLists delivers one policy body to the plugin's own config parse.
func parseLists(t *testing.T, listBody string) (map[string]*rejectList, error) {
	t.Helper()

	bgpCfg, ok := configjson.ParseBGPSubtree(subtreeJSON(t, listBody))
	require.True(t, ok)

	return parseRejectASNLists(bgpCfg)
}

// TestPositionKeyExpansion asserts the position vocabulary itself: which
// primitive positions each key an operator can type expands to.
//
// The table is the contract, so the table is what is under test. Written
// against the matcher instead, an expansion that changed would still pass every
// case the matcher's own tests happened to cover, and every list in every
// config would quietly mean something else.
//
// VALIDATES: R-12's mitigation, and the vocabulary AC-3 through AC-12 are read
// against.
// PREVENTS: via silently growing to include neighbor, which accepts the exact
// leak this filter exists to catch.
func TestPositionKeyExpansion(t *testing.T) {
	cases := []struct {
		key  string
		want []position
	}{
		{key: "direct", want: []position{positionDirect}},
		{key: "transit", want: []position{positionTransit}},
		{key: "origin", want: []position{positionOrigin}},
		{key: "indirect", want: []position{positionTransit, positionOrigin}},
		{key: "anywhere", want: []position{positionDirect, positionTransit, positionOrigin}},
	}

	primitives := []position{positionDirect, positionTransit, positionOrigin}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			set, ok := positionsByKey[tc.key]
			require.True(t, ok, "the key must be in the vocabulary")

			wanted := make(map[position]bool, len(tc.want))
			for _, p := range tc.want {
				wanted[p] = true
			}
			for _, p := range primitives {
				assert.Equal(t, wanted[p], set.holds(p),
					"%s must %shold %s", tc.key, map[bool]string{true: "", false: "not "}[wanted[p]], p)
			}
		})
	}

	assert.Len(t, positionsByKey, len(cases),
		"the vocabulary has five position leaves; regex is the sixth leaf and names no position")
	_, hasRegex := positionsByKey[positionKeyRegex]
	assert.False(t, hasRegex,
		"regex values are patterns matched against the whole path, so no position applies to them")
}

// TestParseRefusesListWithNoPosition covers AC-15.
//
// Nothing else refuses it. An absent leaf-list has no node for any schema walk
// to reach, so no YANG bound can see the fault. Phase 1 measured that against
// the old shape and it is unchanged by the move to six leaf-lists.
//
// VALIDATES: AC-15.
// PREVENTS: a list that rejects nothing while reading in the config file like a
// safety filter, which is the guard-shaped zero ai/rules/principles.md forbids.
func TestParseRefusesListWithNoPosition(t *testing.T) {
	_, err := parseLists(t, `        reject-asn NO-TRANSIT {
        }`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NO-TRANSIT", "the message must name the list")
	assert.Contains(t, err.Error(), "no ASN and no pattern", "the message must name what is missing")
}

// TestParseRefusesEmptyASNList covers AC-16.
//
// An empty leaf-list and an absent one are one state by the time the config
// reaches here: Tree.ToMap omits a leaf-list with no active member
// (internal/core/configvalue), so `indirect [ ]` and no via leaf at all deliver the
// same list. Both are the empty reject set, so both meet AC-15's refusal, and a
// list whose every leaf is written empty is refused exactly like one that wrote
// nothing.
//
// VALIDATES: AC-16.
// PREVENTS: an empty leaf-list reading as a configured rule, for the same reason
// as AC-15.
func TestParseRefusesEmptyASNList(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{{
		name: "one_leaf_written_empty",
		body: `        reject-asn NO-TRANSIT {
            indirect [ ]
        }`,
	}, {
		name: "every_leaf_written_empty",
		body: `        reject-asn NO-TRANSIT {
            direct [ ]
            transit [ ]
            origin [ ]
            indirect [ ]
            anywhere [ ]
            regex [ ]
        }`,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseLists(t, tc.body)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "NO-TRANSIT", "the message must name the list")
			assert.Contains(t, err.Error(), "no ASN and no pattern",
				"the message must say the list names nothing")
		})
	}
}

// TestSameASNUnderTwoPositionsUnions covers AC-17.
//
// VALIDATES: an ASN written under two position leaves resolves to one entry
// holding the union, and the ASNs of the other key are untouched.
// PREVENTS: the second block overwriting the first, which would silently drop
// half of an operator's policy.
func TestSameASNUnderTwoPositionsUnions(t *testing.T) {
	lists, err := parseLists(t, `        reject-asn NO-TRANSIT {
            indirect [ 3356 174 ]
            direct [ 3356 ]
        }`)
	require.NoError(t, err)

	list := lists["NO-TRANSIT"]
	require.NotNil(t, list)

	assert.True(t, list.positions[3356].holds(positionDirect), "the neighbor block must survive")
	assert.True(t, list.positions[3356].holds(positionTransit), "the via block must survive")
	assert.True(t, list.positions[3356].holds(positionOrigin), "the via block must survive")

	assert.False(t, list.positions[174].holds(positionDirect),
		"174 is listed under via alone, so unioning 3356 must not reach it")
	assert.True(t, list.positions[174].holds(positionTransit))
}

// TestParseRefusesUncompilablePattern covers AC-46.
//
// VALIDATES: a pattern that regexp.Compile refuses stops the config at load,
// naming the list and the pattern.
// PREVENTS: a broken pattern reaching the hot path as a nil regex, or being
// dropped so the operator's rule silently does not run.
func TestParseRefusesUncompilablePattern(t *testing.T) {
	_, err := parseLists(t, `        reject-asn NO-TRANSIT {
            regex [ "^3356 (174" ]
        }`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NO-TRANSIT", "the message must name the list")
	assert.Contains(t, err.Error(), "^3356 (174", "the message must name the pattern")
}

// TestParseRefusesOverlongPattern covers AC-47.
//
// The 512-character cap is bgp-filter-aspath's, applied for the same reason: RE2
// is linear in the subject, so the bound is defense in depth rather than a ReDoS
// guard. The regex leaf-list carries the length in YANG too, and phase 1 measured
// that such a bound fires only under ValidateTreeAllModules, which no daemon path
// runs over bgp.
//
// VALIDATES: AC-47.
// PREVENTS: a bound declared in the schema and enforced by nothing.
func TestParseRefusesOverlongPattern(t *testing.T) {
	_, err := parseLists(t, `        reject-asn NO-TRANSIT {
            regex [ "^`+strings.Repeat("a", maxPatternLen)+`" ]
        }`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NO-TRANSIT", "the message must name the list")
	assert.Contains(t, err.Error(), "512", "the message must name the bound the operator crossed")
}

// TestSchemaRefusesANameWhereAnASNBelongs covers what survives of AC-50.
//
// AC-50 was written against a single `asn` leaf-list whose YANG type was a union
// of uint32 and string, because one leaf name had to carry both an ASN and a
// pattern and a union cannot say which arm belongs under which key. The plugin
// made the check itself. Six typed leaf-lists removed the ambiguity: each of the
// five position leaves is `type uint32`, so the SCHEMA refuses a name written
// where an ASN belongs, before the plugin sees the config at all. The guard moved
// down a layer, so the test moved with it.
//
// The other direction of AC-50, a bare decimal under `regex`, is no longer a
// fault: `regex` is `type string` and an operator who writes a number under it
// has asked for that pattern by name rather than by an ambiguous union arm.
//
// VALIDATES: AC-50's surviving half, at the layer that now enforces it.
// PREVENTS: the five position leaves losing their uint32 type, which would let a
// name through to a parse that has stopped checking for one.
func TestSchemaRefusesANameWhereAnASNBelongs(t *testing.T) {
	for _, leaf := range []string{"direct", "transit", "origin", "indirect", "anywhere"} {
		t.Run(leaf, func(t *testing.T) {
			_, err := config.ParseTreeWithYANG(policyOnly(`        reject-asn NO-TRANSIT {
            `+leaf+` [ LEVEL3 ]
        }`), nil)

			require.Error(t, err, "%s is type uint32 and must refuse a name", leaf)
			assert.Contains(t, err.Error(), "LEVEL3", "the message must name the value")
		})
	}
}

// TestASNBoundaries walks the range of a position leaf-list.
//
// The YANG type is uint32, so a decimal one past the range is refused by the
// schema rather than by the plugin. AS0 and AS4294967295 are both reserved
// (RFC 7607, RFC 6996), which is a reason an operator might list them and no
// reason to refuse one.
//
// VALIDATES: the Boundary Tests row for position leaf-list values.
// PREVENTS: a parse that reads an ASN into a narrower type, where 4294967296
// would wrap to 0 and reject every locally originated route.
func TestASNBoundaries(t *testing.T) {
	accepted := []string{"0", "23456", "4294967295"}
	for _, value := range accepted {
		t.Run("accepts_"+value, func(t *testing.T) {
			lists, err := parseLists(t, `        reject-asn NO-TRANSIT {
            indirect [ `+value+` ]
        }`)
			require.NoError(t, err)
			asn, convErr := strconv.ParseUint(value, 10, 64)
			require.NoError(t, convErr)
			assert.True(t, lists["NO-TRANSIT"].positions[uint32(asn)].holds(positionOrigin))
		})
	}

	t.Run("refuses_4294967296", func(t *testing.T) {
		_, err := config.ParseTreeWithYANG(policyOnly(`        reject-asn NO-TRANSIT {
            indirect [ 4294967296 ]
        }`), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "4294967296", "the message must name the value out of range")
	})
}

// TestRegexOnlyListIsValid covers AC-48.
//
// VALIDATES: a list whose only leaf-list is regex parses, because AC-15 asks the
// list to name something and a pattern is something.
// PREVENTS: AC-15's refusal being written as "at least one ASN", which would
// refuse a list that is entirely patterns.
func TestRegexOnlyListIsValid(t *testing.T) {
	lists, err := parseLists(t, `        reject-asn SHAPES {
            regex [ "^3356 174 " ]
        }`)
	require.NoError(t, err)

	list := lists["SHAPES"]
	require.NotNil(t, list)
	assert.Empty(t, list.positions, "a regex-only list names no ASN at any position")
	require.Len(t, list.patterns, 1)
	assert.Equal(t, "^3356 174 ", list.patterns[0].String())
}

// TestPasteableLeafIsInTheVocabulary holds the leaf that
// `show bgp reject-asn known transit-free` writes its ASNs under to the position
// vocabulary.
//
// VALIDATES: the block the command prints lands on a leaf the parse reads.
// PREVENTS: a printed block an operator pastes and commits that reaches
// parseOneList as no rule at all, which AC-15 would then refuse with a message
// about the operator's config rather than about ze's own output.
func TestPasteableLeafIsInTheVocabulary(t *testing.T) {
	_, ok := positionsByKey[positionKeyPasteable]
	assert.True(t, ok, "the pasteable block names a leaf the parse does not read")
}

// TestParseReadsNthEntries covers the one keyed keyword, end to end from config
// text.
//
// VALIDATES: `nth <n> [ asn ... ]` reaches the hot path as (index, ASN) pairs,
// several entries coexist, and one entry carries several ASNs.
// PREVENTS: the inline form storing the bracket as one joined string, which is
// what the config parser did for a keyed list before 2026-09-03 and which would
// reach here as the single ASN "[ 174 3356 ]".
func TestParseReadsNthEntries(t *testing.T) {
	lists, err := parseLists(t, `        reject-asn NO-TRANSIT {
            nth 1 [ 174 3356 ]
            nth 3 [ 3491 ]
        }`)
	require.NoError(t, err)

	list := lists["NO-TRANSIT"]
	require.NotNil(t, list)
	assert.Empty(t, list.positions, "an nth-only list names no ASN at a partition position")

	for _, want := range []nthKey{{index: 1, asn: 174}, {index: 1, asn: 3356}, {index: 3, asn: 3491}} {
		_, ok := list.nth[want]
		assert.True(t, ok, "nth %d must hold AS%d", want.index, want.asn)
	}
	assert.Len(t, list.nth, 3, "and nothing else")
}

// TestParseRefusesNthWithNoASN covers AC-16 under the keyed keyword.
//
// VALIDATES: an `nth` entry with an empty asn list is refused, naming the list
// and the index.
// PREVENTS: a position an operator wrote that rejects nothing, which is AC-15's
// failure one level down.
func TestParseRefusesNthWithNoASN(t *testing.T) {
	_, err := parseLists(t, `        reject-asn NO-TRANSIT {
            nth 2 [ ]
        }`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "NO-TRANSIT", "the message must name the list")
	assert.Contains(t, err.Error(), "nth 2", "the message must name the position")
}

// TestNthOnlyListIsValid holds AC-15's refusal to the list that truly names
// nothing.
//
// VALIDATES: a list whose only keyword is `nth` parses.
// PREVENTS: AC-15 written as "at least one ASN leaf-list or one pattern", which
// would refuse a list that is entirely nth rules.
func TestNthOnlyListIsValid(t *testing.T) {
	lists, err := parseLists(t, `        reject-asn NO-TRANSIT {
            nth 2 [ 3491 ]
        }`)
	require.NoError(t, err)
	require.NotNil(t, lists["NO-TRANSIT"])
}

// TestSchemaRefusesAnNthIndexOutOfRange walks the bounds of the nth key.
//
// VALIDATES: the uint8 range 1..255 on the index is enforced by the config
// parser, at both ends.
// PREVENTS: index 0, which no token can hold because the count is 1-based, and
// index 256, which would truncate into a small index and fire a rule the
// operator did not write.
func TestSchemaRefusesAnNthIndexOutOfRange(t *testing.T) {
	for _, index := range []string{"0", "256"} {
		t.Run(index, func(t *testing.T) {
			_, err := config.ParseTreeWithYANG(policyOnly(`        reject-asn NO-TRANSIT {
            nth `+index+` [ 3491 ]
        }`), nil)

			require.Error(t, err, "nth %s is outside the declared range", index)
		})
	}

	for _, index := range []string{"1", "255"} {
		t.Run("accepts_"+index, func(t *testing.T) {
			lists, err := parseLists(t, `        reject-asn NO-TRANSIT {
            nth `+index+` [ 3491 ]
        }`)
			require.NoError(t, err)
			require.NotNil(t, lists["NO-TRANSIT"])
		})
	}
}
