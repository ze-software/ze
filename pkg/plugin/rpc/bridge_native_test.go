package rpc

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestDirectBridgeNativeValues(t *testing.T) {
	b := NewDirectBridge()
	b.SetDispatchNative(func(ctx context.Context, method string, input any) (any, bool, error) {
		if method != "owner:native" {
			return nil, false, nil
		}
		value, ok := input.(float64)
		if !ok {
			return nil, true, errors.New("native input must be a number")
		}
		return value, true, nil
	})
	b.SetReady()
	result, handled, err := b.DispatchNative(t.Context(), "owner:native", math.NaN())
	if err != nil || !handled {
		t.Fatalf("native call: handled=%v error=%v", handled, err)
	}
	value, ok := result.(float64)
	if !ok || !math.IsNaN(value) {
		t.Fatalf("native payload changed: %#v", result)
	}
	if result, handled, err := b.DispatchNative(t.Context(), "owner:unknown", nil); result != nil || handled || err != nil {
		t.Fatalf("unknown native operation = %v, %v, %v", result, handled, err)
	}
	if _, handled, err := b.DispatchNative(t.Context(), "owner:native", "wrong type"); !handled || err == nil {
		t.Fatalf("wrong payload = handled %v, error %v", handled, err)
	}
}

func TestDirectBridgeNativeCancellation(t *testing.T) {
	b := NewDirectBridge()
	called := false
	b.SetDispatchNative(func(context.Context, string, any) (any, bool, error) {
		called = true
		return 7, true, nil
	})
	b.SetReady()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, handled, err := b.DispatchNative(ctx, "owner:native", nil); !handled || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled native call = %v, %v", handled, err)
	}
	if called {
		t.Fatal("canceled call reached native owner")
	}
	if value, handled, err := b.DispatchNative(t.Context(), "owner:native", nil); !handled || err != nil || value != 7 {
		t.Fatalf("healthy native control = %v, %v, %v", value, handled, err)
	}
}

func TestDirectBridgeNativeWithoutDispatcher(t *testing.T) {
	b := NewDirectBridge()
	b.SetReady()
	if value, handled, err := b.DispatchNative(t.Context(), "owner:public", 7); value != nil || handled || err != nil {
		t.Fatalf("ready bridge without native dispatcher = %v, %v, %v", value, handled, err)
	}
}

func TestDirectBridgeNativeDrain(t *testing.T) {
	b := NewDirectBridge()
	entered := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan struct{})
	b.SetDispatchNative(func(context.Context, string, any) (any, bool, error) {
		close(entered)
		<-release
		close(completed)
		return 7, true, nil
	})
	b.SetReady()
	result := make(chan error, 1)
	go func() {
		_, _, err := b.DispatchNative(t.Context(), "owner:native", nil)
		result <- err
	}()
	select {
	case <-entered:
	case err := <-result:
		t.Fatalf("native call was not admitted: %v", err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	b.StopDispatch()
	if _, handled, err := b.DispatchNative(t.Context(), "owner:native", nil); !handled || !errors.Is(err, ErrBridgeClosed) {
		t.Errorf("stopped native call = %v, %v", handled, err)
	}
	waiting := make(chan struct{})
	drained := make(chan bool, 1)
	go func() {
		close(waiting)
		b.WaitDispatch()
		select {
		case <-completed:
			drained <- true
		default:
			drained <- false
		}
	}()
	<-waiting
	early := false
	select {
	case <-drained:
		early = true
		t.Error("WaitDispatch returned with native handler active")
	default:
	}
	close(release)
	if err := <-result; err != nil {
		t.Errorf("admitted native call: %v", err)
	}
	if !early {
		if joined := <-drained; !joined {
			t.Error("WaitDispatch did not join native handler completion")
		}
	}
}
