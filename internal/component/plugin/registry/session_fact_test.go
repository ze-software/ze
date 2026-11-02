package registry

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// testSessionFact is the fact the session condition tests name. It is
// registered once per test binary: a second registration is an owner's defect
// and panics, so a repeated run (-count) finds it registered.
const testSessionFact = "test-session-fact"

func registerTestSessionFact() {
	if !SessionFactRegistered(testSessionFact) {
		RegisterSessionFact(testSessionFact)
	}
}

// TestSessionConditionDeclarationJudged checks the Stage 1 rules of a session
// condition (spec-filter-wire-6-stage-filters AC-2): a condition whose facts
// some owner registered is accepted in place of config; a fact no owner
// registered, a fact named twice, values beside the session condition, and a
// declaration carrying both conditions are each refused, naming the plugin
// and the rule.
func TestSessionConditionDeclarationJudged(t *testing.T) {
	registerTestSessionFact()
	filters := []rpc.FilterDecl{{Name: "otc", Direction: rpc.FilterBoth}}
	valid := func() rpc.FilterInsertion {
		return rpc.FilterInsertion{Filter: "bgp-x:otc", Direction: rpc.FilterExport, Priority: 10,
			Session: []rpc.FilterSessionFact{{Fact: testSessionFact, Values: []string{"false"}}}}
	}
	if err := CheckFilterInsertions("bgp-x", filters, nil, []rpc.FilterInsertion{valid()}); err != nil {
		t.Fatalf("a valid session condition was refused: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*rpc.FilterInsertion)
		want   string
	}{
		{"an unregistered fact", func(i *rpc.FilterInsertion) { i.Session[0].Fact = "no-such-fact" }, "no owner registered"},
		{"a fact twice", func(i *rpc.FilterInsertion) { i.Session = append(i.Session, i.Session[0]) }, "twice"},
		{"values beside session", func(i *rpc.FilterInsertion) { i.Values = []string{"on"} }, "values beside"},
		{"both conditions", func(i *rpc.FilterInsertion) { i.Config = []string{"role"} }, "two conditions"},
		{"neither condition", func(i *rpc.FilterInsertion) { i.Session = nil }, "no condition"},
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
