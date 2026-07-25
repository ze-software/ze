// VALIDATES: the dedicated managed-config server serves fleet clients end-to-end --
// auth, config-fetch (full + "current"), ping, config-changed push, duplicate-name
// rejection, and disconnect cleanup (spec-managed-hub-server AC-1..AC-8).
// PREVENTS: regressing the hub server back to dead code (the original defect: the
// ManagedConfigService existed but nothing served it, so config-fetch was never answered).

package server

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"slices"
	"testing"
	"time"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	testClientName   = "edge-01"
	testClientSecret = "edge01-secret-that-is-at-least-32ch"
	testClientConfig = "bgp { peer p1 { } }\n"

	testClient2Name   = "edge-02"
	testClient2Secret = "edge02-secret-that-is-at-least-32ch"
	testClient2Config = "bgp { peer p2 { } }\n"
)

// startTestManagedServer starts a ManagedServer serving a single client "edge-01"
// whose config is testClientConfig, on a port-0 loopback listener. Returns the
// server and its bound address.
func startTestManagedServer(t *testing.T) (*ManagedServer, string) {
	t.Helper()
	readConfig := func(name string) ([]byte, error) {
		switch name {
		case testClientName:
			return []byte(testClientConfig), nil
		case testClient2Name:
			return []byte(testClient2Config), nil
		}
		return nil, ErrClientConfigNotFound
	}
	srv, err := NewManagedServer(ManagedServerConfig{
		Addrs: []string{"127.0.0.1:0"},
		ClientSecrets: map[string]string{
			testClientName:  testClientSecret,
			testClient2Name: testClient2Secret,
		},
		ReadConfig: readConfig,
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(srv.Stop)
	addrs := srv.Addrs()
	if len(addrs) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(addrs))
	}
	return srv, addrs[0].String()
}

// dialTestClient connects to the managed server, authenticates as name with token,
// and returns a MuxConn ready for RPCs. Fails the test on any auth error.
func dialTestClient(t *testing.T, addr, name, token string) *rpc.MuxConn {
	t.Helper()
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test-only; self-signed server cert
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
	_, verb, _, parseErr := rpc.ParseLine(line)
	if parseErr != nil || verb != "ok" {
		conn.Close() //nolint:errcheck // test cleanup
		t.Fatalf("auth not ok: verb=%q err=%v", verb, parseErr)
	}
	mc := rpc.NewMuxConn(rpc.NewConn(conn, conn))
	t.Cleanup(func() { mc.Close() }) //nolint:errcheck // test cleanup
	return mc
}

func fetch(t *testing.T, mc *rpc.MuxConn, version string) fleet.ConfigFetchResponse {
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

// TestManagedServeLoopAnswersConfigFetch: AC-1/AC-2 -- a served client's config-fetch
// returns the client's config (base64) with the correct version hash.
func TestManagedServeLoopAnswersConfigFetch(t *testing.T) {
	_, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)

	resp := fetch(t, mc, "")
	wantVersion := fleet.VersionHash([]byte(testClientConfig))
	if resp.Version != wantVersion {
		t.Errorf("version = %q, want %q", resp.Version, wantVersion)
	}
	gotConfig, err := base64.StdEncoding.DecodeString(resp.Config)
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if string(gotConfig) != testClientConfig {
		t.Errorf("config = %q, want %q", gotConfig, testClientConfig)
	}
}

// TestManagedServeConfigCurrent: AC-3 -- a fetch with the current version returns
// status "current" and no config body.
func TestManagedServeConfigCurrent(t *testing.T) {
	_, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)

	current := fleet.VersionHash([]byte(testClientConfig))
	resp := fetch(t, mc, current)
	if resp.Status != "current" {
		t.Errorf("status = %q, want current", resp.Status)
	}
	if resp.Config != "" {
		t.Errorf("config should be empty when current, got %d bytes", len(resp.Config))
	}
}

// TestManagedServeDuplicateNameRejected: AC-4 -- a second connection with an already-
// connected name is refused; the first stays served.
func TestManagedServeDuplicateNameRejected(t *testing.T) {
	srv, addr := startTestManagedServer(t)
	mc1 := dialTestClient(t, addr, testClientName, testClientSecret)
	// First connection works.
	_ = fetch(t, mc1, "")

	waitForConnected(t, srv, testClientName, true)

	// Second connection authenticates but is dropped by RegisterClient; a fetch fails.
	mc2 := dialTestClient(t, addr, testClientName, testClientSecret)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := mc2.CallRPC(ctx, fleet.VerbConfigFetch, fleet.ConfigFetchRequest{}); err == nil {
		t.Error("duplicate connection fetch should fail (connection refused/closed)")
	}
	// First connection still works.
	_ = fetch(t, mc1, "")
}

// TestManagedServePingOK: AC-7.
func TestManagedServePingOK(t *testing.T) {
	_, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := mc.CallRPC(ctx, fleet.VerbPing, struct{}{}); err != nil {
		t.Errorf("ping: %v", err)
	}
}

// TestManagedServeConfigChangedPush: AC-5 -- NotifyConfigChanged pushes a config-
// changed request to the connected client.
func TestManagedServeConfigChangedPush(t *testing.T) {
	srv, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	_ = fetch(t, mc, "") // ensure connected + registered
	waitForConnected(t, srv, testClientName, true)

	// NotifyConfigChanged only enqueues (non-blocking); notifyWorker delivers it.
	srv.NotifyConfigChanged(testClientName)

	select {
	case req, ok := <-mc.Requests():
		if !ok {
			t.Fatal("connection closed before config-changed arrived")
		}
		if req.Method != fleet.VerbConfigChanged {
			t.Errorf("method = %q, want %q", req.Method, fleet.VerbConfigChanged)
		}
		var cc fleet.ConfigChanged
		if err := json.Unmarshal(req.Params, &cc); err != nil {
			t.Fatalf("unmarshal config-changed: %v", err)
		}
		if cc.Version != fleet.VersionHash([]byte(testClientConfig)) {
			t.Errorf("config-changed version = %q, want %q", cc.Version, fleet.VersionHash([]byte(testClientConfig)))
		}
		if err := mc.SendOK(context.Background(), req.ID); err != nil {
			t.Errorf("reply ok: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for config-changed push")
	}
}

// TestManagedServeAuthReject: A-4 -- a wrong token is rejected (connection fails).
func TestManagedServeAuthReject(t *testing.T) {
	_, addr := startTestManagedServer(t)
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // test-only
		MinVersion:         tls.VersionTLS13,
	}}
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer dialCancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck // test cleanup
	if authErr := pluginipc.SendAuth(context.Background(), conn, "wrong-token-wrong-token-wrong-32", testClientName); authErr != nil {
		t.Fatalf("send auth: %v", authErr)
	}
	line, readErr := pluginipc.ReadLineRaw(conn, 512)
	if readErr != nil {
		return // conn closed on reject is acceptable
	}
	_, verb, _, _ := rpc.ParseLine(line)
	if verb == "ok" {
		t.Error("auth with wrong token should not be ok")
	}
}

// TestManagedServeDisconnectUnregisters: AC-8 -- closing the client connection
// unregisters it so a later config-changed is not pushed.
func TestManagedServeDisconnectUnregisters(t *testing.T) {
	srv, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	_ = fetch(t, mc, "")
	waitForConnected(t, srv, testClientName, true)

	if err := mc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	waitForConnected(t, srv, testClientName, false)

	// NotifyConfigChanged on a disconnected client is a no-op (does not block/panic).
	srv.NotifyConfigChanged(testClientName)
}

// TestManagedServeConfigAckRecorded: AC-6 -- a config-ack is accepted (the hub replies
// ok and keeps the connection served).
func TestManagedServeConfigAckRecorded(t *testing.T) {
	_, addr := startTestManagedServer(t)
	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ack := fleet.ConfigAck{Version: fleet.VersionHash([]byte(testClientConfig)), OK: true}
	if _, err := mc.CallRPC(ctx, fleet.VerbConfigAck, ack); err != nil {
		t.Errorf("config-ack: %v", err)
	}
	// Connection is still usable after the ack.
	_ = fetch(t, mc, "")
}

// TestManagedServeConfigIsolation: AC-10 -- a client only ever receives its own config.
// The served config is keyed by the authenticated session name, and config-fetch carries
// no name field, so a client authenticated as edge-02 gets edge-02's config, never
// edge-01's.
func TestManagedServeConfigIsolation(t *testing.T) {
	srv, addr := startTestManagedServer(t)

	mc1 := dialTestClient(t, addr, testClientName, testClientSecret)
	resp1 := fetch(t, mc1, "")
	cfg1, _ := base64.StdEncoding.DecodeString(resp1.Config)
	if string(cfg1) != testClientConfig {
		t.Errorf("edge-01 got %q, want %q", cfg1, testClientConfig)
	}

	mc2 := dialTestClient(t, addr, testClient2Name, testClient2Secret)
	waitForConnected(t, srv, testClient2Name, true)
	resp2 := fetch(t, mc2, "")
	cfg2, _ := base64.StdEncoding.DecodeString(resp2.Config)
	if string(cfg2) != testClient2Config {
		t.Errorf("edge-02 got %q, want %q (isolation breach)", cfg2, testClient2Config)
	}
	if resp2.Version != fleet.VersionHash([]byte(testClient2Config)) {
		t.Errorf("edge-02 version = %q, want its own config hash", resp2.Version)
	}
}

// TestManagedServeResilientBinding: a server with one unavailable address (collision
// with an already-bound listener) still binds and serves on the remaining addresses,
// rather than failing wholesale. Regression for the review finding that a managed
// block coinciding with the plugin acceptor's block disabled the whole managed server.
func TestManagedServeResilientBinding(t *testing.T) {
	// srv1 occupies a real address.
	srv1, occupied := startTestManagedServer(t)
	_ = srv1

	readConfig := func(name string) ([]byte, error) { return []byte(testClientConfig), nil }
	srv2, err := NewManagedServer(ManagedServerConfig{
		Addrs:         []string{occupied, "127.0.0.1:0"}, // one colliding, one free
		ClientSecrets: map[string]string{testClientName: testClientSecret},
		ReadConfig:    readConfig,
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if startErr := srv2.Start(context.Background()); startErr != nil {
		t.Fatalf("Start should succeed despite one colliding address: %v", startErr)
	}
	t.Cleanup(srv2.Stop)

	addrs := srv2.Addrs()
	if len(addrs) != 1 {
		t.Fatalf("expected 1 bound listener (colliding address skipped), got %d", len(addrs))
	}
	// The listener that did bind serves normally.
	mc := dialTestClient(t, addrs[0].String(), testClientName, testClientSecret)
	_ = fetch(t, mc, "")
}

// TestManagedServeAllAddressesUnavailable: when no address can bind, Start returns an
// error (and releases its context rather than leaking the cancel func).
func TestManagedServeAllAddressesUnavailable(t *testing.T) {
	srv1, occupied := startTestManagedServer(t)
	_ = srv1

	srv2, err := NewManagedServer(ManagedServerConfig{
		Addrs:         []string{occupied}, // only address is already in use
		ClientSecrets: map[string]string{testClientName: testClientSecret},
		ReadConfig:    func(name string) ([]byte, error) { return []byte(testClientConfig), nil },
	})
	if err != nil {
		t.Fatalf("NewManagedServer: %v", err)
	}
	if startErr := srv2.Start(context.Background()); startErr == nil {
		srv2.Stop()
		t.Fatal("Start should fail when no address can bind")
	}
}

// TestManagedServeConfigChangedNoHeadOfLine: a client that accepts a config-changed
// notification but never replies must not block config-changed delivery to other
// clients. Regression for the review finding that the single serial notify worker,
// with no per-push timeout, let one stalled client stall the whole fleet's pushes.
func TestManagedServeConfigChangedNoHeadOfLine(t *testing.T) {
	srv, addr := startTestManagedServer(t)
	mcA := dialTestClient(t, addr, testClientName, testClientSecret)   // will stall
	mcB := dialTestClient(t, addr, testClient2Name, testClient2Secret) // must still get its push
	_ = fetch(t, mcA, "")
	_ = fetch(t, mcB, "")
	waitForConnected(t, srv, testClientName, true)
	waitForConnected(t, srv, testClient2Name, true)

	// Stall A: trigger A's config-changed but never read/reply to it, so the worker
	// handling A's push blocks on A's (absent) reply.
	srv.NotifyConfigChanged(testClientName)
	// Give a worker a moment to pick up and block on A.
	waitForPushed(t, mcA) // A's push is now in flight, unanswered

	// B's config-changed must still arrive promptly (a different pool worker handles it).
	srv.NotifyConfigChanged(testClient2Name)
	select {
	case req, ok := <-mcB.Requests():
		if !ok {
			t.Fatal("B connection closed before its config-changed arrived")
		}
		if req.Method != fleet.VerbConfigChanged {
			t.Fatalf("B method = %q, want %q", req.Method, fleet.VerbConfigChanged)
		}
		_ = mcB.SendOK(context.Background(), req.ID)
	case <-time.After(3 * time.Second):
		t.Fatal("B's config-changed was blocked by A's stalled push (head-of-line blocking)")
	}
}

// waitForPushed drains A's inbound config-changed (so it is confirmed in flight) but
// deliberately does NOT reply, keeping the serving worker blocked on A.
func waitForPushed(t *testing.T, mcA *rpc.MuxConn) {
	t.Helper()
	select {
	case req, ok := <-mcA.Requests():
		if !ok {
			t.Fatal("A connection closed before its config-changed arrived")
		}
		if req.Method != fleet.VerbConfigChanged {
			t.Fatalf("A method = %q, want %q", req.Method, fleet.VerbConfigChanged)
		}
		// Intentionally no reply: A stalls here.
	case <-time.After(3 * time.Second):
		t.Fatal("A's config-changed never arrived")
	}
}

// waitForConnected polls until the client's connected state matches want, or fails.
func waitForConnected(t *testing.T, srv *ManagedServer, name string, want bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if connectedHas(srv, name) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client %q connected=%v not reached", name, want)
}

func connectedHas(srv *ManagedServer, name string) bool {
	return slices.Contains(srv.connectedClients(), name)
}
