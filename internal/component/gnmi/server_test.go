package gnmi

import (
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
)

func treeFunc(t *zeconfig.Tree) func() *zeconfig.Tree { return func() *zeconfig.Tree { return t } }

// startTestServer starts a gNMI server on a random port and waits for it to bind.
func startTestServer(t *testing.T, srv *Server) {
	t.Helper()
	ctx := t.Context()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		if addr := srv.Address(); addr != "" {
			return
		}
		select {
		case <-deadline:
			t.Fatal("server did not bind in time")
		case err := <-errCh:
			t.Fatalf("server failed: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func dialTestServer(t *testing.T, addr string) gpb.GNMIClient {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup
	return gpb.NewGNMIClient(conn)
}

func TestGNMICapabilitiesWiring(t *testing.T) {
	tree := zeconfig.NewTree()
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFunc(tree), nil, yangloader.DefaultLoader, nil)
	startTestServer(t, srv)

	ctx := t.Context()
	client := dialTestServer(t, srv.Address())
	resp, err := client.Capabilities(ctx, &gpb.CapabilityRequest{})
	if err != nil {
		t.Fatalf("Capabilities RPC: %v", err)
	}
	if resp.GNMIVersion == "" {
		t.Error("expected non-empty gNMI version")
	}
}

func TestAuthInterceptor(t *testing.T) {
	tree := zeconfig.NewTree()
	srv := NewServer(Config{
		ListenAddr: "127.0.0.1:0",
		Token:      "test-secret",
	}, treeFunc(tree), nil, nil, nil)
	startTestServer(t, srv)

	ctx := t.Context()
	client := dialTestServer(t, srv.Address())

	// No auth should fail.
	_, err := client.Capabilities(ctx, &gpb.CapabilityRequest{})
	if err == nil {
		t.Fatal("expected auth error without token")
	}

	// Wrong token should fail.
	badCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer wrong-token")
	_, err = client.Capabilities(badCtx, &gpb.CapabilityRequest{})
	if err == nil {
		t.Fatal("expected auth error with wrong token")
	}

	// Correct token should succeed.
	goodCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer test-secret")
	resp, err := client.Capabilities(goodCtx, &gpb.CapabilityRequest{})
	if err != nil {
		t.Fatalf("expected success with correct token, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}
