package l2tpauthradius

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpauthlocal "github.com/ze-software/ze/internal/component/l2tp/plugins/authlocal"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Run the registered engine, including its startup handshake and serialized SDK
// callbacks. The assertions use the shared subscriber handler, not parser fields.
func startAuthProvider(t *testing.T, name, config string) (context.Context, *rpc.DirectBridge) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	reg := registry.Lookup(name)
	if reg == nil {
		t.Fatalf("missing provider %s", name)
	}
	pluginEnd, engineEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	bridge := rpc.NewDirectBridge()
	done := make(chan int, 1)
	go func() { done <- reg.RunEngine(rpc.NewBridgedConn(pluginEnd, bridge)) }()
	t.Cleanup(func() {
		bridge.CloseCallbacks()
		_ = mux.Close()
		_ = pluginEnd.Close()
		_ = engineEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Errorf("%s did not stop", name)
		}
	})
	for stage := range 3 {
		select {
		case request := <-mux.Requests():
			if request == nil {
				t.Fatal("provider startup connection closed")
			}
			if err := mux.SendOK(ctx, request.ID); err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		switch stage {
		case 0:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:configure", &rpc.ConfigureInput{Sections: authConfigSections(config)}); err != nil {
				t.Fatal(err)
			}
		case 1:
			if _, err := mux.CallRPC(ctx, "ze-plugin-callback:share-registry", &rpc.ShareRegistryInput{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return ctx, bridge
}

func TestRegisteredRadiusExternalStartupRefused(t *testing.T) {
	reg := registry.Lookup(Name)
	if reg == nil {
		t.Fatal("RADIUS provider is not registered")
	}
	pluginEnd, engineEnd := net.Pipe()
	done := make(chan struct{})
	var exitCode int
	go func() {
		exitCode = reg.RunEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("external RADIUS provider did not stop")
		}
	})
	if err := engineEnd.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var first [1]byte
	n, err := engineEnd.Read(first[:])
	if n != 0 || err != io.EOF {
		t.Errorf("external provider started RPC instead of refusing startup: bytes=%d error=%v", n, err)
	}
	_ = engineEnd.Close()
	select {
	case <-done:
		if exitCode != 1 {
			t.Errorf("external provider exit=%d, want 1", exitCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("external RADIUS provider did not return")
	}
}

func authConfigSections(config string) []rpc.ConfigSection {
	if config == "" {
		return nil
	}
	return []rpc.ConfigSection{{Root: "l2tp", Data: config}}
}

func authConfigCall(t *testing.T, ctx context.Context, bridge *rpc.DirectBridge, method string, input any, want string) {
	t.Helper()
	params, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	data, err := bridge.SendCallback(ctx, "ze-plugin-callback:"+method, params)
	if err != nil {
		t.Fatal(err)
	}
	var out rpc.ConfigApplyOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != want {
		t.Fatalf("%s status=%q error=%q, want %q", method, out.Status, out.Error, want)
	}
}

func verifyAuthConfig(t *testing.T, ctx context.Context, bridge *rpc.DirectBridge, config, status string) {
	t.Helper()
	authConfigCall(t, ctx, bridge, "config-verify", &rpc.ConfigVerifyInput{Sections: authConfigSections(config)}, status)
}

func applyAuthConfig(t *testing.T, ctx context.Context, bridge *rpc.DirectBridge) {
	t.Helper()
	authConfigCall(t, ctx, bridge, "config-apply", &rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: "l2tp"}}}, rpc.StatusOK)
}

func rollbackAuthConfig(t *testing.T, ctx context.Context, bridge *rpc.DirectBridge) {
	t.Helper()
	if _, err := bridge.SendCallback(ctx, "ze-plugin-callback:config-rollback", json.RawMessage(`{"transaction-id":"provider-reload"}`)); err != nil {
		t.Fatal(err)
	}
}

func assertProviderPAP(t *testing.T, password string, wantAccept, wantRadius bool) {
	t.Helper()
	handler := l2tp.GetAuthHandler()
	if handler == nil {
		t.Fatal("no authentication provider")
	}
	response := newFakeResponder()
	result := handler(ppp.EventAuthRequest{TunnelID: 1, SessionID: 1, Method: ppp.AuthMethodPAP,
		Username: "alice", Response: []byte(password)}, response.respond)
	if result.Handled != wantRadius {
		t.Fatalf("RADIUS ownership=%t, want %t (accept=%t, message=%q)", result.Handled, wantRadius, result.Accept, result.Message)
	}
	accept := result.Accept
	if result.Handled {
		accept = response.waitOne(t).accept
	}
	if accept != wantAccept {
		t.Fatalf("PAP password %q accepted=%t, want %t", password, accept, wantAccept)
	}
}

const localBeforeReload = `{"l2tp":{"auth":{"local":{"user":{"alice":{"password":"before"}}}}}}`
const localAfterReload = `{"l2tp":{"auth":{"local":{"user":{"alice":{"password":"after"}}}}}}`

// MUTATION: leaving OnConfigVerify stateless makes apply keep the old password;
// clearing pending without undoing an applied reload leaves rollback ineffective.
func TestRegisteredLocalCredentialReload(t *testing.T) {
	original := l2tp.GetAuthHandler()
	t.Cleanup(func() { l2tpauthlocal.ResetForTest(); l2tp.RegisterAuthHandler(original) })
	ctx, local := startAuthProvider(t, "l2tp-auth-local", localBeforeReload)
	assertProviderPAP(t, "before", true, false)
	assertProviderPAP(t, "after", false, false)
	verifyAuthConfig(t, ctx, local, localAfterReload, rpc.StatusOK)
	assertProviderPAP(t, "before", true, false)
	assertProviderPAP(t, "after", false, false)
	applyAuthConfig(t, ctx, local)
	assertProviderPAP(t, "before", false, false)
	assertProviderPAP(t, "after", true, false)
	rollbackAuthConfig(t, ctx, local)
	assertProviderPAP(t, "before", true, false)
	assertProviderPAP(t, "after", false, false)

	verifyAuthConfig(t, ctx, local, localAfterReload, rpc.StatusOK)
	verifyAuthConfig(t, ctx, local, `{"l2tp":{"auth":{"local":{"user":42}}}}`, rpc.StatusError)
	rollbackAuthConfig(t, ctx, local)
	assertProviderPAP(t, "before", true, false)
	verifyAuthConfig(t, ctx, local, localAfterReload, rpc.StatusOK)
	applyAuthConfig(t, ctx, local)
	// A failed later transaction must not roll back the preceding success.
	verifyAuthConfig(t, ctx, local, `{`, rpc.StatusError)
	rollbackAuthConfig(t, ctx, local)
	assertProviderPAP(t, "after", true, false)
	assertProviderPAP(t, "before", false, false)
	verifyAuthConfig(t, ctx, local, `{}`, rpc.StatusOK)
	applyAuthConfig(t, ctx, local)
	assertProviderPAP(t, "after", false, false)
}

func TestRegisteredLocalFirstApplyRollback(t *testing.T) {
	original := l2tp.GetAuthHandler()
	t.Cleanup(func() { l2tpauthlocal.ResetForTest(); l2tp.RegisterAuthHandler(original) })
	ctx, local := startAuthProvider(t, "l2tp-auth-local", "")
	assertProviderPAP(t, "before", false, false)
	verifyAuthConfig(t, ctx, local, localBeforeReload, rpc.StatusOK)
	applyAuthConfig(t, ctx, local)
	assertProviderPAP(t, "before", true, false)
	rollbackAuthConfig(t, ctx, local)
	assertProviderPAP(t, "before", false, false)
}

// MUTATION: ignoring errNoRADIUSConfig on reload strands the shared slot on
// RADIUS. Local credential reload must not steal that slot while RADIUS owns it.
func TestRegisteredProviderReloadTransitions(t *testing.T) {
	original := l2tp.GetAuthHandler()
	t.Cleanup(func() { l2tpauthlocal.ResetForTest(); l2tp.RegisterAuthHandler(original) })
	bus := installProviderLifecycleBus(t)
	ctx, local := startAuthProvider(t, "l2tp-auth-local", localBeforeReload)
	_, remote := startAuthProvider(t, Name, `{}`)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		params, err := json.Marshal(&rpc.ConfigureInput{Sections: authConfigSections(`{}`)})
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = remote.SendCallback(cleanupCtx, "ze-plugin-callback:configure", params)
	})
	server, address := startMockRADIUS(t, []byte("reload-key"), radius.CodeAccessReject)
	t.Cleanup(func() { _ = server.Close() })
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	radiusConfig := fmt.Sprintf(`{"l2tp":{"auth":{"radius":{"server":[{"name":"test","address":"127.0.0.1","port":%s,"shared-key":"reload-key"}]}}}}`, port)
	assertProviderPAP(t, "before", true, false)
	verifyAuthConfig(t, ctx, remote, `{"l2tp":{"auth":{"radius":{"timeout":0}}}}`, rpc.StatusError)
	rollbackAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "before", true, false)
	verifyAuthConfig(t, ctx, remote, radiusConfig, rpc.StatusOK)
	assertProviderPAP(t, "before", true, false)
	applyAuthConfig(t, ctx, remote)
	commitProviderLifecycle(t, bus)
	assertProviderPAP(t, "before", false, true)
	verifyAuthConfig(t, ctx, local, localAfterReload, rpc.StatusOK)
	applyAuthConfig(t, ctx, local)
	commitProviderLifecycle(t, bus)
	assertProviderPAP(t, "after", false, true)

	acceptServer, acceptAddress := startMockRADIUS(t, []byte("rotated-key"), radius.CodeAccessAccept)
	t.Cleanup(func() { _ = acceptServer.Close() })
	_, acceptPort, err := net.SplitHostPort(acceptAddress)
	if err != nil {
		t.Fatal(err)
	}
	rotatedConfig := fmt.Sprintf(`{"l2tp":{"auth":{"radius":{"server":[{"name":"rotated","address":"127.0.0.1","port":%s,"shared-key":"rotated-key"}]}}}}`, acceptPort)
	verifyAuthConfig(t, ctx, remote, rotatedConfig, rpc.StatusOK)
	assertProviderPAP(t, "after", false, true)
	applyAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "not-a-local-password", true, true)
	rollbackAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "after", false, true)

	verifyAuthConfig(t, ctx, remote, `{"l2tp":{"auth":{"radius":{"timeout":0}}}}`, rpc.StatusError)
	rollbackAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "after", false, true)
	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	assertProviderPAP(t, "after", false, true)
	applyAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "after", true, false)
	assertProviderPAP(t, "before", false, false)
	rollbackAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "after", false, true)
	verifyAuthConfig(t, ctx, remote, `{}`, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	commitProviderLifecycle(t, bus)
	assertProviderPAP(t, "after", true, false)

	// A new activation must be reversible back to the local owner too.
	verifyAuthConfig(t, ctx, remote, radiusConfig, rpc.StatusOK)
	applyAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "after", false, true)
	rollbackAuthConfig(t, ctx, remote)
	assertProviderPAP(t, "after", true, false)
}
