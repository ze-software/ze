// Design: docs/architecture/api/process-protocol.md -- NLRI decoder response contract.

package ipc_test

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
	_ "github.com/ze-software/ze/internal/component/bgp/server" // Register the real engine decoder RPC.
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRPCDecodeNLRIRealSDKConsumers sends native VPN NLRI through both directions
// of the SDK wire transport and consumes the actual decoder's object or array.
// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field MUST be ignored."
// Announcements retain their labels; RFC 7911 Section 3 framing preserves AP0/AP17.
//
// RFC requirement: RFC8277-2.4-1 positive -- real SDK and engine RPC consumers decode VPN withdrawals with Compatibility 0x800000 into exact RD/prefix/Path Identifier identities, for singleton objects and packed arrays.
// RFC requirement: RFC8277-2.4-1 negative -- zero, S-set and other Compatibility values neither reject withdrawals nor become labels in either transport direction; announcements retain their labels.
func TestRPCDecodeNLRIRealSDKConsumers(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	engine, p := startDecodeNLRITransport(t, ctx)

	serveDone := make(chan error, 1)
	go func() {
		for {
			req, err := engine.ReadRequest(ctx)
			if err != nil {
				serveDone <- err
				return
			}
			handler := registry.CollectRPCHandlers()[req.Method]
			if handler == nil {
				serveDone <- engine.SendError(ctx, req.ID, "unregistered decoder RPC")
				return
			}
			// RFC 8277 Section 2.4: exercise the registered engine consumer.
			result, err := handler(req.Params)
			if err != nil {
				err = engine.SendError(ctx, req.ID, err.Error())
			} else {
				err = engine.SendResult(ctx, req.ID, result)
			}
			if err != nil {
				serveDone <- err
				return
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-serveDone:
		case <-time.After(5 * time.Second):
			t.Error("decoder request worker did not stop")
		}
	})

	for _, direction := range []string{"engine-to-sdk", "sdk-to-engine"} {
		for _, family := range []string{"ipv4/mpls-vpn", "ipv6/mpls-vpn"} {
			for _, addPath := range []bool{false, true} {
				framing := "plain"
				if addPath {
					framing = "addpath"
				}
				for _, action := range []string{"announce", "withdraw"} {
					compatibilities := []string{"000641"}
					if action == "withdraw" {
						compatibilities = []string{"800000", "000000", "000641", "123450", "ffffff"}
					}
					for _, compatibility := range compatibilities {
						for _, cardinality := range []string{"object", "array"} {
							t.Run(direction+"/"+family+"/"+framing+"/"+action+"/"+compatibility+"/"+cardinality, func(t *testing.T) {
								prefixes := []string{"10.0.0.0/8", "11.0.0.0/8"}
								native := []string{"60" + compatibility + "0000fde8000000640a", "60" + compatibility + "0000fde8000000650b"}
								if family == "ipv6/mpls-vpn" {
									prefixes = []string{"2001:db8::/32", "2001:db9::/32"}
									native = []string{"78" + compatibility + "0000fde80000006420010db8", "78" + compatibility + "0000fde80000006520010db9"}
								}
								if addPath {
									native[0] = "00000000" + native[0]
									native[1] = "00000011" + native[1]
								}
								want := []any{
									map[string]any{"rd": "0:65000:100", "prefix": prefixes[0]},
									map[string]any{"rd": "0:65000:101", "prefix": prefixes[1]},
								}
								for i, route := range want {
									fields, ok := route.(map[string]any)
									if !ok {
										t.Fatal("expected route is not an object")
									}
									if addPath {
										fields["path-id"] = float64(i * 17)
									}
									if action == "announce" {
										fields["labels"] = []any{[]any{float64(100), float64(1601)}}
									}
								}
								hex := native[0]
								expected := want[0]
								if cardinality == "array" {
									hex += native[1]
									expected = want
								}
								var raw []byte
								if direction == "engine-to-sdk" {
									// RFC 8277 Section 2.4: consume the real SDK callback response.
									result, err := engine.SendDecodeNLRI(ctx, family, hex, addPath, action == "withdraw")
									if err != nil {
										t.Fatal(err)
									}
									raw = []byte(result)
								} else {
									// RFC 8277 Section 2.4: consume the registered engine handler response.
									result, err := p.DecodeNLRI(ctx, family, hex, addPath, action == "withdraw")
									if err != nil {
										t.Fatal(err)
									}
									raw = result
								}
								var got any
								if err := json.Unmarshal(raw, &got); err != nil {
									t.Fatal(err)
								}
								if !reflect.DeepEqual(got, expected) {
									t.Fatalf("decoded native routes = %s, want %#v", raw, expected)
								}
							})
						}
					}
				}
			}
		}
	}
}

// startDecodeNLRITransport follows the existing SDK test five-stage handshake.
// The external package avoids the vpn -> sdk -> ipc import cycle. The test owns
// both endpoints and MUST close them and join the SDK lifecycle during cleanup.
func startDecodeNLRITransport(t *testing.T, ctx context.Context) (*ipc.PluginConn, *sdk.Plugin) {
	t.Helper()
	engineEnd, pluginEnd := net.Pipe()
	engine := ipc.NewMuxPluginConn(rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)))
	p := sdk.NewWithConn("vpn-decoder-response", pluginEnd)
	p.OnDecodeNLRI(vpn.DecodeNLRIHex)
	runDone := make(chan error, 1)
	go func() { runDone <- p.Run(ctx, sdk.Registration{}) }()
	t.Cleanup(func() {
		// Cleanup MUST close both endpoints before joining Run.
		if err := p.Close(); err != nil {
			t.Errorf("close SDK: %v", err)
		}
		if err := engine.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
			t.Error("SDK lifecycle did not stop")
		}
	})
	for _, stage := range []struct {
		request  string
		callback string
		input    any
	}{
		{rpc.MethodDeclareRegistration, "ze-plugin-callback:configure", rpc.ConfigureInput{}},
		{"ze-plugin-engine:declare-capabilities", "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{}},
		{"ze-plugin-engine:ready", "ze-plugin-callback:deliver-event", rpc.DeliverEventInput{Event: "{}"}},
	} {
		req, err := engine.ReadRequest(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if req.Method != stage.request {
			t.Fatalf("startup request = %q, want %q", req.Method, stage.request)
		}
		if err := engine.SendOK(ctx, req.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.CallRPC(ctx, stage.callback, stage.input); err != nil {
			t.Fatal(err)
		}
	}
	return engine, p
}
