// Design: docs/architecture/api/process-protocol.md -- nested engine calls at Stage 2.

package ipc

import (
	"context"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SendConfigureServing delivers Stage 2 while its sole startup request reader
// serves bounded nested requests. serve returns false, nil only for the legal
// next stage: that request is retained for ReadRequest after configure completes.
// A serve error cancels and joins the configure waiter before returning it.
// The caller MUST own startup reads exclusively. The mux remains the only wire
// reader. Non-mux legacy connections keep their sequential configure behavior.
func (pc *PluginConn) SendConfigureServing(ctx context.Context, sections []rpc.ConfigSection, claims []string, serve func(*rpc.Request) (bool, error)) error {
	if pc.mux == nil {
		return pc.SendConfigure(ctx, sections, claims)
	}
	configureCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	// One lifecycle worker for Stage 2, never one goroutine per nested request.
	go func() { done <- pc.SendConfigure(configureCtx, sections, claims) }()
	defer cancel()
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
			consumed, err := serve(request)
			if err != nil {
				cancel()
				<-done
				return err
			}
			if consumed {
				continue
			}
			pc.pendingStartup = request
			// A conforming peer has answered configure before sending this
			// request. Wait for its waiter without consuming a second stage.
			return <-done
		case <-ctx.Done():
			cancel()
			<-done
			return ctx.Err()
		}
	}
}
