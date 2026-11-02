// Design: docs/architecture/api/process-protocol.md -- Filter Insertion
// Related: filter_insertion.go -- checkInsertionScope, the scope judge

package registry

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAnyPeerScopeDeclarationJudged checks the Stage 1 judge of an
// insertion's scope: `any-peer` is accepted on an export insertion with a
// config condition, and refused on an import insertion, beside a session
// condition, and for a scope no one defined. Method: one valid declaration,
// then one mutation per refusal, each refusal naming the plugin and the rule.
func TestAnyPeerScopeDeclarationJudged(t *testing.T) {
	registerTestSessionFact()
	filters := []rpc.FilterDecl{{Name: "llgr", Direction: rpc.FilterBoth}}
	valid := func() rpc.FilterInsertion {
		return rpc.FilterInsertion{Filter: "bgp-x:llgr", Direction: rpc.FilterExport, Priority: 10,
			Config: []string{"long-lived-stale-time"}, Scope: rpc.FilterScopeAnyPeer}
	}
	if err := CheckFilterInsertions("bgp-x", filters, nil, []rpc.FilterInsertion{valid()}); err != nil {
		t.Fatalf("a valid any-peer insertion was refused: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*rpc.FilterInsertion)
		want   string
	}{
		{"on the import list", func(i *rpc.FilterInsertion) { i.Direction = rpc.FilterImport }, "any-peer on the import list"},
		{"beside a session condition", func(i *rpc.FilterInsertion) {
			i.Config = nil
			i.Session = []rpc.FilterSessionFact{{Fact: testSessionFact}}
		}, "any-peer beside a session condition"},
		{"an unknown scope", func(i *rpc.FilterInsertion) { i.Scope = "every-peer" }, "scope \"every-peer\""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ins := valid()
			tc.mutate(&ins)
			err := CheckFilterInsertions("bgp-x", filters, nil, []rpc.FilterInsertion{ins})
			if err == nil {
				t.Fatalf("accepted %+v", ins)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "bgp-x") {
				t.Fatalf("refusal %q names neither the plugin nor the rule %q", err, tc.want)
			}
		})
	}
}
