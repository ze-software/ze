// Design: docs/architecture/config/syntax.md -- explicit reload source fixtures
package bgpconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func newReloadFileStore(t *testing.T, path string) storage.Storage {
	t.Helper()
	store, err := storage.Create(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return storage.BindConfigSource(store, path, storage.ConfigSourceFile)
}

func TestReactorFactoryUsesCandidateThenAcceptedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "router.conf")
	store := newReloadFileStore(t, path)
	boot := []byte("system { host-name router; }")
	if _, _, err := storage.EnsureActiveVersion(store, path, boot, time.Now()); err != nil {
		t.Fatal(err)
	}
	candidate := []byte(`bgp {
		router-id 10.0.0.1;
		session { asn { local 65000; } }
		peer transit {
			connection {
				remote { ip 192.0.2.1; }
				local { ip 192.168.1.1; }
			}
			session { asn { remote 65001; } }
		}
	}`)
	if _, err := storage.WriteCandidateVersion(store, path, candidate, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Neither the original bootstrap nor a later loose-file edit contains BGP.
	if err := os.WriteFile(path, boot, 0o600); err != nil {
		t.Fatal(err)
	}
	// The caller selects fresh authority in each phase; the factory receives
	// only that canonical map, never a cached candidate or bootstrap bytes.
	coord := plugin.NewCoordinator(nil)
	coord.SetBootstrap(registry.BGPBootstrap{ConfigPath: path, Store: store})
	for _, phase := range []string{"candidate", "accepted"} {
		t.Run(phase, func(t *testing.T) {
			content, err := storage.ReadReloadConfig(store, path)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := config.ReadConfig(string(content), path, nil)
			if err != nil {
				t.Fatal(err)
			}
			coord.SetConfigTree(loaded.Tree.ToRunningMap())
			handle, err := createReactorFromCoordinator(coord, coord.GetConfigTree())
			if err != nil {
				t.Fatal(err)
			}
			reactorHandle, ok := handle.(*reactor.Reactor)
			if !ok {
				t.Fatalf("createReactorFromCoordinator returned %T, want *reactor.Reactor", handle)
			}
			peers := reactorHandle.Peers()
			if len(peers) != 1 || peers[0].Settings().PeerAS != 65001 {
				t.Fatal("reactor did not load the candidate peer")
			}
		})
		if phase == "candidate" {
			if err := storage.WriteConfigFile(store, path, candidate); err != nil {
				t.Fatal(err)
			}
			if err := storage.PromoteCandidate(store, path); err != nil {
				t.Fatal(err)
			}
		}
	}
}
