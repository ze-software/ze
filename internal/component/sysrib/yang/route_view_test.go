// Design: docs/plugin-development/protocol.md -- native routing-view schema.

package yang

import (
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewSchemaRegisteredAndBounded(t *testing.T) {
	schema, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	publication, err := configyang.PublishedRPCs(schema)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		rpc.MethodRouteViewOpen:       false,
		rpc.MethodRouteViewRead:       false,
		rpc.MethodRouteViewCandidates: false,
		rpc.MethodRouteViewDistances:  false,
		rpc.MethodRouteViewPublish:    false,
		rpc.MethodRouteViewClose:      false,
	}
	for _, method := range publication.Protocol {
		if method.Module != "ze-sysrib-api" {
			continue
		}
		if _, ok := want[method.WireMethod]; !ok {
			t.Fatalf("unexpected owner method %q", method.WireMethod)
		}
		if want[method.WireMethod] {
			t.Fatalf("duplicate owner method %q", method.WireMethod)
		}
		want[method.WireMethod] = true
	}
	for method, found := range want {
		if !found {
			t.Fatalf("native schema did not publish %s", method)
		}
	}
	module := schema.GetEntry("ze-sysrib-api")
	if module == nil {
		t.Fatal("system RIB API module is not registered")
	}
	read := module.Dir["route-view-read"]
	if read == nil || read.RPC == nil || read.RPC.Input == nil || read.RPC.Output == nil {
		t.Fatal("selected-view input or output schema is missing")
	}
	for name, bound := range map[string]string{"limit": "1..4096", "wait-ms": "0..1000"} {
		leaf := read.RPC.Input.Dir[name]
		if leaf == nil || leaf.Type == nil || leaf.Type.Range.String() != bound || leaf.Mandatory != gyang.TSTrue {
			t.Fatalf("input %s does not retain mandatory native bound %s", name, bound)
		}
	}
	for _, name := range []string{"session", "epoch", "cursor", "phase", "complete"} {
		leaf := read.RPC.Output.Dir[name]
		if leaf == nil || leaf.Mandatory != gyang.TSTrue {
			t.Fatalf("output %s is not mandatory", name)
		}
	}
	entries := read.RPC.Output.Dir["entries"]
	if entries == nil || entries.ListAttr == nil || entries.ListAttr.MaxElements != rpc.RouteViewRecordsMax {
		t.Fatal("selected-view records are not bounded by the wire maximum")
	}
}
