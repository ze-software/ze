// Design: docs/architecture/api/architecture.md -- gNMI transport, panic containment per RPC
// Related: panic_guard.go -- rpcTask, the boundary under test

package gnmi

import (
	"strings"
	"sync/atomic"
	"testing"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/crashlog"
)

// TestGNMIHandlerPanicContained proves the gNMI RPC boundary ends only the RPC
// or stream that met a Ze defect, with codes.Internal and a plain message, while
// the server and every later RPC on the same connection keep working; each
// panic is counted and crash-recorded on the real guard.
// METHOD: the server's config tree function panics when armed. Armed, a unary
// Get and a Subscribe ONCE stream each reach it on their handler goroutine.
// Disarmed, Capabilities and a Subscribe ONCE on the same server answer.
// PREVENTS: a client-driven panic in a gNMI handler killing the daemon (grpc-go
// recovers nothing), or a contained panic leaving the client without a status.
func TestGNMIHandlerPanicContained(t *testing.T) {
	before, declared := rpcGuard.Panics(taskRPC)
	if !declared {
		t.Fatalf("task %q is not declared on the gnmi boundary", taskRPC)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	tree := zeconfig.NewTree()
	tree.Set("hostname", "ze-test")
	var armed atomic.Bool
	treeFn := func() *zeconfig.Tree {
		if armed.Load() {
			panic("boom: tree defect")
		}
		return tree
	}
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"}, treeFn, nil, nil, nil)
	startTestServer(t, srv)
	client := dialTestServer(t, srv.Address())
	ctx := t.Context()

	armed.Store(true)
	_, err := client.Get(ctx, &gpb.GetRequest{Encoding: gpb.Encoding_JSON_IETF})
	requireInternal(t, "Get", err)

	stream, err := client.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe RPC: %v", err)
	}
	if err = stream.Send(onceRequest()); err != nil {
		t.Fatalf("Send subscribe: %v", err)
	}
	_, err = stream.Recv()
	requireInternal(t, "Subscribe", err)

	armed.Store(false)
	if _, err = client.Capabilities(ctx, &gpb.CapabilityRequest{}); err != nil {
		t.Fatalf("Capabilities after the contained panics: %v", err)
	}
	healthy, err := client.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe after the contained panics: %v", err)
	}
	if err = healthy.Send(onceRequest()); err != nil {
		t.Fatalf("Send subscribe after the contained panics: %v", err)
	}
	resp, err := healthy.Recv()
	if err != nil {
		t.Fatalf("Subscribe ONCE after the contained panics: %v", err)
	}
	if resp.GetUpdate() == nil {
		t.Fatalf("Subscribe ONCE answered %v, want an update", resp)
	}

	if count, _ := rpcGuard.Panics(taskRPC); count != before+2 {
		t.Fatalf("Panics(%q) = %d, want %d: the panics were not counted", taskRPC, count, before+2)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 2 {
		t.Fatalf("crash records = %+v, want two", crashes)
	}
	for _, crash := range crashes {
		if crash.Kind != crashlog.KindRecovered {
			t.Fatalf("crash record %+v is not %q", crash, crashlog.KindRecovered)
		}
		if record := crashlog.ReadCrash(crash.Name); !strings.Contains(record, "gnmi "+taskRPC+", unit client 127.0.0.1:") {
			t.Fatalf("crash record lacks the gnmi rpc origin:\n%s", record)
		}
	}
}

// requireInternal asserts err is the boundary's answer: codes.Internal naming
// the RPC in plain words and the command that finds the crash record.
func requireInternal(t *testing.T, method string, err error) {
	t.Helper()
	answer, isStatus := status.FromError(err)
	if !isStatus {
		t.Fatalf("%s answered %v, want a gRPC status", method, err)
	}
	if answer.Code() != codes.Internal {
		t.Fatalf("%s answered code %s (%q), want Internal", method, answer.Code(), answer.Message())
	}
	for _, want := range []string{"internal Ze defect ended this " + method + " call", "ze show crashes"} {
		if !strings.Contains(answer.Message(), want) {
			t.Fatalf("%s message %q lacks %q", method, answer.Message(), want)
		}
	}
}

// onceRequest is a Subscribe ONCE request over the whole tree.
func onceRequest() *gpb.SubscribeRequest {
	return &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Encoding: gpb.Encoding_JSON_IETF,
				Mode:     gpb.SubscriptionList_ONCE,
			},
		},
	}
}
