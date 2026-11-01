package registry

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterInsertionDeclarationJudged checks every Stage 1 rule an
// auto-insertion is judged by (spec-filter-wire-6-stage-filters AC-2). Method:
// one valid insertion per reference form is accepted, then each case breaks
// exactly one rule and the refusal must name the insertion and the rule.
func TestFilterInsertionDeclarationJudged(t *testing.T) {
	filters := []rpc.FilterDecl{{Name: "otc", Direction: rpc.FilterBoth}}
	types := []string{"reject-asn"}
	valid := func() rpc.FilterInsertion {
		return rpc.FilterInsertion{Filter: "bgp-x:otc", Direction: rpc.FilterImport, Priority: 10,
			Config: []string{"role"}, Required: true, Reason: "RFC 9234 Section 5"}
	}

	if err := CheckFilterInsertions("bgp-x", filters, types, []rpc.FilterInsertion{valid()}); err != nil {
		t.Fatalf("a valid plugin-form insertion was refused: %v", err)
	}
	typed := rpc.FilterInsertion{Filter: "reject-asn:transit-leak", Direction: rpc.FilterExport,
		Priority: rpc.FilterInsertionPriorityMax, Config: []string{"role", "import"}, Values: []string{"peer"}}
	if err := CheckFilterInsertions("bgp-x", filters, types, []rpc.FilterInsertion{typed}); err != nil {
		t.Fatalf("a valid type-form insertion at the last valid priority was refused: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*rpc.FilterInsertion)
		want   string
	}{
		{"another plugin's filter", func(i *rpc.FilterInsertion) { i.Filter = "bgp-y:otc" }, "does not serve"},
		{"an undeclared filter", func(i *rpc.FilterInsertion) { i.Filter = "bgp-x:other" }, "does not serve"},
		{"a type it does not own", func(i *rpc.FilterInsertion) { i.Filter = "prefix-list:A" }, "does not serve"},
		{"no instance", func(i *rpc.FilterInsertion) { i.Filter = "reject-asn:" }, "does not serve"},
		{"a bare name", func(i *rpc.FilterInsertion) { i.Filter = "otc" }, "does not serve"},
		{"both directions", func(i *rpc.FilterInsertion) { i.Direction = rpc.FilterBoth }, "direction"},
		{"no direction", func(i *rpc.FilterInsertion) { i.Direction = 0 }, "direction"},
		{"priority below", func(i *rpc.FilterInsertion) { i.Priority = -1 }, "priority"},
		{"priority above", func(i *rpc.FilterInsertion) { i.Priority = rpc.FilterInsertionPriorityMax + 1 }, "priority"},
		{"no condition", func(i *rpc.FilterInsertion) { i.Config = nil }, "condition"},
		{"an empty path word", func(i *rpc.FilterInsertion) { i.Config = []string{"role", ""} }, "condition"},
		{"required with no reason", func(i *rpc.FilterInsertion) { i.Reason = "" }, "reason"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ins := valid()
			tc.mutate(&ins)
			err := CheckFilterInsertions("bgp-x", filters, types, []rpc.FilterInsertion{ins})
			if err == nil {
				t.Fatalf("accepted %+v", ins)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "bgp-x") {
				t.Fatalf("refusal %q names neither the plugin nor the rule %q", err, tc.want)
			}
		})
	}

	t.Run("duplicate", func(t *testing.T) {
		err := CheckFilterInsertions("bgp-x", filters, types, []rpc.FilterInsertion{valid(), valid()})
		if err == nil || !strings.Contains(err.Error(), "twice") {
			t.Fatalf("a duplicate (filter, direction) was not refused as such: %v", err)
		}
	})
}
