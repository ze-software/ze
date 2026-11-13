package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestTryEngineNativeAvailability(t *testing.T) {
	p := &Plugin{}
	if value, handled, err := p.TryEngineNative(t.Context(), "owner:native", nil); value != nil || handled || err != nil {
		t.Fatalf("external native availability = %v, %v, %v", value, handled, err)
	}
	p.bridge = rpc.NewDirectBridge()
	p.bridge.SetDispatchNative(func(context.Context, string, any) (any, bool, error) { return math.NaN(), true, nil })
	if _, handled, err := p.TryEngineNative(t.Context(), "owner:native", nil); handled || err != nil {
		t.Fatalf("startup native call = %v, %v", handled, err)
	}
	p.bridge.SetReady()
	value, handled, err := p.TryEngineNative(t.Context(), "owner:native", math.NaN())
	if !handled || err != nil {
		t.Fatalf("ready native call = %v, %v", handled, err)
	}
	number, ok := value.(float64)
	if !ok || !math.IsNaN(number) {
		t.Fatalf("native result changed: %#v", value)
	}
}

func TestTryEngineNativePublicFallback(t *testing.T) {
	b := rpc.NewDirectBridge()
	b.SetDispatchNative(func(context.Context, string, any) (any, bool, error) { return nil, false, nil })
	calls := 0
	b.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		calls++
		if method != "owner:public" || string(params) != `{"value":7}` {
			t.Errorf("public fallback input = %s %s", method, params)
		}
		return json.RawMessage(`{"value":8}`), nil
	})
	b.SetReady()
	p := &Plugin{bridge: b}
	_, handled, err := p.TryEngineNative(t.Context(), "owner:public", nil)
	if handled || err != nil {
		t.Fatalf("unsupported native operation = %v, %v", handled, err)
	}
	if calls != 0 {
		t.Fatal("native probe called the public RPC itself")
	}
	value, err := p.callEngineWithResult(t.Context(), "owner:public", struct {
		Value int `json:"value"`
	}{7})
	if err != nil || string(value) != `{"value":8}` || calls != 1 {
		t.Fatalf("public fallback = %s, %v, calls %d", value, err, calls)
	}
}

func TestTryEngineNativeStoppedBridge(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "stopped", true: "failed"}[failure], func(t *testing.T) {
			b := rpc.NewDirectBridge()
			b.SetReady()
			cause := errors.New("native plugin callback failed")
			if failure {
				b.FailCallbacks(cause)
			} else {
				b.StopDispatch()
				cause = rpc.ErrBridgeClosed
			}
			p := &Plugin{bridge: b}
			if _, handled, err := p.TryEngineNative(t.Context(), "owner:native", nil); !handled || !errors.Is(err, cause) {
				t.Fatalf("activated unavailable native call = %v, %v", handled, err)
			}
		})
	}
}

// The nil MuxConn makes an illicit return to startup IPC fail immediately.
// An activated bridge remains the transport after its readiness is withdrawn.
func TestEngineRawStoppedBridgeDoesNotUseStartupIPC(t *testing.T) {
	b := rpc.NewDirectBridge()
	b.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"value":7}`), nil
	})
	b.SetReady()
	p := &Plugin{bridge: b}
	if value, err := p.callEngineRaw(t.Context(), "owner:public", nil); err != nil || string(value) != `{"value":7}` {
		t.Fatalf("healthy direct control = %s, %v", value, err)
	}
	b.StopDispatch()
	defer func() {
		if value := recover(); value != nil {
			t.Errorf("activated stopped bridge returned to startup IPC: %v", value)
		}
	}()
	if _, err := p.callEngineRaw(t.Context(), "owner:public", nil); !errors.Is(err, rpc.ErrBridgeClosed) {
		t.Errorf("stopped direct call = %v", err)
	}
}
