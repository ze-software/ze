// VALIDATES: an explicitly configured hub server block opens the TLS acceptor
// even when no external plugin is registered (spec-test-coverage-gaps AC-2:
// out-of-process clients such as the .ci engine-step runner authenticate with
// the configured secret; before this, ensureAcceptor returned early and the
// configured listener silently never existed).
// PREVENTS: `plugin { hub { server ... } }` configs where the listener only
// appears once some external plugin happens to be declared.
package plugin

import (
	"testing"

	parent "github.com/ze-software/ze/internal/component/plugin"
)

func TestEnsureAcceptorExplicitHubConfigWithoutExternals(t *testing.T) {
	m := NewManager()
	m.SetHubConfig(&parent.HubConfig{
		Servers: []parent.HubServerConfig{{
			Name:   "local",
			Host:   "127.0.0.1",
			Port:   0, // dynamic
			Secret: "explicit-hub-test-secret",
		}},
	})

	// No external plugins: internal-only config set.
	if err := m.ensureAcceptor([]parent.PluginConfig{{Name: "bgp-rib", Internal: true}}); err != nil {
		t.Fatalf("ensureAcceptor: %v", err)
	}
	t.Cleanup(func() {
		if m.acceptor != nil {
			m.acceptor.Stop()
		}
	})

	if m.acceptor == nil {
		t.Fatal("explicit hub server config must open the acceptor without external plugins")
	}
	if m.acceptor.Addr() == nil {
		t.Fatal("acceptor has no listen address")
	}
}

func TestEnsureAcceptorSkippedWithoutConfigOrExternals(t *testing.T) {
	m := NewManager()
	if err := m.ensureAcceptor([]parent.PluginConfig{{Name: "bgp-rib", Internal: true}}); err != nil {
		t.Fatalf("ensureAcceptor: %v", err)
	}
	if m.acceptor != nil {
		t.Fatal("no hub config and no externals: acceptor must stay nil (auto-config is external-only)")
	}
}
