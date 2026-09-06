// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-6, AC-7 -- an import rule
// that names a tag accepts only routes carrying it, and a rule that names none accepts
// every tag. Both polarities, including the tag-0 case a single uint32 cannot express.
// PREVENTS: a tag filter that silently accepts everything, and `tag 0` being read as
// "no filter" so a rule selecting untagged routes matches the whole source.
package redistribute

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

func TestImportRuleMatchesTag(t *testing.T) {
	tagged := RedistRoute{Origin: "static", Source: "static", Family: family.IPv4Unicast, Tag: 42}
	untagged := RedistRoute{Origin: "static", Source: "static", Family: family.IPv4Unicast}

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
			rule:  ImportRule{Source: "static", Destination: "ospf", MatchTag: true, Tag: 42},
			route: tagged,
			want:  true,
		},
		{
			name:  "different tag rejects",
			rule:  ImportRule{Source: "static", Destination: "ospf", MatchTag: true, Tag: 43},
			route: tagged,
			want:  false,
		},
		{
			name:  "a tag filter rejects an untagged route",
			rule:  ImportRule{Source: "static", Destination: "ospf", MatchTag: true, Tag: 42},
			route: untagged,
			want:  false,
		},
		{
			name:  "tag 0 selects the untagged route",
			rule:  ImportRule{Source: "static", Destination: "ospf", MatchTag: true},
			route: untagged,
			want:  true,
		},
		{
			name:  "tag 0 rejects a tagged route",
			rule:  ImportRule{Source: "static", Destination: "ospf", MatchTag: true},
			route: tagged,
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rule.Accept(tc.route, "ospf"); got != tc.want {
				t.Fatalf("Accept = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEvaluatorRulesCopiesTag proves the diagnostic copy carries the tag filter, so a
// caller reading Rules() cannot report a rule as unfiltered when it is not.
func TestEvaluatorRulesCopiesTag(t *testing.T) {
	ev := NewEvaluator([]ImportRule{{Source: "static", Destination: "ospf", MatchTag: true, Tag: 42}})
	out := ev.Rules()
	if len(out) != 1 {
		t.Fatalf("Rules() returned %d rules, want 1", len(out))
	}
	if !out[0].MatchTag || out[0].Tag != 42 {
		t.Fatalf("Rules() = %+v, want the tag filter preserved", out[0])
	}
}
