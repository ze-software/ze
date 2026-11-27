// Design: docs/architecture/fleet-config.md -- first start of a managed client

//go:build ze_core

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/fs"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/component/pki"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
	return bootHubReading(t, func(string) ([]byte, error) { return []byte(bootConfig), nil })
}

// bootHubReading is bootHub whose answer to a config-fetch is what
// readConfig returns, when it returns.
func bootHubReading(t *testing.T, readConfig func(string) ([]byte, error)) (string, *pki.Root) {
	t.Helper()
	root, err := pki.LoadOrGenerateRoot(bootStore(t))
	if err != nil {
		t.Fatalf("hub authority: %v", err)
	}
	srv, err := pluginserver.NewManagedServer(pluginserver.ManagedServerConfig{
		Authority:  root,
		Listeners:  []pluginserver.ManagedListener{{Addr: "127.0.0.1:0", Clients: map[string]string{bootClient: bootToken}}},
		ReadConfig: readConfig,
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

// TestFirstBootFetchOutlivesConnectTimeout proves that at first boot
// ze.managed.connect.timeout bounds reaching the hub and not the fetch: a hub
// that authenticates the client at once and answers its config-fetch only
// after more than the connect bound gives the client its configuration.
//
// VALIDATES: OD-6 and AC-6 at first boot. fetchInitialConfig answers the hub's
// configuration when its answer comes later than the connect bound, and well
// within the fetch's own bound (managed configFetchTimeout).
// PREVENTS: a first boot that can never fetch a configuration the hub takes
// longer than 5 seconds to send, a large one on an ordinary link.
//
// MUTATION: pass connectCtx to managed.FetchConfig in fetchInitialConfig: the
// fetch fails on the connect deadline.
func TestFirstBootFetchOutlivesConnectTimeout(t *testing.T) {
	const connectBound = 500 * time.Millisecond
	if err := env.Set("ze.managed.connect.timeout", connectBound.String()); err != nil {
		t.Fatalf("set ze.managed.connect.timeout: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Set("ze.managed.connect.timeout", ""); err != nil {
			t.Errorf("clear ze.managed.connect.timeout: %v", err)
		}
	})
	addr, root := bootHubReading(t, func(string) ([]byte, error) {
		// The hub's answer comes three connect bounds after the fetch: the
		// scenario is a hub slower than the connect bound, not a wait for state.
		time.Sleep(3 * connectBound)
		return []byte(bootConfig), nil
	})
	boot := managedBootstrap{server: addr, name: bootClient, token: bootToken, root: root.Certificate()}

	data, kept, err := fetchInitialConfig(boot)
	if err != nil {
		t.Fatalf("a first fetch answered after the connect bound failed: %v", err)
	}
	if string(data) != bootConfig {
		t.Errorf("config = %q, want %q", data, bootConfig)
	}
	if kept != nil {
		t.Error("a connection anchored on a stored root kept an authority")
	}
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

// TestBaselineWrittenOnFirstStart proves that the first start records the
// device's baseline: it fetches the configuration from a real hub as the first
// start does and caches it with the function the first start calls.
//
// VALIDATES: AC-2. meta/managed/base-version holds the hash of the cached
// configuration, read back from the store, before any managed client runs.
// PREVENTS: a device whose first reconnect finds no baseline, or one taken
// from the fetched bytes and never checked against what the store holds.
//
// MUTATION: drop RecordActiveAsBaseline from cacheFirstConfig and this fails:
// the baseline is not recorded.
func TestBaselineWrittenOnFirstStart(t *testing.T) {
	addr, root := bootHub(t)
	store := bootStore(t)
	writeKey(t, store, zefs.KeyInstanceName.Pattern, bootClient)
	fetched, _, err := fetchInitialConfig(managedBootstrap{server: addr, name: bootClient, token: bootToken, root: root.Certificate()})
	if err != nil {
		t.Fatalf("fetchInitialConfig: %v", err)
	}

	if cacheErr := cacheFirstConfig(store, bootClient+".conf", fetched); cacheErr != nil {
		t.Fatalf("cacheFirstConfig: %v", cacheErr)
	}

	baseline, err := managed.BaselineVersion(store)
	if err != nil {
		t.Fatalf("BaselineVersion: %v", err)
	}
	if !baseline.Recorded() {
		t.Fatal("the first start recorded no baseline")
	}
	active, err := managed.ActiveVersion(store)
	if err != nil {
		t.Fatalf("ActiveVersion: %v", err)
	}
	if baseline != active {
		t.Errorf("baseline %q is not the hash of the active configuration %q", baseline.Hash(), active.Hash())
	}
	if want := fleet.VersionHash([]byte(bootConfig)); baseline.Hash() != want {
		t.Errorf("baseline %q, want the hash of the hub's configuration %q", baseline.Hash(), want)
	}
}

// TestBaselineStableAcrossRestart proves that a device nobody changed is not
// diverged after a restart: it caches a first configuration, lets the boot
// record version it as a stored-source daemon does (storage.EnsureActiveVersion),
// closes the store, opens it again and compares the two hashes.
//
// VALIDATES: A-1 and AC-3. The hash of the active configuration equals the
// baseline in the reopened store, and the start path gives the managed client
// it builds from that store its baseline access, so the client compares the
// two and applies what its hub sends.
// PREVENTS: a device that holds after every restart.
func TestBaselineStableAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	configName := bootClient + ".conf"
	writeKey(t, store, zefs.KeyInstanceName.Pattern, bootClient)
	if cacheErr := cacheFirstConfig(store, configName, []byte(managedClientConfigText)); cacheErr != nil {
		t.Fatalf("cacheFirstConfig: %v", cacheErr)
	}
	booted, err := storage.ReadActiveConfig(store, configName)
	if err != nil {
		t.Fatalf("ReadActiveConfig: %v", err)
	}
	if _, _, ensureErr := storage.EnsureActiveVersion(store, configName, booted, time.Now()); ensureErr != nil {
		t.Fatalf("EnsureActiveVersion: %v", ensureErr)
	}
	if closeErr := store.Close(); closeErr != nil {
		t.Fatalf("close store: %v", closeErr)
	}

	reopened, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { reopened.Close() }) //nolint:errcheck // test cleanup

	active, err := managed.ActiveVersion(reopened)
	if err != nil {
		t.Fatalf("ActiveVersion: %v", err)
	}
	baseline, err := managed.BaselineVersion(reopened)
	if err != nil {
		t.Fatalf("BaselineVersion: %v", err)
	}
	if !baseline.Recorded() {
		t.Fatal("the baseline did not survive the restart")
	}
	if managed.Diverged(active, baseline) {
		t.Errorf("an unchanged device is diverged after a restart: active %q, baseline %q", active.Hash(), baseline.Hash())
	}
	clientCfg, err := extractManagedClientConfig(reopened, configName)
	if err != nil {
		t.Fatalf("the start path built no managed client from a configuration that names a hub: %v", err)
	}
	if clientCfg.Baseline == nil {
		t.Error("the start path built a managed client with no baseline access, so it would not hold")
	}
	if clientCfg.Chain == nil {
		t.Error("the start path built a managed client with no chain access")
	}
}

// chainBootHub starts a minimal TLS 1.3 hub answering one request handler per
// admitted session and returns its address and CA root.
func chainBootHub(t *testing.T, onFetch func(fleet.ConfigFetchRequest) fleet.ConfigFetchResponse, onUnexpected func(method string)) (string, *pki.Root) {
	t.Helper()
	root, err := pki.LoadOrGenerateRoot(bootStore(t))
	if err != nil {
		t.Fatalf("hub authority: %v", err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, issueErr := root.IssueLeaf("ze-managed-hub", []string{"127.0.0.1"})
			return &cert, issueErr
		},
	})
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer conn.Close() //nolint:errcheck // test cleanup
				if _, readErr := readAuthLine(conn, 512); readErr != nil {
					return
				}
				if _, writeErr := conn.Write([]byte("#0 ok\n")); writeErr != nil {
					return
				}
				rc := rpc.NewConn(conn, conn)
				for {
					req, reqErr := rc.ReadRequest(context.Background())
					if reqErr != nil {
						return
					}
					if req.Method != fleet.VerbConfigFetch {
						if onUnexpected != nil {
							onUnexpected(req.Method)
						}
						_ = rc.SendOK(context.Background(), req.ID)
						continue
					}
					var fetchReq fleet.ConfigFetchRequest
					if jsonErr := json.Unmarshal(req.Params, &fetchReq); jsonErr != nil {
						return
					}
					_ = rc.SendResult(context.Background(), req.ID, onFetch(fetchReq))
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-done
	})
	return ln.Addr().String(), root
}

// failingWriteStore wraps storage.Storage and fails WriteFile to prove a failed
// bootstrap cache persists neither baseline nor chain anchor.
type failingWriteStore struct {
	storage.Storage
}

func (f failingWriteStore) WriteFile(string, []byte, fs.FileMode) error {
	return errors.New("simulated write failure")
}

// TestFirstBootPersistsInitialChainAnchor proves AC-3:
//   - fetchInitialConfig verifies Version and ChainHash == FoldFull(data)
//     before closing the bootstrap connection and sends no bootstrap config-ack.
//   - cacheFirstConfig persists both meta/managed/base-version and
//     meta/managed/chain-state = {V1, FoldFull(B)}, and persists neither if
//     caching fails.
//   - On the first steady-state connection, the client sends V1 and FoldFull(B),
//     converges on Status=="current" with matching ChainHash, and sends no ack.
//
// VALIDATES: spec-fleet-10 AC-3 (first-boot anchor verification, persistence,
// failure atomicity, and steady-state current convergence without ack).
// PREVENTS: first boot dropping the initial chain anchor or accepting corrupt
// bootstrap bytes without verifying Version and FoldFull.
func TestFirstBootPersistsInitialChainAnchor(t *testing.T) {
	v1Bytes := []byte(managedClientConfigText)
	v1 := fleet.VersionHash(v1Bytes)
	c1 := fleet.FoldFull(v1Bytes)

	t.Run("rejects mismatched version and chain hash before returning", func(t *testing.T) {
		badVerAddr, badVerRoot := chainBootHub(t, func(fleet.ConfigFetchRequest) fleet.ConfigFetchResponse {
			return fleet.ConfigFetchResponse{
				Version:   "0000000000000000",
				Config:    base64.StdEncoding.EncodeToString(v1Bytes),
				ChainHash: c1,
			}
		}, nil)
		_, _, err := fetchInitialConfig(managedBootstrap{server: badVerAddr, name: bootClient, token: bootToken, root: badVerRoot.Certificate()})
		if err == nil || !strings.Contains(err.Error(), "version mismatch") {
			t.Fatalf("expected version mismatch error, got %v", err)
		}

		badChainAddr, badChainRoot := chainBootHub(t, func(fleet.ConfigFetchRequest) fleet.ConfigFetchResponse {
			return fleet.ConfigFetchResponse{
				Version:   v1,
				Config:    base64.StdEncoding.EncodeToString(v1Bytes),
				ChainHash: strings.Repeat("a", fleet.ChainHashLen),
			}
		}, nil)
		_, _, err = fetchInitialConfig(managedBootstrap{server: badChainAddr, name: bootClient, token: bootToken, root: badChainRoot.Certificate()})
		if err == nil || !strings.Contains(err.Error(), "chain hash mismatch") {
			t.Fatalf("expected chain hash mismatch error, got %v", err)
		}
	})

	t.Run("cache failure persists neither baseline nor chain anchor", func(t *testing.T) {
		store := bootStore(t)
		writeKey(t, store, zefs.KeyInstanceName.Pattern, bootClient)
		if err := cacheFirstConfig(failingWriteStore{Storage: store}, bootClient+".conf", v1Bytes); err == nil {
			t.Fatal("expected cacheFirstConfig to fail on WriteFile error")
		}
		baseline, err := managed.BaselineVersion(store)
		if err != nil {
			t.Fatalf("BaselineVersion: %v", err)
		}
		if baseline.Recorded() {
			t.Error("baseline was persisted despite cache write failure")
		}
		_, present, err := managed.ReadChainState(store)
		if err != nil {
			t.Fatalf("ReadChainState: %v", err)
		}
		if present {
			t.Error("chain state was persisted despite cache write failure")
		}
	})

	t.Run("first boot caches anchor and steady-state current converges with no ack", func(t *testing.T) {
		var unexpectedRPC atomic.Int32
		var steadyFetch atomic.Pointer[fleet.ConfigFetchRequest]
		steadySeen := make(chan struct{}, 1)

		addr, root := chainBootHub(t, func(req fleet.ConfigFetchRequest) fleet.ConfigFetchResponse {
			if req.Version == "" {
				return fleet.ConfigFetchResponse{
					Mode:      fleet.FetchModeFull,
					Version:   v1,
					Config:    base64.StdEncoding.EncodeToString(v1Bytes),
					ChainHash: c1,
				}
			}
			copied := req
			steadyFetch.Store(&copied)
			select {
			case steadySeen <- struct{}{}:
			default:
			}
			return fleet.ConfigFetchResponse{
				Status:    "current",
				ChainHash: c1,
			}
		}, func(method string) {
			if method == fleet.VerbConfigAck {
				unexpectedRPC.Add(1)
			}
		})

		fetched, _, err := fetchInitialConfig(managedBootstrap{
			server: addr,
			name:   bootClient,
			token:  bootToken,
			root:   root.Certificate(),
		})
		if err != nil {
			t.Fatalf("fetchInitialConfig: %v", err)
		}
		if string(fetched) != string(v1Bytes) {
			t.Fatalf("fetched config bytes differ from hub bytes")
		}
		if unexpectedRPC.Load() != 0 {
			t.Fatalf("bootstrap connection sent %d unexpected config-ack RPCs", unexpectedRPC.Load())
		}

		store := bootStore(t)
		configName := bootClient + ".conf"
		writeKey(t, store, zefs.KeyInstanceName.Pattern, bootClient)
		writeKey(t, store, zefs.KeyInstanceManaged.Pattern, "true\n")
		if err := cacheFirstConfig(store, configName, fetched); err != nil {
			t.Fatalf("cacheFirstConfig: %v", err)
		}

		st, present, err := managed.ReadChainState(store)
		if err != nil {
			t.Fatalf("ReadChainState: %v", err)
		}
		if !present {
			t.Fatal("cacheFirstConfig did not persist meta/managed/chain-state")
		}
		if st.Version != v1 || st.Chain != c1 {
			t.Fatalf("chain state = %+v, want {Version:%s Chain:%s}", st, v1, c1)
		}

		clientCfg, err := extractManagedClientConfig(store, configName)
		if err != nil {
			t.Fatalf("extractManagedClientConfig: %v", err)
		}
		clientCfg.Server = addr
		clientCfg.Token = bootToken
		clientCfg.CA = ""
		clientCfg.HubRoot = root.Certificate()
		inSync := make(chan managed.ChainStatus, 4)
		clientCfg.OnChainState = func(cs managed.ChainStatus) {
			if cs.ChainState == managed.ChainStateInSync {
				select {
				case inSync <- cs:
				default:
				}
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			managed.RunManagedClient(ctx, *clientCfg)
		}()
		t.Cleanup(func() {
			cancel()
			<-done
		})

		select {
		case <-steadySeen:
		case <-time.After(5 * time.Second):
			t.Fatal("steady-state client did not fetch from hub")
		}
		select {
		case cs := <-inSync:
			if cs.ChainHash != c1 {
				t.Fatalf("in-sync chain hash = %s, want %s", cs.ChainHash, c1)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("steady-state client did not report chain-state in-sync")
		}

		req := steadyFetch.Load()
		if req == nil || req.Version != v1 || req.ChainHash != c1 {
			t.Fatalf("steady-state fetch = %+v, want version=%s chain=%s", req, v1, c1)
		}
		if unexpectedRPC.Load() != 0 {
			t.Fatalf("steady-state current fetch sent %d unexpected config-ack RPCs", unexpectedRPC.Load())
		}
	})
}
