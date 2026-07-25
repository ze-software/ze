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
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestStartManagedServerServesBlobConfig(t *testing.T) {
	const (
		clientName   = "edge-01"
		clientSecret = "edge01-secret-that-is-at-least-32ch"
		cfgV1        = "bgp { peer p1 { } }\n"
		cfgV2        = "bgp { peer p1 { } peer p2 { } }\n"
	)

	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "hub.zefs"), dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
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

// TestStartManagedServerNilWithoutClients: AC-11 -- a hub with no client entries starts
// no managed server.
func TestStartManagedServerNilWithoutClients(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "hub.zefs"), dir)
	if err != nil {
		t.Fatalf("NewBlob: %v", err)
	}
	hubConfig := &zePlugin.HubConfig{
		Servers: []zePlugin.HubServerConfig{{Name: "local", Host: "127.0.0.1", Port: 0}},
	}
	if srv := startManagedServer(t.Context(), store, hubConfig); srv != nil {
		t.Error("expected nil managed server when no server block declares clients")
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
