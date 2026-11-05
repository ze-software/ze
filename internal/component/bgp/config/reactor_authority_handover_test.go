// Design: docs/architecture/core-design.md -- one phase-owned running authority.
package bgpconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func handoverConfig(t *testing.T, disabled, localAS string) (string, map[string]any) {
	t.Helper()
	text := strings.Replace(insertionConfig, "%A%", "filter { import { filters [ inactive:third-party:"+disabled+" ]; final accept; } }", 1)
	text = strings.Replace(text, "local 65000", "local "+localAS, 1)
	loaded, err := config.LoadConfig(text, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return text, loaded.Tree.ToRunningMap()
}

func prepareHandoverConfig(t *testing.T) {
	t.Helper()
	t.Cleanup(configyang.RegisterModuleForTest("ze-test-inserter-conf.yang", insertionTestYANG))
	registry.SetRuntimeLoadedPlugins(func() []string { return []string{"third-party"} })
	t.Cleanup(func() { registry.SetRuntimeLoadedPlugins(nil) })
}

func handoverBootstrap(t *testing.T, boot string) registry.BGPBootstrap {
	t.Helper()
	path := filepath.Join(t.TempDir(), "router.conf")
	if err := os.WriteFile(path, []byte(boot), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newReloadFileStore(t, path)
	if _, _, err := storage.EnsureActiveVersion(store, path, []byte(boot), time.Now()); err != nil {
		t.Fatal(err)
	}
	return registry.BGPBootstrap{ConfigPath: path, Store: store}
}

func assertHandoverReactor(t *testing.T, handle registry.BGPReactorHandle, want map[string]any, localAS uint32) *reactor.Reactor {
	t.Helper()
	r, ok := handle.(*reactor.Reactor)
	if !ok {
		t.Fatalf("factory returned %T, want actual reactor", handle)
	}
	if !reflect.DeepEqual(r.ConfigTree(), want) {
		t.Error("factory did not retain the exact selected running authority, including inactive intent")
	}
	for _, peer := range r.Peers() {
		if peer.Settings().GlobalLocalAS != localAS {
			t.Errorf("whole constructor used global AS %d, want selected %d", peer.Settings().GlobalLocalAS, localAS)
		}
	}
	if len(r.Peers()) == 0 {
		t.Fatal("actual constructor produced no peers")
	}
	return r
}

// The selected candidate deliberately differs from stored boot and loose bytes.
// Checking real inherited globals as well as the map refuses a file-global /
// map-peer hybrid, not merely a late replacement of the map after construction.
func TestReactorFactoryConsumesWholeSelectedAuthority(t *testing.T) {
	prepareHandoverConfig(t)
	bootText, boot := handoverConfig(t, "late", "65000")
	_, candidate := handoverConfig(t, "early", "65123")
	coord := plugin.NewCoordinator(boot)
	coord.SetBootstrap(handoverBootstrap(t, bootText))
	coord.SetConfigTree(candidate)
	handle, err := createReactorFromCoordinator(coord, candidate)
	if err != nil {
		t.Fatal(err)
	}
	r := assertHandoverReactor(t, handle, candidate, 65123)
	if err := coord.AttachReactor(r.ReactorLifecycleAdapter()); err != nil {
		t.Fatal(err)
	}
	if err := r.RebuildRunningPeers(); err != nil {
		t.Fatal(err)
	}
	assertHandoverReactor(t, handle, candidate, 65123)

	t.Run("missing authority refuses despite valid boot bytes", func(t *testing.T) {
		absent := plugin.NewCoordinator(nil)
		absent.SetBootstrap(handoverBootstrap(t, bootText))
		if _, err := createReactorFromCoordinator(absent, nil); err == nil {
			t.Error("missing selected authority was silently replaced by boot bytes")
		}
	})
}

// A real accepted map update must survive unload/recreate. The ordinary startup
// factory may not resurrect bootstrap intent; compensation selects its own old
// map explicitly and must rebuild that complete old authority instead.
func TestReactorDetachRecreateUsesCurrentOrCompensationAuthority(t *testing.T) {
	prepareHandoverConfig(t)
	bootText, boot := handoverConfig(t, "late", "65000")
	_, accepted := handoverConfig(t, "early", "65123")
	coord := plugin.NewCoordinator(boot)
	coord.SetBootstrap(handoverBootstrap(t, bootText))
	handle, err := createReactorFromCoordinator(coord, boot)
	if err != nil {
		t.Fatal(err)
	}
	owner := handle.ReactorLifecycleAdapter()
	if err := coord.AttachReactor(owner); err != nil {
		t.Fatal(err)
	}
	coord.SetConfigTree(accepted)
	if err := coord.DetachReactor(owner); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(coord.GetConfigTree(), accepted) {
		t.Error("detach lost latest accepted map and inactive intent")
	}
	for _, phase := range []struct {
		name    string
		tree    map[string]any
		localAS uint32
	}{{"accepted recreation", accepted, 65123}, {"compensation recreation", boot, 65000}} {
		t.Run(phase.name, func(t *testing.T) {
			coord.SetConfigTree(phase.tree)
			next, err := createReactorFromCoordinator(coord, phase.tree)
			if err != nil {
				t.Fatal(err)
			}
			r := assertHandoverReactor(t, next, phase.tree, phase.localAS)
			nextOwner := r.ReactorLifecycleAdapter()
			if err := coord.AttachReactor(nextOwner); err != nil {
				t.Fatal(err)
			}
			if err := r.RebuildRunningPeers(); err != nil {
				t.Fatal(err)
			}
			assertHandoverReactor(t, next, phase.tree, phase.localAS)
			if err := coord.DetachReactor(nextOwner); err != nil {
				t.Fatal(err)
			}
		})
	}
}
