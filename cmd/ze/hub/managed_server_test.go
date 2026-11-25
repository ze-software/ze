// VALIDATES: startManagedServer wires the dedicated managed server from hub config +
// a real blob store: it serves a client's config-fetch from file/active/client-<name>.conf
// and pushes config-changed when that blob is written (spec-managed-hub-server Phase 3/4).
// PREVENTS: the wiring regressing to dead code -- ManagedConfigService existed but nothing
// constructed/started it, so config-fetch was never answered in production.

package hub

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	zepki "github.com/ze-software/ze/internal/component/pki"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func TestStartManagedServerServesBlobConfig(t *testing.T) {
	const (
		clientName   = "edge-01"
		clientSecret = "edge01-secret-that-is-at-least-32ch"
		cfgV1        = "bgp { peer p1 { } }\n"
		cfgV2        = "bgp { peer p1 { } peer p2 { } }\n"
	)

	dir := t.TempDir()
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	// Admin provisions the client's config on the hub blob.
	if err := store.WriteFile(pluginserver.ClientConfigKey(clientName), []byte(cfgV1), 0o600); err != nil {
		t.Fatalf("write client config: %v", err)
	}

	hubConfig := &zePlugin.HubConfig{
		Servers: []zePlugin.HubServerConfig{{
			Name:    "central",
			Host:    "127.0.0.1",
			Port:    0, // OS-assigned; read back via srv.Addrs()
			Clients: map[string]string{clientName: clientSecret},
		}},
	}

	srv := startManagedServer(t.Context(), store, hubConfig)
	if srv == nil {
		t.Fatal("startManagedServer returned nil; expected a server for a hub with client entries")
	}
	t.Cleanup(srv.Stop)
	addrs := srv.Addrs()
	if len(addrs) != 1 {
		t.Fatalf("expected 1 managed listener, got %d", len(addrs))
	}

	mc := dialHubClient(t, addrs[0].String(), clientName, clientSecret)

	// config-fetch returns the blob-stored config.
	resp := hubFetch(t, mc, "")
	got, _ := base64.StdEncoding.DecodeString(resp.Config)
	if string(got) != cfgV1 {
		t.Fatalf("fetched config = %q, want %q", got, cfgV1)
	}
	if resp.Version != fleet.VersionHash([]byte(cfgV1)) {
		t.Errorf("version = %q, want %q", resp.Version, fleet.VersionHash([]byte(cfgV1)))
	}

	// Writing a new config blob for this client pushes config-changed.
	if err := store.WriteFile(pluginserver.ClientConfigKey(clientName), []byte(cfgV2), 0o600); err != nil {
		t.Fatalf("write updated config: %v", err)
	}
	select {
	case req, ok := <-mc.Requests():
		if !ok {
			t.Fatal("connection closed before config-changed arrived")
		}
		if req.Method != fleet.VerbConfigChanged {
			t.Fatalf("method = %q, want %q", req.Method, fleet.VerbConfigChanged)
		}
		var cc fleet.ConfigChanged
		if err := json.Unmarshal(req.Params, &cc); err != nil {
			t.Fatalf("unmarshal config-changed: %v", err)
		}
		if cc.Version != fleet.VersionHash([]byte(cfgV2)) {
			t.Errorf("config-changed version = %q, want the new config's hash", cc.Version)
		}
		_ = mc.SendOK(context.Background(), req.ID)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for config-changed after blob write")
	}
}

// TestStartManagedServerWithoutClients checks the empty registry is available
// for the reload that declares the first device.
func TestStartManagedServerWithoutClients(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	hubConfig := &zePlugin.HubConfig{
		Servers: []zePlugin.HubServerConfig{{Name: "local", Host: "127.0.0.1", Port: 0}},
	}
	defer store.Close() //nolint:errcheck // test cleanup
	srv := startManagedServer(t.Context(), store, hubConfig)
	if srv == nil {
		t.Fatal("no managed server to receive the first serving block on reload")
	}
	defer srv.Stop()
	if len(srv.Addrs()) != 0 {
		t.Fatal("a server block with no clients bound a managed listener")
	}
	for _, key := range []string{zefs.KeyCACert.Pattern, zefs.KeyCAKey.Pattern} {
		_, readErr := store.ReadKey(key)
		require.ErrorIs(t, readErr, fs.ErrNotExist, "idle managed registry created %s", key)
	}
	if devices, err := pluginserver.ManagedRegistrySnapshot(); err != nil || len(devices) != 0 {
		t.Fatalf("empty hub registry = %v, %v", devices, err)
	}
}

func dialHubClient(t *testing.T, addr, name, token string) *rpc.MuxConn {
	t.Helper()
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test-only; self-signed managed listener cert
		MinVersion:         tls.VersionTLS13,
	}}
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer dialCancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if authErr := pluginipc.SendAuth(context.Background(), conn, token, name); authErr != nil {
		t.Fatalf("send auth: %v", authErr)
	}
	line, readErr := pluginipc.ReadLineRaw(conn, 512)
	if readErr != nil {
		conn.Close() //nolint:errcheck // test cleanup
		t.Fatalf("read auth response: %v", readErr)
	}
	if _, verb, _, parseErr := rpc.ParseLine(line); parseErr != nil || verb != "ok" {
		conn.Close() //nolint:errcheck // test cleanup
		t.Fatalf("auth not ok: verb=%q err=%v", verb, parseErr)
	}
	mc := rpc.NewMuxConn(rpc.NewConn(conn, conn))
	t.Cleanup(func() { mc.Close() }) //nolint:errcheck // test cleanup
	return mc
}

func hubFetch(t *testing.T, mc *rpc.MuxConn, version string) fleet.ConfigFetchResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := mc.CallRPC(ctx, fleet.VerbConfigFetch, fleet.ConfigFetchRequest{Version: version})
	if err != nil {
		t.Fatalf("config-fetch: %v", err)
	}
	var resp fleet.ConfigFetchResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal config-fetch response: %v", err)
	}
	return resp
}

// TestPrepareManagedReloadReadsCandidate verifies startup and the candidate
// reader agree on labels, block identity, and the scope of a broken reference.
func TestPrepareManagedReloadReadsCandidate(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	srv := startManagedServer(t.Context(), store, &zePlugin.HubConfig{})
	require.NotNil(t, srv)
	t.Cleanup(srv.Stop)
	root, hub, bus := zeconfig.NewTree(), zeconfig.NewTree(), zeconfig.NewTree()
	root.SetContainer("bus", bus)
	bus.SetContainer("hub", hub)
	blk := zeconfig.NewTree()
	blk.Set("ip", "127.0.0.1")
	blk.Set("port", "0")
	device := zeconfig.NewTree()
	device.Set("secret", "fleet-candidate-device-secret-at-least-32ch")
	label := zeconfig.NewTree()
	label.Set("value", "London")
	device.AddListEntry("label", "site", label)
	blk.AddListEntry("client", "edge-01", device)
	hub.AddListEntry("server", "central", blk)

	plan, err := prepareManagedReload(root, srv)
	require.NoError(t, err)
	require.Empty(t, srv.Addrs(), "preparation must not bind the candidate address")
	_, err = store.ReadKey(zefs.KeyCACert.Pattern)
	require.NoError(t, err, "preparing the first serving block needs the durable root")
	srv.ApplyManagedListeners(plan)
	devices, err := pluginserver.ManagedRegistrySnapshot()
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, "London", devices[0].Labels["site"])
	blk.Set("certificate", "unresolved-candidate-test")
	plan, err = prepareManagedReload(root, srv)
	require.Nil(t, plan)
	require.ErrorContains(t, err, "bus hub server central certificate unresolved-candidate-test")
	require.ErrorContains(t, err, "define pki certificate unresolved-candidate-test")
	require.Len(t, srv.Addrs(), 1)
	label.Set("value", "\n")
	_, err = prepareManagedReload(root, srv)
	var problem *opererr.Error
	require.ErrorAs(t, err, &problem)
	require.Contains(t, problem.Fix(), "set bus hub server central client edge-01 label site value")
	require.Contains(t, problem.Problem(), "invalid value for label site")
}

func TestManagedServerStartupFailurePublished(t *testing.T) {
	closed, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, closed.Close())
	require.Nil(t, startManagedServer(t.Context(), closed, &zePlugin.HubConfig{}))
	t.Cleanup(func() { pluginserver.RecordManagedStartupFailure(nil) })
	_, err = pluginserver.ManagedDoctorSnapshot()
	require.ErrorIs(t, err, fs.ErrClosed)
	var problem *opererr.Error
	require.ErrorAs(t, err, &problem)
	require.Equal(t, "ze show data check", problem.Fix())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	srv := startManagedServer(t.Context(), store, &zePlugin.HubConfig{})
	require.NotNil(t, srv)
	t.Cleanup(srv.Stop)
	record, err := pluginserver.ManagedDoctorSnapshot()
	require.NoError(t, err)
	require.Nil(t, record.StartupFailure)
}

// observedManagedStore records which server consumes a real client-config write.
type observedManagedStore struct {
	storage.Storage
	registrations int
	observed      []int
}

func (s *observedManagedStore) SetWriteObserver(fn func(string)) func() {
	s.registrations++
	registration := s.registrations
	return s.Storage.SetWriteObserver(func(key string) {
		if key == "file/active/client-edge.conf" {
			s.observed = append(s.observed, registration)
		}
		fn(key)
	})
}

func TestManagedServerStopDetachesItsWriteObserver(t *testing.T) {
	backing, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backing.Close()) })
	store := &observedManagedStore{Storage: backing}
	const secret = "edge-device-secret-at-least-32-characters"
	cfg := &zePlugin.HubConfig{Servers: []zePlugin.HubServerConfig{{
		Name: "central", Host: "127.0.0.1", Clients: map[string]string{"edge": secret},
	}}}
	first := startManagedServer(t.Context(), store, cfg)
	require.NotNil(t, first)
	t.Cleanup(first.Stop)
	write := func() {
		t.Helper()
		require.NoError(t, store.WriteFile("client-edge.conf", []byte("config"), 0o600))
	}
	write()
	require.Equal(t, []int{1}, store.observed)
	require.Len(t, first.Addrs(), 1)
	client := dialHubClient(t, first.Addrs()[0].String(), "edge", secret)
	require.Equal(t, fleet.VersionHash([]byte("config")), hubFetch(t, client, "").Version)
	require.NoError(t, client.Close())
	first.Stop()
	write()
	require.Equal(t, []int{1}, store.observed)

	raw, err := store.ReadKey("meta/fleet/device/edge/record")
	require.NoError(t, err)
	var history pluginserver.ManagedDeviceRecord
	require.NoError(t, json.Unmarshal(raw, &history))
	require.Equal(t, "edge", history.Name)
	require.False(t, history.FirstConnected.IsZero())
	require.False(t, history.LastSeen.IsZero())
	require.Equal(t, pluginserver.ManagedFetchServed, history.LastFetchResult)

	second := startManagedServer(t.Context(), store, cfg)
	require.NotNil(t, second)
	t.Cleanup(second.Stop)
	device, err := pluginserver.ManagedDeviceByName("edge")
	require.NoError(t, err)
	require.True(t, device.Persisted)
	require.False(t, device.Connected)
	require.Equal(t, history, device.ManagedDeviceRecord)
	first.Stop()
	write()
	require.Equal(t, []int{1, 2}, store.observed)
	second.Stop()
	write()
	require.Equal(t, []int{1, 2}, store.observed)
}

// The durable CA is needed at startup only for managed listeners that use it.
// Keep the store error and the operator repair available to doctor even though
// construction stopped before a managed server was published.
func TestManagedColdStartWithBrokenAuthority(t *testing.T) {
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	t.Cleanup(func() { pluginserver.RecordManagedStartupFailure(nil) })
	require.NoError(t, store.WriteFile(zefs.KeyCACert.Pattern, []byte("original root"), 0o600))
	cfg := &zePlugin.HubConfig{Servers: []zePlugin.HubServerConfig{{
		Name: "central", Host: "127.0.0.1", Clients: map[string]string{"device": "device-secret"},
	}}}
	require.Nil(t, startManagedServer(t.Context(), store, cfg))
	snapshot, err := pluginserver.ManagedDoctorSnapshot()
	var problem *opererr.Error
	require.ErrorAs(t, err, &problem)
	require.Equal(t, err, snapshot.StartupFailure)
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.Equal(t, "ze data check", problem.Fix())
	require.Contains(t, problem.Why(), "meta/ca/cert and meta/ca/key")
	surviving, err := store.ReadFile(zefs.KeyCACert.Pattern)
	require.NoError(t, err)
	require.Equal(t, "original root", string(surviving))
	_, err = store.ReadFile(zefs.KeyCAKey.Pattern)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

// A failed move from a named certificate to the local CA must give a repair
// the editor can merge even when this candidate adds a new serving block.
func TestManagedCertificateRepairMergesCandidate(t *testing.T) {
	ca, chain, cert, key := chainB64(t, "web-cert", 1)
	candidate := pkiChainConfig(ca, chain, cert, key, "web-cert") + `bus { hub {
		server central { ip 127.0.0.1; port 14401; client edge-01 { secret changed-device-secret-at-least-32-characters; } }
		server "new block" { ip 127.0.0.2; port 14402; client edge-02 { secret added-device-secret-at-least-32-characters; } }
	} }`
	editor, err := cli.NewEditorFromContent([]byte(candidate), "fleet.conf")
	require.NoError(t, err)
	issuer, err := preparePKIConfig(editor.Tree())
	require.NoError(t, err)
	require.NoError(t, zepki.Load(issuer))
	t.Cleanup(func() { _ = zepki.Load(nil) })
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.WriteFile(zefs.KeyCACert.Pattern, []byte("original root"), 0o600))
	srv := startManagedServer(t.Context(), store, &zePlugin.HubConfig{Servers: []zePlugin.HubServerConfig{{
		Name: "central", Host: "127.0.0.1", Certificate: "web-cert",
		Clients: map[string]string{"edge-01": "old-device-secret-at-least-32-characters"},
	}}})
	require.NotNil(t, srv)
	t.Cleanup(srv.Stop)
	require.Len(t, srv.Addrs(), 1)
	before, err := zeconfig.ExtractHubConfig(editor.Tree())
	require.NoError(t, err)
	require.Len(t, before.Servers, 2)
	_, err = prepareManagedReload(editor.Tree(), srv)
	var problem *opererr.Error
	require.ErrorAs(t, err, &problem)
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.Equal(t, "load terminal absolute merge", problem.Fix())
	var lines []string
	for line := range strings.SplitSeq(problem.Why(), "\n") {
		if strings.HasPrefix(line, "set ") {
			lines = append(lines, line)
		}
	}
	fragment, err := editor.ParseLoad(strings.Join(lines, "\n"), nil)
	require.NoError(t, err)
	require.NoError(t, editor.LoadMerge(nil, fragment))
	after, err := zeconfig.ExtractHubConfig(editor.Tree())
	require.NoError(t, err)
	for i := range before.Servers {
		before.Servers[i].Certificate = "web-cert"
	}
	require.Equal(t, before, after)
	issuer, err = preparePKIConfig(editor.Tree())
	require.NoError(t, err)
	require.NoError(t, zepki.Load(issuer))
	_, err = prepareManagedReload(editor.Tree(), srv)
	require.NoError(t, err)
}
