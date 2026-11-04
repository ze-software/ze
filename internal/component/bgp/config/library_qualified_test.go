package bgpconfig

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type qualifiedRejectCaller struct{ calls []string }

func (c *qualifiedRejectCaller) FilterDeclaration(plugin, filter string) (policychain.ExternalDeclaration, bool) {
	return policychain.ExternalDeclaration{Reads: []string{"prefix"}, OnError: rpc.OnErrorReject}, plugin == "ext" && filter == "ALLOW-65010"
}

func (c *qualifiedRejectCaller) CallFilter(plugin string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	input, err := route.Input()
	if err != nil {
		return nil, err
	}
	c.calls = append(c.calls, plugin+":"+input.Filter)
	return &rpc.FilterUpdateOutput{Action: rpc.FilterReject}, nil
}

// TestQualifiedExternalChainCannotRunLocalSuffix loads a real match instance
// beside an external filter of the same suffix. Both static compilation and
// dynamic CompileResolved must retain the qualifier: the external rejection
// governs, and the trace must never report executing the local match.
func TestQualifiedExternalChainCannotRunLocalSuffix(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		name := "static"
		if dynamic {
			name = "resolved dynamic"
		}
		t.Run(name, func(t *testing.T) {
			clearPending(t)
			declareRuntime(t, "ext", []rpc.FilterDecl{{Name: "ALLOW-65010", Direction: rpc.FilterImport, Reads: []string{"prefix"}}})
			var compiled filterapi.Policy
			if dynamic {
				groups, err := loadTemplated(t, "ext:ALLOW-$remote_as", "")
				if err != nil {
					t.Fatal(err)
				}
				template := groups[0].Settings.ImportPolicy
				refs := []policy.Ref{{Name: "ext:ALLOW-65010"}}
				compiled, err = template.WithBoundFilters(refs, template.Bindings()).CompileResolved(filterapi.PeerFilterInfo{PeerAS: 65010})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				text := `policy {
 prefix-list ALLOW-LIST-65010 { entry 203.0.113.0/24 { action accept; } }
 match ALLOW-65010 { prefix-list ALLOW-LIST-65010; }
}
` + strings.ReplaceAll(externalDirectionConfig, "%DIRECTION%", "import")
				text = strings.ReplaceAll(text, "ext:65010", "ext:ALLOW-65010")
				tree, err := config.ParseTreeWithYANG(text, nil)
				if err != nil {
					t.Fatal(err)
				}
				peers, err := PeersFromConfigTree(tree)
				if err != nil {
					t.Fatal(err)
				}
				if len(peers) != 1 {
					t.Fatalf("peers = %d, want one", len(peers))
				}
				compiled = peers[0].ImportPolicy
			}
			chain, ok := compiled.Library().(*policychain.Chain)
			if !ok {
				t.Fatalf("library = %T", compiled.Library())
			}
			caller := new(qualifiedRejectCaller)
			view := new(policyview.View)
			view.Reset(updateWithLocalPref(t), policyview.Session{})
			defer view.Clear()
			sets := new(policychain.RunSets)
			sets.SetRoute(policychain.ExternalRoute{Caller: caller, Direction: "import"})
			var ran []string
			outcome := chain.Trace(view, sets, func(step policychain.MemberTrace) { ran = append(ran, step.Name) })
			if outcome.Action != policychain.OutcomeReject || outcome.Failed {
				t.Errorf("outcome = %+v, want external rejection", outcome)
			}
			if len(caller.calls) != 1 {
				t.Errorf("external calls = %v, want exactly one", caller.calls)
			}
			if len(ran) != 1 || ran[0] != "ext:ALLOW-65010" {
				t.Errorf("executed members = %v, want only ext:ALLOW-65010", ran)
			}
		})
	}
}
