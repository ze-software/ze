package policy_test

import (
	"slices"
	"strings"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/config"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_community_match"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_prefix"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/plugins/isis"
	_ "github.com/ze-software/ze/internal/plugins/policyroute"
)

// referencePolicy defines one instance of each case the source tells apart: a
// prefix-list every attachment here accepts, a community-match IS-IS routes
// cannot feed, a match IS-IS refuses for the tag it reads and BGP refuses
// because no BGP filter serves match, and a policy route entry, which is no
// filter at all.
const referencePolicy = `
policy {
	prefix-list P {
		entry 192.0.2.0/24 { action accept; }
	}
	community-match C {
		entry no-export { type standard; action reject; }
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
// shipped YANG of policy, IS-IS and BGP: completion of a chain offers the union
// of the filter instances of every type whose fields fit that chain's
// attachment, never a policy route entry, and commit judges a value with the
// attachment owner's own check, so a qualified or external BGP name commits
// and an instance whose fields do not fit is refused naming the field.
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
	if !slices.Equal(isis, []string{"P"}) {
		t.Errorf("isis filter import offers %v, want [P]: C reads community and TAGGED reads tag, which IS-IS routes do not carry", isis)
	}
	bgp := v.ReferenceInstances(root, []string{"bgp", "peer", "peer1", "filter", "import", "filters"})
	if !slices.Equal(bgp, []string{"C", "P"}) {
		t.Errorf("bgp peer filter import offers %v, want [C P]: both types fit BGP, and no BGP filter serves match", bgp)
	}
	for _, offered := range [][]string{isis, bgp} {
		if slices.Contains(offered, "R") {
			t.Errorf("a policy route entry was offered as a filter: %v", offered)
		}
	}

	committed := referenceRoot(t, referencePolicy+`
isis {
	filter { import { filters [ C ]; final accept; } }
}
bgp {
	peer peer1 {
		filter { import { filters [ prefix-list:P bgp-filter-community-match:C external-plugin:any ]; final accept; } }
	}
	peer peer2 {
		filter { import { filters [ R ]; final accept; } }
	}
}
`)
	var refused []string
	for _, ve := range v.ValidateReferences(committed) {
		refused = append(refused, ve.Path+": "+ve.Message)
	}
	if len(refused) != 2 {
		t.Fatalf("want exactly the IS-IS C and the BGP R refused, got %d: %v", len(refused), refused)
	}
	joined := strings.Join(refused, "\n")
	for _, want := range []string{
		`isis import filter "C" (community-match) reads community`,
		`"R" is a policy route entry`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("refusals lack %q:\n%s", want, joined)
		}
	}
}
