package gnmi

import (
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	gpb "github.com/openconfig/gnmi/proto/gnmi"

	"github.com/ze-software/ze/internal/component/api"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	yangloader "github.com/ze-software/ze/internal/component/config/yang"
)

func TestGNMIGetWiring(t *testing.T) {
	tree := zeconfig.NewTree()
	bgp := zeconfig.NewTree()
	bgp.Set("router-id", "1.2.3.4")
	tree.SetContainer("bgp", bgp)

	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFunc(tree), nil, nil, nil)
	startTestServer(t, srv)

	conn, err := grpc.NewClient(srv.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	client := gpb.NewGNMIClient(conn)
	ctx := t.Context()

	resp, err := client.Get(ctx, &gpb.GetRequest{
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "bgp"},
				{Name: "router-id"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Get RPC: %v", err)
	}
	if len(resp.Notification) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(resp.Notification))
	}
	val := resp.Notification[0].Update[0].Val.GetStringVal()
	if val != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %q", val)
	}
}

func TestGNMIGetNotFoundWiring(t *testing.T) {
	tree := zeconfig.NewTree()
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFunc(tree), nil, nil, nil)
	startTestServer(t, srv)

	conn, err := grpc.NewClient(srv.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	client := gpb.NewGNMIClient(conn)
	ctx := t.Context()

	_, err = client.Get(ctx, &gpb.GetRequest{
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "nonexistent"},
				{Name: "path"},
			},
		}},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NOT_FOUND, got %v", st.Code())
	}
}

func TestGNMISubscribeOnceWiring(t *testing.T) {
	tree := zeconfig.NewTree()
	tree.Set("hostname", "ze-test")

	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFunc(tree), nil, nil, nil)
	startTestServer(t, srv)

	conn, err := grpc.NewClient(srv.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	client := gpb.NewGNMIClient(conn)
	ctx := t.Context()

	stream, err := client.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe RPC: %v", err)
	}

	err = stream.Send(&gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Mode: gpb.SubscriptionList_ONCE,
			},
		},
	})
	if err != nil {
		t.Fatalf("Send subscribe: %v", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv update: %v", err)
	}
	upd := resp.GetUpdate()
	if upd == nil {
		t.Fatal("expected update notification")
	}

	resp, err = stream.Recv()
	if err != nil {
		t.Fatalf("Recv sync: %v", err)
	}
	if !resp.GetSyncResponse() {
		t.Error("expected sync_response=true")
	}
}

func TestGNMICapabilitiesModelsWiring(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFunc(zeconfig.NewTree()), nil, yangloader.DefaultLoader, nil)
	startTestServer(t, srv)

	conn, err := grpc.NewClient(srv.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	client := gpb.NewGNMIClient(conn)
	resp, err := client.Capabilities(t.Context(), &gpb.CapabilityRequest{})
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}

	if len(resp.SupportedModels) == 0 {
		t.Error("expected YANG models in capabilities response")
	}

	foundZe := false
	for _, m := range resp.SupportedModels {
		if m.Name != "" {
			foundZe = true
			break
		}
	}
	if !foundZe {
		t.Error("expected at least one named YANG model")
	}
}

// TestGNMISetWiring drives Set through the full gRPC path a real gNMI client
// takes -- dial, client.Set, server-side config session + commit. The existing
// TestGNMISetCommitsConfigSession calls srv.Set directly (server-internal); this
// is the only client-level Set coverage, closing the W-5 gap that Get/Subscribe/
// Capabilities already had.
func TestGNMISetWiring(t *testing.T) {
	editor := &configSessionEditor{values: make(map[string]string)}
	sessions := api.NewConfigSessionManager(func() (api.ConfigEditor, error) {
		return editor, nil
	})
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, nil, sessions, nil, nil)
	startTestServer(t, srv)

	conn, err := grpc.NewClient(srv.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // test cleanup

	client := gpb.NewGNMIClient(conn)
	resp, err := client.Set(t.Context(), &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{Elem: []*gpb.PathElem{
				{Name: "bgp"},
				{Name: "router-id"},
			}},
			Val: &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: "10.0.0.1"}},
		}},
	})
	if err != nil {
		t.Fatalf("Set RPC: %v", err)
	}
	if len(resp.Response) != 1 || resp.Response[0].Op != gpb.UpdateResult_UPDATE {
		t.Fatalf("expected one UPDATE result, got %+v", resp.Response)
	}
	// Confirm the update actually reached the config session (not just an OK
	// response): the fake editor records what was committed.
	if !strings.Contains(editor.committedContent, "bgp.router-id = 10.0.0.1") {
		t.Fatalf("committed content %q missing router-id update", editor.committedContent)
	}
}
