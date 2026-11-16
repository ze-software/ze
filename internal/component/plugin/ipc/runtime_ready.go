// Design: docs/architecture/api/process-protocol.md -- initialization receipt
// Related: configure_serving.go -- exclusive startup request ownership

package ipc

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SendRuntimeReadyServing MUST run before the runtime request reader starts.
// It joins the plugin's declared initializer while its sole startup reader
// serves nested engine calls. The caller MUST pass a bounded startup context.
// The single callback worker is canceled and joined on every return path.
func (pc *PluginConn) SendRuntimeReadyServing(ctx context.Context, serve func(*rpc.Request) error) error {
	if pc.bridge.Load() != nil {
		// Generic bridge callbacks queue for the SDK event loop. Nested engine
		// calls already use its separately installed direct dispatch handler.
		_, err := pc.CallRPC(ctx, rpc.MethodRuntimeReady, nil)
		return err
	}
	if pc.mux == nil {
		_, err := pc.CallRPC(ctx, rpc.MethodRuntimeReady, nil)
		return err
	}
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	// One worker owns the complete initialization callback, never one worker
	// per nested request. The mux remains the only underlying wire reader.
	go func() {
		_, err := pc.CallRPC(requestCtx, rpc.MethodRuntimeReady, nil)
		done <- err
	}()
	for {
		select {
		case err := <-done:
			return err
		case request, ok := <-pc.mux.Requests():
			if !ok {
				cancel()
				<-done
				return rpc.ErrMuxConnClosed
			}
			if err := serve(request); err != nil {
				cancel()
				<-done
				return err
			}
		case <-ctx.Done():
			cancel()
			<-done
			return ctx.Err()
		}
	}
}
