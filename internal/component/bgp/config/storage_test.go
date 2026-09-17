// Design: docs/architecture/config/syntax.md -- explicit reload source fixtures
package bgpconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	internalresolve "github.com/ze-software/ze/internal/core/resolve"
)

func newReloadFileStore(t *testing.T, path string) storage.Storage {
	t.Helper()
	store, err := storage.Create(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return internalresolve.BindConfigSource(store, path, internalresolve.ConfigSourceFile)
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
	coord := plugin.NewCoordinator(nil)
	coord.SetBootstrap(registry.BGPBootstrap{ConfigPath: path, ConfigData: boot, Store: store})
	for _, phase := range []string{"candidate", "accepted"} {
		t.Run(phase, func(t *testing.T) {
			handle, err := createReactorFromCoordinator(coord)
			if err != nil {
				t.Fatal(err)
			}
			peers := handle.(*reactor.Reactor).Peers()
			if len(peers) != 1 || peers[0].Settings().PeerAS != 65001 {
				t.Fatal("reactor did not load the candidate peer")
			}
		})
		if phase == "candidate" {
			if err := storage.PromoteCandidate(store, path); err != nil {
				t.Fatal(err)
			}
		}
	}
}
