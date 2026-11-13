// Design: docs/architecture/api/process-protocol.md -- native dispatch admission.

package rpc

import (
	"context"
	"errors"
	"testing"
)

// MUTATION: removing DispatchNative's ready guard admits the pre-ready call.
func TestDirectBridgeNativeRefusesBeforeReady(t *testing.T) {
	bridge := NewDirectBridge()
	calls := 0
	bridge.SetDispatchNative(func(context.Context, string, any) (any, bool, error) {
		calls++
		return 7, true, nil
	})
	if value, handled, err := bridge.DispatchNative(t.Context(), "owner:native", 7); value != nil || !handled || !errors.Is(err, ErrBridgeClosed) {
		t.Errorf("pre-ready native call = %v, %v, %v", value, handled, err)
	}
	if calls != 0 {
		t.Errorf("pre-ready native call reached the owner %d times", calls)
	}
	bridge.SetReady()
	if value, handled, err := bridge.DispatchNative(t.Context(), "owner:native", 7); value != 7 || !handled || err != nil {
		t.Fatalf("ready control = %v, %v, %v", value, handled, err)
	}
	if calls != 1 {
		t.Fatalf("ready control reached the owner %d times, want once", calls)
	}
}

// MUTATION: removing DispatchNative's callbackFailure guard loses the cause.
func TestDirectBridgeNativeRetainsCallbackFailure(t *testing.T) {
	bridge := NewDirectBridge()
	calls := 0
	bridge.SetDispatchNative(func(context.Context, string, any) (any, bool, error) {
		calls++
		return 7, true, nil
	})
	bridge.SetReady()
	if value, handled, err := bridge.DispatchNative(t.Context(), "owner:native", 7); value != 7 || !handled || err != nil {
		t.Fatalf("healthy native control = %v, %v, %v", value, handled, err)
	}
	cause := errors.New("native owner callback stopped")
	bridge.FailCallbacks(cause)
	if value, handled, err := bridge.DispatchNative(t.Context(), "owner:native", 7); value != nil || !handled || !errors.Is(err, cause) {
		t.Fatalf("failed callback native call = %v, %v, %v", value, handled, err)
	}
	if calls != 1 {
		t.Fatalf("callback failure reached the owner again: %d calls", calls)
	}
}
