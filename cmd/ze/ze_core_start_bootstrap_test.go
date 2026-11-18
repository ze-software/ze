// Design: docs/architecture/fleet-config.md -- first start of a managed client

//go:build ze_core

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/component/pki"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	bootClient = "edge-01"
	bootToken  = "edge-01-secret-of-at-least-32-chars"
	bootConfig = "bgp { router-id 192.0.2.1; }\n"
)

// bootStore creates an empty live store for one test.
func bootStore(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test cleanup
	return store
}

func writeKey(t *testing.T, store storage.Storage, key, value string) {
	t.Helper()
	if err := store.WriteKey(key, []byte(value)); err != nil {
		t.Fatalf("write %s: %v", key, err)
	}
}

// bootHub starts a real managed server that issues its certificate from a
// fresh authority and serves bootConfig to bootClient. It returns the hub's
// address and its authority.
func bootHub(t *testing.T) (string, *pki.Root) {
	t.Helper()
	root, err := pki.LoadOrGenerateRoot(bootStore(t))
	if err != nil {
		t.Fatalf("hub authority: %v", err)
	}
	srv, err := pluginserver.NewManagedServer(pluginserver.ManagedServerConfig{
		Authority:  root,
		Listeners:  []pluginserver.ManagedListener{{Addr: "127.0.0.1:0", Clients: map[string]string{bootClient: bootToken}}},
		ReadConfig: func(string) ([]byte, error) { return []byte(bootConfig), nil },
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(srv.Stop)
	return srv.Addrs()[0].String(), root
}

// TestManagedBootstrapReadsWhatInitStored: the first start takes the hub
// address, the client name and the secret from the keys ze init --managed
// wrote, with no flag and no environment.
func TestManagedBootstrapReadsWhatInitStored(t *testing.T) {
	store := bootStore(t)
	writeKey(t, store, zefs.KeyManagedHubAddress.Pattern, "10.0.0.1:1791")
	writeKey(t, store, zefs.KeyInstanceName.Pattern, bootClient)
	writeKey(t, store, zefs.KeyManagedHubToken.Pattern, bootToken)

	boot, err := readManagedBootstrap(store)
	if err != nil {
		t.Fatalf("readManagedBootstrap: %v", err)
	}
	if boot.server != "10.0.0.1:1791" || boot.name != bootClient || boot.token != bootToken {
		t.Errorf("bootstrap = %+v", boot)
	}
	if missing := boot.missing(); missing != nil {
		t.Errorf("a complete bootstrap reports something missing: %v", missing)
	}
	if boot.root != nil || boot.firstUse {
		t.Errorf("no trust was stored, got root=%v firstUse=%v", boot.root != nil, boot.firstUse)
	}
}

// TestManagedBootstrapFlagsWin: --server, --name and --token replace what the
// store holds, each alone.
func TestManagedBootstrapFlagsWin(t *testing.T) {
	store := bootStore(t)
	writeKey(t, store, zefs.KeyManagedHubAddress.Pattern, "10.0.0.1:1791")
	writeKey(t, store, zefs.KeyInstanceName.Pattern, bootClient)
	writeKey(t, store, zefs.KeyManagedHubToken.Pattern, bootToken)
	setManagedOverride(t, "ze.managed.server", "192.0.2.9:1799")

	boot, err := readManagedBootstrap(store)
	if err != nil {
		t.Fatalf("readManagedBootstrap: %v", err)
	}
	if boot.server != "192.0.2.9:1799" || boot.name != bootClient || boot.token != bootToken {
		t.Errorf("bootstrap = %+v", boot)
	}
}

// TestManagedBootstrapNamesWhatIsMissing: a first start that cannot connect
// names the value it lacks and the command that gives it, one at a time.
func TestManagedBootstrapNamesWhatIsMissing(t *testing.T) {
	for _, tc := range []struct {
		name string
		boot managedBootstrap
		fix  string
	}{
		{"no hub", managedBootstrap{name: bootClient, token: bootToken}, "--server"},
		{"no name", managedBootstrap{server: "10.0.0.1:1791", token: bootToken}, "--name"},
		{"no secret", managedBootstrap{server: "10.0.0.1:1791", name: bootClient}, "--token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			missing := tc.boot.missing()
			if missing == nil {
				t.Fatal("an incomplete bootstrap reports nothing missing")
			}
			if !strings.Contains(missing.Error(), tc.fix) {
				t.Errorf("%q does not name %s", missing.Error(), tc.fix)
			}
		})
	}
}

// TestManagedBootstrapTrust: a stored root is the anchor and first use is
// then not asked for, whatever the trust key says; with no root the trust key
// decides.
func TestManagedBootstrapTrust(t *testing.T) {
	_, root := bootHub(t)

	t.Run("stored root", func(t *testing.T) {
		store := bootStore(t)
		writeKey(t, store, zefs.KeyManagedHubCA.Pattern, string(root.CertificatePEM()))
		writeKey(t, store, zefs.KeyManagedHubTrust.Pattern, zefs.ManagedHubTrustFirstUse)
		boot, err := readManagedBootstrap(store)
		if err != nil {
			t.Fatalf("readManagedBootstrap: %v", err)
		}
		if boot.root == nil || boot.firstUse {
			t.Errorf("root=%v firstUse=%v, want the stored root and no first use", boot.root != nil, boot.firstUse)
		}
	})
	t.Run("first use", func(t *testing.T) {
		store := bootStore(t)
		writeKey(t, store, zefs.KeyManagedHubTrust.Pattern, zefs.ManagedHubTrustFirstUse)
		boot, err := readManagedBootstrap(store)
		if err != nil {
			t.Fatalf("readManagedBootstrap: %v", err)
		}
		if boot.root != nil || !boot.firstUse {
			t.Errorf("root=%v firstUse=%v, want first use", boot.root != nil, boot.firstUse)
		}
	})
	t.Run("a root that is not a certificate", func(t *testing.T) {
		store := bootStore(t)
		writeKey(t, store, zefs.KeyManagedHubCA.Pattern, "not a certificate")
		if _, err := readManagedBootstrap(store); err == nil {
			t.Error("an unreadable stored root was accepted")
		}
	})
}

// TestFirstStartAgainstAHubWithItsOwnAuthority is the first start, end to
// end against a real hub that serves a certificate from its own authority.
//
// Method, four first starts against the same hub:
//   - with the hub's root stored (ze init --hub-ca): the config is fetched and
//     nothing is kept, because nothing was learned;
//   - with ANOTHER authority's root stored: refused, which is the impostor
//     case, and the control for the first;
//   - with first use asked for: the config is fetched and the authority kept
//     is the hub's own root, byte for byte;
//   - with neither: refused, because the system pool does not know the hub.
func TestFirstStartAgainstAHubWithItsOwnAuthority(t *testing.T) {
	addr, root := bootHub(t)
	_, other := bootHub(t)
	boot := managedBootstrap{server: addr, name: bootClient, token: bootToken}

	t.Run("stored root", func(t *testing.T) {
		with := boot
		with.root = root.Certificate()
		data, kept, err := fetchInitialConfig(with)
		if err != nil {
			t.Fatalf("fetchInitialConfig: %v", err)
		}
		if string(data) != bootConfig {
			t.Errorf("config = %q", data)
		}
		if kept != nil {
			t.Error("a connection anchored on a stored root kept an authority")
		}
	})
	t.Run("another authority's root", func(t *testing.T) {
		with := boot
		with.root = other.Certificate()
		if _, _, err := fetchInitialConfig(with); err == nil {
			t.Fatal("a hub the stored root did not issue was accepted")
		}
	})
	t.Run("first use", func(t *testing.T) {
		with := boot
		with.firstUse = true
		data, kept, err := fetchInitialConfig(with)
		if err != nil {
			t.Fatalf("fetchInitialConfig: %v", err)
		}
		if string(data) != bootConfig {
			t.Errorf("config = %q", data)
		}
		if kept == nil {
			t.Fatal("first use kept no authority")
		}
		if !kept.Equal(root.Certificate()) {
			t.Errorf("first use kept %q, want the hub's root %q", kept.Subject, root.Certificate().Subject)
		}
	})
	t.Run("no trust", func(t *testing.T) {
		if _, _, err := fetchInitialConfig(boot); err == nil {
			t.Fatal("a hub with its own authority was accepted with no root and no first use")
		}
	})
}

// TestKeptHubRootAnchorsTheNextConnection: the authority a first-use
// connection kept is stored, read back as the client's hub root, and
// authenticates the hub on a connection that asks for no first use. A
// different hub is then refused against it: first use happens once.
func TestKeptHubRootAnchorsTheNextConnection(t *testing.T) {
	addr, _ := bootHub(t)
	otherAddr, _ := bootHub(t)
	store := bootStore(t)
	boot := managedBootstrap{server: addr, name: bootClient, token: bootToken, firstUse: true}

	_, kept, err := fetchInitialConfig(boot)
	if err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := keepHubRoot(store, kept); err != nil {
		t.Fatalf("keepHubRoot: %v", err)
	}

	stored := storedHubRoot(store)
	if stored == nil {
		t.Fatal("the kept authority is not in the store")
	}
	block, _ := pem.Decode(mustRead(t, store, zefs.KeyManagedHubCA.Pattern))
	if block == nil {
		t.Fatal("the stored authority is not PEM")
	}
	if _, parseErr := x509.ParseCertificate(block.Bytes); parseErr != nil {
		t.Fatalf("the stored authority is not a certificate: %v", parseErr)
	}

	again, err := readManagedBootstrap(store)
	if err != nil {
		t.Fatalf("readManagedBootstrap: %v", err)
	}
	if again.firstUse {
		t.Error("a client holding a root still asks for first use")
	}
	next := managedBootstrap{server: addr, name: bootClient, token: bootToken, root: stored}
	if _, _, fetchErr := fetchInitialConfig(next); fetchErr != nil {
		t.Errorf("the kept authority does not authenticate its own hub: %v", fetchErr)
	}
	next.server = otherAddr
	if _, _, fetchErr := fetchInitialConfig(next); fetchErr == nil {
		t.Error("a second hub was accepted against the authority kept from the first")
	}
}

func mustRead(t *testing.T, store storage.Storage, key string) []byte {
	t.Helper()
	data, err := store.ReadFile(key)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return data
}

// TestFirstUseRefusesAHubThatPresentsNoAuthority: first use keeps an
// authority, so a hub that presents its certificate alone gives nothing to
// keep and is refused, before the secret is sent.
func TestFirstUseRefusesAHubThatPresentsNoAuthority(t *testing.T) {
	root, err := pki.LoadOrGenerateRoot(bootStore(t))
	if err != nil {
		t.Fatalf("authority: %v", err)
	}
	leaf, err := root.IssueLeaf("ze-managed-hub", []string{"127.0.0.1"})
	if err != nil {
		t.Fatalf("IssueLeaf: %v", err)
	}
	parsed, err := x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	conf, kept := managed.FirstUseTLSConfig("127.0.0.1:1791")
	state := tlsStateWith(parsed)
	if verifyErr := conf.VerifyConnection(state); !errors.Is(verifyErr, managed.ErrHubPresentedNoAuthority) {
		t.Errorf("err = %v, want managed.ErrHubPresentedNoAuthority", verifyErr)
	}
	if kept() != nil {
		t.Error("an authority was kept from a refused handshake")
	}
}

// tlsStateWith is the connection state of a hub that presented chain.
func tlsStateWith(chain ...*x509.Certificate) tls.ConnectionState {
	return tls.ConnectionState{PeerCertificates: chain}
}
