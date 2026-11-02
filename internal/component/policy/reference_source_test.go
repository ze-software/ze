package policy_test

import (
	"slices"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/config"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community_match"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_prefix"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/plugins/isis"
	_ "github.com/ze-software/ze/internal/plugins/policyroute"
)

// referencePolicy defines one instance of each case the source tells apart: a
// prefix-list and community-match with their executable match wrappers, a
// match both IS-IS and BGP refuse for the tag it reads, and a policy route
// entry, which is no filter at all.
const referencePolicy = `
policy {
	prefix-list P {
		entry 192.0.2.0/24 { action accept; }
	}
	community-match C {
		entry no-export { type standard; action reject; }
	}
	match FROM-P {
		prefix-list P;
	}
	match FROM-C {
		community C;
	}
	match TAGGED {
		tag [ 100 ];
	}
	route R {
		interface "eth*";
	}
}
`

// referenceRoot parses text through the registered YANG modules and lowers it
// as commit and completion hand it to a ze:reference source.
func referenceRoot(t *testing.T, text string) map[string]any {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(text, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tree.ToPluginMap()
}

func referenceValidator(t *testing.T) *yang.Validator {
	t.Helper()
	resolved, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return yang.NewValidator(resolved)
}

// TestPolicyFilterReferenceSource proves the policy-filter source, through the
// shipped YANG of policy, IS-IS and BGP: completion offers executable instances
// whose fields fit the attachment, and reference validation uses its owner's
// check. Plain and qualified match names pass, as does an external name pending
// its Stage 1 declaration; obsolete direct BGP filter names, incompatible fields
// and policy route entries are refused individually.
func TestPolicyFilterReferenceSource(t *testing.T) {
	v := referenceValidator(t)
	root := referenceRoot(t, referencePolicy+`
isis {
	filter { import { final accept; } }
}
bgp {
	peer peer1 {
		filter { import { final accept; } }
	}
}
`)

	isis := v.ReferenceInstances(root, []string{"isis", "filter", "import", "filters"})
	if !slices.Equal(isis, []string{"FROM-P", "P"}) {
		t.Errorf("isis filter import offers %v, want [FROM-P P]", isis)
	}
	bgp := v.ReferenceInstances(root, []string{"bgp", "peer", "peer1", "filter", "import", "filters"})
	if !slices.Equal(bgp, []string{"FROM-C", "FROM-P"}) {
		t.Errorf("bgp peer filter import offers %v, want [FROM-C FROM-P]", bgp)
	}
	for _, offered := range [][]string{isis, bgp} {
		for _, excluded := range []string{"C", "TAGGED", "R"} {
			if slices.Contains(offered, excluded) {
				t.Errorf("ineligible instance %q was offered as a filter: %v", excluded, offered)
			}
		}
	}

	valid := referenceRoot(t, referencePolicy+`
isis {
	filter { import { filters [ P FROM-P ]; final accept; } }
}
bgp {
	peer peer1 {
		filter { import { filters [ FROM-P FROM-C match:FROM-P match:FROM-C external-plugin:any ]; final accept; } }
	}
}
`)
	// An undeclared external name passes reference validation only; Stage 1
	// must still declare the filter and its fields before the chain can run.
	if refused := v.ValidateReferences(valid); len(refused) != 0 {
		t.Fatalf("valid chain references were refused: %v", refused)
	}
	for _, tc := range []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "obsolete-prefix-list",
			config: `bgp { peer peer1 { filter { import { filters [ prefix-list:P ]; final accept; } } } }`,
			want:   `no BGP filter serves the prefix-list type`,
		},
		{
			name:   "obsolete-community-plugin",
			config: `bgp { peer peer1 { filter { import { filters [ bgp-filter-community-match:C ]; final accept; } } } }`,
			want:   `plugin "bgp-filter-community-match" serves no policy filter`,
		},
		{
			name:   "isis-community-fields",
			config: `isis { filter { import { filters [ C ]; final accept; } } }`,
			want:   `isis import filter "C" (community-match) reads community`,
		},
		{
			name:   "bgp-tag-field",
			config: `bgp { peer peer1 { filter { import { filters [ TAGGED ]; final accept; } } } }`,
			want:   `reads tag`,
		},
		{
			name:   "policy-route-not-filter",
			config: `bgp { peer peer1 { filter { import { filters [ R ]; final accept; } } } }`,
			want:   `"R" is a policy route entry`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := referenceRoot(t, referencePolicy+tc.config)
			refused := v.ValidateReferences(candidate)
			if len(refused) != 1 {
				t.Fatalf("want exactly one reference refusal, got %d: %v", len(refused), refused)
			}
			if !strings.Contains(refused[0].Message, tc.want) {
				t.Errorf("refusal %q lacks %q", refused[0].Message, tc.want)
			}
		})
	}
}
