// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-6, AC-7, now on the rule's
// chain -- the route's own tag reaches the chain an import rule names, so a chain that
// matches a tag accepts only routes carrying it, and a rule with no chain accepts every
// tag. Both polarities, including tag 0: redistRoute (internal/plugins/redistribute)
// builds an untagged entry as Tag 0 with HasTag set, so `match tag [ 0 ]` selects it.
// PREVENTS: the route's tag never reaching the chain, so a tag match silently rejects
// everything, a non-zero tag match accepting an untagged route, and a route whose tag
// is unset (HasTag false) being read as tag 0.
package redistribute

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

func TestImportRuleMatchesTag(t *testing.T) {
	tagged := RedistRoute{Origin: "static", Source: "static", Route: ribpolicy.Route{Family: family.IPv4Unicast, Tag: 42, HasTag: true}}
	// untagged is what redistRoute builds for an entry with no tag: Tag 0, HasTag set.
	untagged := RedistRoute{Origin: "static", Source: "static", Route: ribpolicy.Route{Family: family.IPv4Unicast, Tag: 0, HasTag: true}}
	// tagUnset carries no tag at all. redistRoute never builds it, and the evaluator must
	// still refuse it under a tag match: HasTag false means the tag is unknown, never 0
	// (ribpolicy.Route).
	tagUnset := RedistRoute{Origin: "static", Source: "static", Route: ribpolicy.Route{Family: family.IPv4Unicast}}

	cases := []struct {
		name  string
		rule  ImportRule
		route RedistRoute
		want  bool
	}{
		{
			name:  "no tag named accepts a tagged route",
			rule:  ImportRule{Source: "static", Destination: "ospf"},
			route: tagged,
			want:  true,
		},
		{
			name:  "no tag named accepts an untagged route",
			rule:  ImportRule{Source: "static", Destination: "ospf"},
			route: untagged,
			want:  true,
		},
		{
			name:  "matching tag accepts",
			rule:  ImportRule{Source: "static", Destination: "ospf", Final: "accept", Chain: tagChain(42)},
			route: tagged,
			want:  true,
		},
		{
			name:  "different tag rejects",
			rule:  ImportRule{Source: "static", Destination: "ospf", Final: "accept", Chain: tagChain(43)},
			route: tagged,
			want:  false,
		},
		{
			name:  "a non-zero tag filter rejects an untagged route",
			rule:  ImportRule{Source: "static", Destination: "ospf", Final: "accept", Chain: tagChain(42)},
			route: untagged,
			want:  false,
		},
		{
			name:  "tag 0 selects an untagged route",
			rule:  ImportRule{Source: "static", Destination: "ospf", Final: "accept", Chain: tagChain(0)},
			route: untagged,
			want:  true,
		},
		{
			name:  "tag 0 rejects a route whose tag is unset",
			rule:  ImportRule{Source: "static", Destination: "ospf", Final: "accept", Chain: tagChain(0)},
			route: tagUnset,
			want:  false,
		},
		{
			name:  "tag 0 rejects a tagged route",
			rule:  ImportRule{Source: "static", Destination: "ospf", Final: "accept", Chain: tagChain(0)},
			route: tagged,
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := accepts(&tc.rule, tc.route, "ospf"); got != tc.want {
				t.Fatalf("Accept = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEvaluatorRulesCopiesChain proves the diagnostic copy carries the rule's chain,
// so a caller reading Rules() cannot report a rule as unfiltered when it is not.
func TestEvaluatorRulesCopiesChain(t *testing.T) {
	ev := NewEvaluator([]ImportRule{{Source: "static", Destination: "ospf", Filters: []string{"T42"}, Final: "accept", Chain: tagChain(42)}})
	out := ev.Rules()
	if len(out) != 1 {
		t.Fatalf("Rules() returned %d rules, want 1", len(out))
	}
	if out[0].Final != "accept" || out[0].Chain == nil || len(out[0].Filters) != 1 || out[0].Filters[0] != "T42" {
		t.Fatalf("Rules() = %+v, want the chain preserved", out[0])
	}
}
