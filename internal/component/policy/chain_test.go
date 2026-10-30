package policy

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestChainConstruction proves only a named final configures a chain, while
// the zero chain and refs alone stay absent, and WithRefs borrows its input.
//
// VALIDATES: NewChain refuses a missing or unknown final; WithRefs keeps the final.
// PREVENTS: an arbitrary numeric final reaching a protocol's end-of-chain decision.
func TestChainConstruction(t *testing.T) {
	for _, final := range []Final{0, 3, 255} {
		chain, err := NewChain(nil, final)
		if err == nil || chain.Configured() {
			t.Fatalf("NewChain(%d) = %+v, %v; want refusal and absent value", final, chain, err)
		}
	}
	for _, final := range []Final{FinalAccept, FinalReject} {
		chain, err := NewChain(nil, final)
		require.NoError(t, err)
		if !chain.Configured() || chain.Final() != final {
			t.Fatalf("explicit empty chain lost final %d: %+v", final, chain)
		}
		refs := []Ref{{Name: "test:one", Inactive: true}}
		replaced := chain.WithRefs(refs)
		if len(chain.Refs()) != 0 || replaced.Final() != final || &replaced.Refs()[0] != &refs[0] {
			t.Fatal("WithRefs changed the original, lost final, or copied borrowed refs")
		}
	}
	absent := (Chain{}).WithRefs([]Ref{{Name: "automatic"}})
	if absent.Configured() || absent.Final() != 0 {
		t.Fatalf("refs alone configured a chain: %+v", absent)
	}
}

// TestConcatChains walks the inheritance rule every attachment hierarchy uses.
//
// VALIDATES: refs accumulate least specific first, inactive members included;
// the most specific configured chain supplies the final; an absent child keeps
// the inherited final; no configured level leaves the result absent.
// PREVENTS: a child without a final erasing its parent's, or a child's refs
// replacing rather than extending the parent's.
func TestConcatChains(t *testing.T) {
	global := mustChain(t, []Ref{{Name: "ext:global"}}, FinalReject)
	group := mustChain(t, []Ref{{Name: "ext:group", Inactive: true}}, FinalAccept)
	peerFinalOnly := mustChain(t, nil, FinalReject)
	automatic := (Chain{}).WithRefs([]Ref{{Name: "ext:auto"}})

	for _, tc := range []struct {
		name   string
		levels []Chain
		refs   []Ref
		final  Final
	}{
		{"none", nil, nil, 0},
		{"absent-levels", []Chain{{}, {}}, nil, 0},
		{"one", []Chain{global}, []Ref{{Name: "ext:global"}}, FinalReject},
		{"child-final-wins", []Chain{global, group}, []Ref{{Name: "ext:global"}, {Name: "ext:group", Inactive: true}}, FinalAccept},
		{"absent-child-inherits", []Chain{global, {}}, []Ref{{Name: "ext:global"}}, FinalReject},
		{"final-only-child", []Chain{global, group, peerFinalOnly}, []Ref{{Name: "ext:global"}, {Name: "ext:group", Inactive: true}}, FinalReject},
		{"refs-without-final", []Chain{automatic}, []Ref{{Name: "ext:auto"}}, 0},
		{"refs-without-final-after", []Chain{group, automatic}, []Ref{{Name: "ext:group", Inactive: true}, {Name: "ext:auto"}}, FinalAccept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ConcatChains(tc.levels...)
			if !slices.Equal(got.Refs(), tc.refs) || got.Final() != tc.final {
				t.Fatalf("ConcatChains = refs %+v final %d, want %+v final %d", got.Refs(), got.Final(), tc.refs, tc.final)
			}
		})
	}
}

// TestExtractChain reads one layer's `filter { <direction> { ... } }` block.
//
// VALIDATES: a block with a final yields its refs in order with member
// deactivation kept apart from the name; an absent block is an absent chain;
// a block without a final, an unknown final and the list-only form are refused
// naming the layer path.
// PREVENTS: an omitted final silently defaulting, or `inactive:` reaching a name.
func TestExtractChain(t *testing.T) {
	layer := func(final string, members []config.MemberState) *config.Tree {
		tree := config.NewTree()
		block := tree.GetOrCreateContainer("filter").GetOrCreateContainer("import")
		if final != "" {
			block.Set("final", final)
		}
		if members != nil {
			block.SetMemberStates("filters", members)
		}
		return tree
	}

	chain, err := ExtractChain(config.NewTree(), "import", "isis")
	require.NoError(t, err)
	assert.False(t, chain.Configured(), "absent block must be an absent chain")

	members := []config.MemberState{{Value: "A"}, {Value: "B", Inactive: true}}
	chain, err = ExtractChain(layer("reject", members), "import", "isis")
	require.NoError(t, err)
	assert.Equal(t, FinalReject, chain.Final())
	assert.Equal(t, []Ref{{Name: "A"}, {Name: "B", Inactive: true}}, chain.Refs())

	chain, err = ExtractChain(layer("accept", nil), "import", "isis")
	require.NoError(t, err)
	assert.True(t, chain.Configured())
	assert.Empty(t, chain.Refs())

	chain, err = ExtractChain(layer("accept", nil), "export", "isis")
	require.NoError(t, err)
	assert.False(t, chain.Configured(), "the other direction must stay absent")

	for _, final := range []string{"", "permit"} {
		_, err = ExtractChain(layer(final, members), "import", "isis")
		require.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "isis/filter/import/final: missing or invalid final"), err.Error())
	}

	listOnly := config.NewTree()
	listOnly.GetOrCreateContainer("filter").SetSlice("import", []string{"A"})
	_, err = ExtractChain(listOnly, "import", "isis")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "isis/filter/import: list-only policy is no longer supported")
}

// TestCanonicalNameForms resolves the three operator forms of a chain ref.
//
// VALIDATES: a plain name and a `<type>:<name>` both reach
// `<plugin>:<name>` through the filter type's plugin; a `<plugin>:<name>` and
// an unresolvable name pass through untouched.
// PREVENTS: a plain name reaching runtime dispatch as a bare string.
func TestCanonicalNameForms(t *testing.T) {
	reg, err := NewRegistry(Instance{Name: "CUSTOMERS", Type: "prefix-list"})
	require.NoError(t, err)
	types := map[string]string{"prefix-list": "bgp-filter-prefix"}

	for name, want := range map[string]string{
		"CUSTOMERS":                   "bgp-filter-prefix:CUSTOMERS",
		"prefix-list:CUSTOMERS":       "bgp-filter-prefix:CUSTOMERS",
		"bgp-filter-prefix:CUSTOMERS": "bgp-filter-prefix:CUSTOMERS",
		"MISSING":                     "MISSING",
		"other:CUSTOMERS":             "other:CUSTOMERS",
	} {
		assert.Equal(t, want, CanonicalName(name, reg, types), name)
	}
	assert.Equal(t, "CUSTOMERS", CanonicalName("CUSTOMERS", nil, types), "no registry leaves a plain name")
	assert.Equal(t, "CUSTOMERS", CanonicalName("CUSTOMERS", reg, nil), "an unknown type leaves a plain name")
}

func mustChain(t *testing.T, refs []Ref, final Final) Chain {
	t.Helper()
	chain, err := NewChain(refs, final)
	require.NoError(t, err)
	return chain
}
