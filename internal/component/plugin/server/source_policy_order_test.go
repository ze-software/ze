// Design: docs/architecture/api/process-protocol.md -- source-policy owner order.
package server

import (
	"net"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestSourcePolicyOwnerDependencyOrder proves the coordinator's owner order is
// the plugins' registered dependency order, not name order. Method: three
// owners whose names sort against their dependencies ("a-forwarder" depends on
// "z-rib" and optionally on "m-store"; "m-store" depends on "z-rib") are
// ordered, in each input order. Each owner must come after every owner it
// depends on, so the result is z-rib, m-store, a-forwarder. Independent owners
// keep name order, and a dependency cycle refuses the capture.
func TestSourcePolicyOwnerDependencyOrder(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	run := func(net.Conn) int { return 0 }
	registerLifecyclePluginWithDeps(t, "z-rib", nil, nil, run)
	registerLifecyclePluginWithDeps(t, "m-store", []string{"z-rib"}, nil, run)
	registerLifecyclePluginWithDeps(t, "a-forwarder", []string{"z-rib"}, []string{"m-store"}, run)
	registerLifecyclePluginWithDeps(t, "b-free", nil, nil, run)
	registerLifecyclePluginWithDeps(t, "c-free", nil, nil, run)
	owner := func(name string) *SourcePolicyOwner {
		return &SourcePolicyOwner{name: name, proc: process.NewProcess(plugin.PluginConfig{Name: name, Internal: true})}
	}
	names := func(owners []*SourcePolicyOwner) []string {
		out := make([]string, 0, len(owners))
		for _, o := range owners {
			out = append(out, o.name)
		}
		return out
	}
	want := []string{"z-rib", "m-store", "a-forwarder"}
	for _, input := range [][]string{{"a-forwarder", "m-store", "z-rib"}, {"z-rib", "a-forwarder", "m-store"}} {
		owners := make([]*SourcePolicyOwner, 0, len(input))
		for _, name := range input {
			owners = append(owners, owner(name))
		}
		ordered, err := orderSourcePolicyOwners(owners)
		if err != nil {
			t.Fatal(err)
		}
		if got := names(ordered); !slices.Equal(got, want) {
			t.Fatalf("input %v ordered %v, want %v", input, got, want)
		}
	}
	ordered, err := orderSourcePolicyOwners([]*SourcePolicyOwner{owner("c-free"), owner("b-free")})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(ordered); !slices.Equal(got, []string{"b-free", "c-free"}) {
		t.Fatalf("independent owners ordered %v, want name order", got)
	}
	registerLifecyclePluginWithDeps(t, "x-cycle", []string{"y-cycle"}, nil, run)
	registerLifecyclePluginWithDeps(t, "y-cycle", []string{"x-cycle"}, nil, run)
	if _, err := orderSourcePolicyOwners([]*SourcePolicyOwner{owner("x-cycle"), owner("y-cycle")}); err == nil {
		t.Fatal("a dependency cycle between owners was ordered")
	}
}
