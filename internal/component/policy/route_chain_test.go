package policy_test

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestCompileRouteChain proves the one exported compile path every engine
// chain takes (the redistribute import and IGP export attachments call it):
// it compiles named instances from the policy root, runs them in order with
// their sets, applies final after the last, and refuses an absent final, an
// unknown name and a <plugin>:<filter> reference by name.
func TestCompileRouteChain(t *testing.T) {
	tree, err := config.ParseTreeWithYANG(matchPolicy, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	policyCfg := tree.GetContainer(policy.RootName).ToPluginMap()

	tagged := ribpolicy.Route{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24"), Tag: 4242, HasTag: true}
	untagged := ribpolicy.Route{Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.114.0/24")}

	chain, err := policy.CompileRouteChain("ospf import", []string{"LIST-TAG"}, policy.FinalReject, policyCfg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := chain.Evaluate(untagged).Action; got != ribpolicy.ActionReject {
		t.Fatalf("a route LIST-TAG does not match: action %v, want reject", got)
	}
	if got := chain.Evaluate(tagged).Action; got != ribpolicy.ActionReject {
		t.Fatalf("final reject after a passing filter: action %v, want reject", got)
	}

	chain, err = policy.CompileRouteChain("ospf import", []string{"LIST-TAG"}, policy.FinalAccept, policyCfg)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := chain.Evaluate(tagged).Action; got != ribpolicy.ActionAccept {
		t.Fatalf("final accept after a passing filter: action %v, want accept", got)
	}

	refusals := []struct {
		name  string
		refs  []string
		final policy.Final
		want  string
	}{
		{"no final", []string{"LIST-TAG"}, 0, "final"},
		{"unknown instance", []string{"MISSING-INSTANCE"}, policy.FinalAccept, "MISSING-INSTANCE"},
		{"plugin reference", []string{"rpki:valid"}, policy.FinalAccept, "<plugin>:<filter>"},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			_, err := policy.CompileRouteChain("ospf import", tc.refs, tc.final, policyCfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("CompileRouteChain error = %v, want one naming %q", err, tc.want)
			}
		})
	}
}
