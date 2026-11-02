package rs

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestTransparencyInsertedOnRSClients proves where bgp-rs's RFC 7947
// Section 2.2 step goes: one export insertion of a filter the plugin
// declares, placed by the session fact rs-client being true, not required (the
// operator may run a client's list without it), and the same declarations
// on the compiled registration and on Stage 1, which the daemon judges
// (registry.CheckFilterInsertions). Method: read both declarations.
func TestTransparencyInsertedOnRSClients(t *testing.T) {
	insertions := rsInsertions()
	if len(insertions) != 1 {
		t.Fatalf("insertions = %+v, want one", insertions)
	}
	ins := insertions[0]
	if ins.Filter != rsPluginName+":"+transparencyFilterName {
		t.Errorf("filter = %q", ins.Filter)
	}
	if ins.Direction != rpc.FilterExport {
		t.Errorf("direction = %q, want export", ins.Direction)
	}
	if ins.Required {
		t.Error("the step is required; RFC 7947 Section 2.2 leaves the operator room to run without it")
	}
	if len(ins.Config) != 0 || ins.Scope != rpc.FilterScopePeer {
		t.Errorf("config %v scope %q: want a session condition on the destination itself", ins.Config, ins.Scope)
	}
	want := rpc.FilterSessionFact{Fact: policyview.FieldRSClient.String(), Values: []string{"true"}}
	if len(ins.Session) != 1 || ins.Session[0].Fact != want.Fact || len(ins.Session[0].Values) != 1 || ins.Session[0].Values[0] != "true" {
		t.Errorf("session = %+v, want %+v", ins.Session, want)
	}

	decls := rsFilterDecls()
	if len(decls) != 1 || decls[0].Name != transparencyFilterName || decls[0].Direction != sdk.FilterExport {
		t.Fatalf("filters = %+v, want the transparency export step", decls)
	}
	if len(decls[0].Sets) != 1 || decls[0].Sets[0] != policyview.FieldPreserveOpaqueAttributes.String() {
		t.Errorf("sets = %v, want only preserve-opaque-attributes", decls[0].Sets)
	}
	if err := registry.CheckFilterInsertions(rsPluginName, decls, nil, insertions); err != nil {
		t.Errorf("Stage 1 refuses the declarations: %v", err)
	}

	stage1 := routeServerRegistration()
	if len(stage1.Filters) != 1 || stage1.Filters[0].Name != transparencyFilterName || len(stage1.FilterInsertions) != 1 ||
		stage1.FilterInsertions[0].Filter != ins.Filter {
		t.Errorf("Stage 1 declares %+v / %+v, want the compiled registration's", stage1.Filters, stage1.FilterInsertions)
	}
	reg := registry.Lookup(rsPluginName)
	if reg == nil || len(reg.Filters) != 1 || len(reg.FilterInsertions) != 1 {
		t.Fatalf("compiled registration = %+v, want the step and its insertion", reg)
	}
}

// TestTransparencySetsPreserveOpaque proves the step's answer: every route
// toward a client is accepted with preserve-opaque-attributes set to true,
// the treatment the forward path applies in place of RFC 4271 Section 5's
// handling (reactor forward_role_cache_test.go proves the bytes); an unknown
// filter name is refused, and the evaluator honors the fast-path contract.
func TestTransparencySetsPreserveOpaque(t *testing.T) {
	route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: transparencyFilterName})
	out, err := evaluateRSFilter(&route)
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != sdk.FilterModify || len(out.Sets) != 1 {
		t.Fatalf("answer = %+v, want modify with one set", out)
	}
	set := out.Sets[0]
	if set.Field != policyview.FieldPreserveOpaqueAttributes.String() || set.Action != rpc.FilterSetActionSet || string(set.Value) != "true" {
		t.Errorf("set = %s %s %s, want preserve-opaque-attributes set true", set.Field, set.Action, set.Value)
	}

	unknown := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: "nope"})
	if _, err := evaluateRSFilter(&unknown); err == nil {
		t.Error("an unknown filter name was answered")
	}
	routes := []sdk.ConformanceRoute{{Input: rpc.FilterUpdateInput{Filter: transparencyFilterName}}}
	if !sdk.CheckFilterConformance(t, rsPluginName, evaluateRSFilter, routes) {
		t.Fatal("the transparency evaluator breaks the fast-path contract")
	}
}
