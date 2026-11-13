// Design: docs/architecture/api/process-protocol.md -- nested engine calls at Stage 2.

package ipc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestConfigureServingRetainsNextStage drives the scheduling boundary where
// the next startup request is available before configure's waiter completes.
// It proves the pump stops consuming and the driver receives exactly that ID.
func TestConfigureServingRetainsNextStage(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	engineEnd, pluginEnd := net.Pipe()
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	t.Cleanup(func() {
		engineMux.Close() //nolint:errcheck // test cleanup
		pluginMux.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	engine := NewMuxPluginConn(engineMux)
	nextRead := make(chan *rpc.Request, 1)
	configureDone := make(chan error, 1)
	go func() {
		configureDone <- engine.SendConfigureServing(ctx, nil, nil, func(request *rpc.Request) (bool, error) {
			nextRead <- request
			return false, nil
		})
	}()
	var configure *rpc.Request
	select {
	case configure = <-pluginMux.Requests():
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	nextDone := make(chan error, 1)
	go func() {
		_, err := pluginMux.CallRPC(ctx, "ze-plugin-engine:declare-capabilities", nil)
		nextDone <- err
	}()
	var prefetched *rpc.Request
	select {
	case prefetched = <-nextRead:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Delay configure's reply deliberately, making its completion waiter lose
	// the race without relying on a scheduler sleep or probabilistic repeats.
	if err := pluginMux.SendOK(ctx, configure.ID); err != nil {
		t.Fatal(err)
	}
	if err := <-configureDone; err != nil {
		t.Fatal(err)
	}
	request, err := engine.ReadRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if request != prefetched {
		t.Fatal("next-stage request was lost or replaced")
	}
	if err := engine.SendOK(ctx, request.ID); err != nil {
		t.Fatal(err)
	}
	if err := <-nextDone; err != nil {
		t.Fatal(err)
	}
}

// TestConfigureServingCancellation joins the one configure lifecycle worker
// when its peer leaves configure unanswered.
func TestConfigureServingCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	engineEnd, pluginEnd := net.Pipe()
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	t.Cleanup(func() {
		engineMux.Close() //nolint:errcheck // test cleanup
		pluginMux.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	engine := NewMuxPluginConn(engineMux)
	done := make(chan error, 1)
	go func() {
		done <- engine.SendConfigureServing(ctx, nil, nil, func(*rpc.Request) (bool, error) { return true, nil })
	}()
	select {
	case <-pluginMux.Requests():
		cancel()
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("configure cancellation = %v", err)
	}
}

// TestConfigureServingReplyFailure returns a callback's exact failure while
// configure remains unanswered and its parent context is still live.
func TestConfigureServingReplyFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	engineEnd, pluginEnd := net.Pipe()
	engineMux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	pluginMux := rpc.NewMuxConn(rpc.NewConn(pluginEnd, pluginEnd))
	t.Cleanup(func() {
		engineMux.Close() //nolint:errcheck // test cleanup
		pluginMux.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
		pluginEnd.Close() //nolint:errcheck // test cleanup
	})
	engine := NewMuxPluginConn(engineMux)
	replyFailure := errors.New("nested response write failed")
	done := make(chan error, 1)
	go func() {
		done <- engine.SendConfigureServing(ctx, nil, nil, func(*rpc.Request) (bool, error) {
			return true, replyFailure
		})
	}()
	select {
	case <-pluginMux.Requests():
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	nestedDone := make(chan error, 1)
	go func() {
		_, err := pluginMux.CallRPC(ctx, "ze-owner:nested", nil)
		nestedDone <- err
	}()
	if err := <-done; !errors.Is(err, replyFailure) {
		t.Errorf("configure reply failure = %v, want %v", err, replyFailure)
	}
	if ctx.Err() != nil {
		t.Errorf("configure waited for parent cancellation: %v", ctx.Err())
	}
	cancel()
	if err := <-nestedDone; !errors.Is(err, context.Canceled) {
		t.Errorf("nested caller after reply failure = %v", err)
	}
}
