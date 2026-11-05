// Design: docs/architecture/core-design.md -- inactive automatic filter references.
package bgpconfig

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRunningRebuildRetainsInactiveInsertionsAfterBootAndReload exercises the
// actual coordinator factory, declaration rebuild and current running-config
// publication. The second rebuild must retain the operator's disabled member;
// after reload changes which member is disabled, it must use the current tree,
// not the boot bytes. The other automatic member remains active in each case.
func TestRunningRebuildRetainsInactiveInsertionsAfterBootAndReload(t *testing.T) {
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-inserter-conf.yang", insertionTestYANG))
	registry.SetRuntimeLoadedPlugins(func() []string { return []string{"third-party"} })
	t.Cleanup(func() { registry.SetRuntimeLoadedPlugins(nil) })
	declared := false
	registry.SetRuntimeFilterInsertions(func() []registry.ProcessFilterInsertions {
		if !declared {
			return nil
		}
		return []registry.ProcessFilterInsertions{{Process: "third-party", Insertions: []rpc.FilterInsertion{
			{Filter: "third-party:late", Direction: rpc.FilterImport, Priority: 100, Config: rrClient},
			{Filter: "third-party:early", Direction: rpc.FilterImport, Priority: 200, Config: rrClient},
		}}}
	})
	t.Cleanup(func() { registry.SetRuntimeFilterInsertions(nil) })
	text := func(disabled string) string {
		return strings.Replace(insertionConfig, "%A%", "filter { import { filters [ inactive:third-party:"+disabled+" ]; final accept; } }", 1)
	}
	boot, err := config.LoadConfig(text("late"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	coord := plugin.NewCoordinator(boot.Tree.ToRunningMap())
	handle, err := createReactorFromCoordinator(coord, coord.GetConfigTree())
	if err != nil {
		t.Fatal(err)
	}
	r, ok := handle.(*reactor.Reactor)
	if !ok {
		t.Fatalf("factory returned %T, want *reactor.Reactor", handle)
	}
	assertMembers := func(t *testing.T, disabled, active string) {
		t.Helper()
		for _, peer := range r.Peers() {
			if peer.Settings().Name != "A" {
				continue
			}
			var inactiveSeen, activeSeen bool
			for _, ref := range peer.Settings().ImportPolicy.Filters() {
				if ref.Name == "third-party:"+disabled {
					if !ref.Inactive {
						t.Errorf("operator-disabled %s became active", ref.Name)
					}
					inactiveSeen = ref.Inactive
				}
				if ref.Name == "third-party:"+active && !ref.Inactive {
					activeSeen = true
				}
			}
			if !inactiveSeen || !activeSeen {
				t.Errorf("disabled %s retained=%t, automatic %s active=%t", disabled, inactiveSeen, active, activeSeen)
			}
			return
		}
		t.Fatal("running peer A is absent")
	}
	declared = true
	for _, phase := range []string{"boot declaration", "unrelated declaration rebuild"} {
		t.Run(phase, func(t *testing.T) {
			if err := r.RebuildRunningPeers(); err != nil {
				t.Fatal(err)
			}
			assertMembers(t, "late", "early")
		})
	}
	loaded, err := config.LoadConfig(text("early"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	api, ok := r.ReactorLifecycleAdapter().(plugin.ReactorLifecycle)
	if !ok {
		t.Fatal("reactor adapter does not implement plugin.ReactorLifecycle")
	}
	api.SetConfigTree(loaded.Tree.ToRunningMap())
	if err := r.RebuildRunningPeers(); err != nil {
		t.Fatal(err)
	}
	assertMembers(t, "early", "late")
}
